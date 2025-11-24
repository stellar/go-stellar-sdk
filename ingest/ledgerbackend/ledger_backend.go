package ledgerbackend

import (
	"context"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// LedgerBackend represents the interface to a ledger data store.
type LedgerBackend interface {
	// GetLatestLedgerSequence returns the sequence of the latest ledger available
	// in the backend.
	GetLatestLedgerSequence(ctx context.Context) (sequence uint32, err error)
	// GetLedger will block until the ledger is available.
	GetLedger(ctx context.Context, sequence uint32) (xdr.LedgerCloseMeta, error)
	// PrepareRange prepares the given range (including from and to) to be loaded.
	// Some backends (like captive stellar-core) need to initalize data to be
	// able to stream ledgers. Blocks until the first ledger is available.
	PrepareRange(ctx context.Context, ledgerRange Range) error
	// IsPrepared returns true if a given ledgerRange is prepared.
	IsPrepared(ctx context.Context, ledgerRange Range) (bool, error)
	Close() error
}
