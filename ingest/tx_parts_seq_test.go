package ingest

// Test-only projections over the tier-1 iterators: the production read paths
// moved to the Walk (range) and the scanner-based by-hash search, but the
// equivalence tests still drive per-tx parts through these sequences.

import (
	"fmt"
	"iter"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// txPartsSeq adapts a V0/V1 TxProcessing element sequence
// (TransactionResultMeta) to the version-agnostic txPartViews projection.
// Per element it opens the field bundle and resolves Result and
// TxApplyProcessing in wire order — one pass over the element's leading
// fields; the enclosing All() advance then resumes from whatever subtree the
// consumer finished last.
//
// The adapter invokes the inner sequence directly with a fused yield instead
// of ranging over it: per element that is one closure call straight through
// (generated loop -> fused yield -> consumer), skipping the range-over-func
// state machine an intermediate `for range` would add to the hot path.
func txPartsSeq(elems iter.Seq2[xdr.TransactionResultMetaView, error]) iter.Seq2[txPartViews, error] {
	return func(yield func(txPartViews, error) bool) {
		k := 0
		elems(func(elem xdr.TransactionResultMetaView, err error) bool {
			if err != nil {
				yield(txPartViews{}, fmt.Errorf("ingest: TxProcessing element %d: %w", k, err))
				return false
			}
			res, err := elem.Result()
			if err != nil {
				yield(txPartViews{}, fmt.Errorf("ingest: TxProcessing element %d: %w", k, err))
				return false
			}
			meta, err := elem.TxApplyProcessing()
			if err != nil {
				yield(txPartViews{}, fmt.Errorf("ingest: TxProcessing element %d: %w", k, err))
				return false
			}
			k++
			return yield(txPartViews{Result: res, TxApplyProcessing: meta}, nil)
		})
	}
}

// txPartsSeqLCMV2 is txPartsSeq for an LCM V2 TxProcessing array
// (TransactionResultMetaV1 element — Ext leads and PostTxApplyFeeProcessing
// trails). A separate copy for the second element view type; the production
// paths no longer need either (tests drive the read-path helpers through
// these projections).
func txPartsSeqLCMV2(elems iter.Seq2[xdr.TransactionResultMetaV1View, error]) iter.Seq2[txPartViews, error] {
	return func(yield func(txPartViews, error) bool) {
		k := 0
		elems(func(elem xdr.TransactionResultMetaV1View, err error) bool {
			if err != nil {
				yield(txPartViews{}, fmt.Errorf("ingest: TxProcessing element %d: %w", k, err))
				return false
			}
			res, err := elem.Result()
			if err != nil {
				yield(txPartViews{}, fmt.Errorf("ingest: TxProcessing element %d: %w", k, err))
				return false
			}
			meta, err := elem.TxApplyProcessing()
			if err != nil {
				yield(txPartViews{}, fmt.Errorf("ingest: TxProcessing element %d: %w", k, err))
				return false
			}
			k++
			return yield(txPartViews{Result: res, TxApplyProcessing: meta}, nil)
		})
	}
}

// txProcessingHash extracts the 32-byte TransactionHash from a TxProcessing
// entry's projected parts (TransactionResultPair.TransactionHash — the pair's
// first field, so the read is O(1)).
func txProcessingHash(parts txPartViews) (xdr.Hash, error) {
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

// TxProcessing rebuilds the per-version parts sequence from the dispatch's
// view (test-only; the production dispatch no longer carries it).
func (d lcmViewDispatch) TxProcessing() iter.Seq2[txPartViews, error] {
	disc, err := d.lcm.V()
	if err != nil {
		return func(yield func(txPartViews, error) bool) { yield(txPartViews{}, err) }
	}
	switch disc {
	case 0:
		raw, err := d.lcm.MustArmV0().TxProcessing()
		if err != nil {
			return func(yield func(txPartViews, error) bool) { yield(txPartViews{}, err) }
		}
		psc1 := raw.Scan()
		return txPartsSeq(scanSeq2(psc1.Next, psc1.Cur, psc1.Err))
	case 1:
		raw, err := d.lcm.MustArmV1().TxProcessing()
		if err != nil {
			return func(yield func(txPartViews, error) bool) { yield(txPartViews{}, err) }
		}
		psc2 := raw.Scan()
		return txPartsSeq(scanSeq2(psc2.Next, psc2.Cur, psc2.Err))
	default:
		raw, err := d.lcm.MustArmV2().TxProcessing()
		if err != nil {
			return func(yield func(txPartViews, error) bool) { yield(txPartViews{}, err) }
		}
		psc3 := raw.Scan()
		return txPartsSeqLCMV2(scanSeq2(psc3.Next, psc3.Cur, psc3.Err))
	}
}
