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

type feeCaseBucket int

const (
	feeSkip feeCaseBucket = iota
	feeClassic
	feeSoroban
)

type feeCase struct {
	name   string
	tx     txWithHash
	bucket feeCaseBucket
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
		{"classic_single_op_metaV1", feeTxV1(t, []xdr.Operation{bump}, false, feeMetaV1(), 100), feeClassic, 100},
		{"classic_multi_op_rounds_down_metaV2", feeTxV1(t, []xdr.Operation{bump, bump, bump}, false, feeMetaV2(), 100), feeClassic, 33},
		{"classic_single_op_v0_envelope_metaV0", feeTxV0(t, []xdr.Operation{bump}, feeMetaV0(), 100), feeClassic, 100},
		{"classic_multi_op_v0_envelope", feeTxV0(t, []xdr.Operation{bump, bump}, feeMetaV0(), 101), feeClassic, 50},
		{"soroban_invoke_v3_ext1", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(40, 10), 100), feeSoroban, 50},
		{"soroban_extend_v3_ext1", feeTxV1(t, []xdr.Operation{extend}, true, feeMetaV3Ext1(30, 10), 100), feeSoroban, 60},
		{"soroban_restore_v3_ext1", feeTxV1(t, []xdr.Operation{restore}, true, feeMetaV3Ext1(20, 10), 100), feeSoroban, 70},
		{"soroban_invoke_v3_ext0_skipped", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext0(), 100), feeSkip, 0},
		{"soroban_invoke_v3_no_soroban_meta_skipped", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3NoSorobanMeta(), 100), feeSkip, 0},
		{"soroban_invoke_v4_ext1", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV4Ext1(25, 15), 100), feeSoroban, 60},
		{"soroban_invoke_v4_ext0_skipped", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV4Ext0(), 100), feeSkip, 0},
		{"soroban_invoke_v4_no_soroban_meta_skipped", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV4NoSorobanMeta(), 100), feeSkip, 0},
		{"soroban_invoke_metaV0_skipped", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV0(), 100), feeSkip, 0},
		{"soroban_invoke_metaV1_skipped", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV1(), 100), feeSkip, 0},
		{"soroban_invoke_metaV2_skipped", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV2(), 100), feeSkip, 0},
		// The gate is the op type alone: a CLASSIC envelope (no
		// SorobanTransactionData) with a single invoke op and Ext.V1 meta is
		// still soroban-classified, exactly like v1.
		{"classic_envelope_invoke_op_v3_ext1_soroban", feeTxV1(t, []xdr.Operation{invoke}, false, feeMetaV3Ext1(40, 10), 100), feeSoroban, 50},
		// …and the converse: a soroban-FLAGGED envelope (SorobanTransactionData
		// present) whose single op is classic stays classic — even with an
		// Ext.V1 meta attached, which the classic path never reads.
		{"soroban_envelope_classic_op_classic", feeTxV1(t, []xdr.Operation{bump}, true, feeMetaV1(), 100), feeClassic, 100},
		{"soroban_envelope_classic_op_ext1_meta_still_classic",
			feeTxV1(t, []xdr.Operation{bump}, true, feeMetaV3Ext1(40, 10), 100), feeClassic, 100},
		// Multiple ops disqualify the soroban path even when every op is a
		// soroban type and the extension is present.
		{"multi_op_soroban_types_classic", feeTxV1(t, []xdr.Operation{invoke, invoke}, true, feeMetaV3Ext1(40, 10), 101), feeClassic, 50},
		{"fee_bump_classic_single_inner_op", feeBumpFeeTx(t, []xdr.Operation{bump}, false, feeMetaV1(), 200), feeClassic, 200},
		{"fee_bump_classic_two_inner_ops", feeBumpFeeTx(t, []xdr.Operation{bump, bump}, false, feeMetaV1(), 201), feeClassic, 100},
		{"fee_bump_soroban_v3_ext1", feeBumpFeeTx(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(30, 20), 300), feeSoroban, 250},
		{"fee_bump_soroban_v4_ext1", feeBumpFeeTx(t, []xdr.Operation{invoke}, true, feeMetaV4Ext1(30, 20), 300), feeSoroban, 250},
		{"fee_bump_soroban_ext0_skipped", feeBumpFeeTx(t, []xdr.Operation{invoke}, true, feeMetaV3Ext0(), 300), feeSkip, 0},
		{"zero_op_tx_skipped", feeTxV1(t, nil, false, feeMetaV1(), 100), feeSkip, 0},
		{"fee_bump_zero_inner_ops_skipped", feeBumpFeeTx(t, nil, false, feeMetaV1(), 200), feeSkip, 0},
		{"failed_classic_still_counted", failedClassic, feeClassic, 50},
		{"failed_soroban_still_counted", failedSoroban, feeSoroban, 50},
		{"fee_bump_inner_failed_still_counted", innerFailedFeeBump, feeClassic, 200},
		{"fee_charged_zero", feeTxV1(t, []xdr.Operation{bump}, false, feeMetaV1(), 0), feeClassic, 0},
		{"fee_charged_below_op_count_rounds_to_zero", feeTxV1(t, []xdr.Operation{bump, bump, bump}, false, feeMetaV1(), 2), feeClassic, 0},
		{"soroban_zero_resource_fees", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(0, 0), 100), feeSoroban, 100},
		// v1 subtracts in uint64 with no underflow check: a resource fee above
		// FeeCharged wraps. Preserved bug-for-bug.
		{"soroban_resource_fee_above_fee_charged_wraps",
			feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(100, 50), 100), feeSoroban, math.MaxUint64 - 49},
	}
}

// expectedFees folds the cases' annotations into the two apply-order buckets.
func expectedFees(cases []feeCase) (classic, soroban []uint64) {
	for _, c := range cases {
		switch c.bucket {
		case feeClassic:
			classic = append(classic, c.value)
		case feeSoroban:
			soroban = append(soroban, c.value)
		case feeSkip:
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

// TestExtractFees_EquivalentToIngestFeesOracle runs the full matrix in one
// ledger per LCM version (reversed TxSet, so envelope pairing is exercised),
// asserting both oracle equality and the hand-computed buckets — a
// misclassification fails on the explicit expectation, not just on drift.
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

// TestExtractFees_MatrixCellsIsolated runs every matrix cell as a
// single-transaction ledger, so a wrong classification cannot be masked by
// neighboring transactions.
func TestExtractFees_MatrixCellsIsolated(t *testing.T) {
	for _, c := range feeMatrixCases(t) {
		t.Run(c.name, func(t *testing.T) {
			lcm := buildLCM(t, 2, 8900, 1_700_081_000, []txWithHash{c.tx}, false)
			got := requireFeesMatchOracle(t, lcm)
			var wantClassic, wantSoroban []uint64
			switch c.bucket {
			case feeClassic:
				wantClassic = []uint64{c.value}
			case feeSoroban:
				wantSoroban = []uint64{c.value}
			case feeSkip:
			}
			assert.Equal(t, wantClassic, got.ClassicFeesPerOp)
			assert.Equal(t, wantSoroban, got.SorobanInclusionFees)
		})
	}
}

// TestExtractFees_EmptyLedgerEmptyPassphrase pins a subtle piece of v1 parity:
// v1 validates the passphrase lazily (inside envelope hashing, which an empty
// ledger never reaches), so an empty ledger succeeds even with an empty
// passphrase.
func TestExtractFees_EmptyLedgerEmptyPassphrase(t *testing.T) {
	lcm := buildLCM(t, 2, 9910, 1_700_082_100, nil, false)
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	got, err := ExtractFees(xdr.LedgerCloseMetaView(raw), "")
	require.NoError(t, err)
	oracle, err := ingestFeesOracle("", lcm)
	require.NoError(t, err)
	assert.Equal(t, oracle, got)
}

// TestExtractFees_MissingEnvelopeErrors: a hash present in TxProcessing but
// absent from the TxSet (inconsistent LCM) is an error on both paths.
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

// TestExtractFees_DuplicateHash: the same transaction applied twice pairs the
// one envelope to both TxProcessing entries and contributes twice, like v1's
// reader (its by-hash map collapses the duplicate the same way).
func TestExtractFees_DuplicateHash(t *testing.T) {
	tx := feeTxV1(t, []xdr.Operation{feeBumpSequenceOp()}, false, feeMetaV1(), 100)
	lcm := buildLCM(t, 2, 8964, 1_700_084_400, []txWithHash{tx, tx}, false)
	got := requireFeesMatchOracle(t, lcm)
	assert.Equal(t, []uint64{100, 100}, got.ClassicFeesPerOp)
}

// TestExtractFees_ExtraUnappliedTxSetEnvelope: a TxSet envelope with no
// TxProcessing entry contributes nothing on either path (fees come from the
// TxProcessing walk; the TxSet only pairs envelopes).
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

// TestExtractFees_UnknownLCMVersionErrors: an LCM discriminant the dispatch
// does not know is an error (the parsed path cannot even decode such bytes).
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

// TestExtractFees_ParallelTxsPhase covers an LCM V2 whose TxSet uses the V=1
// parallel-txs phase (multiple stages and clusters, shuffled relative to apply
// order) with a fee mix across both buckets.
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

// TestExtractFees_ResourceFeeAboveFeeChargedWraps pins the u64-underflow
// behavior loudly: v1 never checks resourceFee ≤ FeeCharged, so the inclusion
// fee wraps around; ExtractFees preserves that, wrap value and all.
func TestExtractFees_ResourceFeeAboveFeeChargedWraps(t *testing.T) {
	tx := feeTxV1(t, []xdr.Operation{feeInvokeHostFunctionOp()}, true, feeMetaV3Ext1(100, 50), 100)
	lcm := buildLCM(t, 2, 8962, 1_700_084_200, []txWithHash{tx}, false)
	got := requireFeesMatchOracle(t, lcm)
	require.Len(t, got.SorobanInclusionFees, 1)
	assert.Equal(t, uint64(math.MaxUint64)-49, got.SorobanInclusionFees[0],
		"100 − 150 in uint64 wraps to 2^64 − 50")
}

// ---------------------------------------------------------------------------
// Secondary oracle: the parsed LedgerTransaction fee helpers. Where their
// semantics align with the extractor's outputs they are cross-asserted; where
// they deliberately differ, the divergence is pinned explicitly.
// ---------------------------------------------------------------------------

// readerTxByHash fetches the parsed LedgerTransaction for one fixture.
func readerTxByHash(t *testing.T, lcm xdr.LedgerCloseMeta, hash xdr.Hash) LedgerTransaction {
	t.Helper()
	for _, tx := range readerOracle(t, lcm) {
		if tx.Hash == hash {
			return tx
		}
	}
	t.Fatalf("tx %x not found in reader output", hash)
	return LedgerTransaction{}
}

// TestExtractFees_SecondaryOracle_ClassicInclusionFee: for a classic
// non-fee-bump tx, the parsed InclusionFeeCharged computes the same per-op
// value (FeeCharged / opCount) the extractor emits.
func TestExtractFees_SecondaryOracle_ClassicInclusionFee(t *testing.T) {
	bump := feeBumpSequenceOp()
	tx := feeTxV1(t, []xdr.Operation{bump, bump, bump}, false, feeMetaV1(), 100)
	lcm := buildLCM(t, 2, 8970, 1_700_085_000, []txWithHash{tx}, false)
	got := requireFeesMatchOracle(t, lcm)
	require.Equal(t, []uint64{33}, got.ClassicFeesPerOp)

	ref := readerTxByHash(t, lcm, tx.hash)
	incl, ok := ref.InclusionFeeCharged()
	require.True(t, ok)
	assert.EqualValues(t, got.ClassicFeesPerOp[0], incl)

	feeCharged, ok := ref.FeeCharged()
	require.True(t, ok)
	assert.EqualValues(t, got.ClassicFeesPerOp[0], feeCharged/int64(ref.OperationCount()))
}

// TestExtractFees_DivergesFromInclusionFeeCharged_ClassicFeeBump pins the
// known divergence: for a classic fee-bump, the parsed InclusionFeeCharged
// divides FeeCharged by opCount+1 (the protocol inclusion-fee equation counts
// the fee-bump wrapper as one extra fee-paying slot), while v1's IngestFees —
// and therefore ExtractFees — divides by the inner op count alone.
func TestExtractFees_DivergesFromInclusionFeeCharged_ClassicFeeBump(t *testing.T) {
	bump := feeBumpSequenceOp()
	tx := feeBumpFeeTx(t, []xdr.Operation{bump, bump}, false, feeMetaV1(), 210)
	lcm := buildLCM(t, 2, 8971, 1_700_085_100, []txWithHash{tx}, false)
	got := requireFeesMatchOracle(t, lcm)
	require.Equal(t, []uint64{105}, got.ClassicFeesPerOp, "extractor: 210 / 2 inner ops")

	ref := readerTxByHash(t, lcm, tx.hash)
	incl, ok := ref.InclusionFeeCharged()
	require.True(t, ok)
	assert.EqualValues(t, 70, incl, "parsed helper: 210 / (2 ops + 1 fee-bump slot)")

	feeCharged, ok := ref.FeeCharged()
	require.True(t, ok)
	opCount := int64(ref.OperationCount())
	assert.EqualValues(t, got.ClassicFeesPerOp[0], feeCharged/opCount)
	assert.EqualValues(t, incl, feeCharged/(opCount+1))
}

// TestExtractFees_SecondaryOracle_SorobanV3ResourceFees: for a V3+Ext.V1 meta
// the parsed SorobanTotal*ResourceFeeCharged helpers see the same components
// the extractor subtracts.
func TestExtractFees_SecondaryOracle_SorobanV3ResourceFees(t *testing.T) {
	tx := feeTxV1(t, []xdr.Operation{feeInvokeHostFunctionOp()}, true, feeMetaV3Ext1(40, 10), 100)
	lcm := buildLCM(t, 2, 8972, 1_700_085_200, []txWithHash{tx}, false)
	got := requireFeesMatchOracle(t, lcm)
	require.Equal(t, []uint64{50}, got.SorobanInclusionFees)

	ref := readerTxByHash(t, lcm, tx.hash)
	nonRefundable, ok := ref.SorobanTotalNonRefundableResourceFeeCharged()
	require.True(t, ok)
	refundable, ok := ref.SorobanTotalRefundableResourceFeeCharged()
	require.True(t, ok)
	assert.EqualValues(t, 40, nonRefundable)
	assert.EqualValues(t, 10, refundable)
	assert.Equal(t,
		uint64(int64(ref.Result.Result.FeeCharged)-(nonRefundable+refundable)), //nolint:gosec // fixture difference is positive
		got.SorobanInclusionFees[0])
}

// TestExtractFees_SorobanHelpers_V4MetaGap pins the divergence on TxMeta V4:
// the parsed helpers unwrap the extension through UnsafeMeta.GetV3() only, so
// for a V4 meta they report ok=false even though the extension is present and
// both v1 and the extractor read it.
func TestExtractFees_SorobanHelpers_V4MetaGap(t *testing.T) {
	tx := feeTxV1(t, []xdr.Operation{feeInvokeHostFunctionOp()}, true, feeMetaV4Ext1(40, 10), 100)
	lcm := buildLCM(t, 2, 8973, 1_700_085_300, []txWithHash{tx}, false)
	got := requireFeesMatchOracle(t, lcm)
	require.Equal(t, []uint64{50}, got.SorobanInclusionFees, "extractor reads the V4 extension")

	ref := readerTxByHash(t, lcm, tx.hash)
	_, ok := ref.SorobanTotalNonRefundableResourceFeeCharged()
	assert.False(t, ok, "parsed helper is V3-only; the V4 extension is invisible to it")
	_, ok = ref.SorobanTotalRefundableResourceFeeCharged()
	assert.False(t, ok)
}

// TestExtractFees_SorobanHelpers_PanicWhereExtractorSkips pins the divergence
// on the skip cells: where v1 and the extractor silently skip the transaction,
// the parsed helpers panic — Ext.V==0 hits their "unknown SorobanMeta.Ext.V"
// panic, and a missing SorobanMeta nil-derefs.
func TestExtractFees_SorobanHelpers_PanicWhereExtractorSkips(t *testing.T) {
	invoke := feeInvokeHostFunctionOp()
	for _, tc := range []struct {
		name string
		tx   txWithHash
	}{
		{"ext_v0", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext0(), 100)},
		{"missing_soroban_meta", feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3NoSorobanMeta(), 100)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lcm := buildLCM(t, 2, 8974, 1_700_085_400, []txWithHash{tc.tx}, false)
			got := requireFeesMatchOracle(t, lcm)
			assert.Empty(t, got.SorobanInclusionFees, "extractor skips the tx")
			assert.Empty(t, got.ClassicFeesPerOp, "skipped means skipped — not classic")

			ref := readerTxByHash(t, lcm, tc.tx.hash)
			assert.Panics(t, func() { ref.SorobanTotalNonRefundableResourceFeeCharged() })
		})
	}
}

// TestExtractFees_SecondaryOracle_BalanceDeltaRoute: SorobanInclusionFeeCharged
// derives the same inclusion fee via a fully independent route — the fee
// account's balance delta in FeeProcessing (the upfront charge: inclusion fee
// + resource-fee BID) minus the envelope's resource-fee bid — while the
// extractor computes FeeCharged (post-refund: inclusion + resource charged)
// minus the meta's charged resource fees.
func TestExtractFees_SecondaryOracle_BalanceDeltaRoute(t *testing.T) {
	kp := keypair.MustRandom()
	// bid 60; charged 40+10=50; inclusion 25 → FeeCharged 75, upfront 85.
	inner := xdr.Transaction{
		SourceAccount: xdr.MustMuxedAddress(kp.Address()),
		Operations:    []xdr.Operation{feeInvokeHostFunctionOp()},
		Ext:           xdr.TransactionExt{V: 1, SorobanData: &xdr.SorobanTransactionData{ResourceFee: 60}},
	}
	env := xdr.TransactionEnvelope{Type: xdr.EnvelopeTypeEnvelopeTypeTx, V1: &xdr.TransactionV1Envelope{Tx: inner}}
	hash, err := network.HashTransactionInEnvelope(env, viewTestPassphrase)
	require.NoError(t, err)
	res := feeResult(75, true)
	tx := txWithHash{
		env: env, hash: hash, meta: feeMetaV3Ext1(40, 10), result: &res,
		feeChanges: feeAccountChange(kp.Address(), 1_000_085, 1_000_000),
	}

	lcm := buildLCM(t, 2, 8975, 1_700_085_500, []txWithHash{tx}, false)
	got := requireFeesMatchOracle(t, lcm)
	require.Equal(t, []uint64{25}, got.SorobanInclusionFees)

	ref := readerTxByHash(t, lcm, tx.hash)
	incl, ok := ref.SorobanInclusionFeeCharged()
	require.True(t, ok)
	assert.EqualValues(t, got.SorobanInclusionFees[0], incl,
		"balance-delta route must agree with the result/meta route")
	inclViaDispatch, ok := ref.InclusionFeeCharged()
	require.True(t, ok)
	assert.EqualValues(t, got.SorobanInclusionFees[0], inclViaDispatch)
}

// TestExtractFees_BalanceDeltaRoute_MuxedFeeSourceDiverges pins the balance
// route's muxed-account blind spot: FeeAccountAddress returns the M-address,
// the change entries carry the underlying G-address, they never match, and the
// helper reports 0 − bid. The extractor never looks at addresses and stays
// correct.
func TestExtractFees_BalanceDeltaRoute_MuxedFeeSourceDiverges(t *testing.T) {
	kp := keypair.MustRandom()
	accID := xdr.MustAddress(kp.Address())
	muxed := xdr.MuxedAccount{
		Type:     xdr.CryptoKeyTypeKeyTypeMuxedEd25519,
		Med25519: &xdr.MuxedAccountMed25519{Id: 7, Ed25519: *accID.Ed25519},
	}
	inner := xdr.Transaction{
		SourceAccount: muxed,
		Operations:    []xdr.Operation{feeInvokeHostFunctionOp()},
		Ext:           xdr.TransactionExt{V: 1, SorobanData: &xdr.SorobanTransactionData{ResourceFee: 60}},
	}
	env := xdr.TransactionEnvelope{Type: xdr.EnvelopeTypeEnvelopeTypeTx, V1: &xdr.TransactionV1Envelope{Tx: inner}}
	hash, err := network.HashTransactionInEnvelope(env, viewTestPassphrase)
	require.NoError(t, err)
	res := feeResult(75, true)
	tx := txWithHash{
		env: env, hash: hash, meta: feeMetaV3Ext1(40, 10), result: &res,
		feeChanges: feeAccountChange(kp.Address(), 1_000_085, 1_000_000),
	}

	lcm := buildLCM(t, 2, 8976, 1_700_085_600, []txWithHash{tx}, false)
	got := requireFeesMatchOracle(t, lcm)
	require.Equal(t, []uint64{25}, got.SorobanInclusionFees, "extractor is address-blind")

	ref := readerTxByHash(t, lcm, tx.hash)
	incl, ok := ref.SorobanInclusionFeeCharged()
	require.True(t, ok)
	assert.EqualValues(t, -60, incl,
		"balance route misses the muxed fee account and reports 0 − resourceFeeBid")
}

// TestExtractFees_DivergesFromFeeChargedHelper_P20FeeBumpSoroban pins the P20
// divergence: for a fee-bump soroban tx on protocol < 21, the parsed
// FeeCharged() subtracts the refund (working around stellar-core #4188, which
// over-reported the result's FeeCharged pre-P21); v1's IngestFees reads the
// raw result value, so the extractor does too.
func TestExtractFees_DivergesFromFeeChargedHelper_P20FeeBumpSoroban(t *testing.T) {
	kp := keypair.MustRandom()
	feeSource := xdr.MustMuxedAddress(kp.Address())
	inner := xdr.Transaction{
		SourceAccount: xdr.MustMuxedAddress(keypair.MustRandom().Address()),
		Operations:    []xdr.Operation{feeInvokeHostFunctionOp()},
		Ext:           xdr.TransactionExt{V: 1, SorobanData: &xdr.SorobanTransactionData{ResourceFee: 90}},
	}
	env := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTxFeeBump,
		FeeBump: &xdr.FeeBumpTransactionEnvelope{Tx: xdr.FeeBumpTransaction{
			Fee:       55555,
			FeeSource: feeSource,
			InnerTx: xdr.FeeBumpTransactionInnerTx{
				Type: xdr.EnvelopeTypeEnvelopeTypeTx,
				V1:   &xdr.TransactionV1Envelope{Tx: inner},
			},
		}},
	}
	hash, err := network.HashTransactionInEnvelope(env, viewTestPassphrase)
	require.NoError(t, err)
	res := feeFeeBumpResult(innerHashOf(t, env), 300, true)

	// The refund (+40 to the fee source) sits in TxChangesAfter, where the
	// pre-P23 protocols put it.
	meta := feeMetaV3Ext1(30, 20)
	meta.V3.TxChangesAfter = feeAccountChange(kp.Address(), 1_000_000, 1_000_040)
	tx := txWithHash{env: env, hash: hash, meta: meta, result: &res}

	lcm := buildLCM(t, 2, 8977, 1_700_085_700, []txWithHash{tx}, false)
	lcm.V2.LedgerHeader.Header.LedgerVersion = 20

	got := requireFeesMatchOracle(t, lcm)
	require.Equal(t, []uint64{250}, got.SorobanInclusionFees,
		"extractor: raw FeeCharged 300 − (30+20) charged resource fees")

	ref := readerTxByHash(t, lcm, tx.hash)
	feeCharged, ok := ref.FeeCharged()
	require.True(t, ok)
	assert.EqualValues(t, 260, feeCharged,
		"parsed helper on P20: raw 300 − 40 refund")
	assert.EqualValues(t, 300, int64(ref.Result.Result.FeeCharged),
		"the raw result value v1 (and the extractor) read")
}

// TestExtractFees_SorobanResourceFeeBid_IsNotTheChargedFee documents why the
// envelope's SorobanResourceFee (the BID) is not cross-assertable against the
// extractor's subtraction: the bid bounds the charge, it does not equal it.
func TestExtractFees_SorobanResourceFeeBid_IsNotTheChargedFee(t *testing.T) {
	inner := xdr.Transaction{
		SourceAccount: xdr.MustMuxedAddress(keypair.MustRandom().Address()),
		Operations:    []xdr.Operation{feeInvokeHostFunctionOp()},
		Ext:           xdr.TransactionExt{V: 1, SorobanData: &xdr.SorobanTransactionData{ResourceFee: 90}},
	}
	env := xdr.TransactionEnvelope{Type: xdr.EnvelopeTypeEnvelopeTypeTx, V1: &xdr.TransactionV1Envelope{Tx: inner}}
	hash, err := network.HashTransactionInEnvelope(env, viewTestPassphrase)
	require.NoError(t, err)
	res := feeResult(75, true)
	tx := txWithHash{env: env, hash: hash, meta: feeMetaV3Ext1(40, 10), result: &res}

	lcm := buildLCM(t, 2, 8978, 1_700_085_800, []txWithHash{tx}, false)
	got := requireFeesMatchOracle(t, lcm)
	require.Equal(t, []uint64{25}, got.SorobanInclusionFees)

	ref := readerTxByHash(t, lcm, tx.hash)
	bid, ok := ref.SorobanResourceFee()
	require.True(t, ok)
	nonRefundable, ok := ref.SorobanTotalNonRefundableResourceFeeCharged()
	require.True(t, ok)
	refundable, ok := ref.SorobanTotalRefundableResourceFeeCharged()
	require.True(t, ok)
	assert.GreaterOrEqual(t, bid, nonRefundable+refundable,
		"the bid bounds the charged resource fee from above")
	assert.NotEqual(t, bid, nonRefundable+refundable,
		"…and in general does not equal it (refunds)")
}
