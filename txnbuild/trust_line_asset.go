package txnbuild

import (
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// TrustLineAsset represents a Stellar trust line asset.
type TrustLineAsset interface {
	BasicAsset
	GetLiquidityPoolID() (LiquidityPoolId, bool)
	ToXDR() (xdr.TrustLineAsset, error)
}

// LiquidityPoolShareTrustLineAsset represents shares in a liquidity pool on the Stellar network.
type LiquidityPoolShareTrustLineAsset struct {
	LiquidityPoolID LiquidityPoolId
}

// GetType for LiquidityPoolShareAsset returns the enum type of the asset, based on its code length.
func (lpsa LiquidityPoolShareTrustLineAsset) GetType() (AssetType, error) {
	return AssetTypePoolShare, nil
}

// IsNative for LiquidityPoolShareAsset returns false (this is not an XLM asset).
func (lpsa LiquidityPoolShareTrustLineAsset) IsNative() bool { return false }

// GetCode for LiquidityPoolShareAsset returns blank string
func (lpsa LiquidityPoolShareTrustLineAsset) GetCode() string { return "" }

// GetIssuer for LiquidityPoolShareAsset returns blank string
func (lpsa LiquidityPoolShareTrustLineAsset) GetIssuer() string { return "" }

// GetLiquidityPoolID for LiquidityPoolShareTrustLineAsset returns the pool id.
func (lpsa LiquidityPoolShareTrustLineAsset) GetLiquidityPoolID() (LiquidityPoolId, bool) {
	return lpsa.LiquidityPoolID, true
}

// ToXDR for LiquidityPoolShareAsset produces a corresponding XDR asset.
func (lpsa LiquidityPoolShareTrustLineAsset) ToXDR() (xdr.TrustLineAsset, error) {
	xdrPoolId, err := lpsa.LiquidityPoolID.ToXDR()
	if err != nil {
		return xdr.TrustLineAsset{}, errors.Wrap(err, "failed to build XDR liquidity pool id")
	}
	return xdr.TrustLineAsset{LiquidityPoolId: &xdrPoolId}, nil
}

// ToAsset for LiquidityPoolShareTrustLineAsset returns an error.
func (lpsa LiquidityPoolShareTrustLineAsset) ToAsset() (Asset, error) {
	return nil, errors.New("Cannot transform LiquidityPoolShare into Asset")
}

// MustToAsset for LiquidityPoolShareTrustLineAsset panics.
func (lpsa LiquidityPoolShareTrustLineAsset) MustToAsset() Asset {
	panic("Cannot transform LiquidityPoolShare into Asset")
}

// ToChangeTrustAsset for LiquidityPoolShareTrustLineAsset returns an error.
func (lpsa LiquidityPoolShareTrustLineAsset) ToChangeTrustAsset() (ChangeTrustAsset, error) {
	return nil, errors.New("Cannot transform LiquidityPoolShare into ChangeTrustAsset")
}

// MustToChangeTrustAsset for LiquidityPoolShareTrustLineAsset panics.
func (lpsa LiquidityPoolShareTrustLineAsset) MustToChangeTrustAsset() ChangeTrustAsset {
	panic("Cannot transform LiquidityPoolShare into ChangeTrustAsset")
}

// ToTrustLineAsset for LiquidityPoolShareTrustLineAsset returns itself unchanged.
func (lpsa LiquidityPoolShareTrustLineAsset) ToTrustLineAsset() (TrustLineAsset, error) {
	return lpsa, nil
}

// MustToTrustLineAsset for LiquidityPoolShareTrustLineAsset returns itself unchanged.
func (lpsa LiquidityPoolShareTrustLineAsset) MustToTrustLineAsset() TrustLineAsset {
	return lpsa
}

func assetFromTrustLineAssetXDR(xAsset xdr.TrustLineAsset) (TrustLineAsset, error) {
	if xAsset.Type == xdr.AssetTypeAssetTypePoolShare {
		if xAsset.LiquidityPoolId == nil {
			return nil, errors.New("invalid XDR liquidity pool id")
		}
		poolId, err := liquidityPoolIdFromXDR(*xAsset.LiquidityPoolId)
		if err != nil {
			return nil, errors.Wrap(err, "invalid XDR liquidity pool id")
		}
		return LiquidityPoolShareTrustLineAsset{LiquidityPoolID: poolId}, nil
	}
	a, err := assetFromXDR(xAsset.ToAsset())
	if err != nil {
		return nil, err
	}
	return TrustLineAssetWrapper{a}, nil
}

// TrustLineAssetWrapper wraps a native/credit Asset so it generates xdr to be used in a trust line operation.
type TrustLineAssetWrapper struct {
	Asset
}

// GetLiquidityPoolID for TrustLineAssetWrapper returns false.
func (tlaw TrustLineAssetWrapper) GetLiquidityPoolID() (LiquidityPoolId, bool) {
	return LiquidityPoolId{}, false
}

// ToXDR for TrustLineAssetWrapper generates the xdr.TrustLineAsset.
func (tlaw TrustLineAssetWrapper) ToXDR() (xdr.TrustLineAsset, error) {
	x, err := tlaw.Asset.ToXDR()
	if err != nil {
		return xdr.TrustLineAsset{}, err
	}
	return x.ToTrustLineAsset(), nil
}
