package txnbuild

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/price"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLiquidityPoolDeposit(t *testing.T) {
	assetA := NativeAsset{}
	assetB := CreditAsset{
		Code:   "EUR",
		Issuer: "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
	}

	poolId, err := NewLiquidityPoolId(assetA, assetB)
	require.NoError(t, err)

	t.Run("basic", func(t *testing.T) {
		lpd, err := NewLiquidityPoolDeposit(
			"GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
			AssetAmount{assetA, "0.1000000"},
			AssetAmount{assetB, "0.2000000"},
			price.MustParse("0.3"),
			price.MustParse("0.4"),
		)
		require.NoError(t, err)
		assert.Equal(t, LiquidityPoolDeposit{
			SourceAccount:   "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
			LiquidityPoolID: poolId,
			MaxAmountA:      "0.1000000",
			MaxAmountB:      "0.2000000",
			MinPrice:        price.MustParse("0.3"),
			MaxPrice:        price.MustParse("0.4"),
		}, lpd)
	})

	t.Run("reversed assets", func(t *testing.T) {
		_, err := NewLiquidityPoolDeposit(
			"GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
			AssetAmount{assetB, "0.1000000"},
			AssetAmount{assetA, "0.2000000"},
			price.MustParse("0.3"),
			price.MustParse("0.4"),
		)
		require.EqualError(t, err, "AssetA must be <= AssetB")
	})
}

func TestLiquidityPoolDepositRoundTrip(t *testing.T) {
	assetA := NativeAsset{}
	assetB := CreditAsset{
		Code:   "EUR",
		Issuer: "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
	}
	poolId, err := NewLiquidityPoolId(assetA, assetB)
	require.NoError(t, err)

	lpd := &LiquidityPoolDeposit{
		SourceAccount:   "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
		LiquidityPoolID: poolId,
		MaxAmountA:      "0.1000000",
		MaxAmountB:      "0.2000000",
		MinPrice:        price.MustParse("0.3"),
		MaxPrice:        price.MustParse("0.4"),
	}

	testOperationsMarshalingRoundtrip(t, []Operation{lpd}, false)

	// with muxed accounts
	lpd = &LiquidityPoolDeposit{
		SourceAccount:   "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK",
		LiquidityPoolID: poolId,
		MaxAmountA:      "0.1000000",
		MaxAmountB:      "0.2000000",
		MinPrice:        price.MustParse("0.3"),
		MaxPrice:        price.MustParse("0.4"),
	}

	testOperationsMarshalingRoundtrip(t, []Operation{lpd}, true)
}
