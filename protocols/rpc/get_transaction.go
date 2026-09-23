package protocol

import (
	"errors"
	"fmt"
)

const (
	GetTransactionMethodName = "getTransaction"
	// TransactionStatusSuccess indicates the transaction was included in the ledger and
	// it was executed without errors.
	TransactionStatusSuccess = "SUCCESS"
	// TransactionStatusNotFound indicates the transaction was not found in Stellar-RPC's
	// transaction store.
	TransactionStatusNotFound = "NOT_FOUND"
	// TransactionStatusFailed indicates the transaction was included in the ledger and
	// it was executed with an error.
	TransactionStatusFailed = "FAILED"
)

// GetTransactionResponse is the response for the Stellar-RPC getTransaction() endpoint
type GetTransactionResponse struct {
	// LatestLedger is the latest ledger stored in Stellar-RPC.
	LatestLedger uint32 `json:"latestLedger"`
	// LatestLedgerCloseTime is the unix timestamp of when the latest ledger was closed.
	LatestLedgerCloseTime int64 `json:"latestLedgerCloseTime,string"`
	// LatestLedger is the oldest ledger stored in Stellar-RPC.
	OldestLedger uint32 `json:"oldestLedger"`
	// LatestLedgerCloseTime is the unix timestamp of when the oldest ledger was closed.
	OldestLedgerCloseTime int64 `json:"oldestLedgerCloseTime,string"`

	// Many of the fields below are only present if Status is not
	// TransactionNotFound.
	TransactionDetails
	// LedgerCloseTime is the unix timestamp of when the transaction was
	// included in the ledger. It isn't part of `TransactionInfo` because of a
	// bug in which `createdAt` in getTransactions is encoded as a number
	// whereas in getTransaction (singular) it's encoded as a string.
	LedgerCloseTime int64 `json:"createdAt,string"`
}

type GetTransactionRequest struct {
	Hash   string `json:"hash"`
	Format string `json:"xdrFormat,omitempty"`
	// MinLedger and MaxLedger restrict the lookup to ledgers in
	// [MinLedger, MaxLedger], both inclusive. Zero leaves that side
	// unbounded: the lookup then starts at the server's oldest ledger, or
	// ends at its latest. A transaction outside the bounds is reported
	// NOT_FOUND. Bounds beyond the server's ledger range are clamped, not
	// rejected.
	MinLedger uint32 `json:"minLedger,omitempty"`
	MaxLedger uint32 `json:"maxLedger,omitempty"`
}

// IsValid checks the validity of the request parameters.
func (req GetTransactionRequest) IsValid() error {
	return errors.Join(
		IsValidFormat(req.Format),
		validateLedgerBounds(req.MinLedger, req.MaxLedger),
	)
}

func validateLedgerBounds(lo, hi uint32) error {
	if lo != 0 && hi != 0 && lo > hi {
		return fmt.Errorf("minLedger (%d) must not exceed maxLedger (%d)", lo, hi)
	}
	return nil
}
