package network

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashTransaction(t *testing.T) {
	var txe xdr.TransactionEnvelope
	err := xdr.SafeUnmarshalBase64("AAAAAGL8HQvQkbK2HA3WVjRrKmjX00fG8sLI7m0ERwJW/AX3AAAACgAAAAAAAAABAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAArqN6LeOagjxMaUP96Bzfs9e0corNZXzBWJkFoK7kvkwAAAAAO5rKAAAAAAAAAAABVvwF9wAAAEAKZ7IPj/46PuWU6ZOtyMosctNAkXRNX9WCAI5RnfRk+AyxDLoDZP/9l3NvsxQtWj9juQOuoBlFLnWu8intgxQA", &txe)

	require.NoError(t, err)

	expected := [32]byte{
		0xc4, 0x92, 0xd8, 0x7c, 0x46, 0x42, 0x81, 0x5d,
		0xfb, 0x3c, 0x7d, 0xcc, 0xe0, 0x1a, 0xf4, 0xef,
		0xfd, 0x16, 0x2b, 0x03, 0x10, 0x64, 0x09, 0x8a,
		0x0d, 0x78, 0x6b, 0x6e, 0x0a, 0x00, 0xfd, 0x74,
	}
	actual, err := HashTransactionV0(txe.V0.Tx, TestNetworkPassphrase)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)

	actual, err = HashTransactionInEnvelope(txe, TestNetworkPassphrase)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)

	_, err = HashTransactionV0(txe.V0.Tx, "")
	assert.Contains(t, err.Error(), "empty network passphrase")
	_, err = HashTransactionInEnvelope(txe, "")
	assert.Contains(t, err.Error(), "empty network passphrase")

	tx := xdr.Transaction{
		SourceAccount: txe.SourceAccount(),
		Fee:           xdr.Uint32(txe.Fee()),
		Memo:          txe.Memo(),
		Operations:    txe.Operations(),
		SeqNum:        xdr.SequenceNumber(txe.SeqNum()),
		Cond:          xdr.NewPreconditionsWithTimeBounds(txe.TimeBounds()),
	}

	actual, err = HashTransaction(tx, TestNetworkPassphrase)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)

	txe.Type = xdr.EnvelopeTypeEnvelopeTypeTx
	txe.V0 = nil
	txe.V1 = &xdr.TransactionV1Envelope{
		Tx: tx,
	}
	actual, err = HashTransactionInEnvelope(txe, TestNetworkPassphrase)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)

	// sadpath: empty passphrase
	_, err = HashTransaction(tx, "")
	assert.Contains(t, err.Error(), "empty network passphrase")
	_, err = HashTransactionInEnvelope(txe, "")
	assert.Contains(t, err.Error(), "empty network passphrase")

	sourceAID := xdr.MustAddress("GCLOMB72ODBFUGK4E2BK7VMR3RNZ5WSTMEOGNA2YUVHFR3WMH2XBAB6H")
	feeBumpTx := xdr.FeeBumpTransaction{
		Fee:       123456,
		FeeSource: sourceAID.ToMuxedAccount(),
		InnerTx: xdr.FeeBumpTransactionInnerTx{
			Type: xdr.EnvelopeTypeEnvelopeTypeTx,
			V1: &xdr.TransactionV1Envelope{
				Tx:         tx,
				Signatures: []xdr.DecoratedSignature{},
			},
		},
	}

	expected = [32]uint8{
		0x4d, 0x4c, 0xe9, 0x2, 0x63, 0x72, 0x27, 0xfb,
		0x8b, 0x52, 0x2a, 0xe4, 0x8c, 0xcd, 0xd6, 0x9d,
		0x32, 0x51, 0x72, 0x46, 0xd9, 0xfc, 0x23, 0xff,
		0x8b, 0x7a, 0x85, 0xdd, 0x4b, 0xbc, 0xef, 0x5f,
	}
	actual, err = HashFeeBumpTransaction(feeBumpTx, TestNetworkPassphrase)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)

	txe.Type = xdr.EnvelopeTypeEnvelopeTypeTxFeeBump
	txe.V1 = nil
	txe.FeeBump = &xdr.FeeBumpTransactionEnvelope{
		Tx: feeBumpTx,
	}
	actual, err = HashTransactionInEnvelope(txe, TestNetworkPassphrase)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)

	_, err = HashFeeBumpTransaction(feeBumpTx, "")
	assert.Contains(t, err.Error(), "empty network passphrase")
	_, err = HashTransactionInEnvelope(txe, "")
	assert.Contains(t, err.Error(), "empty network passphrase")
}
