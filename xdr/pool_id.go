package xdr

import (
	"bytes"
	"crypto/sha256"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

// NewPoolId derives the id of the constant product liquidity pool holding the
// two given assets. The caller must pass them in protocol order, which is
// strictly AssetA < AssetB; a reversed pair, or the same asset twice, is
// rejected rather than reordered, because the id depends on the order.
func NewPoolId(a, b Asset, fee Int32) (PoolId, error) {
	if !a.LessThan(b) {
		return PoolId{}, errors.New("AssetA must be < AssetB")
	}

	params := LiquidityPoolParameters{
		Type: LiquidityPoolTypeLiquidityPoolConstantProduct,
		ConstantProduct: &LiquidityPoolConstantProductParameters{
			AssetA: a,
			AssetB: b,
			Fee:    fee,
		},
	}

	buf := &bytes.Buffer{}
	if _, err := Marshal(buf, params); err != nil {
		return PoolId{}, errors.Wrap(err, "failed to build liquidity pool id")
	}
	return sha256.Sum256(buf.Bytes()), nil
}
