package ledgerbackend

import (
	"context"
	"io"
	"os"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/pkg/errors"

	"github.com/stellar/go-stellar-sdk/support/datastore"
)

// workerResult is sent from download workers to the writer goroutine.
type workerResult struct {
	payload     []byte
	startLedger uint32
}

// bufferPool provides reusable byte buffers using a channel-based pool.
// Unlike sync.Pool, buffers are NOT subject to GC collection, eliminating
// repeated madvise/memclr overhead from re-allocating large buffers.
type bufferPool struct {
	ch chan []byte
}

func newBufferPool(size int) bufferPool {
	return bufferPool{ch: make(chan []byte, size)}
}

func (p *bufferPool) Get(size int) []byte {
	select {
	case buf := <-p.ch:
		if cap(buf) >= size {
			return buf[:size]
		}
		// Undersized — discard it (let GC collect) so the pool naturally
		// fills with correctly-sized buffers as new ones are allocated.
	default:
	}
	// Allocate 25% extra capacity to absorb ledger size variation.
	return make([]byte, size, size+size/4)
}

func (p *bufferPool) Put(buf []byte) {
	if buf == nil {
		return
	}
	select {
	case p.ch <- buf[:0]:
	default:
	}
}

type ledgerBuffer struct {
	config    BufferedStorageBackendConfig
	dataStore datastore.DataStore
	schema    datastore.DataStoreSchema

	// zstdDecoder is a shared zstd decoder used by workers for decompression.
	// DecodeAll is safe for concurrent use — it uses an internal pool of block decoders.
	zstdDecoder *zstd.Decoder

	// decompressedPool reuses buffers for decompressed batch data.
	decompressedPool bufferPool

	context context.Context
	cancel  context.CancelCauseFunc

	workerWg sync.WaitGroup
	writerWg sync.WaitGroup

	// Pipeline: taskQueue -> workers -> resultChan -> writer -> ledgerQueue -> consumer
	taskQueue   chan uint32
	resultChan  chan workerResult
	ledgerQueue chan []byte

	nextTaskLedger uint32
	ledgerRange    Range

	currentLedger     uint32
	currentLedgerLock sync.RWMutex
}

func (bsb *BufferedStorageBackend) newLedgerBuffer(ledgerRange Range) (*ledgerBuffer, error) {
	ctx, cancel := context.WithCancelCause(context.Background())

	// Use a local buffer size to avoid mutating the shared config.
	bufferSize := bsb.config.BufferSize
	if ledgerRange.bounded {
		bufferSize = uint32(min(int(bufferSize), int(ledgerRange.to-ledgerRange.from)+1))
	}

	decoder, err := zstd.NewReader(nil, zstd.WithDecoderConcurrency(0))
	if err != nil {
		cancel(err)
		return nil, errors.Wrap(err, "failed to create zstd decoder")
	}

	config := bsb.config
	config.BufferSize = bufferSize

	lb := &ledgerBuffer{
		config:           config,
		dataStore:        bsb.dataStore,
		schema:           bsb.schema,
		zstdDecoder:      decoder,
		decompressedPool: newBufferPool(int(bufferSize) + 1),
		taskQueue:        make(chan uint32, bufferSize),
		resultChan:       make(chan workerResult, bufferSize),
		ledgerQueue:      make(chan []byte, bufferSize),
		currentLedger:    ledgerRange.from,
		nextTaskLedger:   ledgerRange.from,
		ledgerRange:      ledgerRange,
		context:          ctx,
		cancel:           cancel,
	}

	lb.workerWg.Add(int(bsb.config.NumWorkers))
	for i := uint32(0); i < bsb.config.NumWorkers; i++ {
		go lb.worker(ctx)
	}

	lb.writerWg.Add(1)
	go lb.writer()

	for i := 0; i <= int(bufferSize); i++ {
		lb.pushTaskQueue()
	}

	return lb, nil
}

func (lb *ledgerBuffer) pushTaskQueue() {
	if lb.ledgerRange.bounded && lb.nextTaskLedger > lb.schema.GetSequenceNumberEndBoundary(lb.ledgerRange.to) {
		return
	}
	lb.taskQueue <- lb.nextTaskLedger
	lb.nextTaskLedger += lb.schema.LedgersPerFile
}

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
	defer lb.workerWg.Done()

	var compressedBuf []byte

	for {
		select {
		case <-ctx.Done():
			return
		case sequence := <-lb.taskQueue:
			for attempt := uint32(0); attempt <= lb.config.RetryLimit; {
				ledgerObject, err := lb.downloadLedgerObject(ctx, sequence, &compressedBuf)
				if err != nil {
					if errors.Is(err, os.ErrNotExist) {
						if !lb.ledgerRange.bounded {
							if !lb.sleepWithContext(ctx, lb.config.RetryWait) {
								return
							}
							continue
						}
						lb.cancel(errors.Wrapf(err, "ledger object containing sequence %v is missing", sequence))
						return
					}
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

				select {
				case lb.resultChan <- workerResult{payload: ledgerObject, startLedger: sequence}:
				case <-ctx.Done():
					return
				}
				break
			}
		}
	}
}

// writer receives unordered results from workers, accumulates out-of-order
// results in a map, and emits them to ledgerQueue in sequential order.
func (lb *ledgerBuffer) writer() {
	defer lb.writerWg.Done()

	pending := make(map[uint32][]byte)
	nextLedger := lb.ledgerRange.from

	for result := range lb.resultChan {
		pending[result.startLedger] = result.payload

		for payload, ok := pending[nextLedger]; ok; payload, ok = pending[nextLedger] {
			delete(pending, nextLedger)

			lb.currentLedgerLock.Lock()
			lb.currentLedger = nextLedger + lb.schema.LedgersPerFile
			lb.currentLedgerLock.Unlock()

			select {
			case lb.ledgerQueue <- payload:
			case <-lb.context.Done():
				return
			}
			nextLedger += lb.schema.LedgersPerFile
		}
	}
}

func (lb *ledgerBuffer) downloadLedgerObject(ctx context.Context, sequence uint32, compressedBuf *[]byte) ([]byte, error) {
	objectKey := lb.schema.GetObjectKeyFromSequenceNumber(sequence)

	reader, compressedSize, err := lb.dataStore.GetFile(ctx, objectKey)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to retrieve file: %s", objectKey)
	}
	defer reader.Close()

	if compressedSize > 0 {
		if int64(cap(*compressedBuf)) < compressedSize {
			*compressedBuf = make([]byte, compressedSize)
		} else {
			*compressedBuf = (*compressedBuf)[:compressedSize]
		}
		_, err = io.ReadFull(reader, *compressedBuf)
	} else {
		*compressedBuf, err = io.ReadAll(reader)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed reading file: %s", objectKey)
	}

	// Pre-allocate the decompression buffer from the pool if possible.
	var dst []byte
	var header zstd.Header
	if err = header.Decode(*compressedBuf); err == nil && header.HasFCS {
		dst = lb.decompressedPool.Get(int(header.FrameContentSize))[:0]
	}

	decompressed, err := lb.zstdDecoder.DecodeAll(*compressedBuf, dst)
	if err != nil {
		if dst != nil {
			lb.decompressedPool.Put(dst)
		}
		return nil, errors.Wrapf(err, "failed decompressing file: %s", objectKey)
	}

	// If DecodeAll had to reallocate (dst too small), return the original
	// pool buffer so it isn't leaked.
	if dst != nil && cap(decompressed) != cap(dst) {
		lb.decompressedPool.Put(dst)
	}

	return decompressed, nil
}

func (lb *ledgerBuffer) getFromLedgerQueue(ctx context.Context) ([]byte, error) {
	for {
		select {
		case <-lb.context.Done():
			return nil, context.Cause(lb.context)
		case <-ctx.Done():
			return nil, ctx.Err()
		case batchBytes := <-lb.ledgerQueue:
			lb.pushTaskQueue()
			return batchBytes, nil
		}
	}
}

// returnBuffer returns a decompressed batch buffer to the pool for reuse.
func (lb *ledgerBuffer) returnBuffer(buf []byte) {
	lb.decompressedPool.Put(buf)
}

func (lb *ledgerBuffer) getLatestLedgerSequence() (uint32, error) {
	lb.currentLedgerLock.RLock()
	defer lb.currentLedgerLock.RUnlock()

	if lb.currentLedger == lb.ledgerRange.from {
		return 0, nil
	}

	return lb.currentLedger - 1, nil
}

func (lb *ledgerBuffer) close() {
	lb.cancel(context.Canceled)
	lb.workerWg.Wait()
	close(lb.resultChan)
	lb.writerWg.Wait()
	if lb.zstdDecoder != nil {
		lb.zstdDecoder.Close()
	}
}
