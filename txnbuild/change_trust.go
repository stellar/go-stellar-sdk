package txnbuild

import (
	"math"

	"github.com/stellar/go-stellar-sdk/amount"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// ChangeTrust represents the Stellar change trust operation. See
// https://developers.stellar.org/docs/start/list-of-operations/
// If Limit is omitted, it defaults to txnbuild.MaxTrustlineLimit.
type ChangeTrust struct {
	Line          ChangeTrustAsset
	Limit         string
	SourceAccount string
}

// MaxTrustlineLimit represents the maximum value that can be set as a trustline limit.
var MaxTrustlineLimit = amount.StringFromInt64(math.MaxInt64)

// RemoveTrustlineOp returns a ChangeTrust operation to remove the trustline of the described asset,
// by setting the limit to "0".
func RemoveTrustlineOp(issuedAsset ChangeTrustAsset) ChangeTrust {
	return ChangeTrust{
		Line:  issuedAsset,
		Limit: "0",
	}
}

// BuildXDR for ChangeTrust returns a fully configured XDR Operation.
func (ct *ChangeTrust) BuildXDR() (xdr.Operation, error) {
	if ct.Line.IsNative() {
		return xdr.Operation{}, errors.New("trustline cannot be extended to a native (XLM) asset")
	}
	xdrLine, err := ct.Line.ToXDR()
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "can't convert trustline asset to XDR")
	}

	if ct.Limit == "" {
		ct.Limit = MaxTrustlineLimit
	}

	xdrLimit, err := amount.Parse(ct.Limit)
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to parse limit amount")
	}

	opType := xdr.OperationTypeChangeTrust
	xdrOp := xdr.ChangeTrustOp{
		Line:  xdrLine,
		Limit: xdrLimit,
	}
	body, err := xdr.NewOperationBody(opType, xdrOp)
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to build XDR OperationBody")
	}
	op := xdr.Operation{Body: body}
	SetOpSourceAccount(&op, ct.SourceAccount)
	return op, nil
}

// FromXDR for ChangeTrust initialises the txnbuild struct from the corresponding xdr Operation.
func (ct *ChangeTrust) FromXDR(xdrOp xdr.Operation) error {
	result, ok := xdrOp.Body.GetChangeTrustOp()
	if !ok {
		return errors.New("error parsing change_trust operation from xdr")
	}

	ct.SourceAccount = accountFromXDR(xdrOp.SourceAccount)
	ct.Limit = amount.String(result.Limit)
	asset, err := assetFromChangeTrustAssetXDR(result.Line)
	if err != nil {
		return errors.Wrap(err, "error parsing asset in change_trust operation")
	}
	ct.Line = asset
	return nil
}

// Validate for ChangeTrust validates the required struct fields. It returns an error if any of the fields are
// invalid. Otherwise, it returns nil.
func (ct *ChangeTrust) Validate() error {
	// only validate limit if it has a value. Empty limit is set to the max trustline limit.
	if ct.Limit != "" {
		err := validateAmount(ct.Limit)
		if err != nil {
			return NewValidationError("Limit", err.Error())
		}
	}

	err := validateChangeTrustAsset(ct.Line)
	if err != nil {
		return NewValidationError("Line", err.Error())
	}
	return nil
}

// GetSourceAccount returns the source account of the operation, or the empty string if not
// set.
func (ct *ChangeTrust) GetSourceAccount() string {
	return ct.SourceAccount
}
