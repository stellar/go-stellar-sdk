package ingest

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// ExtractTxHashes returns every transaction hash of the ledger in apply
// (TxProcessing) order, read straight from each TransactionResultPair without
// decoding anything else — the cheapest possible per-ledger hash listing
// (e.g. for building a tx-hash → ledger index).
//
// Experimental: the view-based extractors are new in this release and their
// signatures may still change.
func ExtractTxHashes(lcm xdr.LedgerCloseMetaView) ([]xdr.Hash, error) {
	d, err := dispatchLCMView(lcm)
	if err != nil {
		return nil, err
	}
	var out []xdr.Hash
	for parts, iterErr := range d.TxProcessing() {
		if iterErr != nil {
			return nil, fmt.Errorf("ingest: TxProcessing iter: %w", iterErr)
		}
		h, herr := txProcessingHash(parts)
		if herr != nil {
			return nil, herr
		}
		out = append(out, h)
	}
	return out, nil
}

// LedgerTransactionEvents is one transaction's contract events plus its hash,
// in the flat raw-bytes shape an events indexer consumes. Every byte slice
// ALIASES the source LedgerCloseMetaView buffer (zero-copy); callers copy what
// they retain.
//
//   - TransactionEvents holds the V4 top-level transaction events, each a raw
//     xdr.TransactionEvent. Read Stage / the inner event zero-copy by wrapping
//     an element: xdr.TransactionEventView(raw).Stage() / .Event().
//   - OperationEvents holds, per operation, the raw xdr.ContractEvent bytes.
//     For V3 SorobanMeta there is a single operation group (the soroban tx has
//     one op); for V4 there is one group per operation.
//
// V0/V1/V2 meta carry no contract events, so both event fields are empty.
type LedgerTransactionEvents struct {
	Hash              [32]byte
	InnerHash         [32]byte   // the inner transaction's hash; meaningful iff FeeBump
	FeeBump           bool       // the transaction is a fee-bump
	TransactionEvents [][]byte   // raw xdr.TransactionEvent (V4 top-level)
	OperationEvents   [][][]byte // raw xdr.ContractEvent, per operation
}

// ExtractLedgerEvents returns the contract events of every transaction in the
// ledger, in apply order, each paired with its transaction hash — hash and
// events come from ONE TxProcessing walk (sizing each element to advance the
// iterator is the dominant cost, so a separate hash pass would nearly double
// it). For a fee-bump transaction, InnerHash carries the inner transaction's
// hash.
//
// It does NOT gate V3 SorobanMeta events on whether the transaction is soroban
// — an events-index consumer relies on the trusted-input invariant
// (SorobanMeta present ⟺ soroban tx); the transaction read path
// (LedgerTransactionViewByHash / LedgerTransactionViewRange) applies that gate
// where the paired envelope is in hand, matching the parsed
// GetTransactionEvents. Diagnostic events are not included — they are a
// read-path concern, available per transaction via
// LedgerTransactionView.DiagnosticEvents.
//
// Experimental: the view-based extractors are new in this release and their
// signatures may still change.
func ExtractLedgerEvents(lcm xdr.LedgerCloseMetaView) ([]LedgerTransactionEvents, error) {
	d, err := dispatchLCMView(lcm)
	if err != nil {
		return nil, err
	}
	var out []LedgerTransactionEvents
	for parts, iterErr := range d.TxProcessing() {
		if iterErr != nil {
			return nil, fmt.Errorf("ingest: TxProcessing iter: %w", iterErr)
		}
		h, inner, feeBump, herr := txProcessingHashes(parts)
		if herr != nil {
			return nil, herr
		}
		tev, terr := transactionEventsFromMeta(parts.TxApplyProcessing)
		if terr != nil {
			return nil, terr
		}
		out = append(out, LedgerTransactionEvents{
			Hash:              [32]byte(h),
			InnerHash:         [32]byte(inner),
			FeeBump:           feeBump,
			TransactionEvents: tev.TransactionEvents,
			OperationEvents:   tev.OperationEvents,
		})
	}
	return out, nil
}
