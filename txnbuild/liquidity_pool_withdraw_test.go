package txnbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLiquidityPoolWithdraw(t *testing.T) {
	assetA := NativeAsset{}
	assetB := CreditAsset{
		Code:   "EUR",
		Issuer: "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
	}

	poolId, err := NewLiquidityPoolId(assetA, assetB)
	require.NoError(t, err)

	t.Run("basic", func(t *testing.T) {
		lpd, err := NewLiquidityPoolWithdraw(
			"GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
			AssetAmount{assetA, "0.1000000"},
			AssetAmount{assetB, "0.2000000"},
			"52.5",
		)
		require.NoError(t, err)
		assert.Equal(t, LiquidityPoolWithdraw{
			SourceAccount:   "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
			LiquidityPoolID: poolId,
			Amount:          "52.5",
			MinAmountA:      "0.1000000",
			MinAmountB:      "0.2000000",
		}, lpd)
	})

	t.Run("reversed assets", func(t *testing.T) {
		_, err := NewLiquidityPoolWithdraw(
			"GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
			AssetAmount{assetB, "0.1000000"},
			AssetAmount{assetA, "0.2000000"},
			"52.5",
		)
		require.EqualError(t, err, "AssetA must be < AssetB")
	})

	t.Run("malformed asset", func(t *testing.T) {
		_, err := NewLiquidityPoolWithdraw(
			"GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
			AssetAmount{CreditAsset{Code: "EUR", Issuer: "malformed"}, "0.1000000"},
			AssetAmount{assetB, "0.2000000"},
			"52.5",
		)
		require.ErrorContains(t, err, "failed to build XDR AssetA ID")
	})
}

func TestLiquidityPoolWithdrawRoundTrip(t *testing.T) {
	assetA := NativeAsset{}
	assetB := CreditAsset{
		Code:   "EUR",
		Issuer: "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
	}
	poolId, err := NewLiquidityPoolId(assetA, assetB)
	require.NoError(t, err)

	lpd := &LiquidityPoolWithdraw{
		SourceAccount:   "GB7BDSZU2Y27LYNLALKKALB52WS2IZWYBDGY6EQBLEED3TJOCVMZRH7H",
		LiquidityPoolID: poolId,
		Amount:          "0.1000000",
		MinAmountA:      "0.1000000",
		MinAmountB:      "0.2000000",
	}

	testOperationsMarshalingRoundtrip(t, []Operation{lpd}, false)

	// with muxed accounts
	lpd = &LiquidityPoolWithdraw{
		SourceAccount:   "MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK",
		LiquidityPoolID: poolId,
		Amount:          "0.1000000",
		MinAmountA:      "0.1000000",
		MinAmountB:      "0.2000000",
	}

	testOperationsMarshalingRoundtrip(t, []Operation{lpd}, true)
}
