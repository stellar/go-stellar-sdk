package ingest

import (
	"encoding/binary"
	"fmt"
	"iter"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// txResultParts is the concrete per-tx projection of a TxProcessing element:
// the result pair and the apply-processing meta, located in wire order from the
// element's field bundle by the same walk that later advances the iterator.
// Both sub-views share the parse's walk, so consuming the meta (the bulk
// field, last in the read order) leaves the record the iterator's advance
// resumes from.
type txResultParts struct {
	Result            xdr.TransactionResultPairView
	TxApplyProcessing xdr.TransactionMetaView
}

// txPartsSeq adapts a V0/V1 TxProcessing element sequence
// (TransactionResultMeta) to the version-agnostic txResultParts projection.
// Per element it opens the field bundle and resolves Result and
// TxApplyProcessing in wire order — one pass over the element's leading
// fields; the enclosing All() advance then resumes from whatever subtree the
// consumer finished last.
//
// The adapter invokes the inner sequence directly with a fused yield instead
// of ranging over it: per element that is one closure call straight through
// (generated loop -> fused yield -> consumer), skipping the range-over-func
// state machine an intermediate `for range` would add to the hot path.
func txPartsSeq(elems iter.Seq2[xdr.TransactionResultMetaView, error]) iter.Seq2[txResultParts, error] {
	return func(yield func(txResultParts, error) bool) {
		k := 0
		elems(func(elem xdr.TransactionResultMetaView, err error) bool {
			if err != nil {
				yield(txResultParts{}, fmt.Errorf("ingest: TxProcessing element %d: %w", k, err))
				return false
			}
			res, err := elem.Result()
			if err != nil {
				yield(txResultParts{}, fmt.Errorf("ingest: TxProcessing element %d: %w", k, err))
				return false
			}
			meta, err := elem.TxApplyProcessing()
			if err != nil {
				yield(txResultParts{}, fmt.Errorf("ingest: TxProcessing element %d: %w", k, err))
				return false
			}
			k++
			return yield(txResultParts{Result: res, TxApplyProcessing: meta}, nil)
		})
	}
}

// txPartsSeqV1 is txPartsSeq for a V2 TxProcessing array
// (TransactionResultMetaV1 element — Ext leads and PostTxApplyFeeProcessing
// trails; the iterator's advance sizes the trailing field). It is a separate
// copy only because the element view type — and therefore its bundle type —
// differs, which a single function cannot span.
func txPartsSeqV1(elems iter.Seq2[xdr.TransactionResultMetaV1View, error]) iter.Seq2[txResultParts, error] {
	return func(yield func(txResultParts, error) bool) {
		k := 0
		elems(func(elem xdr.TransactionResultMetaV1View, err error) bool {
			if err != nil {
				yield(txResultParts{}, fmt.Errorf("ingest: TxProcessing element %d: %w", k, err))
				return false
			}
			res, err := elem.Result()
			if err != nil {
				yield(txResultParts{}, fmt.Errorf("ingest: TxProcessing element %d: %w", k, err))
				return false
			}
			meta, err := elem.TxApplyProcessing()
			if err != nil {
				yield(txResultParts{}, fmt.Errorf("ingest: TxProcessing element %d: %w", k, err))
				return false
			}
			k++
			return yield(txResultParts{Result: res, TxApplyProcessing: meta}, nil)
		})
	}
}

// lcmViewDispatch holds the version-agnostic handles the extractors need from
// one xdr.LedgerCloseMetaView: the LCM view itself (for the ledger header), the
// TxProcessing sequence (apply order), and an enumerator over the TxSet's
// transaction envelopes. dispatchLCMView resolves these from the V0/V1/V2 union
// once, so the extractors never branch on the LCM version themselves. The TxSet
// is in agreed-set / hash-sorted order — which differs from TxProcessing apply
// order — so callers pair envelopes to transactions BY HASH, never by array
// position. V0 uses a plain TransactionSet; V1/V2 use a
// GeneralizedTransactionSet.
type lcmViewDispatch struct {
	lcm xdr.LedgerCloseMetaView
	tp  iter.Seq2[txResultParts, error]
	// txCount is the validated TxProcessing element count, read once at
	// dispatch (O(1)) so extractors can presize per-tx output slices without
	// a counting pass over the iter.Seq2.
	txCount int
	envs    iter.Seq2[xdr.TransactionEnvelopeView, error]
	// findByHash scans TxProcessing with the scanner idiom (no closures per
	// element) for the element whose outer or fee-bump inner hash matches,
	// returning its parts, outer hash, and apply index (-1 = not found).
	findByHash func(hash xdr.Hash) (txResultParts, xdr.Hash, int, error)
	// stepEnvelopes walks the TxSet's envelopes UNSIZED in agreed-set order:
	// visit returns each envelope's consumed size (from HashSized), which is
	// the walk's only advance.
	stepEnvelopes func(visit func(env xdr.TransactionEnvelopeView) (int, bool, error)) error
}

// TransactionResultPair wire layout constants for the by-hash scan's
// direct-offset reads (the spike-validated ResultPair technique). They are
// safe ONLY on a pair whose extent the thin engine already sized (the
// scanner sizes the whole element before Cur()): sizing validated that the
// result code exists and, for the fee-bump codes, that the inner result
// pair (whose first field is the inner hash) is present. The layout is
// pinned against the schema by TestResultPairDirectOffsets — a schema
// change breaks that test loudly, not these reads silently.
const (
	pairOffHash      = 0  // TransactionHash: opaque[32]
	pairOffCode      = 40 // TransactionResult.Result discriminant (after FeeCharged int64 at 32)
	pairOffInnerHash = 44 // InnerTransactionResultPair.TransactionHash (fee-bump arms)
)

// matchTxHashesRaw reads (outer hash, match) off a SIZED element's raw bytes
// at base = the pair's offset within the element, comparing against target
// (fee-bumps match either their outer or inner hash).
func matchTxHashesRaw(elemRaw []byte, base int, target xdr.Hash) (xdr.Hash, bool) {
	h := xdr.Hash(elemRaw[base+pairOffHash : base+pairOffHash+32])
	if h == target {
		return h, true
	}
	code := int32(binary.BigEndian.Uint32(elemRaw[base+pairOffCode : base+pairOffCode+4]))
	if code == int32(xdr.TransactionResultCodeTxFeeBumpInnerSuccess) ||
		code == int32(xdr.TransactionResultCodeTxFeeBumpInnerFailed) {
		if xdr.Hash(elemRaw[base+pairOffInnerHash:base+pairOffInnerHash+32]) == target {
			return h, true
		}
	}
	return h, false
}

// dispatchLCMView opens lcm, reads its discriminator, and returns the
// version-agnostic handles. This is the one place the V0/V1/V2 LCM dispatch
// lives; every view extractor starts here, so none of them branch on the LCM
// version themselves (version-specific behavior, such as V0 ledgers carrying no
// contract events, falls out of the per-version handles resolved here).
// Deliberately unexported: the public surface is the complete extractors
// (ExtractTxHashes, ExtractLedgerEvents, LedgerTransactionViewByHash/Range);
// nothing outside the package needs the navigation scaffolding, and keeping it
// private keeps iter.Seq2 and the txResultParts projection out of public
// signatures.
//
// Per version the field bundle is read in wire order: TxSet first, then
// TxProcessing — resolving TxProcessing's offset is what sizes the TxSet, so
// this is one pass over the LCM prefix, not two.
func dispatchLCMView(lcm xdr.LedgerCloseMetaView) (lcmViewDispatch, error) {
	disc, err := lcm.V()
	if err != nil {
		return lcmViewDispatch{}, fmt.Errorf("ingest: LCM.V: %w", err)
	}

	d := lcmViewDispatch{lcm: lcm}
	switch disc {
	case 0:
		v0, err := lcm.ArmV0()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: LCM V0: %w", err)
		}
		ts, err := v0.TxSet()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V0 TxSet: %w", err)
		}
		raw, err := v0.TxProcessing()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V0 TxProcessing: %w", err)
		}
		n, err := raw.Len()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V0 TxProcessing count: %w", err)
		}
		d.txCount = int(n)
		d.tp = txPartsSeq(raw.All())
		d.findByHash = func(target xdr.Hash) (txResultParts, xdr.Hash, int, error) {
			sc := raw.Scan()
			for idx := 0; sc.Next(); idx++ {
				elem := sc.Cur()
				h, ok := matchTxHashesRaw(elem.MustRaw(), 0, target)
				if !ok {
					continue
				}
				res, err := elem.Result()
				if err != nil {
					return txResultParts{}, xdr.Hash{}, -1, err
				}
				meta, err := elem.TxApplyProcessing()
				if err != nil {
					return txResultParts{}, xdr.Hash{}, -1, err
				}
				return txResultParts{Result: res, TxApplyProcessing: meta}, h, idx, nil
			}
			return txResultParts{}, xdr.Hash{}, -1, sc.Err()
		}
		d.envs = v0TxSetEnvelopes(ts)
		d.stepEnvelopes = func(visit func(env xdr.TransactionEnvelopeView) (int, bool, error)) error {
			return stepTxSetEnvelopes(ts, visit)
		}
	case 1:
		v1, err := lcm.ArmV1()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: LCM V1: %w", err)
		}
		ts, err := v1.TxSet()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V1 TxSet: %w", err)
		}
		raw, err := v1.TxProcessing()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V1 TxProcessing: %w", err)
		}
		n, err := raw.Len()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V1 TxProcessing count: %w", err)
		}
		d.txCount = int(n)
		d.tp = txPartsSeq(raw.All())
		d.findByHash = func(target xdr.Hash) (txResultParts, xdr.Hash, int, error) {
			sc := raw.Scan()
			for idx := 0; sc.Next(); idx++ {
				elem := sc.Cur()
				h, ok := matchTxHashesRaw(elem.MustRaw(), 0, target)
				if !ok {
					continue
				}
				res, err := elem.Result()
				if err != nil {
					return txResultParts{}, xdr.Hash{}, -1, err
				}
				meta, err := elem.TxApplyProcessing()
				if err != nil {
					return txResultParts{}, xdr.Hash{}, -1, err
				}
				return txResultParts{Result: res, TxApplyProcessing: meta}, h, idx, nil
			}
			return txResultParts{}, xdr.Hash{}, -1, sc.Err()
		}
		d.envs = generalizedEnvelopes("V1", ts)
		d.stepEnvelopes = func(visit func(env xdr.TransactionEnvelopeView) (int, bool, error)) error {
			return stepGeneralizedEnvelopes(ts, visit)
		}
	case 2:
		v2, err := lcm.ArmV2()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: LCM V2: %w", err)
		}
		ts, err := v2.TxSet()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V2 TxSet: %w", err)
		}
		raw, err := v2.TxProcessing()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V2 TxProcessing: %w", err)
		}
		n, err := raw.Len()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V2 TxProcessing count: %w", err)
		}
		d.txCount = int(n)
		d.tp = txPartsSeqV1(raw.All())
		d.findByHash = func(target xdr.Hash) (txResultParts, xdr.Hash, int, error) {
			sc := raw.Scan()
			for idx := 0; sc.Next(); idx++ {
				elem := sc.Cur()
				h, ok := matchTxHashesRaw(elem.MustRaw(), 4, target)
				if !ok {
					continue
				}
				res, err := elem.Result()
				if err != nil {
					return txResultParts{}, xdr.Hash{}, -1, err
				}
				meta, err := elem.TxApplyProcessing()
				if err != nil {
					return txResultParts{}, xdr.Hash{}, -1, err
				}
				return txResultParts{Result: res, TxApplyProcessing: meta}, h, idx, nil
			}
			return txResultParts{}, xdr.Hash{}, -1, sc.Err()
		}
		d.envs = generalizedEnvelopes("V2", ts)
		d.stepEnvelopes = func(visit func(env xdr.TransactionEnvelopeView) (int, bool, error)) error {
			return stepGeneralizedEnvelopes(ts, visit)
		}
	default:
		return lcmViewDispatch{}, fmt.Errorf("ingest: unknown LCM V=%d", disc)
	}
	return d, nil
}

// Header returns (LedgerSequence, LedgerCloseTime), delegating to the xdr
// package's LedgerCloseMetaView helpers so the V0/V1/V2 header navigation
// lives in one place (xdr/ledger_close_meta_view.go) — a new LCM version is
// added to that switch once, not re-implemented here.
func (d lcmViewDispatch) Header() (ledgerSeq uint32, closeTime int64, err error) {
	seq, err := d.lcm.LedgerSequence()
	if err != nil {
		return 0, 0, fmt.Errorf("ingest: ledger header: %w", err)
	}
	ct, err := d.lcm.LedgerCloseTime()
	if err != nil {
		return 0, 0, fmt.Errorf("ingest: ledger header: %w", err)
	}
	return seq, ct, nil
}

// TxProcessing returns the TxProcessing sequence in apply order as concrete
// txResultParts (Result + TxApplyProcessing located per element in wire order).
func (d lcmViewDispatch) TxProcessing() iter.Seq2[txResultParts, error] {
	return d.tp
}

// Envelopes enumerates the TxSet's transaction envelopes in agreed-set order
// (NOT apply order). Consumers pair to TxProcessing entries by hash and may
// break early once every wanted hash is resolved. The yielded views alias the
// LCM buffer (zero-copy).
func (d lcmViewDispatch) Envelopes() iter.Seq2[xdr.TransactionEnvelopeView, error] {
	return d.envs
}

// generalizedEnvelopes enumerates every transaction envelope of an LCM version
// whose TxSet is a GeneralizedTransactionSet (phases -> components/clusters ->
// txs), in agreed-set order (NOT apply order; pairing is by hash, so order is
// irrelevant). The nested loops mirror the wire nesting exactly; a malformed
// input surfaces once, in-band, and stops the walk. An unknown phase
// discriminant is surfaced as an error. label tags the LCM version (V1 and V2
// differ only in where the TxSet handle comes from).
func generalizedEnvelopes(label string, ts xdr.GeneralizedTransactionSetView) iter.Seq2[xdr.TransactionEnvelopeView, error] {
	fail := func(yield func(xdr.TransactionEnvelopeView, error) bool, err error) {
		yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: %s envelopes: %w", label, err))
	}
	return func(yield func(xdr.TransactionEnvelopeView, error) bool) {
		v1ts, err := ts.ArmV1TxSet()
		if err != nil {
			fail(yield, err)
			return
		}
		phases, err := v1ts.Phases()
		if err != nil {
			fail(yield, err)
			return
		}
		for phase, err := range phases.All() {
			if err != nil {
				fail(yield, err)
				return
			}
			v, err := phase.V()
			if err != nil {
				fail(yield, err)
				return
			}
			switch v {
			case 0: // V0 components: one fee group per component.
				comps, err := phase.ArmV0Components()
				if err != nil {
					fail(yield, err)
					return
				}
				for comp, err := range comps.All() {
					if err != nil {
						fail(yield, err)
						return
					}
					disc, err := comp.ArmTxsMaybeDiscountedFee()
					if err != nil {
						fail(yield, err)
						return
					}
					txs, err := disc.Txs()
					if err != nil {
						fail(yield, err)
						return
					}
					for env, err := range txs.All() {
						if err != nil {
							fail(yield, err)
							return
						}
						if !yield(env, nil) {
							return
						}
					}
				}
			case 1: // parallel txs: stages -> clusters -> txs.
				pc, err := phase.ArmParallelTxsComponent()
				if err != nil {
					fail(yield, err)
					return
				}
				stages, err := pc.ExecutionStages()
				if err != nil {
					fail(yield, err)
					return
				}
				for stage, err := range stages.All() {
					if err != nil {
						fail(yield, err)
						return
					}
					for cluster, err := range stage.All() {
						if err != nil {
							fail(yield, err)
							return
						}
						for env, err := range cluster.All() {
							if err != nil {
								fail(yield, err)
								return
							}
							if !yield(env, nil) {
								return
							}
						}
					}
				}
			default:
				yield(xdr.TransactionEnvelopeView{},
					fmt.Errorf("ingest: %s unknown TransactionPhase V=%d", label, v))
				return
			}
		}
	}
}

// v0TxSetEnvelopes enumerates every envelope of a V0 plain TransactionSet, in
// agreed-set order (NOT apply order; pairing is by hash, so order is
// irrelevant).
func v0TxSetEnvelopes(ts xdr.TransactionSetView) iter.Seq2[xdr.TransactionEnvelopeView, error] {
	return func(yield func(xdr.TransactionEnvelopeView, error) bool) {
		txs, err := ts.Txs()
		if err != nil {
			yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: V0 envelopes: %w", err))
			return
		}
		for env, err := range txs.All() {
			if err != nil {
				yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: V0 envelopes: %w", err))
				return
			}
			if !yield(env, nil) {
				return
			}
		}
	}
}

// txProcessingHash extracts the 32-byte TransactionHash from a TxProcessing
// entry's projected parts (TransactionResultPair.TransactionHash — the pair's
// first field, so the read is O(1)).
func txProcessingHash(parts txResultParts) (xdr.Hash, error) {
	hv, err := parts.Result.TransactionHash()
	if err != nil {
		return xdr.Hash{}, fmt.Errorf("ingest: tx hash: %w", err)
	}
	h, err := hv.Value()
	if err != nil {
		return xdr.Hash{}, fmt.Errorf("ingest: tx hash: %w", err)
	}
	return h, nil
}

// txProcessingHashes extracts a TxProcessing entry's transaction hash and, for
// a fee-bump entry (feeBump true), the inner transaction's hash from its
// result. Reads follow wire order: hash, then the result union's code, then —
// for the fee-bump codes — the inner result pair's hash.
func txProcessingHashes(parts txResultParts) (h, inner xdr.Hash, feeBump bool, err error) {
	fail := func(err error) (xdr.Hash, xdr.Hash, bool, error) {
		return xdr.Hash{}, xdr.Hash{}, false, fmt.Errorf("ingest: tx hashes: %w", err)
	}
	hv, err := parts.Result.TransactionHash()
	if err != nil {
		return fail(err)
	}
	if h, err = hv.Value(); err != nil {
		return fail(err)
	}
	rv, err := parts.Result.Result()
	if err != nil {
		return fail(err)
	}
	res, err := rv.Result()
	if err != nil {
		return fail(err)
	}
	code, err := res.Code()
	if err != nil {
		return fail(err)
	}
	switch code {
	case xdr.TransactionResultCodeTxFeeBumpInnerSuccess,
		xdr.TransactionResultCodeTxFeeBumpInnerFailed:
		pair, err := res.ArmInnerResultPair()
		if err != nil {
			return fail(err)
		}
		ihv, err := pair.TransactionHash()
		if err != nil {
			return fail(err)
		}
		if inner, err = ihv.Value(); err != nil {
			return fail(err)
		}
		feeBump = true
	}
	return h, inner, feeBump, nil
}

// ---------------------------------------------------------------------------
// Unsized envelope stepping (the by-hash search path).
// ---------------------------------------------------------------------------

// TxSet wire-shape constants for the unsized envelope walk: each is a count
// or discriminant the stepper reads directly from Rest() bytes, advancing
// through envelopes purely by the hasher's consumed size (one traversal
// shared between hashing and iteration). Every read is bounds-checked; the
// shape is pinned against the sized iteration by
// TestUnsizedEnvelopeStep_MatchesSizedIteration.
const (
	phaseDiscV0Components = 0
	phaseDiscParallel     = 1
	compDiscTxsMaybeFee   = 0
)

// readU32 reads a big-endian uint32 at off, bounds-checked.
func readU32(d []byte, off int64) (uint32, int64, error) {
	if off+4 > int64(len(d)) {
		return 0, 0, fmt.Errorf("ingest: txset: need 4 bytes at %d, have %d", off, len(d))
	}
	return binary.BigEndian.Uint32(d[off : off+4]), off + 4, nil
}

// stepEnvelopes advances through n envelopes packed at d[off:], calling visit
// with an UNSIZED view of each; visit returns the envelope's consumed size
// (from the hasher) and whether to stop. Returns the offset past the last
// envelope stepped.
func stepEnvelopes(d []byte, off int64, n uint32, visit func(env xdr.TransactionEnvelopeView) (int, bool, error)) (int64, bool, error) {
	for k := uint32(0); k < n; k++ {
		if off > int64(len(d)) {
			return 0, false, fmt.Errorf("ingest: txset: envelope offset %d beyond %d", off, len(d))
		}
		consumed, stop, err := visit(xdr.ParseTransactionEnvelopeView(d[off:]))
		if err != nil {
			return 0, false, err
		}
		off += int64(consumed)
		if stop {
			return off, true, nil
		}
	}
	return off, false, nil
}

// stepTxSetEnvelopes walks every envelope of a V0 plain TransactionSet
// unsized (agreed-set order).
func stepTxSetEnvelopes(ts xdr.TransactionSetView, visit func(env xdr.TransactionEnvelopeView) (int, bool, error)) error {
	txs, err := ts.Txs()
	if err != nil {
		return err
	}
	n, err := txs.Len()
	if err != nil {
		return err
	}
	sc := txs.Scan()
	_, _, err = stepEnvelopes(sc.Rest(), 0, n, visit)
	return err
}

// stepGeneralizedEnvelopes walks every envelope of a GeneralizedTransactionSet
// unsized: phases -> components/clusters -> txs, all counts read directly,
// envelopes advanced by visit's consumed size.
func stepGeneralizedEnvelopes(ts xdr.GeneralizedTransactionSetView, visit func(env xdr.TransactionEnvelopeView) (int, bool, error)) error {
	v1ts, err := ts.ArmV1TxSet()
	if err != nil {
		return err
	}
	phases, err := v1ts.Phases()
	if err != nil {
		return err
	}
	nPhases, err := phases.Len()
	if err != nil {
		return err
	}
	sc := phases.Scan()
	d := sc.Rest()
	off := int64(0)
	for p := uint32(0); p < nPhases; p++ {
		disc, next, err := readU32(d, off)
		if err != nil {
			return err
		}
		off = next
		switch int32(disc) {
		case phaseDiscV0Components:
			nComps, next, err := readU32(d, off)
			if err != nil {
				return err
			}
			off = next
			for c := uint32(0); c < nComps; c++ {
				compDisc, next, err := readU32(d, off)
				if err != nil {
					return err
				}
				off = next
				if int32(compDisc) != compDiscTxsMaybeFee {
					return fmt.Errorf("ingest: txset: unknown TxSetComponent type %d", int32(compDisc))
				}
				// Optional base fee: flag, then 8-byte Int64 when present.
				flag, next, err := readU32(d, off)
				if err != nil {
					return err
				}
				off = next
				switch flag {
				case 0:
				case 1:
					off += 8
				default:
					return fmt.Errorf("ingest: txset: bad optional flag %d", flag)
				}
				nTxs, next, err := readU32(d, off)
				if err != nil {
					return err
				}
				off = next
				newOff, stopped, err := stepEnvelopes(d, off, nTxs, visit)
				if err != nil {
					return err
				}
				if stopped {
					return nil
				}
				off = newOff
			}
		case phaseDiscParallel:
			// ParallelTxsComponent: optional base fee, then stages -> clusters -> txs.
			flag, next, err := readU32(d, off)
			if err != nil {
				return err
			}
			off = next
			switch flag {
			case 0:
			case 1:
				off += 8
			default:
				return fmt.Errorf("ingest: txset: bad optional flag %d", flag)
			}
			nStages, next, err := readU32(d, off)
			if err != nil {
				return err
			}
			off = next
			for s := uint32(0); s < nStages; s++ {
				nClusters, next, err := readU32(d, off)
				if err != nil {
					return err
				}
				off = next
				for cl := uint32(0); cl < nClusters; cl++ {
					nTxs, next, err := readU32(d, off)
					if err != nil {
						return err
					}
					off = next
					newOff, stopped, err := stepEnvelopes(d, off, nTxs, visit)
					if err != nil {
						return err
					}
					if stopped {
						return nil
					}
					off = newOff
				}
			}
		default:
			return fmt.Errorf("ingest: txset: unknown TransactionPhase V=%d", int32(disc))
		}
	}
	return nil
}
