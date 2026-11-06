//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite
package txnbuild

import (
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// LiquidityPoolId represents the Stellar liquidity pool id.
type LiquidityPoolId [32]byte

func NewLiquidityPoolId(a, b Asset) (LiquidityPoolId, error) {
	xdrAssetA, err := a.ToXDR()
	if err != nil {
		return LiquidityPoolId{}, errors.Wrap(err, "failed to build XDR AssetA ID")
	}

	xdrAssetB, err := b.ToXDR()
	if err != nil {
		return LiquidityPoolId{}, errors.Wrap(err, "failed to build XDR AssetB ID")
	}

	// xdr.NewPoolId enforces the pool ordering invariant (strictly AssetA <
	// AssetB). Its error is returned as-is so callers see the same message the
	// XDR layer produces.
	id, err := xdr.NewPoolId(xdrAssetA, xdrAssetB, xdr.LiquidityPoolFeeV18)
	if err != nil {
		return LiquidityPoolId{}, err
	}
	return LiquidityPoolId(id), nil
}

func (lpi LiquidityPoolId) ToXDR() (xdr.PoolId, error) {
	return xdr.PoolId(lpi), nil
}

func liquidityPoolIdFromXDR(poolId xdr.PoolId) (LiquidityPoolId, error) {
	return LiquidityPoolId(poolId), nil
}
