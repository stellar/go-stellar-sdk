//go:build xdr_hello_world

package xdr

import "fmt"

func operationBodyGoStringForXdrHelloWorld(o OperationBody) (string, error, bool) {
	switch {
	case o.HelloWorldOp != nil:
		return fmt.Sprintf("HelloWorldOp: &%#v", *o.HelloWorldOp), nil, true
	default:
		return "", nil, false
	}
}
