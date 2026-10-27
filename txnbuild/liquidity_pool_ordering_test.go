package txnbuild

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/stretchr/testify/require"
)

func orderedPair(t *testing.T, a, b CreditAsset) (CreditAsset, CreditAsset) {
	t.Helper()
	aXDR, err := a.ToXDR()
	require.NoError(t, err)
	bXDR, err := b.ToXDR()
	require.NoError(t, err)
	if bXDR.LessThan(aXDR) {
		return b, a
	}
	return a, b
}

func TestChangeTrustPoolParamsCanAlwaysBeReadBack(t *testing.T) {
	built := 0
	for i := 0; i < 2000; i++ {
		a := CreditAsset{Code: "USD", Issuer: keypair.MustRandom().Address()}
		b := CreditAsset{Code: "USD", Issuer: keypair.MustRandom().Address()}

		for _, pair := range [][2]CreditAsset{{a, b}, {b, a}} {
			op := &ChangeTrust{
				Line: LiquidityPoolShareChangeTrustAsset{
					LiquidityPoolParameters: LiquidityPoolParameters{
						AssetA: pair[0],
						AssetB: pair[1],
						Fee:    LiquidityPoolFeeV18,
					},
				},
				Limit: MaxTrustlineLimit,
			}

			xdrOp, err := op.BuildXDR()
			if err != nil {
				continue // refusing an out-of-order pair is correct
			}
			built++

			cp := xdrOp.Body.MustChangeTrustOp().Line.MustLiquidityPool().ConstantProduct
			_, err = xdr.NewPoolId(cp.AssetA, cp.AssetB, cp.Fee)
			require.NoError(t, err,
				"txnbuild produced pool parameters whose pool id cannot be derived")
		}
	}
	require.Equal(t, 2000, built,
		"exactly one order of each distinct pair must build")
}

func TestChangeTrustRejectsOutOfOrderPoolParams(t *testing.T) {
	first, second := orderedPair(t,
		CreditAsset{Code: "USD", Issuer: keypair.MustRandom().Address()},
		CreditAsset{Code: "USD", Issuer: keypair.MustRandom().Address()},
	)

	inOrder := &ChangeTrust{
		Line: LiquidityPoolShareChangeTrustAsset{
			LiquidityPoolParameters: LiquidityPoolParameters{
				AssetA: first, AssetB: second, Fee: LiquidityPoolFeeV18,
			},
		},
		Limit: MaxTrustlineLimit,
	}
	require.NoError(t, inOrder.Validate())
	_, err := inOrder.BuildXDR()
	require.NoError(t, err)

	reversed := &ChangeTrust{
		Line: LiquidityPoolShareChangeTrustAsset{
			LiquidityPoolParameters: LiquidityPoolParameters{
				AssetA: second, AssetB: first, Fee: LiquidityPoolFeeV18,
			},
		},
		Limit: MaxTrustlineLimit,
	}
	require.Error(t, reversed.Validate())
	_, err = reversed.BuildXDR()
	require.Error(t, err)
}

func TestChangeTrustRejectsIdenticalPoolAssets(t *testing.T) {
	asset := CreditAsset{Code: "USD", Issuer: keypair.MustRandom().Address()}

	op := &ChangeTrust{
		Line: LiquidityPoolShareChangeTrustAsset{
			LiquidityPoolParameters: LiquidityPoolParameters{
				AssetA: asset, AssetB: asset, Fee: LiquidityPoolFeeV18,
			},
		},
		Limit: MaxTrustlineLimit,
	}
	require.Error(t, op.Validate())
	_, err := op.BuildXDR()
	require.Error(t, err)

	_, err = LiquidityPoolParameters{
		AssetA: asset, AssetB: asset, Fee: LiquidityPoolFeeV18,
	}.ToXDR()
	require.Error(t, err)

	native := NativeAsset{}
	_, err = LiquidityPoolParameters{
		AssetA: native, AssetB: native, Fee: LiquidityPoolFeeV18,
	}.ToXDR()
	require.Error(t, err, "two native assets are also not a valid pair")
}

func TestLiquidityPoolParametersToXDRRejectsOutOfOrderPair(t *testing.T) {
	first, second := orderedPair(t,
		CreditAsset{Code: "EUR", Issuer: keypair.MustRandom().Address()},
		CreditAsset{Code: "EUR", Issuer: keypair.MustRandom().Address()},
	)

	_, err := LiquidityPoolParameters{
		AssetA: first, AssetB: second, Fee: LiquidityPoolFeeV18,
	}.ToXDR()
	require.NoError(t, err)

	_, err = LiquidityPoolParameters{
		AssetA: second, AssetB: first, Fee: LiquidityPoolFeeV18,
	}.ToXDR()
	require.Error(t, err)
}

func TestChangeTrustAcceptsNativePairedPool(t *testing.T) {
	op := &ChangeTrust{
		Line: LiquidityPoolShareChangeTrustAsset{
			LiquidityPoolParameters: LiquidityPoolParameters{
				AssetA: NativeAsset{},
				AssetB: CreditAsset{Code: "USD", Issuer: keypair.MustRandom().Address()},
				Fee:    LiquidityPoolFeeV18,
			},
		},
		Limit: MaxTrustlineLimit,
	}
	require.NoError(t, op.Validate())
	_, err := op.BuildXDR()
	require.NoError(t, err)
}
