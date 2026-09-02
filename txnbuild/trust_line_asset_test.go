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
