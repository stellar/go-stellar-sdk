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

	// Scanner-idiom scan (no per-element closures); envelope pairing is by
	// the outer hash, also on an inner-hash match.
	parts, h, applyIdx, err := d.findByHash(xdr.Hash(hash))
	if err != nil {
		return LedgerTransactionView{}, false, fmt.Errorf("ingest: TxProcessing scan: %w", err)
	}
	if applyIdx < 0 {
		return LedgerTransactionView{}, false, nil
	}
	part, err := collectTxParts(parts, h)
	if err != nil {
		return LedgerTransactionView{}, false, err
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

	parts, err := collectTxProcessingRange(lcm, startIdx, limit)
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
// equals target, via the UNSIZED envelope walk: envelopes advance purely by
// HashSized's consumed size, so hashing and iteration share one traversal
// (the multi-hash range path keeps the sized map-based pairing).
func findEnvelopeByHash(d lcmViewDispatch, hasher *network.TransactionViewHasher, target [32]byte) (envInfo, error) {
	var found *envInfo
	visit := func(rest []byte) (int, bool, error) {
		h, consumed, err := hasher.HashSized(rest)
		if err != nil {
			return 0, false, err
		}
		if h != target {
			return consumed, false, nil
		}
		info, err := resolveEnvelope(xdr.ParseTransactionEnvelopeView(rest[:consumed]))
		if err != nil {
			return 0, false, err
		}
		found = &info
		return consumed, true, nil
	}
	if err := d.stepEnvelopes(visit); err != nil {
		return envInfo{}, err
	}
	if found == nil {
		return envInfo{}, errMissingEnvelope(target)
	}
	return *found, nil
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
		if tx, err = v1e.Tx(); err != nil {
			return fail(err)
		}
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		fb, err := env.ArmFeeBump()
		if err != nil {
			return fail(err)
		}
		fbTx, err := fb.Tx()
		if err != nil {
			return fail(err)
		}
		innerTx, err := fbTx.InnerTx()
		if err != nil {
			return fail(err)
		}
		v1e, err := innerTx.ArmV1()
		if err != nil {
			return fail(err)
		}
		if tx, err = v1e.Tx(); err != nil {
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
	ext, err := tx.Ext()
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
func collectTxParts(parts txPartViews, hash xdr.Hash) (txViewParts, error) {
	p := txViewParts{txHash: [32]byte(hash)}

	rv, err := parts.Result.Result()
	if err != nil {
		return p, fmt.Errorf("ingest: tx result: %w", err)
	}
	if p.resultRaw, err = rv.Raw(); err != nil {
		return p, fmt.Errorf("ingest: tx result: %w", err)
	}

	successful, err := rv.Successful()
	if err != nil {
		return p, fmt.Errorf("ingest: tx result: %w", err)
	}
	p.successful = successful

	// Single dispatched walk: contract events + diagnostics + version in one
	// pass (one SorobanMeta unwrap for V3, instead of one per extractor).
	ver, tev, diag, err := metaEventRaws(parts.TxApplyProcessing)
	if err != nil {
		return p, err
	}
	p.txEventRaws = tev.TransactionEvents
	p.opEventRaws = tev.OperationEvents
	p.diagRaws = diag
	p.metaIsV3 = ver == 3

	// The meta's exact extent (one sizing pass; tier-1 views carry no
	// traversal state).
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

// collectTxProcessingRange gathers per-tx fields for apply indices
// [start, start+count) in ONE generated Walk over the LCM (count == 0 means
// "all from start"): result/meta raw extents come from the exact-extent
// views the Walk delivers (slice operations, no re-sizing), events and
// diagnostics from their positions, and the walk stops cleanly (ErrStopWalk)
// after the last wanted transaction's TxMeta. Phase two pairs envelopes by
// hash (the TxSet is in agreed-set order, never apply order).
func collectTxProcessingRange(lcm xdr.LedgerCloseMetaView, start, count int) ([]txViewParts, error) {
	unbounded := count <= 0
	end := start + count
	if !unbounded && end < start { // start+count overflowed
		unbounded = true
	}
	var out []txViewParts
	cur := -1 // index into out for the tx being collected, -1 = out of range
	w := xdr.LedgerCloseMetaWalk{
		TxProcessingBegin: func(n int) error {
			capHint := n - start
			if !unbounded && count < capHint {
				capHint = count
			}
			if capHint > 0 {
				// The prealloc is capped: limit is caller-controlled (the
				// getTransactions page size), and a hostile huge limit must
				// not panic makeslice; real ledgers carry ~1e3 txs, so past
				// the cap the slice just grows by append.
				out = make([]txViewParts, 0, min(capHint, 1<<12))
			}
			return nil
		},
		ResultPair: func(tx int, pair xdr.TransactionResultPairView) error {
			cur = -1
			if !unbounded && tx >= end {
				return xdr.ErrStopWalk
			}
			if tx < start {
				return nil
			}
			raw, err := pair.Raw() // exact-extent: slice op
			if err != nil {
				return fmt.Errorf("ingest: TxProcessing element %d result: %w", tx, err)
			}
			p := txViewParts{
				resultRaw:   pairResultRaw(raw),
				txEventRaws: [][]byte{},
				opEventRaws: [][][]byte{},
				diagRaws:    [][]byte{},
			}
			copy(p.txHash[:], raw[:32])
			rv, err := pair.Result()
			if err != nil {
				return fmt.Errorf("ingest: TxProcessing element %d result: %w", tx, err)
			}
			if p.successful, err = rv.Successful(); err != nil {
				return fmt.Errorf("ingest: TxProcessing element %d result: %w", tx, err)
			}
			out = append(out, p)
			cur = len(out) - 1
			return nil
		},
		MetaVersion: func(_ int, v int32) error {
			if cur >= 0 {
				out[cur].metaIsV3 = v == 3
			}
			return nil
		},
		V4Ops: func(_, nOps int) error {
			if cur >= 0 {
				out[cur].opEventRaws = make([][][]byte, 0, nOps)
			}
			return nil
		},
		OpEventsBegin: func(_, _, n int) error {
			if cur >= 0 {
				out[cur].opEventRaws = append(out[cur].opEventRaws, make([][]byte, 0, n))
			}
			return nil
		},
		TxEventsBegin: func(_, n int) error {
			if cur >= 0 && n > 0 {
				out[cur].txEventRaws = make([][]byte, 0, n)
			}
			return nil
		},
		DiagEventsBegin: func(_, n int) error {
			if cur >= 0 && n > 0 {
				out[cur].diagRaws = make([][]byte, 0, n)
			}
			return nil
		},
		OpEvent: func(_, _, _ int, e xdr.ContractEventView) error {
			if cur >= 0 {
				g := out[cur].opEventRaws
				g[len(g)-1] = append(g[len(g)-1], e.MustRaw())
			}
			return nil
		},
		TxEvent: func(_, _ int, e xdr.TransactionEventView) error {
			if cur >= 0 {
				out[cur].txEventRaws = append(out[cur].txEventRaws, e.MustRaw())
			}
			return nil
		},
		DiagEvent: func(_, _ int, e xdr.DiagnosticEventView) error {
			if cur >= 0 {
				out[cur].diagRaws = append(out[cur].diagRaws, e.MustRaw())
			}
			return nil
		},
		TxMeta: func(tx int, meta xdr.TransactionMetaView) error {
			if cur >= 0 {
				raw, err := meta.Raw() // exact-extent: slice op
				if err != nil {
					return fmt.Errorf("ingest: TxProcessing element %d meta: %w", tx, err)
				}
				out[cur].metaRaw = raw
				if !unbounded && tx == end-1 {
					return xdr.ErrStopWalk
				}
			}
			return nil
		},
	}
	if err := xdr.WalkLedgerCloseMeta(lcm, &w); err != nil {
		return nil, err
	}
	return out, nil
}
