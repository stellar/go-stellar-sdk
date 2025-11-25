package ledgerbackend

import (
	"bytes"
	"context"
	"io"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/stellar/go-stellar-sdk/support/collections/heap"
	"github.com/stellar/go-stellar-sdk/support/compressxdr"
	"github.com/stellar/go-stellar-sdk/support/datastore"
	"github.com/stellar/go-stellar-sdk/xdr"
)

type ledgerBatchObject struct {
	payload     []byte
	startLedger int // Ledger sequence used as the priority for the priorityqueue.
}

type ledgerBuffer struct {
	// Passed through from BufferedStorageBackend to control lifetime of ledgerBuffer instance
	config    BufferedStorageBackendConfig
	dataStore datastore.DataStore
	schema    datastore.DataStoreSchema

	// context used to cancel workers within the ledgerBuffer
	context context.Context
	cancel  context.CancelCauseFunc

	wg sync.WaitGroup

	// The pipes and data structures below help establish the ledgerBuffer invariant which is
	// the number of tasks (both pending and in-flight) + len(ledgerQueue) + ledgerPriorityQueue.Len()
	// is always less than or equal to the config.BufferSize
	taskQueue           chan uint32                   // Buffer next object read
	ledgerQueue         chan []byte                   // Order corrected lcm batches
	ledgerPriorityQueue *heap.Heap[ledgerBatchObject] // Priority is set to the sequence number
	priorityQueueLock   sync.Mutex

	// Keep track of the ledgers to be processed and the next ordering
	// the ledgers should be buffered
	currentLedger     uint32 // The current ledger that should be popped from ledgerPriorityQueue
	nextTaskLedger    uint32 // The next task ledger that should be added to taskQueue
	ledgerRange       Range
	currentLedgerLock sync.RWMutex
}

func (bsb *BufferedStorageBackend) newLedgerBuffer(ledgerRange Range) (*ledgerBuffer, error) {
	ctx, cancel := context.WithCancelCause(context.Background())

	less := func(a, b ledgerBatchObject) bool {
		return a.startLedger < b.startLedger
	}
	// ensure BufferSize does not exceed the total range
	if ledgerRange.bounded {
		bsb.config.BufferSize = uint32(min(int(bsb.config.BufferSize), int(ledgerRange.to-ledgerRange.from)+1))
	}
	pq := heap.New(less, int(bsb.config.BufferSize))

	ledgerBuffer := &ledgerBuffer{
		config:              bsb.config,
		dataStore:           bsb.dataStore,
		schema:              bsb.schema,
		taskQueue:           make(chan uint32, bsb.config.BufferSize),
		ledgerQueue:         make(chan []byte, bsb.config.BufferSize),
		ledgerPriorityQueue: pq,
		currentLedger:       ledgerRange.from,
		nextTaskLedger:      ledgerRange.from,
		ledgerRange:         ledgerRange,
		context:             ctx,
		cancel:              cancel,
	}

	// Start workers to read LCM files
	ledgerBuffer.wg.Add(int(bsb.config.NumWorkers))
	for i := uint32(0); i < bsb.config.NumWorkers; i++ {
		go ledgerBuffer.worker(ctx)
	}

	// Upon initialization, the ledgerBuffer invariant is maintained because
	// we create bsb.config.BufferSize tasks while the len(ledgerQueue) and ledgerPriorityQueue.Len() are 0.
	// Effectively, this is len(taskQueue) + len(ledgerQueue) + ledgerPriorityQueue.Len() <= bsb.config.BufferSize
	// which enforces a limit of max tasks (both pending and in-flight) to be less than or equal to bsb.config.BufferSize.
	// Note: when a task is in-flight it is no longer in the taskQueue
	// but for easier conceptualization, len(taskQueue) can be interpreted as both pending and in-flight tasks
	// where we assume the workers are empty and not processing any tasks.
	for i := 0; i <= int(bsb.config.BufferSize); i++ {
		ledgerBuffer.pushTaskQueue()
	}

	return ledgerBuffer, nil
}

func (lb *ledgerBuffer) pushTaskQueue() {
	// In bounded mode, don't queue past the end boundary ledger for the specified range.
	if lb.ledgerRange.bounded && lb.nextTaskLedger > lb.schema.GetSequenceNumberEndBoundary(lb.ledgerRange.to) {
		return
	}
	lb.taskQueue <- lb.nextTaskLedger
	lb.nextTaskLedger += lb.schema.LedgersPerFile
}

// sleepWithContext returns true upon sleeping without interruption from the context
func (lb *ledgerBuffer) sleepWithContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
		return false
	case <-timer.C:
	}
	return true
}

func (lb *ledgerBuffer) worker(ctx context.Context) {
	defer lb.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case sequence := <-lb.taskQueue:
			for attempt := uint32(0); attempt <= lb.config.RetryLimit; {
				ledgerObject, err := lb.downloadLedgerObject(ctx, sequence)
				if err != nil {
					if errors.Is(err, os.ErrNotExist) {
						// ledgerObject not found and unbounded
						if !lb.ledgerRange.bounded {
							if !lb.sleepWithContext(ctx, lb.config.RetryWait) {
								return
							}
							continue
						}
						lb.cancel(errors.Wrapf(err, "ledger object containing sequence %v is missing", sequence))
						return
					}
					// don't bother retrying if we've received the signal to shut down
					if errors.Is(err, context.Canceled) {
						return
					}
					if attempt == lb.config.RetryLimit {
						err = errors.Wrapf(err, "maximum retries exceeded for downloading object containing sequence %v", sequence)
						lb.cancel(err)
						return
					}
					attempt++
					if !lb.sleepWithContext(ctx, lb.config.RetryWait) {
						return
					}
					continue
				}

				// When we store an object we still maintain the ledger buffer invariant because
				// at this point the current task is finished and we add 1 ledger object to the priority queue.
				// Thus, the number of tasks decreases by 1 and the priority queue length increases by 1.
				// This keeps the overall total the same (<= BufferSize). As long as the the ledger buffer invariant
				// was maintained in the previous state, it is still maintained during this state transition.
				lb.storeObject(ledgerObject, sequence)
				break
			}
		}
	}
}

func (lb *ledgerBuffer) downloadLedgerObject(ctx context.Context, sequence uint32) ([]byte, error) {
	objectKey := lb.schema.GetObjectKeyFromSequenceNumber(sequence)

	reader, err := lb.dataStore.GetFile(ctx, objectKey)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to retrieve file: %s", objectKey)
	}

	defer reader.Close()

	objectBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.Wrapf(err, "failed reading file: %s", objectKey)
	}

	return objectBytes, nil
}

func (lb *ledgerBuffer) storeObject(ledgerObject []byte, sequence uint32) {
	lb.priorityQueueLock.Lock()
	defer lb.priorityQueueLock.Unlock()

	lb.currentLedgerLock.Lock()
	defer lb.currentLedgerLock.Unlock()

	lb.ledgerPriorityQueue.Push(ledgerBatchObject{
		payload:     ledgerObject,
		startLedger: int(sequence),
	})

	// Check if the nextLedger is the next item in the ledgerPriorityQueue
	// The ledgerBuffer invariant is maintained here because items are transferred from the ledgerPriorityQueue to the ledgerQueue.
	// Thus the overall sum of ledgerPriorityQueue.Len() + len(lb.ledgerQueue) remains the same.
	for lb.ledgerPriorityQueue.Len() > 0 && lb.currentLedger == uint32(lb.ledgerPriorityQueue.Peek().startLedger) {
		item := lb.ledgerPriorityQueue.Pop()
		lb.ledgerQueue <- item.payload
		lb.currentLedger += lb.schema.LedgersPerFile
	}
}

func (lb *ledgerBuffer) getFromLedgerQueue(ctx context.Context) (xdr.LedgerCloseMetaBatch, error) {
	for {
		select {
		case <-lb.context.Done():
			return xdr.LedgerCloseMetaBatch{}, context.Cause(lb.context)
		case <-ctx.Done():
			return xdr.LedgerCloseMetaBatch{}, ctx.Err()
		case compressedBinary := <-lb.ledgerQueue:
			// The ledger buffer invariant is maintained here because
			// we create an extra task when consuming one item from the ledger queue.
			// Thus len(ledgerQueue) decreases by 1 and the number of tasks increases by 1.
			// The overall sum below remains the same:
			// len(taskQueue) + len(ledgerQueue) + ledgerPriorityQueue.Len() <= bsb.config.BufferSize
			lb.pushTaskQueue()

			lcmBatch := xdr.LedgerCloseMetaBatch{}
			decoder := compressxdr.NewXDRDecoder(compressxdr.DefaultCompressor, &lcmBatch)
			_, err := decoder.ReadFrom(bytes.NewReader(compressedBinary))
			if err != nil {
				return xdr.LedgerCloseMetaBatch{}, err
			}

			return lcmBatch, nil
		}
	}
}

func (lb *ledgerBuffer) getLatestLedgerSequence() (uint32, error) {
	lb.currentLedgerLock.Lock()
	defer lb.currentLedgerLock.Unlock()

	if lb.currentLedger == lb.ledgerRange.from {
		return 0, nil
	}

	// Subtract 1 to get the latest ledger in buffer
	return lb.currentLedger - 1, nil
}

func (lb *ledgerBuffer) close() {
	lb.cancel(context.Canceled)
	// wait for all workers to finish terminating
	lb.wg.Wait()
}
