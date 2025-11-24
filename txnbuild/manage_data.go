package txnbuild

import (
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// ManageData represents the Stellar manage data operation. See
// https://developers.stellar.org/docs/start/list-of-operations/
type ManageData struct {
	Name          string
	Value         []byte
	SourceAccount string
}

// BuildXDR for ManageData returns a fully configured XDR Operation.
func (md *ManageData) BuildXDR() (xdr.Operation, error) {
	xdrOp := xdr.ManageDataOp{DataName: xdr.String64(md.Name)}

	// No data value clears the named data entry on the account
	if md.Value == nil {
		xdrOp.DataValue = nil
	} else {
		xdrDV := xdr.DataValue(md.Value)
		xdrOp.DataValue = &xdrDV
	}

	opType := xdr.OperationTypeManageData
	body, err := xdr.NewOperationBody(opType, xdrOp)
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to build XDR OperationBody")
	}
	op := xdr.Operation{Body: body}
	SetOpSourceAccount(&op, md.SourceAccount)
	return op, nil
}

// FromXDR for ManageData initialises the txnbuild struct from the corresponding xdr Operation.
func (md *ManageData) FromXDR(xdrOp xdr.Operation) error {
	result, ok := xdrOp.Body.GetManageDataOp()
	if !ok {
		return errors.New("error parsing create_account operation from xdr")
	}

	md.SourceAccount = accountFromXDR(xdrOp.SourceAccount)
	md.Name = string(result.DataName)
	if result.DataValue != nil {
		md.Value = *result.DataValue
	} else {
		md.Value = nil
	}
	return nil
}

// Validate for ManageData validates the required struct fields. It returns an error if any
// of the fields are invalid. Otherwise, it returns nil.
func (md *ManageData) Validate() error {
	if len(md.Name) > 64 {
		return NewValidationError("Name", "maximum length is 64 characters")
	}

	if len(md.Value) > 64 {
		return NewValidationError("Value", "maximum length is 64 bytes")
	}
	return nil
}

// GetSourceAccount returns the source account of the operation, or the empty string if not
// set.
func (md *ManageData) GetSourceAccount() string {
	return md.SourceAccount
}
