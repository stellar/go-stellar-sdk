//go:build xdr_hello_world

package txnbuild

import "github.com/stellar/go-stellar-sdk/xdr"

func operationFromXDRForXdrHelloWorld(xdrOp xdr.Operation) (Operation, bool) {
	switch xdrOp.Body.Type {
	case xdr.OperationTypeHelloWorld:
		return &HelloWorld{}, true
	default:
		return nil, false
	}
}
