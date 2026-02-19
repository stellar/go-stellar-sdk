//go:build !xdr_hello_world

package xdr

func mapOperationResultTrForXdrHelloWorld(_ OperationResultTr) (string, error, bool) {
	return "", nil, false
}
