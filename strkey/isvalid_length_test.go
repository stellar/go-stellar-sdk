package strkey_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/strkey"
)

func TestIsValidRejectsNonCanonicalPayloadLength(t *testing.T) {
	cases := []struct {
		name     string
		vb       strkey.VersionByte
		validate func(interface{}) bool
		validLen int
	}{
		{"contract", strkey.VersionByteContract, strkey.IsValidContractAddress, 32},
		{"ed25519", strkey.VersionByteAccountID, strkey.IsValidEd25519PublicKey, 32},
		{"claimable_balance", strkey.VersionByteClaimableBalance, strkey.IsValidClaimableBalance, 33},
		{"liquidity_pool", strkey.VersionByteLiquidityPool, strkey.IsValidLiquidityPool, 32},
		{"seed", strkey.VersionByteSeed, strkey.IsValidEd25519SecretSeed, 32},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			overlong := make([]byte, tc.validLen+4)
			s, err := strkey.Encode(tc.vb, overlong)
			require.NoError(t, err)
			require.False(t, tc.validate(s))

			short := make([]byte, tc.validLen-1)
			s, err = strkey.Encode(tc.vb, short)
			require.NoError(t, err)
			require.False(t, tc.validate(s))

			canonical := make([]byte, tc.validLen)
			s, err = strkey.Encode(tc.vb, canonical)
			require.NoError(t, err)
			require.True(t, tc.validate(s))
		})
	}

	t.Run("muxed_account", func(t *testing.T) {
		overlong, err := strkey.Encode(strkey.VersionByteMuxedAccount, make([]byte, 44))
		require.NoError(t, err)
		require.False(t, strkey.IsValidMuxedAccountEd25519PublicKey(overlong))

		short, err := strkey.Encode(strkey.VersionByteMuxedAccount, make([]byte, 39))
		require.NoError(t, err)
		require.False(t, strkey.IsValidMuxedAccountEd25519PublicKey(short))

		canonical, err := strkey.Encode(strkey.VersionByteMuxedAccount, make([]byte, 40))
		require.NoError(t, err)
		require.True(t, strkey.IsValidMuxedAccountEd25519PublicKey(canonical))
	})
}
