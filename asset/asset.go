package asset

import (
	"strings"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// NewNativeAsset creates an Asset representing the native token (XLM).
func NewNativeAsset() *Asset {
	return &Asset{AssetType: &Asset_Native{Native: true}}
}

func NewProtoAsset(asset xdr.Asset) *Asset {
	if asset.IsNative() {
		return NewNativeAsset()
	}
	return &Asset{
		AssetType: &Asset_IssuedAsset{
			IssuedAsset: &IssuedAsset{
				// Need to trim the extra null characters from showing in the code when saving to assetCode
				AssetCode: strings.TrimRight(asset.GetCode(), "\x00"),
				Issuer:    asset.GetIssuer(),
			},
		},
	}
}

func (a *Asset) ToXdrAsset() xdr.Asset {
	if a == nil {
		panic("nil asset")
	}
	switch a := a.AssetType.(type) {
	case *Asset_Native:
		return xdr.MustNewNativeAsset()
	case *Asset_IssuedAsset:
		return xdr.MustNewCreditAsset(a.IssuedAsset.AssetCode, a.IssuedAsset.Issuer)
	}
	panic("unknown asset type")
}

func (a *Asset) Equals(other *Asset) bool {
	// If both assets are the same type (native or issued asset)
	if a.AssetType == nil || other.AssetType == nil {
		return false
	}

	switch a := a.AssetType.(type) {
	case *Asset_Native:
		if b, ok := other.AssetType.(*Asset_Native); ok {
			// Both assets are native; compare the native boolean value.
			// Ideally i could simply be returning true here, but it is more idiomatic to check for the flag equality
			return a.Native == b.Native
		}
	case *Asset_IssuedAsset:
		if b, ok := other.AssetType.(*Asset_IssuedAsset); ok {
			// Both assets are issued assets; compare their asset_code and issuer
			return a.IssuedAsset.AssetCode == b.IssuedAsset.AssetCode &&
				a.IssuedAsset.Issuer == b.IssuedAsset.Issuer
		}
	}

	return false
}
