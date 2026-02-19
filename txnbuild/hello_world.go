//go:build xdr_hello_world

package txnbuild

import (
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// HelloWorld represents the Stellar HelloWorld operation.
type HelloWorld struct {
	SourceAccount string
	HelloTo       string
}

// BuildXDR for HelloWorld returns a fully configured XDR Operation.
func (hw *HelloWorld) BuildXDR() (xdr.Operation, error) {
	var helloToAccountID xdr.AccountId
	err := helloToAccountID.SetAddress(hw.HelloTo)
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to set HelloTo address")
	}

	opType := xdr.OperationTypeHelloWorld
	body, err := xdr.NewOperationBody(opType, xdr.HelloWorldOp{
		HelloTo: helloToAccountID,
	})
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to build XDR OperationBody")
	}
	op := xdr.Operation{Body: body}
	SetOpSourceAccount(&op, hw.SourceAccount)
	return op, nil
}

// FromXDR for HelloWorld initialises the txnbuild struct from the corresponding xdr Operation.
func (hw *HelloWorld) FromXDR(xdrOp xdr.Operation) error {
	if xdrOp.Body.Type != xdr.OperationTypeHelloWorld {
		return errors.New("error parsing hello_world operation from xdr")
	}

	op := xdrOp.Body.MustHelloWorldOp()
	hw.SourceAccount = accountFromXDR(xdrOp.SourceAccount)
	hw.HelloTo = op.HelloTo.Address()
	return nil
}

// Validate for HelloWorld validates the required struct fields.
func (hw *HelloWorld) Validate() error {
	if hw.HelloTo == "" {
		return errors.New("hello_world operation requires a HelloTo address")
	}
	return nil
}

// GetSourceAccount returns the source account of the operation, or the empty string if not
// set.
func (hw *HelloWorld) GetSourceAccount() string {
	return hw.SourceAccount
}
