package contract

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/keypair"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordedContractCredB64 is a real SOROBAN_CREDENTIALS_ADDRESS authorization
// entry captured from a live testnet simulateTransaction whose authorizer is a
// *contract* (a deployed custom account / smart wallet that exposes
// __check_auth). Decoded, its credential address is of type
// ScAddressTypeScAddressTypeContract — exactly the case that needs an enforcing
// re-simulation for an accurate footprint.
const recordedContractCredB64 = "AAAAAQAAAAEtnfIN7/Rs0neI2+dcpRVsVNf3ZdzNJLL/dNYVMZEapXPmFSd5k9UEAAAAAAAAAAEAAAAAAAAAAdeSi3LCcDzP6vfrn/TvTVBKVai5efybRQ6iyEK00c5hAAAACHRyYW5zZmVyAAAAAwAAABIAAAABLZ3yDe/0bNJ3iNvnXKUVbFTX92XczSSy/3TWFTGRGqUAAAASAAAAAAAAAACconL5SfOdP/bOdfJ62Yn1AoFKNL0WyZTRfO2+gD5eiAAAAAoAAAAAAAAAAAAAAAAAmJaAAAAAAA=="

func decodeAuthEntry(t *testing.T, b64 string) xdr.SorobanAuthorizationEntry {
	t.Helper()
	var e xdr.SorobanAuthorizationEntry
	require.NoError(t, xdr.SafeUnmarshalBase64(b64, &e))
	return e
}

// addressCredEntry builds a minimal SOROBAN_CREDENTIALS_ADDRESS entry for the
// given address. RootInvocation is left zero — the accessors under test only
// inspect the credential, never the invocation tree.
func addressCredEntry(addr xdr.ScAddress) xdr.SorobanAuthorizationEntry {
	return xdr.SorobanAuthorizationEntry{
		Credentials: xdr.SorobanCredentials{
			Type: xdr.SorobanCredentialsTypeSorobanCredentialsAddress,
			Address: &xdr.SorobanAddressCredentials{
				Address:   addr,
				Nonce:     1,
				Signature: xdr.ScVal{Type: xdr.ScValTypeScvVoid},
			},
		},
	}
}

// signedAddressCredEntry is addressCredEntry with a populated (non-void)
// Signature, modeling an entry the sign/send slice has already signed. That
// slice sets Signature to the structured Soroban signature — a vec of
// {public_key, signature} maps — so the fixture mirrors that shape, but
// NeedsNonInvokerSigningBy only inspects whether the type is non-void.
func signedAddressCredEntry(t *testing.T, addr xdr.ScAddress) xdr.SorobanAuthorizationEntry {
	t.Helper()
	sigMap, err := xdr.ScvMap(map[string]xdr.ScVal{
		"public_key": xdr.ScvBytes(make([]byte, 32)),
		"signature":  xdr.ScvBytes(make([]byte, 64)),
	})
	require.NoError(t, err)
	entry := addressCredEntry(addr)
	entry.Credentials.Address.Signature = xdr.ScvVec(sigMap)
	return entry
}

func accountAddr(t *testing.T) xdr.ScAddress {
	t.Helper()
	return xdr.ScAddress{
		Type:      xdr.ScAddressTypeScAddressTypeAccount,
		AccountId: xdr.MustAddressPtr(keypair.MustRandom().Address()),
	}
}

func contractAddr(seed byte) xdr.ScAddress {
	var cid xdr.ContractId
	for i := range cid {
		cid[i] = seed
	}
	return xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeContract, ContractId: &cid}
}

// simulatedWithAuth returns an AssembledTransaction that looks like Simulate has
// run (non-nil Simulation) carrying the given recorded auth entries, so the
// auth accessors evaluate them rather than short-circuiting.
func simulatedWithAuth(entries ...xdr.SorobanAuthorizationEntry) *AssembledTransaction {
	return &AssembledTransaction{
		Simulation:  &protocol.SimulateTransactionResponse{},
		AuthEntries: entries,
	}
}

// The embedded fixture must really be a contract-address credential, otherwise
// the rest of these tests prove nothing.
func TestRecordedContractCredentialFixtureIsContractAddress(t *testing.T) {
	e := decodeAuthEntry(t, recordedContractCredB64)
	require.Equal(t, xdr.SorobanCredentialsTypeSorobanCredentialsAddress, e.Credentials.Type)
	require.NotNil(t, e.Credentials.Address)
	require.Equal(t, xdr.ScAddressTypeScAddressTypeContract, e.Credentials.Address.Address.Type,
		"fixture must be a contract-address credential")
}

func TestRequiresEnforcingResimulationAndNeedsNonInvokerSigningBy(t *testing.T) {
	sourceEntry, _ := cannedAuthEntry(t) // SourceAccount credentials
	contractEntry := decodeAuthEntry(t, recordedContractCredB64)
	acctEntry := addressCredEntry(accountAddr(t))
	dupContract := contractAddr(0x11)

	cases := []struct {
		name        string
		entries     []xdr.SorobanAuthorizationEntry
		wantEnforce bool
		wantSigners int
	}{
		{"no entries", nil, false, 0},
		{"source account only", []xdr.SorobanAuthorizationEntry{sourceEntry}, false, 0},
		{"classic account address signer", []xdr.SorobanAuthorizationEntry{acctEntry}, false, 1},
		{"recorded contract signer", []xdr.SorobanAuthorizationEntry{contractEntry}, true, 1},
		{
			"mixed source + account + contract",
			[]xdr.SorobanAuthorizationEntry{sourceEntry, acctEntry, contractEntry},
			true, 2,
		},
		{
			"duplicate contract address deduped",
			[]xdr.SorobanAuthorizationEntry{addressCredEntry(dupContract), addressCredEntry(dupContract)},
			true, 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			at := simulatedWithAuth(tc.entries...)
			assert.Equal(t, tc.wantEnforce, at.RequiresEnforcingResimulation())
			assert.Len(t, at.NeedsNonInvokerSigningBy(), tc.wantSigners)
		})
	}
}

func TestNeedsNonInvokerSigningByReturnsTheContractAddress(t *testing.T) {
	at := simulatedWithAuth(decodeAuthEntry(t, recordedContractCredB64))
	signers := at.NeedsNonInvokerSigningBy()
	require.Len(t, signers, 1)
	assert.Equal(t, xdr.ScAddressTypeScAddressTypeContract, signers[0].Type,
		"the recorded contract signer should be surfaced")
}

// An Address-credentialed entry whose Signature is already populated (non-void)
// has been signed, so NeedsNonInvokerSigningBy must skip it. This exercises the
// Signature.Type != ScvVoid guard; every other fixture here carries a void
// signature and so never reaches that branch.
func TestNeedsNonInvokerSigningBySkipsAlreadySignedEntries(t *testing.T) {
	signedContract := signedAddressCredEntry(t, contractAddr(0x22))
	signedAccount := signedAddressCredEntry(t, accountAddr(t))
	unsigned := addressCredEntry(accountAddr(t))

	t.Run("single signed entry yields no signers", func(t *testing.T) {
		at := simulatedWithAuth(signedContract)
		assert.Empty(t, at.NeedsNonInvokerSigningBy())
	})

	t.Run("all entries signed yields no signers", func(t *testing.T) {
		at := simulatedWithAuth(signedContract, signedAccount)
		assert.Empty(t, at.NeedsNonInvokerSigningBy())
	})

	// The signature gate applies regardless of address type: two signed entries
	// (one contract, one account) are skipped and only the still-unsigned signer
	// is reported.
	t.Run("only the still-unsigned signer is reported", func(t *testing.T) {
		at := simulatedWithAuth(signedContract, unsigned, signedAccount)
		signers := at.NeedsNonInvokerSigningBy()
		require.Len(t, signers, 1)

		wantKey, err := unsigned.Credentials.Address.Address.String()
		require.NoError(t, err)
		gotKey, err := signers[0].String()
		require.NoError(t, err)
		assert.Equal(t, wantKey, gotKey, "the still-unsigned signer must be the one reported")
	})
}

// Both accessors mirror IsReadCall's conservatism: nil receiver and
// not-yet-simulated transactions report no signers / no enforcing requirement,
// even if AuthEntries happen to be populated.
func TestAuthAccessorsConservativeBeforeSimulate(t *testing.T) {
	var nilAT *AssembledTransaction
	assert.False(t, nilAT.RequiresEnforcingResimulation())
	assert.Nil(t, nilAT.NeedsNonInvokerSigningBy())

	notSimulated := &AssembledTransaction{
		AuthEntries: []xdr.SorobanAuthorizationEntry{decodeAuthEntry(t, recordedContractCredB64)},
	}
	assert.False(t, notSimulated.RequiresEnforcingResimulation(),
		"must be conservative until Simulate has run")
	assert.Nil(t, notSimulated.NeedsNonInvokerSigningBy())
}
