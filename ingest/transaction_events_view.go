package ingest

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// txMetaEvents is the events-only projection of one meta (historical internal
// shape; the collection itself is xdr.ExtractTransactionMetaEvents).
type txMetaEvents struct {
	TransactionEvents [][]byte
	OperationEvents   [][][]byte
}

// metaEventRaws is the version-dispatched per-meta event collection, shared
// by the read path: a thin delegation to xdr.ExtractTransactionMetaEvents
// (one generated Walk over the meta). wantEvents/wantDiag only trim the
// returned sets (the walk collects both in its single pass); an unknown
// version fails loudly inside the extractor.
func metaEventRaws(mv xdr.TransactionMetaView, wantEvents, wantDiag bool) (int32, txMetaEvents, [][]byte, error) {
	me, err := xdr.ExtractTransactionMetaEvents(mv)
	if err != nil {
		return 0, txMetaEvents{}, nil, fmt.Errorf("ingest: meta events: %w", err)
	}
	tev := txMetaEvents{TransactionEvents: [][]byte{}, OperationEvents: [][][]byte{}}
	diag := [][]byte{}
	if wantEvents {
		tev.TransactionEvents = me.TransactionEvents
		tev.OperationEvents = me.OperationEvents
	}
	if wantDiag {
		diag = me.DiagnosticEvents
	}
	return me.V, tev, diag, nil
}
