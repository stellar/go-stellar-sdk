package token_transfer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// TestExtractAddressMuxedContract verifies that CAP-0084 muxed contract
// destinations are parsed without panicking and yield the underlying base
// contract address. The muxed id is not surfaced here; that is deferred to
// horizon's processor.
func TestExtractAddressMuxedContract(t *testing.T) {
	contractID := xdr.ContractId([32]byte{1})
	val := xdr.ScVal{
		Type: xdr.ScValTypeScvAddress,
		Address: &xdr.ScAddress{
			Type: xdr.ScAddressTypeScAddressTypeMuxedContract,
			MuxedContract: &xdr.MuxedContract{
				Id:         99,
				ContractId: contractID,
			},
		},
	}

	got, err := extractAddress(val)
	require.NoError(t, err)
	require.Equal(t, strkey.MustEncode(strkey.VersionByteContract, contractID[:]), got)
}
