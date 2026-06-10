package xdr

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// These tests cover the Soroban authorization "v2" XDR additions:
//   - SorobanCredentials arms SOROBAN_CREDENTIALS_ADDRESS_V2 and
//     SOROBAN_CREDENTIALS_ADDRESS_WITH_DELEGATES
//   - SorobanDelegateSignature (including its recursive nestedDelegates)
//   - SorobanAddressCredentialsWithDelegates
//   - HashIdPreimage arm ENVELOPE_TYPE_SOROBAN_AUTHORIZATION_WITH_ADDRESS
//
// They verify that the generated encoder/decoder round-trips each construct
// byte-for-byte and that the union constructors/accessors agree with the
// stored discriminant.

func testScAddress(b byte) ScAddress {
	return ScAddress{
		Type:       ScAddressTypeScAddressTypeContract,
		ContractId: &ContractId{b},
	}
}

func testScVal(v bool) ScVal {
	return ScVal{Type: ScValTypeScvBool, B: &v}
}

// testInvocation returns a minimal, fully-populated SorobanAuthorizedInvocation
// (the ContractFn arm requires a non-nil InvokeContractArgs).
func testInvocation() SorobanAuthorizedInvocation {
	return SorobanAuthorizedInvocation{
		Function: SorobanAuthorizedFunction{
			Type: SorobanAuthorizedFunctionTypeSorobanAuthorizedFunctionTypeContractFn,
			ContractFn: &InvokeContractArgs{
				ContractAddress: testScAddress(7),
				FunctionName:    "transfer",
				Args:            []ScVal{testScVal(true)},
			},
		},
	}
}

func testAddressCredentials() SorobanAddressCredentials {
	return SorobanAddressCredentials{
		Address:                   testScAddress(1),
		Nonce:                     42,
		SignatureExpirationLedger: 100,
		Signature:                 testScVal(true),
	}
}

// roundTrip marshals v, unmarshals into a fresh value of the same type, and
// asserts the decoded value equals the original. T must be an XDR type whose
// pointer implements MarshalBinary/UnmarshalBinary.
func roundTrip[T interface {
	MarshalBinary() ([]byte, error)
}, PT interface {
	*T
	UnmarshalBinary([]byte) error
}](t *testing.T, v T) {
	t.Helper()
	data, err := v.MarshalBinary()
	require.NoError(t, err)

	var decoded T
	require.NoError(t, PT(&decoded).UnmarshalBinary(data))
	require.Equal(t, v, decoded)

	// Re-marshaling the decoded value must reproduce the same bytes.
	reData, err := decoded.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, data, reData)
}

func TestSorobanCredentialsAddressV2RoundTrip(t *testing.T) {
	creds, err := NewSorobanCredentials(
		SorobanCredentialsTypeSorobanCredentialsAddressV2,
		testAddressCredentials(),
	)
	require.NoError(t, err)

	// Constructor populated the AddressV2 arm and left the v1 Address arm unset.
	require.NotNil(t, creds.AddressV2)
	require.Nil(t, creds.Address)

	got, ok := creds.GetAddressV2()
	require.True(t, ok)
	require.Equal(t, testAddressCredentials(), got)

	// The v1 accessor must not report a value for a v2 union.
	_, ok = creds.GetAddress()
	require.False(t, ok)

	roundTrip[SorobanCredentials](t, creds)
}

func TestSorobanCredentialsAddressWithDelegatesRoundTrip(t *testing.T) {
	withDelegates := SorobanAddressCredentialsWithDelegates{
		AddressCredentials: testAddressCredentials(),
		Delegates: []SorobanDelegateSignature{
			{
				Address:   testScAddress(2),
				Signature: testScVal(true),
				// A nested delegate exercises the recursive nestedDelegates<> field.
				NestedDelegates: []SorobanDelegateSignature{
					{
						Address:   testScAddress(3),
						Signature: testScVal(false),
					},
				},
			},
			{
				Address:   testScAddress(4),
				Signature: testScVal(false),
			},
		},
	}

	creds, err := NewSorobanCredentials(
		SorobanCredentialsTypeSorobanCredentialsAddressWithDelegates,
		withDelegates,
	)
	require.NoError(t, err)

	require.NotNil(t, creds.AddressWithDelegates)
	got, ok := creds.GetAddressWithDelegates()
	require.True(t, ok)
	require.Equal(t, withDelegates, got)

	roundTrip[SorobanCredentials](t, creds)
}

func TestSorobanDelegateSignatureNestedRoundTrip(t *testing.T) {
	// A delegate signature with two levels of nesting.
	sig := SorobanDelegateSignature{
		Address:   testScAddress(1),
		Signature: testScVal(true),
		NestedDelegates: []SorobanDelegateSignature{
			{
				Address:   testScAddress(2),
				Signature: testScVal(false),
				NestedDelegates: []SorobanDelegateSignature{
					{
						Address:   testScAddress(3),
						Signature: testScVal(true),
					},
				},
			},
		},
	}

	roundTrip[SorobanDelegateSignature](t, sig)
}

func TestHashIdPreimageSorobanAuthorizationWithAddressRoundTrip(t *testing.T) {
	preimage, err := NewHashIdPreimage(
		EnvelopeTypeEnvelopeTypeSorobanAuthorizationWithAddress,
		HashIdPreimageSorobanAuthorizationWithAddress{
			NetworkId:                 Hash{9, 9, 9},
			Nonce:                     1234,
			SignatureExpirationLedger: 5678,
			Address:                   testScAddress(1),
			Invocation:                testInvocation(),
		},
	)
	require.NoError(t, err)

	require.Equal(t, EnvelopeTypeEnvelopeTypeSorobanAuthorizationWithAddress, preimage.Type)
	require.NotNil(t, preimage.SorobanAuthorizationWithAddress)

	got, ok := preimage.GetSorobanAuthorizationWithAddress()
	require.True(t, ok)
	require.Equal(t, Int64(1234), got.Nonce)
	require.Equal(t, testInvocation(), got.Invocation)

	roundTrip[HashIdPreimage](t, preimage)
}

// TestSorobanCredentialsArmForSwitch documents the discriminant -> arm mapping
// for the new credential types, guarding against accidental reordering.
func TestSorobanCredentialsArmForSwitch(t *testing.T) {
	for _, tc := range []struct {
		credType SorobanCredentialsType
		arm      string
	}{
		{SorobanCredentialsTypeSorobanCredentialsSourceAccount, ""},
		{SorobanCredentialsTypeSorobanCredentialsAddress, "Address"},
		{SorobanCredentialsTypeSorobanCredentialsAddressV2, "AddressV2"},
		{SorobanCredentialsTypeSorobanCredentialsAddressWithDelegates, "AddressWithDelegates"},
	} {
		arm, ok := SorobanCredentials{}.ArmForSwitch(int32(tc.credType))
		require.True(t, ok, "type %v should be a valid arm", tc.credType)
		require.Equal(t, tc.arm, arm)
	}
}
