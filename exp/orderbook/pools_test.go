package orderbook

import (
	"math"
	"math/big"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestLiquidityPoolExchanges(t *testing.T) {
	graph := NewOrderBookGraph()
	for _, poolEntry := range []xdr.LiquidityPoolEntry{
		eurUsdLiquidityPool,
		eurYenLiquidityPool,
		nativeUsdPool,
		usdChfLiquidityPool,
	} {
		params := poolEntry.Body.MustConstantProduct().Params
		graph.getOrCreateAssetID(params.AssetA)
		graph.getOrCreateAssetID(params.AssetB)
	}
	t.Run("happy path", func(t *testing.T) {
		for _, assetXDR := range []xdr.Asset{usdAsset, eurAsset} {
			pool := graph.poolFromEntry(eurUsdLiquidityPool)
			asset := graph.getOrCreateAssetID(assetXDR)
			payout, err := makeTrade(pool, asset, tradeTypeDeposit, 500)
			assert.NoError(t, err)
			assert.EqualValues(t, 332, int64(payout))
			// reserves would now be: 668 of A, 1500 of B
			// note pool object is unchanged so looping is safe
		}

		for _, assetXDR := range []xdr.Asset{usdAsset, eurAsset} {
			pool := graph.poolFromEntry(eurUsdLiquidityPool)
			asset := graph.getOrCreateAssetID(assetXDR)
			payout, err := makeTrade(pool, asset, tradeTypeExpectation, 332)
			assert.NoError(t, err)
			assert.EqualValues(t, 499, int64(payout))
		}

		// More sanity checks; if they fail, something was changed about how
		// constant product liquidity pools work.
		//
		// We use these oddly-specific values because we rely on them again to
		// validate paths in later tests.
		testTable := []struct {
			dstAsset       xdr.Asset
			pool           xdr.LiquidityPoolEntry
			expectedPayout xdr.Int64
			expectedInput  xdr.Int64
		}{
			{yenAsset, eurYenLiquidityPool, 100, 112},
			{eurAsset, eurUsdLiquidityPool, 112, 127},
			{nativeAsset, nativeUsdPool, 5, 25},
			{usdAsset, usdChfLiquidityPool, 127, 342},
			{usdAsset, usdChfLiquidityPool, 27, 58},
		}

		for _, test := range testTable {
			pool := graph.poolFromEntry(test.pool)
			asset := graph.getOrCreateAssetID(test.dstAsset)
			needed, err := makeTrade(pool, asset, tradeTypeExpectation, test.expectedPayout)

			assert.NoError(t, err)
			assert.EqualValuesf(t, test.expectedInput, needed,
				"expected exchange of %d %s -> %d %s, got %d",
				test.expectedInput, graph.idToAssetString[getOtherAsset(asset, pool)],
				test.expectedPayout, getCode(test.dstAsset),
				needed)
		}
	})

	t.Run("fail on bad exchange amounts", func(t *testing.T) {
		badValues := []xdr.Int64{math.MaxInt64, math.MaxInt64 - 99, 0, -100}
		for _, badValue := range badValues {
			pool := graph.poolFromEntry(eurUsdLiquidityPool)
			asset := graph.getOrCreateAssetID(usdAsset)

			_, err := makeTrade(pool, asset, tradeTypeDeposit, badValue)
			assert.Error(t, err)
		}
	})
}

// TestLiquidityPoolMath is a robust suite of tests to ensure that theliquidity
// pool calculation functions are correct, taken from Stellar Core's tests here:
//
// https://github.com/stellar/stellar-core/blob/master/src/transactions/test/ExchangeTests.cpp#L948
func TestLiquidityPoolMath(t *testing.T) {
	iMax := xdr.Int64(math.MaxInt64)
	send, recv := tradeTypeDeposit, tradeTypeExpectation

	t.Run("deposit edge cases", func(t *testing.T) {
		// Sending deposit would overflow the reserve:
		//   low reserves but high deposit
		assertPoolExchange(t, send, 100, iMax-100, 100, -1, 0, true, -1, 99)
		assertPoolExchange(t, send, 100, iMax-99, 100, -1, 0, false, -1, -1)

		//   high reserves but low deposit
		assertPoolExchange(t, send, iMax-100, 100, iMax-100, -1, 0, true, -1, 99)
		assertPoolExchange(t, send, iMax-99, 100, iMax-100, -1, 0, false, -1, -1)

		// fromPool = 0
		assertPoolExchange(t, send, 100, 2, 100, -1, 0, true, -1, 1)
		assertPoolExchange(t, send, 100, 1, 100, -1, 0, false, -1, -1)
	})

	t.Run("disburse edge cases", func(t *testing.T) {
		// Receiving maxReceiveFromPool would deplete the reserves entirely:
		//   low reserves and low maxReceive
		assertPoolExchange(t, recv, 100, -1, 100, 99, 0, true, 9900, -1)
		assertPoolExchange(t, recv, 100, -1, 100, 100, 0, false, -1, -1)

		//   high reserves and high maxReceive
		assertPoolExchange(t, recv, 100, -1, iMax/100, iMax/100-1, 0, true, iMax-107, -1)
		assertPoolExchange(t, recv, 100, -1, iMax/100, iMax/100, 0, false, -1, -1)

		// If
		//           fromPool = k*(maxBps - feeBps) and
		//   reservesFromPool = fromPool + 1
		// then
		//     (reservesToPool * fromPool) / (maxBps - feeBps)
		//         = k * reservesToPool
		// so if k = 101 and reservesToPool = iMax / 100 then B / C > iMax during division
		assertPoolExchange(t, recv, iMax/100, -1, 101*10000+1, 101*10000, 0, false, -1, -1)

		// If
		//            fromPool = maxBps - feeBps and
		//    reservesFromPool = fromPool + 1
		// then
		//      toPool = maxBps * reservesToPool
		// so if reservesToPool = iMax / 100 then the division overflows.
		assertPoolExchange(t, recv, iMax/100, -1, 10000+1, 10000, 0, false, -1, -1)

		// Pool receives more than it has available reserves for
		assertPoolExchange(t, recv, iMax-100, -1, iMax/2, 49, 0, true, 98, -1)
		assertPoolExchange(t, recv, iMax-100, -1, iMax/2, 50, 0, false, -1, -1)
	})

	t.Run("No fees", func(t *testing.T) {
		// Deposits
		assertPoolExchange(t, send, 100, 100, 100, -1, 0, true, -1, 50)      // work exactly
		assertPoolExchange(t, send, 100, 50, 100, -1, 0, true, -1, 33)       // require sending
		assertPoolExchange(t, send, 100, 0, 100, -1, 0, false, -1, -1)       // sending 0
		assertPoolExchange(t, send, 100, iMax-99, 100, -1, 0, false, -1, -1) // sending too much

		// Disburses
		assertPoolExchange(t, recv, 100, -1, 100, 50, 0, true, 100, -1)  // work exactly
		assertPoolExchange(t, recv, 100, -1, 100, 33, 0, true, 50, -1)   // require recving
		assertPoolExchange(t, recv, 100, -1, 100, 0, 0, true, 0, -1)     // receiving 0
		assertPoolExchange(t, recv, 100, -1, 100, 100, 0, false, -1, -1) // receiving too much
	})

	// These test cases look weird because they actually charge 31 bps instead
	// of 30 bps. But this is expected, because you pay fees on the fees you
	// provided: I want to send 10000 after fees, so I send 100030.... but that
	// doesn't work because 0.997 * 10030 = 9999.910 is too low.
	t.Run("30 bps fee actually charges 30 bps", func(t *testing.T) {

		// With no fee, sending 10000 would receive 10000. So to receive
		// 1000 we need to send ceil(10000 / 0.997) = 10031.
		assertPoolExchange(t, send, 10000, 10031, 20000, -1, 30, true, -1, 10000)

		// With no fee, sending 10000 would receive 10000. So to send
		// ceil(10000 / 0.997) = 10031 we need to receive 10000.
		assertPoolExchange(t, recv, 10000, -1, 20000, 10000, 30, true, 10031, -1)
	})

	t.Run("Potential Internal Overflow", func(t *testing.T) {

		// Test for internal uint128 underflow/overflow in CalculatePoolPayout() and  CalculatePoolExpectation() by providing
		// input values which cause the maximum internal calculations

		assertPoolExchange(t, send, math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MaxInt64, 0, false, 0, 0)
		assertPoolExchange(t, send, math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MaxInt64, 0, false, 0, 0)
		assertPoolExchange(t, recv, math.MaxInt64, math.MaxInt64, math.MaxInt64, 0, 0, true, 0, -1)

		// Check with reserveB < disbursed
		assertPoolExchange(t, recv, math.MaxInt64, math.MaxInt64, 0, 1, 0, false, 0, 0)

		// Check with calculated deposit overflows reserveA
		assertPoolExchange(t, recv, 9223372036654845862, 0, 2694994506, 4515739, 30, false, 0, 0)

		// Check with poolFeeBips > 10000
		assertPoolExchange(t, send, math.MaxInt64, math.MaxInt64, math.MaxInt64, math.MaxInt64, 10001, false, 0, 0)
		assertPoolExchange(t, recv, math.MaxInt64, math.MaxInt64, math.MaxInt64, 0, 10010, false, 0, 0)

		assertPoolExchange(t, send, 92017260901926686, 9157376027422527, 4000000000000000000, 30, 1, true, -1, 362009430194478152)
	})
}

// assertPoolExchange validates that pool inputs match their expected outputs.
func assertPoolExchange(t *testing.T,
	exchangeType int,
	reservesBeingDeposited, deposited xdr.Int64,
	reservesBeingDisbursed, disbursed xdr.Int64,
	poolFeeBips xdr.Int32,
	expectedReturn bool, expectedDeposited, expectedDisbursed xdr.Int64,
) {
	var ok bool
	toPool, fromPool := xdr.Int64(-1), xdr.Int64(-1)

	switch exchangeType {
	case tradeTypeDeposit:
		fromPool, _, ok = CalculatePoolPayout(
			reservesBeingDeposited, reservesBeingDisbursed,
			deposited, poolFeeBips, false)
		fromPoolBig, _, okBig := calculatePoolPayoutBig(
			reservesBeingDeposited, reservesBeingDisbursed,
			deposited, poolFeeBips)
		assert.Equal(t, okBig, ok)
		assert.Equal(t, fromPoolBig, fromPool)

	case tradeTypeExpectation:
		toPool, _, ok = CalculatePoolExpectation(
			reservesBeingDeposited, reservesBeingDisbursed,
			disbursed, poolFeeBips, false)
		toPoolBig, _, okBig := calculatePoolExpectationBig(
			reservesBeingDeposited, reservesBeingDisbursed,
			disbursed, poolFeeBips,
		)
		assert.Equal(t, okBig, ok)
		assert.Equal(t, toPoolBig, toPool)

	default:
		t.FailNow()
	}

	assert.Equal(t, expectedReturn, ok, "wrong exchange success state")
	if expectedReturn {
		assert.EqualValues(t, expectedDisbursed, fromPool, "wrong payout")
		assert.EqualValues(t, expectedDeposited, toPool, "wrong expectation")
	}
}

func TestCalculatePoolExpectations(t *testing.T) {
	for i := 0; i < 1000000; i++ {
		reserveA := xdr.Int64(rand.Int63())
		reserveB := xdr.Int64(rand.Int63())
		disbursed := xdr.Int64(rand.Int63())

		result, roundingSlippage, ok := calculatePoolExpectationBig(reserveA, reserveB, disbursed, 30)
		result1, roundingSlippage1, ok1 := CalculatePoolExpectation(reserveA, reserveB, disbursed, 30, true)
		if assert.Equal(t, ok, ok1) {
			assert.Equal(t, result, result1)
			assert.Equal(t, roundingSlippage, roundingSlippage1)
		}
	}
}

func TestCalculatePoolExpectationsRoundingSlippage(t *testing.T) {
	t.Run("big", func(t *testing.T) {
		reserveA := xdr.Int64(3740000000)
		reserveB := xdr.Int64(162020000000)
		disbursed := xdr.Int64(1)

		result, roundingSlippage, ok := CalculatePoolExpectation(reserveA, reserveB, disbursed, 30, true)
		require.True(t, ok)
		assert.Equal(t, xdr.Int64(1), result)
		assert.Equal(t, xdr.Int64(4229), roundingSlippage)
	})

	t.Run("small", func(t *testing.T) {
		reserveA := xdr.Int64(200)
		reserveB := xdr.Int64(400)
		disbursed := xdr.Int64(20)

		result, roundingSlippage, ok := CalculatePoolExpectation(reserveA, reserveB, disbursed, 30, true)
		require.True(t, ok)
		assert.Equal(t, xdr.Int64(11), result)
		assert.Equal(t, xdr.Int64(4), roundingSlippage)
	})

	t.Run("very small", func(t *testing.T) {
		reserveA := xdr.Int64(200)
		reserveB := xdr.Int64(400)
		disbursed := xdr.Int64(50)

		result, roundingSlippage, ok := CalculatePoolExpectation(reserveA, reserveB, disbursed, 30, true)
		require.True(t, ok)
		assert.Equal(t, xdr.Int64(29), result)
		assert.Equal(t, xdr.Int64(1), roundingSlippage)
	})
}

func TestCalculatePoolPayout(t *testing.T) {
	for i := 0; i < 1000000; i++ {
		reserveA := xdr.Int64(rand.Int63())
		reserveB := xdr.Int64(rand.Int63())
		received := xdr.Int64(rand.Int63())

		result, roundingSlippage, ok := calculatePoolPayoutBig(reserveA, reserveB, received, 30)
		result1, roundingSlippage1, ok1 := CalculatePoolPayout(reserveA, reserveB, received, 30, true)
		if assert.Equal(t, ok, ok1) {
			assert.Equal(t, result, result1)
			assert.Equal(t, roundingSlippage, roundingSlippage1)
		}
	}
}

func TestCalculatePoolPayoutRoundingSlippage(t *testing.T) {
	t.Run("big", func(t *testing.T) {
		reserveA := xdr.Int64(162020000000)
		reserveB := xdr.Int64(3740000000)
		received := xdr.Int64(50)

		result, roundingSlippage, ok := CalculatePoolPayout(reserveA, reserveB, received, 30, true)
		require.True(t, ok)
		assert.Equal(t, xdr.Int64(1), result)
		assert.Equal(t, xdr.Int64(13), roundingSlippage)
	})

	t.Run("small", func(t *testing.T) {
		reserveA := xdr.Int64(200)
		reserveB := xdr.Int64(300)
		received := xdr.Int64(1)

		result, roundingSlippage, ok := CalculatePoolPayout(reserveA, reserveB, received, 30, true)
		require.True(t, ok)
		assert.Equal(t, xdr.Int64(1), result)
		assert.Equal(t, xdr.Int64(32), roundingSlippage)
	})

	t.Run("very small", func(t *testing.T) {
		reserveA := xdr.Int64(200)
		reserveB := xdr.Int64(210)
		received := xdr.Int64(1)

		result, roundingSlippage, ok := CalculatePoolPayout(reserveA, reserveB, received, 30, true)
		require.True(t, ok)
		assert.Equal(t, xdr.Int64(1), result)
		assert.Equal(t, xdr.Int64(3), roundingSlippage)
	})
}

// CalculatePoolPayout calculates the amount of `reserveB` disbursed from the
// pool for a `received` amount of `reserveA` . From CAP-38:
//
//	y = floor[(1 - F) Yx / (X + x - Fx)]
//
// It returns false if the calculation overflows.
func calculatePoolPayoutBig(reserveA, reserveB, received xdr.Int64, feeBips xdr.Int32) (xdr.Int64, xdr.Int64, bool) {
	if feeBips < 0 || feeBips >= maxBasisPoints {
		return 0, 0, false
	}
	X, Y := big.NewInt(int64(reserveA)), big.NewInt(int64(reserveB))
	F, x := big.NewInt(int64(feeBips)), big.NewInt(int64(received))
	S := new(big.Int) // Rounding Slippage

	// would this deposit overflow the reserve?
	if received > math.MaxInt64-reserveA {
		return 0, 0, false
	}

	// We do all of the math in bips, so it's all upscaled by this value.
	maxBips := big.NewInt(10000)
	f := new(big.Int).Sub(maxBips, F) // upscaled 1 - F

	// right half: X + (1 - F)x
	denom := X.Mul(X, maxBips).Add(X, new(big.Int).Mul(x, f))
	if denom.Cmp(big.NewInt(0)) == 0 { // avoid div-by-zero panic
		return 0, 0, false
	}

	// left half, a: (1 - F) Yx
	numer := Y.Mul(Y, x).Mul(Y, f)

	// divide & check overflow
	result, rem := new(big.Int), new(big.Int)
	result.Div(numer, denom)
	rem.Mod(numer, denom)

	if rem.Cmp(big.NewInt(0)) > 0 {
		// Recalculate with more precision
		unrounded, rounded := new(big.Int), new(big.Int)
		unrounded.Mul(numer, maxBips).Div(unrounded, denom)
		rounded.Mul(result, maxBips)
		S.Sub(unrounded, rounded)
		S.Abs(S).Mul(S, maxBips)
		S.Div(S, unrounded)
		S.Div(S, big.NewInt(100))
	}

	i := xdr.Int64(result.Int64())
	s := xdr.Int64(S.Int64())
	ok := result.IsInt64() && i > 0 && S.IsInt64() && s >= 0
	return i, s, ok
}

// calculatePoolExpectation determines how much of `reserveA` you would need to
// put into a pool to get the `disbursed` amount of `reserveB`.
//
//	x = ceil[Xy / ((Y - y)(1 - F))]
//
// It returns false if the calculation overflows.
func calculatePoolExpectationBig(
	reserveA, reserveB, disbursed xdr.Int64, feeBips xdr.Int32,
) (xdr.Int64, xdr.Int64, bool) {
	if feeBips < 0 || feeBips >= maxBasisPoints {
		return 0, 0, false
	}
	X, Y := big.NewInt(int64(reserveA)), big.NewInt(int64(reserveB))
	F, y := big.NewInt(int64(feeBips)), big.NewInt(int64(disbursed))
	S := new(big.Int) // Rounding Slippage

	// sanity check: disbursing shouldn't underflow the reserve
	if disbursed >= reserveB {
		return 0, 0, false
	}

	// We do all of the math in bips, so it's all upscaled by this value.
	maxBips := big.NewInt(10000)
	f := new(big.Int).Sub(maxBips, F) // upscaled 1 - F

	denom := Y.Sub(Y, y).Mul(Y, f)     // right half: (Y - y)(1 - F)
	if denom.Cmp(big.NewInt(0)) == 0 { // avoid div-by-zero panic
		return 0, 0, false
	}

	numer := X.Mul(X, y).Mul(X, maxBips) // left half: Xy

	result, rem := new(big.Int), new(big.Int)
	result.DivMod(numer, denom, rem)

	// hacky way to ceil(): if there's a remainder, add 1
	if rem.Cmp(big.NewInt(0)) > 0 {
		result.Add(result, big.NewInt(1))

		// Recalculate with more precision
		unrounded, rounded := new(big.Int), new(big.Int)
		unrounded.Mul(numer, maxBips).Div(unrounded, denom)
		rounded.Mul(result, maxBips)
		S.Sub(unrounded, rounded)
		S.Abs(S).Mul(S, maxBips)
		S.Div(S, unrounded)
		S.Div(S, big.NewInt(100))
	}

	i := xdr.Int64(result.Int64())
	s := xdr.Int64(S.Int64())
	ok := result.IsInt64() && i >= 0 && i <= math.MaxInt64-reserveA && S.IsInt64() && s >= 0
	return i, s, ok
}
