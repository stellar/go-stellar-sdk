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

		raw, err := LedgerCloseMetaView(data).Raw()
		require.NoError(t, err, "iteration %d", i)
		require.Equal(t, data, raw, "iteration %d", i)

		require.NoError(t, LedgerCloseMetaView(data).ValidateFull(), "iteration %d", i)
	}
}

// TestView_RandXDR_AccessorCorrectness navigates into the view via the union
// arm selector, the nested struct field, and a variable-length array element,
// then compares each sub-view's Raw() bytes against MarshalBinary() on the
// equivalent value field. Any offset-arithmetic bug in the generated
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
		view := LedgerCloseMetaView(data)

		// Discriminant: view must report the same arm as the value.
		vVal, err := view.V()
		require.NoError(t, err)
		require.Equal(t, int32(lcm.V), vVal, "iter %d", i)

		// LedgerHeader() resolves the version arm internally; compare its
		// bytes against the value-side field.
		hdrView, err := view.LedgerHeader()
		require.NoError(t, err)
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
			v0, e := view.V0()
			require.NoError(t, e)
			tp, e := v0.TxProcessing()
			require.NoError(t, e)
			txView, e = tp.At(idx)
			require.NoError(t, e)
		case 1:
			txValue = &lcm.MustV1().TxProcessing[idx]
			v1, e := view.V1()
			require.NoError(t, e)
			tp, e := v1.TxProcessing()
			require.NoError(t, e)
			txView, e = tp.At(idx)
			require.NoError(t, e)
		case 2:
			txValue = &lcm.MustV2().TxProcessing[idx]
			v2, e := view.V2()
			require.NoError(t, e)
			tp, e := v2.TxProcessing()
			require.NoError(t, e)
			txView, e = tp.At(idx)
			require.NoError(t, e)
		}
		txWant, err := txValue.MarshalBinary()
		require.NoError(t, err)
		txGot, err := txView.Raw()
		require.NoError(t, err)
		require.Equal(t, txWant, txGot, "iter %d: TxProcessing[%d]", i, idx)
	}
}

// TestView_RandXDR_Fields exercises the single-walk Fields() locate on the
// TxProcessing element — the struct type the ingest extractors consume via
// Fields(). For a random element, every bundle field must be trimmed to its
// exact wire extent (so `[]byte(f.X)` equals the value-side X marshaled, which
// is what makes the free MetaRaw() / `[]byte(trimmed)` extraction correct), and
// f.View must equal the whole element. This validates the locate's per-field
// offsets and trimming under random shapes, across both element layouts
// (TransactionResultMeta for V0/V1, TransactionResultMetaV1 for V2).
func TestView_RandXDR_Fields(t *testing.T) {
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
		view := LedgerCloseMetaView(data)

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

		switch lcm.V {
		case 0, 1:
			var elem TransactionResultMeta
			var elemView TransactionResultMetaView
			if lcm.V == 0 {
				elem = lcm.MustV0().TxProcessing[idx]
				v0, e := view.V0()
				require.NoError(t, e)
				tp, e := v0.TxProcessing()
				require.NoError(t, e)
				elemView, e = tp.At(idx)
				require.NoError(t, e)
			} else {
				elem = lcm.MustV1().TxProcessing[idx]
				v1, e := view.V1()
				require.NoError(t, e)
				tp, e := v1.TxProcessing()
				require.NoError(t, e)
				elemView, e = tp.At(idx)
				require.NoError(t, e)
			}
			f, e := elemView.Fields()
			require.NoError(t, e)
			require.Equal(t, marshal(&elem), []byte(f.View), "iter %d: View", i)
			require.Equal(t, marshal(&elem.Result), []byte(f.Result), "iter %d: Result", i)
			require.Equal(t, marshal(&elem.FeeProcessing), []byte(f.FeeProcessing), "iter %d: FeeProcessing", i)
			require.Equal(t, marshal(&elem.TxApplyProcessing), []byte(f.TxApplyProcessing), "iter %d: TxApplyProcessing", i)
		case 2:
			elem := lcm.MustV2().TxProcessing[idx]
			v2, e := view.V2()
			require.NoError(t, e)
			tp, e := v2.TxProcessing()
			require.NoError(t, e)
			elemView, e := tp.At(idx)
			require.NoError(t, e)
			f, e := elemView.Fields()
			require.NoError(t, e)
			require.Equal(t, marshal(&elem), []byte(f.View), "iter %d: V1 View", i)
			require.Equal(t, marshal(&elem.Ext), []byte(f.Ext), "iter %d: V1 Ext", i)
			require.Equal(t, marshal(&elem.Result), []byte(f.Result), "iter %d: V1 Result", i)
			require.Equal(t, marshal(&elem.FeeProcessing), []byte(f.FeeProcessing), "iter %d: V1 FeeProcessing", i)
			require.Equal(t, marshal(&elem.TxApplyProcessing), []byte(f.TxApplyProcessing), "iter %d: V1 TxApplyProcessing", i)
			require.Equal(t, marshal(&elem.PostTxApplyFeeProcessing), []byte(f.PostTxApplyFeeProcessing), "iter %d: V1 PostTxApplyFeeProcessing", i)
		}
	}
}
