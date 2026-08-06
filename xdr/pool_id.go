package xdr

import (
	"bytes"
	"crypto/sha256"

	"github.com/stellar/go-stellar-sdk/support/errors"
)

// NewPoolId requires a and b in protocol order, strictly a < b. The id depends
// on that order, so a reversed or identical pair is rejected, not reordered.
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
