package ingest

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// LedgerTransactionView is the zero-copy, raw-bytes view of one transaction's
// detail — the view-path parallel of the parsed LedgerTransaction (the name
// also keeps it distinct from the generated xdr.TransactionView). Where
// LedgerTransaction holds decoded xdr.TransactionEnvelope / TransactionResult
// / TransactionMeta values, the fields here are raw XDR wire bytes that ALIAS
// the source LedgerCloseMetaView buffer (no UnmarshalBinary); callers copy
// what they retain. Produced by the getTransaction/getTransactions read path
// (LedgerTransactionViewByHash / LedgerTransactionViewRange).
//
// The per-operation arity of ContractEvents must match that reported by
// LedgerTransaction.GetTransactionEvents.
type LedgerTransactionView struct {
	Hash              [32]byte
	ApplicationOrder  int32                      // 1-based apply order within the ledger
	FeeBump           bool                       // envelope type is TX_FEE_BUMP
	Successful        bool                       // result code is txSUCCESS / txFEE_BUMP_INNER_SUCCESS
	Envelope          []byte                     // raw xdr.TransactionEnvelope
	Result            []byte                     // raw xdr.TransactionResult
	Meta              []byte                     // raw xdr.TransactionMeta
	DiagnosticEvents  []xdr.DiagnosticEventView  // V3/V4 diagnostic events
	TransactionEvents []xdr.TransactionEventView // V4 top-level events
	ContractEvents    [][]xdr.ContractEventView  // per operation (arity: see above)
	LedgerSequence    uint32
	LedgerCloseTime   int64
}

// envInfo is one envelope resolved while enumerating a TxSet: its raw bytes
// (zero-copy alias), envelope type, and whether it is a Soroban transaction.
type envInfo struct {
	raw       []byte
	typ       xdr.EnvelopeType
	isSoroban bool
}

// LedgerTransactionViewByHash finds the transaction with the given hash in the
// ledger and returns its materialized detail. A fee-bump transaction matches
// either of its hashes — its own (result-pair) hash or the inner transaction's.
// found=false (nil error) if the hash is not present. All byte fields alias
// the lcm view buffer (zero-copy). The passphrase hashes TxSet envelopes so
// each is paired to its TxProcessing entry by hash (the TxSet is in agreed-set
// order, not apply order).
//
// Experimental: the view-based extractors are new in this release and their
// signatures may still change.
func LedgerTransactionViewByHash(lcm xdr.LedgerCloseMetaView, hash [32]byte, passphrase string) (LedgerTransactionView, bool, error) {
	d, err := dispatchLCMView(lcm)
	if err != nil {
		return LedgerTransactionView{}, false, err
	}
	hasher, err := network.NewTransactionViewHasher(passphrase)
	if err != nil {
		return LedgerTransactionView{}, false, err
	}
	ledgerSeq, ledgerCloseTime, err := d.Header()
	if err != nil {
		return LedgerTransactionView{}, false, err
	}

	var applyIdx int
	found, err := d.txs.walk(nil, &offsetRecord{}, func(i int, result xdr.TransactionResultPairView) (bool, bool) {
		h, inner, feeBump := resultHashes(result)
		match := h == hash || (feeBump && inner == hash)
		applyIdx = i
		return match, match
	})
	if err != nil {
		return LedgerTransactionView{}, false, err
	}
	if len(found) == 0 {
		return LedgerTransactionView{}, false, nil
	}
	// Envelope pairing is by the outer hash, also on an inner-hash match.
	env, err := findEnvelopeByHash(d, hasher, found[0].Hash)
	if err != nil {
		return LedgerTransactionView{}, false, err
	}
	tx, err := transactionView(&found[0], env, applyIdx, ledgerSeq, ledgerCloseTime)
	if err != nil {
		return LedgerTransactionView{}, false, err
	}
	return tx, true, nil
}

// LedgerTransactionViewRange returns up to limit transactions in apply order
// (TxProcessing order) starting at startIdx (0-based). limit == 0 returns all
// from startIdx; limit < 0 is an error (symmetric with startIdx). startIdx past
// the end yields an empty slice (nil error); startIdx < 0 is an error. The
// passphrase hashes TxSet envelopes for by-hash pairing. All byte fields alias
// the lcm view buffer (zero-copy).
//
// Experimental: the view-based extractors are new in this release and their
// signatures may still change.
func LedgerTransactionViewRange(lcm xdr.LedgerCloseMetaView, startIdx, limit int, passphrase string) ([]LedgerTransactionView, error) {
	if startIdx < 0 {
		return nil, fmt.Errorf("ingest: startIdx %d < 0", startIdx)
	}
	if limit < 0 {
		return nil, fmt.Errorf("ingest: limit %d < 0", limit)
	}
	d, err := dispatchLCMView(lcm)
	if err != nil {
		return nil, err
	}
	hasher, err := network.NewTransactionViewHasher(passphrase)
	if err != nil {
		return nil, err
	}
	ledgerSeq, ledgerCloseTime, err := d.Header()
	if err != nil {
		return nil, err
	}

	parts, err := collectTxProcessingRange(d.txs, startIdx, limit)
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		return nil, nil
	}

	want := make([][32]byte, len(parts))
	for k := range parts {
		want[k] = parts[k].Hash
	}
	byHash, err := envelopesForHashes(d, hasher, want)
	if err != nil {
		return nil, err
	}

	out := make([]LedgerTransactionView, len(parts))
	for k := range parts {
		env, ok := byHash[parts[k].Hash]
		if !ok {
			return nil, errMissingEnvelope(parts[k].Hash)
		}
		if out[k], err = transactionView(&parts[k], env, startIdx+k, ledgerSeq, ledgerCloseTime); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// transactionView combines a walked transaction with its paired envelope.
// applyIdx is 0-based; ApplicationOrder is 1-based.
func transactionView(
	part *LedgerTxParts, env envInfo, applyIdx int, ledgerSeq uint32, ledgerCloseTime int64,
) (LedgerTransactionView, error) {
	// The result is the trimmed pair's last field, so its view is exact.
	result, err := part.Result.Result()
	if err != nil {
		return LedgerTransactionView{}, fmt.Errorf("ingest: tx result: %w", err)
	}
	successful, err := result.Successful()
	if err != nil {
		return LedgerTransactionView{}, err
	}
	entry := part.walkedEntry()
	events, err := metaEvents(part.Meta, entry, nil)
	if err != nil {
		return LedgerTransactionView{}, err
	}
	diagnostics, err := metaDiagnostics(part.Meta, entry)
	if err != nil {
		return LedgerTransactionView{}, err
	}
	return LedgerTransactionView{
		Hash:              part.Hash,
		ApplicationOrder:  int32(applyIdx) + 1, //nolint:gosec // apply index fits int32
		FeeBump:           env.typ == xdr.EnvelopeTypeEnvelopeTypeTxFeeBump,
		Successful:        successful,
		Envelope:          env.raw,
		Result:            result,
		Meta:              part.Meta,
		DiagnosticEvents:  diagnostics,
		TransactionEvents: events.TransactionEvents,
		ContractEvents:    alignV3ContractEvents(events.OperationEvents, entry[entryVersion] == 3, env.isSoroban), //nolint:mnd // V3
		LedgerSequence:    ledgerSeq,
		LedgerCloseTime:   ledgerCloseTime,
	}, nil
}

// envelopesForHashes enumerates the TxSet and returns the envelopes whose
// transaction hashes appear in want, mirroring
// LedgerTransactionReader.storeTransactions: every envelope is hashed so a
// TxProcessing entry's TransactionHash locates its OWN envelope (the TxSet is
// in agreed-set order, NOT apply order, so positional pairing would mispair).
// Enumeration stops as soon as every wanted hash is resolved, so a small page
// does not pay for the whole TxSet.
func envelopesForHashes(d lcmViewDispatch, hasher *network.TransactionViewHasher, want [][32]byte) (map[[32]byte]envInfo, error) {
	need := make(map[[32]byte]struct{}, len(want))
	for _, h := range want {
		need[h] = struct{}{}
	}
	byHash := make(map[[32]byte]envInfo, len(need))
	for env, err := range d.Envelopes() {
		if err != nil {
			return nil, err
		}
		// Hash first and skip unwanted envelopes before extracting their details:
		// the membership test needs only the hash, so the type/soroban/raw reads
		// below run only for the envelopes actually paired (on a by-hash lookup or
		// a small page, that is far fewer than the whole TxSet that gets hashed).
		h, err := hasher.Hash(env)
		if err != nil {
			return nil, err
		}
		if _, ok := need[h]; !ok {
			continue
		}
		info, err := resolveEnvelope(env)
		if err != nil {
			return nil, err
		}
		byHash[h] = info
		delete(need, h)
		if len(need) == 0 {
			break
		}
	}
	return byHash, nil
}

// errMissingEnvelope is the single construction site for the inconsistent-LCM
// condition (a TxProcessing hash with no matching TxSet envelope), shared by
// the by-hash and range paths so they cannot drift.
func errMissingEnvelope(hash [32]byte) error {
	return fmt.Errorf(
		"ingest: tx %x present in TxProcessing but missing from TxSet (inconsistent LCM)", hash)
}

// findEnvelopeByHash resolves the single envelope whose transaction hash
// equals target. It is the one-element case of envelopesForHashes (same loop,
// same early stop on resolution), kept as a wrapper so the pairing logic
// exists in exactly one place.
func findEnvelopeByHash(d lcmViewDispatch, hasher *network.TransactionViewHasher, target [32]byte) (envInfo, error) {
	byHash, err := envelopesForHashes(d, hasher, [][32]byte{target})
	if err != nil {
		return envInfo{}, err
	}
	info, ok := byHash[target]
	if !ok {
		return envInfo{}, errMissingEnvelope(target)
	}
	return info, nil
}

// resolveEnvelope reads a matched envelope's details — its type discriminant,
// the soroban flag, and its raw bytes — into an envInfo. Called only for an
// envelope that matched a wanted hash; the hashing that selects which envelopes
// reach here is done in envelopesForHashes.
func resolveEnvelope(env xdr.TransactionEnvelopeView) (envInfo, error) {
	typ, isSoroban, err := envelopeTypeAndSoroban(env)
	if err != nil {
		return envInfo{}, err
	}
	raw, err := env.Raw()
	if err != nil {
		return envInfo{}, fmt.Errorf("ingest: envelope raw: %w", err)
	}
	return envInfo{raw: raw, typ: typ, isSoroban: isSoroban}, nil
}

// envelopeTypeAndSoroban reads the envelope-type discriminant and the
// soroban flag (Tx.Ext union discriminant 1 ⟺ SorobanTransactionData present,
// mirroring LedgerTransaction.IsSorobanTx; for a fee-bump, the inner
// transaction's). TX_V0 predates Soroban, so it is never soroban.
func envelopeTypeAndSoroban(env xdr.TransactionEnvelopeView) (typ xdr.EnvelopeType, isSoroban bool, err error) {
	err = xdr.TryVoid(func() {
		typ = env.MustType()
		switch typ {
		case xdr.EnvelopeTypeEnvelopeTypeTx:
			isSoroban = txExtIsSoroban(env.MustV1().MustTx())
		case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
			isSoroban = txExtIsSoroban(env.MustFeeBump().MustTx().MustInnerTx().MustV1().MustTx())
		}
	})
	if err != nil {
		return 0, false, fmt.Errorf("ingest: envelope type/soroban: %w", err)
	}
	return typ, isSoroban, nil
}

// txExtIsSoroban reads Tx.Ext's union discriminant. Must-style: panics with
// *xdr.ViewError on malformed input, recovered by the caller's TryVoid.
func txExtIsSoroban(tx xdr.TransactionView) bool {
	return tx.MustExt().MustV() == 1
}

// alignV3ContractEvents gives a V3 meta's per-operation contract events the
// same arity GetTransactionEvents produces, decided entirely from the envelope:
//
//   - not a Soroban tx: no operation slots at all, so any events the meta
//     carries are dropped (V3 classic operations have none on the wire).
//   - a Soroban tx: exactly one operation slot, even when SorobanMeta is
//     absent (a charged-but-never-executed transaction, a real pubnet shape
//     on protocols 20-22), and then the slot is empty.
func alignV3ContractEvents(opEvents [][]xdr.ContractEventView, metaV3, isSoroban bool) [][]xdr.ContractEventView {
	switch {
	case !metaV3:
		return opEvents
	case !isSoroban:
		return [][]xdr.ContractEventView{}
	case len(opEvents) == 0:
		return [][]xdr.ContractEventView{{}}
	default:
		return opEvents
	}
}

// collectTxProcessingRange walks TxProcessing once and returns the parts of
// apply indices [start, start+count). count == 0 means "all from start". A
// start past the end yields an empty slice (not an error).
func collectTxProcessingRange(txs txProcessing, start, count int) ([]LedgerTxParts, error) {
	if start >= txs.count {
		return nil, nil
	}
	end := txs.count
	if count > 0 && count < end-start {
		end = start + count
	}
	out, rec := presized(end - start)
	return txs.walk(out, rec, func(i int, _ xdr.TransactionResultPairView) (bool, bool) {
		return i >= start, i+1 >= end
	})
}
