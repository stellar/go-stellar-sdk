package ingest

import (
	"fmt"
	"iter"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// lcmViewDispatch holds what the extractors need from one
// xdr.LedgerCloseMetaView: the LCM view itself (for the ledger header), the
// TxProcessing array (apply order), and the version's arm, from which
// Envelopes enumerates the TxSet. The TxSet is in agreed-set order, which
// differs from TxProcessing apply order, so callers pair envelopes to
// transactions by hash, never by position. V0 uses a plain TransactionSet;
// V1/V2 use a GeneralizedTransactionSet.
type lcmViewDispatch struct {
	lcm xdr.LedgerCloseMetaView
	txs txProcessing
	// One of these is set, by LCM version.
	v0 xdr.LedgerCloseMetaV0View
	v1 xdr.LedgerCloseMetaV1View
	v2 xdr.LedgerCloseMetaV2View
}

// dispatchLCMView opens lcm, reads its discriminator, and returns its handles.
// Every view extractor starts here.
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
		if d.txs, err = newTxProcessing(lcm, raw, false); err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V0 TxProcessing: %w", err)
		}
		d.v0 = v0
	case 1:
		v1, err := lcm.V1()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: LCM V1: %w", err)
		}
		raw, err := v1.TxProcessing()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V1 TxProcessing: %w", err)
		}
		if d.txs, err = newTxProcessing(lcm, raw, false); err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V1 TxProcessing: %w", err)
		}
		d.v1 = v1
	case 2:
		v2, err := lcm.V2()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: LCM V2: %w", err)
		}
		raw, err := v2.TxProcessing()
		if err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V2 TxProcessing: %w", err)
		}
		if d.txs, err = newTxProcessing(lcm, raw, true); err != nil {
			return lcmViewDispatch{}, fmt.Errorf("ingest: V2 TxProcessing: %w", err)
		}
		d.v2 = v2
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
	switch {
	case d.v0 != nil:
		return v0TxSetEnvelopes(d.v0.TxSet)
	case d.v1 != nil:
		return generalizedEnvelopes("V1", d.v1.TxSet)
	default:
		return generalizedEnvelopes("V2", d.v2.TxSet)
	}
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
