package ingest

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// The extractors must return exactly what a walk written only with the
// generated accessors finds: the same wire elements (same bytes at the same
// offsets), the same empty-versus-nil shapes and the same fee buckets, on every
// fixture ledger. Only the exported API is exercised.

type refTx struct {
	hash, innerHash [32]byte
	feeBump         bool
	result          xdr.TransactionResultPairView
	meta            xdr.TransactionMetaView
	txEvents        []xdr.TransactionEventView
	opEvents        [][]xdr.ContractEventView
	diagnostics     []xdr.DiagnosticEventView
	sorobanMeta     bool
	feeExt          bool
	resourceFee     int64
}

func refLedger(t *testing.T, raw []byte) []refTx {
	t.Helper()
	lcm := xdr.LedgerCloseMetaView(raw)
	var out []refTx
	add := func(result xdr.TransactionResultPairView, meta xdr.TransactionMetaView) {
		out = append(out, refTransaction(t, result, meta))
	}
	switch lcm.MustV() {
	case 0:
		for _, elem := range lcm.MustV0().MustTxProcessing().MustAll() {
			f, err := elem.Fields()
			require.NoError(t, err)
			add(f.Result, f.TxApplyProcessing)
		}
	case 1:
		for _, elem := range lcm.MustV1().MustTxProcessing().MustAll() {
			f, err := elem.Fields()
			require.NoError(t, err)
			add(f.Result, f.TxApplyProcessing)
		}
	case 2:
		for _, elem := range lcm.MustV2().MustTxProcessing().MustAll() {
			f, err := elem.Fields()
			require.NoError(t, err)
			add(f.Result, f.TxApplyProcessing)
		}
	default:
		t.Fatalf("unsupported LCM version")
	}
	return out
}

func refTransaction(t *testing.T, result xdr.TransactionResultPairView, meta xdr.TransactionMetaView) refTx {
	t.Helper()
	tx := refTx{
		hash:        [32]byte(result.MustTransactionHash().MustValue()),
		result:      result,
		meta:        meta,
		txEvents:    []xdr.TransactionEventView{},
		opEvents:    [][]xdr.ContractEventView{},
		diagnostics: []xdr.DiagnosticEventView{},
	}
	res := result.MustResult().MustResult()
	switch res.MustCode() {
	case xdr.TransactionResultCodeTxFeeBumpInnerSuccess, xdr.TransactionResultCodeTxFeeBumpInnerFailed:
		tx.feeBump = true
		tx.innerHash = [32]byte(res.MustInnerResultPair().MustTransactionHash().MustValue())
	default:
	}

	var ext xdr.SorobanTransactionMetaExtView
	switch meta.MustV() {
	case 3:
		sm, ok := meta.MustV3().MustSorobanMeta().MustUnwrap()
		if !ok {
			break
		}
		tx.sorobanMeta = true
		ext = sm.MustExt()
		tx.opEvents = [][]xdr.ContractEventView{sm.MustEvents().MustAll()}
		tx.diagnostics = sm.MustDiagnosticEvents().MustAll()
	case 4:
		v4 := meta.MustV4()
		ops := v4.MustOperations()
		tx.opEvents = make([][]xdr.ContractEventView, ops.MustCount())
		for k := range tx.opEvents {
			tx.opEvents[k] = ops.MustAt(k).MustEvents().MustAll()
		}
		tx.txEvents = v4.MustEvents().MustAll()
		tx.diagnostics = v4.MustDiagnosticEvents().MustAll()
		if sm, ok := v4.MustSorobanMeta().MustUnwrap(); ok {
			tx.sorobanMeta = true
			ext = sm.MustExt()
		}
	default:
	}
	if tx.sorobanMeta && ext.MustV() == 1 {
		v1 := ext.MustV1()
		tx.feeExt = true
		tx.resourceFee = v1.MustTotalNonRefundableResourceFeeCharged().MustValue() +
			v1.MustTotalRefundableResourceFeeCharged().MustValue()
	}
	return tx
}

func refFees(txs []refTx) (classic, soroban []uint64) {
	for _, tx := range txs {
		//nolint:gosec // fixture fees are non-negative
		feeCharged := uint64(tx.result.MustResult().MustFeeCharged().MustValue())
		if tx.sorobanMeta {
			if tx.feeExt {
				//nolint:gosec // fixture resource fees never exceed the fee charged
				soroban = append(soroban, feeCharged-uint64(tx.resourceFee))
			}
			continue
		}
		res := tx.result.MustResult().MustResult()
		var ops int
		switch res.MustCode() {
		case xdr.TransactionResultCodeTxSuccess, xdr.TransactionResultCodeTxFailed:
			ops = res.MustResults().MustCount()
		case xdr.TransactionResultCodeTxFeeBumpInnerSuccess, xdr.TransactionResultCodeTxFeeBumpInnerFailed:
			inner := res.MustInnerResultPair().MustResult().MustResult()
			switch inner.MustCode() {
			case xdr.TransactionResultCodeTxSuccess, xdr.TransactionResultCodeTxFailed:
				ops = inner.MustResults().MustCount()
			default:
			}
		default:
		}
		if ops > 0 {
			classic = append(classic, feeCharged/uint64(ops))
		}
	}
	return classic, soroban
}

// assertSameElements asserts got holds the very wire elements want holds:
// the same count, the same nil-ness, and each element the same length at the
// same address, which also makes the bytes equal.
func assertSameElements[W, G ~[]byte](t *testing.T, want []W, got []G, ctx string) {
	t.Helper()
	require.Equal(t, want == nil, got == nil, "%s: nil-ness", ctx)
	require.Len(t, got, len(want), "%s: count", ctx)
	for i := range want {
		require.Len(t, got[i], len(want[i]), "%s[%d]: length", ctx, i)
		require.NotEmpty(t, got[i], "%s[%d]", ctx, i)
		assert.True(t, &want[i][0] == &got[i][0], "%s[%d]: must alias its own wire element", ctx, i)
	}
}

func assertSameGroups[W, G ~[]byte](t *testing.T, want [][]W, got [][]G, ctx string) {
	t.Helper()
	require.Equal(t, want == nil, got == nil, "%s: nil-ness", ctx)
	require.Len(t, got, len(want), "%s: groups", ctx)
	for k := range want {
		assertSameElements(t, want[k], got[k], fmt.Sprintf("%s[%d]", ctx, k))
	}
}

func assertSameView[W, G ~[]byte](t *testing.T, want W, got G, ctx string) {
	t.Helper()
	require.Len(t, got, len(want), ctx)
	require.NotEmpty(t, got, ctx)
	assert.True(t, &want[0] == &got[0], "%s: must alias the ledger buffer", ctx)
}

// envelopeIsSoroban mirrors LedgerTransaction.IsSorobanTx on an envelope view.
func envelopeIsSoroban(t *testing.T, env []byte) bool {
	t.Helper()
	e := xdr.TransactionEnvelopeView(env)
	switch e.MustType() {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		return e.MustV1().MustTx().MustExt().MustV() == 1
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		return e.MustFeeBump().MustTx().MustInnerTx().MustV1().MustTx().MustExt().MustV() == 1
	default:
		return false
	}
}

// assertLedgerMatchesReference runs every exported extractor over raw and
// checks each against the generated-accessor reference.
func assertLedgerMatchesReference(t *testing.T, raw []byte, passphrase string) {
	t.Helper()
	view := xdr.LedgerCloseMetaView(raw)
	ref := refLedger(t, raw)

	parts, err := ExtractLedgerTxParts(view)
	require.NoError(t, err)
	require.Len(t, parts, len(ref))
	for i, tx := range ref {
		ctx := fmt.Sprintf("tx %d", i)
		assert.Equal(t, tx.hash, parts[i].Hash, ctx)
		assert.Equal(t, tx.innerHash, parts[i].InnerHash, ctx)
		assert.Equal(t, tx.feeBump, parts[i].FeeBump, ctx)
		assertSameView(t, tx.result, parts[i].Result, ctx+" Result")
		assertSameView(t, tx.meta, parts[i].Meta, ctx+" Meta")
	}

	events, err := EventsFromTxParts(parts)
	require.NoError(t, err)
	require.Len(t, events, len(ref))
	for i, tx := range ref {
		ctx := fmt.Sprintf("tx %d", i)
		assertSameElements(t, tx.txEvents, events[i].TransactionEvents, ctx+" TransactionEvents")
		assertSameGroups(t, tx.opEvents, events[i].OperationEvents, ctx+" OperationEvents")
	}

	fees, err := FeesFromTxParts(parts)
	require.NoError(t, err)
	wantClassic, wantSoroban := refFees(ref)
	assert.Equal(t, wantClassic, fees.ClassicFeesPerOp, "ClassicFeesPerOp")
	assert.Equal(t, wantSoroban, fees.SorobanInclusionFees, "SorobanInclusionFees")

	all, err := LedgerTransactionViewRange(view, 0, 0, passphrase)
	require.NoError(t, err)
	if len(ref) == 0 {
		assert.Empty(t, all)
		return
	}
	require.Len(t, all, len(ref))
	for i, tx := range ref {
		byHash, found, err := LedgerTransactionViewByHash(view, tx.hash, passphrase)
		require.NoError(t, err)
		require.True(t, found, "tx %d", i)
		for name, got := range map[string]LedgerTransactionView{"range": all[i], "by-hash": byHash} {
			ctx := fmt.Sprintf("tx %d %s", i, name)
			assertSameView(t, tx.result.MustResult().MustRaw(), got.Result, ctx+" Result")
			assertSameView(t, tx.meta, got.Meta, ctx+" Meta")
			assertSameElements(t, tx.diagnostics, got.DiagnosticEvents, ctx+" DiagnosticEvents")
			assertSameElements(t, tx.txEvents, got.TransactionEvents, ctx+" TransactionEvents")

			wantOps := tx.opEvents
			if tx.meta.MustV() == 3 {
				switch {
				case !envelopeIsSoroban(t, got.Envelope):
					wantOps = [][]xdr.ContractEventView{}
				case len(wantOps) == 0:
					wantOps = [][]xdr.ContractEventView{{}}
				}
			}
			assertSameGroups(t, wantOps, got.ContractEvents, ctx+" ContractEvents")
		}
	}
}

// equivalenceV4Meta is a V4 meta with a ledger-entry spine on every operation,
// so a walk that steps an operation by the wrong amount lands inside it.
func equivalenceV4Meta(opEvents []int, txEvents, diagnostics int, soroban bool) xdr.TransactionMeta {
	changes := func(seed byte) xdr.LedgerEntryChanges {
		out := make(xdr.LedgerEntryChanges, 2)
		for i := range out {
			var ed xdr.Uint256
			ed[0], ed[31] = seed, byte(i)
			key := xdr.LedgerKey{Type: xdr.LedgerEntryTypeAccount, Account: &xdr.LedgerKeyAccount{
				AccountId: xdr.AccountId{Type: xdr.PublicKeyTypePublicKeyTypeEd25519, Ed25519: &ed},
			}}
			out[i] = xdr.LedgerEntryChange{Type: xdr.LedgerEntryChangeTypeLedgerEntryRemoved, Removed: &key}
		}
		return out
	}
	v4 := &xdr.TransactionMetaV4{TxChangesBefore: changes(0xb0), TxChangesAfter: changes(0xa0)}
	for i, n := range opEvents {
		op := xdr.OperationMetaV2{Changes: changes(byte(i + 1))}
		for j := range n {
			op.Events = append(op.Events, vContractEvent(fmt.Sprintf("op%d-%d", i, j)))
		}
		v4.Operations = append(v4.Operations, op)
	}
	for i := range txEvents {
		v4.Events = append(v4.Events, xdr.TransactionEvent{
			Stage: xdr.TransactionEventStageTransactionEventStageAfterTx,
			Event: vContractEvent(fmt.Sprintf("tx-%d", i)),
		})
	}
	for i := range diagnostics {
		v4.DiagnosticEvents = append(v4.DiagnosticEvents, vDiagEvent(fmt.Sprintf("diag-%d", i)))
	}
	if soroban {
		rv := xdr.ScVal{Type: xdr.ScValTypeScvVoid}
		v4.SorobanMeta = &xdr.SorobanTransactionMetaV2{Ext: feeSorobanExtV1(30, 20), ReturnValue: &rv}
	}
	return xdr.TransactionMeta{V: 4, V4: v4}
}

func equivalenceMetas() []xdr.TransactionMeta {
	metas := []xdr.TransactionMeta{
		equivalenceV4Meta(nil, 0, 0, false),
		equivalenceV4Meta([]int{0}, 0, 0, false),
		equivalenceV4Meta([]int{3, 0, 5, 1, 0, 2}, 3, 3, true),
		equivalenceV4Meta([]int{0, 0, 0}, 1, 0, false),
		equivalenceV4Meta([]int{7, 7}, 0, 4, true),
		equivalenceV4Meta(nil, 2, 2, true),
		vMetaV3SorobanWithDiag(
			[]xdr.ContractEvent{vContractEvent("a"), vContractEvent("b")},
			[]xdr.DiagnosticEvent{vDiagEvent("d0"), vDiagEvent("d1"), vDiagEvent("d2")}),
		vMetaV3SorobanWithDiag(nil, []xdr.DiagnosticEvent{vDiagEvent("d0")}),
		vMetaV3Soroban(nil),
		feeMetaV3Ext1(40, 10),
	}
	for _, c := range arityMetaCases() {
		metas = append(metas, c.meta)
	}
	return metas
}

// equivalenceLedgers is every fixture ledger shape the package builds, per LCM
// version where the shape allows it.
func equivalenceLedgers(t *testing.T) map[string][]byte {
	t.Helper()
	marshal := func(lcm xdr.LedgerCloseMeta) []byte {
		raw, err := lcm.MarshalBinary()
		require.NoError(t, err)
		return raw
	}
	envCases := arityEnvCases()
	fees := feeMatrixCases(t)
	feeTxs := make([]txWithHash, len(fees))
	for i, c := range fees {
		feeTxs[i] = c.tx
	}
	ledgers := map[string][]byte{}
	for _, version := range []int32{0, 1, 2} {
		var txs []txWithHash
		for _, meta := range equivalenceMetas() {
			for _, ec := range envCases {
				txs = append(txs, ec.build(t, meta))
			}
		}
		fb := feeBumpTx(t, vMetaV3Soroban([]xdr.ContractEvent{vContractEvent("fb")}))
		txs = append(txs, fb, txV0(t, nil, 3))
		ledgers[fmt.Sprintf("lcmV%d/shapes", version)] = marshal(buildLCM(t, version, 7100, 1_700_000_000, txs, true))
		ledgers[fmt.Sprintf("lcmV%d/fees", version)] = marshal(buildLCM(t, version, 7200, 1_700_000_000, feeTxs, false))
		ledgers[fmt.Sprintf("lcmV%d/empty", version)] = marshal(buildLCM(t, version, 7300, 1_700_000_000, nil, false))
	}
	parallel := make([]txWithHash, 6)
	for i := range parallel {
		parallel[i] = feeTxV1(t, []xdr.Operation{feeInvokeHostFunctionOp()}, true,
			equivalenceV4Meta([]int{i, 1}, i%3, i%2, true), 100)
	}
	ledgers["lcmV2/parallel"] = marshal(buildParallelTxsLCM(t, 7400, 1_700_000_000, parallel, [][][]int{{{5, 4}, {3}}, {{2, 1, 0}}}))
	for _, version := range []int32{1, 2} {
		ledgers[fmt.Sprintf("lcmV%d/multi-phase", version)] = marshal(buildMultiPhaseLCM(t, version, 7500, 1_700_000_000, parallel, 2))
	}
	return ledgers
}

func TestExtractors_MatchGeneratedAccessorReference(t *testing.T) {
	for name, raw := range equivalenceLedgers(t) {
		t.Run(name, func(t *testing.T) {
			assertLedgerMatchesReference(t, raw, viewTestPassphrase)
		})
	}
}

func TestExtractors_MatchGeneratedAccessorReference_RealLedger(t *testing.T) {
	raw, err := os.ReadFile("../xdr/testdata/ledger_58752000.bin")
	require.NoError(t, err)
	assertLedgerMatchesReference(t, raw, network.PublicNetworkPassphrase)
}
