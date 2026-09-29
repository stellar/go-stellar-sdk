package txnbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestLiquidityPoolShareTrustLineAssetToXDR(t *testing.T) {
	poolID := LiquidityPoolId{1, 2, 3}

	assetXDR, err := (LiquidityPoolShareTrustLineAsset{LiquidityPoolID: poolID}).ToXDR()
	require.NoError(t, err)
	assert.Equal(t, xdr.AssetTypeAssetTypePoolShare, assetXDR.Type)
	require.NotNil(t, assetXDR.LiquidityPoolId)
	assert.Equal(t, xdr.PoolId(poolID), *assetXDR.LiquidityPoolId)
}

func TestLiquidityPoolShareChangeTrustAssetToTrustLineAssetValidation(t *testing.T) {
	creditAsset := CreditAsset{Code: "USD", Issuer: newKeypair0().Address()}
	testCases := []struct {
		name       string
		parameters LiquidityPoolParameters
		wantError  string
	}{
		{
			name: "missing asset A",
			parameters: LiquidityPoolParameters{
				AssetB: creditAsset,
				Fee:    LiquidityPoolFeeV18,
			},
			wantError: "liquidity pool asset A must not be nil",
		},
		{
			name: "missing asset B",
			parameters: LiquidityPoolParameters{
				AssetA: NativeAsset{},
				Fee:    LiquidityPoolFeeV18,
			},
			wantError: "liquidity pool asset B must not be nil",
		},
		{
			name: "unsupported fee",
			parameters: LiquidityPoolParameters{
				AssetA: NativeAsset{},
				AssetB: creditAsset,
				Fee:    LiquidityPoolFeeV18 + 1,
			},
			wantError: "liquidity pool fee must be 30",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			asset := LiquidityPoolShareChangeTrustAsset{
				LiquidityPoolParameters: testCase.parameters,
			}

			_, err := asset.ToTrustLineAsset()
			require.EqualError(t, err, testCase.wantError)
			_, ok := asset.GetLiquidityPoolID()
			assert.False(t, ok)
		})
	}
}
