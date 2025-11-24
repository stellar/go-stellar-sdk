package xdr_test

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
)

func TestUint256String(t *testing.T) {
	assert.Equal(t, "0", xdr.Uint256{}.String())
	assert.Equal(t, "1", xdr.Uint256{
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 1,
	}.String())
	assert.Equal(t, "16909060", xdr.Uint256{
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 1, 2, 3, 4,
	}.String())
}
