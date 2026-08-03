package ingest

import (
	"fmt"
	"iter"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// transactionEventsFromMeta walks a TransactionMetaView and returns its
// contract events as raw zero-copy bytes (see TxEvents in extract.go, the
// exported shape EventsFromTxParts returns). It does
// NOT gate V3 SorobanMeta events on whether the transaction is soroban — the
// events-index path relies on the trusted-input invariant (SorobanMeta present
// ⟺ soroban tx), while the read path applies the gate downstream where the
// paired envelope is in hand. Diagnostic events are collected separately
// (metaEventRaws' wantDiag arm, because the two sets have different gating
// semantics).
//
// Supported meta versions:
//
//   - V0/V1/V2: no contract events (V0 is legacy pre-Soroban, Operations only).
//   - V3: SorobanMeta.Events become OperationEvents[0] when SorobanMeta is
//     present; V3 has no top-level TransactionEvents.
//   - V4: top-level Events + per-operation Events.
func transactionEventsFromMeta(metaView xdr.TransactionMetaView) (TxEvents, error) {
	_, tev, _, err := metaEventRaws(metaView, true, false)
	return tev, err
}

// metaEventRaws is the ONE version-dispatched walk over a TransactionMetaView,
// shared by transactionEventsFromMeta (the EventsFromTxParts arm) and the
// read path's collectTxParts (which wants both sets plus the version in a single
// pass — for V3 that means a single SorobanMeta unwrap, which the generated
// accessor locates by sizing every preceding field). wantEvents/wantDiag skip
// the collection work a caller doesn't need; the returned version lets the
// read path derive its V3 gate without a third walk.
//
// Keeping a single switch here means a future meta version (V5) is added in
// exactly one place — contract events and diagnostics cannot drift apart on
// version support.
func metaEventRaws(metaView xdr.TransactionMetaView, wantEvents, wantDiag bool) (int32, TxEvents, [][]byte, error) {
	v, err := metaView.V()
	if err != nil {
		return 0, TxEvents{}, nil, fmt.Errorf("ingest: meta.V: %w", err)
	}
	// Empty (not nil) slices deliberately: the parsed reference path
	// (db-layer ParseTransaction lineage) always allocates empty slices, so
	// returning nil here would diverge from it purely on the nil-vs-empty
	// axis. Empty composite literals do not heap-allocate.
	tev := TxEvents{TransactionEvents: [][]byte{}, OperationEvents: [][][]byte{}}
	diag := [][]byte{}
	// The per-version walkers use Must accessors; one TryVoid per arm recovers a
	// malformed-input *xdr.ViewError into err.
	switch v {
	case 0, 1, 2:
		// V0 (legacy pre-Soroban, Operations only), V1, V2 carry no events.
	case 3:
		err = xdr.TryVoid(func() { v3EventRaws(metaView, wantEvents, wantDiag, &tev, &diag) })
	case 4:
		err = xdr.TryVoid(func() { v4EventRaws(metaView, wantEvents, wantDiag, &tev, &diag) })
	default:
		return v, TxEvents{}, nil, fmt.Errorf("ingest: unsupported TransactionMeta V=%d", v)
	}
	if err != nil {
		return v, TxEvents{}, nil, err
	}
	return v, tev, diag, nil
}

// v3EventRaws fills tev/diag from a V3 meta's SorobanMeta (one unwrap covers
// both sets). Absent SorobanMeta leaves the empty defaults in place. Must-style:
// panics with *xdr.ViewError on malformed input, recovered by metaEventRaws' Try.
func v3EventRaws(metaView xdr.TransactionMetaView, wantEvents, wantDiag bool, tev *TxEvents, diag *[][]byte) {
	sorobanMetaView, present := metaView.MustV3().MustSorobanMeta().MustUnwrap()
	if !present {
		return
	}
	if wantEvents {
		// V3 has no top-level TransactionEvents; the soroban tx's single op
		// carries the events.
		tev.OperationEvents = [][][]byte{collectRaws(sorobanMetaView.MustEvents().MustIter())}
	}
	if wantDiag {
		*diag = collectRaws(sorobanMetaView.MustDiagnosticEvents().MustIter())
	}
}

// v4EventRaws fills tev/diag from a V4 meta (top-level Events + per-op Events,
// top-level DiagnosticEvents). Must-style (see v3EventRaws).
func v4EventRaws(metaView xdr.TransactionMetaView, wantEvents, wantDiag bool, tev *TxEvents, diag *[][]byte) {
	metaV4View := metaView.MustV4()
	if wantEvents {
		tev.TransactionEvents = collectRaws(metaV4View.MustEvents().MustIter())
		opsView := metaV4View.MustOperations()
		opEventRaws := make([][][]byte, 0, opsView.MustCount())
		for opView := range opsView.MustIter() {
			opEventRaws = append(opEventRaws, collectRaws(opView.MustEvents().MustIter()))
		}
		tev.OperationEvents = opEventRaws
	}
	if wantDiag {
		*diag = collectRaws(metaV4View.MustDiagnosticEvents().MustIter())
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
