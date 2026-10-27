package network_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const testPassphrase = network.TestNetworkPassphrase

func mustEnvelopeView(t *testing.T, env xdr.TransactionEnvelope) xdr.TransactionEnvelopeView {
	t.Helper()
	raw, err := env.MarshalBinary()
	require.NoError(t, err)
	return xdr.TransactionEnvelopeView(raw)
}

// TestTransactionViewHasher_MatchesParsed proves the view hasher returns the
// same hash as the parsed network.HashTransactionInEnvelope for every envelope
// type (classic Tx, soroban Tx, legacy TX_V0 with and without TimeBounds,
// fee-bump). The TimeBounds-absent V0 case pins the wire-equivalence claim in
// Hash's doc comment: absent TimeBounds (0) and PRECOND_NONE (0) encode
// identically, so the V0-as-V1 hashing needs no re-encode for either arm.
func TestTransactionViewHasher_MatchesParsed(t *testing.T) {
	src := xdr.MustMuxedAddress(keypair.MustRandom().Address())
	classicTx := xdr.Transaction{SourceAccount: src, Fee: 100, SeqNum: 1}
	sorobanTx := classicTx
	sorobanTx.Ext = xdr.TransactionExt{V: 1, SorobanData: &xdr.SorobanTransactionData{}}

	v0NoTB := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTxV0,
		V0: &xdr.TransactionV0Envelope{Tx: xdr.TransactionV0{
			SourceAccountEd25519: xdr.Uint256{1, 2, 3},
			Fee:                  100,
			SeqNum:               7,
			TimeBounds:           nil, // the absent arm of the optional
			Operations: []xdr.Operation{{Body: xdr.OperationBody{
				Type: xdr.OperationTypeInflation,
			}}},
		}},
	}

	var v0Env xdr.TransactionEnvelope
	require.NoError(t, xdr.SafeUnmarshalBase64(
		"AAAAAGL8HQvQkbK2HA3WVjRrKmjX00fG8sLI7m0ERwJW/AX3AAAACgAAAAAAAAABAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAArqN6LeOagjxMaUP96Bzfs9e0corNZXzBWJkFoK7kvkwAAAAAO5rKAAAAAAAAAAABVvwF9wAAAEAKZ7IPj/46PuWU6ZOtyMosctNAkXRNX9WCAI5RnfRk+AyxDLoDZP/9l3NvsxQtWj9juQOuoBlFLnWu8intgxQA",
		&v0Env))

	cases := []struct {
		name string
		env  xdr.TransactionEnvelope
	}{
		{
			name: "classic-tx",
			env:  xdr.TransactionEnvelope{Type: xdr.EnvelopeTypeEnvelopeTypeTx, V1: &xdr.TransactionV1Envelope{Tx: classicTx}},
		},
		{
			name: "soroban-tx",
			env:  xdr.TransactionEnvelope{Type: xdr.EnvelopeTypeEnvelopeTypeTx, V1: &xdr.TransactionV1Envelope{Tx: sorobanTx}},
		},
		{
			name: "legacy-tx-v0",
			env:  v0Env,
		},
		{
			name: "legacy-tx-v0-no-timebounds",
			env:  v0NoTB,
		},
		{
			name: "fee-bump-soroban-inner",
			env: xdr.TransactionEnvelope{
				Type: xdr.EnvelopeTypeEnvelopeTypeTxFeeBump,
				FeeBump: &xdr.FeeBumpTransactionEnvelope{Tx: xdr.FeeBumpTransaction{
					Fee:       123456,
					FeeSource: src,
					InnerTx: xdr.FeeBumpTransactionInnerTx{
						Type: xdr.EnvelopeTypeEnvelopeTypeTx,
						V1:   &xdr.TransactionV1Envelope{Tx: sorobanTx},
					},
				}},
			},
		},
	}

	hasher, err := network.NewTransactionViewHasher(testPassphrase)
	require.NoError(t, err)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, err := network.HashTransactionInEnvelope(tc.env, testPassphrase)
			require.NoError(t, err)

			view := mustEnvelopeView(t, tc.env)
			got, err := hasher.Hash(view)
			require.NoError(t, err)
			assert.Equal(t, want, got, "view hash must match parsed HashTransactionInEnvelope")
		})
	}
}

func TestTransactionViewHasher_EmptyPassphrase(t *testing.T) {
	_, err := network.NewTransactionViewHasher("")
	require.ErrorContains(t, err, "empty network passphrase")
	_, err = network.NewTransactionViewHasher("   ")
	require.ErrorContains(t, err, "empty network passphrase")
}

func TestTransactionViewHasher_InvalidType(t *testing.T) {
	hasher, err := network.NewTransactionViewHasher(testPassphrase)
	require.NoError(t, err)
	// A non-transaction envelope type (e.g. an SCP value tag) must be rejected.
	bad := xdr.TransactionEnvelopeView([]byte{0x00, 0x00, 0x00, 0x09})
	_, err = hasher.Hash(bad)
	require.Error(t, err)
}
