package xdr

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/gxdr"
	"github.com/stellar/go-stellar-sdk/randxdr"
)

// TestView_RandXDR_RawRoundTrip is a property-style test: for N random values
// of LedgerCloseMeta (which transitively covers TransactionEnvelope, LedgerEntry,
// and most other view types), marshal the value, wrap bytes in a view, then
// Raw() must return the input bytes byte-for-byte. Catches size/offset
// regressions anywhere in the view traversal.
func TestView_RandXDR_RawRoundTrip(t *testing.T) {
	const iterations = 100
	gen := randxdr.NewGenerator()

	for i := range iterations {
		shape := &gxdr.LedgerCloseMeta{}
		gen.Next(shape, randxdr.LedgerCloseMetaPresets)

		var v LedgerCloseMeta
		require.NoError(t, gxdr.Convert(shape, &v))

		data, err := v.MarshalBinary()
		require.NoError(t, err)

		raw, err := NewLedgerCloseMetaView(data).Raw()
		require.NoError(t, err, "iteration %d", i)
		require.Equal(t, data, raw, "iteration %d", i)

		// Views (no walk) must behave identically.
		rawDetached, err := LedgerCloseMetaView{view{d: data}}.Raw()
		require.NoError(t, err, "iteration %d", i)
		require.Equal(t, data, rawDetached, "iteration %d", i)

		require.NoError(t, NewLedgerCloseMetaView(data).ValidateFull(), "iteration %d", i)
	}
}

// nthTxProcessing returns element idx of a TxProcessing array view via All(),
// requiring in-band iteration to reach it without error.
func nthTxProcessing[V any](t *testing.T, all func(func(V, error) bool), idx int) V {
	t.Helper()
	var out V
	i := 0
	found := false
	for elem, err := range all {
		require.NoError(t, err)
		if i == idx {
			out = elem
			found = true
			break
		}
		i++
	}
	require.True(t, found, "element %d not reached", idx)
	return out
}

// TestView_RandXDR_AccessorCorrectness navigates into the view via the union
// arm selector, the nested struct field accessors, and a variable-length array
// element, then compares each sub-view's Raw() bytes against MarshalBinary()
// on the equivalent value field. Any offset-arithmetic bug in the generated
// accessors surfaces as a byte mismatch — the field's type doesn't matter
// because both sides are compared as canonical XDR bytes.
//
// Complements TestView_RandXDR_RawRoundTrip (which proves the top-level slice
// is correct end-to-end) by proving every intermediate navigation step lands
// on the right sub-slice.
func TestView_RandXDR_AccessorCorrectness(t *testing.T) {
	const iterations = 100
	gen := randxdr.NewGenerator()
	rng := rand.New(rand.NewSource(1))

	for i := range iterations {
		shape := &gxdr.LedgerCloseMeta{}
		gen.Next(shape, randxdr.LedgerCloseMetaPresets)

		var lcm LedgerCloseMeta
		require.NoError(t, gxdr.Convert(shape, &lcm))

		data, err := lcm.MarshalBinary()
		require.NoError(t, err)
		view := NewLedgerCloseMetaView(data)

		// Discriminant: view must report the same arm as the value.
		vVal, err := view.V()
		require.NoError(t, err)
		require.Equal(t, int32(lcm.V), vVal, "iter %d", i)

		// Navigate to LedgerHeader via the selected arm and compare bytes
		// against the value-side field.
		var hdrView LedgerHeaderHistoryEntryView
		switch lcm.V {
		case 0:
			v0, e := view.ArmV0()
			require.NoError(t, e)
			hdrView, e = v0.LedgerHeader()
			require.NoError(t, e)
		case 1:
			v1, e := view.ArmV1()
			require.NoError(t, e)
			hdrView, e = v1.LedgerHeader()
			require.NoError(t, e)
		case 2:
			v2, e := view.ArmV2()
			require.NoError(t, e)
			hdrView, e = v2.LedgerHeader()
			require.NoError(t, e)
		}
		hdrWant, err := lcm.LedgerHeaderHistoryEntry().MarshalBinary()
		require.NoError(t, err)
		hdrGot, err := hdrView.Raw()
		require.NoError(t, err)
		require.Equal(t, hdrWant, hdrGot, "iter %d: LedgerHeader", i)

		// Navigate to a random TxProcessing element, if any. V0/V1 use
		// TransactionResultMeta; V2 uses TransactionResultMetaV1. Both satisfy
		// BinaryMarshaler and both view types satisfy Raw(), so we hold them
		// as interfaces for the comparison.
		txCount := lcm.CountTransactions()
		if txCount == 0 {
			continue
		}
		idx := rng.Intn(txCount)
		var txValue interface{ MarshalBinary() ([]byte, error) }
		var txView interface{ Raw() ([]byte, error) }
		switch lcm.V {
		case 0:
			txValue = &lcm.MustV0().TxProcessing[idx]
			v0, e := view.ArmV0()
			require.NoError(t, e)
			tp, e := v0.TxProcessing()
			require.NoError(t, e)
			txView = nthTxProcessing(t, tp.All(), idx)
		case 1:
			txValue = &lcm.MustV1().TxProcessing[idx]
			v1, e := view.ArmV1()
			require.NoError(t, e)
			tp, e := v1.TxProcessing()
			require.NoError(t, e)
			txView = nthTxProcessing(t, tp.All(), idx)
		case 2:
			txValue = &lcm.MustV2().TxProcessing[idx]
			v2, e := view.ArmV2()
			require.NoError(t, e)
			tp, e := v2.TxProcessing()
			require.NoError(t, e)
			txView = nthTxProcessing(t, tp.All(), idx)
		}
		txWant, err := txValue.MarshalBinary()
		require.NoError(t, err)
		txGot, err := txView.Raw()
		require.NoError(t, err)
		require.Equal(t, txWant, txGot, "iter %d: TxProcessing[%d]", i, idx)
	}
}

// TestView_RandXDR_Accessors exercises the lazy field accessors on the
// TxProcessing element — the struct type the ingest extractors consume via
// bundles. For a random element, every bundle accessor's Raw() must equal the
// value-side field marshaled (which is what makes free `MetaRaw()`-style
// extraction correct), the element's own Raw() must equal the whole element,
// and the results must be identical whether fields are read in wire order or
// in reverse (out-of-order access pays catch-up sizing but lands on the same
// offsets). This validates the bundle's per-field offsets under random shapes,
// across both element layouts (TransactionResultMeta for V0/V1,
// TransactionResultMetaV1 for V2).
func TestView_RandXDR_Accessors(t *testing.T) {
	const iterations = 100
	gen := randxdr.NewGenerator()
	rng := rand.New(rand.NewSource(2))

	for i := range iterations {
		shape := &gxdr.LedgerCloseMeta{}
		gen.Next(shape, randxdr.LedgerCloseMetaPresets)

		var lcm LedgerCloseMeta
		require.NoError(t, gxdr.Convert(shape, &lcm))

		data, err := lcm.MarshalBinary()
		require.NoError(t, err)
		view := NewLedgerCloseMetaView(data)

		txCount := lcm.CountTransactions()
		if txCount == 0 {
			continue
		}
		idx := rng.Intn(txCount)

		marshal := func(m interface{ MarshalBinary() ([]byte, error) }) []byte {
			b, e := m.MarshalBinary()
			require.NoError(t, e)
			return b
		}
		rawOf := func(v interface{ Raw() ([]byte, error) }) []byte {
			b, e := v.Raw()
			require.NoError(t, e)
			return b
		}

		switch lcm.V {
		case 0, 1:
			var elem TransactionResultMeta
			var elemView TransactionResultMetaView
			if lcm.V == 0 {
				elem = lcm.MustV0().TxProcessing[idx]
				v0, e := view.ArmV0()
				require.NoError(t, e)
				tp, e := v0.TxProcessing()
				require.NoError(t, e)
				elemView = nthTxProcessing(t, tp.All(), idx)
			} else {
				elem = lcm.MustV1().TxProcessing[idx]
				v1, e := view.ArmV1()
				require.NoError(t, e)
				tp, e := v1.TxProcessing()
				require.NoError(t, e)
				elemView = nthTxProcessing(t, tp.All(), idx)
			}
			require.Equal(t, marshal(&elem), rawOf(elemView), "iter %d: element", i)

			// In wire order.
			result, e := elemView.Result()
			require.NoError(t, e)
			require.Equal(t, marshal(&elem.Result), rawOf(result), "iter %d: Result", i)
			feeProc, e := elemView.FeeProcessing()
			require.NoError(t, e)
			require.Equal(t, marshal(&elem.FeeProcessing), rawOf(feeProc), "iter %d: FeeProcessing", i)
			applyProc, e := elemView.TxApplyProcessing()
			require.NoError(t, e)
			require.Equal(t, marshal(&elem.TxApplyProcessing), rawOf(applyProc), "iter %d: TxApplyProcessing", i)

			// Repeated access must land on identical offsets (accessors are pure).
			applyProc2, e := elemView.TxApplyProcessing()
			require.NoError(t, e)
			require.Equal(t, rawOf(applyProc), rawOf(applyProc2), "iter %d: reverse TxApplyProcessing", i)
			result2, e := elemView.Result()
			require.NoError(t, e)
			require.Equal(t, rawOf(result), rawOf(result2), "iter %d: reverse Result", i)
		case 2:
			elem := lcm.MustV2().TxProcessing[idx]
			v2, e := view.ArmV2()
			require.NoError(t, e)
			tp, e := v2.TxProcessing()
			require.NoError(t, e)
			elemView := nthTxProcessing(t, tp.All(), idx)
			require.Equal(t, marshal(&elem), rawOf(elemView), "iter %d: V1 element", i)

			ext, e := elemView.Ext()
			require.NoError(t, e)
			require.Equal(t, marshal(&elem.Ext), rawOf(ext), "iter %d: V1 Ext", i)
			result, e := elemView.Result()
			require.NoError(t, e)
			require.Equal(t, marshal(&elem.Result), rawOf(result), "iter %d: V1 Result", i)
			feeProc, e := elemView.FeeProcessing()
			require.NoError(t, e)
			require.Equal(t, marshal(&elem.FeeProcessing), rawOf(feeProc), "iter %d: V1 FeeProcessing", i)
			applyProc, e := elemView.TxApplyProcessing()
			require.NoError(t, e)
			require.Equal(t, marshal(&elem.TxApplyProcessing), rawOf(applyProc), "iter %d: V1 TxApplyProcessing", i)
			postFee, e := elemView.PostTxApplyFeeProcessing()
			require.NoError(t, e)
			require.Equal(t, marshal(&elem.PostTxApplyFeeProcessing), rawOf(postFee), "iter %d: V1 PostTxApplyFeeProcessing", i)
		}
	}
}
