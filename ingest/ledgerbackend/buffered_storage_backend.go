// BufferedStorageBackend is a ledger backend that provides buffered access over a given DataStore.
// The DataStore must contain files generated from a LedgerExporter.

package ledgerbackend

import (
	"context"
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
type BufferedStorageBackend struct {
	config BufferedStorageBackendConfig

	bsBackendLock sync.RWMutex

	// ledgerBuffer is the buffer for LedgerCloseMeta data read in parallel.
	ledgerBuffer *ledgerBuffer

	dataStore  datastore.DataStore
	schema     datastore.DataStoreSchema
	prepared   *Range // Non-nil if any range is prepared
	closed     bool   // False until the core is closed
	lcmBatch   xdr.LedgerCloseMetaBatch
	nextLedger uint32
	lastLedger uint32
}

// NewBufferedStorageBackend returns a new BufferedStorageBackend instance.
func NewBufferedStorageBackend(config BufferedStorageBackendConfig, dataStore datastore.DataStore, schema datastore.DataStoreSchema) (*BufferedStorageBackend, error) {
	if config.BufferSize == 0 {
		return nil, errors.New("buffer size must be > 0")
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

// getBatchForSequence checks if the requested sequence is in the cached batch.
// Otherwise will continuously load in the next LedgerCloseMetaBatch until found.
func (bsb *BufferedStorageBackend) getBatchForSequence(ctx context.Context, sequence uint32) error {
	// Sequence inside the current cached LedgerCloseMetaBatch
	if sequence >= uint32(bsb.lcmBatch.StartSequence) && sequence <= uint32(bsb.lcmBatch.EndSequence) {
		return nil
	}

	// Sequence is before the current LedgerCloseMetaBatch
	// Does not support retrieving LedgerCloseMeta before the current cached batch
	if sequence < uint32(bsb.lcmBatch.StartSequence) {
		return errors.New("requested sequence precedes current LedgerCloseMetaBatch")
	}

	// Sequence is beyond the current LedgerCloseMetaBatch
	var err error
	bsb.lcmBatch, err = bsb.ledgerBuffer.getFromLedgerQueue(ctx)
	if err != nil {
		return errors.Wrap(err, "failed getting next ledger batch from queue")
	}
	return nil
}

// nextExpectedSequence returns nextLedger (if currently set) or start of
// prepared range. Otherwise it returns 0.
// This is done because `nextLedger` is 0 between the moment Stellar-Core is
// started and streaming the first ledger (in such case we return first ledger
// in requested range).
func (bsb *BufferedStorageBackend) nextExpectedSequence() uint32 {
	if bsb.nextLedger == 0 && bsb.prepared != nil {
		return bsb.prepared.from
	}
	return bsb.nextLedger
}

// GetLedger returns the LedgerCloseMeta for the specified ledger sequence number
func (bsb *BufferedStorageBackend) GetLedger(ctx context.Context, sequence uint32) (xdr.LedgerCloseMeta, error) {
	bsb.bsBackendLock.RLock()
	defer bsb.bsBackendLock.RUnlock()

	if bsb.closed {
		return xdr.LedgerCloseMeta{}, errors.New("BufferedStorageBackend is closed; cannot GetLedger")
	}

	if bsb.prepared == nil {
		return xdr.LedgerCloseMeta{}, errors.New("session is not prepared, call PrepareRange first")
	}

	if sequence < bsb.ledgerBuffer.ledgerRange.from {
		return xdr.LedgerCloseMeta{}, errors.New("requested sequence preceeds current LedgerRange")
	}

	if bsb.ledgerBuffer.ledgerRange.bounded {
		if sequence > bsb.ledgerBuffer.ledgerRange.to {
			return xdr.LedgerCloseMeta{}, errors.New("requested sequence beyond current LedgerRange")
		}
	}

	if sequence < bsb.lastLedger {
		return xdr.LedgerCloseMeta{}, errors.New("requested sequence preceeds the lastLedger")
	}

	if sequence > bsb.nextExpectedSequence() {
		return xdr.LedgerCloseMeta{}, errors.New("requested sequence is not the lastLedger nor the next available ledger")
	}

	err := bsb.getBatchForSequence(ctx, sequence)
	if err != nil {
		return xdr.LedgerCloseMeta{}, err
	}

	ledgerCloseMeta, err := bsb.lcmBatch.GetLedger(sequence)
	if err != nil {
		return xdr.LedgerCloseMeta{}, err
	}
	bsb.lastLedger = bsb.nextLedger
	bsb.nextLedger++

	return ledgerCloseMeta, nil
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

// startPreparingRange prepares the ledger range by setting the range in the ledgerBuffer
func (bsb *BufferedStorageBackend) startPreparingRange(ledgerRange Range) (bool, error) {
	if bsb.isPrepared(ledgerRange) {
		return true, nil
	}

	var err error
	bsb.ledgerBuffer, err = bsb.newLedgerBuffer(ledgerRange)
	if err != nil {
		return false, err
	}

	bsb.nextLedger = ledgerRange.from

	return false, nil
}
