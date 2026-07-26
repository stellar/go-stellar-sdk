package network

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// TransactionViewHasher computes transaction hashes straight from envelope view
// wire bytes — the zero-copy twin of HashTransactionInEnvelope. The tx-hash
// preimage is
//
//	SHA256(networkID ‖ envelope-type tag (4 bytes) ‖ Transaction XDR)
//
// and every piece is available as a wire-byte slice off the view, so no
// envelope decode (UnmarshalBinary) is needed. The network ID is derived once
// and the SHA-256 state is reused, so hashing allocates nothing per envelope.
//
// A TransactionViewHasher is NOT safe for concurrent use (it reuses one hash
// state); use one per goroutine.
type TransactionViewHasher struct {
	networkID [32]byte
	h         hash.Hash
	// tag and out are reusable scratch buffers: locals passed to the
	// hash.Hash interface methods are forced to the heap (the compiler
	// cannot prove Write/Sum do not retain them), which would cost two
	// allocations per envelope hashed.
	tag [4]byte
	out [32]byte
}

// NewTransactionViewHasher derives the network ID from passphrase. The
// empty-passphrase guard is the same validatePassphrase hashTx uses, surfaced
// once per hasher instead of once per envelope.
func NewTransactionViewHasher(passphrase string) (*TransactionViewHasher, error) {
	if err := validatePassphrase(passphrase); err != nil {
		return nil, err
	}
	return &TransactionViewHasher{networkID: ID(passphrase), h: sha256.New()}, nil
}

// ed25519KeyTypePrefix is the 4-byte XDR discriminant
// CryptoKeyTypeKeyTypeEd25519 (0) that converting a TransactionV0 to a
// Transaction prepends to the source account key on the wire.
var ed25519KeyTypePrefix = [4]byte{0, 0, 0, 0}

// sum computes SHA256(networkID ‖ tag ‖ parts...) using the hasher's scratch
// buffers (zero allocations; the result array is returned by value).
func (th *TransactionViewHasher) sum(tag xdr.EnvelopeType, parts ...[]byte) [32]byte {
	th.h.Reset()
	th.h.Write(th.networkID[:])
	binary.BigEndian.PutUint32(th.tag[:], uint32(tag)) //nolint:gosec // enum tag is small and non-negative
	th.h.Write(th.tag[:])
	for _, p := range parts {
		th.h.Write(p)
	}
	th.h.Sum(th.out[:0])
	return th.out
}

// Hash returns the transaction hash, reading everything from the envelope
// view's wire bytes without decoding the envelope. The envelope type is read
// internally to select the preimage shape but deliberately NOT returned —
// callers that need the type (or soroban-ness) read those discriminants
// themselves; neither involves hashing or the passphrase.
//
// A TX_V0 envelope hashes as its V1 conversion (see HashTransactionV0): on the
// wire that conversion is exactly the 4-byte ed25519 key-type prefix followed
// by the unchanged TransactionV0 bytes — the V0 optional-TimeBounds and V1
// Preconditions encodings are identical (absent ⇒ 0 ⇒ PRECOND_NONE, present ⇒
// 1 + TimeBounds ⇒ PRECOND_TIME), as are all fields after them (both Ext arms
// are void).
func (th *TransactionViewHasher) Hash(env xdr.TransactionEnvelopeView) ([32]byte, error) {
	typ, err := env.Type()
	if err != nil {
		return [32]byte{}, fmt.Errorf("network: envelope type: %w", err)
	}
	var txHash [32]byte
	switch typ {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		raw, err := v1TxRaw(env)
		if err != nil {
			return [32]byte{}, fmt.Errorf("network: envelope (%v): %w", typ, err)
		}
		txHash = th.sum(typ, raw)
	case xdr.EnvelopeTypeEnvelopeTypeTxV0:
		raw, err := v0TxRaw(env)
		if err != nil {
			return [32]byte{}, fmt.Errorf("network: envelope (%v): %w", typ, err)
		}
		txHash = th.sum(xdr.EnvelopeTypeEnvelopeTypeTx, ed25519KeyTypePrefix[:], raw)
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		raw, err := feeBumpTxRaw(env)
		if err != nil {
			return [32]byte{}, fmt.Errorf("network: envelope (%v): %w", typ, err)
		}
		txHash = th.sum(typ, raw)
	default:
		return [32]byte{}, fmt.Errorf("network: invalid transaction envelope type %v", typ)
	}
	return txHash, nil
}

// v1TxRaw returns the wire bytes of the V1 arm's Transaction (the first field
// of TransactionV1Envelope, so entering the bundle is O(1)).
func v1TxRaw(env xdr.TransactionEnvelopeView) ([]byte, error) {
	arm, err := env.ArmV1()
	if err != nil {
		return nil, err
	}
	f, err := arm.Fields()
	if err != nil {
		return nil, err
	}
	tx, err := f.Tx()
	if err != nil {
		return nil, err
	}
	return tx.Raw()
}

// v0TxRaw is v1TxRaw for the legacy TX_V0 arm.
func v0TxRaw(env xdr.TransactionEnvelopeView) ([]byte, error) {
	arm, err := env.ArmV0()
	if err != nil {
		return nil, err
	}
	f, err := arm.Fields()
	if err != nil {
		return nil, err
	}
	tx, err := f.Tx()
	if err != nil {
		return nil, err
	}
	return tx.Raw()
}

// feeBumpTxRaw is v1TxRaw for the fee-bump arm.
func feeBumpTxRaw(env xdr.TransactionEnvelopeView) ([]byte, error) {
	arm, err := env.ArmFeeBump()
	if err != nil {
		return nil, err
	}
	f, err := arm.Fields()
	if err != nil {
		return nil, err
	}
	tx, err := f.Tx()
	if err != nil {
		return nil, err
	}
	return tx.Raw()
}
