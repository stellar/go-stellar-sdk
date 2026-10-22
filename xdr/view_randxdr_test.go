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

// TestView_RandXDR_LedgerEntryLedgerKey checks LedgerEntryView.LedgerKey
// against the marshaled LedgerEntry.LedgerKey on random entries of every type.
func TestView_RandXDR_LedgerEntryLedgerKey(t *testing.T) {
	const iterations = 1000
	gen := randxdr.NewGenerator()
	types := map[LedgerEntryType]bool{}

	for i := range iterations {
		shape := &gxdr.LedgerEntry{}
		gen.Next(shape, nil)

		var entry LedgerEntry
		require.NoError(t, gxdr.Convert(shape, &entry))
		types[entry.Data.Type] = true

		key, err := entry.LedgerKey()
		require.NoError(t, err)
		want, err := key.MarshalBinary()
		require.NoError(t, err)

		data, err := entry.MarshalBinary()
		require.NoError(t, err)
		// A prefix too short for the key gives an error, and the first that
		// holds it gives the key, before the end of the entry. The capacity is
		// capped so a read past the prefix panics.
		n := 0
		for ; n < len(data); n++ {
			if got, e := LedgerEntryView(data[:n:n]).LedgerKey(); e == nil {
				require.Equal(t, want, []byte(got), "iteration %d, prefix %d", i, n)
				break
			}
		}
		require.Less(t, n, len(data), "iteration %d: the key needs the whole entry", i)
		got, err := LedgerEntryView(data).LedgerKey()
		require.NoError(t, err, "iteration %d", i)
		clear(data) // the key must not alias the entry
		require.Equal(t, want, []byte(got), "iteration %d", i)
	}
	require.Len(t, types, len(LedgerEntryTypeMap), "every entry type must be generated")
}

// TestView_RandXDR_TransactionEnvelopeTransaction checks that
// TransactionEnvelopeView.Transaction returns exactly the XDR of the
// transaction the decoded TransactionEnvelope helpers read, for random V0, V1
// and fee-bump envelopes.
func TestView_RandXDR_TransactionEnvelopeTransaction(t *testing.T) {
	const iterations = 200
	gen := randxdr.NewGenerator()
	types := map[EnvelopeType]bool{}

	for i := range iterations {
		shape := &gxdr.TransactionEnvelope{}
		gen.Next(shape, randxdr.LedgerCloseMetaPresets)

		var env TransactionEnvelope
		require.NoError(t, gxdr.Convert(shape, &env))
		types[env.Type] = true
		raw, err := env.MarshalBinary()
		require.NoError(t, err)

		var ext TransactionExt // a V0 transaction has no Soroban data
		switch env.Type {
		case EnvelopeTypeEnvelopeTypeTx:
			ext = env.V1.Tx.Ext
		case EnvelopeTypeEnvelopeTypeTxFeeBump:
			ext = env.FeeBump.Tx.InnerTx.V1.Tx.Ext
		}
		want, err := Transaction{
			SourceAccount: env.SourceAccount(),
			Fee:           Uint32(env.Fee()),
			SeqNum:        SequenceNumber(env.SeqNum()),
			Cond:          env.Preconditions(),
			Memo:          env.Memo(),
			Operations:    env.Operations(),
			Ext:           ext,
		}.MarshalBinary()
		require.NoError(t, err)

		tx, err := TransactionEnvelopeView(raw).Transaction()
		require.NoError(t, err, "iteration %d", i)
		txXDR, err := tx.Raw()
		require.NoError(t, err, "iteration %d", i)
		require.Equal(t, want, txXDR, "iteration %d", i)
	}
	for _, typ := range []EnvelopeType{
		EnvelopeTypeEnvelopeTypeTxV0,
		EnvelopeTypeEnvelopeTypeTx,
		EnvelopeTypeEnvelopeTypeTxFeeBump,
	} {
		require.True(t, types[typ], "no %v envelope was generated", typ)
	}

	_, err := TransactionEnvelopeView{0, 0, 0, 1}.Transaction() // ENVELOPE_TYPE_SCP
	require.Error(t, err)
	_, err = Try(func() []byte { // a V1 type with no transaction
		return TransactionEnvelopeView{0, 0, 0, 2}.MustTransaction().MustRaw()
	})
	require.Error(t, err)
}

// TestView_RandXDR_MuxedAccountToAccountId checks MuxedAccountView.ToAccountId
// against the marshaled MuxedAccount.ToAccountId for random muxed accounts of
// both key types.
func TestView_RandXDR_MuxedAccountToAccountId(t *testing.T) {
	gen := randxdr.NewGenerator()
	types := map[CryptoKeyType]bool{}
	for i := range 100 {
		shape := &gxdr.MuxedAccount{}
		gen.Next(shape, nil)

		var account MuxedAccount
		require.NoError(t, gxdr.Convert(shape, &account))
		types[account.Type] = true
		want, err := account.ToAccountId().MarshalBinary()
		require.NoError(t, err)

		raw, err := account.MarshalBinary()
		require.NoError(t, err)
		// The account is followed by other bytes, as inside an envelope.
		got, err := MuxedAccountView(append(raw, 1, 2, 3, 4)).ToAccountId()
		require.NoError(t, err, "iteration %d", i)
		require.Equal(t, want, []byte(got), "iteration %d", i)
	}
	require.Len(t, types, 2, "both key types must be generated")

	for name, invalid := range map[string][]byte{
		// KEY_TYPE_PRE_AUTH_TX is a CryptoKeyType but not a muxed account type.
		"pre-auth tx key type":    append([]byte{0, 0, 0, 1}, make([]byte, 32)...),
		"truncated ed25519":       append([]byte{0, 0, 0, 0}, make([]byte, 31)...),
		"truncated muxed ed25519": append([]byte{0, 0, 1, 0}, make([]byte, 39)...),
	} {
		_, err := MuxedAccountView(invalid).ToAccountId()
		require.Error(t, err, name)
	}
}
