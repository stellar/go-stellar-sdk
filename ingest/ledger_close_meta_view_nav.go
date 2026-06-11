package ingest

import (
	"fmt"
	"iter"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// TxResultMetaView is the view-side interface satisfied by both
// xdr.TransactionResultMetaView (V0/V1 LCM TxProcessing element) and
// xdr.TransactionResultMetaV1View (V2 LCM TxProcessing element), letting
// V0/V1/V2 share one TxProcessing iteration body.
type TxResultMetaView interface {
	Result() (xdr.TransactionResultPairView, error)
	MustResult() xdr.TransactionResultPairView
	TxApplyProcessing() (xdr.TransactionMetaView, error)
}

// widenTxResultMeta adapts a concrete per-version TxProcessing sequence into a
// sequence of the TxResultMetaView interface.
func widenTxResultMeta[E TxResultMetaView](seq iter.Seq2[E, error]) iter.Seq2[TxResultMetaView, error] {
	return func(yield func(TxResultMetaView, error) bool) {
		for elem, err := range seq {
			if !yield(elem, err) {
				return
			}
		}
	}
}

// LedgerCloseMetaViewDispatch holds the version-specific handles the extractors
// need from one xdr.LedgerCloseMetaView: the version discriminant, the ledger
// header view, the TxProcessing sequence (apply order), and an enumerator over
// the version-specific TxSet's transaction envelopes. The TxSet is in
// agreed-set / hash-sorted order — which differs from TxProcessing apply order
// — so callers pair envelopes to transactions BY HASH, never by array
// position. V0 uses a plain TransactionSet; V1/V2 use a
// GeneralizedTransactionSet.
type LedgerCloseMetaViewDispatch struct {
	Version int32

	lcm  xdr.LedgerCloseMetaView
	tp   iter.Seq2[TxResultMetaView, error]
	envs iter.Seq2[xdr.TransactionEnvelopeView, error]
}

// DispatchLedgerCloseMetaView opens lcm, reads its discriminator, and returns
// the version-specific handles. This is the one place the V0/V1/V2 LCM dispatch
// lives; every view extractor starts here and branches on Version for
// version-sensitive behavior (e.g. V0 ledgers carry no contract events).
func DispatchLedgerCloseMetaView(lcm xdr.LedgerCloseMetaView) (LedgerCloseMetaViewDispatch, error) {
	dv, err := lcm.V()
	if err != nil {
		return LedgerCloseMetaViewDispatch{}, fmt.Errorf("ingest: LCM.V: %w", err)
	}
	disc, err := dv.Value()
	if err != nil {
		return LedgerCloseMetaViewDispatch{}, fmt.Errorf("ingest: LCM.V value: %w", err)
	}

	d := LedgerCloseMetaViewDispatch{Version: disc, lcm: lcm}
	switch disc {
	case 0:
		v0, err := lcm.V0()
		if err != nil {
			return LedgerCloseMetaViewDispatch{}, fmt.Errorf("ingest: LCM V0: %w", err)
		}
		raw, err := v0.TxProcessing()
		if err != nil {
			return LedgerCloseMetaViewDispatch{}, fmt.Errorf("ingest: V0 TxProcessing: %w", err)
		}
		d.tp = widenTxResultMeta(raw.Iter())
		d.envs = func(yield func(xdr.TransactionEnvelopeView, error) bool) {
			txSet, err := v0.TxSet()
			if err != nil {
				yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: V0 TxSet: %w", err))
				return
			}
			enumerateEnvelopesFromV0TxSet(txSet, yield)
		}
	case 1:
		v1, err := lcm.V1()
		if err != nil {
			return LedgerCloseMetaViewDispatch{}, fmt.Errorf("ingest: LCM V1: %w", err)
		}
		raw, err := v1.TxProcessing()
		if err != nil {
			return LedgerCloseMetaViewDispatch{}, fmt.Errorf("ingest: V1 TxProcessing: %w", err)
		}
		d.tp = widenTxResultMeta(raw.Iter())
		d.envs = generalizedEnvs("V1", v1.TxSet)
	case 2:
		v2, err := lcm.V2()
		if err != nil {
			return LedgerCloseMetaViewDispatch{}, fmt.Errorf("ingest: LCM V2: %w", err)
		}
		raw, err := v2.TxProcessing()
		if err != nil {
			return LedgerCloseMetaViewDispatch{}, fmt.Errorf("ingest: V2 TxProcessing: %w", err)
		}
		d.tp = widenTxResultMeta(raw.Iter())
		d.envs = generalizedEnvs("V2", v2.TxSet)
	default:
		return LedgerCloseMetaViewDispatch{}, fmt.Errorf("ingest: unknown LCM V=%d", disc)
	}
	return d, nil
}

// Header returns (LedgerSequence, LedgerCloseTime), delegating to the xdr
// package's LedgerCloseMetaView helpers so the V0/V1/V2 header navigation
// lives in one place (xdr/ledger_close_meta_view.go) — a new LCM version is
// added to that switch once, not re-implemented here.
func (d LedgerCloseMetaViewDispatch) Header() (ledgerSeq uint32, closeTime int64, err error) {
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

// TxProcessing returns the TxProcessing sequence in apply order. Every LCM
// version's element satisfies TxResultMetaView.
func (d LedgerCloseMetaViewDispatch) TxProcessing() iter.Seq2[TxResultMetaView, error] {
	return d.tp
}

// Envelopes enumerates the TxSet's transaction envelopes in agreed-set order
// (NOT apply order). Consumers pair to TxProcessing entries by hash and may
// break early once every wanted hash is resolved. The yielded views alias the
// LCM buffer (zero-copy).
func (d LedgerCloseMetaViewDispatch) Envelopes() iter.Seq2[xdr.TransactionEnvelopeView, error] {
	return d.envs
}

// generalizedEnvs builds the envelope enumerator for an LCM version whose
// TxSet is a GeneralizedTransactionSet (V1 and V2 differ only in where the
// TxSet handle comes from). label tags errors with the LCM version.
func generalizedEnvs(label string, txSet func() (xdr.GeneralizedTransactionSetView, error)) iter.Seq2[xdr.TransactionEnvelopeView, error] {
	return func(yield func(xdr.TransactionEnvelopeView, error) bool) {
		ts, err := txSet()
		if err != nil {
			yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: %s TxSet: %w", label, err))
			return
		}
		enumerateEnvelopesFromGeneralized(ts, yield)
	}
}

// TxProcessingHash extracts the 32-byte TransactionHash from a TxProcessing
// entry view (TransactionResultPair.TransactionHash). HashView is a fixed
// opaque[32], so the value is always exactly 32 bytes on success.
func TxProcessingHash(tx TxResultMetaView) (xdr.Hash, error) {
	hb, err := xdr.Try(func() []byte {
		return tx.MustResult().MustTransactionHash().MustValue()
	})
	if err != nil {
		return xdr.Hash{}, fmt.Errorf("ingest: tx hash: %w", err)
	}
	return xdr.Hash(hb), nil
}

// enumerateEnvelopesFromV0TxSet yields every envelope of a V0 TransactionSet.
// The order is agreed-set order (NOT apply order); pairing is by hash so order
// is irrelevant. Stops if the consumer breaks.
func enumerateEnvelopesFromV0TxSet(txSet xdr.TransactionSetView, yield func(xdr.TransactionEnvelopeView, error) bool) {
	txs, err := txSet.Txs()
	if err != nil {
		yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: V0 TxSet.Txs: %w", err))
		return
	}
	i := 0
	for env, eerr := range txs.Iter() {
		if eerr != nil {
			yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: V0 envelope at %d: %w", i, eerr))
			return
		}
		if !yield(env, nil) {
			return
		}
		i++
	}
}

// enumerateEnvelopesFromGeneralized yields every envelope of a V1/V2
// GeneralizedTransactionSet (phases -> components/clusters -> txs). Stops if the
// consumer breaks.
func enumerateEnvelopesFromGeneralized(txSet xdr.GeneralizedTransactionSetView, yield func(xdr.TransactionEnvelopeView, error) bool) {
	v1Set, err := txSet.V1TxSet()
	if err != nil {
		yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: V1TxSet: %w", err))
		return
	}
	phases, err := v1Set.Phases()
	if err != nil {
		yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: Phases: %w", err))
		return
	}
	for phase, perr := range phases.Iter() {
		if perr != nil {
			yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: phase iter: %w", perr))
			return
		}
		pv, err := phase.V()
		if err != nil {
			yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: phase.V: %w", err))
			return
		}
		pDisc, err := pv.Value()
		if err != nil {
			yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: phase.V value: %w", err))
			return
		}
		switch pDisc {
		case 0:
			comps, err := phase.V0Components()
			if err != nil {
				yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: V0Components: %w", err))
				return
			}
			if !enumerateV0Components(comps, yield) {
				return
			}
		case 1:
			ptx, err := phase.ParallelTxsComponent()
			if err != nil {
				yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: ParallelTxsComponent: %w", err))
				return
			}
			if !enumerateParallelTxs(ptx, yield) {
				return
			}
		default:
			yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: unknown TransactionPhase V=%d", pDisc))
			return
		}
	}
}

// enumerateV0Components yields every envelope in V0-style phase components (one
// component per fee group). Returns false if the consumer broke or an error was
// yielded (so the caller stops too).
func enumerateV0Components(comps xdr.TransactionPhaseV0ComponentsView, yield func(xdr.TransactionEnvelopeView, error) bool) bool {
	for comp, cerr := range comps.Iter() {
		if cerr != nil {
			yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: component iter: %w", cerr))
			return false
		}
		tdf, err := comp.TxsMaybeDiscountedFee()
		if err != nil {
			yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: TxsMaybeDiscountedFee: %w", err))
			return false
		}
		txs, err := tdf.Txs()
		if err != nil {
			yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: component Txs: %w", err))
			return false
		}
		for env, eerr := range txs.Iter() {
			if eerr != nil {
				yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: component envelope iter: %w", eerr))
				return false
			}
			if !yield(env, nil) {
				return false
			}
		}
	}
	return true
}

// enumerateParallelTxs yields every envelope in V1-style parallel-txs (stages
// -> clusters -> txs). Returns false if the consumer broke or an error was
// yielded.
func enumerateParallelTxs(ptx xdr.ParallelTxsComponentView, yield func(xdr.TransactionEnvelopeView, error) bool) bool {
	stages, err := ptx.ExecutionStages()
	if err != nil {
		yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: ExecutionStages: %w", err))
		return false
	}
	for stage, serr := range stages.Iter() {
		if serr != nil {
			yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: stage iter: %w", serr))
			return false
		}
		for cluster, cerr := range stage.Iter() {
			if cerr != nil {
				yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: cluster iter: %w", cerr))
				return false
			}
			for env, eerr := range cluster.Iter() {
				if eerr != nil {
					yield(xdr.TransactionEnvelopeView{}, fmt.Errorf("ingest: cluster envelope iter: %w", eerr))
					return false
				}
				if !yield(env, nil) {
					return false
				}
			}
		}
	}
	return true
}
