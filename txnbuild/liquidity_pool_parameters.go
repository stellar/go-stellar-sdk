//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite
package txnbuild

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const LiquidityPoolFeeV18 = xdr.LiquidityPoolFeeV18

// LiquidityPoolParameters represents the Stellar liquidity pool parameters
type LiquidityPoolParameters struct {
	AssetA Asset
	AssetB Asset
	Fee    int32
}

func (lpi LiquidityPoolParameters) ToXDR() (xdr.LiquidityPoolParameters, error) {
	xdrAssetA, err := lpi.AssetA.ToXDR()
	if err != nil {
		return xdr.LiquidityPoolParameters{}, errors.Wrap(err, "failed to build XDR AssetA ID")
	}

	xdrAssetB, err := lpi.AssetB.ToXDR()
	if err != nil {
		return xdr.LiquidityPoolParameters{}, errors.Wrap(err, "failed to build XDR AssetB ID")
	}

	return xdr.LiquidityPoolParameters{
		Type: xdr.LiquidityPoolTypeLiquidityPoolConstantProduct,
		ConstantProduct: &xdr.LiquidityPoolConstantProductParameters{
			AssetA: xdrAssetA,
			AssetB: xdrAssetB,
			Fee:    xdr.Int32(lpi.Fee),
		},
	}, nil
}

func liquidityPoolParametersFromXDR(params xdr.LiquidityPoolParameters) (LiquidityPoolParameters, error) {
	if params.Type != xdr.LiquidityPoolTypeLiquidityPoolConstantProduct {
		return LiquidityPoolParameters{}, fmt.Errorf("failed to parse XDR type")
	}
	assetA, err := assetFromXDR(params.ConstantProduct.AssetA)
	if err != nil {
		return LiquidityPoolParameters{}, errors.Wrap(err, "failed to parse XDR AssetA")
	}
	assetB, err := assetFromXDR(params.ConstantProduct.AssetB)
	if err != nil {
		return LiquidityPoolParameters{}, errors.Wrap(err, "failed to parse XDR AssetB")
	}
	return LiquidityPoolParameters{
		AssetA: assetA,
		AssetB: assetB,
		Fee:    int32(params.ConstantProduct.Fee),
	}, nil
}
