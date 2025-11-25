package xdr

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/gxdr"
	"github.com/stellar/go-stellar-sdk/randxdr"
	"github.com/stellar/go-stellar-sdk/strkey"
)

func TestScValEqualsCoverage(t *testing.T) {
	gen := randxdr.NewGenerator()
	for i := 0; i < 30000; i++ {
		scVal := ScVal{}

		shape := &gxdr.SCVal{}
		gen.Next(
			shape,
			[]randxdr.Preset{},
		)
		require.NoError(t, gxdr.Convert(shape, &scVal))

		clonedScVal := ScVal{}
		require.NoError(t, gxdr.Convert(shape, &clonedScVal))
		require.True(t, scVal.Equals(clonedScVal), "scVal: %#v, clonedScVal: %#v", scVal, clonedScVal)
	}
}

func TestScValStringCoverage(t *testing.T) {
	gen := randxdr.NewGenerator()
	for i := 0; i < 30000; i++ {
		scVal := ScVal{}

		shape := &gxdr.SCVal{}
		gen.Next(
			shape,
			[]randxdr.Preset{},
		)
		require.NoError(t, gxdr.Convert(shape, &scVal))

		var str string
		require.NotPanics(t, func() {
			str = scVal.String()
		})
		require.NotEqual(t, str, "unknown")
	}
}

func TestScAddressString(t *testing.T) {
	contractID := [32]byte{1}
	cbID := [32]byte{1}
	poolID := [32]byte{1}

	for _, testCase := range []struct {
		address  ScAddress
		expected string
	}{
		{
			address: ScAddress{
				Type: ScAddressTypeScAddressTypeAccount,
				AccountId: &AccountId{
					Type:    PublicKeyTypePublicKeyTypeEd25519,
					Ed25519: &Uint256{1},
				},
			},
			expected: AccountId{
				Type:    PublicKeyTypePublicKeyTypeEd25519,
				Ed25519: &Uint256{1},
			}.Address(),
		},
		{
			address: ScAddress{
				Type: ScAddressTypeScAddressTypeMuxedAccount,
				MuxedAccount: &MuxedEd25519Account{
					Id:      1,
					Ed25519: Uint256{2},
				},
			},
			expected: (&MuxedAccount{
				Type: CryptoKeyTypeKeyTypeMuxedEd25519,
				Med25519: &MuxedAccountMed25519{
					Id:      1,
					Ed25519: Uint256{2},
				},
			}).Address(),
		},
		{
			address: ScAddress{
				Type:       ScAddressTypeScAddressTypeContract,
				ContractId: &ContractId{1},
			},
			expected: strkey.MustEncode(strkey.VersionByteContract, contractID[:]),
		},
		{
			address: ScAddress{
				Type: ScAddressTypeScAddressTypeClaimableBalance,
				ClaimableBalanceId: &ClaimableBalanceId{
					Type: ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
					V0:   &Hash{1},
				},
			},
			expected: strkey.MustEncode(strkey.VersionByteClaimableBalance, append([]byte{0}, cbID[:]...)), // The Cb type is included when encoding in strkey
		},
		{
			address: ScAddress{
				Type:            ScAddressTypeScAddressTypeLiquidityPool,
				LiquidityPoolId: &PoolId{1},
			},
			expected: strkey.MustEncode(strkey.VersionByteLiquidityPool, poolID[:]),
		},
	} {
		t.Run(testCase.address.Type.String(), func(t *testing.T) {
			str, err := testCase.address.String()
			require.NoError(t, err)
			require.Equal(t, testCase.expected, str)
		})
	}
}

func TestScAddressStringCoverage(t *testing.T) {
	gen := randxdr.NewGenerator()
	for i := 0; i < 30000; i++ {
		scAddress := ScAddress{}

		shape := &gxdr.SCAddress{}
		gen.Next(
			shape,
			[]randxdr.Preset{},
		)
		require.NoError(t, gxdr.Convert(shape, &scAddress))

		_, err := scAddress.String()
		require.NoError(t, err)
	}
}

func TestScAddressEqualsCoverage(t *testing.T) {
	gen := randxdr.NewGenerator()
	for i := 0; i < 30000; i++ {
		scAddress := ScAddress{}

		shape := &gxdr.SCAddress{}
		gen.Next(
			shape,
			[]randxdr.Preset{},
		)
		require.NoError(t, gxdr.Convert(shape, &scAddress))

		clonedScAddress := ScAddress{}
		require.NoError(t, gxdr.Convert(shape, &clonedScAddress))
		require.True(
			t,
			scAddress.Equals(clonedScAddress),
			"scAddress: %#v, clonedScAddress: %#v", scAddress, clonedScAddress,
		)
	}
}
