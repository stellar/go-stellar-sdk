package ingest

import (
	"encoding/binary"
	"fmt"
	"iter"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// txPartViews is the per-tx projection of a TxProcessing element: the result
// pair and the apply-processing meta as tier-1 views, resolved by the
// element's accessors in wire order.
type txPartViews struct {
	Result            xdr.TransactionResultPairView
	TxApplyProcessing xdr.TransactionMetaView
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
	lcm  xdr.LedgerCloseMetaView
	envs iter.Seq2[xdr.TransactionEnvelopeView, error]
	// findByHash scans TxProcessing with the scanner idiom (no closures per
	// element) for the element whose outer or fee-bump inner hash matches,
	// returning its parts, outer hash, and apply index (-1 = not found).
	findByHash func(hash xdr.Hash) (txPartViews, xdr.Hash, int, error)
	// stepEnvelopes walks the TxSet's envelopes UNSIZED in agreed-set order:
	// visit receives the remaining bytes (envelope-first) and returns the
	// envelope's consumed size (from HashSized), the walk's only advance.
	stepEnvelopes func(visit func(rest []byte) (int, bool, error)) error
}

// TransactionResultPair wire layout constants for the by-hash scan's
// direct-offset reads. They are safe ONLY on a pair whose extent the thin
// engine already sized (the scanner sizes the whole element before Cur()):
// sizing validated that the
// result code exists and, for the fee-bump codes, that the inner result
// pair (whose first field is the inner hash) is present. The layout is
// pinned against the schema by TestResultPairDirectOffsets — a schema
// change breaks that test loudly, not these reads silently.
const (
	pairOffHash      = 0  // TransactionHash: opaque[32]
	pairOffResult    = 32 // TransactionResult (the pair is hash ‖ result, both exact)
	pairOffCode      = 40 // TransactionResult.Result discriminant (after FeeCharged int64 at 32)
	pairOffInnerHash = 44 // InnerTransactionResultPair.TransactionHash (fee-bump arms)
	// pairBaseV2Elem is the result pair's offset inside an LCM V2
	// TxProcessing element: TransactionResultMetaV1 leads with an
	// ExtensionPoint, a fixed 4-byte void union (V0/V1 elements start at 0).
	pairBaseV2Elem = 4
)

// pairResultRaw slices a sized result pair's TransactionResult bytes — the
// one shared resolver for pair-layout knowledge on the range and by-hash
// paths.
func pairResultRaw(pairRaw []byte) []byte { return pairRaw[pairOffResult:] }

// txScanner is the element-scanner shape scanForHash consumes (satisfied by
// the generated *XScanner types).
type txScanner[E any] interface {
	Next() bool
	Cur() E
	Err() error
}

// txPartElem is what scanForHash needs of a TxProcessing element view. The
// two element types (TransactionResultMetaView for LCM V0/V1,
// TransactionResultMetaV1View for LCM V2) have identical method sets here,
// so one generic scan serves all three LCM versions.
type txPartElem interface {
	MustRaw() []byte
	Result() (xdr.TransactionResultPairView, error)
	TxApplyProcessing() (xdr.TransactionMetaView, error)
}

// scanForHash walks a TxProcessing scanner for the element whose outer or
// fee-bump inner hash matches target (direct-offset reads at pairBase within
// each sized element), resolving the match's part views. Returns apply index
// -1 when absent.
func scanForHash[E txPartElem](sc txScanner[E], pairBase int, target xdr.Hash) (txPartViews, xdr.Hash, int, error) {
	for idx := 0; sc.Next(); idx++ {
		elem := sc.Cur()
		h, ok := matchTxHashesRaw(elem.MustRaw(), pairBase, target)
		if !ok {
			continue
		}
		res, err := elem.Result()
		if err != nil {
			return txPartViews{}, xdr.Hash{}, -1, fmt.Errorf("ingest: TxProcessing element %d: %w", idx, err)
		}
		meta, err := elem.TxApplyProcessing()
		if err != nil {
			return txPartViews{}, xdr.Hash{}, -1, fmt.Errorf("ingest: TxProcessing element %d: %w", idx, err)
		}
		return txPartViews{Result: res, TxApplyProcessing: meta}, h, idx, nil
	}
	if err := sc.Err(); err != nil {
		return txPartViews{}, xdr.Hash{}, -1, fmt.Errorf("ingest: TxProcessing scan: %w", err)
	}
	return txPartViews{}, xdr.Hash{}, -1, nil
}

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
// private keeps iter.Seq2 and the txPartViews projection out of public
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
		d.findByHash = func(target xdr.Hash) (txPartViews, xdr.Hash, int, error) {
			sc := raw.Scan()
			return scanForHash(&sc, 0, target)
		}
		d.envs = v0TxSetEnvelopes(ts)
		d.stepEnvelopes = func(visit func(rest []byte) (int, bool, error)) error {
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
		d.findByHash = func(target xdr.Hash) (txPartViews, xdr.Hash, int, error) {
			sc := raw.Scan()
			return scanForHash(&sc, 0, target)
		}
		d.envs = generalizedEnvelopes("V1", ts)
		d.stepEnvelopes = func(visit func(rest []byte) (int, bool, error)) error {
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
		d.findByHash = func(target xdr.Hash) (txPartViews, xdr.Hash, int, error) {
			sc := raw.Scan()
			return scanForHash(&sc, pairBaseV2Elem, target)
		}
		d.envs = generalizedEnvelopes("V2", ts)
		d.stepEnvelopes = func(visit func(rest []byte) (int, bool, error)) error {
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
		psc := phases.Scan()
		for psc.Next() {
			phase := psc.Cur()
			v, err := phase.V()
			if err != nil {
				fail(yield, err)
				return
			}
			switch v {
			case phaseDiscV0Components: // one fee group per component.
				comps, err := phase.ArmV0Components()
				if err != nil {
					fail(yield, err)
					return
				}
				csc := comps.Scan()
				for csc.Next() {
					comp := csc.Cur()
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
					tsc := txs.Scan()
					for tsc.Next() {
						if !yield(tsc.Cur(), nil) {
							return
						}
					}
					if err := tsc.Err(); err != nil {
						fail(yield, err)
						return
					}
				}
				if err := csc.Err(); err != nil {
					fail(yield, err)
					return
				}
			case phaseDiscParallel: // parallel txs: stages -> clusters -> txs.
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
				ssc := stages.Scan()
				for ssc.Next() {
					stage := ssc.Cur()
					clsc := stage.Scan()
					for clsc.Next() {
						cluster := clsc.Cur()
						esc := cluster.Scan()
						for esc.Next() {
							if !yield(esc.Cur(), nil) {
								return
							}
						}
						if err := esc.Err(); err != nil {
							fail(yield, err)
							return
						}
					}
					if err := clsc.Err(); err != nil {
						fail(yield, err)
						return
					}
				}
				if err := ssc.Err(); err != nil {
					fail(yield, err)
					return
				}
			default:
				yield(xdr.TransactionEnvelopeView{},
					fmt.Errorf("ingest: %s unknown TransactionPhase V=%d", label, v))
				return
			}
		}
		if err := psc.Err(); err != nil {
			fail(yield, err)
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
		sc := txs.Scan()
		for sc.Next() {
			if !yield(sc.Cur(), nil) {
				return
			}
		}
		if err := sc.Err(); err != nil {
			yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: V0 envelopes: %w", err))
		}
	}
}

// txProcessingHashes extracts a TxProcessing entry's transaction hash and, for
// a fee-bump entry (feeBump true), the inner transaction's hash from its
// result. Reads follow wire order: hash, then the result union's code, then —
// for the fee-bump codes — the inner result pair's hash.
func txProcessingHashes(parts txPartViews) (h, inner xdr.Hash, feeBump bool, err error) {
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

// TxSet wire-shape constants for the unsized envelope walk. The phase and
// component discriminants reuse the generated schema values; the shape is
// pinned against the sized iteration by
// TestUnsizedEnvelopeStep_MatchesSizedIteration.
const (
	phaseDiscV0Components = 0
	phaseDiscParallel     = 1
)

// wireCursor steps through untrusted bytes with a sticky error: every read
// is bounds-checked once, and the first failure poisons the cursor.
type wireCursor struct {
	d   []byte
	off int64
	err error
}

func (c *wireCursor) u32(what string) uint32 {
	if c.err != nil {
		return 0
	}
	if c.off+4 > int64(len(c.d)) {
		c.err = fmt.Errorf("ingest: txset: need 4 bytes for %s at %d, have %d", what, c.off, len(c.d))
		return 0
	}
	v := binary.BigEndian.Uint32(c.d[c.off : c.off+4])
	c.off += 4
	return v
}

// skipOptionalInt64 advances past an optional Int64 (flag, then 8 bytes when
// present) — the base-fee shape both component kinds share.
func (c *wireCursor) skipOptionalInt64(what string) {
	switch flag := c.u32(what); {
	case c.err != nil:
	case flag == 0:
	case flag == 1:
		c.off += 8
	default:
		c.err = fmt.Errorf("ingest: txset: bad optional flag %d for %s", flag, what)
	}
}

// advanceEnvelopes advances the cursor through n envelopes, calling visit
// with the remaining bytes at each; visit returns the envelope's consumed
// size (the walk's only advance) and whether to stop.
func (c *wireCursor) advanceEnvelopes(n uint32, visit func(rest []byte) (int, bool, error)) (stopped bool) {
	for k := uint32(0); k < n && c.err == nil; k++ {
		if c.off > int64(len(c.d)) {
			c.err = fmt.Errorf("ingest: txset: envelope offset %d beyond %d", c.off, len(c.d))
			return false
		}
		consumed, stop, err := visit(c.d[c.off:])
		if err != nil {
			c.err = err
			return false
		}
		c.off += int64(consumed)
		if stop {
			return true
		}
	}
	return false
}

// stepTxSetEnvelopes walks every envelope of a V0 plain TransactionSet
// unsized (agreed-set order).
func stepTxSetEnvelopes(ts xdr.TransactionSetView, visit func(rest []byte) (int, bool, error)) error {
	txs, err := ts.Txs()
	if err != nil {
		return err
	}
	n, err := txs.Len()
	if err != nil {
		return err
	}
	sc := txs.Scan()
	c := wireCursor{d: sc.Rest()}
	c.advanceEnvelopes(n, visit)
	return c.err
}

// stepGeneralizedEnvelopes walks every envelope of a GeneralizedTransactionSet
// unsized: phases -> components/clusters -> txs, all counts read through a
// bounds-checked cursor, envelopes advanced by visit's consumed size.
func stepGeneralizedEnvelopes(ts xdr.GeneralizedTransactionSetView, visit func(rest []byte) (int, bool, error)) error {
	v1ts, err := ts.ArmV1TxSet()
	if err != nil {
		return fmt.Errorf("ingest: txset: %w", err)
	}
	phases, err := v1ts.Phases()
	if err != nil {
		return fmt.Errorf("ingest: txset: %w", err)
	}
	nPhases, err := phases.Len()
	if err != nil {
		return fmt.Errorf("ingest: txset: %w", err)
	}
	sc := phases.Scan()
	c := wireCursor{d: sc.Rest()}
	for p := uint32(0); p < nPhases && c.err == nil; p++ {
		switch disc := c.u32("phase discriminant"); int32(disc) {
		case phaseDiscV0Components:
			nComps := c.u32("component count")
			for ci := uint32(0); ci < nComps && c.err == nil; ci++ {
				if compDisc := c.u32("component type"); c.err == nil &&
					int32(compDisc) != int32(xdr.TxSetComponentTypeTxsetCompTxsMaybeDiscountedFee) {
					return fmt.Errorf("ingest: txset: unknown TxSetComponent type %d", int32(compDisc))
				}
				c.skipOptionalInt64("base fee")
				if c.advanceEnvelopes(c.u32("tx count"), visit) {
					return nil
				}
			}
		case phaseDiscParallel:
			c.skipOptionalInt64("base fee")
			nStages := c.u32("stage count")
			for si := uint32(0); si < nStages && c.err == nil; si++ {
				nClusters := c.u32("cluster count")
				for cl := uint32(0); cl < nClusters && c.err == nil; cl++ {
					if c.advanceEnvelopes(c.u32("tx count"), visit) {
						return nil
					}
				}
			}
		default:
			if c.err == nil {
				return fmt.Errorf("ingest: txset: unknown TransactionPhase V=%d", int32(disc))
			}
		}
	}
	return c.err
}
