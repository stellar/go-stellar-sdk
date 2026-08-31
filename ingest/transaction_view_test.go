package ingest

import (
	"fmt"
	"io"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const viewTestPassphrase = network.TestNetworkPassphrase

func vContractEvent(topic string) xdr.ContractEvent {
	var contractID xdr.ContractId
	contractID[0] = 0xab
	sym := xdr.ScSymbol(topic)
	return xdr.ContractEvent{
		ContractId: &contractID,
		Type:       xdr.ContractEventTypeContract,
		Body: xdr.ContractEventBody{V: 0, V0: &xdr.ContractEventV0{
			Topics: []xdr.ScVal{{Type: xdr.ScValTypeScvSymbol, Sym: &sym}},
			Data:   xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &sym},
		}},
	}
}

func vResult(success bool) xdr.TransactionResult {
	return feeResult(100, success, 0)
}

func vMetaV3Soroban(evs []xdr.ContractEvent) xdr.TransactionMeta {
	return xdr.TransactionMeta{V: 3, V3: &xdr.TransactionMetaV3{SorobanMeta: &xdr.SorobanTransactionMeta{
		Events: evs, ReturnValue: xdr.ScVal{Type: xdr.ScValTypeScvVoid},
	}}}
}

// vMetaV3SorobanWithDiag is vMetaV3Soroban plus diagnostic events — needed to
// exercise the diagnostics arm of the V3 path (incl. the classic-envelope
// divergence case where contract events are gated but diagnostics are not).
func vMetaV3SorobanWithDiag(evs []xdr.ContractEvent, diags []xdr.DiagnosticEvent) xdr.TransactionMeta {
	m := vMetaV3Soroban(evs)
	m.V3.SorobanMeta.DiagnosticEvents = diags
	return m
}

func vDiagEvent(topic string) xdr.DiagnosticEvent {
	return xdr.DiagnosticEvent{InSuccessfulContractCall: true, Event: vContractEvent(topic)}
}

func vMetaV4OpEvents(opEvents [][]xdr.ContractEvent) xdr.TransactionMeta {
	ops := make([]xdr.OperationMetaV2, len(opEvents))
	for i, evs := range opEvents {
		ops[i] = xdr.OperationMetaV2{Events: evs}
	}
	return xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{Operations: ops}}
}

type txWithHash struct {
	env    xdr.TransactionEnvelope
	hash   xdr.Hash
	meta   xdr.TransactionMeta
	result *xdr.TransactionResult // nil → vResult(true)
}

// resultPair is the TxProcessing result pair for this tx: the fixture's
// explicit result when set, a plain success otherwise.
func (tx txWithHash) resultPair() xdr.TransactionResultPair {
	res := vResult(true)
	if tx.result != nil {
		res = *tx.result
	}
	return xdr.TransactionResultPair{TransactionHash: tx.hash, Result: res}
}

func sorobanTx(t testing.TB, topic string) txWithHash {
	t.Helper()
	env := xdr.TransactionEnvelope{Type: xdr.EnvelopeTypeEnvelopeTypeTx, V1: &xdr.TransactionV1Envelope{
		Tx: xdr.Transaction{
			SourceAccount: xdr.MustMuxedAddress(keypair.MustRandom().Address()),
			Ext:           xdr.TransactionExt{V: 1, SorobanData: &xdr.SorobanTransactionData{}},
		},
	}}
	hash, err := network.HashTransactionInEnvelope(env, viewTestPassphrase)
	require.NoError(t, err)
	return txWithHash{env: env, hash: hash, meta: vMetaV3Soroban([]xdr.ContractEvent{vContractEvent(topic)})}
}

// classicV3Tx is a V3-meta transaction on a NON-soroban (classic) envelope —
// exercises the V3 contract-event soroban gate (events must be dropped).
func classicV3Tx(t testing.TB, topic string) txWithHash {
	t.Helper()
	env := xdr.TransactionEnvelope{Type: xdr.EnvelopeTypeEnvelopeTypeTx, V1: &xdr.TransactionV1Envelope{
		Tx: xdr.Transaction{SourceAccount: xdr.MustMuxedAddress(keypair.MustRandom().Address())},
	}}
	hash, err := network.HashTransactionInEnvelope(env, viewTestPassphrase)
	require.NoError(t, err)
	return txWithHash{env: env, hash: hash, meta: vMetaV3Soroban([]xdr.ContractEvent{vContractEvent(topic)})}
}

func txV0(t testing.TB, tb *xdr.TimeBounds, seq int64) txWithHash {
	t.Helper()
	accID := xdr.MustAddress(keypair.MustRandom().Address())
	env := xdr.TransactionEnvelope{Type: xdr.EnvelopeTypeEnvelopeTypeTxV0, V0: &xdr.TransactionV0Envelope{
		Tx: xdr.TransactionV0{
			SourceAccountEd25519: *accID.Ed25519, Fee: 100, SeqNum: xdr.SequenceNumber(seq),
			TimeBounds: tb, Memo: xdr.Memo{Type: xdr.MemoTypeMemoNone},
		},
	}}
	hash, err := network.HashTransactionInEnvelope(env, viewTestPassphrase)
	require.NoError(t, err)
	return txWithHash{env: env, hash: hash, meta: vMetaV4OpEvents([][]xdr.ContractEvent{{vContractEvent("v0ev")}})}
}

// fixtureHeader is the ledger header every LCM builder uses (protocol 23).
func fixtureHeader(ledgerSeq uint32, closeTime int64) xdr.LedgerHeaderHistoryEntry {
	return xdr.LedgerHeaderHistoryEntry{Header: xdr.LedgerHeader{
		ScpValue:      xdr.StellarValue{CloseTime: xdr.TimePoint(closeTime)}, //nolint:gosec // test close times are positive
		LedgerSeq:     xdr.Uint32(ledgerSeq),
		LedgerVersion: 23,
	}}
}

// resultMetas builds the V0/V1 TxProcessing array for the given fixtures.
func resultMetas(txs []txWithHash) []xdr.TransactionResultMeta {
	proc := make([]xdr.TransactionResultMeta, len(txs))
	for i, tx := range txs {
		proc[i] = xdr.TransactionResultMeta{TxApplyProcessing: tx.meta, Result: tx.resultPair()}
	}
	return proc
}

// resultMetaV1s is resultMetas for the V2 TxProcessing array.
func resultMetaV1s(txs []txWithHash) []xdr.TransactionResultMetaV1 {
	proc := make([]xdr.TransactionResultMetaV1, len(txs))
	for i, tx := range txs {
		proc[i] = xdr.TransactionResultMetaV1{TxApplyProcessing: tx.meta, Result: tx.resultPair()}
	}
	return proc
}

// buildLCM builds an LCM of the given version. reverseTxSet lists the TxSet
// envelopes in REVERSED order relative to TxProcessing apply order, exercising
// agreed-set vs apply-order skew (positional pairing would mispair).
func buildLCM(t testing.TB, version int32, ledgerSeq uint32, closeTime int64, txs []txWithHash, reverseTxSet bool) xdr.LedgerCloseMeta {
	t.Helper()
	envs := make([]xdr.TransactionEnvelope, len(txs))
	for i, tx := range txs {
		if reverseTxSet {
			envs[len(txs)-1-i] = tx.env
		} else {
			envs[i] = tx.env
		}
	}
	header := fixtureHeader(ledgerSeq, closeTime)

	if version == 0 {
		var prev xdr.Hash
		prev[0] = 0x77
		return xdr.LedgerCloseMeta{V: 0, V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: header, TxSet: xdr.TransactionSet{PreviousLedgerHash: prev, Txs: envs},
			TxProcessing: resultMetas(txs),
		}}
	}

	comp := []xdr.TxSetComponent{{Type: xdr.TxSetComponentTypeTxsetCompTxsMaybeDiscountedFee,
		TxsMaybeDiscountedFee: &xdr.TxSetComponentTxsMaybeDiscountedFee{Txs: envs}}}
	txSet := xdr.GeneralizedTransactionSet{V: 1, V1TxSet: &xdr.TransactionSetV1{
		Phases: []xdr.TransactionPhase{{V: 0, V0Components: &comp}},
	}}
	switch version {
	case 1:
		return xdr.LedgerCloseMeta{V: 1, V1: &xdr.LedgerCloseMetaV1{
			LedgerHeader: header, TxSet: txSet, TxProcessing: resultMetas(txs),
		}}
	case 2:
		return xdr.LedgerCloseMeta{V: 2, V2: &xdr.LedgerCloseMetaV2{
			LedgerHeader: header, TxSet: txSet, TxProcessing: resultMetaV1s(txs),
		}}
	default:
		t.Fatalf("unsupported version %d", version)
		return xdr.LedgerCloseMeta{}
	}
}

// readerOracle reads the LCM with the parsed LedgerTransactionReader and returns
// the transactions in apply order.
func readerOracle(t *testing.T, lcm xdr.LedgerCloseMeta) []LedgerTransaction {
	t.Helper()
	r, err := NewLedgerTransactionReaderFromLedgerCloseMeta(viewTestPassphrase, lcm)
	require.NoError(t, err)
	var out []LedgerTransaction
	for {
		tx, err := r.Read()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		out = append(out, tx)
	}
	return out
}

// assertMatchesReader asserts a view Transaction equals the parsed reader's
// transaction field-by-field, wire-identical.
func assertMatchesReader(t *testing.T, want LedgerTransaction, got LedgerTransactionView, applyIdx int) {
	t.Helper()
	ctx := func(f string) string { return fmt.Sprintf("%s mismatch tx %d", f, applyIdx) }

	assert.Equal(t, [32]byte(want.Hash), got.Hash, ctx("Hash"))
	assert.Equal(t, int32(want.Index), got.ApplicationOrder, ctx("ApplicationOrder"))
	assert.Equal(t, want.Envelope.Type == xdr.EnvelopeTypeEnvelopeTypeTxFeeBump, got.FeeBump, ctx("FeeBump"))
	assert.Equal(t, want.Result.Result.Successful(), got.Successful, ctx("Successful"))

	wantEnv, err := want.Envelope.MarshalBinary()
	require.NoError(t, err)
	assert.Equal(t, wantEnv, got.Envelope, ctx("Envelope"))
	wantRes, err := want.Result.Result.MarshalBinary()
	require.NoError(t, err)
	assert.Equal(t, wantRes, got.Result, ctx("Result"))
	wantMeta, err := want.UnsafeMeta.MarshalBinary()
	require.NoError(t, err)
	assert.Equal(t, wantMeta, got.Meta, ctx("Meta"))

	// Events: diagnostic, transaction-level, per-op contract (gated by the
	// reader on IsSorobanTx for V3 — the view path must match).
	// Ledger header fields come straight from the LCM the reader holds.
	assert.Equal(t, uint32(want.Ledger.LedgerSequence()), got.LedgerSequence, ctx("LedgerSequence"))
	assert.Equal(t, want.Ledger.LedgerCloseTime(), got.LedgerCloseTime, ctx("LedgerCloseTime"))

	// Diagnostics oracle is the STANDALONE GetDiagnosticEvents — ungated on
	// IsSorobanTx — matching stellar-rpc's db.ParseTransaction (the original
	// read-path reference this struct mirrors). GetTransactionEvents' case 3
	// gates everything on IsSorobanTx, so its DiagnosticEvents field would be
	// empty for a classic-envelope V3 tx where this path (deliberately)
	// retains them; only ContractEvents carries that gate.
	diag, err := want.GetDiagnosticEvents()
	require.NoError(t, err)
	assertRawEventsMatch(t, diag, got.DiagnosticEvents, ctx("DiagnosticEvents"))

	te, err := want.GetTransactionEvents()
	require.NoError(t, err)
	assertRawEventsMatch(t, te.TransactionEvents, got.TransactionEvents, ctx("TransactionEvents"))
	require.Len(t, got.ContractEvents, len(te.OperationEvents), ctx("ContractEvents op count"))
	for op := range te.OperationEvents {
		assertRawEventsMatch(t, te.OperationEvents[op], got.ContractEvents[op], ctx(fmt.Sprintf("ContractEvents[%d]", op)))
	}
}

type binaryMarshaler interface{ MarshalBinary() ([]byte, error) }

func assertRawEventsMatch[E binaryMarshaler](t *testing.T, want []E, gotRaw [][]byte, ctx string) {
	t.Helper()
	require.Len(t, gotRaw, len(want), ctx+" len")
	for i := range want {
		wb, err := want[i].MarshalBinary()
		require.NoError(t, err)
		assert.Equal(t, wb, gotRaw[i], fmt.Sprintf("%s[%d] bytes", ctx, i))
	}
}

// TestTransactionViewRange_MatchesReader is the field-by-field differential
// across LCM V0/V1/V2 with a mix of meta versions, including a fee-bump-free
// soroban tx, a classic V3 tx (gate), a V0 envelope, and a V4 op-event tx.
func TestTransactionViewRange_MatchesReader(t *testing.T) {
	for _, version := range []int32{0, 1, 2} {
		t.Run(fmt.Sprintf("lcmV%d", version), func(t *testing.T) {
			txs := []txWithHash{
				sorobanTx(t, "soroban-a"),
				classicV3Tx(t, "classic-v3-gated"),
				txV0(t, &xdr.TimeBounds{MinTime: 1, MaxTime: 1_900_000_000}, 9),
				{
					env:  sorobanTx(t, "ignored").env, // a V4-meta soroban tx
					hash: xdr.Hash{},
					meta: vMetaV4OpEvents([][]xdr.ContractEvent{{vContractEvent("v4-op")}}),
				},
			}
			// Fix the 4th tx's hash to match its envelope.
			h, err := network.HashTransactionInEnvelope(txs[3].env, viewTestPassphrase)
			require.NoError(t, err)
			txs[3].hash = h

			lcm := buildLCM(t, version, 9000+uint32(version), 1_700_040_000, txs, true /*reversed*/)
			raw, err := lcm.MarshalBinary()
			require.NoError(t, err)
			view := xdr.LedgerCloseMetaView(raw)

			oracle := readerOracle(t, lcm)
			require.Len(t, oracle, len(txs))

			got, err := LedgerTransactionViewRange(view, 0, 0, viewTestPassphrase)
			require.NoError(t, err)
			require.Len(t, got, len(oracle))
			for k := range got {
				assertMatchesReader(t, oracle[k], got[k], k)
			}

			// The classic V3 tx (index 1) must have its contract events gated off.
			assert.Empty(t, got[1].ContractEvents, "classic V3 tx contract events must be gated to empty")
		})
	}
}

// TestTransactionViewByHash covers found + not-found, against the reader.
func TestTransactionViewByHash(t *testing.T) {
	txs := []txWithHash{sorobanTx(t, "a"), sorobanTx(t, "b"), sorobanTx(t, "c")}
	lcm := buildLCM(t, 2, 9100, 1_700_041_000, txs, true)
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	view := xdr.LedgerCloseMetaView(raw)
	oracle := readerOracle(t, lcm)

	for k, tx := range txs {
		got, found, byHashErr := LedgerTransactionViewByHash(view, [32]byte(tx.hash), viewTestPassphrase)
		require.NoError(t, byHashErr)
		require.True(t, found, "tx %d should be found", k)
		// Find the oracle entry with this hash (apply order may differ from txs order).
		var want LedgerTransaction
		for _, o := range oracle {
			if [32]byte(o.Hash) == [32]byte(tx.hash) {
				want = o
			}
		}
		assertMatchesReader(t, want, got, int(want.Index)-1)
	}

	var missing [32]byte
	missing[0] = 0xde
	_, found, err := LedgerTransactionViewByHash(view, missing, viewTestPassphrase)
	require.NoError(t, err)
	assert.False(t, found, "absent hash must report found=false, nil error")
}

// TestTransactionViewRange_Cursor covers startIdx/limit slicing and edges.
func TestTransactionViewRange_Cursor(t *testing.T) {
	txs := []txWithHash{sorobanTx(t, "a"), sorobanTx(t, "b"), sorobanTx(t, "c"), sorobanTx(t, "d")}
	lcm := buildLCM(t, 2, 9200, 1_700_042_000, txs, false)
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	view := xdr.LedgerCloseMetaView(raw)

	// startIdx=1, limit=2 → txs at apply index 1,2 (ApplicationOrder 2,3).
	page, err := LedgerTransactionViewRange(view, 1, 2, viewTestPassphrase)
	require.NoError(t, err)
	require.Len(t, page, 2)
	assert.Equal(t, int32(2), page[0].ApplicationOrder)
	assert.Equal(t, int32(3), page[1].ApplicationOrder)

	// limit=0 from startIdx → all remaining.
	all, err := LedgerTransactionViewRange(view, 0, 0, viewTestPassphrase)
	require.NoError(t, err)
	require.Len(t, all, 4)

	// startIdx past end → empty, nil error.
	empty, err := LedgerTransactionViewRange(view, 99, 0, viewTestPassphrase)
	require.NoError(t, err)
	assert.Empty(t, empty)

	// Negative args → error.
	_, err = LedgerTransactionViewRange(view, -1, 0, viewTestPassphrase)
	require.Error(t, err)
	_, err = LedgerTransactionViewRange(view, 0, -1, viewTestPassphrase)
	require.Error(t, err)
}

// feeBumpTx returns a fee-bump envelope wrapping an op-less soroban inner tx,
// with the given meta and a real fee-bump result (txFEE_BUMP_INNER_SUCCESS
// carrying the inner hash). The TxProcessing hash for a fee-bump entry is the
// OUTER hash.
func feeBumpTx(t testing.TB, meta xdr.TransactionMeta) txWithHash {
	t.Helper()
	return feeBumpFeeTx(t, nil, true, meta, 200)
}

// innerHashOf is the network hash of a fee-bump envelope's inner transaction.
func innerHashOf(t testing.TB, env xdr.TransactionEnvelope) xdr.Hash {
	t.Helper()
	innerEnv := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1:   env.FeeBump.Tx.InnerTx.V1,
	}
	h, err := network.HashTransactionInEnvelope(innerEnv, viewTestPassphrase)
	require.NoError(t, err)
	return h
}

// TestTransactionView_EquivalentToLedgerTransaction is the explicit
// LedgerTransaction ↔ LedgerTransactionView equivalence test: every field of the
// zero-copy view Transaction must be derivable from — and wire-identical to —
// the parsed LedgerTransaction for the same LCM, including the ledger header
// fields (LedgerSequence, LedgerCloseTime) and the diagnostic-events arm.
//
// The fixture set deliberately includes the divergence-prone shapes:
//   - soroban V3 with contract + diagnostic events (everything populated)
//   - classic-envelope V3 WITH diagnostic events (contract events gated,
//     diagnostics NOT gated — matching db.ParseTransaction / the standalone
//     GetDiagnosticEvents, the read path's original oracle)
//   - fee-bump wrapping a soroban inner (FeeBump flag + outer-hash pairing)
//   - V4 with top-level staged events + per-op events + diagnostics
func TestTransactionView_EquivalentToLedgerTransaction(t *testing.T) {
	v4Full := xdr.TransactionMeta{
		V: 4,
		V4: &xdr.TransactionMetaV4{
			Events: []xdr.TransactionEvent{
				{Stage: xdr.TransactionEventStageTransactionEventStageBeforeAllTxs, Event: vContractEvent("v4-pre")},
				{Stage: xdr.TransactionEventStageTransactionEventStageAfterTx, Event: vContractEvent("v4-post")},
			},
			DiagnosticEvents: []xdr.DiagnosticEvent{vDiagEvent("v4-diag")},
			Operations: []xdr.OperationMetaV2{
				{Events: []xdr.ContractEvent{vContractEvent("v4-opA")}},
				{Events: []xdr.ContractEvent{vContractEvent("v4-opB")}},
			},
		},
	}
	sorobanWithDiag := sorobanTx(t, "ignored")
	sorobanWithDiag.meta = vMetaV3SorobanWithDiag(
		[]xdr.ContractEvent{vContractEvent("v3-ev")},
		[]xdr.DiagnosticEvent{vDiagEvent("v3-diag-a"), vDiagEvent("v3-diag-b")})
	classicWithDiag := classicV3Tx(t, "ignored")
	classicWithDiag.meta = vMetaV3SorobanWithDiag(
		[]xdr.ContractEvent{vContractEvent("v3-classic-ev")},
		[]xdr.DiagnosticEvent{vDiagEvent("v3-classic-diag")})

	txs := []txWithHash{
		sorobanWithDiag,
		classicWithDiag,
		feeBumpTx(t, vMetaV3Soroban([]xdr.ContractEvent{vContractEvent("fb-ev")})),
		{env: sorobanTx(t, "x").env, meta: v4Full},
	}
	h, err := network.HashTransactionInEnvelope(txs[3].env, viewTestPassphrase)
	require.NoError(t, err)
	txs[3].hash = h

	for _, version := range []int32{0, 1, 2} {
		t.Run(fmt.Sprintf("lcmV%d", version), func(t *testing.T) {
			lcm := buildLCM(t, version, 9500+uint32(version), 1_700_050_000, txs, true /*reversed TxSet*/)
			raw, err := lcm.MarshalBinary()
			require.NoError(t, err)
			view := xdr.LedgerCloseMetaView(raw)

			oracle := readerOracle(t, lcm)
			require.Len(t, oracle, len(txs))

			got, err := LedgerTransactionViewRange(view, 0, 0, viewTestPassphrase)
			require.NoError(t, err)
			require.Len(t, got, len(oracle))
			for k := range got {
				assertMatchesReader(t, oracle[k], got[k], k)
			}

			// The classic-envelope V3 tx (apply index 1): contract events
			// gated off, diagnostics retained (ungated, like
			// GetDiagnosticEvents / db.ParseTransaction).
			assert.Empty(t, got[1].ContractEvents, "classic V3 contract events must be gated")
			assert.Len(t, got[1].DiagnosticEvents, 1, "classic V3 diagnostics must NOT be gated")

			// Fee-bump flag from the envelope type.
			assert.True(t, got[2].FeeBump, "fee-bump envelope must set FeeBump")
		})
	}
}

// TestTransactionViewRange_ExtremeLimit guards the exported API against
// caller-controlled limits: a huge limit must behave like "all from startIdx"
// (no makeslice panic from preallocating limit, no start+limit overflow
// silently yielding an empty page).
func TestTransactionViewRange_ExtremeLimit(t *testing.T) {
	txs := []txWithHash{sorobanTx(t, "a"), sorobanTx(t, "b"), sorobanTx(t, "c"), sorobanTx(t, "d")}
	lcm := buildLCM(t, 2, 9300, 1_700_043_000, txs, false)
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	view := xdr.LedgerCloseMetaView(raw)

	all, err := LedgerTransactionViewRange(view, 0, math.MaxInt, viewTestPassphrase)
	require.NoError(t, err, "huge limit must not panic")
	require.Len(t, all, 4)

	tail, err := LedgerTransactionViewRange(view, 1, math.MaxInt, viewTestPassphrase)
	require.NoError(t, err)
	require.Len(t, tail, 3, "start+limit overflow must not yield a silently-empty page")
	assert.Equal(t, int32(2), tail[0].ApplicationOrder)
}

// buildParallelTxsLCM builds an LCM V2 whose GeneralizedTransactionSet uses a
// V=1 TransactionPhase (ParallelTxsComponent) with multiple ExecutionStages,
// multiple clusters, and a cluster holding >1 tx — exercising
// enumerateParallelTxs. The clusters' envelope order is deliberately NOT the
// apply order: apply order is txs[0..n) (TxProcessing), but the clusters list
// them in a shuffled layout, so hash-pairing is exercised here too. layout is
// a slice of stages; each stage is a slice of clusters; each cluster is a
// slice of indices into txs.
func buildParallelTxsLCM(t testing.TB, ledgerSeq uint32, closeTime int64, txs []txWithHash, layout [][][]int) xdr.LedgerCloseMeta {
	t.Helper()

	stages := make([]xdr.ParallelTxExecutionStage, 0, len(layout))
	for _, stage := range layout {
		clusters := make(xdr.ParallelTxExecutionStage, 0, len(stage))
		for _, cluster := range stage {
			cl := make(xdr.DependentTxCluster, 0, len(cluster))
			for _, idx := range cluster {
				cl = append(cl, txs[idx].env)
			}
			clusters = append(clusters, cl)
		}
		stages = append(stages, clusters)
	}

	phases := []xdr.TransactionPhase{{
		V:                    1,
		ParallelTxsComponent: &xdr.ParallelTxsComponent{ExecutionStages: stages},
	}}

	return xdr.LedgerCloseMeta{
		V: 2,
		V2: &xdr.LedgerCloseMetaV2{
			LedgerHeader: fixtureHeader(ledgerSeq, closeTime),
			TxSet:        xdr.GeneralizedTransactionSet{V: 1, V1TxSet: &xdr.TransactionSetV1{Phases: phases}},
			TxProcessing: resultMetaV1s(txs),
		},
	}
}

// TestTransactionViewRange_ParallelTxsPhase exercises the V=1 ParallelTxs
// phase (multiple stages, multiple clusters, a multi-tx cluster) with paging
// windows that cross cluster and stage boundaries, asserting wire-parity with
// the parsed reader; TransactionViewByHash is checked for every tx.
func TestTransactionViewRange_ParallelTxsPhase(t *testing.T) {
	txs := make([]txWithHash, 6)
	for i := range txs {
		txs[i] = sorobanTx(t, fmt.Sprintf("ptx-%d", i))
	}
	// Stage0: cluster{tx5,tx4}, cluster{tx3}. Stage1: cluster{tx2,tx1,tx0}.
	// (Layout intentionally not apply order; pairing is by hash.)
	layout := [][][]int{
		{{5, 4}, {3}},
		{{2, 1, 0}},
	}
	lcm := buildParallelTxsLCM(t, 8201, 1_700_040_001, txs, layout)
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	view := xdr.LedgerCloseMetaView(raw)

	oracle := readerOracle(t, lcm)
	require.Len(t, oracle, 6)

	check := func(start, limit int) {
		got, gerr := LedgerTransactionViewRange(view, start, limit, viewTestPassphrase)
		require.NoError(t, gerr)
		want := len(oracle) - start
		if limit > 0 && limit < want {
			want = limit
		}
		require.Len(t, got, want, "page start=%d limit=%d", start, limit)
		for k := range got {
			assertMatchesReader(t, oracle[start+k], got[k], start+k)
		}
	}
	check(0, 0) // full
	check(0, 2) // first cluster only
	check(1, 3) // page crossing the stage0 cluster{5,4}/cluster{3} boundary
	check(3, 0) // start in second stage
	check(2, 3) // crosses stage0->stage1 boundary

	// And TransactionViewByHash for each tx.
	for k := range oracle {
		got, found, byHashErr := LedgerTransactionViewByHash(view, [32]byte(oracle[k].Hash), viewTestPassphrase)
		require.NoError(t, byHashErr)
		require.True(t, found, "tx %d should be found", k)
		assertMatchesReader(t, oracle[k], got, int(oracle[k].Index)-1)
	}
}

// TestLedgerTransactionViewByHash_FeeBumpInnerHash: a fee-bump transaction is
// resolvable by BOTH its hashes — its own (result-pair) hash and the inner
// transaction's — and both lookups land on the same transaction, with the
// envelope still paired by the outer hash.
func TestLedgerTransactionViewByHash_FeeBumpInnerHash(t *testing.T) {
	fb := feeBumpTx(t, vMetaV3Soroban([]xdr.ContractEvent{vContractEvent("fb-inner")}))
	txs := []txWithHash{sorobanTx(t, "a"), fb, sorobanTx(t, "b")}
	lcm := buildLCM(t, 2, 9700, 1_700_060_000, txs, true /*reversed TxSet*/)
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	view := xdr.LedgerCloseMetaView(raw)

	innerHash := innerHashOf(t, fb.env)
	require.NotEqual(t, fb.hash, innerHash, "outer and inner hashes must differ")

	byOuter, found, err := LedgerTransactionViewByHash(view, fb.hash, viewTestPassphrase)
	require.NoError(t, err)
	require.True(t, found, "fee-bump must be resolvable by its outer hash")

	byInner, found, err := LedgerTransactionViewByHash(view, innerHash, viewTestPassphrase)
	require.NoError(t, err)
	require.True(t, found, "fee-bump must be resolvable by its inner hash")

	assert.Equal(t, byOuter.ApplicationOrder, byInner.ApplicationOrder, "both hashes must resolve the same tx")
	assert.True(t, byInner.FeeBump)
	assert.Equal(t, byOuter.Envelope, byInner.Envelope, "inner-hash match must pair the same (outer-keyed) envelope")

	// A non-fee-bump tx has no inner hash: nothing else matches by accident.
	var absent [32]byte
	absent[0] = 0xEE
	_, found, err = LedgerTransactionViewByHash(view, absent, viewTestPassphrase)
	require.NoError(t, err)
	assert.False(t, found)
}

// TestExtractLedgerTxParts_FeeBumpInnerHash: the walk reports the inner hash
// for a fee-bump element and the zero array for everything else.
func TestExtractLedgerTxParts_FeeBumpInnerHash(t *testing.T) {
	fb := feeBumpTx(t, vMetaV3Soroban([]xdr.ContractEvent{vContractEvent("fb-ev")}))
	txs := []txWithHash{sorobanTx(t, "plain"), fb}
	lcm := buildLCM(t, 1, 9701, 1_700_060_100, txs, false)
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)

	txParts, err := ExtractLedgerTxParts(xdr.LedgerCloseMetaView(raw))
	require.NoError(t, err)
	require.Len(t, txParts, 2)

	assert.False(t, txParts[0].FeeBump, "non-fee-bump must not set FeeBump")
	assert.Equal(t, [32]byte{}, txParts[0].InnerHash, "non-fee-bump InnerHash stays the zero array")
	require.True(t, txParts[1].FeeBump, "fee-bump must set FeeBump")
	assert.Equal(t, innerHashOf(t, fb.env), xdr.Hash(txParts[1].InnerHash))
	assert.Equal(t, fb.hash, xdr.Hash(txParts[1].Hash), "Hash stays the outer hash")
}

// ---------------------------------------------------------------------------
// Event-arity differential: the view path vs GetTransactionEvents
// ---------------------------------------------------------------------------

// arityMetaCase is one TransactionMeta shape in the event matrix. A non-empty
// skip means the shape has no decode oracle to differentiate against; it stays
// in the table (as a reported SKIP, not a silent omission) so the exclusion is
// visible next to the shapes that are covered.
type arityMetaCase struct {
	name string
	meta xdr.TransactionMeta
	skip string
}

// arityEnvCase is one envelope shape in the event matrix: the builder wraps the
// meta under test in that envelope, with a matching result.
type arityEnvCase struct {
	name  string
	build func(t testing.TB, meta xdr.TransactionMeta) txWithHash
}

func arityMetaCases() []arityMetaCase {
	return []arityMetaCase{
		// The parsed reader itself ACCEPTS a V0 meta; only the decode event
		// APIs reject it, which is what leaves the row without an oracle.
		{"v0", feeMetaV0(), "TransactionMeta V0 has no decode oracle: GetTransactionEvents and " +
			"GetDiagnosticEvents both error with \"unsupported TransactionMeta version: 0\"; " +
			"the view path's V0 tolerance is pinned by TestTransactionEventsFromMeta_LegacyV0"},
		{"v1", feeMetaV1(), ""},
		{"v2", feeMetaV2(), ""},
		// THE REGRESSION: a V3 meta with NO SorobanMeta. On a soroban
		// envelope this is a transaction that was charged but never executed
		// — a real pubnet shape on protocols 20-22. GetTransactionEvents
		// still reports ONE (empty) operation slot for it.
		{"v3-soroban-meta-absent", feeMetaV3NoSorobanMeta(), ""},
		{"v3-soroban-meta-empty", feeMetaV3Ext0(), ""},
		{"v3-events", vMetaV3Soroban([]xdr.ContractEvent{vContractEvent("v3-a"), vContractEvent("v3-b")}), ""},
		{"v3-diagnostics-only", vMetaV3SorobanWithDiag(nil, []xdr.DiagnosticEvent{vDiagEvent("v3-diag")}), ""},
		{"v3-events-and-diagnostics", vMetaV3SorobanWithDiag(
			[]xdr.ContractEvent{vContractEvent("v3-ev")},
			[]xdr.DiagnosticEvent{vDiagEvent("v3-diag-a"), vDiagEvent("v3-diag-b")}), ""},
		{"v4-no-operations", feeMetaV4NoSorobanMeta(), ""},
		{"v4-op-events", vMetaV4OpEvents([][]xdr.ContractEvent{{vContractEvent("v4-opA")}, {vContractEvent("v4-opB")}}), ""},
		{"v4-full", xdr.TransactionMeta{
			V: 4,
			V4: &xdr.TransactionMetaV4{
				Events: []xdr.TransactionEvent{
					{Stage: xdr.TransactionEventStageTransactionEventStageBeforeAllTxs, Event: vContractEvent("v4-pre")},
					{Stage: xdr.TransactionEventStageTransactionEventStageAfterTx, Event: vContractEvent("v4-post")},
				},
				DiagnosticEvents: []xdr.DiagnosticEvent{vDiagEvent("v4-diag")},
				Operations: []xdr.OperationMetaV2{
					{Events: []xdr.ContractEvent{vContractEvent("v4-a")}},
					{Events: nil},
				},
			},
		}, ""},
	}
}

func arityEnvCases() []arityEnvCase {
	return []arityEnvCase{
		{"classic", func(t testing.TB, m xdr.TransactionMeta) txWithHash {
			return feeTxV1(t, []xdr.Operation{feeBumpSequenceOp()}, false, m, 100)
		}},
		{"soroban", func(t testing.TB, m xdr.TransactionMeta) txWithHash {
			return feeTxV1(t, []xdr.Operation{feeInvokeHostFunctionOp()}, true, m, 100)
		}},
		{"feebump-classic", func(t testing.TB, m xdr.TransactionMeta) txWithHash {
			return feeBumpFeeTx(t, []xdr.Operation{feeBumpSequenceOp()}, false, m, 200)
		}},
		{"feebump-soroban", func(t testing.TB, m xdr.TransactionMeta) txWithHash {
			return feeBumpFeeTx(t, []xdr.Operation{feeInvokeHostFunctionOp()}, true, m, 200)
		}},
		{"envelope-v0", func(t testing.TB, m xdr.TransactionMeta) txWithHash {
			return feeTxV0(t, []xdr.Operation{feeBumpSequenceOp()}, m, 100)
		}},
	}
}

// assertViewEventsMatchDecode asserts the view's raw event fields carry the
// same arity and bytes as LedgerTransaction.GetTransactionEvents' output.
func assertViewEventsMatchDecode(t *testing.T, want LedgerTransaction, got LedgerTransactionView) {
	t.Helper()
	te, err := want.GetTransactionEvents()
	require.NoError(t, err)

	assertRawEventsMatch(t, te.TransactionEvents, got.TransactionEvents, "TransactionEvents")

	require.Len(t, got.ContractEvents, len(te.OperationEvents),
		"per-operation ContractEvents arity must equal GetTransactionEvents' OperationEvents arity")
	for op := range te.OperationEvents {
		assertRawEventsMatch(t, te.OperationEvents[op], got.ContractEvents[op], fmt.Sprintf("ContractEvents[%d]", op))
	}

	// The view's DiagnosticEvents field mirrors the ungated standalone
	// GetDiagnosticEvents (see assertMatchesReader) — asserted for every cell.
	diag, err := want.GetDiagnosticEvents()
	require.NoError(t, err)
	assertRawEventsMatch(t, diag, got.DiagnosticEvents, "DiagnosticEvents")

	// The two decode oracles disagree on exactly one wire-impossible shape: V3
	// SorobanMeta diagnostics under a classic envelope (GetTransactionEvents
	// gates on IsSorobanTx; GetDiagnosticEvents doesn't). The skip is keyed on
	// the oracles differing, so a new divergence anywhere else fails here.
	t.Run("diagnostics-vs-GetTransactionEvents", func(t *testing.T) {
		if len(te.DiagnosticEvents) != len(diag) {
			require.EqualValues(t, 3, want.UnsafeMeta.V,
				"the decode diagnostics APIs may only disagree on a V3 meta")
			require.False(t, want.IsSorobanTx(),
				"the decode diagnostics APIs may only disagree under a classic envelope")
			t.Skipf("decode APIs disagree by design on this shape: GetTransactionEvents reports %d "+
				"diagnostic events (its V3 arm is gated on IsSorobanTx), GetDiagnosticEvents reports %d; "+
				"pinned by TestTransactionEventsFromMeta_V3GateIsCallerResponsibility",
				len(te.DiagnosticEvents), len(diag))
		}
		assertRawEventsMatch(t, te.DiagnosticEvents, got.DiagnosticEvents,
			"DiagnosticEvents (GetTransactionEvents oracle)")
	})
}

// TestTransactionView_EventsAgreeWithGetTransactionEvents pins the view path
// (LedgerTransactionViewRange / LedgerTransactionViewByHash) against
// LedgerTransaction.GetTransactionEvents: for every LCM version × meta shape ×
// envelope shape, both must report the same arity and bytes for
// TransactionEvents, per-operation ContractEvents, and DiagnosticEvents.
// Shapes with no decode oracle are reported as SKIPs, not omitted.
func TestTransactionView_EventsAgreeWithGetTransactionEvents(t *testing.T) {
	envCases := arityEnvCases()
	for _, lcmVersion := range []int32{0, 1, 2} {
		for _, mc := range arityMetaCases() {
			t.Run(fmt.Sprintf("lcmV%d/meta-%s", lcmVersion, mc.name), func(t *testing.T) {
				if mc.skip != "" {
					t.Skip(mc.skip)
				}
				txs := make([]txWithHash, len(envCases))
				for i, ec := range envCases {
					txs[i] = ec.build(t, mc.meta)
				}
				// Reversed TxSet: apply order (TxProcessing) differs from
				// agreed-set order, so a mispairing would surface here too.
				// Every cell builds its own LCM, so one ledger sequence serves
				// them all; the subtest name identifies the cell.
				lcm := buildLCM(t, lcmVersion, 9800, 1_700_080_000, txs, true /*reversed TxSet*/)
				raw, err := lcm.MarshalBinary()
				require.NoError(t, err)
				view := xdr.LedgerCloseMetaView(raw)

				oracle := readerOracle(t, lcm)
				require.Len(t, oracle, len(txs))

				got, err := LedgerTransactionViewRange(view, 0, 0, viewTestPassphrase)
				require.NoError(t, err)
				require.Len(t, got, len(oracle))

				for k, ec := range envCases {
					t.Run(ec.name, func(t *testing.T) {
						t.Run("range", func(t *testing.T) {
							assertMatchesReader(t, oracle[k], got[k], k)
							assertViewEventsMatchDecode(t, oracle[k], got[k])
						})
						// The by-hash entry point must agree with the range
						// entry point — they share the assembly, and this
						// keeps that shared-ness honest.
						t.Run("byhash", func(t *testing.T) {
							byHash, found, byHashErr := LedgerTransactionViewByHash(
								view, [32]byte(oracle[k].Hash), viewTestPassphrase)
							require.NoError(t, byHashErr)
							require.True(t, found)
							assertViewEventsMatchDecode(t, oracle[k], byHash)
						})
					})
				}
			})
		}
	}
}

// TestTransactionView_V3SorobanMetaAbsent_OneEmptyOperationSlot is the named
// regression case behind the differential above: V3 meta + Soroban envelope +
// absent SorobanMeta must yield ONE empty operation slot ([[]]), matching
// GetTransactionEvents; the view path used to report zero ([]).
func TestTransactionView_V3SorobanMetaAbsent_OneEmptyOperationSlot(t *testing.T) {
	// Third fixture: a result whose code carries no per-operation list at all
	// (charged, never ran). Neither event API reads the result, so it must
	// land on the same one empty slot.
	charged := feeTxV1(t, []xdr.Operation{feeInvokeHostFunctionOp()}, true, feeMetaV3NoSorobanMeta(), 100)
	chargedRes := feeNoOpListResult(100, xdr.TransactionResultCodeTxInternalError)
	charged.result = &chargedRes

	txs := []txWithHash{
		feeTxV1(t, []xdr.Operation{feeInvokeHostFunctionOp()}, true, feeMetaV3NoSorobanMeta(), 100),
		feeTxV1(t, []xdr.Operation{feeBumpSequenceOp()}, false, feeMetaV3NoSorobanMeta(), 100),
		charged,
	}
	lcm := buildLCM(t, 2, 9900, 1_700_090_000, txs, true /*reversed TxSet*/)
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)

	oracle := readerOracle(t, lcm)
	require.Len(t, oracle, 3)
	require.True(t, oracle[0].IsSorobanTx(), "fixture 0 must be a soroban tx")
	require.False(t, oracle[1].IsSorobanTx(), "fixture 1 must be a classic tx")
	require.True(t, oracle[2].IsSorobanTx(), "fixture 2 must be a soroban tx")
	require.False(t, oracle[2].Result.Result.Successful(), "fixture 2 must have failed")

	got, err := LedgerTransactionViewRange(xdr.LedgerCloseMetaView(raw), 0, 0, viewTestPassphrase)
	require.NoError(t, err)
	require.Len(t, got, 3)

	// Soroban envelope: one empty operation slot on BOTH paths.
	sorobanDecoded, err := oracle[0].GetTransactionEvents()
	require.NoError(t, err)
	require.Len(t, sorobanDecoded.OperationEvents, 1, "decode path reports one operation slot")
	require.Empty(t, sorobanDecoded.OperationEvents[0], "decode path's slot is empty")
	require.Len(t, got[0].ContractEvents, 1, "view path must report the same single operation slot")
	assert.Empty(t, got[0].ContractEvents[0], "view path's slot must be empty")

	// Classic envelope: the V3 soroban gate still applies — no slots at all,
	// on both paths.
	classicDecoded, err := oracle[1].GetTransactionEvents()
	require.NoError(t, err)
	assert.Empty(t, classicDecoded.OperationEvents, "decode path gates V3 events for a classic tx")
	assert.Empty(t, got[1].ContractEvents, "view path must gate them the same way")

	// Charged but never executed, with the matching result code: still one
	// empty slot on both paths.
	chargedDecoded, err := oracle[2].GetTransactionEvents()
	require.NoError(t, err)
	require.Len(t, chargedDecoded.OperationEvents, 1, "decode path reports one operation slot")
	require.Len(t, got[2].ContractEvents, 1, "view path must report the same single operation slot")
	assert.Empty(t, got[2].ContractEvents[0], "view path's slot must be empty")
}
