package ingest

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// Span tests: the write-time half (ExtractLedgerTxParts' element offsets +
// ExtractLedgerTxEnvelopeSpans) and the read-time half
// (LedgerTransactionViewFromParts) must together reproduce, byte for byte and
// field for field, what LedgerTransactionViewByHash produces from the whole
// ledger — on every LCM version, TxSet shape and meta shape the package has a
// fixture for.

// spanFixtureTxs is the fixture set the span tests run on: one transaction per
// shape whose element or envelope layout the spans have to get right — a
// Soroban V3 meta with contract AND diagnostic events, a classic envelope
// under a V3 meta (the soroban gate, which only the envelope can decide), a
// TX_V0 envelope, a fee-bump wrapping a Soroban inner (the envelope span is
// the OUTER envelope), a V4 meta with top-level, diagnostic and per-operation
// events, and a V3 meta with no SorobanMeta at all (the one-empty-slot arity).
func spanFixtureTxs(t testing.TB) []txWithHash {
	t.Helper()

	sorobanWithDiag := sorobanTx(t, "ignored")
	sorobanWithDiag.meta = vMetaV3SorobanWithDiag(
		[]xdr.ContractEvent{vContractEvent("span-v3-ev")},
		[]xdr.DiagnosticEvent{vDiagEvent("span-v3-diag-a"), vDiagEvent("span-v3-diag-b")})

	classicWithDiag := classicV3Tx(t, "ignored")
	classicWithDiag.meta = vMetaV3SorobanWithDiag(
		[]xdr.ContractEvent{vContractEvent("span-classic-ev")},
		[]xdr.DiagnosticEvent{vDiagEvent("span-classic-diag")})

	v4Full := sorobanTx(t, "ignored")
	v4Full.meta = xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{
		Events: []xdr.TransactionEvent{
			{Stage: xdr.TransactionEventStageTransactionEventStageBeforeAllTxs, Event: vContractEvent("span-v4-pre")},
			{Stage: xdr.TransactionEventStageTransactionEventStageAfterTx, Event: vContractEvent("span-v4-post")},
		},
		DiagnosticEvents: []xdr.DiagnosticEvent{vDiagEvent("span-v4-diag")},
		Operations: []xdr.OperationMetaV2{
			{Events: []xdr.ContractEvent{vContractEvent("span-v4-opA")}},
			{Events: nil},
		},
	}}

	return []txWithHash{
		sorobanWithDiag,
		classicWithDiag,
		txV0(t, &xdr.TimeBounds{MinTime: 1, MaxTime: 1_900_000_000}, 11),
		feeBumpTx(t, vMetaV3Soroban([]xdr.ContractEvent{vContractEvent("span-fb-ev")})),
		v4Full,
		feeTxV1(t, []xdr.Operation{feeInvokeHostFunctionOp()}, true, feeMetaV3NoSorobanMeta(), 100),
	}
}

// elemHashOffset is where a TxProcessing element's leading transaction hash
// sits inside the element: byte 0 of an LCM V0/V1 element (a
// TransactionResultMeta opens with TransactionResultPair.transactionHash) and
// byte 4 of a V2 one (a TransactionResultMetaV1 opens with a 4-byte
// ExtensionPoint).
func elemHashOffset(lcmVersion int32) int {
	if lcmVersion == 2 {
		return 4
	}
	return 0
}

// txProcessingArrayView returns the ledger's TxProcessing array view, untrimmed
// (it runs to the end of the buffer) — only its start offset is wanted, for
// the tiling property.
func txProcessingArrayView(t *testing.T, view xdr.LedgerCloseMetaView) []byte {
	t.Helper()
	version, err := view.V()
	require.NoError(t, err)
	switch version {
	case 0:
		v0, verr := view.V0()
		require.NoError(t, verr)
		arr, aerr := v0.TxProcessing()
		require.NoError(t, aerr)
		return arr
	case 1:
		v1, verr := view.V1()
		require.NoError(t, verr)
		arr, aerr := v1.TxProcessing()
		require.NoError(t, aerr)
		return arr
	case 2:
		v2, verr := view.V2()
		require.NoError(t, verr)
		arr, aerr := v2.TxProcessing()
		require.NoError(t, aerr)
		return arr
	default:
		t.Fatalf("unsupported LCM version %d", version)
		return nil
	}
}

// assertElemSpansTile is the tiling property: the element spans partition the
// TxProcessing array exactly — the first starts right after the array's 4-byte
// count prefix, that prefix counts them, and each element ends precisely where
// the next begins, so no ledger byte between them is unaccounted for.
func assertElemSpansTile(t *testing.T, view xdr.LedgerCloseMetaView, txParts []LedgerTxParts) {
	t.Helper()
	arrStart, _, err := viewSpan(view, txProcessingArrayView(t, view))
	require.NoError(t, err)

	count := binary.BigEndian.Uint32(view[arrStart : arrStart+4])
	assert.EqualValues(t, len(txParts), count, "TxProcessing count prefix must count the elements")
	require.NotEmpty(t, txParts)
	assert.Equal(t, arrStart+4, txParts[0].ElemStart, "the first element follows the 4-byte count prefix")

	for i := range txParts {
		assert.Less(t, txParts[i].ElemStart, txParts[i].ElemEnd, "element %d span must be non-empty", i)
		assert.LessOrEqual(t, txParts[i].ElemEnd, len(view), "element %d span must stay inside the ledger", i)
		if i+1 < len(txParts) {
			assert.Equal(t, txParts[i].ElemEnd, txParts[i+1].ElemStart, "elements %d and %d must tile", i, i+1)
		}
	}
}

// assertSameBacking asserts two byte slices are the same bytes AND the same
// memory — the zero-copy contract: nothing on the span path copies.
func assertSameBacking(t *testing.T, want, got []byte, ctx string) {
	t.Helper()
	require.Equal(t, want, got, ctx)
	require.NotEmpty(t, got, ctx)
	assert.True(t, &want[0] == &got[0], "%s must alias the ledger buffer, not a copy", ctx)
}

// assertTransactionViewsEqual compares two LedgerTransactionViews field by
// field (and then whole, so a field added later cannot go unchecked), and
// pins that the rebuilt byte fields alias the same buffer the by-hash path
// read rather than copies of it.
func assertTransactionViewsEqual(t *testing.T, want, got LedgerTransactionView, i int) {
	t.Helper()
	ctx := func(f string) string { return fmt.Sprintf("%s mismatch tx %d", f, i) }

	assert.Equal(t, want.Hash, got.Hash, ctx("Hash"))
	assert.Equal(t, want.ApplicationOrder, got.ApplicationOrder, ctx("ApplicationOrder"))
	assert.Equal(t, want.FeeBump, got.FeeBump, ctx("FeeBump"))
	assert.Equal(t, want.Successful, got.Successful, ctx("Successful"))
	assert.Equal(t, want.Envelope, got.Envelope, ctx("Envelope"))
	assert.Equal(t, want.Result, got.Result, ctx("Result"))
	assert.Equal(t, want.Meta, got.Meta, ctx("Meta"))
	assert.Equal(t, want.DiagnosticEvents, got.DiagnosticEvents, ctx("DiagnosticEvents"))
	assert.Equal(t, want.TransactionEvents, got.TransactionEvents, ctx("TransactionEvents"))
	assert.Equal(t, want.ContractEvents, got.ContractEvents, ctx("ContractEvents"))
	assert.Equal(t, want.LedgerSequence, got.LedgerSequence, ctx("LedgerSequence"))
	assert.Equal(t, want.LedgerCloseTime, got.LedgerCloseTime, ctx("LedgerCloseTime"))
	assert.Equal(t, want, got, ctx("LedgerTransactionView"))

	assertSameBacking(t, want.Envelope, got.Envelope, ctx("Envelope"))
	assertSameBacking(t, want.Result, got.Result, ctx("Result"))
	assertSameBacking(t, want.Meta, got.Meta, ctx("Meta"))
}

// assertSpansRebuildLedger is the whole span contract for one ledger: spans
// out, transactions back in, identical to the by-hash read path.
func assertSpansRebuildLedger(t *testing.T, lcmVersion int32, lcm xdr.LedgerCloseMeta) {
	t.Helper()
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	view := xdr.LedgerCloseMetaView(raw)

	txParts, err := ExtractLedgerTxParts(view)
	require.NoError(t, err)
	require.NotEmpty(t, txParts, "fixture ledger must carry transactions")

	envSpans, err := ExtractLedgerTxEnvelopeSpans(view, viewTestPassphrase)
	require.NoError(t, err)
	require.Len(t, envSpans, len(txParts), "one envelope span per transaction")

	spanByHash := make(map[[32]byte]TxEnvelopeSpan, len(envSpans))
	for k, s := range envSpans {
		_, dup := spanByHash[s.Hash]
		require.False(t, dup, "envelope hashes are unique within a ledger")
		spanByHash[s.Hash] = s
		assert.Less(t, s.Start, s.End, "envelope span %d must be non-empty", k)
		if k > 0 {
			assert.LessOrEqual(t, envSpans[k-1].End, s.Start, "envelope spans %d and %d must not overlap", k-1, k)
		}
	}

	assertElemSpansTile(t, view, txParts)

	ledgerSeq, err := view.LedgerSequence()
	require.NoError(t, err)
	closeTime, err := view.LedgerCloseTime()
	require.NoError(t, err)

	for i, part := range txParts {
		want, found, byHashErr := LedgerTransactionViewByHash(view, part.Hash, viewTestPassphrase)
		require.NoError(t, byHashErr)
		require.True(t, found, "tx %d must be found by hash", i)

		// The element span is exactly the element: its leading bytes are the
		// transaction hash the walk reported...
		elem := raw[part.ElemStart:part.ElemEnd]
		hashOff := elemHashOffset(lcmVersion)
		assert.Equal(t, part.Hash[:], elem[hashOff:hashOff+32], "tx %d element leading hash", i)

		// ...and it encloses the result and meta the by-hash path returns.
		resStart, resEnd, spanErr := viewSpan(raw, want.Result)
		require.NoError(t, spanErr)
		assert.GreaterOrEqual(t, resStart, part.ElemStart, "tx %d result must sit inside the element span", i)
		assert.LessOrEqual(t, resEnd, part.ElemEnd, "tx %d result must sit inside the element span", i)
		metaStart, metaEnd, spanErr := viewSpan(raw, want.Meta)
		require.NoError(t, spanErr)
		assert.GreaterOrEqual(t, metaStart, part.ElemStart, "tx %d meta must sit inside the element span", i)
		assert.LessOrEqual(t, metaEnd, part.ElemEnd, "tx %d meta must sit inside the element span", i)

		// The envelope span, paired by hash, is exactly the envelope.
		envSpan, ok := spanByHash[part.Hash]
		require.True(t, ok, "tx %d must have an envelope span", i)
		env := raw[envSpan.Start:envSpan.End]
		assertSameBacking(t, want.Envelope, env, fmt.Sprintf("tx %d envelope span bytes", i))

		got, fromPartsErr := LedgerTransactionViewFromParts(env, elem, lcmVersion, i, ledgerSeq, closeTime)
		require.NoError(t, fromPartsErr)
		assertTransactionViewsEqual(t, want, got, i)

		// A span generous at the tail (here: everything to the end of the
		// ledger) must trim to the same exact transaction.
		loose, looseErr := LedgerTransactionViewFromParts(
			raw[envSpan.Start:], raw[part.ElemStart:], lcmVersion, i, ledgerSeq, closeTime)
		require.NoError(t, looseErr)
		assert.Equal(t, want, loose, "tx %d rebuilt from over-long spans", i)
	}
}

// TestLedgerTxSpans_RebuildTransactions is the main differential, across LCM
// V0/V1/V2 with a TxSet listed in the reverse of apply order (so anything
// pairing positionally instead of by hash mispairs here).
func TestLedgerTxSpans_RebuildTransactions(t *testing.T) {
	txs := spanFixtureTxs(t)
	for _, version := range []int32{0, 1, 2} {
		t.Run(fmt.Sprintf("lcmV%d", version), func(t *testing.T) {
			lcm := buildLCM(t, version, 9600, 1_700_070_000, txs, true /*reversed TxSet*/)
			assertSpansRebuildLedger(t, version, lcm)
		})
	}
}

// TestLedgerTxSpans_MultiPhaseTxSet runs the same contract over a TxSet
// carrying BOTH phase shapes in one ledger (V=0 components and a V=1
// parallel-txs phase), envelopes reversed within each phase.
func TestLedgerTxSpans_MultiPhaseTxSet(t *testing.T) {
	txs := spanFixtureTxs(t)
	for _, version := range []int32{1, 2} {
		t.Run(fmt.Sprintf("lcmV%d", version), func(t *testing.T) {
			lcm := buildMultiPhaseLCM(t, version, 9610, 1_700_071_000, txs, 3)
			assertSpansRebuildLedger(t, version, lcm)
		})
	}
}

// TestLedgerTxSpans_ParallelTxsPhase runs it over a parallel-txs phase with
// several stages, several clusters and a multi-transaction cluster, laid out
// in neither apply nor reverse-apply order.
func TestLedgerTxSpans_ParallelTxsPhase(t *testing.T) {
	txs := make([]txWithHash, 6)
	for i := range txs {
		txs[i] = sorobanTx(t, fmt.Sprintf("span-ptx-%d", i))
	}
	// Stage0: cluster{tx5,tx4}, cluster{tx3}. Stage1: cluster{tx2,tx1,tx0}.
	layout := [][][]int{
		{{5, 4}, {3}},
		{{2, 1, 0}},
	}
	lcm := buildParallelTxsLCM(t, 9620, 1_700_072_000, txs, layout)
	assertSpansRebuildLedger(t, 2, lcm)
}

// TestLedgerTxEnvelopeSpans_TxSetOrderIsNotApplyOrder pins that the envelope
// spans come back in TxSet order, which here is the reverse of apply order:
// the pairing a consumer does is by hash, and this is the ledger shape that
// punishes it for pairing by position instead.
func TestLedgerTxEnvelopeSpans_TxSetOrderIsNotApplyOrder(t *testing.T) {
	txs := []txWithHash{sorobanTx(t, "a"), sorobanTx(t, "b"), sorobanTx(t, "c")}
	lcm := buildLCM(t, 2, 9630, 1_700_073_000, txs, true /*reversed TxSet*/)
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	view := xdr.LedgerCloseMetaView(raw)

	txParts, err := ExtractLedgerTxParts(view)
	require.NoError(t, err)
	spans, err := ExtractLedgerTxEnvelopeSpans(view, viewTestPassphrase)
	require.NoError(t, err)
	require.Len(t, spans, len(txParts))

	for i := range spans {
		assert.Equal(t, txParts[len(txParts)-1-i].Hash, spans[i].Hash,
			"span %d must be the envelope of the LAST-but-%d transaction in apply order", i, i)
	}
}

// TestLedgerTxSpans_EmptyLedger: a ledger with no transactions yields no spans
// and no error, on both entry points.
func TestLedgerTxSpans_EmptyLedger(t *testing.T) {
	for _, version := range []int32{0, 1, 2} {
		t.Run(fmt.Sprintf("lcmV%d", version), func(t *testing.T) {
			lcm := buildLCM(t, version, 9640, 1_700_074_000, nil, false)
			raw, err := lcm.MarshalBinary()
			require.NoError(t, err)
			view := xdr.LedgerCloseMetaView(raw)

			txParts, err := ExtractLedgerTxParts(view)
			require.NoError(t, err)
			assert.Empty(t, txParts)

			spans, err := ExtractLedgerTxEnvelopeSpans(view, viewTestPassphrase)
			require.NoError(t, err)
			assert.Empty(t, spans)
		})
	}
}

// TestExtractLedgerTxEnvelopeSpans_Errors: a bad passphrase and an unknown LCM
// version are errors, not panics or empty results.
func TestExtractLedgerTxEnvelopeSpans_Errors(t *testing.T) {
	lcm := buildLCM(t, 2, 9650, 1_700_075_000, []txWithHash{sorobanTx(t, "a")}, false)
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)

	_, err = ExtractLedgerTxEnvelopeSpans(xdr.LedgerCloseMetaView(raw), "")
	require.Error(t, err, "an empty passphrase cannot hash envelopes")

	unknown := make([]byte, len(raw))
	copy(unknown, raw)
	binary.BigEndian.PutUint32(unknown[:4], 9)
	_, err = ExtractLedgerTxEnvelopeSpans(xdr.LedgerCloseMetaView(unknown), viewTestPassphrase)
	require.Error(t, err, "an unknown LCM version must error")
}

// spanOfFirstTx returns the first transaction's element and envelope spans,
// sliced out of a freshly built ledger — the starting point for the FromParts
// error cases.
func spanOfFirstTx(t *testing.T, lcmVersion int32) (elem, env []byte) {
	t.Helper()
	txs := []txWithHash{sorobanTx(t, "err-a"), sorobanTx(t, "err-b")}
	lcm := buildLCM(t, lcmVersion, 9660, 1_700_076_000, txs, true)
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	view := xdr.LedgerCloseMetaView(raw)

	txParts, err := ExtractLedgerTxParts(view)
	require.NoError(t, err)
	require.NotEmpty(t, txParts)
	spans, err := ExtractLedgerTxEnvelopeSpans(view, viewTestPassphrase)
	require.NoError(t, err)

	for _, s := range spans {
		if s.Hash == txParts[0].Hash {
			return raw[txParts[0].ElemStart:txParts[0].ElemEnd], raw[s.Start:s.End]
		}
	}
	t.Fatal("no envelope span for the first transaction")
	return nil, nil
}

// TestLedgerTransactionViewFromParts_MalformedInputsError: every truncation of
// either span is an error and never a panic — the exported entry point takes
// bytes from a caller's store, so it has to be total on malformed input.
func TestLedgerTransactionViewFromParts_MalformedInputsError(t *testing.T) {
	elem, env := spanOfFirstTx(t, 2)

	for cut := 0; cut < len(elem); cut++ {
		_, err := LedgerTransactionViewFromParts(env, elem[:cut], 2, 0, 100, 1_700_076_000)
		require.Error(t, err, "a %d-byte element prefix must error", cut)
	}
	for cut := 0; cut < len(env); cut++ {
		_, err := LedgerTransactionViewFromParts(env[:cut], elem, 2, 0, 100, 1_700_076_000)
		require.Error(t, err, "a %d-byte envelope prefix must error", cut)
	}
}

// TestLedgerTransactionViewFromParts_ArgumentErrors: the version discriminant
// and the apply index are validated, and the wrong element view for the
// version is rejected rather than silently misread.
func TestLedgerTransactionViewFromParts_ArgumentErrors(t *testing.T) {
	elemV2, envV2 := spanOfFirstTx(t, 2)
	elemV1, envV1 := spanOfFirstTx(t, 1)

	t.Run("unknown lcm version", func(t *testing.T) {
		_, err := LedgerTransactionViewFromParts(envV2, elemV2, 9, 0, 100, 1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown LCM V=9")
	})
	t.Run("negative apply index", func(t *testing.T) {
		_, err := LedgerTransactionViewFromParts(envV2, elemV2, 2, -1, 100, 1)
		require.Error(t, err)
	})
	t.Run("apply index overflowing int32", func(t *testing.T) {
		_, err := LedgerTransactionViewFromParts(envV2, elemV2, 2, math.MaxInt32, 100, 1)
		require.Error(t, err)
	})
	t.Run("a V1 element read as V2 is not the same transaction", func(t *testing.T) {
		// A TransactionResultMetaV1 opens with a 4-byte ExtensionPoint that a
		// TransactionResultMeta does not have, so the two readings of the same
		// bytes cannot both be right; whichever fails must fail loudly.
		_, err := LedgerTransactionViewFromParts(envV1, elemV1, 2, 0, 100, 1)
		if err == nil {
			t.Fatal("reading a V0/V1 element as a V2 one must not silently succeed")
		}
	})
}

// TestLedgerTransactionViewFromParts_PairingIsTheCallers documents the one
// thing the entry point deliberately does NOT check: that the envelope belongs
// to the element. A mismatched pair is assembled without complaint — the
// caller owns the pairing — so a consumer's index, not this function, is what
// keeps the two spans together.
func TestLedgerTransactionViewFromParts_PairingIsTheCallers(t *testing.T) {
	txs := []txWithHash{sorobanTx(t, "pair-a"), feeBumpTx(t, vMetaV3Soroban(nil))}
	lcm := buildLCM(t, 2, 9670, 1_700_077_000, txs, true)
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	view := xdr.LedgerCloseMetaView(raw)

	txParts, err := ExtractLedgerTxParts(view)
	require.NoError(t, err)
	require.Len(t, txParts, 2)
	spans, err := ExtractLedgerTxEnvelopeSpans(view, viewTestPassphrase)
	require.NoError(t, err)
	require.Len(t, spans, 2)

	spanByHash := map[[32]byte]TxEnvelopeSpan{}
	for _, s := range spans {
		spanByHash[s.Hash] = s
	}
	// Element of tx 0 with the envelope of tx 1 (a fee-bump).
	wrongEnv := spanByHash[txParts[1].Hash]
	mixed, err := LedgerTransactionViewFromParts(
		raw[wrongEnv.Start:wrongEnv.End], raw[txParts[0].ElemStart:txParts[0].ElemEnd], 2, 0, 9670, 1_700_077_000)
	require.NoError(t, err, "a mismatched pair is the caller's problem, not an error here")
	assert.Equal(t, txParts[0].Hash, mixed.Hash, "the hash comes from the element")
	assert.True(t, mixed.FeeBump, "the fee-bump flag comes from the envelope")
}
