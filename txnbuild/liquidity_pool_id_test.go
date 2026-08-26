package txnbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLiquidityPoolId(t *testing.T) {
	a := NativeAsset{}
	b := CreditAsset{Code: "ABC", Issuer: "GDQNY3PBOJOKYZSRMK2S7LHHGWZIUISD4QORETLMXEWXBI7KFZZMKTL3"}

	id, err := NewLiquidityPoolId(a, b)
	if assert.NoError(t, err) {
		assert.Equal(t, LiquidityPoolId([32]byte{
			0xcc, 0x22, 0x41, 0x49, 0x97, 0xd7, 0xe3, 0xd9,
			0xa9, 0xac, 0x3b, 0x1d, 0x65, 0xca, 0x9c, 0xc3,
			0xe5, 0xf3, 0x5c, 0xe3, 0x3e, 0x0b, 0xd6, 0xa8,
			0x85, 0x64, 0x8b, 0x11, 0xaa, 0xa3, 0xb7, 0x2d,
		}), id, "pool id should match")
	}

	// Wrong asset id order should fail. If users mess this up, and we were to
	// silently fix it they could set the wrong MaxAmounts when depositing.
	_, err = NewLiquidityPoolId(b, a)
	assert.EqualError(t, err, "AssetA must be < AssetB")
}
