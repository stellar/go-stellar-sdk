package ingest

import (
	"fmt"
	"iter"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// txResultParts is the concrete per-tx projection of a TxProcessing element: the
// result pair and the apply-processing meta, both located (and trimmed to their
// exact wire extent) in the single Fields() walk that also advances the
// iterator. Because the meta view is trimmed, MetaRaw() is a free slice rather
// than a sizing re-walk.
type txResultParts struct {
	Result            xdr.TransactionResultPairView
	TxApplyProcessing xdr.TransactionMetaView
}

// MetaRaw returns the apply-processing meta's exact wire bytes. The view came
// from Fields() (already trimmed), so this is a plain conversion, not a walk.
func (p txResultParts) MetaRaw() []byte { return []byte(p.TxApplyProcessing) }

// The TxProcessing extractors all walk the per-version TxProcessing array the
// same way, but the array is a different view type in each LCM version: V0 and V1
// hold a TransactionResultMeta element, V2 holds a TransactionResultMetaV1. The
// two walk functions below are that one walk written once per element type, each
// generic over the array view type so the V0 and V1 arrays share a single
// function instead of duplicating it. txMetaArray / txMetaV1Array name what the
// walk requires of an array view:
//
//   - ~[]byte:  the view is a named []byte, which the walk reslices to advance.
//   - Count():  the validated element count, used as the loop bound.
//   - At(int):  At(0) returns the first element. Going through At() keeps the
//     array's wire layout (its length prefix) the view's concern
//     rather than hardcoding an offset here, and fixing the element
//     return type makes that element's Fields() method callable.
type txMetaArray interface {
	~[]byte
	Count() (int, error)
	At(int) (xdr.TransactionResultMetaView, error)
}

// txMetaV1Array is txMetaArray for the V2 array (TransactionResultMetaV1 element).
type txMetaV1Array interface {
	~[]byte
	Count() (int, error)
	At(int) (xdr.TransactionResultMetaV1View, error)
}

// txProcessingPartsMeta walks a V0/V1 TxProcessing array, yielding each element's
// {Result, TxApplyProcessing}. Each element is walked exactly once: Fields()
// locates the element's sub-fields and, as a byproduct, reports the element's
// total wire size as len(f.View); the loop advances to the next element by
// reslicing past it (elem[len(f.View):]). So locating fields and advancing the
// iterator are the same single walk — there is no extra pass just to size each
// element. (Fields() is the generated per-struct accessor that locates every
// field of a node in one pass.)
func txProcessingPartsMeta[A txMetaArray](arr A) iter.Seq2[txResultParts, error] {
	return func(yield func(txResultParts, error) bool) {
		count, err := arr.Count()
		if err != nil {
			yield(txResultParts{}, fmt.Errorf("ingest: TxProcessing count: %w", err))
			return
		}
		if count == 0 {
			return
		}
		elem, err := arr.At(0)
		if err != nil {
			yield(txResultParts{}, fmt.Errorf("ingest: TxProcessing At(0): %w", err))
			return
		}
		for k := 0; k < count; k++ {
			f, ferr := elem.Fields()
			if ferr != nil {
				yield(txResultParts{}, fmt.Errorf("ingest: TxProcessing element %d: %w", k, ferr))
				return
			}
			if !yield(txResultParts{Result: f.Result, TxApplyProcessing: f.TxApplyProcessing}, nil) {
				return
			}
			elem = elem[len(f.View):]
		}
	}
}

// txProcessingPartsMetaV1 is txProcessingPartsMeta for the V2 array; it is a
// separate copy only because its element view type — and therefore its Fields
// bundle type — differs, which a single generic cannot span.
func txProcessingPartsMetaV1[A txMetaV1Array](arr A) iter.Seq2[txResultParts, error] {
	return func(yield func(txResultParts, error) bool) {
		count, err := arr.Count()
		if err != nil {
			yield(txResultParts{}, fmt.Errorf("ingest: TxProcessing count: %w", err))
			return
		}
		if count == 0 {
			return
		}
		elem, err := arr.At(0)
		if err != nil {
			yield(txResultParts{}, fmt.Errorf("ingest: TxProcessing At(0): %w", err))
			return
		}
		for k := 0; k < count; k++ {
			f, ferr := elem.Fields()
			if ferr != nil {
				yield(txResultParts{}, fmt.Errorf("ingest: TxProcessing element %d: %w", k, ferr))
				return
			}
			if !yield(txResultParts{Result: f.Result, TxApplyProcessing: f.TxApplyProcessing}, nil) {
				return
			}
			elem = elem[len(f.View):]
		}
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
func dispatchLCMView(lcm xdr.LedgerCloseMetaView) (lcmViewDispatch, error) {
	disc, err := lcm.V()
	if err != nil {
		return lcmViewDispatch{}, fmt.Errorf("ingest: LCM.V: %w", err)
	}

	d := lcmViewDispatch{lcm: lcm}
	switch disc {
	case 0:
		v0, err := lcm.V0()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: LCM V0: %w", err)
		}
		raw, err := v0.TxProcessing()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V0 TxProcessing: %w", err)
		}
		if d.txCount, err = raw.Count(); err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V0 TxProcessing count: %w", err)
		}
		d.tp = txProcessingPartsMeta(raw)
		d.envs = v0TxSetEnvelopes(v0.TxSet)
	case 1:
		v1, err := lcm.V1()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: LCM V1: %w", err)
		}
		raw, err := v1.TxProcessing()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V1 TxProcessing: %w", err)
		}
		if d.txCount, err = raw.Count(); err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V1 TxProcessing count: %w", err)
		}
		d.tp = txProcessingPartsMeta(raw)
		d.envs = generalizedEnvelopes("V1", v1.TxSet)
	case 2:
		v2, err := lcm.V2()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: LCM V2: %w", err)
		}
		raw, err := v2.TxProcessing()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V2 TxProcessing: %w", err)
		}
		if d.txCount, err = raw.Count(); err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V2 TxProcessing count: %w", err)
		}
		d.tp = txProcessingPartsMetaV1(raw)
		d.envs = generalizedEnvelopes("V2", v2.TxSet)
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
// txResultParts (Result + TxApplyProcessing located per element in one walk).
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
// irrelevant). The whole nested walk is one Must-based traversal under a single
// Try: a malformed-input *xdr.ViewError is recovered and yielded once, a
// consumer break (yield returns false) returns out of the walk cleanly, and an
// unknown phase discriminant is surfaced as an error. label tags the LCM version
// (V1 and V2 differ only in where the TxSet handle comes from).
func generalizedEnvelopes(label string, getTxSet func() (xdr.GeneralizedTransactionSetView, error)) iter.Seq2[xdr.TransactionEnvelopeView, error] {
	return func(yield func(xdr.TransactionEnvelopeView, error) bool) {
		ts, err := getTxSet()
		if err != nil {
			yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: %s TxSet: %w", label, err))
			return
		}
		var unknownPhase int32
		sawUnknownPhase := false
		walkErr := xdr.TryVoid(func() {
			for phase := range ts.MustV1TxSet().MustPhases().MustIter() {
				switch v := phase.MustV(); v {
				case 0: // V0 components: one fee group per component.
					for comp := range phase.MustV0Components().MustIter() {
						for env := range comp.MustTxsMaybeDiscountedFee().MustTxs().MustIter() {
							if !yield(env, nil) {
								return
							}
						}
					}
				case 1: // parallel txs: stages -> clusters -> txs.
					for stage := range phase.MustParallelTxsComponent().MustExecutionStages().MustIter() {
						for cluster := range stage.MustIter() {
							for env := range cluster.MustIter() {
								if !yield(env, nil) {
									return
								}
							}
						}
					}
				default:
					unknownPhase, sawUnknownPhase = v, true
					return
				}
			}
		})
		switch {
		case sawUnknownPhase:
			yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: %s unknown TransactionPhase V=%d", label, unknownPhase))
		case walkErr != nil:
			yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: %s envelopes: %w", label, walkErr))
		}
	}
}

// txProcessingHash extracts the 32-byte TransactionHash from a TxProcessing
// entry's projected parts (TransactionResultPair.TransactionHash). HashView is a
// fixed opaque[32] whose Value() now returns a typed xdr.Hash, so no
// length-dependent conversion is needed.
func txProcessingHash(parts txResultParts) (xdr.Hash, error) {
	h, err := xdr.Try(func() xdr.Hash {
		return parts.Result.MustTransactionHash().MustValue()
	})
	if err != nil {
		return xdr.Hash{}, fmt.Errorf("ingest: tx hash: %w", err)
	}
	return h, nil
}

// txProcessingHashes extracts a TxProcessing entry's transaction hash and, for
// a fee-bump entry (feeBump true), the inner transaction's hash from its result.
func txProcessingHashes(parts txResultParts) (h, inner xdr.Hash, feeBump bool, err error) {
	err = xdr.TryVoid(func() {
		h = parts.Result.MustTransactionHash().MustValue()
		res := parts.Result.MustResult().MustResult()
		switch res.MustCode() {
		case xdr.TransactionResultCodeTxFeeBumpInnerSuccess,
			xdr.TransactionResultCodeTxFeeBumpInnerFailed:
			inner = res.MustInnerResultPair().MustTransactionHash().MustValue()
			feeBump = true
		}
	})
	if err != nil {
		return xdr.Hash{}, xdr.Hash{}, false, fmt.Errorf("ingest: tx hashes: %w", err)
	}
	return h, inner, feeBump, nil
}

// v0TxSetEnvelopes enumerates every envelope of a V0 plain TransactionSet, in
// agreed-set order (NOT apply order; pairing is by hash, so order is
// irrelevant). Same Must-under-Try shape as generalizedEnvelopes.
func v0TxSetEnvelopes(getTxSet func() (xdr.TransactionSetView, error)) iter.Seq2[xdr.TransactionEnvelopeView, error] {
	return func(yield func(xdr.TransactionEnvelopeView, error) bool) {
		ts, err := getTxSet()
		if err != nil {
			yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: V0 TxSet: %w", err))
			return
		}
		if err := xdr.TryVoid(func() {
			for env := range ts.MustTxs().MustIter() {
				if !yield(env, nil) {
					return
				}
			}
		}); err != nil {
			yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: V0 envelopes: %w", err))
		}
	}
}
