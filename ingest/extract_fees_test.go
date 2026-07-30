package ingest

import (
	"errors"
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

// ---------------------------------------------------------------------------
// Primary oracle: stellar-rpc v1's FeeWindows.IngestFees
// (cmd/stellar-rpc/internal/feewindow/feewindow.go), ported verbatim over the
// parsed xdr.LedgerCloseMeta. The db-rollback plumbing is dropped and the two
// window buckets become the returned LedgerFees. This is the behavioral
// contract for ExtractFees: every fixture asserts the two agree. Test-only —
// nothing oracle-related ships in the package.
// ---------------------------------------------------------------------------

// oracleInt64ToUint64 is feewindow.go's int64ToUint64, kept verbatim.
func oracleInt64ToUint64(value int64, fieldName string) (uint64, error) {
	if value < 0 {
		return 0, errors.New(fieldName + " cannot be negative")
	}
	return uint64(value), nil
}

//nolint:gocognit,gocyclo // verbatim port; must not drift from the original's shape
func ingestFeesOracle(networkPassPhrase string, meta xdr.LedgerCloseMeta) (LedgerFees, error) {
	reader, err := NewLedgerTransactionReaderFromLedgerCloseMeta(networkPassPhrase, meta)
	if err != nil {
		return LedgerFees{}, err
	}
	var sorobanInclusionFees []uint64
	var classicFees []uint64
	for {
		tx, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return LedgerFees{}, err
		}
		feeCharged, err := oracleInt64ToUint64(int64(tx.Result.Result.FeeCharged), "fee charged")
		if err != nil {
			return LedgerFees{}, err
		}
		ops := tx.Envelope.Operations()
		if len(ops) == 0 {
			// should not happen
			continue
		}
		if len(ops) == 1 {
			switch ops[0].Body.Type { //nolint:exhaustive
			case xdr.OperationTypeInvokeHostFunction, xdr.OperationTypeExtendFootprintTtl, xdr.OperationTypeRestoreFootprint:
				var sorobanFees xdr.SorobanTransactionMetaExtV1
				switch tx.UnsafeMeta.V {
				case 3:
					if tx.UnsafeMeta.V3.SorobanMeta == nil || tx.UnsafeMeta.V3.SorobanMeta.Ext.V != 1 {
						continue
					}
					sorobanFees = *tx.UnsafeMeta.V3.SorobanMeta.Ext.V1
				case 4:
					if tx.UnsafeMeta.V4.SorobanMeta == nil || tx.UnsafeMeta.V4.SorobanMeta.Ext.V != 1 {
						continue
					}
					sorobanFees = *tx.UnsafeMeta.V4.SorobanMeta.Ext.V1
				default:
					continue
				}
				resourceFeeCharged := sorobanFees.TotalNonRefundableResourceFeeCharged +
					sorobanFees.TotalRefundableResourceFeeCharged
				resourceFeeChargedUint64, convErr := oracleInt64ToUint64(int64(resourceFeeCharged), "resource fee charged")
				if convErr != nil {
					return LedgerFees{}, convErr
				}
				inclusionFee := feeCharged - resourceFeeChargedUint64
				sorobanInclusionFees = append(sorobanInclusionFees, inclusionFee)
				continue
			}
		}
		feePerOp := feeCharged / uint64(len(ops))
		classicFees = append(classicFees, feePerOp)
	}
	return LedgerFees{
		ClassicFeesPerOp:     classicFees,
		SorobanInclusionFees: sorobanInclusionFees,
		LedgerSequence:       meta.LedgerSequence(),
		LedgerCloseTime:      meta.LedgerCloseTime(),
	}, nil
}

// ExtractFeesOracleForTesting exposes the ported v1 walk to the package's
// external tests (extract_real_ledger_test.go, extract_bench_test.go). Being
// declared in a _test.go file, it does not ship.
var ExtractFeesOracleForTesting = ingestFeesOracle

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

// feeResult is a TransactionResult with a configurable FeeCharged.
func feeResult(feeCharged int64, success bool) xdr.TransactionResult {
	code := xdr.TransactionResultCodeTxFailed
	if success {
		code = xdr.TransactionResultCodeTxSuccess
	}
	ops := []xdr.OperationResult{}
	return xdr.TransactionResult{
		FeeCharged: xdr.Int64(feeCharged),
		Result:     xdr.TransactionResultResult{Code: code, Results: &ops},
	}
}

// feeFeeBumpResult is a txFEE_BUMP_INNER_SUCCESS (or _FAILED) result with a
// configurable (outer) FeeCharged.
func feeFeeBumpResult(innerHash xdr.Hash, feeCharged int64, innerSuccess bool) xdr.TransactionResult {
	outerCode := xdr.TransactionResultCodeTxFeeBumpInnerFailed
	innerCode := xdr.TransactionResultCodeTxFailed
	if innerSuccess {
		outerCode = xdr.TransactionResultCodeTxFeeBumpInnerSuccess
		innerCode = xdr.TransactionResultCodeTxSuccess
	}
	ops := []xdr.OperationResult{}
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

// feeTxV1 is a V1-envelope transaction: ops + meta + a success result carrying
// feeCharged. sorobanEnv attaches an (empty) SorobanTransactionData — the
// envelope soroban flag, deliberately independent of the op types.
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
	res := feeResult(feeCharged, true)
	return txWithHash{env: env, hash: hash, meta: meta, result: &res}
}

// feeTxV0 is feeTxV1 on a legacy TX_V0 envelope (never soroban).
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
	res := feeResult(feeCharged, true)
	return txWithHash{env: env, hash: hash, meta: meta, result: &res}
}

// feeBumpFeeTx wraps the ops in a fee-bump envelope; outerFeeCharged lands in
// the OUTER result (the value the classification reads), while the ops are the
// INNER transaction's.
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
	res := feeFeeBumpResult(innerHashOf(t, env), outerFeeCharged, true)
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

// feeAccountChange is a State→Updated balance move for one account — the shape
// FeeProcessing / TxChangesAfter carry for fee charges and refunds.
func feeAccountChange(address string, before, after int64) xdr.LedgerEntryChanges {
	entry := func(balance int64) xdr.LedgerEntry {
		return xdr.LedgerEntry{Data: xdr.LedgerEntryData{
			Type:    xdr.LedgerEntryTypeAccount,
			Account: &xdr.AccountEntry{AccountId: xdr.MustAddress(address), Balance: xdr.Int64(balance)},
		}}
	}
	state := entry(before)
	updated := entry(after)
	return xdr.LedgerEntryChanges{
		{Type: xdr.LedgerEntryChangeTypeLedgerEntryState, State: &state},
		{Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated, Updated: &updated},
	}
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
// type (V0/V1/fee-bump) × shape (classic single/multi-op, each soroban op
// type, Ext.V==0, missing SorobanMeta, 0-op, failed, wrap).
func feeMatrixCases(t testing.TB) []feeCase {
	t.Helper()
	invoke := feeInvokeHostFunctionOp()
	extend := feeExtendFootprintTtlOp()
	restore := feeRestoreFootprintOp()
	bump := feeBumpSequenceOp()

	failedClassic := feeTxV1(t, []xdr.Operation{bump, bump}, false, feeMetaV1(), 100)
	failedRes := feeResult(100, false)
	failedClassic.result = &failedRes

	failedSoroban := feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(40, 10), 100)
	failedSorobanRes := feeResult(100, false)
	failedSoroban.result = &failedSorobanRes

	innerFailedFeeBump := feeBumpFeeTx(t, []xdr.Operation{bump}, false, feeMetaV1(), 200)
	innerFailedRes := feeFeeBumpResult(innerHashOf(t, innerFailedFeeBump.env), 200, false)
	innerFailedFeeBump.result = &innerFailedRes

	return []feeCase{
		{"classic_single_op_metaV1", feeTxV1(t, []xdr.Operation{bump}, false, feeMetaV1(), 100), feeBucketClassic, 100},
		{"classic_multi_op_rounds_down_metaV2", feeTxV1(t, []xdr.Operation{bump, bump, bump}, false, feeMetaV2(), 100), feeBucketClassic, 33},
		{"classic_single_op_v0_envelope_metaV0", feeTxV0(t, []xdr.Operation{bump}, feeMetaV0(), 100), feeBucketClassic, 100},
		{"classic_multi_op_v0_envelope", feeTxV0(t, []xdr.Operation{bump, bump}, feeMetaV0(), 101), feeBucketClassic, 50},
		{"soroban_invoke_v3_ext1", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(40, 10), 100), feeBucketSoroban, 50},
		{"soroban_extend_v3_ext1", feeTxV1(t, []xdr.Operation{extend}, true, feeMetaV3Ext1(30, 10), 100), feeBucketSoroban, 60},
		{"soroban_restore_v3_ext1", feeTxV1(t, []xdr.Operation{restore}, true, feeMetaV3Ext1(20, 10), 100), feeBucketSoroban, 70},
		{"soroban_invoke_v3_ext0_skipped", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext0(), 100), feeBucketNone, 0},
		{"soroban_invoke_v3_no_soroban_meta_skipped", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3NoSorobanMeta(), 100), feeBucketNone, 0},
		{"soroban_invoke_v4_ext1", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV4Ext1(25, 15), 100), feeBucketSoroban, 60},
		{"soroban_invoke_v4_ext0_skipped", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV4Ext0(), 100), feeBucketNone, 0},
		{"soroban_invoke_v4_no_soroban_meta_skipped", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV4NoSorobanMeta(), 100), feeBucketNone, 0},
		{"soroban_invoke_metaV0_skipped", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV0(), 100), feeBucketNone, 0},
		{"soroban_invoke_metaV1_skipped", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV1(), 100), feeBucketNone, 0},
		{"soroban_invoke_metaV2_skipped", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV2(), 100), feeBucketNone, 0},
		// The gate is the op type alone: a CLASSIC envelope (no
		// SorobanTransactionData) with a single invoke op and Ext.V1 meta is
		// still soroban-classified, exactly like v1.
		{"classic_envelope_invoke_op_v3_ext1_soroban",
			feeTxV1(t, []xdr.Operation{invoke}, false, feeMetaV3Ext1(40, 10), 100), feeBucketSoroban, 50},
		// …and the converse: a soroban-FLAGGED envelope (SorobanTransactionData
		// present) whose single op is classic stays classic — even with an
		// Ext.V1 meta attached, which the classic path never reads.
		{"soroban_envelope_classic_op_classic", feeTxV1(t, []xdr.Operation{bump}, true, feeMetaV1(), 100), feeBucketClassic, 100},
		{"soroban_envelope_classic_op_ext1_meta_still_classic",
			feeTxV1(t, []xdr.Operation{bump}, true, feeMetaV3Ext1(40, 10), 100), feeBucketClassic, 100},
		// Multiple ops disqualify the soroban path even when every op is a
		// soroban type and the extension is present.
		{"multi_op_soroban_types_classic", feeTxV1(t, []xdr.Operation{invoke, invoke}, true, feeMetaV3Ext1(40, 10), 101), feeBucketClassic, 50},
		{"fee_bump_classic_single_inner_op", feeBumpFeeTx(t, []xdr.Operation{bump}, false, feeMetaV1(), 200), feeBucketClassic, 200},
		{"fee_bump_classic_two_inner_ops", feeBumpFeeTx(t, []xdr.Operation{bump, bump}, false, feeMetaV1(), 201), feeBucketClassic, 100},
		{"fee_bump_soroban_v3_ext1", feeBumpFeeTx(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(30, 20), 300), feeBucketSoroban, 250},
		{"fee_bump_soroban_v4_ext1", feeBumpFeeTx(t, []xdr.Operation{invoke}, true, feeMetaV4Ext1(30, 20), 300), feeBucketSoroban, 250},
		{"fee_bump_soroban_ext0_skipped", feeBumpFeeTx(t, []xdr.Operation{invoke}, true, feeMetaV3Ext0(), 300), feeBucketNone, 0},
		{"zero_op_tx_skipped", feeTxV1(t, nil, false, feeMetaV1(), 100), feeBucketNone, 0},
		{"fee_bump_zero_inner_ops_skipped", feeBumpFeeTx(t, nil, false, feeMetaV1(), 200), feeBucketNone, 0},
		{"failed_classic_still_counted", failedClassic, feeBucketClassic, 50},
		{"failed_soroban_still_counted", failedSoroban, feeBucketSoroban, 50},
		{"fee_bump_inner_failed_still_counted", innerFailedFeeBump, feeBucketClassic, 200},
		{"fee_charged_zero", feeTxV1(t, []xdr.Operation{bump}, false, feeMetaV1(), 0), feeBucketClassic, 0},
		{"fee_charged_below_op_count_rounds_to_zero", feeTxV1(t, []xdr.Operation{bump, bump, bump}, false, feeMetaV1(), 2), feeBucketClassic, 0},
		{"soroban_zero_resource_fees", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(0, 0), 100), feeBucketSoroban, 100},
		// v1 subtracts in uint64 with no underflow check: a resource fee above
		// FeeCharged wraps. Preserved bug-for-bug.
		{"soroban_resource_fee_above_fee_charged_wraps",
			feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(100, 50), 100), feeBucketSoroban, math.MaxUint64 - 49},
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

// requireFeesMatchOracle asserts ExtractFees on the marshaled LCM equals the
// ported v1 walk on the parsed LCM, and returns the view result.
func requireFeesMatchOracle(t *testing.T, lcm xdr.LedgerCloseMeta) LedgerFees {
	t.Helper()
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	got, err := ExtractFees(xdr.LedgerCloseMetaView(raw), viewTestPassphrase)
	require.NoError(t, err)
	oracle, err := ingestFeesOracle(viewTestPassphrase, lcm)
	require.NoError(t, err)
	assert.Equal(t, oracle, got, "view path must match the ported v1 IngestFees walk")
	return got
}

func TestExtractFees_EquivalentToIngestFeesOracle(t *testing.T) {
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
			got := requireFeesMatchOracle(t, lcm)
			assert.Equal(t, wantClassic, got.ClassicFeesPerOp)
			assert.Equal(t, wantSoroban, got.SorobanInclusionFees)
			assert.Equal(t, seq, got.LedgerSequence)
			assert.Equal(t, int64(1_700_080_000), got.LedgerCloseTime)
		})
	}
}

func TestExtractFees_MatrixCellsIsolated(t *testing.T) {
	for _, c := range feeMatrixCases(t) {
		t.Run(c.name, func(t *testing.T) {
			lcm := buildLCM(t, 2, 8900, 1_700_081_000, []txWithHash{c.tx}, false)
			got := requireFeesMatchOracle(t, lcm)
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

func TestExtractFees_EmptyLedgerEmptyPassphrase(t *testing.T) {
	// v1 validates the passphrase inside envelope hashing, which an empty
	// TxSet never reaches — so the empty passphrase must NOT be an error here.
	lcm := buildLCM(t, 2, 9910, 1_700_082_100, nil, false)
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	got, err := ExtractFees(xdr.LedgerCloseMetaView(raw), "")
	require.NoError(t, err)
	oracle, err := ingestFeesOracle("", lcm)
	require.NoError(t, err)
	assert.Equal(t, oracle, got)
}

func TestExtractFees_EmptyTxProcessingNonEmptyTxSet(t *testing.T) {
	// The converse of the empty-ledger case: with envelopes present, v1 hashes
	// them all at reader construction even though nothing was applied, so a bad
	// passphrase IS an error — while a valid one yields empty buckets.
	tx := feeTxV1(t, []xdr.Operation{feeBumpSequenceOp()}, false, feeMetaV1(), 100)
	lcm := buildLCM(t, 2, 9911, 1_700_082_200, []txWithHash{tx}, false)
	lcm.V2.TxProcessing = nil

	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	_, err = ExtractFees(xdr.LedgerCloseMetaView(raw), "")
	require.Error(t, err, "empty passphrase must fail once there is an envelope to hash")
	_, oerr := ingestFeesOracle("", lcm)
	require.Error(t, oerr, "the v1 walk rejects it too")

	got := requireFeesMatchOracle(t, lcm)
	assert.Empty(t, got.ClassicFeesPerOp)
	assert.Empty(t, got.SorobanInclusionFees)
}

func TestExtractFees_PreProtocol10MetaGuard(t *testing.T) {
	// v1's reader refuses a pre-protocol-10 ledger whose meta is older than V2
	// while FeeProcessing is populated (badMetaVersionErr — the meta came from
	// an outdated stellar-core).
	bump := feeBumpSequenceOp()
	withFees := feeTxV1(t, []xdr.Operation{bump}, false, feeMetaV1(), 100)
	withFees.feeChanges = feeAccountChange(keypair.MustRandom().Address(), 200, 100)

	t.Run("guard_trips", func(t *testing.T) {
		lcm := buildLCM(t, 1, 8967, 1_700_084_700, []txWithHash{withFees}, false)
		setLedgerVersion(t, &lcm, 9)
		raw, err := lcm.MarshalBinary()
		require.NoError(t, err)
		_, err = ExtractFees(xdr.LedgerCloseMetaView(raw), viewTestPassphrase)
		require.ErrorContains(t, err, "TransactionMeta.V=2 is required")
		_, oerr := ingestFeesOracle(viewTestPassphrase, lcm)
		require.ErrorContains(t, oerr, "TransactionMeta.V=2 is required")
	})
	t.Run("empty_fee_processing_passes", func(t *testing.T) {
		plain := feeTxV1(t, []xdr.Operation{bump}, false, feeMetaV1(), 100)
		lcm := buildLCM(t, 1, 8968, 1_700_084_800, []txWithHash{plain}, false)
		setLedgerVersion(t, &lcm, 9)
		got := requireFeesMatchOracle(t, lcm)
		assert.Equal(t, []uint64{100}, got.ClassicFeesPerOp)
	})
	t.Run("meta_v2_passes", func(t *testing.T) {
		v2 := feeTxV1(t, []xdr.Operation{bump}, false, feeMetaV2(), 100)
		v2.feeChanges = feeAccountChange(keypair.MustRandom().Address(), 200, 100)
		lcm := buildLCM(t, 1, 8969, 1_700_084_900, []txWithHash{v2}, false)
		setLedgerVersion(t, &lcm, 9)
		got := requireFeesMatchOracle(t, lcm)
		assert.Equal(t, []uint64{100}, got.ClassicFeesPerOp)
	})
}

func TestExtractFees_MissingEnvelopeErrors(t *testing.T) {
	txs := []txWithHash{
		feeTxV1(t, []xdr.Operation{feeBumpSequenceOp()}, false, feeMetaV1(), 100),
		feeTxV1(t, []xdr.Operation{feeBumpSequenceOp()}, false, feeMetaV1(), 100),
	}
	lcm := buildLCM(t, 2, 8963, 1_700_084_300, txs, false)
	dropped := lcm.V2.TxSet.V1TxSet.Phases[0].V0Components
	(*dropped)[0].TxsMaybeDiscountedFee.Txs = (*dropped)[0].TxsMaybeDiscountedFee.Txs[:1]

	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	_, err = ExtractFees(xdr.LedgerCloseMetaView(raw), viewTestPassphrase)
	require.ErrorContains(t, err, "missing from TxSet")
	_, oerr := ingestFeesOracle(viewTestPassphrase, lcm)
	require.Error(t, oerr, "the v1 walk rejects it too")
}

func TestExtractFees_DuplicateHash(t *testing.T) {
	tx := feeTxV1(t, []xdr.Operation{feeBumpSequenceOp()}, false, feeMetaV1(), 100)
	lcm := buildLCM(t, 2, 8964, 1_700_084_400, []txWithHash{tx, tx}, false)
	got := requireFeesMatchOracle(t, lcm)
	assert.Equal(t, []uint64{100, 100}, got.ClassicFeesPerOp)
}

func TestExtractFees_ExtraUnappliedTxSetEnvelope(t *testing.T) {
	applied := feeTxV1(t, []xdr.Operation{feeBumpSequenceOp()}, false, feeMetaV1(), 100)
	extra := feeTxV1(t, []xdr.Operation{feeBumpSequenceOp()}, false, feeMetaV1(), 999)
	lcm := buildLCM(t, 2, 8965, 1_700_084_500, []txWithHash{applied}, false)
	comps := lcm.V2.TxSet.V1TxSet.Phases[0].V0Components
	(*comps)[0].TxsMaybeDiscountedFee.Txs = append((*comps)[0].TxsMaybeDiscountedFee.Txs, extra.env)

	got := requireFeesMatchOracle(t, lcm)
	assert.Equal(t, []uint64{100}, got.ClassicFeesPerOp)
	assert.Empty(t, got.SorobanInclusionFees)
}

func TestExtractFees_UnknownLCMVersionErrors(t *testing.T) {
	lcm := buildLCM(t, 2, 8966, 1_700_084_600, nil, false)
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	raw[3] = 9 // union discriminant is a big-endian int32 at offset 0
	_, err = ExtractFees(xdr.LedgerCloseMetaView(raw), viewTestPassphrase)
	require.ErrorContains(t, err, "unknown LCM")
}

func TestExtractFees_EmptyLedger(t *testing.T) {
	for _, version := range []int32{0, 1, 2} {
		t.Run(fmt.Sprintf("lcmV%d", version), func(t *testing.T) {
			seq := 9900 + uint32(version) //nolint:gosec // version ∈ {0,1,2}
			lcm := buildLCM(t, version, seq, 1_700_082_000, nil, false)
			got := requireFeesMatchOracle(t, lcm)
			assert.Empty(t, got.ClassicFeesPerOp)
			assert.Empty(t, got.SorobanInclusionFees)
			assert.Equal(t, seq, got.LedgerSequence)
			assert.Equal(t, int64(1_700_082_000), got.LedgerCloseTime)
		})
	}
}

func TestExtractFees_ParallelTxsPhase(t *testing.T) {
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
	got := requireFeesMatchOracle(t, lcm)
	assert.Equal(t, []uint64{50, 100}, got.ClassicFeesPerOp)
	assert.Equal(t, []uint64{50, 250, 60}, got.SorobanInclusionFees)
}

// ---------------------------------------------------------------------------
// Error edges
// ---------------------------------------------------------------------------

func TestExtractFees_NegativeFeeChargedErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		tx   txWithHash
	}{
		{"classic", feeTxV1(t, []xdr.Operation{feeBumpSequenceOp()}, false, feeMetaV1(), -1)},
		// The fee check precedes the 0-op skip in v1, so an op-less tx with a
		// negative fee still errors.
		{"zero_op", feeTxV1(t, nil, false, feeMetaV1(), -1)},
		// …and precedes the soroban meta read: a soroban-shaped tx with a
		// negative fee errors on the fee, not on anything meta-related.
		{"soroban_shaped", feeTxV1(t, []xdr.Operation{feeInvokeHostFunctionOp()}, true, feeMetaV3Ext1(40, 10), -1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lcm := buildLCM(t, 2, 8960, 1_700_084_000, []txWithHash{tc.tx}, false)
			raw, err := lcm.MarshalBinary()
			require.NoError(t, err)
			_, err = ExtractFees(xdr.LedgerCloseMetaView(raw), viewTestPassphrase)
			require.ErrorContains(t, err, "negative")
			_, oerr := ingestFeesOracle(viewTestPassphrase, lcm)
			require.Error(t, oerr, "the v1 walk rejects it too")
		})
	}
}

func TestExtractFees_NegativeResourceFeeErrors(t *testing.T) {
	invoke := feeInvokeHostFunctionOp()
	for _, tc := range []struct {
		name string
		tx   txWithHash
	}{
		{"negative_component", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(-10, 5), 100)},
		// v1 adds the two components in int64 first; a sum past MaxInt64 wraps
		// negative and errors rather than summing wide. Preserved bug-for-bug.
		{"sum_overflows_int64", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(math.MaxInt64, 1), 100)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lcm := buildLCM(t, 2, 8961, 1_700_084_100, []txWithHash{tc.tx}, false)
			raw, err := lcm.MarshalBinary()
			require.NoError(t, err)
			_, err = ExtractFees(xdr.LedgerCloseMetaView(raw), viewTestPassphrase)
			require.ErrorContains(t, err, "negative")
			_, oerr := ingestFeesOracle(viewTestPassphrase, lcm)
			require.Error(t, oerr, "the v1 walk rejects it too")
		})
	}
}
