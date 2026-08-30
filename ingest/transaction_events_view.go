package ingest

import (
	"fmt"

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
func metaEventRaws(metaView xdr.TransactionMetaView, wantEvents, wantDiag bool) (int32, TxEvents, []xdr.DiagnosticEventView, error) {
	v, err := metaView.V()
	if err != nil {
		return 0, TxEvents{}, nil, fmt.Errorf("ingest: meta.V: %w", err)
	}
	// Empty (not nil) slices deliberately: the parsed reference path
	// (db-layer ParseTransaction lineage) always allocates empty slices, so
	// returning nil here would diverge from it purely on the nil-vs-empty
	// axis. Empty composite literals do not heap-allocate.
	tev := TxEvents{TransactionEvents: []xdr.TransactionEventView{}, OperationEvents: [][]xdr.ContractEventView{}}
	diag := []xdr.DiagnosticEventView{}
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
func v3EventRaws(metaView xdr.TransactionMetaView, wantEvents, wantDiag bool, tev *TxEvents, diag *[]xdr.DiagnosticEventView) {
	sorobanMetaView, present := metaView.MustV3().MustSorobanMeta().MustUnwrap()
	if !present {
		return
	}
	// V3 has no top-level TransactionEvents; the soroban tx's single op
	// carries the contract events (tev.OperationEvents[0]).
	//
	// With BOTH event sets wanted (the read path), locate every SorobanMeta
	// field in one pass — asking Events and then DiagnosticEvents through
	// the single-field accessors would size Ext and Events twice. With one
	// set wanted, the single accessor sizes strictly less than a full
	// locate, so it stays.
	switch {
	case wantEvents && wantDiag:
		f := mustFields(sorobanMetaView.Fields())
		tev.OperationEvents = [][]xdr.ContractEventView{f.Events.MustAll()}
		*diag = f.DiagnosticEvents.MustAll()
	case wantEvents:
		tev.OperationEvents = [][]xdr.ContractEventView{sorobanMetaView.MustEvents().MustAll()}
	case wantDiag:
		*diag = sorobanMetaView.MustDiagnosticEvents().MustAll()
	}
}

// v4EventRaws fills tev/diag from a V4 meta (top-level Events + per-op Events,
// top-level DiagnosticEvents). Must-style (see v3EventRaws).
func v4EventRaws(metaView xdr.TransactionMetaView, wantEvents, wantDiag bool, tev *TxEvents, diag *[]xdr.DiagnosticEventView) {
	// Locate every V4 meta field in ONE pass, whichever sets are wanted:
	// Events and Operations sit deep in the struct, so even the events-only
	// product path pays overlapping prefix walks through the single-field
	// accessors — and DiagnosticEvents is the LAST field, so on the read
	// path its accessor would re-walk the entire meta interior (every
	// operation included) a second time for every transaction.
	f := mustFields(metaView.MustV4().Fields())
	if wantEvents {
		tev.TransactionEvents = f.Events.MustAll()
		tev.OperationEvents = opEventRaws(f.Operations)
	}
	if wantDiag {
		*diag = f.DiagnosticEvents.MustAll()
	}
}

// mustFields is the missing Must twin of the generated Fields() accessors: it
// panics with the returned *xdr.ViewError, which the callers' TryVoid
// recovers into an ordinary error — exactly how the generated Must accessors
// behave.
func mustFields[T any](f T, err error) T {
	if err != nil {
		panic(err)
	}
	return f
}

// opEventRaws walks a V4 operations array ONCE, collecting each operation's
// contract events. Must-style: the Must accessors panic with *xdr.ViewError,
// recovered by metaEventRaws' TryVoid.
//
// It steps the operations array by hand rather than using the generated
// Iter()/All(). Those size each operation's WHOLE interior (Ext + Changes +
// Events) just to advance or trim, and then MustEvents() re-walks Ext +
// Changes to locate the events and the collection re-walks Events — about
// double the work. Because Events is the LAST OperationMetaV2 field, the
// events offset (one locate) plus the events array's own extent is exactly
// the operation's size, so each operation's interior is sized once.
//
// Note this is about the OUTER array only: the inner events array uses the
// generated MustAll(), which already sizes each element exactly once.
func opEventRaws(ops xdr.TransactionMetaV4OperationsView) [][]xdr.ContractEventView {
	n := ops.MustCount()
	out := make([][]xdr.ContractEventView, 0, n)
	if n == 0 {
		return out
	}
	op := ops.MustAt(0)
	for k := 0; k < n; k++ {
		evs := op.MustEvents()
		events := evs.MustAll()
		out = append(out, events)
		// The events array's wire extent = count prefix + every element.
		// MustAll already sized each element exactly once in order to trim it,
		// so summing the trimmed lengths adds no sizing pass. MustRaw would
		// size the whole array again, so it is used ONLY on the empty arm
		// (where it is the O(1) prefix read).
		var evExtent int
		if len(events) == 0 {
			evExtent = len(evs.MustRaw())
		} else {
			// Prefix width from At(0)'s offset rather than a hardcoded 4.
			evExtent = len(evs) - len(evs.MustAt(0))
			for i := range events {
				evExtent += len(events[i])
			}
		}
		// len(op)-len(evs) is the events array's offset within the op (the
		// accessor returns a suffix of the same underlying buffer).
		op = op[(len(op)-len(evs))+evExtent:]
	}
	return out
}
