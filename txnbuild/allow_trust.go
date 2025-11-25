package txnbuild

import (
	"bytes"
	"fmt"

	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Deprecated: use SetTrustLineFlags instead.
// AllowTrust represents the Stellar allow trust operation. See
// https://developers.stellar.org/docs/start/list-of-operations/
type AllowTrust struct {
	Trustor                        string
	Type                           Asset
	Authorize                      bool
	AuthorizeToMaintainLiabilities bool
	ClawbackEnabled                bool
	SourceAccount                  string
}

// BuildXDR for AllowTrust returns a fully configured XDR Operation.
func (at *AllowTrust) BuildXDR() (xdr.Operation, error) {
	var xdrOp xdr.AllowTrustOp

	// Set XDR address associated with the trustline
	err := xdrOp.Trustor.SetAddress(at.Trustor)
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to set trustor address")
	}

	// Validate this is an issued asset
	if at.Type.IsNative() {
		return xdr.Operation{}, errors.New("trustline doesn't exist for a native (XLM) asset")
	}

	// AllowTrust has a special asset type - map to it
	xdrAsset := xdr.Asset{}

	xdrOp.Asset, err = xdrAsset.ToAssetCode(at.Type.GetCode())
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "can't convert asset for trustline to allow trust asset type")
	}

	// Set XDR auth flag
	if at.Authorize {
		xdrOp.Authorize = xdr.Uint32(xdr.TrustLineFlagsAuthorizedFlag)
	} else if at.AuthorizeToMaintainLiabilities {
		xdrOp.Authorize = xdr.Uint32(xdr.TrustLineFlagsAuthorizedToMaintainLiabilitiesFlag)
	}

	opType := xdr.OperationTypeAllowTrust
	body, err := xdr.NewOperationBody(opType, xdrOp)
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to build XDR OperationBody")
	}
	op := xdr.Operation{Body: body}
	SetOpSourceAccount(&op, at.SourceAccount)
	return op, nil
}

func assetCodeToCreditAsset(assetCode xdr.AssetCode) (CreditAsset, error) {
	switch assetCode.Type {
	case xdr.AssetTypeAssetTypeCreditAlphanum4:
		code := bytes.Trim(assetCode.AssetCode4[:], "\x00")
		return CreditAsset{Code: string(code[:])}, nil
	case xdr.AssetTypeAssetTypeCreditAlphanum12:
		code := bytes.Trim(assetCode.AssetCode12[:], "\x00")
		return CreditAsset{Code: string(code[:])}, nil
	default:
		return CreditAsset{}, fmt.Errorf("unknown asset type: %d", assetCode.Type)
	}

}

// FromXDR for AllowTrust initialises the txnbuild struct from the corresponding xdr Operation.
func (at *AllowTrust) FromXDR(xdrOp xdr.Operation) error {
	result, ok := xdrOp.Body.GetAllowTrustOp()
	if !ok {
		return errors.New("error parsing allow_trust operation from xdr")
	}

	at.SourceAccount = accountFromXDR(xdrOp.SourceAccount)
	at.Trustor = result.Trustor.Address()
	flag := xdr.TrustLineFlags(result.Authorize)
	at.Authorize = flag.IsAuthorized()
	at.AuthorizeToMaintainLiabilities = flag.IsAuthorizedToMaintainLiabilitiesFlag()
	t, err := assetCodeToCreditAsset(result.Asset)
	if err != nil {
		return errors.Wrap(err, "error parsing allow_trust operation from xdr")
	}
	at.Type = t

	return nil
}

// Validate for AllowTrust validates the required struct fields. It returns an error if any of the fields are
// invalid. Otherwise, it returns nil.
func (at *AllowTrust) Validate() error {
	err := validateStellarPublicKey(at.Trustor)
	if err != nil {
		return NewValidationError("Trustor", err.Error())
	}

	err = validateAssetCode(at.Type)
	if err != nil {
		return NewValidationError("Type", err.Error())
	}
	return nil
}

// GetSourceAccount returns the source account of the operation, or the empty string if not
// set.
func (at *AllowTrust) GetSourceAccount() string {
	return at.SourceAccount
}
