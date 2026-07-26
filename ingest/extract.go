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
//     an element: xdr.ParseTransactionEventView(raw).
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
// ledger, in apply order, each paired with its transaction hash. The loop IS
// the single pass: each TxProcessing element's bundle locates Result and
// TxApplyProcessing in wire order, the hashes are read from the result pair,
// the meta's event leaves are drained by nested loops in wire order, and the
// iterator's advance past the element resumes from the walk record that
// consumption left behind. For a fee-bump transaction, InnerHash carries the
// inner transaction's hash.
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
	// Presized from the dispatch's validated TxProcessing count; kept nil for
	// an empty ledger (the historical contract — ExtractLedgerEvents has
	// always returned nil, not empty, when there are no transactions).
	var out []LedgerTransactionEvents
	if d.txCount > 0 {
		out = make([]LedgerTransactionEvents, 0, d.txCount)
	}
	for parts, iterErr := range d.TxProcessing() {
		if iterErr != nil {
			return nil, fmt.Errorf("ingest: TxProcessing iter: %w", iterErr)
		}
		h, inner, feeBump, err := txProcessingHashes(parts)
		if err != nil {
			return nil, err
		}
		_, tev, _, err := metaEventRaws(parts.TxApplyProcessing, true, false)
		if err != nil {
			return nil, err
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
