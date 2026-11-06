package ingest

import (
	"context"
	"io"

	"github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// ChangeReader provides convenient, streaming access to a sequence of Changes.
type ChangeReader interface {
	// Read should return the next `Change` in the leader. If there are no more
	// changes left it should return an `io.EOF` error.
	Read() (Change, error)
	// Close should be called when reading is finished. This is especially
	// helpful when there are still some changes available so reader can stop
	// streaming them.
	Close() error
}

// ledgerChangeReaderState defines possible states of LedgerChangeReader.
type ledgerChangeReaderState int

const (
	// feeChangesState is active when LedgerChangeReader is reading fee changes.
	feeChangesState ledgerChangeReaderState = iota
	// metaChangesState is active when LedgerChangeReader is reading transaction meta changes.
	// For parallel phase transactions (CAP-63) only txChangesBefore is streamed per
	// transaction; their remaining changes are deferred to the end of their phase.
	metaChangesState
	// postTxApplyState is active when LedgerChangeReader is reading postTxApplyFeeProcessing changes
	postTxApplyState
	// upgradeChanges is active when LedgerChangeReader is reading upgrade changes.
	upgradeChangesState
)

// LedgerChangeReader is a ChangeReader which returns Changes from Stellar Core
// for a single ledger
type LedgerChangeReader struct {
	*LedgerTransactionReader
	state        ledgerChangeReaderState
	pending      []Change
	pendingIndex int
	// deferred holds the operation changes of the parallel phase being read in
	// metaChangesState. It is flushed into pending when the phase ends and is
	// bounded by one phase.
	deferred     []Change
	upgradeIndex int
}

// Ensure LedgerChangeReader implements ChangeReader
var _ ChangeReader = (*LedgerChangeReader)(nil)

// NewLedgerChangeReader constructs a new LedgerChangeReader instance bound to the given ledger.
// Note that the returned LedgerChangeReader is not thread safe and should not be shared
// by multiple goroutines.
func NewLedgerChangeReader(ctx context.Context, backend ledgerbackend.LedgerBackend, networkPassphrase string, sequence uint32) (*LedgerChangeReader, error) {
	transactionReader, err := NewLedgerTransactionReader(ctx, backend, networkPassphrase, sequence)
	if err != nil {
		return nil, err
	}

	return &LedgerChangeReader{
		LedgerTransactionReader: transactionReader,
		state:                   feeChangesState,
	}, nil
}

// NewLedgerChangeReaderFromLedgerCloseMeta constructs a new LedgerChangeReader instance bound to the given ledger.
// Note that the returned LedgerChangeReader is not thread safe and should not be shared
// by multiple goroutines.
func NewLedgerChangeReaderFromLedgerCloseMeta(networkPassphrase string, ledger xdr.LedgerCloseMeta) (*LedgerChangeReader, error) {
	transactionReader, err := NewLedgerTransactionReaderFromLedgerCloseMeta(networkPassphrase, ledger)
	if err != nil {
		return nil, err
	}

	return &LedgerChangeReader{
		LedgerTransactionReader: transactionReader,
		state:                   feeChangesState,
	}, nil
}

type compactingChangeReader struct {
	input     ChangeReader
	changes   []Change
	compacted bool
	config    ChangeCompactorConfig
}

func (c *compactingChangeReader) compact() error {
	compactor := NewChangeCompactor(c.config)
	for {
		change, err := c.input.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err = compactor.AddChange(change); err != nil {
			return err
		}
	}
	c.changes = compactor.GetChanges()
	c.compacted = true
	return nil
}

func (c *compactingChangeReader) Read() (Change, error) {
	if !c.compacted {
		if err := c.compact(); err != nil {
			return Change{}, err
		}
	}
	if len(c.changes) == 0 {
		return Change{}, io.EOF
	}
	change := c.changes[0]
	c.changes = c.changes[1:]
	return change, nil
}

func (c *compactingChangeReader) Close() error {
	return c.input.Close()
}

// NewCompactingChangeReader wraps a given ChangeReader and returns a ChangeReader
// which compacts all the the Changes extracted from the input.
func NewCompactingChangeReader(input ChangeReader, config ChangeCompactorConfig) ChangeReader {
	return &compactingChangeReader{
		input:  input,
		config: config,
	}
}

// Read returns the next change in the stream.
// If there are no changes remaining io.EOF is returned as an error.
func (r *LedgerChangeReader) Read() (Change, error) {
	// Changes within a ledger should be read in the following order:
	// - fee changes of all transactions,
	// - transaction meta changes of all transactions (parallel phase txChangesBefore
	//   before any parallel phase operation change, see metaChangesState),
	// - post tx apply fee changes for all transactions,
	// - upgrade changes.
	// Because a single transaction can introduce many changes we read all the
	// changes from a single transaction  and save them in r.pending.
	// When Read() is called we stream pending changes first. We also call Read()
	// recursively after adding some changes (what will return them from r.pending)
	// to not duplicate the code.
	if r.pendingIndex < len(r.pending) {
		next := r.pending[r.pendingIndex]
		r.pendingIndex++
		if r.pendingIndex == len(r.pending) {
			r.pendingIndex = 0
			r.pending = r.pending[:0]
		}
		return next, nil
	}

	switch r.state {
	case feeChangesState, metaChangesState, postTxApplyState:
		tx, err := r.LedgerTransactionReader.Read()
		if err != nil {
			if err == io.EOF {
				// If done streaming fee changes rewind to stream meta changes
				// If done streaming meta changes flush the deferred changes of the last phase
				// and rewind to stream postTxApply changes
				// If done streaming postTxApply changes then we progress to the next state (upgrade changes)
				if r.state == metaChangesState {
					r.flushDeferred()
				}
				if r.state != postTxApplyState {
					r.LedgerTransactionReader.Rewind()
				}
				r.state++
				return r.Read()
			}
			return Change{}, err
		}

		switch r.state {
		case feeChangesState:
			r.pending = append(r.pending, tx.GetFeeChanges()...)
		case metaChangesState:
			// Phases are applied in index order, so the first transaction of a
			// phase means every transaction of the previous phase has been read.
			idx := int(tx.Index) - 1
			start, parallel := r.phaseOf(idx)
			if idx == start {
				r.flushDeferred()
			}
			changes, nBefore, err := tx.changesWithSplit()
			if err != nil {
				return Change{}, err
			}
			if parallel {
				// txChangesBefore now, operation changes (and txChangesAfter, if
				// present) once the phase ends.
				r.pending = append(r.pending, changes[:nBefore]...)
				r.deferred = append(r.deferred, changes[nBefore:]...)
			} else {
				r.pending = append(r.pending, changes...)
			}
		case postTxApplyState:
			r.pending = append(r.pending, tx.GetPostApplyFeeChanges()...)
		}
		return r.Read()

	case upgradeChangesState:
		// Get upgrade changes
		if r.upgradeIndex < len(r.LedgerTransactionReader.lcm.UpgradesProcessing()) {
			changes := GetChangesFromLedgerEntryChanges(
				r.LedgerTransactionReader.lcm.UpgradesProcessing()[r.upgradeIndex].Changes,
			)
			ledgerUpgrades := r.LedgerTransactionReader.lcm.UpgradesProcessing()
			for i := range changes {
				changes[i].Reason = LedgerEntryChangeReasonUpgrade
				changes[i].Ledger = &r.lcm
				changes[i].LedgerUpgrade = &ledgerUpgrades[r.upgradeIndex].Upgrade
			}
			r.pending = append(r.pending, changes...)
			r.upgradeIndex++
			return r.Read()
		}
	}

	return Change{}, io.EOF
}

// flushDeferred moves the deferred parallel phase changes into pending.
func (r *LedgerChangeReader) flushDeferred() {
	r.pending = append(r.pending, r.deferred...)
	r.deferred = nil
}

// Close should be called when reading is finished.
func (r *LedgerChangeReader) Close() error {
	r.pending = nil
	r.deferred = nil
	return r.LedgerTransactionReader.Close()
}
