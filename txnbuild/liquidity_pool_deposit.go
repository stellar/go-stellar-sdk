//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite
package txnbuild

import (
	"github.com/stellar/go-stellar-sdk/amount"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// LiquidityPoolDeposit represents the Stellar liquidity pool deposit operation. See
// https://developers.stellar.org/docs/start/list-of-operations/
type LiquidityPoolDeposit struct {
	SourceAccount   string
	LiquidityPoolID LiquidityPoolId
	MaxAmountA      string
	MaxAmountB      string
	MinPrice        xdr.Price
	MaxPrice        xdr.Price
}

// NewLiquidityPoolDeposit creates a new LiquidityPoolDeposit operation,
// checking the ordering assets so we generate the correct pool id. minPrice,
// and maxPrice are in terms of a/b. Each AssetAmount is a pair of the asset
// with the maximum amount of that asset to deposit.
func NewLiquidityPoolDeposit(
	sourceAccount string,
	a, b AssetAmount,
	minPrice,
	maxPrice xdr.Price,
) (LiquidityPoolDeposit, error) {
	if b.Asset.LessThan(a.Asset) {
		return LiquidityPoolDeposit{}, errors.New("AssetA must be <= AssetB")
	}

	poolId, err := NewLiquidityPoolId(a.Asset, b.Asset)
	if err != nil {
		return LiquidityPoolDeposit{}, err
	}

	return LiquidityPoolDeposit{
		SourceAccount:   sourceAccount,
		LiquidityPoolID: poolId,
		MaxAmountA:      a.Amount,
		MaxAmountB:      b.Amount,
		MinPrice:        minPrice,
		MaxPrice:        maxPrice,
	}, nil
}

// BuildXDR for LiquidityPoolDeposit returns a fully configured XDR Operation.
func (lpd *LiquidityPoolDeposit) BuildXDR() (xdr.Operation, error) {
	xdrLiquidityPoolId, err := lpd.LiquidityPoolID.ToXDR()
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "couldn't build liquidity pool ID XDR")
	}

	xdrMaxAmountA, err := amount.Parse(lpd.MaxAmountA)
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to parse 'MaxAmountA'")
	}

	xdrMaxAmountB, err := amount.Parse(lpd.MaxAmountB)
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to parse 'MaxAmountB'")
	}

	xdrOp := xdr.LiquidityPoolDepositOp{
		LiquidityPoolId: xdrLiquidityPoolId,
		MaxAmountA:      xdrMaxAmountA,
		MaxAmountB:      xdrMaxAmountB,
		MinPrice:        lpd.MinPrice,
		MaxPrice:        lpd.MaxPrice,
	}

	opType := xdr.OperationTypeLiquidityPoolDeposit
	body, err := xdr.NewOperationBody(opType, xdrOp)
	if err != nil {
		return xdr.Operation{}, errors.Wrap(err, "failed to build XDR OperationBody")
	}
	op := xdr.Operation{Body: body}
	SetOpSourceAccount(&op, lpd.SourceAccount)
	return op, nil
}

// FromXDR for LiquidityPoolDeposit initializes the txnbuild struct from the corresponding xdr Operation.
func (lpd *LiquidityPoolDeposit) FromXDR(xdrOp xdr.Operation) error {
	result, ok := xdrOp.Body.GetLiquidityPoolDepositOp()
	if !ok {
		return errors.New("error parsing liquidity_pool_deposit operation from xdr")
	}

	liquidityPoolID, err := liquidityPoolIdFromXDR(result.LiquidityPoolId)
	if err != nil {
		return errors.New("error parsing LiquidityPoolId in liquidity_pool_deposit operation from xdr")
	}
	lpd.LiquidityPoolID = liquidityPoolID

	lpd.SourceAccount = accountFromXDR(xdrOp.SourceAccount)
	lpd.MaxAmountA = amount.String(result.MaxAmountA)
	lpd.MaxAmountB = amount.String(result.MaxAmountB)
	if result.MinPrice != (xdr.Price{}) {
		lpd.MinPrice = result.MinPrice
	}
	if result.MaxPrice != (xdr.Price{}) {
		lpd.MaxPrice = result.MaxPrice
	}

	return nil
}

// Validate for LiquidityPoolDeposit validates the required struct fields. It returns an error if any of the fields are
// invalid. Otherwise, it returns nil.
func (lpd *LiquidityPoolDeposit) Validate() error {
	err := validateAmount(lpd.MaxAmountA)
	if err != nil {
		return NewValidationError("MaxAmountA", err.Error())
	}

	err = validateAmount(lpd.MaxAmountB)
	if err != nil {
		return NewValidationError("MaxAmountB", err.Error())
	}

	err = validatePrice(lpd.MinPrice)
	if err != nil {
		return NewValidationError("MinPrice", err.Error())
	}

	err = validatePrice(lpd.MaxPrice)
	if err != nil {
		return NewValidationError("MaxPrice", err.Error())
	}

	return nil
}

// GetSourceAccount returns the source account of the operation, or nil if not
// set.
func (lpd *LiquidityPoolDeposit) GetSourceAccount() string {
	return lpd.SourceAccount
}
