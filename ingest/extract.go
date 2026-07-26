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
// ledger, in apply order, each paired with its transaction hash — hash, events,
// and iterator advance all come from ONE fused TxProcessing descent. Each
// element is walked exactly once: FieldsFused locates the element's fields
// while the TxApplyProcessing hook (the fused metaEventWalk) drains the event
// leaves AND reports the meta's size, so the walk never blind-sizes a meta it
// is about to re-walk. For a fee-bump transaction, InnerHash carries the inner
// transaction's hash.
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
	st := newLedgerEventsExtract(d.txCount)
	if err := d.drainTP(st.perTx, st.perTxV1); err != nil {
		return nil, err
	}
	return st.out, nil
}

// ledgerEventsExtract is ExtractLedgerEvents' per-call state: the fused meta
// walk plus the per-element TxProcessing callbacks for both element types
// (V0/V1's TransactionResultMeta and V2's TransactionResultMetaV1), all built
// ONCE per extract so the per-element path allocates no closures. The only
// hook installed on the element bundles is TxApplyProcessing = the fused meta
// walk; the remaining fields (Result, FeeProcessing, and — on the V2 element —
// the trailing PostTxApplyFeeProcessing) keep their default size() walks,
// which the element advance (len of the located View) therefore includes.
type ledgerEventsExtract struct {
	metaEventWalk
	out []LedgerTransactionEvents

	rm      xdr.TransactionResultMetaFieldsHooks
	rmV1    xdr.TransactionResultMetaV1FieldsHooks
	perTx   func(int, xdr.TransactionResultMetaView, int) (int, error)
	perTxV1 func(int, xdr.TransactionResultMetaV1View, int) (int, error)
}

func newLedgerEventsExtract(txCount int) *ledgerEventsExtract {
	st := &ledgerEventsExtract{}
	st.metaEventWalk.init(true, false)
	st.rm = xdr.TransactionResultMetaFieldsHooks{TxApplyProcessing: st.advance}
	st.rmV1 = xdr.TransactionResultMetaV1FieldsHooks{TxApplyProcessing: st.advance}
	st.perTx = st.onTx
	st.perTxV1 = st.onTxV1
	// Presized from the dispatch's validated TxProcessing count; kept nil for
	// an empty ledger (the historical contract — ExtractLedgerEvents has
	// always returned nil, not empty, when there are no transactions).
	if txCount > 0 {
		st.out = make([]LedgerTransactionEvents, 0, txCount)
	}
	return st
}

// onTx is the DrainFused per-element callback for a V0/V1 TxProcessing array.
func (st *ledgerEventsExtract) onTx(i int, elem xdr.TransactionResultMetaView, depth int) (int, error) {
	st.reset()
	f, err := elem.FieldsFused(st.rm, depth)
	if err != nil {
		return 0, fmt.Errorf("ingest: TxProcessing element %d: %w", i, err)
	}
	return len(f.View), st.emit(txResultParts{Result: f.Result, TxApplyProcessing: f.TxApplyProcessing})
}

// onTxV1 is onTx for a V2 TxProcessing array (TransactionResultMetaV1
// element — five fields, whose trailing PostTxApplyFeeProcessing the located
// View, and therefore the advance, includes).
func (st *ledgerEventsExtract) onTxV1(i int, elem xdr.TransactionResultMetaV1View, depth int) (int, error) {
	st.reset()
	f, err := elem.FieldsFused(st.rmV1, depth)
	if err != nil {
		return 0, fmt.Errorf("ingest: TxProcessing element %d: %w", i, err)
	}
	return len(f.View), st.emit(txResultParts{Result: f.Result, TxApplyProcessing: f.TxApplyProcessing})
}

// emit reads the element's hashes and appends the transaction's entry from the
// walk outputs the fused element descent just filled.
func (st *ledgerEventsExtract) emit(parts txResultParts) error {
	h, inner, feeBump, err := txProcessingHashes(parts)
	if err != nil {
		return err
	}
	st.out = append(st.out, LedgerTransactionEvents{
		Hash:              [32]byte(h),
		InnerHash:         [32]byte(inner),
		FeeBump:           feeBump,
		TransactionEvents: st.tev.TransactionEvents,
		OperationEvents:   st.tev.OperationEvents,
	})
	return nil
}
