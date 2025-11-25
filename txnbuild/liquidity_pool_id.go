//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite
package txnbuild

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// LiquidityPoolId represents the Stellar liquidity pool id.
type LiquidityPoolId [32]byte

func NewLiquidityPoolId(a, b Asset) (LiquidityPoolId, error) {
	if b.LessThan(a) {
		return LiquidityPoolId{}, fmt.Errorf("AssetA must be <= AssetB")
	}

	xdrAssetA, err := a.ToXDR()
	if err != nil {
		return LiquidityPoolId{}, errors.Wrap(err, "failed to build XDR AssetA ID")
	}

	xdrAssetB, err := b.ToXDR()
	if err != nil {
		return LiquidityPoolId{}, errors.Wrap(err, "failed to build XDR AssetB ID")
	}

	id, err := xdr.NewPoolId(xdrAssetA, xdrAssetB, xdr.LiquidityPoolFeeV18)
	if err != nil {
		return LiquidityPoolId{}, errors.Wrap(err, "failed to build XDR liquidity pool id")
	}
	return LiquidityPoolId(id), nil
}

func (lpi LiquidityPoolId) ToXDR() (xdr.PoolId, error) {
	return xdr.PoolId(lpi), nil
}

func liquidityPoolIdFromXDR(poolId xdr.PoolId) (LiquidityPoolId, error) {
	return LiquidityPoolId(poolId), nil
}
