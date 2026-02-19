//go:build xdr_hello_world

package operations

import (
	"encoding/json"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// HelloWorld is the json resource representing a single operation whose type is
// HelloWorld.
type HelloWorld struct {
	Base
	HelloTo string `json:"hello_to"`
}

func init() {
	TypeNames[xdr.OperationTypeHelloWorld] = "hello_world"
}

func unmarshalOperationForXdrHelloWorld(operationTypeID int32, dataString []byte) (Operation, error, bool) {
	switch xdr.OperationType(operationTypeID) {
	case xdr.OperationTypeHelloWorld:
		var op HelloWorld
		if err := json.Unmarshal(dataString, &op); err != nil {
			return nil, err, true
		}
		return op, nil, true
	default:
		return nil, nil, false
	}
}
