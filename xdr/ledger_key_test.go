package xdr

import (
	"encoding/base64"
	"testing"

	"github.com/stellar/go-stellar-sdk/gxdr"
	"github.com/stellar/go-stellar-sdk/randxdr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLedgerKeyTrustLineBinaryMaxLength(t *testing.T) {
	key := &LedgerKey{}
	err := key.SetTrustline(
		MustAddress("GBFLTCDLOE6YQ74B66RH3S2UW5I2MKZ5VLTM75F4YMIWUIXRIFVNRNIF"),
		MustNewCreditAsset("123456789012", "GBFLTCDLOE6YQ74B66RH3S2UW5I2MKZ5VLTM75F4YMIWUIXRIFVNRNIF").ToTrustLineAsset(),
	)
	assert.NoError(t, err)

	compressed, err := key.MarshalBinary()
	assert.NoError(t, err)
	assert.Equal(t, len(compressed), 92)
	bcompressed := base64.StdEncoding.EncodeToString(compressed)
	assert.Equal(t, len(bcompressed), 124)
}

func TestTrimRightZeros(t *testing.T) {
	require.Equal(t, []byte(nil), trimRightZeros(nil))
	require.Equal(t, []byte{}, trimRightZeros([]byte{}))
	require.Equal(t, []byte{}, trimRightZeros([]byte{0x0}))
	require.Equal(t, []byte{}, trimRightZeros([]byte{0x0, 0x0}))
	require.Equal(t, []byte{0x1}, trimRightZeros([]byte{0x1}))
	require.Equal(t, []byte{0x1}, trimRightZeros([]byte{0x1, 0x0}))
	require.Equal(t, []byte{0x1}, trimRightZeros([]byte{0x1, 0x0, 0x0}))
	require.Equal(t, []byte{0x1}, trimRightZeros([]byte{0x1, 0x0, 0x0, 0x0}))
	require.Equal(t, []byte{0x1, 0x2}, trimRightZeros([]byte{0x1, 0x2}))
	require.Equal(t, []byte{0x1, 0x2}, trimRightZeros([]byte{0x1, 0x2, 0x0}))
	require.Equal(t, []byte{0x1, 0x2}, trimRightZeros([]byte{0x1, 0x2, 0x0, 0x0}))
	require.Equal(t, []byte{0x0, 0x2}, trimRightZeros([]byte{0x0, 0x2, 0x0, 0x0}))
	require.Equal(t, []byte{0x0, 0x2, 0x0, 0x1}, trimRightZeros([]byte{0x0, 0x2, 0x0, 0x1, 0x0}))
}

func TestLedgerKeyEqualsCoverage(t *testing.T) {
	gen := randxdr.NewGenerator()
	for i := 0; i < 10000; i++ {
		ledgerKey := LedgerKey{}

		shape := &gxdr.LedgerKey{}
		gen.Next(
			shape,
			[]randxdr.Preset{},
		)
		assert.NoError(t, gxdr.Convert(shape, &ledgerKey))

		clonedLedgerKey := LedgerKey{}
		assert.NoError(t, gxdr.Convert(shape, &clonedLedgerKey))
		assert.True(t, ledgerKey.Equals(clonedLedgerKey))
	}
}
