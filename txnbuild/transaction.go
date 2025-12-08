/*
Package txnbuild implements transactions and operations on the Stellar network.
This library provides an interface to the Stellar transaction model. It supports the building of Go applications on
top of the Stellar network (https://www.stellar.org/). Transactions constructed by this library may be submitted
to any Horizon instance for processing onto the ledger, using any Stellar SDK client. The recommended client for Go
programmers is horizonclient (https://github.com/stellar/go-stellar-sdk/tree/master/clients/horizonclient). Together, these two
libraries provide a complete Stellar SDK.
For more information and further examples, see https://github.com/stellar/go-stellar-sdk/blob/master/docs/reference/readme.md
*/
package txnbuild

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"math/bits"
	"strings"
	"time"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/support/collections/set"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// MinBaseFee is the minimum transaction fee for the Stellar network of 100 stroops (0.00001 XLM).
const MinBaseFee = 100

// Account represents the aspects of a Stellar account necessary to construct transactions. See
// https://developers.stellar.org/docs/glossary/accounts/
type Account interface {
	GetAccountID() string
	IncrementSequenceNumber() (int64, error)
	GetSequenceNumber() (int64, error)
}

func hashHex(e xdr.TransactionEnvelope, networkStr string) (string, error) {
	h, err := network.HashTransactionInEnvelope(e, networkStr)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h[:]), nil
}

func concatSignatures(
	e xdr.TransactionEnvelope,
	networkStr string,
	signatures []xdr.DecoratedSignature,
	kps ...*keypair.Full,
) ([]xdr.DecoratedSignature, error) {
	// Hash the transaction
	h, err := network.HashTransactionInEnvelope(e, networkStr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to hash transaction")
	}

	extended := make(
		[]xdr.DecoratedSignature,
		len(signatures),
		len(signatures)+len(kps),
	)
	copy(extended, signatures)
	// Sign the hash
	for _, kp := range kps {
		sig, err := kp.SignDecorated(h[:])
		if err != nil {
			return nil, errors.Wrap(err, "failed to sign transaction")
		}
		extended = append(extended, sig)
	}
	return extended, nil
}

func concatSignatureDecorated(e xdr.TransactionEnvelope, signatures []xdr.DecoratedSignature, newSignatures []xdr.DecoratedSignature) ([]xdr.DecoratedSignature, error) {
	extended := make([]xdr.DecoratedSignature, len(signatures)+len(newSignatures))
	copy(extended, signatures)
	copy(extended[len(signatures):], newSignatures)
	return extended, nil
}

func concatSignatureBase64(e xdr.TransactionEnvelope, signatures []xdr.DecoratedSignature, networkStr, publicKey, signature string) ([]xdr.DecoratedSignature, error) {
	if signature == "" {
		return nil, errors.New("signature not presented")
	}

	kp, err := keypair.ParseAddress(publicKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse the public key %s", publicKey)
	}

	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to base64-decode the signature %s", signature)
	}

	h, err := network.HashTransactionInEnvelope(e, networkStr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to hash transaction")
	}

	err = kp.Verify(h[:], sigBytes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to verify the signature")
	}

	extended := make([]xdr.DecoratedSignature, len(signatures), len(signatures)+1)
	copy(extended, signatures)
	extended = append(extended, xdr.DecoratedSignature{
		Hint:      xdr.SignatureHint(kp.Hint()),
		Signature: xdr.Signature(sigBytes),
	})

	return extended, nil
}

func stringsToKP(keys ...string) ([]*keypair.Full, error) {
	var signers []*keypair.Full
	for _, k := range keys {
		kp, err := keypair.Parse(k)
		if err != nil {
			return nil, errors.Wrapf(err, "provided string %s is not a valid Stellar key", k)
		}
		kpf, ok := kp.(*keypair.Full)
		if !ok {
			return nil, errors.New("provided string %s is not a valid Stellar secret key")
		}
		signers = append(signers, kpf)
	}

	return signers, nil
}

func concatHashX(signatures []xdr.DecoratedSignature, preimage []byte) ([]xdr.DecoratedSignature, error) {
	if maxSize := xdr.Signature(preimage).XDRMaxSize(); len(preimage) > maxSize {
		return nil, errors.Errorf(
			"preimage cannot be more than %d bytes", maxSize,
		)
	}
	extended := make(
		[]xdr.DecoratedSignature,
		len(signatures),
		len(signatures)+1,
	)
	copy(extended, signatures)

	preimageHash := sha256.Sum256(preimage)
	var hint [4]byte
	// copy the last 4-bytes of the signer public key to be used as hint
	copy(hint[:], preimageHash[28:])

	sig := xdr.DecoratedSignature{
		Hint:      xdr.SignatureHint(hint),
		Signature: xdr.Signature(preimage),
	}
	return append(extended, sig), nil
}

func marshallBinary(e xdr.TransactionEnvelope, signatures []xdr.DecoratedSignature) ([]byte, error) {
	switch e.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		e.V1.Signatures = signatures
	case xdr.EnvelopeTypeEnvelopeTypeTxV0:
		e.V0.Signatures = signatures
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		e.FeeBump.Signatures = signatures
	default:
		panic("invalid transaction type: " + e.Type.String())
	}

	var txBytes bytes.Buffer
	_, err := xdr.Marshal(&txBytes, e)
	if err != nil {
		return nil, err
	}
	return txBytes.Bytes(), nil
}

func marshallBase64(e xdr.TransactionEnvelope, signatures []xdr.DecoratedSignature) (string, error) {
	binary, err := marshallBinary(e, signatures)
	if err != nil {
		return "", errors.Wrap(err, "failed to get XDR bytestring")
	}

	return base64.StdEncoding.EncodeToString(binary), nil
}

func marshallBase64Bytes(e xdr.TransactionEnvelope, signatures []xdr.DecoratedSignature) ([]byte, error) {
	binary, err := marshallBinary(e, signatures)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get XDR bytestring")
	}

	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(binary)))
	base64.StdEncoding.Encode(encoded, binary)
	return encoded, nil
}

// Transaction represents a Stellar transaction. See
// https://developers.stellar.org/docs/glossary/transactions/
// A Transaction may be wrapped by a FeeBumpTransaction in which case
// the account authorizing the FeeBumpTransaction will pay for the transaction fees
// instead of the Transaction's source account.
type Transaction struct {
	envelope      xdr.TransactionEnvelope
	baseFee       int64
	maxFee        int64
	sourceAccount SimpleAccount
	operations    []Operation
	memo          Memo
	preconditions Preconditions
}

// BaseFee returns the per operation fee for this transaction.
func (t *Transaction) BaseFee() int64 {
	return t.baseFee
}

// MaxFee returns the total fees which can be spent to submit this transaction.
func (t *Transaction) MaxFee() int64 {
	return t.maxFee
}

// SourceAccount returns the account which is originating this account.
func (t *Transaction) SourceAccount() SimpleAccount {
	return t.sourceAccount
}

// SequenceNumber returns the sequence number of the transaction.
func (t *Transaction) SequenceNumber() int64 {
	return t.sourceAccount.Sequence
}

// Memo returns the memo configured for this transaction.
func (t *Transaction) Memo() Memo {
	return t.memo
}

// Timebounds returns the Timebounds configured for this transaction.
func (t *Transaction) Timebounds() TimeBounds {
	return t.preconditions.TimeBounds
}

// Operations returns the list of operations included in this transaction.
// The contents of the returned slice should not be modified.
func (t *Transaction) Operations() []Operation {
	return t.operations
}

// Signatures returns the list of signatures attached to this transaction.
// The contents of the returned slice should not be modified.
func (t *Transaction) Signatures() []xdr.DecoratedSignature {
	return t.envelope.Signatures()
}

// Hash returns the network specific hash of this transaction
// encoded as a byte array.
func (t *Transaction) Hash(networkStr string) ([32]byte, error) {
	return network.HashTransactionInEnvelope(t.envelope, networkStr)
}

// HashHex returns the network specific hash of this transaction
// encoded as a hexadecimal string.
func (t *Transaction) HashHex(network string) (string, error) {
	return hashHex(t.envelope, network)
}

func (t *Transaction) clone(signatures []xdr.DecoratedSignature) *Transaction {
	newTx := new(Transaction)
	*newTx = *t
	newTx.envelope = t.envelope

	switch newTx.envelope.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		newTx.envelope.V1 = new(xdr.TransactionV1Envelope)
		*newTx.envelope.V1 = *t.envelope.V1
		newTx.envelope.V1.Signatures = signatures
	case xdr.EnvelopeTypeEnvelopeTypeTxV0:
		newTx.envelope.V0 = new(xdr.TransactionV0Envelope)
		*newTx.envelope.V0 = *t.envelope.V0
		newTx.envelope.V0.Signatures = signatures
	default:
		panic("invalid transaction type: " + newTx.envelope.Type.String())
	}

	return newTx
}

// Sign returns a new Transaction instance which extends the current instance
// with additional signatures derived from the given list of keypair instances.
func (t *Transaction) Sign(network string, kps ...*keypair.Full) (*Transaction, error) {
	extendedSignatures, err := concatSignatures(t.envelope, network, t.Signatures(), kps...)
	if err != nil {
		return nil, err
	}

	return t.clone(extendedSignatures), nil
}

// SignWithKeyString returns a new Transaction instance which extends the current instance
// with additional signatures derived from the given list of private key strings.
func (t *Transaction) SignWithKeyString(network string, keys ...string) (*Transaction, error) {
	kps, err := stringsToKP(keys...)
	if err != nil {
		return nil, err
	}
	return t.Sign(network, kps...)
}

// SignHashX returns a new Transaction instance which extends the current instance
// with HashX signature type.
// See description here: https://developers.stellar.org/docs/glossary/multisig/#hashx
func (t *Transaction) SignHashX(preimage []byte) (*Transaction, error) {
	extendedSignatures, err := concatHashX(t.Signatures(), preimage)
	if err != nil {
		return nil, err
	}

	return t.clone(extendedSignatures), nil
}

// ClearSignatures returns a new Transaction instance which extends the current instance
// with signatures removed.
func (t *Transaction) ClearSignatures() (*Transaction, error) {
	return t.clone(nil), nil
}

// AddSignatureDecorated returns a new Transaction instance which extends the current instance
// with an additional decorated signature(s).
func (t *Transaction) AddSignatureDecorated(signature ...xdr.DecoratedSignature) (*Transaction, error) {
	extendedSignatures, err := concatSignatureDecorated(t.envelope, t.Signatures(), signature)
	if err != nil {
		return nil, err
	}

	return t.clone(extendedSignatures), nil
}

// AddSignatureBase64 returns a new Transaction instance which extends the current instance
// with an additional signature derived from the given base64-encoded signature.
func (t *Transaction) AddSignatureBase64(network, publicKey, signature string) (*Transaction, error) {
	extendedSignatures, err := concatSignatureBase64(t.envelope, t.Signatures(), network, publicKey, signature)
	if err != nil {
		return nil, err
	}

	return t.clone(extendedSignatures), nil
}

// ToXDR returns the a xdr.TransactionEnvelope which is equivalent to this transaction.
// The envelope should not be modified because any changes applied may
// affect the internals of the Transaction instance.
func (t *Transaction) ToXDR() xdr.TransactionEnvelope {
	return t.envelope
}

// MarshalBinary returns the binary XDR representation of the transaction envelope.
func (t *Transaction) MarshalBinary() ([]byte, error) {
	return marshallBinary(t.envelope, t.Signatures())
}

// MarshalText returns the base64 XDR representation of the transaction envelope.
func (t *Transaction) MarshalText() ([]byte, error) {
	return marshallBase64Bytes(t.envelope, t.Signatures())
}

// UnmarshalText consumes into the value the base64 XDR representation of the
// transaction envelope.
func (t *Transaction) UnmarshalText(b []byte) error {
	gtx, err := TransactionFromXDR(string(b))
	if err != nil {
		return err
	}
	tx, ok := gtx.Transaction()
	if !ok {
		return errors.New("transaction envelope unmarshaled into FeeBumpTransaction is not a fee bump transaction")
	}
	*t = *tx
	return nil
}

// Base64 returns the base 64 XDR representation of the transaction envelope.
func (t *Transaction) Base64() (string, error) {
	return marshallBase64(t.envelope, t.Signatures())
}

// ToGenericTransaction creates a GenericTransaction containing the Transaction.
func (t *Transaction) ToGenericTransaction() *GenericTransaction {
	return &GenericTransaction{simple: t}
}

// ClaimableBalanceID returns the claimable balance ID for the operation at the given index within the transaction.
// given index (which should be a `CreateClaimableBalance` operation).
func (t *Transaction) ClaimableBalanceID(operationIndex int) (string, error) {
	if operationIndex < 0 || operationIndex >= len(t.operations) {
		return "", errors.New("invalid operation index")
	}

	if _, ok := t.operations[operationIndex].(*CreateClaimableBalance); !ok {
		return "", errors.New("operation is not CreateClaimableBalance")
	}

	// We mimic the relevant code from Stellar Core
	// https://github.com/stellar/stellar-core/blob/9f3cc04e6ec02c38974c42545a86cdc79809252b/src/test/TestAccount.cpp#L285
	//
	// Note that the source account must be *unmuxed* for this to work.
	muxedAccountId := xdr.MustMuxedAddress(t.sourceAccount.AccountID).ToAccountId()
	operationId := xdr.HashIdPreimage{
		Type: xdr.EnvelopeTypeEnvelopeTypeOpId,
		OperationId: &xdr.HashIdPreimageOperationId{
			SourceAccount: muxedAccountId,
			SeqNum:        xdr.SequenceNumber(t.sourceAccount.Sequence),
			OpNum:         xdr.Uint32(operationIndex),
		},
	}

	binaryDump, err := operationId.MarshalBinary()
	if err != nil {
		return "", errors.Wrap(err, "invalid claimable balance operation")
	}

	hash := sha256.Sum256(binaryDump)
	balanceIdXdr, err := xdr.NewClaimableBalanceId(
		// TODO: look into whether this be determined programmatically from the operation structure.
		xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
		xdr.Hash(hash))
	if err != nil {
		return "", errors.Wrap(err, "unable to parse balance ID as XDR")
	}

	balanceIdHex, err := xdr.MarshalHex(balanceIdXdr)
	if err != nil {
		return "", errors.Wrap(err, "unable to encode balance ID as hex")
	}

	return balanceIdHex, nil
}

// FeeBumpTransaction represents a CAP 15 fee bump transaction.
// Fee bump transactions allow an arbitrary account to pay the fee for a transaction.
type FeeBumpTransaction struct {
	envelope   xdr.TransactionEnvelope
	baseFee    int64
	maxFee     int64
	feeAccount string
	inner      *Transaction
}

// BaseFee returns the per operation fee for this transaction.
func (t *FeeBumpTransaction) BaseFee() int64 {
	return t.baseFee
}

// MaxFee returns the total fees which can be spent to submit this transaction.
func (t *FeeBumpTransaction) MaxFee() int64 {
	return t.maxFee
}

// FeeAccount returns the address of the account which will be paying for the inner transaction.
func (t *FeeBumpTransaction) FeeAccount() string {
	return t.feeAccount
}

// Signatures returns the list of signatures attached to this transaction.
// The contents of the returned slice should not be modified.
func (t *FeeBumpTransaction) Signatures() []xdr.DecoratedSignature {
	return t.envelope.FeeBumpSignatures()
}

// Hash returns the network specific hash of this transaction
// encoded as a byte array.
func (t *FeeBumpTransaction) Hash(networkStr string) ([32]byte, error) {
	return network.HashTransactionInEnvelope(t.envelope, networkStr)
}

// HashHex returns the network specific hash of this transaction
// encoded as a hexadecimal string.
func (t *FeeBumpTransaction) HashHex(network string) (string, error) {
	return hashHex(t.envelope, network)
}

func (t *FeeBumpTransaction) clone(signatures []xdr.DecoratedSignature) *FeeBumpTransaction {
	newTx := new(FeeBumpTransaction)
	*newTx = *t
	newTx.envelope.FeeBump = new(xdr.FeeBumpTransactionEnvelope)
	*newTx.envelope.FeeBump = *t.envelope.FeeBump
	newTx.envelope.FeeBump.Signatures = signatures
	return newTx
}

// Sign returns a new FeeBumpTransaction instance which extends the current instance
// with additional signatures derived from the given list of keypair instances.
func (t *FeeBumpTransaction) Sign(network string, kps ...*keypair.Full) (*FeeBumpTransaction, error) {
	extendedSignatures, err := concatSignatures(t.envelope, network, t.Signatures(), kps...)
	if err != nil {
		return nil, err
	}

	return t.clone(extendedSignatures), nil
}

// SignWithKeyString returns a new FeeBumpTransaction instance which extends the current instance
// with additional signatures derived from the given list of private key strings.
func (t *FeeBumpTransaction) SignWithKeyString(network string, keys ...string) (*FeeBumpTransaction, error) {
	kps, err := stringsToKP(keys...)
	if err != nil {
		return nil, err
	}
	return t.Sign(network, kps...)
}

// SignHashX returns a new FeeBumpTransaction instance which extends the current instance
// with HashX signature type.
// See description here: https://developers.stellar.org/docs/glossary/multisig/#hashx
func (t *FeeBumpTransaction) SignHashX(preimage []byte) (*FeeBumpTransaction, error) {
	extendedSignatures, err := concatHashX(t.Signatures(), preimage)
	if err != nil {
		return nil, err
	}

	return t.clone(extendedSignatures), nil
}

// ClearSignatures returns a new Transaction instance which extends the current instance
// with signatures removed.
func (t *FeeBumpTransaction) ClearSignatures() (*FeeBumpTransaction, error) {
	return t.clone(nil), nil
}

// AddSignatureDecorated returns a new FeeBumpTransaction instance which extends the current instance
// with an additional decorated signature(s).
func (t *FeeBumpTransaction) AddSignatureDecorated(signature ...xdr.DecoratedSignature) (*FeeBumpTransaction, error) {
	extendedSignatures, err := concatSignatureDecorated(t.envelope, t.Signatures(), signature)
	if err != nil {
		return nil, err
	}

	return t.clone(extendedSignatures), nil
}

// AddSignatureBase64 returns a new FeeBumpTransaction instance which extends the current instance
// with an additional signature derived from the given base64-encoded signature.
func (t *FeeBumpTransaction) AddSignatureBase64(network, publicKey, signature string) (*FeeBumpTransaction, error) {
	extendedSignatures, err := concatSignatureBase64(t.envelope, t.Signatures(), network, publicKey, signature)
	if err != nil {
		return nil, err
	}

	return t.clone(extendedSignatures), nil
}

// ToXDR returns the a xdr.TransactionEnvelope which is equivalent to this transaction.
// The envelope should not be modified because any changes applied may
// affect the internals of the FeeBumpTransaction instance.
func (t *FeeBumpTransaction) ToXDR() xdr.TransactionEnvelope {
	return t.envelope
}

// MarshalBinary returns the binary XDR representation of the transaction envelope.
func (t *FeeBumpTransaction) MarshalBinary() ([]byte, error) {
	return marshallBinary(t.envelope, t.Signatures())
}

// MarshalText returns the base64 XDR representation of the transaction
// envelope.
func (t *FeeBumpTransaction) MarshalText() ([]byte, error) {
	return marshallBase64Bytes(t.envelope, t.Signatures())
}

// UnmarshalText consumes into the value the base64 XDR representation of the
// transaction envelope.
func (t *FeeBumpTransaction) UnmarshalText(b []byte) error {
	gtx, err := TransactionFromXDR(string(b))
	if err != nil {
		return err
	}
	fbtx, ok := gtx.FeeBump()
	if !ok {
		return errors.New("transaction envelope unmarshaled into Transaction is not a transaction")
	}
	*t = *fbtx
	return nil
}

// Base64 returns the base 64 XDR representation of the transaction envelope.
func (t *FeeBumpTransaction) Base64() (string, error) {
	return marshallBase64(t.envelope, t.Signatures())
}

// ToGenericTransaction creates a GenericTransaction containing the
// FeeBumpTransaction.
func (t *FeeBumpTransaction) ToGenericTransaction() *GenericTransaction {
	return &GenericTransaction{feeBump: t}
}

// InnerTransaction returns the Transaction which is wrapped by
// this FeeBumpTransaction instance.
func (t *FeeBumpTransaction) InnerTransaction() *Transaction {
	innerCopy := new(Transaction)
	*innerCopy = *t.inner
	return innerCopy
}

// GenericTransaction represents a parsed transaction envelope returned by TransactionFromXDR.
// A GenericTransaction can be either a Transaction or a FeeBumpTransaction.
type GenericTransaction struct {
	simple  *Transaction
	feeBump *FeeBumpTransaction
}

// NewGenericTransactionWithTransaction creates a GenericTransaction containing
// a Transaction.
func NewGenericTransactionWithTransaction(tx *Transaction) *GenericTransaction {
	return &GenericTransaction{simple: tx}
}

// NewGenericTransactionWithFeeBumpTransaction creates a GenericTransaction
// containing a FeeBumpTransaction.
func NewGenericTransactionWithFeeBumpTransaction(feeBumpTx *FeeBumpTransaction) *GenericTransaction {
	return &GenericTransaction{feeBump: feeBumpTx}
}

// Transaction unpacks the GenericTransaction instance into a Transaction.
// The function also returns a boolean which is true if the GenericTransaction can be
// unpacked into a Transaction.
func (t GenericTransaction) Transaction() (*Transaction, bool) {
	return t.simple, t.simple != nil
}

// FeeBump unpacks the GenericTransaction instance into a FeeBumpTransaction.
// The function also returns a boolean which is true if the GenericTransaction
// can be unpacked into a FeeBumpTransaction.
func (t GenericTransaction) FeeBump() (*FeeBumpTransaction, bool) {
	return t.feeBump, t.feeBump != nil
}

// ToXDR returns the a xdr.TransactionEnvelope which is equivalent to this
// transaction. The envelope should not be modified because any changes applied
// may affect the internals of the GenericTransaction.
func (t *GenericTransaction) ToXDR() (xdr.TransactionEnvelope, error) {
	if tx, ok := t.Transaction(); ok {
		return tx.envelope, nil
	}
	if fbtx, ok := t.FeeBump(); ok {
		return fbtx.envelope, nil
	}
	return xdr.TransactionEnvelope{}, fmt.Errorf("unable to get xdr of empty GenericTransaction")
}

// Hash returns the network specific hash of this transaction
// encoded as a byte array.
func (t GenericTransaction) Hash(networkStr string) ([32]byte, error) {
	if tx, ok := t.Transaction(); ok {
		return tx.Hash(networkStr)
	}
	if fbtx, ok := t.FeeBump(); ok {
		return fbtx.Hash(networkStr)
	}
	return [32]byte{}, fmt.Errorf("unable to get hash of empty GenericTransaction")
}

// HashHex returns the network specific hash of this transaction
// encoded as a hexadecimal string.
func (t GenericTransaction) HashHex(network string) (string, error) {
	if tx, ok := t.Transaction(); ok {
		return tx.HashHex(network)
	}
	if fbtx, ok := t.FeeBump(); ok {
		return fbtx.HashHex(network)
	}
	return "", fmt.Errorf("unable to get hash of empty GenericTransaction")
}

// MarshalBinary returns the binary XDR representation of the transaction
// envelope.
func (t *GenericTransaction) MarshalBinary() ([]byte, error) {
	if tx, ok := t.Transaction(); ok {
		return tx.MarshalBinary()
	}
	if fbtx, ok := t.FeeBump(); ok {
		return fbtx.MarshalBinary()
	}
	return nil, errors.New("unable to marshal empty GenericTransaction")
}

// MarshalText returns the base64 XDR representation of the transaction
// envelope.
func (t *GenericTransaction) MarshalText() ([]byte, error) {
	if tx, ok := t.Transaction(); ok {
		return tx.MarshalText()
	}
	if fbtx, ok := t.FeeBump(); ok {
		return fbtx.MarshalText()
	}
	return nil, errors.New("unable to marshal empty GenericTransaction")
}

// UnmarshalText consumes into the value the base64 XDR representation of the
// transaction envelope.
func (t *GenericTransaction) UnmarshalText(b []byte) error {
	gtx, err := TransactionFromXDR(string(b))
	if err != nil {
		return err
	}
	*t = *gtx
	return nil
}

// TransactionFromXDR parses the supplied transaction envelope in base64 XDR
// and returns a GenericTransaction instance.
func TransactionFromXDR(txeB64 string) (*GenericTransaction, error) {
	var xdrEnv xdr.TransactionEnvelope
	err := xdr.SafeUnmarshalBase64(txeB64, &xdrEnv)
	if err != nil {
		return nil, errors.Wrap(err, "unable to unmarshal transaction envelope")
	}
	return transactionFromParsedXDR(xdrEnv)
}

func transactionFromParsedXDR(xdrEnv xdr.TransactionEnvelope) (*GenericTransaction, error) {
	var err error
	newTx := &GenericTransaction{}

	if xdrEnv.IsFeeBump() {
		var innerTx *GenericTransaction
		innerTx, err = transactionFromParsedXDR(xdr.TransactionEnvelope{
			Type: xdr.EnvelopeTypeEnvelopeTypeTx,
			V1:   xdrEnv.FeeBump.Tx.InnerTx.V1,
		})
		if err != nil {
			return newTx, errors.New("could not parse inner transaction")
		}
		feeBumpAccount := xdrEnv.FeeBumpAccount()
		feeAccount := feeBumpAccount.Address()

		newTx.feeBump = &FeeBumpTransaction{
			envelope: xdrEnv,
			// A fee-bump transaction has an effective number of operations equal to one plus the
			// number of operations in the inner transaction. Correspondingly, the minimum fee for
			// the fee-bump transaction is one base fee more than the minimum fee for the inner
			// transaction.
			baseFee:    xdrEnv.FeeBumpFee() / int64(len(innerTx.simple.operations)+1),
			maxFee:     xdrEnv.FeeBumpFee(),
			inner:      innerTx.simple,
			feeAccount: feeAccount,
		}

		return newTx, nil
	}
	sourceAccount := xdrEnv.SourceAccount()
	accountID := sourceAccount.Address()

	totalFee := int64(xdrEnv.Fee())
	baseFee := totalFee
	if count := int64(len(xdrEnv.Operations())); count > 0 {
		baseFee = baseFee / count
	}

	newTx.simple = &Transaction{
		envelope: xdrEnv,
		baseFee:  baseFee,
		maxFee:   totalFee,
		sourceAccount: SimpleAccount{
			AccountID: accountID,
			Sequence:  xdrEnv.SeqNum(),
		},
		operations: nil,
		memo:       nil,
	}

	newTx.simple.preconditions.FromXDR(xdrEnv.Preconditions())
	newTx.simple.memo, err = memoFromXDR(xdrEnv.Memo())
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse memo")
	}

	operations := xdrEnv.Operations()
	for _, op := range operations {
		newOp, err := operationFromXDR(op)
		if err != nil {
			return nil, err
		}
		// if it's a soroban transaction, and we found a InvokeHostFunction operation.
		if xdrEnv.V1 != nil && xdrEnv.V1.Tx.Ext.V != 0 {
			if invoke, ok := newOp.(*InvokeHostFunction); ok {
				invoke.Ext = xdrEnv.V1.Tx.Ext
			}
		}
		newTx.simple.operations = append(newTx.simple.operations, newOp)
	}

	return newTx, nil
}

// TransactionParams is a container for parameters which are used to construct
// new Transaction instances
type TransactionParams struct {
	SourceAccount        Account
	IncrementSequenceNum bool
	Operations           []Operation
	BaseFee              int64
	Memo                 Memo
	Preconditions        Preconditions
}

// NewTransaction returns a new Transaction instance
func NewTransaction(params TransactionParams) (*Transaction, error) {
	if params.SourceAccount == nil {
		return nil, errors.New("transaction has no source account")
	}

	var sequence int64
	var err error
	if params.IncrementSequenceNum {
		sequence, err = params.SourceAccount.IncrementSequenceNumber()
	} else {
		sequence, err = params.SourceAccount.GetSequenceNumber()
	}
	if err != nil {
		return nil, errors.Wrap(err, "could not obtain account sequence")
	}

	tx := &Transaction{
		baseFee: params.BaseFee,
		sourceAccount: SimpleAccount{
			AccountID: params.SourceAccount.GetAccountID(),
			Sequence:  sequence,
		},
		operations:    params.Operations,
		memo:          params.Memo,
		preconditions: params.Preconditions,
	}
	var sourceAccount xdr.MuxedAccount
	if err = sourceAccount.SetAddress(tx.sourceAccount.AccountID); err != nil {
		return nil, errors.Wrap(err, "account id is not valid")
	}
	if tx.baseFee < 0 {
		return nil, errors.Errorf("base fee cannot be negative")
	}

	if len(tx.operations) == 0 {
		return nil, errors.New("transaction has no operations")
	}

	// Check that all preconditions are valid
	if err = tx.preconditions.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid preconditions")
	}

	precondXdr, err := tx.preconditions.BuildXDR()
	if err != nil {
		return nil, errors.Wrap(err, "invalid preconditions")
	}

	envelope := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{
				SourceAccount: sourceAccount,
				SeqNum:        xdr.SequenceNumber(sequence),
				Cond:          precondXdr,
			},
			Signatures: nil,
		},
	}

	// Handle the memo, if one is present
	if tx.memo != nil {
		var xdrMemo xdr.Memo
		xdrMemo, err = tx.memo.ToXDR()
		if err != nil {
			return nil, errors.Wrap(err, "couldn't build memo XDR")
		}
		envelope.V1.Tx.Memo = xdrMemo
	}

	var sorobanOp SorobanOperation

	for _, op := range tx.operations {
		if verr := op.Validate(); verr != nil {
			return nil, errors.Wrap(verr, fmt.Sprintf("validation failed for %T operation", op))
		}
		xdrOperation, err2 := op.BuildXDR()
		if err2 != nil {
			return nil, errors.Wrap(err2, fmt.Sprintf("failed to build operation %T", op))
		}
		envelope.V1.Tx.Operations = append(envelope.V1.Tx.Operations, xdrOperation)

		if scOp, ok := op.(SorobanOperation); ok {
			// this is a smart contract operation.
			// smart contract operations are limited to 1 operation / transaction.
			sorobanOp = scOp
		}
	}

	// In case it's a smart contract transaction, we need to include the Ext field within the envelope.
	var sorobanFee uint64
	if sorobanOp != nil {
		envelope.V1.Tx.Ext, err = sorobanOp.BuildTransactionExt()
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to build operation %T", sorobanOp))
		}
		if envelope.V1.Tx.Ext.SorobanData != nil {
			sorobanFee = uint64(envelope.V1.Tx.Ext.SorobanData.ResourceFee)
		}
	}

	// check if maxFee fits in a uint32
	// 64 bit fees are only available in fee bump transactions
	// if maxFee is negative then there must have been an int overflow
	hi, lo := bits.Mul64(uint64(params.BaseFee), uint64(len(params.Operations)))
	if hi > 0 || lo > math.MaxUint32 {
		return nil, errors.Errorf(
			"base fee %d results in an overflow of max fee", params.BaseFee)
	}
	totalFee := lo + sorobanFee
	if totalFee < lo || totalFee > math.MaxUint32 {
		return nil, fmt.Errorf("soroban fee %v results in an overflow of max fee", sorobanFee)
	}
	tx.maxFee = int64(totalFee)
	envelope.V1.Tx.Fee = xdr.Uint32(totalFee)

	tx.envelope = envelope
	return tx, nil
}

// FeeBumpTransactionParams is a container for parameters
// which are used to construct new FeeBumpTransaction instances
type FeeBumpTransactionParams struct {
	Inner      *Transaction
	FeeAccount string
	BaseFee    int64
}

func convertToV1(tx *Transaction) (*Transaction, error) {
	sourceAccount := tx.SourceAccount()
	signatures := tx.Signatures()
	tx, err := NewTransaction(TransactionParams{
		SourceAccount:        &sourceAccount,
		IncrementSequenceNum: false,
		Operations:           tx.Operations(),
		BaseFee:              tx.BaseFee(),
		Memo:                 tx.Memo(),
		Preconditions:        Preconditions{TimeBounds: tx.Timebounds()},
	})
	if err != nil {
		return tx, err
	}
	tx.envelope.V1.Signatures = signatures
	return tx, nil
}

// NewFeeBumpTransaction returns a new FeeBumpTransaction instance
func NewFeeBumpTransaction(params FeeBumpTransactionParams) (*FeeBumpTransaction, error) {
	inner := params.Inner
	if inner == nil {
		return nil, errors.New("inner transaction is missing")
	}
	switch inner.envelope.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTx, xdr.EnvelopeTypeEnvelopeTypeTxV0:
	default:
		return nil, errors.Errorf("%s transactions cannot be fee bumped", inner.envelope.Type)
	}

	innerEnv := inner.ToXDR()
	if innerEnv.Type == xdr.EnvelopeTypeEnvelopeTypeTxV0 {
		var err error
		inner, err = convertToV1(inner)
		if err != nil {
			return nil, errors.Wrap(err, "could not upgrade transaction from v0 to v1")
		}
	} else if innerEnv.Type != xdr.EnvelopeTypeEnvelopeTypeTx {
		return nil, errors.Errorf("%v transactions cannot be fee bumped", innerEnv.Type.String())
	}

	tx := &FeeBumpTransaction{
		baseFee: params.BaseFee,
		// A fee-bump transaction has an effective number of operations equal to one plus the
		// number of operations in the inner transaction. Correspondingly, the minimum fee for
		// the fee-bump transaction is one base fee more than the minimum fee for the inner
		// transaction.
		feeAccount: params.FeeAccount,
		inner:      new(Transaction),
	}
	*tx.inner = *inner

	if tx.baseFee < tx.inner.baseFee {
		return tx, errors.New("base fee cannot be lower than provided inner transaction fee")
	}
	if tx.baseFee < MinBaseFee {
		return tx, errors.Errorf(
			"base fee cannot be lower than network minimum of %d", MinBaseFee,
		)
	}

	var sorobanFee uint64
	if inner.envelope.V1 != nil && inner.envelope.V1.Tx.Ext.SorobanData != nil {
		sorobanFee = uint64(inner.envelope.V1.Tx.Ext.SorobanData.ResourceFee)
	}

	hi, lo := bits.Mul64(uint64(params.BaseFee), uint64(len(inner.operations)+1))
	if hi > 0 || lo > math.MaxInt64 {
		return nil, errors.Errorf("base fee %d results in an overflow of max fee", params.BaseFee)
	}
	totalFee := lo + sorobanFee
	if totalFee < lo || totalFee > math.MaxInt64 {
		return nil, fmt.Errorf("soroban fee %v results in an overflow of max fee", sorobanFee)
	}
	tx.maxFee = int64(totalFee)

	var feeSource xdr.MuxedAccount
	if err := feeSource.SetAddress(tx.feeAccount); err != nil {
		return tx, errors.Wrap(err, "fee account is not a valid address")
	}

	tx.envelope = xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTxFeeBump,
		FeeBump: &xdr.FeeBumpTransactionEnvelope{
			Tx: xdr.FeeBumpTransaction{
				FeeSource: feeSource,
				Fee:       xdr.Int64(tx.maxFee),
				InnerTx: xdr.FeeBumpTransactionInnerTx{
					Type: xdr.EnvelopeTypeEnvelopeTypeTx,
					V1:   innerEnv.V1,
				},
			},
		},
	}

	return tx, nil
}

// BuildChallengeTx is a factory method that creates a valid SEP 10 challenge, for use in web authentication.
// "timebound" is the time duration the transaction should be valid for, and must be greater than 1s (300s is recommended).
// Muxed accounts or ID memos can be provided to identity a user of a shared Stellar account.
// More details on SEP 10: https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0010.md
func BuildChallengeTx(serverSignerSecret, clientAccountID, webAuthDomain, homeDomain, network string, timebound time.Duration, memo *MemoID) (*Transaction, error) {
	if timebound < time.Second {
		return nil, errors.New("provided timebound must be at least 1s (300s is recommended)")
	}

	serverKP, err := keypair.Parse(serverSignerSecret)
	if err != nil {
		return nil, err
	}

	// SEP10 spec requires 48 byte cryptographic-quality random string
	randomNonce, err := generateRandomNonce(48)
	if err != nil {
		return nil, err
	}
	// Encode 48-byte nonce to base64 for a total of 64-bytes
	randomNonceToString := base64.StdEncoding.EncodeToString(randomNonce)
	if len(randomNonceToString) != 64 {
		return nil, errors.New("64 byte long random nonce required")
	}

	if _, err = xdr.AddressToAccountId(clientAccountID); err != nil {
		if _, err = xdr.AddressToMuxedAccount(clientAccountID); err != nil {
			return nil, errors.Wrapf(err, "%s is not a valid account id or muxed account", clientAccountID)
		} else if memo != nil {
			return nil, errors.New("memos are not valid for challenge transactions with a muxed client account")
		}
	}

	// represent server signing account as SimpleAccount
	sa := SimpleAccount{
		AccountID: serverKP.Address(),
		Sequence:  0,
	}

	currentTime := time.Now().UTC()
	maxTime := currentTime.Add(timebound)

	// Create a SEP 10 compatible response. See
	// https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0010.md#response
	txParams := TransactionParams{
		SourceAccount:        &sa,
		IncrementSequenceNum: false,
		Operations: []Operation{
			&ManageData{
				SourceAccount: clientAccountID,
				Name:          homeDomain + " auth",
				Value:         []byte(randomNonceToString),
			},
			&ManageData{
				SourceAccount: serverKP.Address(),
				Name:          "web_auth_domain",
				Value:         []byte(webAuthDomain),
			},
		},
		BaseFee: MinBaseFee,
		Preconditions: Preconditions{
			TimeBounds: NewTimebounds(currentTime.Unix(), maxTime.Unix()),
		},
	}
	// Do not replace this if-then-assign block by assigning `memo` within the `TransactionParams`
	// struct above. Doing so will cause errors as described here: https://go.dev/doc/faq#nil_error
	if memo != nil {
		txParams.Memo = memo
	}
	tx, err := NewTransaction(txParams)
	if err != nil {
		return nil, err
	}
	tx, err = tx.Sign(network, serverKP.(*keypair.Full))
	if err != nil {
		return nil, err
	}

	return tx, nil
}

// generateRandomNonce creates a cryptographically secure random slice of `n` bytes.
func generateRandomNonce(n int) ([]byte, error) {
	binary := make([]byte, n)
	_, err := rand.Read(binary)

	if err != nil {
		return []byte{}, err
	}

	return binary, err
}

// ReadChallengeTx reads a SEP 10 challenge transaction and returns the decoded
// transaction and client account ID contained within.
//
// Before calling this function, retrieve the SIGNING_KEY included in the TOML file
// hosted on the service's homeDomain and ensure it matches the serverAccountID you
// intend to pass.
//
// This function verifies the serverAccountID signed the challenge. If the
// serverAccountID also matches the SIGNING_KEY included in the TOML file hosted the
// service's homeDomain passed, malicious web services will not be able to use the
// challenge transaction POSTed back to the authentication endpoint.
//
// The challenge's first Manage Data operation is expected to contain the homeDomain
// parameter passed to the function.
//
// If the challenge contains a subsequent Manage Data operation with key
// web_auth_domain the value will be checked to match the webAuthDomain
// provided. If it does not match the function will return an error.
//
// It does not verify that the transaction has been signed by the client or
// that any signatures other than the servers on the transaction are valid. Use
// one of the following functions to completely verify the transaction:
// - VerifyChallengeTxThreshold
// - VerifyChallengeTxSigners
//
// The returned clientAccountID may be a Stellar account (G...) or Muxed account (M...) address. If
// the address is muxed, or if the memo returned is non-nil, the challenge transaction
// is being used to authenticate a user of a shared Stellar account.
func ReadChallengeTx(challengeTx, serverAccountID, network, webAuthDomain string, homeDomains []string) (tx *Transaction, clientAccountID string, matchedHomeDomain string, memo *MemoID, err error) {
	parsed, err := TransactionFromXDR(challengeTx)
	if err != nil {
		return tx, clientAccountID, matchedHomeDomain, memo, errors.Wrap(err, "could not parse challenge")
	}

	var isSimple bool
	tx, isSimple = parsed.Transaction()
	if !isSimple {
		return tx, clientAccountID, matchedHomeDomain, memo, errors.New("challenge cannot be a fee bump transaction")
	}

	// Enforce no muxed accounts (at least until we understand their impact)
	if tx.envelope.SourceAccount().Type == xdr.CryptoKeyTypeKeyTypeMuxedEd25519 {
		err = errors.New("invalid source account: only valid Ed25519 accounts are allowed in challenge transactions")
		return tx, clientAccountID, matchedHomeDomain, memo, err
	}

	// verify transaction source
	if tx.SourceAccount().AccountID != serverAccountID {
		return tx, clientAccountID, matchedHomeDomain, memo, errors.New("transaction source account is not equal to server's account")
	}

	// verify sequence number
	if tx.SourceAccount().Sequence != 0 {
		return tx, clientAccountID, matchedHomeDomain, memo, errors.New("transaction sequence number must be 0")
	}

	// verify timebounds
	if tx.Timebounds().MaxTime == TimeoutInfinite {
		return tx, clientAccountID, matchedHomeDomain, memo, errors.New("transaction requires non-infinite timebounds")
	}
	// Apply a grace period to the challenge MinTime to account for clock drift between the server and client
	var gracePeriod int64 = 5 * 60 // seconds
	currentTime := time.Now().UTC().Unix()
	if currentTime+gracePeriod < tx.Timebounds().MinTime || currentTime > tx.Timebounds().MaxTime {
		return tx, clientAccountID, matchedHomeDomain, memo, errors.Errorf("transaction is not within range of the specified timebounds (currentTime=%d, MinTime=%d, MaxTime=%d)",
			currentTime, tx.Timebounds().MinTime, tx.Timebounds().MaxTime)
	}

	// verify operation
	operations := tx.Operations()
	if len(operations) < 1 {
		return tx, clientAccountID, matchedHomeDomain, memo, errors.New("transaction requires at least one manage_data operation")
	}
	op, ok := operations[0].(*ManageData)
	if !ok {
		return tx, clientAccountID, matchedHomeDomain, memo, errors.New("operation type should be manage_data")
	}
	if op.SourceAccount == "" {
		return tx, clientAccountID, matchedHomeDomain, memo, errors.New("operation should have a source account")
	}
	for _, homeDomain := range homeDomains {
		if op.Name == homeDomain+" auth" {
			matchedHomeDomain = homeDomain
			break
		}
	}
	if matchedHomeDomain == "" {
		return tx, clientAccountID, matchedHomeDomain, memo, errors.Errorf("operation key does not match any homeDomains passed (key=%q, homeDomains=%v)", op.Name, homeDomains)
	}

	clientAccountID = op.SourceAccount
	firstOpSourceAccountType := tx.envelope.Operations()[0].SourceAccount.Type
	if firstOpSourceAccountType != xdr.CryptoKeyTypeKeyTypeMuxedEd25519 && firstOpSourceAccountType != xdr.CryptoKeyTypeKeyTypeEd25519 {
		err = errors.New("invalid source account for first operation: only valid Ed25519 or muxed accounts are valid")
		return tx, clientAccountID, matchedHomeDomain, memo, err
	}
	if tx.Memo() != nil {
		if firstOpSourceAccountType == xdr.CryptoKeyTypeKeyTypeMuxedEd25519 {
			err = errors.New("memos are not valid for challenge transactions with a muxed client account")
			return tx, clientAccountID, matchedHomeDomain, memo, err
		}
		var txMemo MemoID
		if txMemo, ok = (tx.Memo()).(MemoID); !ok {
			err = errors.New("invalid memo, only ID memos are permitted")
			return tx, clientAccountID, matchedHomeDomain, memo, err
		}
		memo = &txMemo
	}

	// verify manage data value
	nonceB64 := string(op.Value)
	if len(nonceB64) != 64 {
		return tx, clientAccountID, matchedHomeDomain, memo, errors.New("random nonce encoded as base64 should be 64 bytes long")
	}
	nonceBytes, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil {
		return tx, clientAccountID, matchedHomeDomain, memo, errors.Wrap(err, "failed to decode random nonce provided in manage_data operation")
	}
	if len(nonceBytes) != 48 {
		return tx, clientAccountID, matchedHomeDomain, memo, errors.New("random nonce before encoding as base64 should be 48 bytes long")
	}

	// verify subsequent operations are manage data ops and known, or unknown with source account set to server account
	for _, op := range operations[1:] {
		op, ok := op.(*ManageData)
		if !ok {
			return tx, clientAccountID, matchedHomeDomain, memo, errors.New("operation type should be manage_data")
		}
		if op.SourceAccount == "" {
			return tx, clientAccountID, matchedHomeDomain, memo, errors.New("operation should have a source account")
		}
		switch op.Name {
		case "web_auth_domain":
			if op.SourceAccount != serverAccountID {
				return tx, clientAccountID, matchedHomeDomain, memo, errors.New("web auth domain operation must have server source account")
			}
			if !bytes.Equal(op.Value, []byte(webAuthDomain)) {
				return tx, clientAccountID, matchedHomeDomain, memo, errors.Errorf("web auth domain operation value is %q but expect %q", string(op.Value), webAuthDomain)
			}
		default:
			// verify unknown subsequent operations are manage data ops with source account set to server account
			if op.SourceAccount != serverAccountID {
				return tx, clientAccountID, matchedHomeDomain, memo, errors.New("subsequent operations are unrecognized")
			}
		}
	}

	err = verifyTxSignature(tx, network, serverAccountID)
	if err != nil {
		return tx, clientAccountID, matchedHomeDomain, memo, err
	}

	return tx, clientAccountID, matchedHomeDomain, memo, nil
}

// VerifyChallengeTxThreshold verifies that for a SEP 10 challenge transaction
// all signatures on the transaction are accounted for and that the signatures
// meet a threshold on an account. A transaction is verified if it is signed by
// the server account, and all other signatures match a signer that has been
// provided as an argument, and those signatures meet a threshold on the
// account.
//
// Signers that are not prefixed as an address/account ID strkey (G...) will be
// ignored.
//
// The homeDomain field is reserved for future use and not used.
//
// If the challenge contains a subsequent Manage Data operation with key
// web_auth_domain the value will be checked to match the webAuthDomain
// provided. If it does not match the function will return an error.
//
// Errors will be raised if:
//   - The transaction is invalid according to ReadChallengeTx.
//   - No client signatures are found on the transaction.
//   - One or more signatures in the transaction are not identifiable as the
//     server account or one of the signers provided in the arguments.
//   - The signatures are all valid but do not meet the threshold.
func VerifyChallengeTxThreshold(challengeTx, serverAccountID, network, webAuthDomain string, homeDomains []string, threshold Threshold, signerSummary SignerSummary) (signersFound []string, err error) {
	signers := make([]string, 0, len(signerSummary))
	for s := range signerSummary {
		signers = append(signers, s)
	}

	signersFound, err = VerifyChallengeTxSigners(challengeTx, serverAccountID, network, webAuthDomain, homeDomains, signers...)
	if err != nil {
		return nil, err
	}

	weight := int32(0)
	for _, s := range signersFound {
		weight += signerSummary[s]
	}

	if weight < int32(threshold) {
		return nil, errors.Errorf("signers with weight %d do not meet threshold %d", weight, threshold)
	}

	return signersFound, nil
}

// VerifyChallengeTxSigners verifies that for a SEP 10 challenge transaction
// all signatures on the transaction are accounted for. A transaction is
// verified if it is signed by the server account, and all other signatures
// match a signer that has been provided as an argument. Additional signers can
// be provided that do not have a signature, but all signatures must be matched
// to a signer for verification to succeed. If verification succeeds a list of
// signers that were found is returned, excluding the server account ID.
//
// Signers that are not prefixed as an address/account ID strkey (G...) will be
// ignored.
//
// The homeDomain field is reserved for future use and not used.
//
// If the challenge contains a subsequent Manage Data operation with key
// web_auth_domain the value will be checked to match the webAuthDomain
// provided. If it does not match the function will return an error.
//
// Errors will be raised if:
//   - The transaction is invalid according to ReadChallengeTx.
//   - No client signatures are found on the transaction.
//   - One or more signatures in the transaction are not identifiable as the
//     server account or one of the signers provided in the arguments.
func VerifyChallengeTxSigners(challengeTx, serverAccountID, network, webAuthDomain string, homeDomains []string, signers ...string) ([]string, error) {
	// Read the transaction which validates its structure.
	tx, _, _, _, err := ReadChallengeTx(challengeTx, serverAccountID, network, webAuthDomain, homeDomains)
	if err != nil {
		return nil, err
	}

	// Ensure the server account ID is an address and not a seed.
	serverKP, err := keypair.ParseAddress(serverAccountID)
	if err != nil {
		return nil, err
	}

	// Deduplicate the client signers and ensure the server is not included
	// anywhere we check or output the list of signers.
	clientSigners := []string{}
	clientSignersSeen := set.Set[string]{}
	for _, signer := range signers {
		// Ignore the server signer if it is in the signers list. It's
		// important when verifying signers of a challenge transaction that we
		// only verify and return client signers. If an account has the server
		// as a signer the server should not play a part in the authentication
		// of the client.
		if signer == serverKP.Address() {
			continue
		}
		// Deduplicate.
		if clientSignersSeen.Contains(signer) {
			continue
		}
		// Ignore non-G... account/address signers.
		strkeyVersionByte, strkeyErr := strkey.Version(signer)
		if strkeyErr != nil {
			continue
		}
		if strkeyVersionByte != strkey.VersionByteAccountID {
			continue
		}
		clientSigners = append(clientSigners, signer)
		clientSignersSeen.Add(signer)
	}

	// Don't continue if none of the signers provided are in the final list.
	if len(clientSigners) == 0 {
		return nil, errors.New("no verifiable signers provided, at least one G... address must be provided")
	}

	// Verify all the transaction's signers (server and client) in one
	// hit. We do this in one hit here even though the server signature was
	// checked in the ReadChallengeTx to ensure that every signature and signer
	// are consumed only once on the transaction.
	allSigners := append([]string{serverKP.Address()}, clientSigners...)
	allSignersFound, err := verifyTxSignatures(tx, network, allSigners...)
	if err != nil {
		return nil, err
	}

	// Confirm the server is in the list of signers found and remove it.
	serverSignerFound := false
	signersFound := make([]string, 0, len(allSignersFound)-1)
	for _, signer := range allSignersFound {
		if signer == serverKP.Address() {
			serverSignerFound = true
			continue
		}
		signersFound = append(signersFound, signer)
	}

	// Confirm we matched a signature to the server signer.
	if !serverSignerFound {
		return nil, errors.Errorf("transaction not signed by %s", serverKP.Address())
	}

	// Confirm we matched signatures to the client signers.
	if len(signersFound) == 0 {
		return nil, errors.Errorf("transaction not signed by %s", strings.Join(clientSigners, ", "))
	}

	// Confirm all signatures were consumed by a signer.
	if len(allSignersFound) != len(tx.Signatures()) {
		return signersFound, errors.Errorf("transaction has unrecognized signatures")
	}

	return signersFound, nil
}

// verifyTxSignature checks if a transaction has been signed by the provided Stellar account.
func verifyTxSignature(tx *Transaction, network string, signer string) error {
	signersFound, err := verifyTxSignatures(tx, network, signer)
	if len(signersFound) == 0 {
		return errors.Errorf("transaction not signed by %s", signer)
	}
	return err
}

// verifyTxSignatures checks if a transaction has been signed by one or more of
// the signers, returning a list of signers that were found to have signed the
// transaction.
func verifyTxSignatures(tx *Transaction, network string, signers ...string) ([]string, error) {
	txHash, err := tx.Hash(network)
	if err != nil {
		return nil, err
	}

	// find and verify signatures
	signatureUsed := map[int]bool{}
	signersFound := set.Set[string]{}
	for _, signer := range signers {
		kp, err := keypair.ParseAddress(signer)
		if err != nil {
			return nil, errors.Wrap(err, "signer not address")
		}

		for i, decSig := range tx.Signatures() {
			if signatureUsed[i] {
				continue
			}
			if decSig.Hint != kp.Hint() {
				continue
			}
			err := kp.Verify(txHash[:], decSig.Signature)
			if err == nil {
				signatureUsed[i] = true
				signersFound.Add(signer)
				break
			}
		}
	}

	signersFoundList := make([]string, 0, len(signersFound))
	for _, signer := range signers {
		if signersFound.Contains(signer) {
			signersFoundList = append(signersFoundList, signer)
			delete(signersFound, signer)
		}
	}
	return signersFoundList, nil
}
