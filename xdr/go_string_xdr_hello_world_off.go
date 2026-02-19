//go:build !xdr_hello_world

package xdr

func operationBodyGoStringForXdrHelloWorld(_ OperationBody) (string, error, bool) {
	return "", nil, false
}
