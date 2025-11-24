package txnbuild

import (
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

type ExtendFootprintTtl struct {
	ExtendTo      uint32
	SourceAccount string
	Ext           xdr.TransactionExt
}

func (f *ExtendFootprintTtl) BuildXDR() (xdr.Operation, error) {
	xdrOp := xdr.ExtendFootprintTtlOp{
		Ext: xdr.ExtensionPoint{
			V: 0,
		},
		ExtendTo: xdr.Uint32(f.ExtendTo),
	}

	body, err := xdr.NewOperationBody(xdr.OperationTypeExtendFootprintTtl, xdrOp)
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to build XDR Operation")
	}

	op := xdr.Operation{Body: body}

	SetOpSourceAccount(&op, f.SourceAccount)
	return op, nil
}

func (f *ExtendFootprintTtl) FromXDR(xdrOp xdr.Operation) error {
	result, ok := xdrOp.Body.GetExtendFootprintTtlOp()
	if !ok {
		return errors.New("error parsing invoke host function operation from xdr")
	}
	f.SourceAccount = accountFromXDR(xdrOp.SourceAccount)
	f.ExtendTo = uint32(result.ExtendTo)
	return nil
}

func (f *ExtendFootprintTtl) Validate() error {
	if f.SourceAccount != "" {
		_, err := xdr.AddressToMuxedAccount(f.SourceAccount)
		if err != nil {
			return NewValidationError("SourceAccount", err.Error())
		}
	}
	return nil
}

func (f *ExtendFootprintTtl) GetSourceAccount() string {
	return f.SourceAccount
}

func (f *ExtendFootprintTtl) BuildTransactionExt() (xdr.TransactionExt, error) {
	return f.Ext, nil
}
