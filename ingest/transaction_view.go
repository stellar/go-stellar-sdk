package ingest

import (
	"fmt"
	"iter"

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
type LedgerTransactionView struct {
	Hash              [32]byte
	ApplicationOrder  int32      // 1-based apply order within the ledger
	FeeBump           bool       // envelope type is TX_FEE_BUMP
	Successful        bool       // result code is txSUCCESS / txFEE_BUMP_INNER_SUCCESS
	Envelope          []byte     // raw xdr.TransactionEnvelope
	Result            []byte     // raw xdr.TransactionResult
	Meta              []byte     // raw xdr.TransactionMeta
	DiagnosticEvents  [][]byte   // raw xdr.DiagnosticEvent (V3/V4 diagnostic)
	TransactionEvents [][]byte   // raw xdr.TransactionEvent (V4 top-level)
	ContractEvents    [][][]byte // raw xdr.ContractEvent, per operation
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

// txViewParts holds the per-tx fields gathered from a single pass over a
// TxProcessing view (everything except the envelope, which lives in the
// agreed-set-ordered TxSet and is paired back by hash). metaIsV3 lets the
// assembly path gate V3 ContractEvents on the envelope-derived IsSorobanTx
// check, the way the parsed reader's GetTransactionEvents does.
type txViewParts struct {
	resultRaw   []byte
	metaRaw     []byte
	txHash      [32]byte
	successful  bool
	diagRaws    [][]byte
	txEventRaws [][]byte
	opEventRaws [][][]byte
	metaIsV3    bool
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

	applyIdx := -1
	var part txViewParts
	idx := 0
	for parts, iterErr := range d.TxProcessing() {
		if iterErr != nil {
			return LedgerTransactionView{}, false, fmt.Errorf("ingest: TxProcessing iter: %w", iterErr)
		}
		h, inner, feeBump, herr := txProcessingHashes(parts)
		if herr != nil {
			return LedgerTransactionView{}, false, herr
		}
		match := h == xdr.Hash(hash)
		if !match && feeBump {
			match = inner == xdr.Hash(hash)
		}
		if match {
			// Envelope pairing is by the outer hash, also on an inner-hash match.
			part, err = collectTxParts(parts, h)
			if err != nil {
				return LedgerTransactionView{}, false, err
			}
			applyIdx = idx
			break
		}
		idx++
	}
	if applyIdx < 0 {
		return LedgerTransactionView{}, false, nil
	}

	env, err := findEnvelopeByHash(d, hasher, part.txHash)
	if err != nil {
		return LedgerTransactionView{}, false, err
	}
	return assembleTransaction(part, env, applyIdx, ledgerSeq, ledgerCloseTime), true, nil
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

	parts, err := collectTxProcessingRange(d.TxProcessing(), startIdx, limit)
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		return nil, nil
	}

	want := make([][32]byte, len(parts))
	for k := range parts {
		want[k] = parts[k].txHash
	}
	byHash, err := envelopesForHashes(d, hasher, want)
	if err != nil {
		return nil, err
	}

	out := make([]LedgerTransactionView, len(parts))
	for k := range parts {
		env, ok := byHash[parts[k].txHash]
		if !ok {
			return nil, errMissingEnvelope(parts[k].txHash)
		}
		out[k] = assembleTransaction(parts[k], env, startIdx+k, ledgerSeq, ledgerCloseTime)
	}
	return out, nil
}

// assembleTransaction combines the per-tx parts with the paired envelope into a
// LedgerTransactionView. applyIdx is 0-based; ApplicationOrder is 1-based.
func assembleTransaction(part txViewParts, env envInfo, applyIdx int, ledgerSeq uint32, ledgerCloseTime int64) LedgerTransactionView {
	return LedgerTransactionView{
		Hash:              part.txHash,
		ApplicationOrder:  int32(applyIdx) + 1, //nolint:gosec // apply index fits int32
		FeeBump:           env.typ == xdr.EnvelopeTypeEnvelopeTypeTxFeeBump,
		Successful:        part.successful,
		Envelope:          env.raw,
		Result:            part.resultRaw,
		Meta:              part.metaRaw,
		DiagnosticEvents:  part.diagRaws,
		TransactionEvents: part.txEventRaws,
		ContractEvents:    gateV3ContractEvents(part, env.isSoroban),
		LedgerSequence:    ledgerSeq,
		LedgerCloseTime:   ledgerCloseTime,
	}
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
func envelopeTypeAndSoroban(env xdr.TransactionEnvelopeView) (xdr.EnvelopeType, bool, error) {
	fail := func(err error) (xdr.EnvelopeType, bool, error) {
		return 0, false, fmt.Errorf("ingest: envelope type/soroban: %w", err)
	}
	typ, err := env.Type()
	if err != nil {
		return fail(err)
	}
	var tx xdr.TransactionView
	switch typ {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		v1e, err := env.ArmV1()
		if err != nil {
			return fail(err)
		}
		ef, err := v1e.Fields()
		if err != nil {
			return fail(err)
		}
		if tx, err = ef.Tx(); err != nil {
			return fail(err)
		}
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		fb, err := env.ArmFeeBump()
		if err != nil {
			return fail(err)
		}
		fbf, err := fb.Fields()
		if err != nil {
			return fail(err)
		}
		fbTx, err := fbf.Tx()
		if err != nil {
			return fail(err)
		}
		tf, err := fbTx.Fields()
		if err != nil {
			return fail(err)
		}
		innerTx, err := tf.InnerTx()
		if err != nil {
			return fail(err)
		}
		v1e, err := innerTx.ArmV1()
		if err != nil {
			return fail(err)
		}
		ef, err := v1e.Fields()
		if err != nil {
			return fail(err)
		}
		if tx, err = ef.Tx(); err != nil {
			return fail(err)
		}
	default:
		// TX_V0 (or an unknown type): never soroban; nothing more to read.
		return typ, false, nil
	}
	isSoroban, err := txExtIsSoroban(tx)
	if err != nil {
		return fail(err)
	}
	return typ, isSoroban, nil
}

// txExtIsSoroban reads Tx.Ext's union discriminant (resolving the bundle
// through the transaction's leading fields to locate Ext).
func txExtIsSoroban(tx xdr.TransactionView) (bool, error) {
	f, err := tx.Fields()
	if err != nil {
		return false, err
	}
	ext, err := f.Ext()
	if err != nil {
		return false, err
	}
	v, err := ext.V()
	if err != nil {
		return false, err
	}
	return v == 1, nil
}

// collectTxParts gathers the per-tx result/meta/events for one TxProcessing
// entry view (hash already read by the caller), reading in wire order; the V3
// soroban gate is applied later by gateV3ContractEvents once the paired
// envelope is known.
func collectTxParts(parts txResultParts, hash xdr.Hash) (txViewParts, error) {
	p := txViewParts{txHash: [32]byte(hash)}

	rf, err := parts.Result.Fields()
	if err != nil {
		return p, fmt.Errorf("ingest: tx result: %w", err)
	}
	rv, err := rf.Result()
	if err != nil {
		return p, fmt.Errorf("ingest: tx result: %w", err)
	}
	if p.resultRaw, err = rv.Raw(); err != nil {
		return p, fmt.Errorf("ingest: tx result: %w", err)
	}

	successful, err := rv.Successful()
	if err != nil {
		return p, err
	}
	p.successful = successful

	// Single dispatched walk: contract events + diagnostics + version in one
	// pass (one SorobanMeta unwrap for V3, instead of one per extractor).
	ver, tev, diag, err := metaEventRaws(parts.TxApplyProcessing, true, true)
	if err != nil {
		return p, err
	}
	p.txEventRaws = tev.TransactionEvents
	p.opEventRaws = tev.OperationEvents
	p.diagRaws = diag
	p.metaIsV3 = ver == 3

	// The meta's exact extent, sized AFTER its interior was consumed above so
	// the walk record resumes the trailing fields instead of re-walking the
	// drained arrays.
	if p.metaRaw, err = parts.TxApplyProcessing.Raw(); err != nil {
		return p, fmt.Errorf("ingest: tx meta: %w", err)
	}
	return p, nil
}

// gateV3ContractEvents zeroes ContractEvents for a V3 meta whose envelope is NOT
// a Soroban tx, matching the parsed reader (GetTransactionEvents returns no
// OperationEvents for a non-Soroban V3 tx). V4 per-op events and the diagnostic
// field are unaffected.
func gateV3ContractEvents(p txViewParts, isSoroban bool) [][][]byte {
	if p.metaIsV3 && !isSoroban {
		return [][][]byte{}
	}
	return p.opEventRaws
}

// collectTxProcessingRange walks the TxProcessing iterable once and gathers
// per-tx fields for apply indices [start, start+count). count == 0 means "all
// from start". A start past the end yields an empty slice (not an error).
func collectTxProcessingRange(tp iter.Seq2[txResultParts, error], start, count int) ([]txViewParts, error) {
	unbounded := count <= 0
	end := start + count
	if !unbounded && end < start { // start+count overflowed: nothing past MaxInt exists anyway
		unbounded = true
	}
	var out []txViewParts
	if !unbounded {
		// count is caller-controlled (the getTransactions limit): cap the
		// prealloc so a huge limit cannot panic in makeslice; real ledgers
		// carry ~1e3 txs, so past the cap the slice just grows by append.
		out = make([]txViewParts, 0, min(count, 1<<12))
	}
	idx := 0
	for parts, iterErr := range tp {
		if iterErr != nil {
			return nil, fmt.Errorf("ingest: TxProcessing iter: %w", iterErr)
		}
		if !unbounded && idx >= end {
			break
		}
		if idx >= start {
			h, herr := txProcessingHash(parts)
			if herr != nil {
				return nil, herr
			}
			p, perr := collectTxParts(parts, h)
			if perr != nil {
				return nil, perr
			}
			out = append(out, p)
		}
		idx++
	}
	return out, nil
}
