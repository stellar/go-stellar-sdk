// Package network contains functions that deal with stellar network passphrases
// and IDs.
package network

import (
	"bytes"
	"strings"

	"github.com/stellar/go-stellar-sdk/hash"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const (
	// PublicNetworkPassphrase is the pass phrase used for every transaction intended for the public stellar network
	PublicNetworkPassphrase = "Public Global Stellar Network ; September 2015"
	// TestNetworkPassphrase is the pass phrase used for every transaction intended for the SDF-run test network
	TestNetworkPassphrase = "Test SDF Network ; September 2015"
	// FutureNetworkPassphrase is the pass phrase used for every transaction intended for the SDF-run future network
	FutureNetworkPassphrase = "Test SDF Future Network ; October 2022"
)

var (
	// PublicNetworkhistoryArchiveURLs is a list of history archive URLs for stellar 'pubnet'
	PublicNetworkhistoryArchiveURLs = []string{
		"https://history.stellar.org/prd/core-live/core_live_001/",
		"https://history.stellar.org/prd/core-live/core_live_002/",
		"https://history.stellar.org/prd/core-live/core_live_003/",
	}

	// TestNetworkhistoryArchiveURLs is a list of history archive URLs for stellar 'testnet'
	TestNetworkhistoryArchiveURLs = []string{
		"https://history.stellar.org/prd/core-testnet/core_testnet_001/",
		"https://history.stellar.org/prd/core-testnet/core_testnet_002/",
		"https://history.stellar.org/prd/core-testnet/core_testnet_003",
	}

	// FutureNetworkhistoryArchiveURLs is a list of history archive URLs for stellar 'futurenet'
	FutureNetworkhistoryArchiveURLs = []string{
		"http://history.stellar.org/dev/core-futurenet/core_futurenet_001/",
		"http://history.stellar.org/dev/core-futurenet/core_futurenet_002/",
		"http://history.stellar.org/dev/core-futurenet/core_futurenet_003/",
	}
)

// ID returns the network ID derived from the provided passphrase.  This value
// also happens to be the raw (i.e. not strkey encoded) secret key for the root
// account of the network.
func ID(passphrase string) [32]byte {
	return hash.Hash([]byte(passphrase))
}

// HashTransactionInEnvelope derives the network specific hash for the transaction
// contained in the provided envelope using the network identified by the supplied passphrase.
// The resulting hash is the value that can be signed by stellar secret key to
// authorize the transaction identified by the hash to stellar validators.
func HashTransactionInEnvelope(envelope xdr.TransactionEnvelope, passphrase string) ([32]byte, error) {
	var hash [32]byte
	var err error
	switch envelope.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		hash, err = HashTransaction(envelope.V1.Tx, passphrase)
	case xdr.EnvelopeTypeEnvelopeTypeTxV0:
		hash, err = HashTransactionV0(envelope.V0.Tx, passphrase)
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		hash, err = HashFeeBumpTransaction(envelope.FeeBump.Tx, passphrase)
	default:
		err = errors.New("invalid transaction type")
	}
	return hash, err
}

// HashTransaction derives the network specific hash for the provided
// transaction using the network identified by the supplied passphrase.  The
// resulting hash is the value that can be signed by stellar secret key to
// authorize the transaction identified by the hash to stellar validators.
func HashTransaction(tx xdr.Transaction, passphrase string) ([32]byte, error) {
	taggedTx := xdr.TransactionSignaturePayloadTaggedTransaction{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		Tx:   &tx,
	}
	return hashTx(taggedTx, passphrase)
}

// HashFeeBumpTransaction derives the network specific hash for the provided
// fee bump transaction using the network identified by the supplied passphrase.  The
// resulting hash is the value that can be signed by stellar secret key to
// authorize the transaction identified by the hash to stellar validators.
func HashFeeBumpTransaction(tx xdr.FeeBumpTransaction, passphrase string) ([32]byte, error) {
	taggedTx := xdr.TransactionSignaturePayloadTaggedTransaction{
		Type:    xdr.EnvelopeTypeEnvelopeTypeTxFeeBump,
		FeeBump: &tx,
	}
	return hashTx(taggedTx, passphrase)
}

// HashTransactionV0 derives the network specific hash for the provided
// legacy transaction using the network identified by the supplied passphrase.  The
// resulting hash is the value that can be signed by stellar secret key to
// authorize the transaction identified by the hash to stellar validators.
func HashTransactionV0(tx xdr.TransactionV0, passphrase string) ([32]byte, error) {
	sa, err := xdr.NewMuxedAccount(xdr.CryptoKeyTypeKeyTypeEd25519, tx.SourceAccountEd25519)
	if err != nil {
		return [32]byte{}, err
	}

	v1Tx := xdr.Transaction{
		SourceAccount: sa,
		Fee:           tx.Fee,
		Memo:          tx.Memo,
		Operations:    tx.Operations,
		SeqNum:        tx.SeqNum,
		Cond:          xdr.NewPreconditionsWithTimeBounds(tx.TimeBounds),
	}
	return HashTransaction(v1Tx, passphrase)
}

func hashTx(
	tx xdr.TransactionSignaturePayloadTaggedTransaction,
	passphrase string,
) ([32]byte, error) {
	if strings.TrimSpace(passphrase) == "" {
		return [32]byte{}, errors.New("empty network passphrase")
	}

	var txBytes bytes.Buffer
	payload := xdr.TransactionSignaturePayload{
		NetworkId:         ID(passphrase),
		TaggedTransaction: tx,
	}

	_, err := xdr.Marshal(&txBytes, payload)
	if err != nil {
		return [32]byte{}, errors.Wrap(err, "marshal tx failed")
	}

	return hash.Hash(txBytes.Bytes()), nil
}
