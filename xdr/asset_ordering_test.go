package xdr_test

import (
	"bytes"
	"testing"

	"github.com/stellar/go-stellar-sdk/keypair"
	. "github.com/stellar/go-stellar-sdk/xdr"

	"github.com/stretchr/testify/require"
)

func TestAssetLessThanMatchesXDROrdering(t *testing.T) {
	for i := 0; i < 500; i++ {
		issuer1 := keypair.MustRandom().Address()
		issuer2 := keypair.MustRandom().Address()

		assets := []Asset{
			MustNewNativeAsset(),
			MustNewCreditAsset("A", issuer1),
			MustNewCreditAsset("USD", issuer1),
			MustNewCreditAsset("USD", issuer2),
			MustNewCreditAsset("ZZZZ", issuer1),
			MustNewCreditAsset("LONGASSET12", issuer1),
			MustNewCreditAsset("LONGASSET12", issuer2),
			MustNewCreditAsset("AAAAAAAAAAAA", issuer2),
		}

		for _, a := range assets {
			for _, b := range assets {
				aBytes, err := a.MarshalBinary()
				require.NoError(t, err)
				bBytes, err := b.MarshalBinary()
				require.NoError(t, err)

				require.Equal(t, bytes.Compare(aBytes, bBytes) < 0, a.LessThan(b),
					"LessThan disagrees with the XDR encoding for %s vs %s",
					a.StringCanonical(), b.StringCanonical())
			}
		}
	}
}

func TestAssetLessThanComparesIssuerBytesNotStrkey(t *testing.T) {
	a := MustNewCreditAsset("USD", "GCXHWP6ILITHEZWVNCCTPJCT7ZIQ2JGKJH7XXR4VYI7PMAGOIHBMHHHM")
	b := MustNewCreditAsset("USD", "GC3BT2M7I2M5PJWE4VWYRVSOHSE6YBD2QKWVJH4TX7GSA3UHYCDH2YCD")

	require.Less(t, b.GetIssuer(), a.GetIssuer(),
		"precondition: compared as strkey text, b sorts first")

	aIssuer := a.MustAlphaNum4().Issuer.MustEd25519()
	bIssuer := b.MustAlphaNum4().Issuer.MustEd25519()
	require.Negative(t, bytes.Compare(aIssuer[:], bIssuer[:]),
		"precondition: compared as raw bytes, a sorts first")

	require.True(t, a.LessThan(b))
	require.False(t, b.LessThan(a))

	_, err := NewPoolId(a, b, LiquidityPoolFeeV18)
	require.NoError(t, err, "the byte-ordered pair must be accepted")

	_, err = NewPoolId(b, a, LiquidityPoolFeeV18)
	require.Error(t, err, "the strkey-ordered pair must be rejected")
}

func TestNewPoolIdRequiresStrictOrdering(t *testing.T) {
	issuer := keypair.MustRandom().Address()
	asset := MustNewCreditAsset("USD", issuer)
	native := MustNewNativeAsset()

	_, err := NewPoolId(native, asset, LiquidityPoolFeeV18)
	require.NoError(t, err)

	_, err = NewPoolId(asset, asset, LiquidityPoolFeeV18)
	require.Error(t, err, "a pool cannot pair an asset with itself")

	_, err = NewPoolId(native, native, LiquidityPoolFeeV18)
	require.Error(t, err, "two native assets are not a valid pair either")

	_, err = NewPoolId(asset, native, LiquidityPoolFeeV18)
	require.Error(t, err, "a reversed pair is rejected")
}

func TestAssetLessThanIsAStrictOrder(t *testing.T) {
	for i := 0; i < 2000; i++ {
		a := MustNewCreditAsset("USD", keypair.MustRandom().Address())
		b := MustNewCreditAsset("USD", keypair.MustRandom().Address())

		require.False(t, a.LessThan(a))
		if a.LessThan(b) {
			require.False(t, b.LessThan(a))
		}
	}
}

func TestAssetLessThanOrdersByTypeThenCode(t *testing.T) {
	issuer := keypair.MustRandom().Address()

	native := MustNewNativeAsset()
	alphaNum4 := MustNewCreditAsset("USD", issuer)
	alphaNum12 := MustNewCreditAsset("LONGASSET12", issuer)

	require.True(t, native.LessThan(alphaNum4))
	require.True(t, alphaNum4.LessThan(alphaNum12),
		"alphanum4 sorts before alphanum12 regardless of code")

	aaa := MustNewCreditAsset("AAA", issuer)
	bbb := MustNewCreditAsset("BBB", issuer)
	require.True(t, aaa.LessThan(bbb))
}
