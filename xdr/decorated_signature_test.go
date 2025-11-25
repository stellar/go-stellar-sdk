package xdr_test

import (
	"fmt"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
)

var (
	signature = []byte{0, 1, 2, 3, 4, 5, 6, 7, 8}
	hint      = [4]byte{9, 10, 11, 12}
)

func TestDecoratedSignature(t *testing.T) {
	decoSig := xdr.NewDecoratedSignature(signature, hint)
	assert.EqualValues(t, hint, decoSig.Hint)
	assert.EqualValues(t, signature, decoSig.Signature)
}

func TestDecoratedSignatureForPayload(t *testing.T) {
	testCases := []struct {
		payload      []byte
		expectedHint [4]byte
	}{
		{
			[]byte{13, 14, 15, 16, 17, 18, 19, 20, 21},
			[4]byte{27, 25, 31, 25},
		},
		{
			[]byte{18, 19, 20},
			[4]byte{27, 25, 31, 12},
		},
		{
			[]byte{},
			hint,
		},
	}

	for _, testCase := range testCases {
		t.Run(
			fmt.Sprintf("%d-byte payload", len(testCase.payload)),
			func(t *testing.T) {
				decoSig := xdr.NewDecoratedSignatureForPayload(
					signature, hint, testCase.payload)
				assert.EqualValues(t, testCase.expectedHint, decoSig.Hint)
				assert.EqualValues(t, signature, decoSig.Signature)
			})
	}
}
