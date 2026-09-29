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
		// bad checksum
		"WA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQGAAAAAAAAAPCIA6IH",
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

func TestMuxedContract_SetContractID(t *testing.T) {
	muxed := MuxedContract{}
	assert.EqualError(t, muxed.SetContractID(""), "invalid contract address")
	assert.EqualError(t, muxed.SetContractID("GA3D5KRYM6CB7OWQ6TWYRR3Z4T7GNZLKERYNZGGA5SOAOPIFY6YQHES5"), "invalid contract address")
	assert.Equal(t, MuxedContract{}, muxed)
}
