package ingest

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// TxEnvelopeSpan is one TxSet envelope's transaction hash paired with the byte
// span of its TransactionEnvelope inside the ledger's raw bytes.
//
// Start and End are offsets from the FIRST BYTE of the xdr.LedgerCloseMetaView
// they were extracted from, so lcm[Start:End] is that complete
// TransactionEnvelope — byte for byte what LedgerTransactionView.Envelope
// aliases for the same transaction. For a fee-bump transaction the span covers
// the OUTER (fee-bump) envelope and Hash is the fee-bump transaction's own
// hash, the one TxProcessing carries; the inner transaction's hash never
// appears here.
//
// The span itself is plain integers — it neither aliases nor pins the ledger
// buffer — but it is only meaningful against the exact byte sequence it was
// computed from.
type TxEnvelopeSpan struct {
	Hash  [32]byte // network transaction hash of the envelope
	Start int      // offset of the envelope's first byte within the LCM bytes
	End   int      // offset one past the envelope's last byte
}

// ExtractLedgerTxEnvelopeSpans walks the ledger's TxSet once and returns one
// TxEnvelopeSpan per transaction envelope, in TxSet (agreed-set) order. That
// is NOT apply order, so a caller pairs these spans to the apply-ordered
// elements of ExtractLedgerTxParts BY HASH, never by position:
//
//	txParts, err := ingest.ExtractLedgerTxParts(lcmView)
//	envSpans, err := ingest.ExtractLedgerTxEnvelopeSpans(lcmView, passphrase)
//	byHash := make(map[[32]byte]ingest.TxEnvelopeSpan, len(envSpans))
//	for _, s := range envSpans {
//		byHash[s.Hash] = s
//	}
//	env := byHash[txParts[i].Hash] // the envelope paired with element i
//
// Hashing is what the pairing needs, so every envelope is hashed with
// network.TransactionViewHasher (hence the passphrase) exactly as
// LedgerTransactionViewByHash / LedgerTransactionViewRange pair theirs.
// Envelopes are enumerated from every phase of the TxSet — the V0 components
// of a generalized set, the stages and clusters of a parallel phase, and the
// plain transaction set of an LCM V0 ledger.
//
// This is the TxSet counterpart of ExtractLedgerTxParts, deliberately a
// separate entry point: ExtractLedgerTxParts reads only TxProcessing, and
// consumers that need no envelopes never pay for the TxSet walk or the
// hashing.
//
// Experimental: the view-based extractors are new in this release and their
// signatures may still change.
func ExtractLedgerTxEnvelopeSpans(lcmView xdr.LedgerCloseMetaView, passphrase string) ([]TxEnvelopeSpan, error) {
	d, err := dispatchLCMView(lcmView)
	if err != nil {
		return nil, err
	}
	hasher, err := network.NewTransactionViewHasher(passphrase)
	if err != nil {
		return nil, err
	}
	var out []TxEnvelopeSpan
	for env, envErr := range d.Envelopes() {
		if envErr != nil {
			return nil, envErr
		}
		hash, hashErr := hasher.Hash(env)
		if hashErr != nil {
			return nil, hashErr
		}
		// Envelopes are yielded untrimmed (an array element view runs to the
		// end of its array), so Raw() is what sizes this one to its exact
		// wire extent — the same call resolveEnvelope makes on the read path.
		raw, rawErr := env.Raw()
		if rawErr != nil {
			return nil, fmt.Errorf("ingest: envelope raw: %w", rawErr)
		}
		start, end, spanErr := viewSpan(lcmView, raw)
		if spanErr != nil {
			return nil, fmt.Errorf("ingest: envelope span: %w", spanErr)
		}
		out = append(out, TxEnvelopeSpan{Hash: hash, Start: start, End: end})
	}
	return out, nil
}

// viewSpan returns sub's byte offsets within base. Every generated *View is a
// reslice of the buffer it was opened on (the view types are named []byte and
// the accessors only ever narrow them, never copy), so the offset is the
// difference in capacities and the end is that plus the view's length — O(1),
// no re-walk of the wire bytes.
//
// The pointer check makes the one assumption explicit: a view that is not a
// reslice of base yields an error instead of a plausible-looking wrong span.
func viewSpan(base, sub []byte) (start, end int, err error) {
	start = cap(base) - cap(sub)
	end = start + len(sub)
	notASlice := start < 0 || len(sub) == 0 || end > len(base) || &base[start] != &sub[0]
	if notASlice {
		return 0, 0, fmt.Errorf(
			"ingest: a %d-byte view is not a slice of the %d-byte ledger buffer", len(sub), len(base))
	}
	return start, end, nil
}
