package ingest

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// txMetaEvents is the events-only projection of one meta.
type txMetaEvents struct {
	TransactionEvents [][]byte
	OperationEvents   [][][]byte
}

// metaEventRaws is the version-dispatched per-meta event collection for the
// read path: one xdr.WalkTransactionMeta over the meta, collecting contract
// events, top-level transaction events, and diagnostics with
// version-discriminating group construction. V0/V1/V2 metas return their
// version with empty spines WITHOUT walking the arm (their arms carry no
// events), via an ErrStopWalk early-out. All three sets come from the single
// pass; the caller keeps what it needs.
func metaEventRaws(mv xdr.TransactionMetaView) (int32, txMetaEvents, [][]byte, error) {
	version := int32(-1)
	tev := txMetaEvents{TransactionEvents: [][]byte{}, OperationEvents: [][][]byte{}}
	diag := [][]byte{}
	w := xdr.LedgerCloseMetaWalk{
		MetaVersion: func(_ int, v int32) error {
			version = v
			if v >= 0 && v <= 2 {
				return xdr.ErrStopWalk
			}
			return nil
		},
		V4Ops: func(_, nOps int) error {
			tev.OperationEvents = make([][][]byte, 0, nOps)
			return nil
		},
		OpEventsBegin: func(_, _, count int) error {
			tev.OperationEvents = append(tev.OperationEvents, make([][]byte, 0, count))
			return nil
		},
		OpEvent: func(_, _, _ int, e xdr.ContractEventView) error {
			g := tev.OperationEvents
			g[len(g)-1] = append(g[len(g)-1], e.MustRaw())
			return nil
		},
		TxEvent: func(_, _ int, e xdr.TransactionEventView) error {
			tev.TransactionEvents = append(tev.TransactionEvents, e.MustRaw())
			return nil
		},
		DiagEvent: func(_, _ int, e xdr.DiagnosticEventView) error {
			diag = append(diag, e.MustRaw())
			return nil
		},
	}
	if err := xdr.WalkTransactionMeta(mv, &w); err != nil {
		return 0, txMetaEvents{}, nil, fmt.Errorf("ingest: meta events: %w", err)
	}
	if version < 0 {
		// MetaVersion never fired: the meta discriminant was unreadable.
		return 0, txMetaEvents{}, nil, fmt.Errorf("ingest: meta events: empty meta")
	}
	return version, tev, diag, nil
}
