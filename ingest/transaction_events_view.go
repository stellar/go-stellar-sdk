package ingest

import (
	"fmt"
	"iter"

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
//     xdr.TransactionEvent.
//   - OperationEvents holds, per operation, the raw xdr.ContractEvent bytes.
//     For V3 SorobanMeta there is a single operation group (the soroban tx has
//     one op); for V4 there is one group per operation.
//
// V0/V1/V2 meta carry no contract events, so both fields are empty.
type txMetaEvents struct {
	TransactionEvents [][]byte   // raw xdr.TransactionEvent (V4 top-level)
	OperationEvents   [][][]byte // raw xdr.ContractEvent, per operation
}

// The three raw-event collectors below are the one loop "drain an event array
// into per-element raw slices", written once per element view type (concrete
// bodies — no generics or interfaces on the traversal path, mirroring the
// generated iterators). Each element's Raw() records the element's span into
// the walk, which is exactly what makes the iterator's own advance past it
// O(1): one traversal per element in total. Spines are presized from the
// validated count and empty-not-nil (the parsed reference path always
// allocates empty slices; nil would diverge purely on the nil-vs-empty axis).

func collectContractEventRaws(n uint32, all iter.Seq2[xdr.ContractEventView, error]) ([][]byte, error) {
	out := make([][]byte, 0, n)
	for ev, err := range all {
		if err != nil {
			return nil, err
		}
		raw, err := ev.Raw()
		if err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, nil
}

func collectTransactionEventRaws(n uint32, all iter.Seq2[xdr.TransactionEventView, error]) ([][]byte, error) {
	out := make([][]byte, 0, n)
	for ev, err := range all {
		if err != nil {
			return nil, err
		}
		raw, err := ev.Raw()
		if err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, nil
}

func collectDiagnosticEventRaws(n uint32, all iter.Seq2[xdr.DiagnosticEventView, error]) ([][]byte, error) {
	out := make([][]byte, 0, n)
	for ev, err := range all {
		if err != nil {
			return nil, err
		}
		raw, err := ev.Raw()
		if err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, nil
}

// metaV3EventRaws walks a V3 arm in wire order: the bundle resolves through
// the optional SorobanMeta (sizing TxChangesBefore / Operations /
// TxChangesAfter on the way — the one skip pass XDR layout demands), then the
// wanted leaves are drained. V3 has no top-level TransactionEvents; the
// soroban tx's single op carries the events (one operation group).
func metaV3EventRaws(v3 xdr.TransactionMetaV3View, wantEvents, wantDiag bool, tev *txMetaEvents, diag *[][]byte) error {
	f, err := v3.Fields()
	if err != nil {
		return err
	}
	smOpt, err := f.SorobanMeta()
	if err != nil {
		return err
	}
	sm, present, err := smOpt.Unwrap()
	if err != nil {
		return err
	}
	if !present {
		// Nothing to drain — but the bundle just sized the whole arm to locate
		// SorobanMeta. Pin that as the arm's extent (Raw resumes from the
		// bundle's frontier entry, so this only sizes the 4-byte absent flag):
		// the walk record it leaves makes the enclosing element's advance O(1)
		// instead of a blind re-walk of the meta. On a classic-heavy ledger
		// this is the dominant per-tx shape.
		if _, err := v3.Raw(); err != nil {
			return err
		}
		return nil
	}
	smf, err := sm.Fields()
	if err != nil {
		return err
	}
	if wantEvents {
		evs, err := smf.Events()
		if err != nil {
			return err
		}
		n, err := evs.Len()
		if err != nil {
			return err
		}
		raws, err := collectContractEventRaws(n, evs.All())
		if err != nil {
			return err
		}
		tev.OperationEvents = append(tev.OperationEvents, raws)
	}
	if wantDiag {
		des, err := smf.DiagnosticEvents()
		if err != nil {
			return err
		}
		n, err := des.Len()
		if err != nil {
			return err
		}
		if *diag, err = collectDiagnosticEventRaws(n, des.All()); err != nil {
			return err
		}
	}
	return nil
}

// metaV4EventRaws walks a V4 arm in wire order: Operations (per-op Events
// leaf), then the top-level Events, then DiagnosticEvents — each field's
// offset resolved by the bundle from the previous field's consumption record,
// so the whole collection is one pass over the meta's wanted extent.
func metaV4EventRaws(v4 xdr.TransactionMetaV4View, wantEvents, wantDiag bool, tev *txMetaEvents, diag *[][]byte) error {
	f, err := v4.Fields()
	if err != nil {
		return err
	}
	if wantEvents {
		ops, err := f.Operations()
		if err != nil {
			return err
		}
		nOps, err := ops.Len()
		if err != nil {
			return err
		}
		tev.OperationEvents = make([][][]byte, 0, nOps)
		for op, err := range ops.All() {
			if err != nil {
				return err
			}
			opf, err := op.Fields()
			if err != nil {
				return err
			}
			evs, err := opf.Events()
			if err != nil {
				return err
			}
			n, err := evs.Len()
			if err != nil {
				return err
			}
			raws, err := collectContractEventRaws(n, evs.All())
			if err != nil {
				return err
			}
			tev.OperationEvents = append(tev.OperationEvents, raws)
		}
		evs, err := f.Events()
		if err != nil {
			return err
		}
		n, err := evs.Len()
		if err != nil {
			return err
		}
		if tev.TransactionEvents, err = collectTransactionEventRaws(n, evs.All()); err != nil {
			return err
		}
	}
	if wantDiag {
		des, err := f.DiagnosticEvents()
		if err != nil {
			return err
		}
		n, err := des.Len()
		if err != nil {
			return err
		}
		if *diag, err = collectDiagnosticEventRaws(n, des.All()); err != nil {
			return err
		}
	}
	return nil
}

// metaEventRaws is the ONE version-dispatched walk over a TransactionMetaView,
// shared by the extract path (ExtractLedgerEvents) and the read path's
// collectTxParts (which wants both sets plus the version in a single pass).
// wantEvents/wantDiag skip the collection work a caller doesn't need; the
// returned version lets the read path derive its V3 gate without a third walk.
//
// Keeping a single dispatch here means a future meta version (V5) is added in
// exactly one place — contract events and diagnostics cannot drift apart on
// version support — and an unknown version fails loudly.
func metaEventRaws(mv xdr.TransactionMetaView, wantEvents, wantDiag bool) (int32, txMetaEvents, [][]byte, error) {
	v, err := mv.V()
	if err != nil {
		return 0, txMetaEvents{}, nil, fmt.Errorf("ingest: meta.V: %w", err)
	}
	// Empty (not nil) spines deliberately: the parsed reference path
	// (db-layer ParseTransaction lineage) always allocates empty slices, so
	// nil would diverge from it purely on the nil-vs-empty axis. Empty
	// composite literals do not heap-allocate.
	tev := txMetaEvents{TransactionEvents: [][]byte{}, OperationEvents: [][][]byte{}}
	diag := [][]byte{}
	switch v {
	case 0, 1, 2:
		// V0 (legacy pre-Soroban, Operations only), V1, V2 carry no events —
		// return the empty defaults without walking the meta at all.
		return v, tev, diag, nil
	case 3:
		v3, err := mv.ArmV3()
		if err != nil {
			return v, txMetaEvents{}, nil, err
		}
		if err := metaV3EventRaws(v3, wantEvents, wantDiag, &tev, &diag); err != nil {
			return v, txMetaEvents{}, nil, err
		}
		return v, tev, diag, nil
	case 4:
		v4, err := mv.ArmV4()
		if err != nil {
			return v, txMetaEvents{}, nil, err
		}
		if err := metaV4EventRaws(v4, wantEvents, wantDiag, &tev, &diag); err != nil {
			return v, txMetaEvents{}, nil, err
		}
		return v, tev, diag, nil
	default:
		return v, txMetaEvents{}, nil, fmt.Errorf("ingest: unsupported TransactionMeta V=%d", v)
	}
}
