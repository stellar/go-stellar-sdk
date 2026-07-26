package ingest

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// txMetaEvents is the zero-copy view analog of TransactionEvents, carrying raw
// XDR wire bytes instead of decoded values and WITHOUT DiagnosticEvents
// (collected separately via metaEventRaws' wantDiag arm, because the two sets
// have different gating semantics — see metaEventRaws). Unexported: the
// public shape is ExtractLedgerEvents' LedgerTransactionEvents (which adds the
// tx hash from the same TxProcessing walk).
// Every byte slice ALIASES the source LedgerCloseMetaView buffer (no
// UnmarshalBinary) — callers copy what they retain.
//
//   - TransactionEvents holds the V4 top-level transaction events, each a raw
//     xdr.TransactionEvent. Read Stage / the inner event zero-copy by wrapping
//     an element: xdr.TransactionEventView(raw).Stage() / .Event().
//   - OperationEvents holds, per operation, the raw xdr.ContractEvent bytes.
//     For V3 SorobanMeta there is a single operation group (the soroban tx has
//     one op); for V4 there is one group per operation.
//
// V0/V1/V2 meta carry no contract events, so both fields are empty.
type txMetaEvents struct {
	TransactionEvents [][]byte   // raw xdr.TransactionEvent (V4 top-level)
	OperationEvents   [][][]byte // raw xdr.ContractEvent, per operation
}

// metaEventWalk owns the fused descent over a TransactionMetaView: the hook
// tree wired into the generated SizeFused/FieldsFused/DrainFused primitives,
// plus the per-meta outputs the leaf hooks fill. Because every hook both
// consumes its node AND reports the node's advance, one descent replaces the
// old accessor chain (which re-located every consumed field by re-sizing its
// preceding siblings) — and, on the extract path, the walk's total advance
// doubles as the TxApplyProcessing field size, killing the parent locate's
// blind meta size() walk entirely.
//
// The hook tree is built ONCE per walk lifetime (init) and closes over only
// the receiver, so running the walk per transaction allocates nothing.
// Outputs must be reset() before each meta.
//
// Version support lives in the arms literal in init — a future meta version
// (V5) is added in exactly one place, and until then it fails loudly through
// the Unhandled hook (preserving the historical unsupported-version contract).
type metaEventWalk struct {
	wantEvents bool
	wantDiag   bool

	// Per-meta outputs, reset() per transaction. Empty (not nil) spines
	// deliberately: the parsed reference path (db-layer ParseTransaction
	// lineage) always allocates empty slices, so nil would diverge from it
	// purely on the nil-vs-empty axis. Empty composite literals do not
	// heap-allocate.
	tev  txMetaEvents
	diag [][]byte

	// Pre-bound hook bundles, passed (by value — a handful of func words)
	// into the generated fused walks.
	arms    xdr.TransactionMetaArmHooks
	v3      xdr.TransactionMetaV3FieldsHooks
	v4      xdr.TransactionMetaV4FieldsHooks
	soroban xdr.SorobanTransactionMetaFieldsHooks
	op      xdr.OperationMetaV2FieldsHooks
	// perOp is walkOp pre-bound once so drainOps' per-tx DrainFused call
	// passes a stored func value instead of allocating a method value.
	perOp func(int, xdr.OperationMetaV2View, int) (int, error)
}

// init builds the hook tree. V0/V1/V2 arms stay nil (blind size — they carry
// no events); leaf drains are installed only for the wanted sets, so an
// unwanted set costs exactly the blind size() the old walk paid for it.
func (w *metaEventWalk) init(wantEvents, wantDiag bool) {
	w.wantEvents, w.wantDiag = wantEvents, wantDiag
	w.arms = xdr.TransactionMetaArmHooks{
		V3:        w.armV3,
		V4:        w.armV4,
		Unhandled: metaUnhandled,
	}
	w.v3 = xdr.TransactionMetaV3FieldsHooks{SorobanMeta: w.sizeSorobanOpt}
	if wantEvents {
		w.v4.Operations = w.drainOps
		w.v4.Events = w.drainTxEvents
		w.soroban.Events = w.drainSorobanEvents
		w.op.Events = w.drainOpEvents
	}
	if wantDiag {
		w.v4.DiagnosticEvents = w.drainDiag
		w.soroban.DiagnosticEvents = w.drainSorobanDiag
	}
	w.perOp = w.walkOp
	w.reset()
}

// reset clears the per-meta outputs to their empty-not-nil defaults.
func (w *metaEventWalk) reset() {
	w.tev = txMetaEvents{TransactionEvents: [][]byte{}, OperationEvents: [][][]byte{}}
	w.diag = [][]byte{}
}

// metaUnhandled is the SizeFused Unhandled hook: an unknown TransactionMeta
// version fails loudly, in the same terms the version switch always used.
func metaUnhandled(disc int32) error {
	return fmt.Errorf("ingest: unsupported TransactionMeta V=%d", disc)
}

// advance runs the fused descent over one meta and returns its total wire
// size — this IS the TxApplyProcessing sizer on the extract path.
func (w *metaEventWalk) advance(mv xdr.TransactionMetaView, depth int) (int, error) {
	return mv.SizeFused(w.arms, depth)
}

// armV3 walks the V3 arm: one FieldsFused pass sizes TxChangesBefore /
// Operations / TxChangesAfter and descends into SorobanMeta via its hook.
func (w *metaEventWalk) armV3(v xdr.TransactionMetaV3View, depth int) (int, error) {
	f, err := v.FieldsFused(w.v3, depth)
	if err != nil {
		return 0, err
	}
	return len(f.View), nil
}

// armV4 walks the V4 arm: one FieldsFused pass sizes the skipped fields and
// drains Operations / Events / DiagnosticEvents through their hooks.
func (w *metaEventWalk) armV4(v xdr.TransactionMetaV4View, depth int) (int, error) {
	f, err := v.FieldsFused(w.v4, depth)
	if err != nil {
		return 0, err
	}
	return len(f.View), nil
}

// sizeSorobanOpt unwraps V3's optional SorobanMeta; when present, one
// FieldsFused pass over the inner SorobanTransactionMeta drains the
// events/diagnostics leaves. Absent SorobanMeta leaves the empty defaults in
// place (advance = the 4-byte flag).
func (w *metaEventWalk) sizeSorobanOpt(v xdr.TransactionMetaV3SorobanMetaOptView, depth int) (int, error) {
	sm, present, err := v.Unwrap()
	if err != nil {
		return 0, err
	}
	if !present {
		return 4, nil
	}
	f, err := sm.FieldsFused(w.soroban, depth+1)
	if err != nil {
		return 0, err
	}
	return 4 + len(f.View), nil
}

// drainOps presizes the per-op spine from the validated count (O(1)) and
// drains the V4 Operations array, one fused pass per op.
func (w *metaEventWalk) drainOps(a xdr.TransactionMetaV4OperationsView, depth int) (int, error) {
	count, err := a.Count()
	if err != nil {
		return 0, err
	}
	w.tev.OperationEvents = make([][][]byte, 0, count)
	return a.DrainFused(w.perOp, depth)
}

// walkOp walks one OperationMetaV2: FieldsFused sizes Changes and drains the
// op's Events leaf (which appends this op's group to the spine — exactly once
// per op, in op order, since Events is a mandatory field).
func (w *metaEventWalk) walkOp(_ int, op xdr.OperationMetaV2View, depth int) (int, error) {
	f, err := op.FieldsFused(w.op, depth)
	if err != nil {
		return 0, err
	}
	return len(f.View), nil
}

// rawsAdvance is the wire extent of a drained variable-count array: the
// 4-byte count prefix plus each element's exact (already padded) extent —
// AllRaw's slices are back-to-back, so summing lengths re-derives the advance
// its walk already computed, with no re-sizing.
func rawsAdvance(raws [][]byte) int {
	n := 4
	for _, r := range raws {
		n += len(r)
	}
	return n
}

// The event/diagnostic leaves drain via the generated AllRaw — one size() per
// element, count-presized, empty-not-nil (see the nil-vs-empty note on the
// outputs).

func (w *metaEventWalk) drainTxEvents(a xdr.TransactionMetaV4EventsView, _ int) (int, error) {
	raws, err := a.AllRaw()
	if err != nil {
		return 0, err
	}
	w.tev.TransactionEvents = raws
	return rawsAdvance(raws), nil
}

func (w *metaEventWalk) drainOpEvents(a xdr.OperationMetaV2EventsView, _ int) (int, error) {
	raws, err := a.AllRaw()
	if err != nil {
		return 0, err
	}
	w.tev.OperationEvents = append(w.tev.OperationEvents, raws)
	return rawsAdvance(raws), nil
}

func (w *metaEventWalk) drainSorobanEvents(a xdr.SorobanTransactionMetaEventsView, _ int) (int, error) {
	raws, err := a.AllRaw()
	if err != nil {
		return 0, err
	}
	// V3 has no top-level TransactionEvents; the soroban tx's single op
	// carries the events (one operation group).
	w.tev.OperationEvents = append(w.tev.OperationEvents, raws)
	return rawsAdvance(raws), nil
}

func (w *metaEventWalk) drainDiag(a xdr.TransactionMetaV4DiagnosticEventsView, _ int) (int, error) {
	raws, err := a.AllRaw()
	if err != nil {
		return 0, err
	}
	w.diag = raws
	return rawsAdvance(raws), nil
}

func (w *metaEventWalk) drainSorobanDiag(a xdr.SorobanTransactionMetaDiagnosticEventsView, _ int) (int, error) {
	raws, err := a.AllRaw()
	if err != nil {
		return 0, err
	}
	w.diag = raws
	return rawsAdvance(raws), nil
}

// metaEventRaws is the ONE version-dispatched walk over a TransactionMetaView,
// shared by the extract path (ExtractLedgerEvents' fused TxProcessing drain
// runs the same metaEventWalk hook tree) and the read path's collectTxParts
// (which wants both sets plus the version in a single pass — one fused descent
// instead of a restart-from-zero accessor chain). wantEvents/wantDiag skip the
// collection work a caller doesn't need; the returned version lets the read
// path derive its V3 gate without a third walk.
//
// Keeping a single hook tree here means a future meta version (V5) is added in
// exactly one place — contract events and diagnostics cannot drift apart on
// version support — and an unknown version fails loudly (metaUnhandled).
//
// This standalone form builds the hook tree per call; a per-transaction caller
// (collectTxParts) holds a metaEventWalk and uses the walk method directly so
// the tree is built once per ledger read, not once per tx.
func metaEventRaws(mv xdr.TransactionMetaView, wantEvents, wantDiag bool) (int32, txMetaEvents, [][]byte, error) {
	var w metaEventWalk
	w.init(wantEvents, wantDiag)
	return w.metaEventRaws(mv)
}

// metaEventRaws is the reusable form of the package-level metaEventRaws: the
// version dispatch plus the fused descent, over a hook tree built once by
// init. Outputs are reset per call, so one walk serves every tx of a ledger.
func (w *metaEventWalk) metaEventRaws(mv xdr.TransactionMetaView) (int32, txMetaEvents, [][]byte, error) {
	v, err := mv.V()
	if err != nil {
		return 0, txMetaEvents{}, nil, fmt.Errorf("ingest: meta.V: %w", err)
	}
	switch v {
	case 0, 1, 2:
		// V0 (legacy pre-Soroban, Operations only), V1, V2 carry no events —
		// return the empty defaults without walking the meta at all.
		return v, txMetaEvents{TransactionEvents: [][]byte{}, OperationEvents: [][][]byte{}}, [][]byte{}, nil
	case 3, 4:
		w.reset()
		if _, err := w.advance(mv, 0); err != nil {
			return v, txMetaEvents{}, nil, err
		}
		return v, w.tev, w.diag, nil
	default:
		return v, txMetaEvents{}, nil, fmt.Errorf("ingest: unsupported TransactionMeta V=%d", v)
	}
}
