//go:build xdr_hello_world

package xdr

func mapOperationResultTrForXdrHelloWorld(o OperationResultTr) (string, error, bool) {
	switch o.Type {
	case OperationTypeHelloWorld:
		return o.HelloWorldResult.Code.String(), nil, true
	default:
		return "", nil, false
	}
}
