package txnbuild

import (
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// ChangeTrustAsset represents a Stellar change trust asset.
type ChangeTrustAsset interface {
	BasicAsset
	GetLiquidityPoolID() (LiquidityPoolId, bool)
	GetLiquidityPoolParameters() (LiquidityPoolParameters, bool)
	ToXDR() (xdr.ChangeTrustAsset, error)
	ToChangeTrustAsset() (ChangeTrustAsset, error)
	ToTrustLineAsset() (TrustLineAsset, error)
}

// LiquidityPoolShareChangeTrustAsset represents non-XLM assets on the Stellar network.
type LiquidityPoolShareChangeTrustAsset struct {
	LiquidityPoolParameters LiquidityPoolParameters
}

// GetType for LiquidityPoolShareChangeTrustAsset returns the enum type of the asset, based on its code length.
func (lpsa LiquidityPoolShareChangeTrustAsset) GetType() (AssetType, error) {
	return AssetTypePoolShare, nil
}

// IsNative for LiquidityPoolShareChangeTrustAsset returns false (this is not an XLM asset).
func (lpsa LiquidityPoolShareChangeTrustAsset) IsNative() bool { return false }

// GetCode for LiquidityPoolShareChangeTrustAsset returns blank string
func (lpsa LiquidityPoolShareChangeTrustAsset) GetCode() string { return "" }

// GetIssuer for LiquidityPoolShareChangeTrustAsset returns blank string
func (lpsa LiquidityPoolShareChangeTrustAsset) GetIssuer() string { return "" }

// GetLiquidityPoolID for LiquidityPoolShareChangeTrustAsset returns the pool id computed from the parameters.
func (lpsa LiquidityPoolShareChangeTrustAsset) GetLiquidityPoolID() (LiquidityPoolId, bool) {
	poolId, err := NewLiquidityPoolId(lpsa.LiquidityPoolParameters.AssetA, lpsa.LiquidityPoolParameters.AssetB)
	return poolId, err == nil
}

// GetLiquidityPoolParameters for LiquidityPoolShareChangeTrustAsset returns the pool parameters.
func (lpsa LiquidityPoolShareChangeTrustAsset) GetLiquidityPoolParameters() (LiquidityPoolParameters, bool) {
	return lpsa.LiquidityPoolParameters, true
}

// ToXDR for LiquidityPoolShareChangeTrustAsset produces a corresponding XDR change trust asset.
func (lpsa LiquidityPoolShareChangeTrustAsset) ToXDR() (xdr.ChangeTrustAsset, error) {
	xdrPoolParams, err := lpsa.LiquidityPoolParameters.ToXDR()
	if err != nil {
		return xdr.ChangeTrustAsset{}, errors.Wrap(err, "failed to build XDR liquidity pool parameters")
	}
	return xdr.ChangeTrustAsset{Type: xdr.AssetTypeAssetTypePoolShare, LiquidityPool: &xdrPoolParams}, nil
}

// ToAsset for LiquidityPoolShareChangeTrustAsset returns an error.
func (lpsa LiquidityPoolShareChangeTrustAsset) ToAsset() (Asset, error) {
	return nil, errors.New("Cannot transform LiquidityPoolShare into Asset")
}

// MustToAsset for LiquidityPoolShareChangeTrustAsset panics.
func (lpsa LiquidityPoolShareChangeTrustAsset) MustToAsset() Asset {
	panic("Cannot transform LiquidityPoolShare into Asset")
}

// ToChangeTrustAsset for LiquidityPoolShareChangeTrustAsset returns itself unchanged.
func (lpsa LiquidityPoolShareChangeTrustAsset) ToChangeTrustAsset() (ChangeTrustAsset, error) {
	return lpsa, nil
}

// MustToChangeTrustAsset for LiquidityPoolShareChangeTrustAsset returns itself unchanged.
func (lpsa LiquidityPoolShareChangeTrustAsset) MustToChangeTrustAsset() ChangeTrustAsset {
	return lpsa
}

// ToTrustLineAsset for LiquidityPoolShareChangeTrustAsset hashes the pool parameters to get the pool id, and converts this to a TrustLineAsset.
func (lpsa LiquidityPoolShareChangeTrustAsset) ToTrustLineAsset() (TrustLineAsset, error) {
	poolId, err := NewLiquidityPoolId(lpsa.LiquidityPoolParameters.AssetA, lpsa.LiquidityPoolParameters.AssetB)
	if err != nil {
		return nil, err
	}
	return LiquidityPoolShareTrustLineAsset{
		LiquidityPoolID: poolId,
	}, nil
}

// MustToTrustLineAsset for LiquidityPoolShareChangeTrustAsset hashes the pool parameters to get the pool id, and converts this to a TrustLineAsset. It panics on failure.
func (lpsa LiquidityPoolShareChangeTrustAsset) MustToTrustLineAsset() TrustLineAsset {
	tla, err := lpsa.ToTrustLineAsset()
	if err != nil {
		panic(err)
	}
	return tla
}

func assetFromChangeTrustAssetXDR(xAsset xdr.ChangeTrustAsset) (ChangeTrustAsset, error) {
	if xAsset.Type == xdr.AssetTypeAssetTypePoolShare {
		params, err := liquidityPoolParametersFromXDR(*xAsset.LiquidityPool)
		if err != nil {
			return nil, errors.Wrap(err, "invalid XDR liquidity pool parameters")
		}
		return LiquidityPoolShareChangeTrustAsset{LiquidityPoolParameters: params}, nil
	}
	a, err := assetFromXDR(xAsset.ToAsset())
	if err != nil {
		return nil, err
	}
	return ChangeTrustAssetWrapper{a}, nil
}

// ChangeTrustAssetWrapper wraps a native/credit Asset so it generates xdr to be used in a change trust operation.
type ChangeTrustAssetWrapper struct {
	Asset
}

// GetLiquidityPoolID for ChangeTrustAssetWrapper returns false.
func (ctaw ChangeTrustAssetWrapper) GetLiquidityPoolID() (LiquidityPoolId, bool) {
	return LiquidityPoolId{}, false
}

// GetLiquidityPoolParameters for ChangeTrustAssetWrapper returns false.
func (ctaw ChangeTrustAssetWrapper) GetLiquidityPoolParameters() (LiquidityPoolParameters, bool) {
	return LiquidityPoolParameters{}, false
}

// ToXDR for ChangeTrustAssetWrapper generates the xdr.TrustLineAsset.
func (ctaw ChangeTrustAssetWrapper) ToXDR() (xdr.ChangeTrustAsset, error) {
	x, err := ctaw.Asset.ToXDR()
	if err != nil {
		return xdr.ChangeTrustAsset{}, err
	}
	return x.ToChangeTrustAsset(), nil
}
