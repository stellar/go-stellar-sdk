// BufferedStorageBackend is a ledger backend that provides buffered access over a given DataStore.
// The DataStore must contain files generated from a LedgerExporter.

package ledgerbackend

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/stellar/go-stellar-sdk/support/datastore"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Ensure BufferedStorageBackend implements LedgerBackend
var _ LedgerBackend = (*BufferedStorageBackend)(nil)

type BufferedStorageBackendConfig struct {
	BufferSize uint32        `toml:"buffer_size"`
	NumWorkers uint32        `toml:"num_workers"`
	RetryLimit uint32        `toml:"retry_limit"`
	RetryWait  time.Duration `toml:"retry_wait"`
}

// BufferedStorageBackend is a ledger backend that reads from a storage service.
// The storage service contains files generated from the ledgerExporter.
//
// Except for the Close function, BufferedStorageBackend is not thread-safe and
// should not be accessed by multiple go routines. Close is thread-safe and can
// be called from another go routine. Once Close is called it will interrupt and
// cancel any pending operations.
type BufferedStorageBackend struct {
	config BufferedStorageBackendConfig

	// bsBackendLock is held by Close (write) so it can interrupt in-flight
	// operations, and by every other method (read) so they observe a stable
	// closed flag. Concurrent reads between non-Close methods are *not* made
	// safe by this lock — the type's thread-safety contract requires callers
	// to serialize externally; the read lock here only protects against
	// concurrent Close.
	bsBackendLock sync.RWMutex

	// ledgerBuffer is the buffer for LedgerCloseMeta data read in parallel.
	ledgerBuffer *ledgerBuffer

	dataStore  datastore.DataStore
	schema     datastore.DataStoreSchema
	prepared   *Range // Non-nil if any range is prepared
	closed     bool   // False until the core is closed
	nextLedger uint32
	lastLedger uint32

	// Current batch state
	batchBytes   []byte                    // raw decompressed bytes (for pool return)
	ledgerSlices []xdr.LedgerCloseMetaView // per-ledger exact-extent views into batchBytes
	batchStart   uint32
	batchEnd     uint32
	batchIndex   uint32 // how many ledgers consumed from this batch
}

// NewBufferedStorageBackend returns a new BufferedStorageBackend instance.
func NewBufferedStorageBackend(config BufferedStorageBackendConfig, dataStore datastore.DataStore, schema datastore.DataStoreSchema) (*BufferedStorageBackend, error) {
	if config.BufferSize == 0 {
		return nil, errors.New("buffer size must be > 0")
	}

	if config.NumWorkers == 0 {
		return nil, errors.New("number of workers must be > 0")
	}

	if config.NumWorkers > config.BufferSize {
		return nil, errors.New("number of workers must be <= BufferSize")
	}

	if schema.LedgersPerFile <= 0 {
		return nil, errors.New("ledgersPerFile must be > 0")
	}

	bsBackend := &BufferedStorageBackend{
		config:    config,
		dataStore: dataStore,
		schema:    schema,
	}

	return bsBackend, nil
}

// GetLatestLedgerSequence returns the most recent ledger sequence number available in the buffer.
func (bsb *BufferedStorageBackend) GetLatestLedgerSequence(ctx context.Context) (uint32, error) {
	bsb.bsBackendLock.RLock()
	defer bsb.bsBackendLock.RUnlock()

	if bsb.closed {
		return 0, errors.New("BufferedStorageBackend is closed; cannot GetLatestLedgerSequence")
	}

	if bsb.prepared == nil {
		return 0, errors.New("BufferedStorageBackend must be prepared, call PrepareRange first")
	}

	latestSeq, err := bsb.ledgerBuffer.getLatestLedgerSequence()
	if err != nil {
		return 0, err
	}

	return latestSeq, nil
}

// loadBatchForSequence ensures the raw batch containing the given sequence is loaded.
// If the sequence is within the current batch, it's a no-op.
// Otherwise, it fetches the next batch from the ledger queue and pre-computes
// byte offsets for each ledger boundary using the generated view API.
func (bsb *BufferedStorageBackend) loadBatchForSequence(ctx context.Context, sequence uint32) error {
	// Check if sequence is in the current batch
	if bsb.batchBytes != nil && sequence >= bsb.batchStart && sequence <= bsb.batchEnd {
		return nil
	}

	// Sequence is before the current batch
	if bsb.batchBytes != nil && sequence < bsb.batchStart {
		return errors.New("requested sequence precedes current LedgerCloseMetaBatch")
	}

	// Return the old batch buffer to the pool before loading a new one.
	if bsb.batchBytes != nil {
		bsb.ledgerBuffer.returnBuffer(bsb.batchBytes)
		bsb.batchBytes = nil
	}

	// Pull compressed batch from queue and decompress on consumer thread.
	compressed, err := bsb.ledgerBuffer.getFromLedgerQueue(ctx)
	if err != nil {
		return errors.Wrap(err, "failed getting next ledger batch from queue")
	}
	batchBytes, err := bsb.ledgerBuffer.decompress(compressed)
	if err != nil {
		return errors.Wrap(err, "failed decompressing batch")
	}
	installed := false
	defer func() {
		if !installed {
			bsb.ledgerBuffer.returnBuffer(batchBytes)
		}
	}()

	view := xdr.LedgerCloseMetaBatchView(batchBytes)

	startView, err := view.StartSequence()
	if err != nil {
		return fmt.Errorf("reading batch start sequence: %w", err)
	}
	start, err := startView.Value()
	if err != nil {
		return fmt.Errorf("reading batch start sequence value: %w", err)
	}
	endView, err := view.EndSequence()
	if err != nil {
		return fmt.Errorf("reading batch end sequence: %w", err)
	}
	end, err := endView.Value()
	if err != nil {
		return fmt.Errorf("reading batch end sequence value: %w", err)
	}
	metas, err := view.LedgerCloseMetas()
	if err != nil {
		return fmt.Errorf("reading batch ledger metas: %w", err)
	}
	slices, err := metas.All()
	if err != nil {
		return fmt.Errorf("materializing batch ledgers: %w", err)
	}
	if len(slices) == 0 {
		return fmt.Errorf("batch is empty: startSequence=%d endSequence=%d", start, end)
	}

	bsb.batchBytes = batchBytes
	bsb.ledgerSlices = slices
	bsb.batchStart = start
	bsb.batchEnd = end
	bsb.batchIndex = 0
	installed = true

	return nil
}

// nextExpectedSequence returns nextLedger (if currently set) or start of
// prepared range. Otherwise it returns 0.
// nextLedger is 0 before the first batch is loaded; in that case we return
// the first ledger in the prepared range.
func (bsb *BufferedStorageBackend) nextExpectedSequence() uint32 {
	if bsb.nextLedger == 0 && bsb.prepared != nil {
		return bsb.prepared.from
	}
	return bsb.nextLedger
}

func (bsb *BufferedStorageBackend) validateSequence(sequence uint32) error {
	if bsb.closed {
		return errors.New("BufferedStorageBackend is closed; cannot GetLedger or GetLedgerRaw")
	}
	if bsb.prepared == nil {
		return errors.New("session is not prepared, call PrepareRange first")
	}
	if sequence < bsb.ledgerBuffer.ledgerRange.from {
		return fmt.Errorf("requested sequence %d precedes current LedgerRange [%d, %d]",
			sequence, bsb.ledgerBuffer.ledgerRange.from, bsb.ledgerBuffer.ledgerRange.to)
	}
	if bsb.ledgerBuffer.ledgerRange.bounded {
		if sequence > bsb.ledgerBuffer.ledgerRange.to {
			return fmt.Errorf("requested sequence %d beyond current LedgerRange [%d, %d]",
				sequence, bsb.ledgerBuffer.ledgerRange.from, bsb.ledgerBuffer.ledgerRange.to)
		}
	}
	if sequence < bsb.lastLedger {
		return fmt.Errorf("requested sequence %d precedes the lastLedger %d", sequence, bsb.lastLedger)
	}
	if sequence > bsb.nextExpectedSequence() {
		return fmt.Errorf("requested sequence %d is not the lastLedger %d nor the next available ledger %d",
			sequence, bsb.lastLedger, bsb.nextExpectedSequence())
	}
	return nil
}

// getLedgerRaw is the internal implementation that returns raw XDR bytes for a
// single LedgerCloseMeta. The returned bytes alias an internal buffer and are
// only valid until the next getLedgerRaw call. Caller must hold bsBackendLock.
func (bsb *BufferedStorageBackend) getLedgerRaw(ctx context.Context, sequence uint32) ([]byte, error) {
	if err := bsb.validateSequence(sequence); err != nil {
		return nil, err
	}

	if err := bsb.loadBatchForSequence(ctx, sequence); err != nil {
		return nil, err
	}

	targetIndex := sequence - bsb.batchStart
	i := int(targetIndex)
	if i >= len(bsb.ledgerSlices) {
		return nil, fmt.Errorf("ledger index %d out of range for batch", i)
	}
	rawBytes := []byte(bsb.ledgerSlices[i])

	// Only advance state when we're actually consuming a new ledger.
	// Re-requests of the most-recently-served sequence return the cached
	// bytes idempotently (matches CaptiveStellarCore.GetLedger behavior).
	// validateSequence already rejects sequences earlier than lastLedger.
	if targetIndex >= bsb.batchIndex {
		bsb.batchIndex = targetIndex + 1
		bsb.lastLedger = bsb.nextLedger
		bsb.nextLedger++
	}

	return rawBytes, nil
}

// GetLedgerRaw returns the raw XDR bytes for a single LedgerCloseMeta
// without performing XDR decoding. This is significantly faster than GetLedger
// when the caller doesn't need a decoded struct (e.g., for forwarding,
// replication, or when the caller will decode selectively).
//
// For the common case of LedgersPerFile=1, this avoids all XDR decoding
// and simply returns the bytes after the batch header.
//
// The returned byte slice is a safe copy that the caller owns.
func (bsb *BufferedStorageBackend) GetLedgerRaw(ctx context.Context, sequence uint32) ([]byte, error) {
	bsb.bsBackendLock.RLock()
	defer bsb.bsBackendLock.RUnlock()

	rawBytes, err := bsb.getLedgerRaw(ctx, sequence)
	if err != nil {
		return nil, err
	}
	result := make([]byte, len(rawBytes))
	copy(result, rawBytes)
	return result, nil
}

// GetLedger returns the LedgerCloseMeta for the specified ledger sequence number.
func (bsb *BufferedStorageBackend) GetLedger(ctx context.Context, sequence uint32) (xdr.LedgerCloseMeta, error) {
	bsb.bsBackendLock.RLock()
	defer bsb.bsBackendLock.RUnlock()

	// Use internal getLedgerRaw which returns aliased bytes — safe here because
	// we decode immediately before the next call can recycle the buffer.
	rawBytes, err := bsb.getLedgerRaw(ctx, sequence)
	if err != nil {
		return xdr.LedgerCloseMeta{}, err
	}
	var lcm xdr.LedgerCloseMeta
	if err := xdr.SafeUnmarshal(rawBytes, &lcm); err != nil {
		return xdr.LedgerCloseMeta{}, fmt.Errorf("error decoding ledger %d: %w", sequence, err)
	}
	return lcm, nil
}

// PrepareRange checks if the starting and ending (if bounded) ledgers exist.
func (bsb *BufferedStorageBackend) PrepareRange(ctx context.Context, ledgerRange Range) error {
	bsb.bsBackendLock.Lock()
	defer bsb.bsBackendLock.Unlock()

	if bsb.closed {
		return errors.New("BufferedStorageBackend is closed; cannot PrepareRange")
	}

	if alreadyPrepared, err := bsb.startPreparingRange(ledgerRange); err != nil {
		return errors.Wrap(err, "error starting prepare range")
	} else if alreadyPrepared {
		return nil
	}

	bsb.prepared = &ledgerRange

	return nil
}

// IsPrepared returns true if a given ledgerRange is prepared.
func (bsb *BufferedStorageBackend) IsPrepared(ctx context.Context, ledgerRange Range) (bool, error) {
	bsb.bsBackendLock.RLock()
	defer bsb.bsBackendLock.RUnlock()

	if bsb.closed {
		return false, errors.New("BufferedStorageBackend is closed; cannot IsPrepared")
	}

	return bsb.isPrepared(ledgerRange), nil
}

func (bsb *BufferedStorageBackend) isPrepared(ledgerRange Range) bool {
	if bsb.closed {
		return false
	}

	if bsb.prepared == nil {
		return false
	}

	if bsb.ledgerBuffer.ledgerRange.from > ledgerRange.from {
		return false
	}

	if bsb.ledgerBuffer.ledgerRange.bounded && !ledgerRange.bounded {
		return false
	}

	if !bsb.ledgerBuffer.ledgerRange.bounded && !ledgerRange.bounded {
		return true
	}

	if !bsb.ledgerBuffer.ledgerRange.bounded && ledgerRange.bounded {
		return true
	}

	if bsb.ledgerBuffer.ledgerRange.to >= ledgerRange.to {
		return true
	}

	return false
}

// Close closes existing BufferedStorageBackend processes.
// Note, once a BufferedStorageBackend instance is closed it can no longer be used and
// all subsequent calls to PrepareRange(), GetLedger(), etc will fail.
// Close is thread-safe and can be called from another go routine.
func (bsb *BufferedStorageBackend) Close() error {
	bsb.bsBackendLock.RLock()
	defer bsb.bsBackendLock.RUnlock()

	if bsb.ledgerBuffer != nil {
		bsb.ledgerBuffer.close()
	}
	bsb.closed = true

	return nil
}

// releaseCurrentBatch returns the batch buffer to the pool and closes the ledger buffer.
func (bsb *BufferedStorageBackend) releaseCurrentBatch() {
	if bsb.batchBytes != nil && bsb.ledgerBuffer != nil {
		bsb.ledgerBuffer.returnBuffer(bsb.batchBytes)
	}
	bsb.batchBytes = nil
	bsb.ledgerSlices = nil
	if bsb.ledgerBuffer != nil {
		bsb.ledgerBuffer.close()
		bsb.ledgerBuffer = nil
	}
}

// startPreparingRange prepares the ledger range by setting the range in the ledgerBuffer
func (bsb *BufferedStorageBackend) startPreparingRange(ledgerRange Range) (bool, error) {
	if bsb.isPrepared(ledgerRange) {
		return true, nil
	}

	bsb.releaseCurrentBatch()

	var err error
	bsb.ledgerBuffer, err = bsb.newLedgerBuffer(ledgerRange)
	if err != nil {
		return false, err
	}

	bsb.nextLedger = ledgerRange.from

	return false, nil
}
