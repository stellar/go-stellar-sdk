package txnbuild

import (
	"bytes"

	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// AssetType represents the type of a Stellar asset.
type AssetType xdr.AssetType

// AssetTypeNative, AssetTypeCreditAlphanum4, AssetTypeCreditAlphanum12 enumerate the different
// types of asset on the Stellar network.
const (
	AssetTypeNative           AssetType = AssetType(xdr.AssetTypeAssetTypeNative)
	AssetTypeCreditAlphanum4  AssetType = AssetType(xdr.AssetTypeAssetTypeCreditAlphanum4)
	AssetTypeCreditAlphanum12 AssetType = AssetType(xdr.AssetTypeAssetTypeCreditAlphanum12)
	AssetTypePoolShare        AssetType = AssetType(xdr.AssetTypeAssetTypePoolShare)
)

// Breaks out some stuff common to all assets
type BasicAsset interface {
	GetType() (AssetType, error)
	IsNative() bool
	GetCode() string
	GetIssuer() string

	// Conversions to the 3 asset types
	ToAsset() (Asset, error)
	MustToAsset() Asset

	ToChangeTrustAsset() (ChangeTrustAsset, error)
	MustToChangeTrustAsset() ChangeTrustAsset

	ToTrustLineAsset() (TrustLineAsset, error)
	MustToTrustLineAsset() TrustLineAsset
}

// Asset represents a Stellar asset.
type Asset interface {
	BasicAsset
	LessThan(other Asset) bool
	ToXDR() (xdr.Asset, error)
}

// NativeAsset represents the native XLM asset.
type NativeAsset struct{}

// GetType for NativeAsset returns the enum type of the asset.
func (na NativeAsset) GetType() (AssetType, error) {
	return AssetTypeNative, nil
}

// IsNative for NativeAsset returns true (this is an XLM asset).
func (na NativeAsset) IsNative() bool { return true }

// GetCode for NativeAsset returns an empty string (XLM doesn't have a code).
func (na NativeAsset) GetCode() string { return "" }

// GetIssuer for NativeAsset returns an empty string (XLM doesn't have an issuer).
func (na NativeAsset) GetIssuer() string { return "" }

// LessThan returns true if this asset sorts before some other asset.
func (na NativeAsset) LessThan(other Asset) bool { return true }

// ToXDR for NativeAsset produces a corresponding XDR asset.
func (na NativeAsset) ToXDR() (xdr.Asset, error) {
	xdrAsset := xdr.Asset{}
	err := xdrAsset.SetNative()
	if err != nil {
		return xdr.Asset{}, err
	}
	return xdrAsset, nil
}

// ToAsset for NativeAsset returns itself unchanged.
func (na NativeAsset) ToAsset() (Asset, error) {
	return na, nil
}

// MustToAsset for NativeAsset returns itself unchanged.
func (na NativeAsset) MustToAsset() Asset {
	return na
}

// ToChangeTrustAsset for NativeAsset produces a corresponding ChangeTrustAsset.
func (na NativeAsset) ToChangeTrustAsset() (ChangeTrustAsset, error) {
	return ChangeTrustAssetWrapper{na}, nil
}

// MustToChangeTrustAsset for NativeAsset produces a corresponding ChangeTrustAsset.
func (na NativeAsset) MustToChangeTrustAsset() ChangeTrustAsset {
	return ChangeTrustAssetWrapper{na}
}

// ToTrustLineAsset for NativeAsset produces a corresponding TrustLineAsset.
func (na NativeAsset) ToTrustLineAsset() (TrustLineAsset, error) {
	return TrustLineAssetWrapper{na}, nil
}

// MustToTrustLineAsset for NativeAsset produces a corresponding TrustLineAsset.
func (na NativeAsset) MustToTrustLineAsset() TrustLineAsset {
	return TrustLineAssetWrapper{na}
}

// CreditAsset represents non-XLM assets on the Stellar network.
type CreditAsset struct {
	Code   string
	Issuer string
}

// GetType for CreditAsset returns the enum type of the asset, based on its code length.
func (ca CreditAsset) GetType() (AssetType, error) {
	switch {
	case len(ca.Code) >= 1 && len(ca.Code) <= 4:
		return AssetTypeCreditAlphanum4, nil
	case len(ca.Code) >= 5 && len(ca.Code) <= 12:
		return AssetTypeCreditAlphanum12, nil
	default:
		return AssetTypeCreditAlphanum4, errors.New("asset code length must be between 1 and 12 characters")
	}
}

// IsNative for CreditAsset returns false (this is not an XLM asset).
func (ca CreditAsset) IsNative() bool { return false }

// GetCode for CreditAsset returns the asset code.
func (ca CreditAsset) GetCode() string { return ca.Code }

// GetIssuer for CreditAsset returns the address of the issuing account.
func (ca CreditAsset) GetIssuer() string { return ca.Issuer }

// LessThan returns true if this asset sorts before some other asset.
func (ca CreditAsset) LessThan(other Asset) bool {
	caXDR, err := ca.ToXDR()
	if err != nil {
		return false
	}
	otherXDR, err := other.ToXDR()
	if err != nil {
		return false
	}
	return caXDR.LessThan(otherXDR)
}

// ToXDR for CreditAsset produces a corresponding XDR asset.
func (ca CreditAsset) ToXDR() (xdr.Asset, error) {
	xdrAsset := xdr.Asset{}
	var issuer xdr.AccountId

	err := issuer.SetAddress(ca.Issuer)
	if err != nil {
		return xdr.Asset{}, err
	}

	err = xdrAsset.SetCredit(ca.Code, issuer)
	if err != nil {
		return xdr.Asset{}, errors.Wrap(err, "asset code length must be between 1 and 12 characters")
	}

	return xdrAsset, nil
}

// ToAsset for CreditAsset returns itself unchanged.
func (ca CreditAsset) ToAsset() (Asset, error) {
	return ca, nil
}

// MustToAsset for CreditAsset returns itself unchanged.
func (ca CreditAsset) MustToAsset() Asset {
	return ca
}

// ToChangeTrustAsset for CreditAsset produces a corresponding XDR asset.
func (ca CreditAsset) ToChangeTrustAsset() (ChangeTrustAsset, error) {
	return ChangeTrustAssetWrapper{ca}, nil
}

// MustToChangeTrustAsset for CreditAsset produces a corresponding XDR asset.
func (ca CreditAsset) MustToChangeTrustAsset() ChangeTrustAsset {
	return ChangeTrustAssetWrapper{ca}
}

// ToTrustLineAsset for CreditAsset produces a corresponding XDR asset.
func (ca CreditAsset) ToTrustLineAsset() (TrustLineAsset, error) {
	return TrustLineAssetWrapper{ca}, nil
}

// MustToTrustLineAsset for CreditAsset produces a corresponding XDR asset.
func (ca CreditAsset) MustToTrustLineAsset() TrustLineAsset {
	return TrustLineAssetWrapper{ca}
}

// to do: consider exposing function or adding it to asset interface
func assetFromXDR(xAsset xdr.Asset) (Asset, error) {
	switch xAsset.Type {
	case xdr.AssetTypeAssetTypeNative:
		return NativeAsset{}, nil
	case xdr.AssetTypeAssetTypeCreditAlphanum4:
		code := bytes.Trim(xAsset.AlphaNum4.AssetCode[:], "\x00")
		return CreditAsset{
			Code:   string(code[:]),
			Issuer: xAsset.AlphaNum4.Issuer.Address(),
		}, nil
	case xdr.AssetTypeAssetTypeCreditAlphanum12:
		code := bytes.Trim(xAsset.AlphaNum12.AssetCode[:], "\x00")
		return CreditAsset{
			Code:   string(code[:]),
			Issuer: xAsset.AlphaNum12.Issuer.Address(),
		}, nil
	}

	return nil, errors.New("invalid asset")
}

// MustAssetFromXDR constructs an Asset from its xdr representation.
// If the given asset xdr is invalid, MustAssetFromXDR will panic.
func MustAssetFromXDR(xAsset xdr.Asset) Asset {
	asset, err := assetFromXDR(xAsset)
	if err != nil {
		panic(err)
	}
	return asset
}
