package ingest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// These tests pin the SHAPE of the meta event walk, not just its output: that
// a V4 meta's operations array is stepped once, landing on each operation's
// true wire offset, and that every collected slice is presized to its exact
// element count. The fixture builders (vContractEvent, vDiagEvent,
// vMetaV3Soroban, buildEventsLCM, …) live alongside the other view tests in
// this package.

// walkOpChanges gives an operation a non-trivial spine: an operation whose
// interior is nothing but its events array would hide a stepping bug, since
// the walk's arithmetic would coincide with the events offset. Two removed
// ledger keys sit between Ext and Events, so a step that is off by any amount
// lands inside them and the offsets asserted below diverge.
func walkOpChanges(seed byte) xdr.LedgerEntryChanges {
	key := func(k byte) xdr.LedgerKey {
		var ed xdr.Uint256
		ed[0], ed[31] = seed, k
		return xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeAccount,
			Account: &xdr.LedgerKeyAccount{AccountId: xdr.AccountId{
				Type: xdr.PublicKeyTypePublicKeyTypeEd25519, Ed25519: &ed,
			}},
		}
	}
	removed0, removed1 := key(0), key(1)
	return xdr.LedgerEntryChanges{
		{Type: xdr.LedgerEntryChangeTypeLedgerEntryRemoved, Removed: &removed0},
		{Type: xdr.LedgerEntryChangeTypeLedgerEntryRemoved, Removed: &removed1},
	}
}

// walkMetaV4 builds a V4 meta with one operation per entry of opEventCounts
// (that many contract events each, plus changes), txEvents top-level
// transaction events and diags diagnostic events.
func walkMetaV4(opEventCounts []int, txEvents, diags int) xdr.TransactionMeta {
	ops := make([]xdr.OperationMetaV2, len(opEventCounts))
	for i, n := range opEventCounts {
		evs := make([]xdr.ContractEvent, n)
		for j := range evs {
			evs[j] = vContractEvent(fmt.Sprintf("op%d-ev%d", i, j))
		}
		ops[i] = xdr.OperationMetaV2{Changes: walkOpChanges(byte(i + 1)), Events: evs}
	}
	stages := make([]xdr.TransactionEvent, txEvents)
	for i := range stages {
		stages[i] = xdr.TransactionEvent{
			Stage: xdr.TransactionEventStageTransactionEventStageAfterTx,
			Event: vContractEvent(fmt.Sprintf("tx-ev%d", i)),
		}
	}
	diagnostics := make([]xdr.DiagnosticEvent, diags)
	for i := range diagnostics {
		diagnostics[i] = vDiagEvent(fmt.Sprintf("diag%d", i))
	}
	return xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{
		Operations: ops, Events: stages, DiagnosticEvents: diagnostics,
	}}
}

// metaViewOf marshals a meta fixture and opens a view on the exact bytes, so
// the raw buffer is available for offset assertions.
func metaViewOf(t *testing.T, meta xdr.TransactionMeta) ([]byte, xdr.TransactionMetaView) {
	t.Helper()
	raw, err := meta.MarshalBinary()
	require.NoError(t, err)
	return raw, xdr.TransactionMetaView(raw)
}

// assertSameSpans asserts two collected raw sets are the same bytes AT THE
// SAME OFFSETS inside base — the zero-copy contract plus proof that both
// routes located the same wire elements, not merely equal-looking ones.
func assertSameSpans(t *testing.T, base []byte, want, got [][]byte, ctx string) {
	t.Helper()
	require.Len(t, got, len(want), "%s: element count", ctx)
	for i := range want {
		assert.Equal(t, want[i], got[i], "%s: element %d bytes", ctx, i)
		wantStart, wantEnd, err := viewSpan(base, want[i])
		require.NoError(t, err, "%s: element %d reference span", ctx, i)
		gotStart, gotEnd, err := viewSpan(base, got[i])
		require.NoError(t, err, "%s: element %d span", ctx, i)
		assert.Equal(t, [2]int{wantStart, wantEnd}, [2]int{gotStart, gotEnd},
			"%s: element %d must be collected from its own wire offset", ctx, i)
	}
}

// TestOpEventRaws_VisitsEachOperationAtItsTrueOffset pins the V4 operations
// walk: opEventRaws steps the array by hand (it measures each operation from
// its events offset and extent rather than re-sizing the whole interior to
// advance), so the pin that matters is that every step lands on the operation
// the generated random-access accessor finds at that index — same bytes, same
// offsets, in order, one group per operation and no more. A walk that visited
// an operation twice, skipped one, or drifted by a byte could not reproduce
// the reference offsets.
func TestOpEventRaws_VisitsEachOperationAtItsTrueOffset(t *testing.T) {
	cases := [][]int{
		{1},
		{0},
		{3, 0, 5, 1, 0, 2},
		{0, 0, 0},
		{7, 7},
		{},
	}
	for _, counts := range cases {
		t.Run(fmt.Sprintf("ops=%v", counts), func(t *testing.T) {
			raw, metaView := metaViewOf(t, walkMetaV4(counts, 3, 2))
			v4, err := metaView.V4()
			require.NoError(t, err)
			ops, err := v4.Operations()
			require.NoError(t, err)

			got := opEventRaws(ops)
			require.Len(t, got, len(counts), "one event group per operation")
			assert.Equal(t, len(got), cap(got), "the group slice is presized from the operation count")

			for k, n := range counts {
				// Reference: locate operation k from scratch through the
				// generated accessors, independent of the hand-rolled step.
				op, opErr := ops.At(k)
				require.NoError(t, opErr)
				evs, evErr := op.Events()
				require.NoError(t, evErr)
				all, allErr := evs.All()
				require.NoError(t, allErr)
				want := make([][]byte, len(all))
				for i := range all {
					want[i] = all[i]
				}

				require.Len(t, got[k], n, "operation %d event count", k)
				assertSameSpans(t, raw, want, got[k], fmt.Sprintf("operation %d events", k))
				assert.Equal(t, len(got[k]), cap(got[k]),
					"operation %d events are presized from the array count", k)
			}
		})
	}
}

// refRaws re-collects an array view's elements through the generated All()
// accessor — a route independent of the package's own collection helper.
func refRaws[E ~[]byte, A interface{ All() ([]E, error) }](t *testing.T, arr A) [][]byte {
	t.Helper()
	all, err := arr.All()
	require.NoError(t, err)
	out := make([][]byte, len(all))
	for i := range all {
		out[i] = all[i]
	}
	return out
}

// refMetaEventRaws is metaEventRaws written the obvious way: every array
// reached through its own single-field accessor, every element collected with
// the generated All(), each operation located by index. It deliberately shares
// no code with the fused walk under test, so the two agreeing on bytes AND
// offsets is a real differential result.
func refMetaEventRaws(
	t *testing.T, metaView xdr.TransactionMetaView, wantEvents, wantDiag bool,
) (int32, TxEvents, [][]byte) {
	t.Helper()
	v, err := metaView.V()
	require.NoError(t, err)
	tev := TxEvents{TransactionEvents: [][]byte{}, OperationEvents: [][][]byte{}}
	diag := [][]byte{}

	switch v {
	case 0, 1, 2:
		// No contract events on these meta versions.
	case 3:
		v3, v3Err := metaView.V3()
		require.NoError(t, v3Err)
		opt, optErr := v3.SorobanMeta()
		require.NoError(t, optErr)
		soroban, present, unwrapErr := opt.Unwrap()
		require.NoError(t, unwrapErr)
		if !present {
			break
		}
		if wantEvents {
			evs, evErr := soroban.Events()
			require.NoError(t, evErr)
			tev.OperationEvents = [][][]byte{refRaws[xdr.ContractEventView](t, evs)}
		}
		if wantDiag {
			de, deErr := soroban.DiagnosticEvents()
			require.NoError(t, deErr)
			diag = refRaws[xdr.DiagnosticEventView](t, de)
		}
	case 4:
		v4, v4Err := metaView.V4()
		require.NoError(t, v4Err)
		if wantEvents {
			evs, evErr := v4.Events()
			require.NoError(t, evErr)
			tev.TransactionEvents = refRaws[xdr.TransactionEventView](t, evs)

			ops, opsErr := v4.Operations()
			require.NoError(t, opsErr)
			n, countErr := ops.Count()
			require.NoError(t, countErr)
			groups := make([][][]byte, 0, n)
			for k := 0; k < n; k++ {
				op, opErr := ops.At(k)
				require.NoError(t, opErr)
				opEvs, opEvErr := op.Events()
				require.NoError(t, opEvErr)
				groups = append(groups, refRaws[xdr.ContractEventView](t, opEvs))
			}
			tev.OperationEvents = groups
		}
		if wantDiag {
			de, deErr := v4.DiagnosticEvents()
			require.NoError(t, deErr)
			diag = refRaws[xdr.DiagnosticEventView](t, de)
		}
	default:
		t.Fatalf("unsupported meta version %d", v)
	}
	return v, tev, diag
}

// TestMetaEventRaws_MatchesAccessorWalk is the differential test over every
// meta shape the walk supports and every wantEvents/wantDiag combination its
// two callers use: the fused walk's products must be byte-for-byte — and
// offset-for-offset — what the plain accessor route produces, empty-not-nil
// slices included.
func TestMetaEventRaws_MatchesAccessorWalk(t *testing.T) {
	emptyOps := []xdr.OperationMeta{}
	cases := []struct {
		name string
		meta xdr.TransactionMeta
	}{
		{"v0", xdr.TransactionMeta{V: 0, Operations: &emptyOps}},
		{"v1", xdr.TransactionMeta{V: 1, V1: &xdr.TransactionMetaV1{}}},
		{"v2", xdr.TransactionMeta{V: 2, V2: &xdr.TransactionMetaV2{}}},
		{"v3-no-soroban-meta", xdr.TransactionMeta{V: 3, V3: &xdr.TransactionMetaV3{}}},
		{"v3-soroban-events-only", vMetaV3Soroban([]xdr.ContractEvent{
			vContractEvent("a"), vContractEvent("b"), vContractEvent("c"),
		})},
		{"v3-soroban-no-events", vMetaV3Soroban(nil)},
		{"v3-soroban-events-and-diagnostics", vMetaV3SorobanWithDiag(
			[]xdr.ContractEvent{vContractEvent("a"), vContractEvent("b"), vContractEvent("c")},
			[]xdr.DiagnosticEvent{vDiagEvent("d0"), vDiagEvent("d1"), vDiagEvent("d2")},
		)},
		{"v3-soroban-diagnostics-only", vMetaV3SorobanWithDiag(
			nil, []xdr.DiagnosticEvent{vDiagEvent("d0"), vDiagEvent("d1")},
		)},
		{"v4-no-operations", walkMetaV4(nil, 3, 2)},
		{"v4-operations-without-events", walkMetaV4([]int{0, 0}, 0, 0)},
		{"v4-per-op-events", walkMetaV4([]int{3, 0, 5, 1}, 0, 0)},
		{"v4-per-op-events-and-tx-events", walkMetaV4([]int{3, 0, 5, 1}, 3, 0)},
		{"v4-everything", walkMetaV4([]int{3, 0, 5, 1, 0, 2}, 3, 3)},
		{"v4-diagnostics-only", walkMetaV4(nil, 0, 3)},
	}

	wants := []struct {
		name                 string
		wantEvents, wantDiag bool
	}{
		{"events", true, false}, // the EventsFromTxParts arm
		{"diagnostics", false, true},
		{"both", true, true}, // the read path
	}

	for _, tc := range cases {
		for _, w := range wants {
			t.Run(tc.name+"/"+w.name, func(t *testing.T) {
				raw, metaView := metaViewOf(t, tc.meta)

				gotVer, gotEvents, gotDiag, err := metaEventRaws(metaView, w.wantEvents, w.wantDiag)
				require.NoError(t, err)
				wantVer, wantEvents, wantDiag := refMetaEventRaws(t, metaView, w.wantEvents, w.wantDiag)

				assert.Equal(t, wantVer, gotVer, "meta version")
				assert.Equal(t, wantEvents, gotEvents, "TxEvents")
				assert.Equal(t, wantDiag, gotDiag, "diagnostic events")

				assertSameSpans(t, raw, wantEvents.TransactionEvents, gotEvents.TransactionEvents, "transaction events")
				assertSameSpans(t, raw, wantDiag, gotDiag, "diagnostic events")
				require.Len(t, gotEvents.OperationEvents, len(wantEvents.OperationEvents), "operation groups")
				for k := range wantEvents.OperationEvents {
					assertSameSpans(t, raw, wantEvents.OperationEvents[k], gotEvents.OperationEvents[k],
						fmt.Sprintf("operation %d events", k))
				}
			})
		}
	}
}

// TestViewExtractorsPresizeTheirOutput pins the presizing: every per-ledger
// and per-transaction slice the extractors return is allocated at its exact
// final length, which append-growth cannot produce for any of these counts
// (3, 5 and 6 would land on capacity 4 and 8). It covers both product paths —
// EventsFromTxParts and the LedgerTransactionView read path, which also
// collects diagnostics.
func TestViewExtractorsPresizeTheirOutput(t *testing.T) {
	metas := []xdr.TransactionMeta{
		walkMetaV4([]int{3, 0, 5, 1, 0, 2}, 3, 3),
		vMetaV3SorobanWithDiag(
			[]xdr.ContractEvent{vContractEvent("a"), vContractEvent("b"), vContractEvent("c")},
			[]xdr.DiagnosticEvent{vDiagEvent("d0"), vDiagEvent("d1"), vDiagEvent("d2")},
		),
		walkMetaV4([]int{5}, 0, 0),
		vMetaV3Soroban([]xdr.ContractEvent{vContractEvent("x"), vContractEvent("y"), vContractEvent("z")}),
		walkMetaV4([]int{3, 3}, 5, 0),
	}
	lcm := buildEventsLCM(t, 9100, 1_700_000_000, metas)
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	view := xdr.LedgerCloseMetaView(raw)

	txParts, err := ExtractLedgerTxParts(view)
	require.NoError(t, err)
	require.Len(t, txParts, len(metas))
	assert.Equal(t, len(txParts), cap(txParts), "the parts slice is presized from the TxProcessing count")

	txEvents, err := EventsFromTxParts(txParts)
	require.NoError(t, err)
	require.Len(t, txEvents, len(txParts))
	assert.Equal(t, len(txEvents), cap(txEvents), "the events slice is presized from the parts count")
	for i, te := range txEvents {
		assert.Equal(t, len(te.TransactionEvents), cap(te.TransactionEvents),
			"tx %d: transaction events are presized", i)
		assert.Equal(t, len(te.OperationEvents), cap(te.OperationEvents),
			"tx %d: operation groups are presized", i)
		for k, group := range te.OperationEvents {
			assert.Equal(t, len(group), cap(group), "tx %d: operation %d events are presized", i, k)
		}
	}

	views, err := LedgerTransactionViewRange(view, 0, 0, viewTestPassphrase)
	require.NoError(t, err)
	require.Len(t, views, len(txParts))
	for i, v := range views {
		assert.Equal(t, len(v.DiagnosticEvents), cap(v.DiagnosticEvents),
			"tx %d: diagnostic events are presized", i)
		assert.Equal(t, len(v.TransactionEvents), cap(v.TransactionEvents),
			"tx %d: transaction events are presized", i)
		for k, group := range v.ContractEvents {
			assert.Equal(t, len(group), cap(group), "tx %d: operation %d events are presized", i, k)
		}
	}
}
