package ledgerbackend

import (
	"context"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/pkg/errors"

	"github.com/stellar/go-stellar-sdk/support/collections/heap"
	"github.com/stellar/go-stellar-sdk/support/datastore"
)

// maxBatchObjectSize bounds the size of a single batch object — used both for
// the compressed-bytes pre-allocation (driven by data store Content-Length /
// Attrs.Size) and the decompressed-bytes pre-allocation (driven by zstd's
// FrameContentSize header). Either source could be a corrupt or hostile
// number that would otherwise trigger an arbitrarily large allocation.
// 1 GiB is well above any realistic Stellar batch size.
const maxBatchObjectSize = 1 << 30

type ledgerBatchObject struct {
	payload     []byte
	startLedger int // Ledger sequence used as the priority for the priorityqueue.
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
	// Passed through from BufferedStorageBackend to control lifetime of ledgerBuffer instance
	config    BufferedStorageBackendConfig
	dataStore datastore.DataStore
	schema    datastore.DataStoreSchema

	// zstdDecoder is a single zstd decoder used by the BSB consumer to
	// decompress batches one-at-a-time. Sequential decompression keeps peak
	// heap small and avoids the GC pressure that comes with N concurrent
	// DecodeAll calls in workers.
	zstdDecoder *zstd.Decoder

	// compressedPool reuses the small (~14 KB) buffers that workers fill from
	// the data store. Workers Get from it; the consumer Puts back inside
	// decompress() once the bytes have been decompressed.
	compressedPool bufferPool

	// decompressedPool reuses buffers for decompressed batch data — driven by
	// the BSB consumer (via decompress + returnBuffer).
	decompressedPool bufferPool

	// lastDecompressedSize is the size of the previously decompressed batch,
	// used to pre-size the next decompress's destination buffer when the zstd
	// frame carries no FrameContentSize (streaming-compressed objects). Batches
	// are similar-sized, so this lets decompress reuse a pooled buffer instead
	// of letting DecodeAll allocate the whole output every call. Consumer-only
	// (written from decompress), so no lock.
	lastDecompressedSize int

	// context used to cancel workers within the ledgerBuffer
	context context.Context
	cancel  context.CancelCauseFunc

	wg sync.WaitGroup

	// closeOnce ensures close() is idempotent — cancel + Wait + decoder.Close
	// are safe to repeat, but keeping the same shape here matches the rest of
	// the lifecycle.
	closeOnce sync.Once

	// The pipes and data structures below help establish the ledgerBuffer invariant which is
	// the number of tasks (both pending and in-flight) + len(ledgerQueue) + ledgerPriorityQueue.Len()
	// is always less than or equal to bufferSize (= min(config.BufferSize, range_size+1), the cap of taskQueue/ledgerQueue)
	taskQueue           chan uint32                   // Buffer next object read
	ledgerQueue         chan []byte                   // Order corrected lcm batches (compressed)
	ledgerPriorityQueue *heap.Heap[ledgerBatchObject] // Priority is set to the sequence number
	priorityQueueLock   sync.Mutex

	// Keep track of the ledgers to be processed and the next ordering
	// the ledgers should be buffered
	// Byte accounting for BufferBytes. Atomics, not fields under
	// priorityQueueLock: overBudget runs on the consumer goroutine, which must
	// not take that lock — storeObject holds it across a blocking send.
	//
	// heldBytes/queuedCount: queued payload, charged at buffer capacity. Their
	// quotient is the mean used for unstarted tasks; taken from the queue, not
	// a lifetime total, so it tracks objects growing across history.
	heldBytes   atomic.Int64
	queuedCount atomic.Int64
	// Downloads running now, charged at the size the datastore reports.
	inFlightBytes atomic.Int64
	inFlightCount atomic.Int64
	// Fallback mean for when the consumer has drained the queue and the
	// quotient above is undefined; without it the budget stops applying there.
	lastSize atomic.Int64
	// Tasks pushed but not yet consumed — what BufferSize bounds. Consumer-owned.
	outstanding atomic.Int64

	currentLedger     uint32 // The current ledger that should be popped from ledgerPriorityQueue
	nextTaskLedger    uint32 // The next task ledger that should be added to taskQueue
	ledgerRange       Range
	currentLedgerLock sync.RWMutex
}

func (bsb *BufferedStorageBackend) newLedgerBuffer(ledgerRange Range) (*ledgerBuffer, error) {
	ctx, cancel := context.WithCancelCause(context.Background())

	// Use a local buffer size to avoid mutating the shared config.
	bufferSize := bsb.config.BufferSize
	if ledgerRange.bounded {
		bufferSize = uint32(min(int(bufferSize), int(ledgerRange.to-ledgerRange.from)+1))
	}

	// WithDecoderConcurrency(1): this decoder is driven serially by the BSB
	// consumer (decompress() is consumer-thread-only), so a single
	// decode/buffer combo is correct AND the most memory-efficient choice for
	// DecodeAll. Concurrency 0 (=GOMAXPROCS) keeps GOMAXPROCS decoder combos
	// and allocates per-call decode state, which dominated allocation under
	// parallel multi-chunk ingest.
	decoder, err := zstd.NewReader(nil, zstd.WithDecoderConcurrency(1))
	if err != nil {
		cancel(err)
		return nil, errors.Wrap(err, "failed to create zstd decoder")
	}

	config := bsb.config
	config.BufferSize = bufferSize

	less := func(a, b ledgerBatchObject) bool {
		return a.startLedger < b.startLedger
	}

	lb := &ledgerBuffer{
		config:      config,
		dataStore:   bsb.dataStore,
		schema:      bsb.schema,
		zstdDecoder: decoder,
		// Sized by concurrency, not queue depth: only NumWorkers Gets can be
		// outstanding, so further slots are retention heldBytes cannot see. Get
		// discards only UNDERSIZED buffers, so a BufferSize-sized pool spanning
		// object growth ends holding BufferSize tip-sized buffers.
		compressedPool:      newBufferPool(int(config.NumWorkers) + 1),
		decompressedPool:    newBufferPool(2),
		taskQueue:           make(chan uint32, bufferSize),
		ledgerQueue:         make(chan []byte, bufferSize),
		ledgerPriorityQueue: heap.New(less, int(bufferSize)),
		currentLedger:       ledgerRange.from,
		nextTaskLedger:      ledgerRange.from,
		ledgerRange:         ledgerRange,
		context:             ctx,
		cancel:              cancel,
	}

	// Start workers to read LCM files
	lb.wg.Add(int(bsb.config.NumWorkers))
	for i := uint32(0); i < bsb.config.NumWorkers; i++ {
		go lb.worker(ctx)
	}

	// Upon initialization, the ledgerBuffer invariant is maintained because
	// we create up to bufferSize+1 tasks (bufferSize in the queue + 1 in-flight
	// in a worker) while len(ledgerQueue) and ledgerPriorityQueue.Len() are 0.
	// Effectively, this is len(taskQueue) + len(ledgerQueue) + ledgerPriorityQueue.Len() <= bufferSize
	// which enforces a limit of max tasks (both pending and in-flight) to be less than or equal to bufferSize+1.
	// Note: when a task is in-flight it is no longer in the taskQueue
	// but for easier conceptualization, len(taskQueue) can be interpreted as both pending and in-flight tasks
	// where we assume the workers are empty and not processing any tasks.
	// Nothing has arrived yet, so the budget cannot bind: open with just enough
	// to saturate the workers and let the first consume take it from there.
	initial := int(bufferSize)
	if config.BufferBytes > 0 {
		initial = min(initial, int(config.NumWorkers))
	}
	for i := 0; i <= initial; i++ {
		lb.pushTaskQueue()
	}

	return lb, nil
}

// overBudget reports whether the pipeline has reached BufferBytes. Queued
// payload and running downloads are charged exactly; only dispatched-but-
// unstarted tasks are estimated. Bounding arrived bytes alone would bound the
// queue, not the pipeline behind it.
func (lb *ledgerBuffer) overBudget() bool {
	if lb.config.BufferBytes <= 0 {
		return false
	}
	held := lb.heldBytes.Load()
	queued := lb.queuedCount.Load()
	projected := held + lb.inFlightBytes.Load()

	mean := lb.lastSize.Load()
	if queued > 0 {
		mean = held / queued
	}
	if mean > 0 {
		notStarted := max(lb.outstanding.Load()-queued-lb.inFlightCount.Load(), 0)
		projected += notStarted * mean
	}
	return projected >= lb.config.BufferBytes
}

// replenish refills to whichever bound binds first. With the budget disabled
// this is one task per consumed object — the previous behavior.
func (lb *ledgerBuffer) replenish() {
	// <=, not <: the constructor fills to BufferSize+1 and the pre-BufferBytes
	// code pushed one per consume, so that is the depth to restore.
	for lb.outstanding.Load() <= int64(lb.config.BufferSize) && !lb.overBudget() {
		if !lb.pushTaskQueue() {
			return // past the range end, or shutting down: nothing left to queue
		}
	}
}

// pushTaskQueue queues the next task and reports whether it did. It owns the
// outstanding count, so a push that no-ops — past the range end, or on
// cancellation — cannot inflate it with a task that will never arrive.
func (lb *ledgerBuffer) pushTaskQueue() bool {
	// In bounded mode, don't queue past the end boundary ledger for the specified range.
	if lb.ledgerRange.bounded && lb.nextTaskLedger > lb.schema.GetSequenceNumberEndBoundary(lb.ledgerRange.to) {
		return false
	}
	// Guard the send with lb.context.Done() so a consumer can't deadlock
	// here if cancellation arrives mid-send: workers exit on ctx.Done and
	// stop draining taskQueue, which would otherwise leave this goroutine
	// stuck on the unbounded send forever.
	select {
	case lb.taskQueue <- lb.nextTaskLedger:
		lb.nextTaskLedger += lb.schema.LedgersPerFile
		lb.outstanding.Add(1)
		return true
	case <-lb.context.Done():
		return false
	}
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
				ledgerObject, charge, err := lb.downloadLedgerObject(ctx, sequence)
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
				// This keeps the overall total the same (<= bufferSize). As long as the the ledger buffer invariant
				// was maintained in the previous state, it is still maintained during this state transition.
				// Release AFTER registration, so the two charges briefly overlap
				// rather than briefly vanish.
				stored := lb.storeObject(ctx, ledgerObject, sequence)
				lb.releaseInFlight(charge)
				if !stored {
					return
				}
				break
			}
		}
	}
}

// storeObject is called by a worker when its download completes. It pushes
// the result onto the priority queue and drains everything now in-order to
// ledgerQueue, all under priorityQueueLock. Returns false if the context was
// cancelled mid-drain (caller should exit).
//
// Doing the ordering+emit work in worker context (rather than in a separate
// writer goroutine) avoids an extra channel hop and the scheduler-delay
// penalty that comes from goroutine wake-ups under heavy concurrent download
// load.
//
// The send to ledgerQueue happens while priorityQueueLock is held. This is
// intentional: when ledgerQueue is full the lock-blocked send naturally
// throttles other workers' storeObject calls, providing backpressure that
// caps in-flight memory. At BufferSize >= NumWorkers the queue rarely
// fills, so the serialization cost is negligible in practice.
func (lb *ledgerBuffer) storeObject(ctx context.Context, payload []byte, sequence uint32) bool {
	lb.priorityQueueLock.Lock()
	defer lb.priorityQueueLock.Unlock()

	// Capacity, not length: buffers are pooled and over-allocated, so capacity
	// is what the process holds — ~2.3x payload on pubnet tip objects.
	lb.heldBytes.Add(int64(cap(payload)))
	lb.queuedCount.Add(1)
	lb.lastSize.Store(int64(cap(payload)))
	lb.ledgerPriorityQueue.Push(ledgerBatchObject{payload: payload, startLedger: int(sequence)})

	// Check if the nextLedger is the next item in the ledgerPriorityQueue
	// The ledgerBuffer invariant is maintained here because items are transferred from the ledgerPriorityQueue to the ledgerQueue.
	// Thus the overall sum of ledgerPriorityQueue.Len() + len(lb.ledgerQueue) remains the same.
	for lb.ledgerPriorityQueue.Len() > 0 && lb.currentLedger == uint32(lb.ledgerPriorityQueue.Peek().startLedger) {
		item := lb.ledgerPriorityQueue.Pop()
		select {
		case lb.ledgerQueue <- item.payload:
		case <-ctx.Done():
			// Popped but never delivered: getFromLedgerQueue will not see it,
			// so release its charge here or the counters drift on shutdown.
			lb.releaseHeld(item.payload)
			return false
		case <-lb.context.Done():
			lb.releaseHeld(item.payload)
			return false
		}
		lb.currentLedgerLock.Lock()
		lb.currentLedger += lb.schema.LedgersPerFile
		lb.currentLedgerLock.Unlock()
	}
	return true
}

// downloadLedgerObject fetches the compressed object bytes only. Decompression
// is deferred to the consumer (BSB.loadBatchForSequence) so that, with N
// concurrent workers, only one zstd context is alive at a time — keeping peak
// heap small and GC overhead low. The returned slice is from compressedPool;
// the consumer returns it via returnCompressedBuffer after decompressing.
// downloadLedgerObject returns the object and the in-flight charge the CALLER
// must release once the payload is registered in heldBytes. Releasing here
// would leave a window where the bytes exist but are priced as a mean-sized
// unstarted task, letting a larger-than-mean object dispatch past the budget.
func (lb *ledgerBuffer) downloadLedgerObject(ctx context.Context, sequence uint32) ([]byte, int64, error) {
	objectKey := lb.schema.GetObjectKeyFromSequenceNumber(sequence)

	reader, compressedSize, err := lb.dataStore.GetFile(ctx, objectKey)
	if err != nil {
		return nil, 0, errors.Wrapf(err, "unable to retrieve file: %s", objectKey)
	}
	defer reader.Close()

	// One predicate for both pre-allocation and byte accounting: an implausible
	// size (corrupt Content-Length) must drive neither.
	sized := compressedSize > 0 && compressedSize <= maxBatchObjectSize

	// Charged on entry; handed to the caller on success, released here on every
	// error path.
	handedOff := false
	if sized {
		lb.inFlightBytes.Add(compressedSize)
		lb.inFlightCount.Add(1)
		defer func() {
			if !handedOff {
				lb.releaseInFlight(compressedSize)
			}
		}()
	}

	// Without a usable size, fall back to ReadAll, which grows naturally and is
	// bounded by what the reader actually produces.
	var compressed []byte
	if sized {
		compressed = lb.compressedPool.Get(int(compressedSize))
		if _, err = io.ReadFull(reader, compressed); err != nil {
			lb.compressedPool.Put(compressed)
			return nil, 0, errors.Wrapf(err, "failed reading file: %s", objectKey)
		}
	} else {
		compressed, err = io.ReadAll(reader)
		if err != nil {
			return nil, 0, errors.Wrapf(err, "failed reading file: %s", objectKey)
		}
	}

	handedOff = true
	if !sized {
		return compressed, 0, nil
	}
	return compressed, compressedSize, nil
}

// releaseHeld drops a queued payload's charge.
func (lb *ledgerBuffer) releaseHeld(payload []byte) {
	lb.heldBytes.Add(-int64(cap(payload)))
	lb.queuedCount.Add(-1)
}

// releaseInFlight drops a charge taken by downloadLedgerObject.
func (lb *ledgerBuffer) releaseInFlight(charge int64) {
	if charge <= 0 {
		return
	}
	lb.inFlightBytes.Add(-charge)
	lb.inFlightCount.Add(-1)
}

// decompress turns a compressed batch into decompressed bytes and returns
// the compressed buffer to the pool. Must be called only by the BSB
// consumer: keeping a single zstd context active at a time avoids the GC
// pressure that comes with N concurrent DecodeAll calls in workers.
func (lb *ledgerBuffer) decompress(compressed []byte) ([]byte, error) {
	defer lb.compressedPool.Put(compressed)

	// Pre-size the destination so DecodeAll appends into a pooled buffer
	// instead of allocating the whole output on every call. Prefer the frame's
	// FrameContentSize; when it's absent (streaming-compressed objects carry no
	// FCS) fall back to the previous batch's size — batches are similar-sized,
	// so this keeps the decompressedPool effective rather than letting every
	// DecodeAll grow from nil. An implausibly large FCS (corrupt/hostile) is
	// skipped, letting DecodeAll grow naturally.
	sizeHint := lb.lastDecompressedSize
	var header zstd.Header
	if err := header.Decode(compressed); err == nil && header.HasFCS && header.FrameContentSize <= maxBatchObjectSize {
		sizeHint = int(header.FrameContentSize)
	}
	var dst []byte
	if sizeHint > 0 {
		dst = lb.decompressedPool.Get(sizeHint)[:0]
	}

	decompressed, err := lb.zstdDecoder.DecodeAll(compressed, dst)
	if err != nil {
		if dst != nil {
			lb.decompressedPool.Put(dst)
		}
		return nil, errors.Wrap(err, "failed decompressing batch")
	}

	// If DecodeAll had to reallocate (dst too small), return the original
	// pool buffer so it isn't leaked.
	if dst != nil && cap(decompressed) != cap(dst) {
		lb.decompressedPool.Put(dst)
	}

	// Clamp the fallback hint so a single outsized/corrupt batch can't
	// permanently inflate the pooled buffer for subsequent (normally
	// similar-sized) batches — mirrors the maxBatchObjectSize cap on the
	// FrameContentSize path above.
	lb.lastDecompressedSize = min(len(decompressed), maxBatchObjectSize)
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
			// The ledger buffer invariant is maintained here because
			// we create an extra task when consuming one item from the ledger queue.
			// Thus len(ledgerQueue) decreases by 1 and the number of tasks increases by 1.
			// The overall sum below remains the same:
			// len(taskQueue) + len(ledgerQueue) + ledgerPriorityQueue.Len() <= bufferSize
			lb.releaseHeld(batchBytes)
			lb.outstanding.Add(-1)
			lb.replenish()
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

	// Subtract 1 to get the latest ledger in buffer
	return lb.currentLedger - 1, nil
}

func (lb *ledgerBuffer) close() {
	lb.closeOnce.Do(func() {
		lb.cancel(context.Canceled)
		// wait for all workers to finish terminating
		lb.wg.Wait()
		if lb.zstdDecoder != nil {
			lb.zstdDecoder.Close()
		}
	})
}
