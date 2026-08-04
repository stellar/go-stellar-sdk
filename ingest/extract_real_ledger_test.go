package ingest_test

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Real-ledger equivalence tests: every public view extractor must produce
// output identical to the eager-decode reference path
// (LedgerTransactionReader / GetTransactionEvents / GetDiagnosticEvents) on a
// real pubnet ledger. The synthetic differential tests elsewhere in the
// package cover shapes a modern ledger cannot contain (LCM V0, older meta
// versions, TX_V0 arms, the V3 soroban gate, cursor and error edges); these
// cover the real thing at production size.

const realLedgerPath = "../xdr/testdata/ledger_58752000.bin"

func loadRealLedger(tb testing.TB) []byte {
	tb.Helper()
	data, err := os.ReadFile(realLedgerPath)
	if err != nil {
		tb.Fatalf("testdata not found: %v", err)
	}
	return data
}

// refTransactions decodes the ledger and drains the parsed reference reader.
func refTransactions(tb testing.TB, raw []byte) []ingest.LedgerTransaction {
	tb.Helper()
	var lcm xdr.LedgerCloseMeta
	require.NoError(tb, xdr.SafeUnmarshal(raw, &lcm))
	reader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(network.PublicNetworkPassphrase, lcm)
	require.NoError(tb, err)
	defer func() { require.NoError(tb, reader.Close()) }()
	var out []ingest.LedgerTransaction
	for {
		tx, err := reader.Read()
		if err == io.EOF {
			return out
		}
		require.NoError(tb, err)
		out = append(out, tx)
	}
}

func TestExtractLedgerTxParts_RealLedgerEquivalence(t *testing.T) {
	raw := loadRealLedger(t)
	refTxs := refTransactions(t, raw)

	txParts, err := ingest.ExtractLedgerTxParts(xdr.LedgerCloseMetaView(raw))
	require.NoError(t, err)
	require.Len(t, txParts, len(refTxs))
	require.NotEmpty(t, txParts, "fixture ledger must carry transactions")

	feeBumps := 0
	for i, ref := range refTxs {
		assert.Equal(t, [32]byte(ref.Hash), txParts[i].Hash, "tx %d hash", i)

		wantFeeBump := ref.Envelope.Type == xdr.EnvelopeTypeEnvelopeTypeTxFeeBump
		assert.Equal(t, wantFeeBump, txParts[i].FeeBump, "tx %d fee-bump flag", i)
		if wantFeeBump {
			feeBumps++
			innerPair := ref.Result.Result.Result.InnerResultPair
			require.NotNil(t, innerPair, "tx %d fee-bump result must carry the inner pair", i)
			assert.Equal(t, [32]byte(innerPair.TransactionHash), txParts[i].InnerHash, "tx %d inner hash", i)
		}

		refResult, err := ref.Result.MarshalBinary()
		require.NoError(t, err)
		assert.Equal(t, refResult, []byte(txParts[i].Result), "tx %d result bytes", i)
		refMeta, err := ref.UnsafeMeta.MarshalBinary()
		require.NoError(t, err)
		assert.Equal(t, refMeta, []byte(txParts[i].Meta), "tx %d meta bytes", i)
	}
	require.NotZero(t, feeBumps, "fixture ledger must carry fee-bumps")
}

func TestEventsFromTxParts_RealLedgerEquivalence(t *testing.T) {
	raw := loadRealLedger(t)
	refTxs := refTransactions(t, raw)

	txParts, err := ingest.ExtractLedgerTxParts(xdr.LedgerCloseMetaView(raw))
	require.NoError(t, err)
	got, err := ingest.EventsFromTxParts(txParts)
	require.NoError(t, err)
	require.Len(t, got, len(refTxs))

	for i, ref := range refTxs {
		require.Equal(t, [32]byte(ref.Hash), txParts[i].Hash, "tx %d hash", i)
		refEvents, err := ref.GetTransactionEvents()
		require.NoError(t, err)

		require.Len(t, got[i].TransactionEvents, len(refEvents.TransactionEvents), "tx %d tx-events", i)
		for j, refEv := range refEvents.TransactionEvents {
			refRaw, err := refEv.MarshalBinary()
			require.NoError(t, err)
			assert.Equal(t, refRaw, got[i].TransactionEvents[j], "tx %d tx-event %d", i, j)
		}
		require.Len(t, got[i].OperationEvents, len(refEvents.OperationEvents), "tx %d op groups", i)
		for op, refOpEvents := range refEvents.OperationEvents {
			require.Len(t, got[i].OperationEvents[op], len(refOpEvents), "tx %d op %d events", i, op)
			for j, refEv := range refOpEvents {
				refRaw, err := refEv.MarshalBinary()
				require.NoError(t, err)
				assert.Equal(t, refRaw, got[i].OperationEvents[op][j], "tx %d op %d event %d", i, op, j)
			}
		}
	}
}

func TestLedgerTransactionViewRange_RealLedgerEquivalence(t *testing.T) {
	raw := loadRealLedger(t)
	refTxs := refTransactions(t, raw)

	got, err := ingest.LedgerTransactionViewRange(
		xdr.LedgerCloseMetaView(raw), 0, 0, network.PublicNetworkPassphrase)
	require.NoError(t, err)
	require.Len(t, got, len(refTxs))

	for i, ref := range refTxs {
		v := got[i]
		require.Equal(t, [32]byte(ref.Hash), v.Hash, "tx %d hash", i)
		assert.Equal(t, int32(i+1), v.ApplicationOrder, "tx %d apply order", i)
		assert.Equal(t, ref.Envelope.Type == xdr.EnvelopeTypeEnvelopeTypeTxFeeBump, v.FeeBump, "tx %d fee-bump", i)
		assert.Equal(t, ref.Result.Result.Successful(), v.Successful, "tx %d successful", i)

		refEnv, err := ref.Envelope.MarshalBinary()
		require.NoError(t, err)
		assert.Equal(t, refEnv, v.Envelope, "tx %d envelope bytes", i)
		refRes, err := ref.Result.Result.MarshalBinary()
		require.NoError(t, err)
		assert.Equal(t, refRes, v.Result, "tx %d result bytes", i)
		refMeta, err := ref.UnsafeMeta.MarshalBinary()
		require.NoError(t, err)
		assert.Equal(t, refMeta, v.Meta, "tx %d meta bytes", i)

		refDiag, err := ref.GetDiagnosticEvents()
		require.NoError(t, err)
		require.Len(t, v.DiagnosticEvents, len(refDiag), "tx %d diagnostics", i)
		for j, refEv := range refDiag {
			refRaw, err := refEv.MarshalBinary()
			require.NoError(t, err)
			assert.Equal(t, refRaw, v.DiagnosticEvents[j], "tx %d diagnostic %d", i, j)
		}
	}
}

// isSorobanFeeShape re-derives the OLD (v1, envelope-based) soroban gate from
// the parsed transaction: exactly one operation of a soroban type.
//
// DELIBERATELY the old definition, not the shipped one — do not "simplify"
// this to read SorobanMeta. FeesFromTxParts classifies from TxProcessing
// alone (SorobanMeta presence, per-op result counts); no production code
// reads envelopes for fees anymore. Re-deriving the expectation from the
// envelope makes this test the one standing place where the two definitions
// are compared on real data — if a protocol change ever breaks the
// "results + meta tell you everything the envelope would" equivalence, this
// is what catches it.
func isSorobanFeeShape(ref ingest.LedgerTransaction) bool {
	ops := ref.Envelope.Operations()
	if len(ops) != 1 {
		return false
	}
	switch ops[0].Body.Type { //nolint:exhaustive
	case xdr.OperationTypeInvokeHostFunction, xdr.OperationTypeExtendFootprintTtl, xdr.OperationTypeRestoreFootprint:
		return true
	default:
		return false
	}
}

// sorobanMetaExtV1 unwraps SorobanTransactionMetaExtV1 from a parsed meta,
// V3 or V4.
func sorobanMetaExtV1(meta xdr.TransactionMeta) (xdr.SorobanTransactionMetaExtV1, bool) {
	switch meta.V {
	case 3:
		if sm := meta.V3.SorobanMeta; sm != nil && sm.Ext.V == 1 {
			return *sm.Ext.V1, true
		}
	case 4:
		if sm := meta.V4.SorobanMeta; sm != nil && sm.Ext.V == 1 {
			return *sm.Ext.V1, true
		}
	}
	return xdr.SorobanTransactionMetaExtV1{}, false
}

func TestFeesFromTxParts_RealLedgerEquivalence(t *testing.T) {
	raw := loadRealLedger(t)

	txParts, err := ingest.ExtractLedgerTxParts(xdr.LedgerCloseMetaView(raw))
	require.NoError(t, err)
	got, err := ingest.FeesFromTxParts(txParts)
	require.NoError(t, err)

	require.NotEmpty(t, got.ClassicFeesPerOp, "fixture ledger must carry classic fees")
	require.NotEmpty(t, got.SorobanInclusionFees, "fixture ledger must carry soroban fees")

	refTxs := refTransactions(t, raw)
	classicIdx, sorobanIdx := 0, 0
	for i, ref := range refTxs {
		ops := ref.Envelope.Operations()
		if len(ops) == 0 {
			continue
		}
		if isSorobanFeeShape(ref) {
			ext, ok := sorobanMetaExtV1(ref.UnsafeMeta)
			require.True(t, ok, "tx %d: every real soroban tx must carry the fee ext", i)
			require.Less(t, sorobanIdx, len(got.SorobanInclusionFees), "tx %d", i)
			gotFee := got.SorobanInclusionFees[sorobanIdx]
			sorobanIdx++

			charged := int64(ext.TotalNonRefundableResourceFeeCharged) + int64(ext.TotalRefundableResourceFeeCharged)
			require.LessOrEqual(t, charged, int64(ref.Result.Result.FeeCharged), "tx %d: real resource fees never exceed FeeCharged", i)
			//nolint:gosec // both sides are non-negative, checked above
			assert.Equal(t, uint64(int64(ref.Result.Result.FeeCharged)-charged), gotFee, "tx %d soroban inclusion fee", i)
			continue
		}

		require.Less(t, classicIdx, len(got.ClassicFeesPerOp), "tx %d", i)
		gotFee := got.ClassicFeesPerOp[classicIdx]
		classicIdx++

		rawFeeCharged := int64(ref.Result.Result.FeeCharged)
		//nolint:gosec // real-ledger fees are non-negative; len(ops) > 0
		assert.Equal(t, uint64(rawFeeCharged)/uint64(len(ops)), gotFee, "tx %d classic per-op", i)
	}
	assert.Equal(t, len(got.ClassicFeesPerOp), classicIdx, "every classic fee accounted for")
	assert.Equal(t, len(got.SorobanInclusionFees), sorobanIdx, "every soroban fee accounted for")
}

func TestLedgerTransactionViewByHash_RealLedgerEquivalence(t *testing.T) {
	raw := loadRealLedger(t)
	refTxs := refTransactions(t, raw)
	require.NotEmpty(t, refTxs)

	// First, middle, last — different positions in TxProcessing and the
	// (differently-ordered) TxSet.
	for _, i := range []int{0, len(refTxs) / 2, len(refTxs) - 1} {
		ref := refTxs[i]
		v, found, err := ingest.LedgerTransactionViewByHash(
			xdr.LedgerCloseMetaView(raw), ref.Hash, network.PublicNetworkPassphrase)
		require.NoError(t, err)
		require.True(t, found, "tx %d (%x)", i, ref.Hash)
		assert.Equal(t, int32(i+1), v.ApplicationOrder)
		refEnv, err := ref.Envelope.MarshalBinary()
		require.NoError(t, err)
		assert.Equal(t, refEnv, v.Envelope)
	}

	_, found, err := ingest.LedgerTransactionViewByHash(
		xdr.LedgerCloseMetaView(raw), [32]byte{0xde, 0xad}, network.PublicNetworkPassphrase)
	require.NoError(t, err)
	require.False(t, found, "absent hash is a clean miss")
}
