//go:build !xdr_hello_world

package txnbuild

import "github.com/stellar/go-stellar-sdk/xdr"

func operationFromXDRForXdrHelloWorld(_ xdr.Operation) (Operation, bool) {
	return nil, false
}
