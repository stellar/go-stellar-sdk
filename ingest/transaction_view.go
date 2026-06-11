package ingest

import (
	"fmt"
	"iter"

	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Transaction is the materialized zero-copy detail for one transaction. It is
// the view analog of LedgerTransaction for the getTransaction/getTransactions
// read path: raw XDR fields ALIAS the source LedgerCloseMetaView buffer (no
// UnmarshalBinary), so callers copy what they retain.
type Transaction struct {
	Hash              [32]byte
	ApplicationOrder  int32      // 1-based apply order within the ledger
	FeeBump           bool       // envelope type is TX_FEE_BUMP
	Successful        bool       // result code is txSUCCESS / txFEE_BUMP_INNER_SUCCESS
	Envelope          []byte     // raw xdr.TransactionEnvelope
	Result            []byte     // raw xdr.TransactionResult
	Meta              []byte     // raw xdr.TransactionMeta
	Events            [][]byte   // raw xdr.DiagnosticEvent (V3/V4 diagnostic)
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

// TransactionViewByHash finds the transaction with the given hash in the ledger
// and returns its materialized detail. found=false (nil error) if the hash is
// not present. All byte fields alias the lcm view buffer (zero-copy). The
// passphrase hashes TxSet envelopes so each is paired to its TxProcessing entry
// by hash (the TxSet is in agreed-set order, not apply order).
func TransactionViewByHash(lcm xdr.LedgerCloseMetaView, hash [32]byte, passphrase string) (Transaction, bool, error) {
	d, err := DispatchLedgerCloseMetaView(lcm)
	if err != nil {
		return Transaction{}, false, err
	}
	hasher, err := network.NewTransactionViewHasher(passphrase)
	if err != nil {
		return Transaction{}, false, err
	}
	ledgerSeq, ledgerCloseTime, err := d.Header()
	if err != nil {
		return Transaction{}, false, err
	}

	applyIdx := -1
	var part txViewParts
	idx := 0
	for txView, iterErr := range d.TxProcessing() {
		if iterErr != nil {
			return Transaction{}, false, fmt.Errorf("ingest: TxProcessing iter: %w", iterErr)
		}
		h, herr := TxProcessingHash(txView)
		if herr != nil {
			return Transaction{}, false, herr
		}
		if h == xdr.Hash(hash) {
			part, err = collectTxParts(txView, h)
			if err != nil {
				return Transaction{}, false, err
			}
			applyIdx = idx
			break
		}
		idx++
	}
	if applyIdx < 0 {
		return Transaction{}, false, nil
	}

	env, err := findEnvelopeByHash(d, hasher, part.txHash)
	if err != nil {
		return Transaction{}, false, err
	}
	return assembleTransaction(part, env, applyIdx, ledgerSeq, ledgerCloseTime), true, nil
}

// TransactionViewRange returns up to limit transactions in apply order
// (TxProcessing order) starting at startIdx (0-based). limit == 0 returns all
// from startIdx; limit < 0 is an error (symmetric with startIdx). startIdx past
// the end yields an empty slice (nil error); startIdx < 0 is an error. The
// passphrase hashes TxSet envelopes for by-hash pairing. All byte fields alias
// the lcm view buffer (zero-copy).
func TransactionViewRange(lcm xdr.LedgerCloseMetaView, startIdx, limit int, passphrase string) ([]Transaction, error) {
	if startIdx < 0 {
		return nil, fmt.Errorf("ingest: startIdx %d < 0", startIdx)
	}
	if limit < 0 {
		return nil, fmt.Errorf("ingest: limit %d < 0", limit)
	}
	d, err := DispatchLedgerCloseMetaView(lcm)
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

	out := make([]Transaction, len(parts))
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
// Transaction. applyIdx is 0-based; ApplicationOrder is 1-based.
func assembleTransaction(part txViewParts, env envInfo, applyIdx int, ledgerSeq uint32, ledgerCloseTime int64) Transaction {
	return Transaction{
		Hash:              part.txHash,
		ApplicationOrder:  int32(applyIdx) + 1, //nolint:gosec // apply index fits int32
		FeeBump:           env.typ == xdr.EnvelopeTypeEnvelopeTypeTxFeeBump,
		Successful:        part.successful,
		Envelope:          env.raw,
		Result:            part.resultRaw,
		Meta:              part.metaRaw,
		Events:            part.diagRaws,
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
func envelopesForHashes(d LedgerCloseMetaViewDispatch, hasher *network.TransactionViewHasher, want [][32]byte) (map[[32]byte]envInfo, error) {
	need := make(map[[32]byte]struct{}, len(want))
	for _, h := range want {
		need[h] = struct{}{}
	}
	byHash := make(map[[32]byte]envInfo, len(need))
	for env, err := range d.Envelopes() {
		if err != nil {
			return nil, err
		}
		info, h, err := resolveEnvelope(hasher, env)
		if err != nil {
			return nil, err
		}
		if _, ok := need[h]; ok {
			byHash[h] = info
			delete(need, h)
			if len(need) == 0 {
				break
			}
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
func findEnvelopeByHash(d LedgerCloseMetaViewDispatch, hasher *network.TransactionViewHasher, target [32]byte) (envInfo, error) {
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

// resolveEnvelope hashes an envelope view and reads its raw bytes, returning the
// envInfo and the transaction hash key.
func resolveEnvelope(hasher *network.TransactionViewHasher, env xdr.TransactionEnvelopeView) (envInfo, [32]byte, error) {
	h, typ, isSoroban, err := hasher.Hash(env)
	if err != nil {
		return envInfo{}, [32]byte{}, err
	}
	raw, err := env.Raw()
	if err != nil {
		return envInfo{}, [32]byte{}, fmt.Errorf("ingest: envelope raw: %w", err)
	}
	return envInfo{raw: raw, typ: typ, isSoroban: isSoroban}, h, nil
}

// collectTxParts gathers the per-tx result/meta/events for one TxProcessing
// entry view (hash already read by the caller). Event extraction defers to the
// xdr view helpers; the V3 soroban gate is applied later by gateV3ContractEvents
// once the paired envelope is known.
func collectTxParts(txView TxResultMetaView, hash xdr.Hash) (txViewParts, error) {
	p := txViewParts{txHash: [32]byte(hash)}

	rp, err := txView.Result()
	if err != nil {
		return p, fmt.Errorf("ingest: Result: %w", err)
	}
	rv, err := rp.Result()
	if err != nil {
		return p, fmt.Errorf("ingest: Result.Result: %w", err)
	}
	p.resultRaw, err = rv.Raw()
	if err != nil {
		return p, fmt.Errorf("ingest: Result.Raw: %w", err)
	}
	p.successful, err = rv.Successful()
	if err != nil {
		return p, err
	}

	metaView, err := txView.TxApplyProcessing()
	if err != nil {
		return p, fmt.Errorf("ingest: TxApplyProcessing: %w", err)
	}
	p.metaRaw, err = metaView.Raw()
	if err != nil {
		return p, fmt.Errorf("ingest: Meta.Raw: %w", err)
	}

	// Single dispatched walk: contract events + diagnostics + version in one
	// pass (one SorobanMeta unwrap for V3, instead of one per extractor).
	ver, tev, diag, err := metaEventRaws(metaView, true, true)
	if err != nil {
		return p, err
	}
	p.txEventRaws = tev.TransactionEvents
	p.opEventRaws = tev.OperationEvents
	p.diagRaws = diag
	p.metaIsV3 = ver == 3
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
func collectTxProcessingRange(tp iter.Seq2[TxResultMetaView, error], start, count int) ([]txViewParts, error) {
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
	for txView, iterErr := range tp {
		if iterErr != nil {
			return nil, fmt.Errorf("ingest: TxProcessing iter: %w", iterErr)
		}
		if !unbounded && idx >= end {
			break
		}
		if idx >= start {
			h, herr := TxProcessingHash(txView)
			if herr != nil {
				return nil, herr
			}
			p, perr := collectTxParts(txView, h)
			if perr != nil {
				return nil, perr
			}
			out = append(out, p)
		}
		idx++
	}
	return out, nil
}
