package ingest_test

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// The shared view-test fixture builders (vContractEvent, vResult,
// vMetaV3Soroban, vMetaV4OpEvents, sorobanTx, buildLCM, viewTestPassphrase, …)
// live in transaction_view_test.go — same package, one fixture vocabulary.

func txMetaWithStagedEvents(stageEvents []xdr.TransactionEvent) xdr.TransactionMeta {
	return xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{Events: stageEvents}}
}

// buildEventsLCMVersion adapts a bare meta list to the shared buildLCM
// fixture: each meta rides a fresh soroban envelope, TxSet in apply order, so
// the parsed LedgerTransactionReader and the view path agree on per-tx
// ordering.
func buildEventsLCMVersion(t testing.TB, version int32, ledgerSeq uint32, closeTimestamp int64, txMetas []xdr.TransactionMeta) xdr.LedgerCloseMeta {
	t.Helper()
	txs := make([]txWithHash, 0, len(txMetas))
	for _, meta := range txMetas {
		tx := sorobanTx(t, "fixture")
		tx.meta = meta
		txs = append(txs, tx)
	}
	return buildLCM(t, version, ledgerSeq, closeTimestamp, txs, false)
}

func buildEventsLCM(t testing.TB, ledgerSeq uint32, closeTimestamp int64, txMetas []xdr.TransactionMeta) xdr.LedgerCloseMeta {
	t.Helper()
	return buildEventsLCMVersion(t, 2, ledgerSeq, closeTimestamp, txMetas)
}

// TestTransactionEventsFromMeta_MatchesParsedReader proves the view-based
// per-tx event grouping is wire-identical to the parsed reference path
// (ingest.LedgerTransaction.GetTransactionEvents), across every supported meta
// shape, for LCM V1 and V2.
func TestTransactionEventsFromMeta_MatchesParsedReader(t *testing.T) {
	cases := []struct {
		name  string
		metas []xdr.TransactionMeta
	}{
		{"v4-op-events-only", []xdr.TransactionMeta{
			vMetaV4OpEvents([][]xdr.ContractEvent{{vContractEvent("a")}, {vContractEvent("b")}}),
			vMetaV4OpEvents([][]xdr.ContractEvent{{vContractEvent("c")}}),
		}},
		{"v4-top-level-stages", []xdr.TransactionMeta{
			txMetaWithStagedEvents([]xdr.TransactionEvent{
				{Stage: xdr.TransactionEventStageTransactionEventStageBeforeAllTxs, Event: vContractEvent("before")},
				{Stage: xdr.TransactionEventStageTransactionEventStageAfterAllTxs, Event: vContractEvent("after")},
				{Stage: xdr.TransactionEventStageTransactionEventStageAfterTx, Event: vContractEvent("aftertx")},
			}),
		}},
		{"v4-mixed", []xdr.TransactionMeta{{
			V: 4,
			V4: &xdr.TransactionMetaV4{
				Events: []xdr.TransactionEvent{
					{Stage: xdr.TransactionEventStageTransactionEventStageBeforeAllTxs, Event: vContractEvent("pre")},
					{Stage: xdr.TransactionEventStageTransactionEventStageAfterTx, Event: vContractEvent("post")},
				},
				Operations: []xdr.OperationMetaV2{
					{Events: []xdr.ContractEvent{vContractEvent("opA")}},
					{Events: []xdr.ContractEvent{vContractEvent("opB")}},
				},
			},
		}}},
		{"v1-v2-skip", []xdr.TransactionMeta{
			{V: 1, V1: &xdr.TransactionMetaV1{}},
			{V: 2, V2: &xdr.TransactionMetaV2{}},
		}},
		{"v3-soroban", []xdr.TransactionMeta{
			vMetaV3Soroban([]xdr.ContractEvent{vContractEvent("v3a"), vContractEvent("v3b")}),
			vMetaV3Soroban([]xdr.ContractEvent{vContractEvent("v3c")}),
			vMetaV3Soroban(nil),
		}},
	}

	for _, version := range []int32{1, 2} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("lcmV%d/%s", version, tc.name), func(t *testing.T) {
				lcm := buildEventsLCMVersion(t, version, 4000+uint32(version), 1_700_000_000, tc.metas)
				assertEventsViewMatchesReader(t, lcm)
			})
		}
	}
}

func assertEventsViewMatchesReader(t *testing.T, lcm xdr.LedgerCloseMeta) {
	t.Helper()
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	view := xdr.LedgerCloseMetaView(raw)

	// Parsed oracle: collect per-tx events in apply order.
	reader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(viewTestPassphrase, lcm)
	require.NoError(t, err)
	var oracle []ingest.TransactionEvents
	for {
		tx, err := reader.Read()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		te, err := tx.GetTransactionEvents()
		require.NoError(t, err)
		oracle = append(oracle, te)
	}

	// View path: walk TxProcessing, extract per-tx event groups.
	d, err := ingest.DispatchLedgerCloseMetaView(view)
	require.NoError(t, err)
	seq, _, err := d.Header()
	require.NoError(t, err)
	require.Equal(t, uint32(lcm.LedgerSequence()), seq, "ledger seq")

	i := 0
	for tx, iterErr := range d.TxProcessing() {
		require.NoError(t, iterErr)
		require.Less(t, i, len(oracle), "more view txs than oracle txs")

		// Hash agrees with the parsed reader's stored hash.
		h, err := ingest.TxProcessingHash(tx)
		require.NoError(t, err)

		metaView, err := tx.TxApplyProcessing()
		require.NoError(t, err)
		vev, err := ingest.TransactionEventsFromMeta(metaView)
		require.NoError(t, err)

		want := oracle[i]
		ctx := func(f string) string { return fmt.Sprintf("%s mismatch tx %d (hash %x)", f, i, h) }

		require.Len(t, vev.TransactionEvents, len(want.TransactionEvents), ctx("TransactionEvents len"))
		for j := range want.TransactionEvents {
			wantBytes, err := want.TransactionEvents[j].MarshalBinary()
			require.NoError(t, err)
			assert.Equal(t, wantBytes, vev.TransactionEvents[j], ctx(fmt.Sprintf("TransactionEvents[%d] bytes", j)))
		}

		require.Len(t, vev.OperationEvents, len(want.OperationEvents), ctx("OperationEvents len"))
		for op := range want.OperationEvents {
			require.Len(t, vev.OperationEvents[op], len(want.OperationEvents[op]), ctx(fmt.Sprintf("OperationEvents[%d] len", op)))
			for j := range want.OperationEvents[op] {
				wantBytes, err := want.OperationEvents[op][j].MarshalBinary()
				require.NoError(t, err)
				assert.Equal(t, wantBytes, vev.OperationEvents[op][j], ctx(fmt.Sprintf("OperationEvents[%d][%d] bytes", op, j)))
			}
		}
		i++
	}
	require.Equal(t, len(oracle), i, "view tx count differs from oracle")
}

// TestDiagnosticEventsFromMeta_MatchesParsedReader cross-checks the raw
// diagnostic-event bytes against the parsed reader for V3/V4 metas that carry
// diagnostic events.
func TestDiagnosticEventsFromMeta_MatchesParsedReader(t *testing.T) {
	v3WithDiag := xdr.TransactionMeta{
		V: 3,
		V3: &xdr.TransactionMetaV3{SorobanMeta: &xdr.SorobanTransactionMeta{
			Events:           []xdr.ContractEvent{vContractEvent("v3-ev")},
			DiagnosticEvents: []xdr.DiagnosticEvent{vDiagEvent("v3-diag1"), vDiagEvent("v3-diag2")},
			ReturnValue:      xdr.ScVal{Type: xdr.ScValTypeScvVoid},
		}},
	}
	v4WithDiag := xdr.TransactionMeta{
		V: 4,
		V4: &xdr.TransactionMetaV4{
			DiagnosticEvents: []xdr.DiagnosticEvent{vDiagEvent("v4-diag")},
			Operations:       []xdr.OperationMetaV2{{Events: []xdr.ContractEvent{vContractEvent("v4-op")}}},
		},
	}
	lcm := buildEventsLCM(t, 4500, 1_700_005_000, []xdr.TransactionMeta{v3WithDiag, v4WithDiag})
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	view := xdr.LedgerCloseMetaView(raw)

	reader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(viewTestPassphrase, lcm)
	require.NoError(t, err)
	var oracle [][]xdr.DiagnosticEvent
	for {
		tx, err := reader.Read()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		de, err := tx.GetDiagnosticEvents()
		require.NoError(t, err)
		oracle = append(oracle, de)
	}

	d, err := ingest.DispatchLedgerCloseMetaView(view)
	require.NoError(t, err)
	i := 0
	for tx, iterErr := range d.TxProcessing() {
		require.NoError(t, iterErr)
		metaView, err := tx.TxApplyProcessing()
		require.NoError(t, err)
		vdiag, err := ingest.DiagnosticEventsFromMeta(metaView)
		require.NoError(t, err)
		require.Len(t, vdiag, len(oracle[i]), "diag len tx %d", i)
		for j := range oracle[i] {
			wantBytes, err := oracle[i][j].MarshalBinary()
			require.NoError(t, err)
			assert.Equal(t, wantBytes, vdiag[j], "diag bytes tx %d ev %d", i, j)
		}
		i++
	}
	require.Equal(t, len(oracle), i)
}

// TestTransactionEventsFromMeta_LegacyV0 confirms legacy TransactionMeta V0
// (which the parsed reader rejects) is tolerated as event-free by the view path.
func TestTransactionEventsFromMeta_LegacyV0(t *testing.T) {
	lcm := buildEventsLCM(t, 4600, 1_700_006_000, []xdr.TransactionMeta{
		{V: 0, Operations: &[]xdr.OperationMeta{}},
	})
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	d, err := ingest.DispatchLedgerCloseMetaView(xdr.LedgerCloseMetaView(raw))
	require.NoError(t, err)
	for tx, iterErr := range d.TxProcessing() {
		require.NoError(t, iterErr)
		metaView, err := tx.TxApplyProcessing()
		require.NoError(t, err)
		vev, err := ingest.TransactionEventsFromMeta(metaView)
		require.NoError(t, err, "legacy meta V0 must be event-free, not error")
		assert.Empty(t, vev.TransactionEvents)
		assert.Empty(t, vev.OperationEvents)
	}
}

// TestTransactionEventsFromMeta_V3GateIsCallerResponsibility pins the ONE
// deliberate divergence between the view extractor and the parsed
// GetTransactionEvents: for a V3 meta, TransactionEventsFromMeta emits
// SorobanMeta.Events whenever SorobanMeta is present (the events-index path
// relies on the trusted-input invariant "SorobanMeta present ⟺ soroban tx"),
// while GetTransactionEvents gates case 3 on IsSorobanTx. The read path
// re-applies that gate downstream (gateV3ContractEvents, exercised by
// TestTransactionView_EquivalentToLedgerTransaction); this test asserts the
// split itself so a future "fix" aligning the two doesn't silently change
// either consumer's contract.
func TestTransactionEventsFromMeta_V3GateIsCallerResponsibility(t *testing.T) {
	// Classic (non-soroban) envelope paired with a V3 meta that carries
	// SorobanMeta.Events — input that violates the trusted-input invariant.
	classicEnv := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{SourceAccount: xdr.MustMuxedAddress(keypair.MustRandom().Address())},
		},
	}
	hash, err := network.HashTransactionInEnvelope(classicEnv, viewTestPassphrase)
	require.NoError(t, err)
	meta := vMetaV3Soroban([]xdr.ContractEvent{vContractEvent("gated")})

	lcm := xdr.LedgerCloseMeta{V: 1, V1: &xdr.LedgerCloseMetaV1{
		LedgerHeader: xdr.LedgerHeaderHistoryEntry{Header: xdr.LedgerHeader{
			ScpValue: xdr.StellarValue{CloseTime: 1_700_060_000}, LedgerSeq: 4700,
		}},
		TxSet: xdr.GeneralizedTransactionSet{V: 1, V1TxSet: &xdr.TransactionSetV1{
			Phases: []xdr.TransactionPhase{{V: 0, V0Components: &[]xdr.TxSetComponent{{
				Type: xdr.TxSetComponentTypeTxsetCompTxsMaybeDiscountedFee,
				TxsMaybeDiscountedFee: &xdr.TxSetComponentTxsMaybeDiscountedFee{
					Txs: []xdr.TransactionEnvelope{classicEnv},
				},
			}}}},
		}},
		TxProcessing: []xdr.TransactionResultMeta{{
			TxApplyProcessing: meta,
			Result:            xdr.TransactionResultPair{TransactionHash: hash, Result: vResult(true)},
		}},
	}}
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)

	// Parsed path: GetTransactionEvents gates on IsSorobanTx → no events.
	oracle := func() ingest.TransactionEvents {
		r, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(viewTestPassphrase, lcm)
		require.NoError(t, err)
		tx, err := r.Read()
		require.NoError(t, err)
		require.False(t, tx.IsSorobanTx(), "fixture must be a classic tx")
		te, err := tx.GetTransactionEvents()
		require.NoError(t, err)
		return te
	}()
	assert.Empty(t, oracle.OperationEvents, "parsed GetTransactionEvents gates V3 events for a classic tx")

	// View extractor: ungated — emits the present SorobanMeta.Events; the
	// soroban gate is the read path's job (it has the paired envelope).
	d, err := ingest.DispatchLedgerCloseMetaView(xdr.LedgerCloseMetaView(raw))
	require.NoError(t, err)
	for tx, iterErr := range d.TxProcessing() {
		require.NoError(t, iterErr)
		mv, err := tx.TxApplyProcessing()
		require.NoError(t, err)
		vev, err := ingest.TransactionEventsFromMeta(mv)
		require.NoError(t, err)
		require.Len(t, vev.OperationEvents, 1, "view extractor must emit ungated V3 events")
		require.Len(t, vev.OperationEvents[0], 1)
	}

	// And the read path re-establishes parity with the parsed reader by
	// applying the gate: contract events empty for the same LCM.
	got, err := ingest.TransactionViewRange(xdr.LedgerCloseMetaView(raw), 0, 0, viewTestPassphrase)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Empty(t, got[0].ContractEvents, "read path must gate V3 contract events for a classic tx")
}
