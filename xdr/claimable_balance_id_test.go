package xdr

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stellar/go-stellar-sdk/gxdr"
	"github.com/stellar/go-stellar-sdk/randxdr"
	"github.com/stellar/go-stellar-sdk/strkey"
)

func TestClaimableBalanceIdStrKey(t *testing.T) {
	gen := randxdr.NewGenerator()
	// generate 10,000 random claimable balance ids and ensure that the strkey
	// encoding / decoding round trips successfully
	for i := 0; i < 10000; i++ {
		id := &ClaimableBalanceId{}
		shape := &gxdr.ClaimableBalanceID{}
		gen.Next(
			shape,
			[]randxdr.Preset{},
		)
		assert.NoError(t, gxdr.Convert(shape, id))

		encoded, err := id.EncodeToStrkey()
		assert.NoError(t, err)

		var decoded ClaimableBalanceId
		assert.NoError(t, decoded.DecodeFromStrkey(encoded))

		serializedBytes, err := id.MarshalBinary()
		assert.NoError(t, err)
		serializedDecoded, err := decoded.MarshalBinary()
		assert.NoError(t, err)
		assert.Equal(t, serializedBytes, serializedDecoded)
	}
}

func TestClaimableBalanceIdDecodeErrors(t *testing.T) {
	var decoded ClaimableBalanceId
	payload := []byte{
		// first byte represents ClaimableBalanceIDType (in this case it's V0)
		0x00,
		// the remaining 32 bytes are the contents of the claimable balance hash
		0x3f, 0x0c, 0x34, 0xbf, 0x93, 0xad, 0x0d, 0x99,
		0x71, 0xd0, 0x4c, 0xcc, 0x90, 0xf7, 0x05, 0x51,
		0x1c, 0x83, 0x8a, 0xad, 0x97, 0x34, 0xa4, 0xa2,
		0xfb, 0x0d, 0x7a, 0x03, 0xfc, 0x7f, 0xe8, 0x9a,
	}
	address := "BAAD6DBUX6J22DMZOHIEZTEQ64CVCHEDRKWZONFEUL5Q26QD7R76RGR4TU"
	assert.Equal(t, address, strkey.MustEncode(strkey.VersionByteClaimableBalance, payload))

	payload[0] = 1
	invalidIdTypeAddress := strkey.MustEncode(strkey.VersionByteClaimableBalance, payload)
	assert.EqualError(t, decoded.DecodeFromStrkey(invalidIdTypeAddress), "invalid claimable balance id type: 1")

	payload[0] = 0
	payload = append(payload, 0)
	invalidLengthAddress := strkey.MustEncode(strkey.VersionByteClaimableBalance, payload)
	assert.EqualError(t, decoded.DecodeFromStrkey(invalidLengthAddress), "invalid payload length, expected 33 but got 34")
}
