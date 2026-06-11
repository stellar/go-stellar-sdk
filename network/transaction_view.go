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

// Hash returns the transaction hash, envelope type, and whether the transaction
// is a Soroban transaction, reading everything from the envelope view's wire
// bytes without decoding the envelope. isSoroban mirrors
// ingest.LedgerTransaction.IsSorobanTx (read from the inner Tx.Ext view
// discriminant; for a fee-bump it inspects the inner transaction).
//
// A TX_V0 envelope hashes as its V1 conversion (see HashTransactionV0): on the
// wire that conversion is exactly the 4-byte ed25519 key-type prefix followed
// by the unchanged TransactionV0 bytes — the V0 optional-TimeBounds and V1
// Preconditions encodings are identical (absent ⇒ 0 ⇒ PRECOND_NONE, present ⇒
// 1 + TimeBounds ⇒ PRECOND_TIME), as are all fields after them (both Ext arms
// are void).
func (th *TransactionViewHasher) Hash(env xdr.TransactionEnvelopeView) (txHash [32]byte, typ xdr.EnvelopeType, isSoroban bool, err error) {
	typ, err = xdr.Try(func() xdr.EnvelopeType { return env.MustType().MustValue() })
	if err != nil {
		return [32]byte{}, 0, false, fmt.Errorf("network: envelope type: %w", err)
	}
	switch typ {
	case xdr.EnvelopeTypeEnvelopeTypeTx,
		xdr.EnvelopeTypeEnvelopeTypeTxV0,
		xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
	default:
		return [32]byte{}, 0, false, fmt.Errorf("network: invalid transaction envelope type %v", typ)
	}
	err = xdr.TryVoid(func() {
		switch typ {
		case xdr.EnvelopeTypeEnvelopeTypeTx:
			tx := env.MustV1().MustTx()
			txHash = th.sum(typ, tx.MustRaw())
			isSoroban = txExtIsSoroban(tx)
		case xdr.EnvelopeTypeEnvelopeTypeTxV0:
			txHash = th.sum(xdr.EnvelopeTypeEnvelopeTypeTx,
				ed25519KeyTypePrefix[:], env.MustV0().MustTx().MustRaw())
		case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
			fbTx := env.MustFeeBump().MustTx()
			txHash = th.sum(typ, fbTx.MustRaw())
			isSoroban = txExtIsSoroban(fbTx.MustInnerTx().MustV1().MustTx())
		}
	})
	if err != nil {
		return [32]byte{}, 0, false, fmt.Errorf("network: envelope (%v): %w", typ, err)
	}
	return txHash, typ, isSoroban, nil
}

// txExtIsSoroban reads Tx.Ext's union discriminant (1 ⟺ SorobanTransactionData
// present), mirroring ingest.LedgerTransaction.IsSorobanTx. Must-style: panics
// with *xdr.ViewError on malformed input, recovered by the caller's TryVoid.
func txExtIsSoroban(tx xdr.TransactionView) bool {
	return tx.MustExt().MustV().MustValue() == 1
}

// HashTransactionInEnvelopeView is the one-shot view twin of
// HashTransactionInEnvelope: it allocates a TransactionViewHasher, hashes a
// single envelope view, and returns just the hash. Callers hashing many
// envelopes should reuse a TransactionViewHasher via NewTransactionViewHasher.
func HashTransactionInEnvelopeView(env xdr.TransactionEnvelopeView, passphrase string) ([32]byte, error) {
	th, err := NewTransactionViewHasher(passphrase)
	if err != nil {
		return [32]byte{}, err
	}
	h, _, _, err := th.Hash(env)
	return h, err
}
