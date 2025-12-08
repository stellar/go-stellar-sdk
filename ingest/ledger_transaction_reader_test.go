package ingest

import (
	"io"
	"testing"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/support/collections/set"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	passphrase = network.TestNetworkPassphrase
	// Test prep:
	//   - two different envelopes which resolve to two different hashes
	//   - two basically-empty metas that contain the corresponding hashes
	//   - a ledger that has 5 txs with metas corresponding to these two envs
	//   - specifically, in the order [first, first, second, second, second]
	//
	// This tests both hash <--> envelope mapping and indexed iteration.
	txEnvs, txHashes, txMetas = makeTransactions(5)
	// barebones LCM structure so that the tx reader works w/o nil derefs, 5 txs
	ledgerCloseMeta = xdr.LedgerCloseMeta{V: 1,
		V1: &xdr.LedgerCloseMetaV1{
			TxProcessing: txMetas,
			TxSet: xdr.GeneralizedTransactionSet{V: 1,
				V1TxSet: &xdr.TransactionSetV1{
					Phases: []xdr.TransactionPhase{{V: 0,
						V0Components: &[]xdr.TxSetComponent{{
							TxsMaybeDiscountedFee: &xdr.TxSetComponentTxsMaybeDiscountedFee{
								Txs: txEnvs,
							}},
						},
					}},
				},
			},
		},
	}
)

func TestTransactionReader(t *testing.T) {
	s := set.NewSet[xdr.Hash](5)
	for _, hash := range txHashes {
		s.Add(hash)
	}
	require.Lenf(t, s, len(txHashes), "precondition: hashes aren't unique, envs: %+v", txEnvs)

	// simplest case: read from start

	reader, err := NewLedgerTransactionReaderFromLedgerCloseMeta(passphrase, ledgerCloseMeta)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		tx, ierr := reader.Read()
		require.NoError(t, ierr)
		assert.EqualValues(t, i+1, tx.Index, "iteration i=%d", i)

		thisHash, ierr := network.HashTransactionInEnvelope(tx.Envelope, passphrase)
		require.NoError(t, ierr)
		assert.Equal(t, txEnvs[tx.Index-1], tx.Envelope)
		assert.Equal(t, txHashes[tx.Index-1], thisHash)
	}
	_, err = reader.Read()
	require.ErrorIs(t, err, io.EOF)

	// start reading from the middle set of txs

	require.NoError(t, reader.Seek(2))
	for i := 0; i < 3; i++ {
		tx, ierr := reader.Read()
		require.NoError(t, ierr)
		assert.EqualValues(t,
			/* txIndex is 1-based, iter is 0-based, start at 3rd tx, 5 total */
			1+(i+2)%5,
			tx.Index,
			"iteration i=%d", i)

		thisHash, ierr := network.HashTransactionInEnvelope(tx.Envelope, passphrase)
		require.NoError(t, ierr)
		assert.Equal(t, txEnvs[tx.Index-1], tx.Envelope)
		assert.Equal(t, txHashes[tx.Index-1], thisHash)
	}
	_, err = reader.Read()
	require.ErrorIs(t, err, io.EOF)

	// edge case: start from the last tx
	require.NoError(t, reader.Seek(4))
	tx, ierr := reader.Read()
	require.NoError(t, ierr)
	assert.EqualValues(t, 5, tx.Index)

	thisHash, ierr := network.HashTransactionInEnvelope(tx.Envelope, passphrase)
	require.NoError(t, ierr)
	assert.Equal(t, txEnvs[4], tx.Envelope)
	assert.Equal(t, txHashes[4], thisHash)
	_, err = reader.Read()
	require.ErrorIs(t, err, io.EOF)

	// error case: too far or too close
	for _, idx := range []int{-1, 5, 6} {
		rdr, err := NewLedgerTransactionReaderFromLedgerCloseMeta(passphrase, ledgerCloseMeta)
		require.NoError(t, err)
		require.Error(t, rdr.Seek(idx), "no error when trying seek=%d", idx)
	}
}

func makeTransactions(count int) (
	envs []xdr.TransactionEnvelope,
	hashes [][32]byte,
	metas []xdr.TransactionResultMeta,
) {
	seqNum := 123_456
	for i := 0; i < count; i++ {
		txEnv := xdr.TransactionEnvelope{
			Type: xdr.EnvelopeTypeEnvelopeTypeTx,
			V1: &xdr.TransactionV1Envelope{
				Tx: xdr.Transaction{
					Ext:           xdr.TransactionExt{V: 0},
					SourceAccount: xdr.MustMuxedAddress(keypair.MustRandom().Address()),
					Operations:    []xdr.Operation{},
					Fee:           xdr.Uint32(seqNum + i),
					SeqNum:        xdr.SequenceNumber(seqNum + i),
				},
				Signatures: []xdr.DecoratedSignature{},
			},
		}

		txHash, _ := network.HashTransactionInEnvelope(txEnv, passphrase)
		txMeta := xdr.TransactionResultMeta{
			Result:            xdr.TransactionResultPair{TransactionHash: xdr.Hash(txHash)},
			TxApplyProcessing: xdr.TransactionMeta{V: 3, V3: &xdr.TransactionMetaV3{}},
		}

		envs = append(envs, txEnv)
		hashes = append(hashes, txHash)
		metas = append(metas, txMeta)
	}

	return
}
