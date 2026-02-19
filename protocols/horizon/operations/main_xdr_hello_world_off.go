//go:build !xdr_hello_world

package operations

func unmarshalOperationForXdrHelloWorld(_ int32, _ []byte) (Operation, error, bool) {
	return nil, nil, false
}
