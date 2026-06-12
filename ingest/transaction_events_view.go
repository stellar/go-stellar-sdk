package ingest

import (
	"fmt"
	"iter"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// txMetaEvents is the zero-copy view analog of TransactionEvents, carrying raw
// XDR wire bytes instead of decoded values and WITHOUT DiagnosticEvents
// (returned separately by diagnosticEventsFromMeta, because the two sets have
// different gating semantics — see transactionEventsFromMeta). Unexported: the
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

// transactionMetaViewVersion returns the TransactionMeta union discriminant
// from a view without decoding the body. The full-history path tolerates V0
// (legacy pre-Soroban meta) which the parsed reference path rejects.
func transactionMetaViewVersion(mv xdr.TransactionMetaView) (int32, error) {
	vv, err := mv.V()
	if err != nil {
		return 0, fmt.Errorf("ingest: meta.V: %w", err)
	}
	v, err := vv.Value()
	if err != nil {
		return 0, fmt.Errorf("ingest: meta.V value: %w", err)
	}
	return v, nil
}

// transactionEventsFromMeta walks a TransactionMetaView and returns its
// contract events as raw zero-copy bytes (see txMetaEvents). It does
// NOT gate V3 SorobanMeta events on whether the transaction is soroban — the
// events-index path relies on the trusted-input invariant (SorobanMeta present
// ⟺ soroban tx), while the read path applies the gate downstream where the
// paired envelope is in hand. Diagnostic events are returned separately by
// diagnosticEventsFromMeta.
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

// diagnosticEventsFromMeta returns the raw xdr.DiagnosticEvent bytes carried by
// a TransactionMetaView (zero-copy, aliasing the view buffer). V3 carries them
// in SorobanMeta.DiagnosticEvents (only when SorobanMeta is present); V4 in the
// top-level DiagnosticEvents; V0/V1/V2 carry none. Diagnostic events are NOT
// gated on IsSorobanTx — the parsed reference path (the standalone
// GetDiagnosticEvents, which db-layer consumers use) returns them regardless.
func diagnosticEventsFromMeta(mv xdr.TransactionMetaView) ([][]byte, error) {
	_, _, diag, err := metaEventRaws(mv, false, true)
	return diag, err
}

// metaEventRaws is the ONE version-dispatched walk over a TransactionMetaView,
// shared by transactionEventsFromMeta, diagnosticEventsFromMeta, and the read
// path's collectTxParts (which wants both sets plus the version in a single
// pass — for V3 that means a single SorobanMeta unwrap, which the generated
// accessor locates by sizing every preceding field). wantEvents/wantDiag skip
// the collection work a caller doesn't need; the returned version lets the
// read path derive its V3 gate without a third walk.
//
// Keeping a single switch here means a future meta version (V5) is added in
// exactly one place — contract events and diagnostics cannot drift apart on
// version support.
func metaEventRaws(mv xdr.TransactionMetaView, wantEvents, wantDiag bool) (int32, txMetaEvents, [][]byte, error) {
	v, err := transactionMetaViewVersion(mv)
	if err != nil {
		return 0, txMetaEvents{}, nil, err
	}
	// Empty (not nil) slices deliberately: the parsed reference path
	// (db-layer ParseTransaction lineage) always allocates empty slices, so
	// returning nil here would diverge from it purely on the nil-vs-empty
	// axis. Empty composite literals do not heap-allocate.
	tev := txMetaEvents{TransactionEvents: [][]byte{}, OperationEvents: [][][]byte{}}
	diag := [][]byte{}
	switch v {
	case 0, 1, 2:
		// V0 (legacy pre-Soroban, Operations only), V1, V2 carry no events.
	case 3:
		if err := v3EventRaws(mv, wantEvents, wantDiag, &tev, &diag); err != nil {
			return v, txMetaEvents{}, nil, err
		}
	case 4:
		if err := v4EventRaws(mv, wantEvents, wantDiag, &tev, &diag); err != nil {
			return v, txMetaEvents{}, nil, err
		}
	default:
		return v, txMetaEvents{}, nil, fmt.Errorf("ingest: unsupported TransactionMeta V=%d", v)
	}
	return v, tev, diag, nil
}

// v3EventRaws fills tev/diag from a V3 meta's SorobanMeta (one unwrap covers
// both sets). Absent SorobanMeta leaves the empty defaults in place.
func v3EventRaws(mv xdr.TransactionMetaView, wantEvents, wantDiag bool, tev *txMetaEvents, diag *[][]byte) error {
	v3, err := mv.V3()
	if err != nil {
		return fmt.Errorf("ingest: meta V3: %w", err)
	}
	smOpt, err := v3.SorobanMeta()
	if err != nil {
		return fmt.Errorf("ingest: SorobanMeta opt: %w", err)
	}
	sm, present, err := smOpt.Unwrap()
	if err != nil {
		return fmt.Errorf("ingest: SorobanMeta unwrap: %w", err)
	}
	if !present {
		return nil
	}
	if wantEvents {
		eventsView, err := sm.Events()
		if err != nil {
			return fmt.Errorf("ingest: V3 SorobanMeta.Events: %w", err)
		}
		contractRaws, err := collectRaws("ContractEvent", eventsView.Iter())
		if err != nil {
			return err
		}
		// V3 has no top-level TransactionEvents; the soroban tx's single op
		// carries the events.
		tev.OperationEvents = [][][]byte{contractRaws}
	}
	if wantDiag {
		diagView, err := sm.DiagnosticEvents()
		if err != nil {
			return fmt.Errorf("ingest: V3 DiagnosticEvents: %w", err)
		}
		if *diag, err = collectRaws("DiagnosticEvent", diagView.Iter()); err != nil {
			return err
		}
	}
	return nil
}

// v4EventRaws fills tev/diag from a V4 meta (top-level Events + per-op Events,
// top-level DiagnosticEvents).
func v4EventRaws(mv xdr.TransactionMetaView, wantEvents, wantDiag bool, tev *txMetaEvents, diag *[][]byte) error {
	v4, err := mv.V4()
	if err != nil {
		return fmt.Errorf("ingest: meta V4: %w", err)
	}
	if wantEvents {
		txEventsView, err := v4.Events()
		if err != nil {
			return fmt.Errorf("ingest: V4 Events: %w", err)
		}
		if tev.TransactionEvents, err = collectRaws("TransactionEvent", txEventsView.Iter()); err != nil {
			return err
		}
		opsView, err := v4.Operations()
		if err != nil {
			return fmt.Errorf("ingest: V4 Operations: %w", err)
		}
		opCount, err := opsView.Count()
		if err != nil {
			return fmt.Errorf("ingest: V4 Operations count: %w", err)
		}
		opEventRaws := make([][][]byte, 0, opCount)
		for op, opErr := range opsView.Iter() {
			if opErr != nil {
				return fmt.Errorf("ingest: V4 op iter: %w", opErr)
			}
			evView, err := op.Events()
			if err != nil {
				return fmt.Errorf("ingest: V4 op.Events: %w", err)
			}
			evRaws, err := collectRaws("ContractEvent", evView.Iter())
			if err != nil {
				return err
			}
			opEventRaws = append(opEventRaws, evRaws)
		}
		tev.OperationEvents = opEventRaws
	}
	if wantDiag {
		diagView, err := v4.DiagnosticEvents()
		if err != nil {
			return fmt.Errorf("ingest: V4 DiagnosticEvents: %w", err)
		}
		if *diag, err = collectRaws("DiagnosticEvent", diagView.Iter()); err != nil {
			return err
		}
	}
	return nil
}

// collectRaws drains a view iterator into the elements' raw wire bytes
// (zero-copy aliases). It returns an EMPTY (not nil) slice on no events — see
// the nil-vs-empty note in metaEventRaws. kind only labels errors.
func collectRaws[V interface{ Raw() ([]byte, error) }](kind string, it iter.Seq2[V, error]) ([][]byte, error) {
	out := [][]byte{}
	for ev, err := range it {
		if err != nil {
			return nil, fmt.Errorf("ingest: %s iter: %w", kind, err)
		}
		raw, err := ev.Raw()
		if err != nil {
			return nil, fmt.Errorf("ingest: %s.Raw: %w", kind, err)
		}
		out = append(out, raw)
	}
	return out, nil
}
