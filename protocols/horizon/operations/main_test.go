package operations

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestTypeNamesAllCovered(t *testing.T) {
	for typ, s := range xdr.OperationTypeToStringMap {
		_, ok := TypeNames[xdr.OperationType(typ)]
		assert.True(t, ok, s)
	}
}

func TestUnmarshalOperationAllCovered(t *testing.T) {
	mistmatchErr := errors.New("Invalid operation format, unable to unmarshal json response")
	for typ, s := range xdr.OperationTypeToStringMap {
		_, err := UnmarshalOperation(typ, []byte{})
		assert.Error(t, err, s)
		// it should be a parsing error, not the default branch
		assert.NotEqual(t, mistmatchErr, err, s)
	}

	// make sure the check works for an unknown operation type
	_, err := UnmarshalOperation(200000, []byte{})
	assert.Error(t, err)
	assert.Equal(t, mistmatchErr, err)
}
