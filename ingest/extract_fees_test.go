package ingest

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// ---------------------------------------------------------------------------
// Fixture builders
// ---------------------------------------------------------------------------

func feeBumpSequenceOp() xdr.Operation {
	return xdr.Operation{Body: xdr.OperationBody{
		Type:           xdr.OperationTypeBumpSequence,
		BumpSequenceOp: &xdr.BumpSequenceOp{BumpTo: 1},
	}}
}

func feeInvokeHostFunctionOp() xdr.Operation {
	var contractID xdr.ContractId
	contractID[0] = 0xfe
	return xdr.Operation{Body: xdr.OperationBody{
		Type: xdr.OperationTypeInvokeHostFunction,
		InvokeHostFunctionOp: &xdr.InvokeHostFunctionOp{
			HostFunction: xdr.HostFunction{
				Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
				InvokeContract: &xdr.InvokeContractArgs{
					ContractAddress: xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeContract, ContractId: &contractID},
					FunctionName:    "noop",
				},
			},
		},
	}}
}

func feeExtendFootprintTtlOp() xdr.Operation {
	return xdr.Operation{Body: xdr.OperationBody{
		Type:                 xdr.OperationTypeExtendFootprintTtl,
		ExtendFootprintTtlOp: &xdr.ExtendFootprintTtlOp{ExtendTo: 100},
	}}
}

func feeRestoreFootprintOp() xdr.Operation {
	return xdr.Operation{Body: xdr.OperationBody{
		Type:               xdr.OperationTypeRestoreFootprint,
		RestoreFootprintOp: &xdr.RestoreFootprintOp{},
	}}
}

// feeOpResults builds n per-operation results. The classification only counts
// them, so the void opNOT_SUPPORTED arm keeps the fixtures minimal.
func feeOpResults(n int) []xdr.OperationResult {
	ops := make([]xdr.OperationResult, n)
	for i := range ops {
		ops[i] = xdr.OperationResult{Code: xdr.OperationResultCodeOpNotSupported}
	}
	return ops
}

// feeResult is a TransactionResult with a configurable FeeCharged and
// opCount per-operation results — the count FeesFromTxParts derives the
// operation count from.
func feeResult(feeCharged int64, success bool, opCount int) xdr.TransactionResult {
	code := xdr.TransactionResultCodeTxFailed
	if success {
		code = xdr.TransactionResultCodeTxSuccess
	}
	ops := feeOpResults(opCount)
	return xdr.TransactionResult{
		FeeCharged: xdr.Int64(feeCharged),
		Result:     xdr.TransactionResultResult{Code: code, Results: &ops},
	}
}

// feeInternalErrorResult is a txINTERNAL_ERROR result — the one applied-tx
// result code with no per-operation list at all.
func feeInternalErrorResult(feeCharged int64) xdr.TransactionResult {
	return xdr.TransactionResult{
		FeeCharged: xdr.Int64(feeCharged),
		Result:     xdr.TransactionResultResult{Code: xdr.TransactionResultCodeTxInternalError},
	}
}

// feeFeeBumpResult is a txFEE_BUMP_INNER_SUCCESS (or _FAILED) result with a
// configurable (outer) FeeCharged and innerOpCount INNER per-operation
// results.
func feeFeeBumpResult(innerHash xdr.Hash, feeCharged int64, innerSuccess bool, innerOpCount int) xdr.TransactionResult {
	outerCode := xdr.TransactionResultCodeTxFeeBumpInnerFailed
	innerCode := xdr.TransactionResultCodeTxFailed
	if innerSuccess {
		outerCode = xdr.TransactionResultCodeTxFeeBumpInnerSuccess
		innerCode = xdr.TransactionResultCodeTxSuccess
	}
	ops := feeOpResults(innerOpCount)
	return xdr.TransactionResult{
		FeeCharged: xdr.Int64(feeCharged),
		Result: xdr.TransactionResultResult{
			Code: outerCode,
			InnerResultPair: &xdr.InnerTransactionResultPair{
				TransactionHash: innerHash,
				Result: xdr.InnerTransactionResult{Result: xdr.InnerTransactionResultResult{
					Code: innerCode, Results: &ops,
				}},
			},
		},
	}
}

// feeBumpInnerInternalErrorResult is a fee-bump result whose INNER result is
// txINTERNAL_ERROR — no inner per-operation list.
func feeBumpInnerInternalErrorResult(innerHash xdr.Hash, feeCharged int64) xdr.TransactionResult {
	return xdr.TransactionResult{
		FeeCharged: xdr.Int64(feeCharged),
		Result: xdr.TransactionResultResult{
			Code: xdr.TransactionResultCodeTxFeeBumpInnerFailed,
			InnerResultPair: &xdr.InnerTransactionResultPair{
				TransactionHash: innerHash,
				Result: xdr.InnerTransactionResult{Result: xdr.InnerTransactionResultResult{
					Code: xdr.TransactionResultCodeTxInternalError,
				}},
			},
		},
	}
}

// feeTxV1 is a V1-envelope transaction: ops + meta + a success result carrying
// feeCharged, with one per-operation result per envelope op. sorobanEnv
// attaches an (empty) SorobanTransactionData — the envelope soroban flag,
// which the classification never reads.
func feeTxV1(t testing.TB, ops []xdr.Operation, sorobanEnv bool, meta xdr.TransactionMeta, feeCharged int64) txWithHash {
	t.Helper()
	tx := xdr.Transaction{
		SourceAccount: xdr.MustMuxedAddress(keypair.MustRandom().Address()),
		Operations:    ops,
	}
	if sorobanEnv {
		tx.Ext = xdr.TransactionExt{V: 1, SorobanData: &xdr.SorobanTransactionData{}}
	}
	env := xdr.TransactionEnvelope{Type: xdr.EnvelopeTypeEnvelopeTypeTx, V1: &xdr.TransactionV1Envelope{Tx: tx}}
	hash, err := network.HashTransactionInEnvelope(env, viewTestPassphrase)
	require.NoError(t, err)
	res := feeResult(feeCharged, true, len(ops))
	return txWithHash{env: env, hash: hash, meta: meta, result: &res}
}

// feeTxV0 is feeTxV1 on a legacy TX_V0 envelope.
func feeTxV0(t testing.TB, ops []xdr.Operation, meta xdr.TransactionMeta, feeCharged int64) txWithHash {
	t.Helper()
	accID := xdr.MustAddress(keypair.MustRandom().Address())
	env := xdr.TransactionEnvelope{Type: xdr.EnvelopeTypeEnvelopeTypeTxV0, V0: &xdr.TransactionV0Envelope{
		Tx: xdr.TransactionV0{
			SourceAccountEd25519: *accID.Ed25519,
			Fee:                  100,
			Operations:           ops,
			Memo:                 xdr.Memo{Type: xdr.MemoTypeMemoNone},
		},
	}}
	hash, err := network.HashTransactionInEnvelope(env, viewTestPassphrase)
	require.NoError(t, err)
	res := feeResult(feeCharged, true, len(ops))
	return txWithHash{env: env, hash: hash, meta: meta, result: &res}
}

// feeBumpFeeTx wraps the ops in a fee-bump envelope; outerFeeCharged lands in
// the OUTER result (the value the classification reads), while the
// per-operation results — the count it reads — are the INNER result's.
func feeBumpFeeTx(t testing.TB, ops []xdr.Operation, sorobanEnv bool, meta xdr.TransactionMeta, outerFeeCharged int64) txWithHash {
	t.Helper()
	inner := xdr.Transaction{
		SourceAccount: xdr.MustMuxedAddress(keypair.MustRandom().Address()),
		Operations:    ops,
	}
	if sorobanEnv {
		inner.Ext = xdr.TransactionExt{V: 1, SorobanData: &xdr.SorobanTransactionData{}}
	}
	env := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTxFeeBump,
		FeeBump: &xdr.FeeBumpTransactionEnvelope{Tx: xdr.FeeBumpTransaction{
			Fee:       55555,
			FeeSource: xdr.MustMuxedAddress(keypair.MustRandom().Address()),
			InnerTx: xdr.FeeBumpTransactionInnerTx{
				Type: xdr.EnvelopeTypeEnvelopeTypeTx,
				V1:   &xdr.TransactionV1Envelope{Tx: inner},
			},
		}},
	}
	hash, err := network.HashTransactionInEnvelope(env, viewTestPassphrase)
	require.NoError(t, err)
	res := feeFeeBumpResult(innerHashOf(t, env), outerFeeCharged, true, len(ops))
	return txWithHash{env: env, hash: hash, meta: meta, result: &res}
}

func feeMetaV0() xdr.TransactionMeta {
	return xdr.TransactionMeta{V: 0, Operations: &[]xdr.OperationMeta{}}
}

func feeMetaV1() xdr.TransactionMeta {
	return xdr.TransactionMeta{V: 1, V1: &xdr.TransactionMetaV1{}}
}

func feeMetaV2() xdr.TransactionMeta {
	return xdr.TransactionMeta{V: 2, V2: &xdr.TransactionMetaV2{}}
}

func feeSorobanExtV1(nonRefundable, refundable int64) xdr.SorobanTransactionMetaExt {
	return xdr.SorobanTransactionMetaExt{V: 1, V1: &xdr.SorobanTransactionMetaExtV1{
		TotalNonRefundableResourceFeeCharged: xdr.Int64(nonRefundable),
		TotalRefundableResourceFeeCharged:    xdr.Int64(refundable),
	}}
}

func feeMetaV3Ext1(nonRefundable, refundable int64) xdr.TransactionMeta {
	return xdr.TransactionMeta{V: 3, V3: &xdr.TransactionMetaV3{SorobanMeta: &xdr.SorobanTransactionMeta{
		Ext:         feeSorobanExtV1(nonRefundable, refundable),
		ReturnValue: xdr.ScVal{Type: xdr.ScValTypeScvVoid},
	}}}
}

func feeMetaV3Ext0() xdr.TransactionMeta {
	return xdr.TransactionMeta{V: 3, V3: &xdr.TransactionMetaV3{SorobanMeta: &xdr.SorobanTransactionMeta{
		ReturnValue: xdr.ScVal{Type: xdr.ScValTypeScvVoid},
	}}}
}

func feeMetaV3NoSorobanMeta() xdr.TransactionMeta {
	return xdr.TransactionMeta{V: 3, V3: &xdr.TransactionMetaV3{}}
}

func feeMetaV4Ext1(nonRefundable, refundable int64) xdr.TransactionMeta {
	return xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{SorobanMeta: &xdr.SorobanTransactionMetaV2{
		Ext: feeSorobanExtV1(nonRefundable, refundable),
	}}}
}

func feeMetaV4Ext0() xdr.TransactionMeta {
	return xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{SorobanMeta: &xdr.SorobanTransactionMetaV2{}}}
}

func feeMetaV4NoSorobanMeta() xdr.TransactionMeta {
	return xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{}}
}

// ---------------------------------------------------------------------------
// Synthetic matrix
// ---------------------------------------------------------------------------

// feeCase annotates one fixture with its expected classification — the
// production feeBucket enum names the bucket.
type feeCase struct {
	name   string
	tx     txWithHash
	bucket feeBucket
	value  uint64
}

// feeMatrixCases is one transaction per cell of the classification matrix:
// {LCM version is applied by the caller} × TxMeta version (0–4) × envelope
// type (V0/V1/fee-bump) × shape (classic single/multi-op, soroban with/without
// the fee ext, missing SorobanMeta, 0-op, failed, txINTERNAL_ERROR).
//
// The classification reads ONLY TxProcessing: the soroban gate is SorobanMeta
// presence in the meta, and the op count is the number of per-operation
// results. Several cells pin exactly that — shapes where the envelope would
// say one thing and TxProcessing another (impossible on ledgers a correctly
// functioning core produces, where SorobanMeta present ⟺ soroban tx and the
// result carries one per-op entry per envelope op).
func feeMatrixCases(t testing.TB) []feeCase {
	t.Helper()
	invoke := feeInvokeHostFunctionOp()
	extend := feeExtendFootprintTtlOp()
	restore := feeRestoreFootprintOp()
	bump := feeBumpSequenceOp()

	failedClassic := feeTxV1(t, []xdr.Operation{bump, bump}, false, feeMetaV1(), 100)
	failedRes := feeResult(100, false, 2)
	failedClassic.result = &failedRes

	failedSoroban := feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(40, 10), 100)
	failedSorobanRes := feeResult(100, false, 1)
	failedSoroban.result = &failedSorobanRes

	innerFailedFeeBump := feeBumpFeeTx(t, []xdr.Operation{bump}, false, feeMetaV1(), 200)
	innerFailedRes := feeFeeBumpResult(innerHashOf(t, innerFailedFeeBump.env), 200, false, 1)
	innerFailedFeeBump.result = &innerFailedRes

	// The result's per-op count is authoritative: one envelope op, three
	// per-op results → classic fee is divided by three.
	resultsAuthoritative := feeTxV1(t, []xdr.Operation{bump}, false, feeMetaV1(), 99)
	threeOpRes := feeResult(99, true, 3)
	resultsAuthoritative.result = &threeOpRes

	internalError := feeTxV1(t, []xdr.Operation{bump}, false, feeMetaV1(), 100)
	internalErrorRes := feeInternalErrorResult(100)
	internalError.result = &internalErrorRes

	innerInternalErrorFeeBump := feeBumpFeeTx(t, []xdr.Operation{bump}, false, feeMetaV1(), 200)
	innerInternalErrorRes := feeBumpInnerInternalErrorResult(innerHashOf(t, innerInternalErrorFeeBump.env), 200)
	innerInternalErrorFeeBump.result = &innerInternalErrorRes

	return []feeCase{
		{"classic_single_op_metaV1", feeTxV1(t, []xdr.Operation{bump}, false, feeMetaV1(), 100), feeBucketClassic, 100},
		{"classic_multi_op_rounds_down_metaV2", feeTxV1(t, []xdr.Operation{bump, bump, bump}, false, feeMetaV2(), 100), feeBucketClassic, 33},
		{"classic_single_op_v0_envelope_metaV0", feeTxV0(t, []xdr.Operation{bump}, feeMetaV0(), 100), feeBucketClassic, 100},
		{"classic_multi_op_v0_envelope", feeTxV0(t, []xdr.Operation{bump, bump}, feeMetaV0(), 101), feeBucketClassic, 50},
		{"soroban_invoke_v3_ext1", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(40, 10), 100), feeBucketSoroban, 50},
		{"soroban_extend_v3_ext1", feeTxV1(t, []xdr.Operation{extend}, true, feeMetaV3Ext1(30, 10), 100), feeBucketSoroban, 60},
		{"soroban_restore_v3_ext1", feeTxV1(t, []xdr.Operation{restore}, true, feeMetaV3Ext1(20, 10), 100), feeBucketSoroban, 70},
		{"soroban_meta_v3_ext0_skipped", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext0(), 100), feeBucketNone, 0},
		{"soroban_invoke_v4_ext1", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV4Ext1(25, 15), 100), feeBucketSoroban, 60},
		{"soroban_meta_v4_ext0_skipped", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV4Ext0(), 100), feeBucketNone, 0},
		// The soroban gate is SorobanMeta presence, nothing else. A
		// soroban-typed op whose meta carries NO SorobanMeta is classic (v1,
		// gating on the op type, skipped these).
		{"invoke_op_no_soroban_meta_v3_classic", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3NoSorobanMeta(), 100), feeBucketClassic, 100},
		{"invoke_op_no_soroban_meta_v4_classic", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV4NoSorobanMeta(), 100), feeBucketClassic, 100},
		{"invoke_op_metaV0_classic", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV0(), 100), feeBucketClassic, 100},
		{"invoke_op_metaV1_classic", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV1(), 100), feeBucketClassic, 100},
		{"invoke_op_metaV2_classic", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV2(), 100), feeBucketClassic, 100},
		// …and the converse: SorobanMeta present classifies soroban no matter
		// what the envelope's ops look like (v1, gating on the op type, called
		// these classic). The envelope's SorobanTransactionData flag is never
		// consulted either way.
		{"classic_envelope_invoke_op_v3_ext1_soroban",
			feeTxV1(t, []xdr.Operation{invoke}, false, feeMetaV3Ext1(40, 10), 100), feeBucketSoroban, 50},
		{"classic_op_with_soroban_meta_soroban",
			feeTxV1(t, []xdr.Operation{bump}, true, feeMetaV3Ext1(40, 10), 100), feeBucketSoroban, 50},
		{"multi_op_with_soroban_meta_soroban",
			feeTxV1(t, []xdr.Operation{invoke, invoke}, true, feeMetaV3Ext1(40, 10), 101), feeBucketSoroban, 51},
		{"soroban_envelope_classic_op_classic", feeTxV1(t, []xdr.Operation{bump}, true, feeMetaV1(), 100), feeBucketClassic, 100},
		{"fee_bump_classic_single_inner_op", feeBumpFeeTx(t, []xdr.Operation{bump}, false, feeMetaV1(), 200), feeBucketClassic, 200},
		{"fee_bump_classic_two_inner_ops", feeBumpFeeTx(t, []xdr.Operation{bump, bump}, false, feeMetaV1(), 201), feeBucketClassic, 100},
		{"fee_bump_soroban_v3_ext1", feeBumpFeeTx(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(30, 20), 300), feeBucketSoroban, 250},
		{"fee_bump_soroban_v4_ext1", feeBumpFeeTx(t, []xdr.Operation{invoke}, true, feeMetaV4Ext1(30, 20), 300), feeBucketSoroban, 250},
		{"fee_bump_soroban_meta_ext0_skipped", feeBumpFeeTx(t, []xdr.Operation{invoke}, true, feeMetaV3Ext0(), 300), feeBucketNone, 0},
		{"zero_op_results_skipped", feeTxV1(t, nil, false, feeMetaV1(), 100), feeBucketNone, 0},
		{"fee_bump_zero_inner_op_results_skipped", feeBumpFeeTx(t, nil, false, feeMetaV1(), 200), feeBucketNone, 0},
		{"failed_classic_multi_op_still_counted", failedClassic, feeBucketClassic, 50},
		{"failed_soroban_still_counted", failedSoroban, feeBucketSoroban, 50},
		{"fee_bump_inner_failed_still_counted", innerFailedFeeBump, feeBucketClassic, 200},
		{"results_count_is_authoritative", resultsAuthoritative, feeBucketClassic, 33},
		{"internal_error_skipped", internalError, feeBucketNone, 0},
		{"fee_bump_inner_internal_error_skipped", innerInternalErrorFeeBump, feeBucketNone, 0},
		{"fee_charged_zero", feeTxV1(t, []xdr.Operation{bump}, false, feeMetaV1(), 0), feeBucketClassic, 0},
		{"fee_charged_below_op_count_rounds_to_zero", feeTxV1(t, []xdr.Operation{bump, bump, bump}, false, feeMetaV1(), 2), feeBucketClassic, 0},
		{"soroban_zero_resource_fees", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(0, 0), 100), feeBucketSoroban, 100},
		// Boundary of the exceeds-FeeCharged error: exactly equal is fine and
		// yields a zero inclusion fee.
		{"soroban_resource_fee_equals_fee_charged",
			feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(60, 40), 100), feeBucketSoroban, 0},
	}
}

// expectedFees folds the cases' annotations into the two apply-order buckets.
func expectedFees(cases []feeCase) (classic, soroban []uint64) {
	for _, c := range cases {
		switch c.bucket {
		case feeBucketClassic:
			classic = append(classic, c.value)
		case feeBucketSoroban:
			soroban = append(soroban, c.value)
		case feeBucketNone:
		}
	}
	return classic, soroban
}

// feesFromLCM marshals the fixture ledger, walks it once, and reads the fee
// product off the walk.
func feesFromLCM(t *testing.T, lcm xdr.LedgerCloseMeta) LedgerFees {
	t.Helper()
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	txParts, err := ExtractLedgerTxParts(xdr.LedgerCloseMetaView(raw))
	require.NoError(t, err)
	got, err := FeesFromTxParts(txParts)
	require.NoError(t, err)
	return got
}

// feesFromLCMErr is feesFromLCM for the error cases: the first error from the
// walk or the fee product, whichever fires.
func feesFromLCMErr(t *testing.T, lcm xdr.LedgerCloseMeta) error {
	t.Helper()
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	txParts, err := ExtractLedgerTxParts(xdr.LedgerCloseMetaView(raw))
	if err != nil {
		return err
	}
	_, err = FeesFromTxParts(txParts)
	return err
}

func TestFeesFromTxParts_FixtureMatrix(t *testing.T) {
	cases := feeMatrixCases(t)
	txs := make([]txWithHash, len(cases))
	for i, c := range cases {
		txs[i] = c.tx
	}
	wantClassic, wantSoroban := expectedFees(cases)

	for _, version := range []int32{0, 1, 2} {
		t.Run(fmt.Sprintf("lcmV%d", version), func(t *testing.T) {
			seq := 8800 + uint32(version) //nolint:gosec // version ∈ {0,1,2}
			lcm := buildLCM(t, version, seq, 1_700_080_000, txs, true /*reversed TxSet*/)
			got := feesFromLCM(t, lcm)
			assert.Equal(t, wantClassic, got.ClassicFeesPerOp)
			assert.Equal(t, wantSoroban, got.SorobanInclusionFees)
		})
	}
}

func TestFeesFromTxParts_MatrixCellsIsolated(t *testing.T) {
	for _, c := range feeMatrixCases(t) {
		t.Run(c.name, func(t *testing.T) {
			lcm := buildLCM(t, 2, 8900, 1_700_081_000, []txWithHash{c.tx}, false)
			got := feesFromLCM(t, lcm)
			var wantClassic, wantSoroban []uint64
			switch c.bucket {
			case feeBucketClassic:
				wantClassic = []uint64{c.value}
			case feeBucketSoroban:
				wantSoroban = []uint64{c.value}
			case feeBucketNone:
			}
			assert.Equal(t, wantClassic, got.ClassicFeesPerOp)
			assert.Equal(t, wantSoroban, got.SorobanInclusionFees)
		})
	}
}

// TestFeesFromTxParts_TxSetNotConsulted pins the reshape's core property: the
// classification never reads the TxSet, so gutting every envelope changes
// nothing. (Under the old envelope-pairing extractor this exact ledger was a
// missing-envelope error.)
func TestFeesFromTxParts_TxSetNotConsulted(t *testing.T) {
	txs := []txWithHash{
		feeTxV1(t, []xdr.Operation{feeBumpSequenceOp(), feeBumpSequenceOp()}, false, feeMetaV1(), 101),
		feeTxV1(t, []xdr.Operation{feeInvokeHostFunctionOp()}, true, feeMetaV3Ext1(40, 10), 100),
	}
	lcm := buildLCM(t, 2, 8963, 1_700_084_300, txs, false)
	lcm.V2.TxSet.V1TxSet.Phases = nil

	got := feesFromLCM(t, lcm)
	assert.Equal(t, []uint64{50}, got.ClassicFeesPerOp)
	assert.Equal(t, []uint64{50}, got.SorobanInclusionFees)
}

func TestExtractLedgerTxParts_UnknownLCMVersionErrors(t *testing.T) {
	lcm := buildLCM(t, 2, 8966, 1_700_084_600, nil, false)
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	raw[3] = 9 // union discriminant is a big-endian int32 at offset 0
	_, err = ExtractLedgerTxParts(xdr.LedgerCloseMetaView(raw))
	require.ErrorContains(t, err, "unknown LCM")
}

func TestFeesFromTxParts_EmptyLedger(t *testing.T) {
	for _, version := range []int32{0, 1, 2} {
		t.Run(fmt.Sprintf("lcmV%d", version), func(t *testing.T) {
			seq := 9900 + uint32(version) //nolint:gosec // version ∈ {0,1,2}
			lcm := buildLCM(t, version, seq, 1_700_082_000, nil, false)
			got := feesFromLCM(t, lcm)
			assert.Empty(t, got.ClassicFeesPerOp)
			assert.Empty(t, got.SorobanInclusionFees)
		})
	}
}

func TestFeesFromTxParts_ParallelTxsPhase(t *testing.T) {
	invoke := feeInvokeHostFunctionOp()
	bump := feeBumpSequenceOp()
	txs := []txWithHash{
		feeTxV1(t, []xdr.Operation{bump, bump}, false, feeMetaV1(), 101),           // classic 50
		feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(40, 10), 100),      // soroban 50
		feeBumpFeeTx(t, []xdr.Operation{invoke}, true, feeMetaV4Ext1(30, 20), 300), // soroban 250
		feeTxV1(t, []xdr.Operation{bump}, false, feeMetaV2(), 100),                 // classic 100
		feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext0(), 100),            // skipped
		feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV4Ext1(25, 15), 100),      // soroban 60
	}
	layout := [][][]int{
		{{5, 4}, {3}},
		{{2, 1, 0}},
	}
	lcm := buildParallelTxsLCM(t, 8951, 1_700_083_000, txs, layout)
	got := feesFromLCM(t, lcm)
	assert.Equal(t, []uint64{50, 100}, got.ClassicFeesPerOp)
	assert.Equal(t, []uint64{50, 250, 60}, got.SorobanInclusionFees)
}

// ---------------------------------------------------------------------------
// Error edges
// ---------------------------------------------------------------------------

func TestFeesFromTxParts_NegativeFeeChargedErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		tx   txWithHash
	}{
		{"classic", feeTxV1(t, []xdr.Operation{feeBumpSequenceOp()}, false, feeMetaV1(), -1)},
		// The fee check precedes the no-per-op-results skip, so an op-less tx
		// with a negative fee still errors.
		{"zero_op", feeTxV1(t, nil, false, feeMetaV1(), -1)},
		// …and precedes the soroban meta classification: a soroban tx with a
		// negative fee errors on the fee, not on anything meta-related.
		{"soroban", feeTxV1(t, []xdr.Operation{feeInvokeHostFunctionOp()}, true, feeMetaV3Ext1(40, 10), -1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lcm := buildLCM(t, 2, 8960, 1_700_084_000, []txWithHash{tc.tx}, false)
			require.ErrorContains(t, feesFromLCMErr(t, lcm), "negative")
		})
	}
}

func TestFeesFromTxParts_NegativeResourceFeeErrors(t *testing.T) {
	invoke := feeInvokeHostFunctionOp()
	for _, tc := range []struct {
		name string
		tx   txWithHash
	}{
		{"negative_component", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(-10, 5), 100)},
		// The two components are added in int64 first; a sum past MaxInt64
		// wraps negative and errors rather than summing wide.
		{"sum_overflows_int64", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(math.MaxInt64, 1), 100)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lcm := buildLCM(t, 2, 8961, 1_700_084_100, []txWithHash{tc.tx}, false)
			require.ErrorContains(t, feesFromLCMErr(t, lcm), "negative")
		})
	}
}

// A charged resource fee above FeeCharged is corrupt input and errors loudly.
// (v1 wrapped the uint64 subtraction here — a deliberate delta, see
// FeesFromTxParts.)
func TestFeesFromTxParts_ResourceFeeExceedsFeeChargedErrors(t *testing.T) {
	tx := feeTxV1(t, []xdr.Operation{feeInvokeHostFunctionOp()}, true, feeMetaV3Ext1(100, 50), 100)
	lcm := buildLCM(t, 2, 8962, 1_700_084_200, []txWithHash{tx}, false)
	require.ErrorContains(t, feesFromLCMErr(t, lcm), "exceeds fee charged")
}
