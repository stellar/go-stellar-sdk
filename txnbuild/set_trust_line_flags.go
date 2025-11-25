package txnbuild

import (
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// TrustLineFlag represents the bitmask flags used to set and clear account authorization options.
type TrustLineFlag uint32

// TrustLineAuthorized is a flag that indicates whether the trustline is authorized.
const TrustLineAuthorized = TrustLineFlag(xdr.TrustLineFlagsAuthorizedFlag)

// TrustLineAuthorizedToMaintainLiabilities is a flag that if set, will allow a trustline to maintain liabilities
// without permitting any other operations.
const TrustLineAuthorizedToMaintainLiabilities = TrustLineFlag(xdr.TrustLineFlagsAuthorizedToMaintainLiabilitiesFlag)

// TrustLineClawbackEnabled is a flag that if set allows clawing back assets.
const TrustLineClawbackEnabled = TrustLineFlag(xdr.TrustLineFlagsTrustlineClawbackEnabledFlag)

// SetTrustLineFlags represents the Stellar set trust line flags operation. See
// https://developers.stellar.org/docs/start/list-of-operations/
type SetTrustLineFlags struct {
	Trustor       string
	Asset         Asset
	SetFlags      []TrustLineFlag
	ClearFlags    []TrustLineFlag
	SourceAccount string
}

// BuildXDR for SetTrustLineFlags  returns a fully configured XDR Operation.
func (stf *SetTrustLineFlags) BuildXDR() (xdr.Operation, error) {
	var xdrOp xdr.SetTrustLineFlagsOp

	// Set XDR address associated with the trustline
	err := xdrOp.Trustor.SetAddress(stf.Trustor)
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to set trustor address")
	}

	// Validate this is an issued asset
	if stf.Asset.IsNative() {
		return xdr.Operation{}, errors.New("trustline doesn't exist for a native (XLM) asset")
	}

	xdrOp.Asset, err = stf.Asset.ToXDR()
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "can't convert asset to XDR")
	}

	xdrOp.ClearFlags = trustLineFlagsToXDR(stf.ClearFlags)
	xdrOp.SetFlags = trustLineFlagsToXDR(stf.SetFlags)

	opType := xdr.OperationTypeSetTrustLineFlags
	body, err := xdr.NewOperationBody(opType, xdrOp)
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to build XDR OperationBody")
	}
	op := xdr.Operation{Body: body}
	SetOpSourceAccount(&op, stf.SourceAccount)
	return op, nil
}

func trustLineFlagsToXDR(flags []TrustLineFlag) xdr.Uint32 {
	var result xdr.Uint32
	for _, flag := range flags {
		result = result | xdr.Uint32(flag)
	}
	return result
}

// FromXDR for SetTrustLineFlags  initialises the txnbuild struct from the corresponding xdr Operation.
func (stf *SetTrustLineFlags) FromXDR(xdrOp xdr.Operation) error {
	op, ok := xdrOp.Body.GetSetTrustLineFlagsOp()
	if !ok {
		return errors.New("error parsing allow_trust operation from xdr")
	}

	stf.SourceAccount = accountFromXDR(xdrOp.SourceAccount)
	stf.Trustor = op.Trustor.Address()
	asset, err := assetFromXDR(op.Asset)
	if err != nil {
		return errors.Wrap(err, "error parsing asset from xdr")
	}
	stf.Asset = asset
	stf.ClearFlags = fromXDRTrustlineFlag(op.ClearFlags)
	stf.SetFlags = fromXDRTrustlineFlag(op.SetFlags)

	return nil
}

func fromXDRTrustlineFlag(flags xdr.Uint32) []TrustLineFlag {
	flagsValue := xdr.TrustLineFlags(flags)
	var result []TrustLineFlag
	if flagsValue.IsAuthorized() {
		result = append(result, TrustLineAuthorized)
	}
	if flagsValue.IsAuthorizedToMaintainLiabilitiesFlag() {
		result = append(result, TrustLineAuthorizedToMaintainLiabilities)
	}
	if flagsValue.IsClawbackEnabledFlag() {
		result = append(result, TrustLineClawbackEnabled)
	}
	return result
}

// Validate for SetTrustLineFlags  validates the required struct fields. It returns an error if any of the fields are
// invalid. Otherwise, it returns nil.
func (stf *SetTrustLineFlags) Validate() error {
	err := validateStellarPublicKey(stf.Trustor)
	if err != nil {
		return NewValidationError("Trustor", err.Error())
	}

	err = validateAssetCode(stf.Asset)
	if err != nil {
		return NewValidationError("Asset", err.Error())
	}
	return nil
}

// GetSourceAccount returns the source account of the operation, or nil if not
// set.
func (stf *SetTrustLineFlags) GetSourceAccount() string {
	return stf.SourceAccount
}
