package ingest

// Stage-3 gates for the generated WalkLedgerCloseMeta beyond the spike
// differential: ordering invariant, fired-set vs mask reachability over the
// corpus (incl. randxdr shapes), the T-1 position manifest coverage tripwire,
// and ErrStopWalk semantics.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// fullSubscription returns a walk subscribing every position, recording the
// fired-position set and asserting the ordering invariant as it goes: wire
// order — nondecreasing txIdx; within a tx, groups in opIdx order with events
// in evIdx order; Begin before its elements.
func fullSubscription(t *testing.T, fired *uint64) *xdr.LedgerCloseMetaWalk {
	t.Helper()
	lastTx, lastOp, lastEv, lastTxEv, lastDiag := -1, -1, -1, -1, -1
	opBegun, txEvBegun, diagBegun := false, false, false
	touch := func(pos uint8) { *fired |= 1 << pos }
	enterTx := func(tx int) {
		require.GreaterOrEqual(t, tx, lastTx, "txIdx must be nondecreasing")
		if tx != lastTx {
			lastTx, lastOp, lastEv, lastTxEv, lastDiag = tx, -1, -1, -1, -1
			opBegun, txEvBegun, diagBegun = false, false, false
		}
	}
	return &xdr.LedgerCloseMetaWalk{
		TxProcessingBegin: func(count int) error {
			require.Equal(t, -1, lastTx, "TxProcessingBegin must fire before any tx")
			require.GreaterOrEqual(t, count, 0)
			touch(xdr.LedgerCloseMetaPosTxProcessingBegin)
			return nil
		},
		ResultPair: func(tx int, pair xdr.TransactionResultPairView) error {
			enterTx(tx)
			touch(xdr.LedgerCloseMetaPosResultPair)
			return nil
		},
		MetaVersion: func(tx int, v int32) error {
			enterTx(tx)
			touch(xdr.LedgerCloseMetaPosMetaVersion)
			return nil
		},
		V4Ops: func(tx, nOps int) error {
			enterTx(tx)
			touch(xdr.LedgerCloseMetaPosV4Ops)
			return nil
		},
		OpEventsBegin: func(tx, op, count int) error {
			enterTx(tx)
			require.Greater(t, op, lastOp, "OpEventsBegin must advance opIdx")
			lastOp, lastEv, opBegun = op, -1, true
			touch(xdr.LedgerCloseMetaPosOpEventsBegin)
			return nil
		},
		OpEvent: func(tx, op, ev int, e xdr.ContractEventView) error {
			enterTx(tx)
			require.True(t, opBegun, "OpEvent requires a preceding OpEventsBegin")
			require.Equal(t, lastOp, op, "OpEvent must stay within the begun group")
			require.Equal(t, lastEv+1, ev, "evIdx must be dense and ordered")
			lastEv = ev
			touch(xdr.LedgerCloseMetaPosOpEvent)
			return nil
		},
		TxEventsBegin: func(tx, count int) error {
			enterTx(tx)
			txEvBegun = true
			touch(xdr.LedgerCloseMetaPosTxEventsBegin)
			return nil
		},
		TxEvent: func(tx, ev int, e xdr.TransactionEventView) error {
			enterTx(tx)
			require.True(t, txEvBegun, "TxEvent requires a preceding TxEventsBegin")
			require.Equal(t, lastTxEv+1, ev, "tx-event evIdx must be dense and ordered")
			lastTxEv = ev
			touch(xdr.LedgerCloseMetaPosTxEvent)
			return nil
		},
		DiagEventsBegin: func(tx, count int) error {
			enterTx(tx)
			diagBegun = true
			touch(xdr.LedgerCloseMetaPosDiagEventsBegin)
			return nil
		},
		DiagEvent: func(tx, ev int, e xdr.DiagnosticEventView) error {
			enterTx(tx)
			require.True(t, diagBegun, "DiagEvent requires a preceding DiagEventsBegin")
			require.Equal(t, lastDiag+1, ev, "diag evIdx must be dense and ordered")
			lastDiag = ev
			touch(xdr.LedgerCloseMetaPosDiagEvent)
			return nil
		},
		TxMeta: func(tx int, meta xdr.TransactionMetaView) error {
			enterTx(tx)
			r, err := meta.Raw()
			require.NoError(t, err, "TxMeta views are exact-extent")
			require.NotEmpty(t, r)
			touch(xdr.LedgerCloseMetaPosTxMeta)
			return nil
		},
	}
}

// TestWalkLCM_OrderingAndMaskReachability: on every corpus fixture (crafted +
// randxdr), a full subscription must fire positions in wire order, and the
// fired set must be a subset of the subscription mask; across the WHOLE
// corpus the union of fired positions must equal every enumerated position
// (the fired-set == mask-reachability gate).
func TestWalkLCM_OrderingAndMaskReachability(t *testing.T) {
	var union uint64
	all := uint64(1)<<len(xdr.LedgerCloseMetaWalkPositions) - 1
	for _, fx := range differentialCorpus(t) {
		var fired uint64
		w := fullSubscription(t, &fired)
		if err := xdr.WalkLedgerCloseMeta(xdr.ParseLedgerCloseMetaView(fx.raw), w); err != nil {
			continue // malformed randxdr shapes may abort; ordering held up to the stop
		}
		require.Zero(t, fired&^all, "%s: fired positions outside the manifest", fx.name)
		union |= fired
	}
	require.Equal(t, all, union,
		"differential corpus must exercise every generated walk position (T-1 manifest tripwire)")
}

// TestWalkLCM_ManifestMatchesStruct pins the manifest against the callback
// struct: every enumerated position name is a field of LedgerCloseMetaWalk.
func TestWalkLCM_ManifestMatchesStruct(t *testing.T) {
	require.Equal(t, []string{
		"TxProcessingBegin", "ResultPair", "MetaVersion", "V4Ops",
		"OpEventsBegin", "OpEvent", "TxEventsBegin", "TxEvent",
		"DiagEventsBegin", "DiagEvent", "TxMeta",
	}, xdr.LedgerCloseMetaWalkPositions)
}

// TestWalkLCM_ErrStopWalk: a callback returning ErrStopWalk stops the walk
// cleanly (nil), delivering nothing further; any other error aborts verbatim.
func TestWalkLCM_ErrStopWalk(t *testing.T) {
	fx := differentialCorpus(t)[0] // lcmV0/meta-matrix: multiple txs
	seen := 0
	w := &xdr.LedgerCloseMetaWalk{
		ResultPair: func(tx int, _ xdr.TransactionResultPairView) error {
			seen++
			if tx == 0 {
				return xdr.ErrStopWalk
			}
			return nil
		},
	}
	require.NoError(t, xdr.WalkLedgerCloseMeta(xdr.ParseLedgerCloseMetaView(fx.raw), w))
	require.Equal(t, 1, seen, "ErrStopWalk must stop after the first delivery")

	boom := &xdr.ViewError{Kind: xdr.ViewErrShortBuffer, Detail: "sentinel"}
	w2 := &xdr.LedgerCloseMetaWalk{
		ResultPair: func(int, xdr.TransactionResultPairView) error { return boom },
	}
	err := xdr.WalkLedgerCloseMeta(xdr.ParseLedgerCloseMetaView(fx.raw), w2)
	require.ErrorIs(t, err, boom, "non-stop errors must abort verbatim")
}
