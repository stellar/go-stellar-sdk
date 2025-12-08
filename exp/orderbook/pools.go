package orderbook

import (
	"math"

	"github.com/holiman/uint256"

	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// There are two different exchanges that can be simulated:
//
// 1. You know how much you can *give* to the pool, and are curious about the
// resulting payout. We call this a "deposit", and you should pass
// tradeTypeDeposit.
//
// 2. You know how much you'd like to *receive* from the pool, and want to know
// how much to deposit to achieve this. We call this an "expectation", and you
// should pass tradeTypeExpectation.
const (
	tradeTypeDeposit     = iota // deposit into pool, what's the payout?
	tradeTypeExpectation = iota // expect payout, what to deposit?
	maxBasisPoints       = 10_000
)

var (
	errPoolOverflows = errors.New("Liquidity pool overflows from this exchange")
	errBadPoolType   = errors.New("Unsupported liquidity pool: must be ConstantProduct")
	errBadTradeType  = errors.New("Unknown pool exchange type requested")
	errBadAmount     = errors.New("Exchange amount must be positive")
)

// makeTrade simulates execution of an exchange with a liquidity pool.
//
// In (1), this returns the amount that would be paid out by the pool (in terms
// of the *other* asset) for depositing `amount` of `asset`.
//
// In (2), this returns the amount of `asset` you'd need to deposit to get
// `amount` of the *other* asset in return.
//
// Refer to https://github.com/stellar/stellar-protocol/blob/master/core/cap-0038.md#pathpaymentstrictsendop-and-pathpaymentstrictreceiveop
// and the calculation functions (below) for details on the exchange algorithm.
//
// Warning: If you pass an asset that is NOT one of the pool reserves, the
// behavior of this function is undefined (for performance).
func makeTrade(
	pool liquidityPool,
	asset int32,
	tradeType int,
	amount xdr.Int64,
) (xdr.Int64, error) {
	details, ok := pool.Body.GetConstantProduct()
	if !ok {
		return 0, errBadPoolType
	}

	if amount <= 0 {
		return 0, errBadAmount
	}

	// determine which asset `amount` corresponds to
	X, Y := details.ReserveA, details.ReserveB
	if pool.assetA != asset {
		X, Y = Y, X
	}

	ok = false
	var result xdr.Int64
	switch tradeType {
	case tradeTypeDeposit:
		result, _, ok = CalculatePoolPayout(X, Y, amount, details.Params.Fee, false)

	case tradeTypeExpectation:
		result, _, ok = CalculatePoolExpectation(X, Y, amount, details.Params.Fee, false)

	default:
		return 0, errBadTradeType
	}

	if !ok {
		// the error isn't strictly accurate (e.g. it could be div-by-0), but
		// from the caller's perspective it's true enough
		return 0, errPoolOverflows
	}
	return result, nil
}

// CalculatePoolPayout calculates the amount of `reserveB` disbursed from the
// pool for a `received` amount of `reserveA` . From CAP-38:
//
//	y = floor[(1 - F) Yx / (X + x - Fx)]
//
// It returns false if the calculation overflows.
func CalculatePoolPayout(reserveA, reserveB, received xdr.Int64, feeBips xdr.Int32, calculateRoundingSlippage bool) (xdr.Int64, xdr.Int64, bool) {
	if feeBips < 0 || feeBips >= maxBasisPoints {
		return 0, 0, false
	}
	X, Y := uint256.NewInt(uint64(reserveA)), uint256.NewInt(uint64(reserveB))
	F, x := uint256.NewInt(uint64(feeBips)), uint256.NewInt(uint64(received))

	// would this deposit overflow the reserve?
	if received > math.MaxInt64-reserveA {
		return 0, 0, false
	}

	// We do all of the math with 4 extra decimal places of precision, so it's
	// all upscaled by this value.
	maxBips := uint256.NewInt(maxBasisPoints)
	f := new(uint256.Int).Sub(maxBips, F) // upscaled 1 - F

	// right half: X + (1 - F)x
	denom := X.Mul(X, maxBips).Add(X, new(uint256.Int).Mul(x, f))
	if denom.IsZero() { // avoid div-by-zero panic
		return 0, 0, false
	}

	// left half, a: (1 - F) Yx
	numer := Y.Mul(Y, x).Mul(Y, f)

	// divide & check overflow
	result := new(uint256.Int)
	result.Div(numer, denom)

	var roundingSlippageBips xdr.Int64
	ok := true
	if calculateRoundingSlippage && !new(uint256.Int).Mod(numer, denom).IsZero() {
		// Calculates the rounding slippage (S) in bips (Basis points)
		//
		// S is the % which the rounded result deviates from the unrounded.
		// i.e. How much "error" did the rounding introduce?
		//
		//      unrounded = Xy / ((Y - y)(1 - F))
		//      expectation = ceil[unrounded]
		//      S = abs(expectation - unrounded) / unrounded
		//
		// For example, for:
		//
		//      X = 200    // 200 stroops of deposited asset in reserves
		//      Y = 300    // 300 stroops of disbursed asset in reserves
		//      y = 3      // disbursing 3 stroops
		//      F = 0.003  // fee is 0.3%
		//      unrounded = (200 * 3) / ((300 - 3)(1 - 0.003)) = 2.03
		//      S = abs(ceil(2.03) - 2.03) / 2.03 = 47.78%
		//      toBips(S) = 4778
		//
		S := new(uint256.Int)
		unrounded, rounded := new(uint256.Int), new(uint256.Int)
		// Upscale to centibips for extra precision
		unrounded.Mul(numer, maxBips).Div(unrounded, denom)
		rounded.Mul(result, maxBips)
		S.Sub(unrounded, rounded)
		S.Abs(S).Mul(S, maxBips)
		S.Div(S, unrounded)
		S.Div(S, uint256.NewInt(100)) // Downscale from centibips to bips
		roundingSlippageBips = xdr.Int64(S.Uint64())
		ok = ok && S.IsUint64() && roundingSlippageBips >= 0
	}

	val := xdr.Int64(result.Uint64())
	ok = ok && result.IsUint64() && val > 0
	return val, roundingSlippageBips, ok
}

// CalculatePoolExpectation determines how much of `reserveA` you would need to
// put into a pool to get the `disbursed` amount of `reserveB`.
//
//	x = ceil[Xy / ((Y - y)(1 - F))]
//
// It returns false if the calculation overflows.
func CalculatePoolExpectation(
	reserveA, reserveB, disbursed xdr.Int64, feeBips xdr.Int32, calculateRoundingSlippage bool,
) (xdr.Int64, xdr.Int64, bool) {
	if feeBips < 0 || feeBips >= maxBasisPoints {
		return 0, 0, false
	}
	X, Y := uint256.NewInt(uint64(reserveA)), uint256.NewInt(uint64(reserveB))
	F, y := uint256.NewInt(uint64(feeBips)), uint256.NewInt(uint64(disbursed))

	// sanity check: disbursing shouldn't underflow the reserve
	if disbursed >= reserveB {
		return 0, 0, false
	}

	// We do all of the math with 4 extra decimal places of precision, so it's
	// all upscaled by this value.
	maxBips := uint256.NewInt(maxBasisPoints)
	f := new(uint256.Int).Sub(maxBips, F) // upscaled 1 - F

	denom := Y.Sub(Y, y).Mul(Y, f) // right half: (Y - y)(1 - F)
	if denom.IsZero() {            // avoid div-by-zero panic
		return 0, 0, false
	}

	numer := X.Mul(X, y).Mul(X, maxBips) // left half: Xy

	result, rem := new(uint256.Int), new(uint256.Int)
	result.Div(numer, denom)
	rem.Mod(numer, denom)

	// hacky way to ceil(): if there's a remainder, add 1
	var roundingSlippageBips xdr.Int64
	ok := true
	if !rem.IsZero() {
		result.AddUint64(result, 1)

		if calculateRoundingSlippage {
			// Calculates the rounding slippage (S) in bips (Basis points)
			//
			// S is the % which the rounded result deviates from the unrounded.
			// i.e. How much "error" did the rounding introduce?
			//
			//      unrounded = Xy / ((Y - y)(1 - F))
			//      expectation = ceil[unrounded]
			//      S = abs(expectation - unrounded) / unrounded
			//
			// For example, for:
			//
			//      X = 200    // 200 stroops of deposited asset in reserves
			//      Y = 300    // 300 stroops of disbursed asset in reserves
			//      y = 3      // disbursing 3 stroops
			//      F = 0.003  // fee is 0.3%
			//      unrounded = (200 * 3) / ((300 - 3)(1 - 0.003)) = 2.03
			//      S = abs(ceil(2.03) - 2.03) / 2.03 = 47.78%
			//      toBips(S) = 4778
			//
			S := new(uint256.Int)
			unrounded, rounded := new(uint256.Int), new(uint256.Int)
			// Upscale to centibips for extra precision
			unrounded.Mul(numer, maxBips).Div(unrounded, denom)
			rounded.Mul(result, maxBips)
			S.Sub(unrounded, rounded)
			S.Abs(S).Mul(S, maxBips)
			S.Div(S, unrounded)
			S.Div(S, uint256.NewInt(100)) // Downscale from centibips to bips
			roundingSlippageBips = xdr.Int64(S.Uint64())
			ok = ok && S.IsUint64() && roundingSlippageBips >= 0
		}
	}

	val := xdr.Int64(result.Uint64())
	ok = ok &&
		result.IsUint64() &&
		val >= 0 &&
		// check that the calculated deposit would not overflow the reserve
		val <= math.MaxInt64-reserveA
	return val, roundingSlippageBips, ok
}

// getOtherAsset returns the other asset in the liquidity pool. Note that
// doesn't check to make sure the passed in `asset` is actually part of the
// pool; behavior in that case is undefined.
func getOtherAsset(asset int32, pool liquidityPool) int32 {
	if pool.assetA == asset {
		return pool.assetB
	}
	return pool.assetA
}
