package strkey

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeRejectsNonCanonicalPayloadLength(t *testing.T) {
	cases := []struct {
		name     string
		vb       VersionByte
		validLen int
	}{
		{"account_id", VersionByteAccountID, 32},
		{"seed", VersionByteSeed, 32},
		{"hash_tx", VersionByteHashTx, 32},
		{"hash_x", VersionByteHashX, 32},
		{"contract", VersionByteContract, 32},
		{"liquidity_pool", VersionByteLiquidityPool, 32},
		{"claimable_balance", VersionByteClaimableBalance, 33},
		{"muxed_account", VersionByteMuxedAccount, 40},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, l := range []int{tc.validLen - 1, tc.validLen + 1, tc.validLen + 4} {
				enc, err := Encode(tc.vb, make([]byte, l))
				require.NoError(t, err)

				_, err = Decode(tc.vb, enc)
				require.ErrorContains(t, err, "invalid payload length", "length %d", l)

				_, _, err = DecodeAny(enc)
				require.ErrorContains(t, err, "invalid payload length", "length %d", l)
			}

			enc, err := Encode(tc.vb, make([]byte, tc.validLen))
			require.NoError(t, err)

			payload, err := Decode(tc.vb, enc)
			require.NoError(t, err)
			require.Len(t, payload, tc.validLen)

			vb, payload, err := DecodeAny(enc)
			require.NoError(t, err)
			require.Equal(t, tc.vb, vb)
			require.Len(t, payload, tc.validLen)
		})
	}
}

// buildSignedPayload assembles the raw bytes of a signed payload: 32-byte
// signer key, 4-byte declared length, and payloadAndPadding appended verbatim
// (so tests can construct declared/actual mismatches and bad padding).
func buildSignedPayload(declared int, payloadAndPadding []byte) []byte {
	raw := make([]byte, 32+4+len(payloadAndPadding))
	binary.BigEndian.PutUint32(raw[32:36], uint32(declared))
	copy(raw[36:], payloadAndPadding)
	return raw
}

func TestDecodeSignedPayloadBoundaries(t *testing.T) {
	pad4 := func(n int) int { return (n + 3) / 4 * 4 }

	for _, declared := range []int{1, 2, 29, 32, 63, 64} {
		body := make([]byte, pad4(declared))
		for i := 0; i < declared; i++ {
			body[i] = 0xff
		}
		enc, err := Encode(VersionByteSignedPayload, buildSignedPayload(declared, body))
		require.NoError(t, err)

		_, err = Decode(VersionByteSignedPayload, enc)
		require.NoError(t, err, "declared length %d", declared)

		_, _, err = DecodeAny(enc)
		require.NoError(t, err, "declared length %d", declared)
	}

	t.Run("declared length 0", func(t *testing.T) {
		enc, err := Encode(VersionByteSignedPayload, buildSignedPayload(0, nil))
		require.NoError(t, err)
		_, err = Decode(VersionByteSignedPayload, enc)
		require.ErrorContains(t, err, "must be between 1 and 64")
	})

	t.Run("declared length 65", func(t *testing.T) {
		enc, err := Encode(VersionByteSignedPayload, buildSignedPayload(65, make([]byte, 64)))
		require.NoError(t, err)
		_, err = Decode(VersionByteSignedPayload, enc)
		require.ErrorContains(t, err, "must be between 1 and 64")
	})

	t.Run("truncated header", func(t *testing.T) {
		enc, err := Encode(VersionByteSignedPayload, make([]byte, 35))
		require.NoError(t, err)
		_, err = Decode(VersionByteSignedPayload, enc)
		require.ErrorContains(t, err, "expected at least 36")
	})

	t.Run("declared shorter than actual", func(t *testing.T) {
		enc, err := Encode(VersionByteSignedPayload, buildSignedPayload(8, make([]byte, 12)))
		require.NoError(t, err)
		_, err = Decode(VersionByteSignedPayload, enc)
		require.ErrorContains(t, err, "declared payload bytes")
	})

	t.Run("declared longer than actual", func(t *testing.T) {
		enc, err := Encode(VersionByteSignedPayload, buildSignedPayload(16, make([]byte, 12)))
		require.NoError(t, err)
		_, err = Decode(VersionByteSignedPayload, enc)
		require.ErrorContains(t, err, "declared payload bytes")
	})

	t.Run("nonzero padding", func(t *testing.T) {
		body := make([]byte, 32)
		for i := 0; i < 29; i++ {
			body[i] = 0xff
		}
		body[31] = 0x01
		enc, err := Encode(VersionByteSignedPayload, buildSignedPayload(29, body))
		require.NoError(t, err)
		_, err = Decode(VersionByteSignedPayload, enc)
		require.ErrorContains(t, err, "padding must be zero")
	})
}

func TestDecodeSignedPayloadSEP23Vectors(t *testing.T) {
	invalid := []struct {
		name    string
		address string
	}{
		{
			"length prefix shorter than payload",
			"PA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAQACAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4DUPB6IAAAAAAAAPM",
		},
		{
			"length prefix longer than payload",
			"PA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAOQCAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4Z2PQ",
		},
		{
			"no zero padding",
			"PA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAOQCAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4DXFH6",
		},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode(VersionByteSignedPayload, tc.address)
			require.Error(t, err)
			_, _, err = DecodeAny(tc.address)
			require.Error(t, err)
		})
	}
}
