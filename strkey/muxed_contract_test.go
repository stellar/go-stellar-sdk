package strkey

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Vectors generated with rs-stellar-strkey@80085faa.
var muxedContractVectors = []struct {
	id      uint64
	address string
}{
	{0, "WA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAAAAAAAAAAAAAMIU"},
	{1, "WA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAAAAAAAAAAAAE4ZU"},
	{123, "WA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAAAAAAAAAAAPPSEK"},
	{123456, "WA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAAAAAAAAAPCIA6IG"},
	{9223372036854775808, "WA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQHAAAAAAAAAAAACMXO"},
	{18446744073709551615, "WA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQH77777777777774SY"},
}

const muxedContractBase = "CA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAXE"

func TestMuxedContract_Address(t *testing.T) {
	for _, v := range muxedContractVectors {
		muxed := MuxedContract{}
		require.NoError(t, muxed.SetContractID(muxedContractBase))
		muxed.SetID(v.id)
		address, err := muxed.Address()
		require.NoError(t, err)
		assert.Equal(t, v.address, address)
	}

	address, err := (&MuxedContract{}).Address()
	require.NoError(t, err)
	assert.Equal(t, "WAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAADCKU", address)
}

func TestDecodeMuxedContract(t *testing.T) {
	for _, v := range muxedContractVectors {
		muxed, err := DecodeMuxedContract(v.address)
		require.NoError(t, err)
		assert.Equal(t, v.id, muxed.ID())
		contractID, err := muxed.ContractID()
		require.NoError(t, err)
		assert.Equal(t, muxedContractBase, contractID)
		contract := muxed.Contract()
		assert.Equal(t, MustDecode(VersionByteContract, muxedContractBase), contract[:])
		assert.True(t, IsValidMuxedContractAddress(v.address))
	}

	for _, invalid := range []string{
		muxedContractBase,
		"MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK",
		// non-canonical: differs from a valid address only in the unused padding bit
		"WA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAAAAAAAAAPCIA6IH",
		// bad checksum
		"WA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAAAAAAAAAPCIA6IE",
		// truncated
		"WA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAAAAAAAAAPCIA6",
		"",
	} {
		muxed, err := DecodeMuxedContract(invalid)
		assert.EqualError(t, err, "invalid muxed contract", invalid)
		assert.Nil(t, muxed)
		assert.False(t, IsValidMuxedContractAddress(invalid), invalid)
	}
}

func TestDecodeMuxedContractRejectionReasons(t *testing.T) {
	_, err := Decode(VersionByteMuxedContract, "WA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAAAAAAAAAPCIA6IE")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid checksum")

	_, err = Decode(VersionByteMuxedContract, "WA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAAAAAAAAAPCIA6IH")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-canonical")
}

// Vectors from the SEP-23 muxed contract addition (stellar/stellar-protocol#2020).
func TestMuxedContractSEP23Vectors(t *testing.T) {
	const contract = "CA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUWDA"
	hash := []byte{
		0x3f, 0x0c, 0x34, 0xbf, 0x93, 0xad, 0x0d, 0x99,
		0x71, 0xd0, 0x4c, 0xcc, 0x90, 0xf7, 0x05, 0x51,
		0x1c, 0x83, 0x8a, 0xad, 0x97, 0x34, 0xa4, 0xa2,
		0xfb, 0x0d, 0x7a, 0x03, 0xfc, 0x7f, 0xe8, 0x9a,
	}

	for _, v := range []struct {
		id      uint64
		address string
	}{
		{0, "WA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAAAWWC"},
		// id above the maximum signed 64-bit integer
		{9223372036854775808, "WA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAACWJY"},
	} {
		muxed, err := DecodeMuxedContract(v.address)
		require.NoError(t, err, v.address)
		assert.Equal(t, v.id, muxed.ID())
		contractBytes := muxed.Contract()
		assert.Equal(t, hash, contractBytes[:])
		contractID, err := muxed.ContractID()
		require.NoError(t, err)
		assert.Equal(t, contract, contractID)
		address, err := muxed.Address()
		require.NoError(t, err)
		assert.Equal(t, v.address, address)
	}

	for _, invalid := range []string{
		// the unused trailing bit is not zero
		"WA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAAAWWD",
		// length congruent to 6 mod 8
		"WA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAAAWWCA",
		// decodes to 44 bytes instead of 43
		"WA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAAAAIOUI",
		// the low 3 bits of the version byte are 7
		"W47QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAADXHW",
		// invalid checksum
		"WA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAAAWWA",
	} {
		muxed, err := DecodeMuxedContract(invalid)
		assert.EqualError(t, err, "invalid muxed contract", invalid)
		assert.Nil(t, muxed)
		assert.False(t, IsValidMuxedContractAddress(invalid), invalid)
	}
}

func TestMuxedContract_SetContractID(t *testing.T) {
	muxed := MuxedContract{}
	assert.EqualError(t, muxed.SetContractID(""), "invalid contract address")
	assert.EqualError(t, muxed.SetContractID("GA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQHES5"), "invalid contract address")
	assert.Equal(t, MuxedContract{}, muxed)
}
