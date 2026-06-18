package ingest

import (
	"fmt"
	"iter"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// txMetaEvents is the zero-copy view analog of TransactionEvents, carrying raw
// XDR wire bytes instead of decoded values and WITHOUT DiagnosticEvents
// (collected separately via metaEventRaws' wantDiag arm, because the two sets
// have different gating semantics — see transactionEventsFromMeta). Unexported: the
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

// transactionEventsFromMeta walks a TransactionMetaView and returns its
// contract events as raw zero-copy bytes (see txMetaEvents). It does
// NOT gate V3 SorobanMeta events on whether the transaction is soroban — the
// events-index path relies on the trusted-input invariant (SorobanMeta present
// ⟺ soroban tx), while the read path applies the gate downstream where the
// paired envelope is in hand. Diagnostic events are collected separately
// (metaEventRaws' wantDiag arm).
//
// Supported meta versions:
//
//   - V0/V1/V2: no contract events (V0 is legacy pre-Soroban, Operations only).
//   - V3: SorobanMeta.Events become OperationEvents[0] when SorobanMeta is
//     present; V3 has no top-level TransactionEvents.
//   - V4: top-level Events + per-operation Events.
func transactionEventsFromMeta(mv xdr.TransactionMetaView) (txMetaEvents, error) {
	_, tev, _, err := metaEventRaws(mv, true, false)
	return tev, err
}

// metaEventRaws is the ONE version-dispatched walk over a TransactionMetaView,
// shared by transactionEventsFromMeta (the ExtractLedgerEvents arm) and the
// read path's collectTxParts (which wants both sets plus the version in a single
// pass — for V3 that means a single SorobanMeta unwrap, which the generated
// accessor locates by sizing every preceding field). wantEvents/wantDiag skip
// the collection work a caller doesn't need; the returned version lets the
// read path derive its V3 gate without a third walk.
//
// Keeping a single switch here means a future meta version (V5) is added in
// exactly one place — contract events and diagnostics cannot drift apart on
// version support.
func metaEventRaws(mv xdr.TransactionMetaView, wantEvents, wantDiag bool) (int32, txMetaEvents, [][]byte, error) {
	v, err := mv.V()
	if err != nil {
		return 0, txMetaEvents{}, nil, fmt.Errorf("ingest: meta.V: %w", err)
	}
	// Empty (not nil) slices deliberately: the parsed reference path
	// (db-layer ParseTransaction lineage) always allocates empty slices, so
	// returning nil here would diverge from it purely on the nil-vs-empty
	// axis. Empty composite literals do not heap-allocate.
	tev := txMetaEvents{TransactionEvents: [][]byte{}, OperationEvents: [][][]byte{}}
	diag := [][]byte{}
	// The per-version walkers use Must accessors; one TryVoid per arm recovers a
	// malformed-input *xdr.ViewError into err.
	switch v {
	case 0, 1, 2:
		// V0 (legacy pre-Soroban, Operations only), V1, V2 carry no events.
	case 3:
		err = xdr.TryVoid(func() { v3EventRaws(mv, wantEvents, wantDiag, &tev, &diag) })
	case 4:
		err = xdr.TryVoid(func() { v4EventRaws(mv, wantEvents, wantDiag, &tev, &diag) })
	default:
		return v, txMetaEvents{}, nil, fmt.Errorf("ingest: unsupported TransactionMeta V=%d", v)
	}
	if err != nil {
		return v, txMetaEvents{}, nil, err
	}
	return v, tev, diag, nil
}

// v3EventRaws fills tev/diag from a V3 meta's SorobanMeta (one unwrap covers
// both sets). Absent SorobanMeta leaves the empty defaults in place. Must-style:
// panics with *xdr.ViewError on malformed input, recovered by metaEventRaws' Try.
func v3EventRaws(mv xdr.TransactionMetaView, wantEvents, wantDiag bool, tev *txMetaEvents, diag *[][]byte) {
	sm, present := mv.MustV3().MustSorobanMeta().MustUnwrap()
	if !present {
		return
	}
	if wantEvents {
		// V3 has no top-level TransactionEvents; the soroban tx's single op
		// carries the events.
		tev.OperationEvents = [][][]byte{collectRaws(sm.MustEvents().MustIter())}
	}
	if wantDiag {
		*diag = collectRaws(sm.MustDiagnosticEvents().MustIter())
	}
}

// v4EventRaws fills tev/diag from a V4 meta (top-level Events + per-op Events,
// top-level DiagnosticEvents). Must-style (see v3EventRaws).
func v4EventRaws(mv xdr.TransactionMetaView, wantEvents, wantDiag bool, tev *txMetaEvents, diag *[][]byte) {
	v4 := mv.MustV4()
	if wantEvents {
		tev.TransactionEvents = collectRaws(v4.MustEvents().MustIter())
		ops := v4.MustOperations()
		opEventRaws := make([][][]byte, 0, ops.MustCount())
		for op := range ops.MustIter() {
			opEventRaws = append(opEventRaws, collectRaws(op.MustEvents().MustIter()))
		}
		tev.OperationEvents = opEventRaws
	}
	if wantDiag {
		*diag = collectRaws(v4.MustDiagnosticEvents().MustIter())
	}
}

// collectRaws drains a view iterator into the elements' raw wire bytes
// (zero-copy aliases). It returns an EMPTY (not nil) slice on no events — see
// the nil-vs-empty note in metaEventRaws. Must-style: MustIter/MustRaw panic on
// malformed input, recovered by metaEventRaws' Try.
func collectRaws[V interface{ MustRaw() []byte }](it iter.Seq[V]) [][]byte {
	out := [][]byte{}
	for ev := range it {
		out = append(out, ev.MustRaw())
	}
	return out
}
