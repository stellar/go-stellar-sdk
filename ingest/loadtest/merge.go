package loadtest

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// MergeOptions configures MergeLedgerBytes.
type MergeOptions struct {
	// HeaderFrom is the index of the input whose header, ext, scpInfo and
	// totalByteSizeOfLiveSorobanState the merged ledger keeps, and whose
	// parallel Soroban phase base fee it uses.
	HeaderFrom int
	// LedgerSeq, when non-zero, replaces the kept header's ledgerSeq.
	LedgerSeq uint32
	// PreviousLedgerHash, when non-nil, becomes the previousLedgerHash of the
	// merged header and tx set. When nil, both take the first input's.
	PreviousLedgerHash *xdr.Hash
	// RemapLedgerSeq, when non-nil, rewrites every ledger sequence stored in
	// the ledger entries of input i (lastModifiedLedgerSeq, TTL
	// liveUntilLedgerSeq and account seqLedger) to RemapLedgerSeq(i, seq).
	RemapLedgerSeq func(input int, seq uint32) uint32
}

// mergeInput holds the wire spans of one input ledger. The fields after
// txProcessing are split once the merge has walked it (see
// mergedWalk.txProcessing).
type mergeInput struct {
	ext          []byte
	header       xdr.LedgerHeaderHistoryEntryView
	phases       []txSetPhase
	txProcessing []byte // txProcessing and every field after it
	upgrades     []byte
	scpInfo      []byte
	sorobanSize  []byte
	evictedKeys  []byte
}

// txSetPhase is one phase of an input's tx set: its v0 components, or its
// parallel component's base fee and execution stages.
type txSetPhase struct {
	v          int32
	components []txSetComponent
	baseFee    []byte
	stages     []byte
}

// txSetComponent is a v0 component's base fee and transactions.
type txSetComponent struct{ baseFee, txs []byte }

// MergeLedgerBytes merges XDR-encoded LedgerCloseMeta V2 ledgers into one
// without decoding them, and returns it with its header hash. The merged
// ledger holds every input's transactions, upgrades and evicted keys in input
// order. Its tx set keeps the inputs' phase layout: classic components are
// regrouped by base fee, and parallel Soroban execution stages are
// concatenated. The tx set hash, result set hash and header hash are
// recomputed, so the ledger is self-consistent.
//
// ingest.LedgerChangeReader reads a ledger's fee changes before any
// transaction's changes and its refunds after them, so inputs that change the
// same account would replay out of order. The merge rewrites those accounts'
// changes (see repair) so that every account replays through consistent
// states to its real final state. Inputs that share no accounts keep their
// changes byte for byte.
func MergeLedgerBytes(ledgers [][]byte, opts MergeOptions) ([]byte, xdr.Hash, error) {
	if len(ledgers) == 0 {
		return nil, xdr.Hash{}, fmt.Errorf("no ledgers to merge")
	}
	if opts.HeaderFrom < 0 || opts.HeaderFrom >= len(ledgers) {
		return nil, xdr.Hash{}, fmt.Errorf("HeaderFrom %d out of range [0, %d)", opts.HeaderFrom, len(ledgers))
	}
	inputs := make([]mergeInput, len(ledgers))
	size := 0
	for i, raw := range ledgers {
		in, err := splitLedger(raw)
		if err != nil {
			return nil, xdr.Hash{}, fmt.Errorf("ledger %d: %w", i, err)
		}
		inputs[i] = in
		size += len(raw)
	}
	var out []byte
	var hash xdr.Hash
	err := tryWalk(func() {
		out, hash = mergeInputs(inputs, opts, size)
	})
	if err != nil {
		return nil, xdr.Hash{}, err
	}
	return out, hash, nil
}

func splitLedger(raw []byte) (mergeInput, error) {
	var in mergeInput
	err := tryWalk(func() {
		lcm := xdr.LedgerCloseMetaView(raw)
		if v := lcm.MustV(); v != 2 { //nolint:mnd // LedgerCloseMeta version discriminant
			check(fmt.Errorf("LedgerCloseMeta version %d is not supported", v))
		}
		v2 := lcm.MustV2()
		in.ext, in.header = v2.MustExt().MustRaw(), v2.MustLedgerHeader()
		txSet := v2.MustTxSet()
		var n int
		in.phases, n = splitTxSet(txSet)
		in.txProcessing = txSet[n:]
	})
	return in, err
}

// offset returns where v starts in root. v must be a subslice of root made
// without a capacity limit, as every view accessor returns.
func offset(root, v []byte) int { return cap(root) - cap(v) }

// splitTxSet splits a GeneralizedTransactionSet into its phases, sizing each
// transaction once, and returns them with the tx set's size.
func splitTxSet(txSet xdr.GeneralizedTransactionSetView) ([]txSetPhase, int) {
	if v := txSet.MustV(); v != 1 {
		check(fmt.Errorf("tx set version %d is not supported", v))
	}
	all := txSet.MustV1TxSet().MustPhases()
	phases := make([]txSetPhase, all.MustCount())
	off := offset(txSet, all) + arrayCountSize
	for p := range phases {
		phase := xdr.TransactionPhaseView(txSet[off:])
		ph := &phases[p]
		ph.v = phase.MustV()
		switch ph.v {
		case 0:
			components := phase.MustV0Components()
			off = offset(txSet, components) + arrayCountSize
			for range components.MustCount() {
				c := xdr.TxSetComponentView(txSet[off:])
				if typ := c.MustType(); typ != xdr.TxSetComponentTypeTxsetCompTxsMaybeDiscountedFee {
					check(fmt.Errorf("tx set component type %d is not supported", typ))
				}
				comp := c.MustTxsMaybeDiscountedFee()
				fee, txs := comp.MustBaseFee().MustRaw(), comp.MustTxs().MustRaw()
				ph.components = append(ph.components, txSetComponent{baseFee: fee, txs: txs})
				off = offset(txSet, txs) + len(txs)
			}
		case 1:
			parallel := phase.MustParallelTxsComponent()
			ph.baseFee, ph.stages = parallel.MustBaseFee().MustRaw(), parallel.MustExecutionStages().MustRaw()
			off = offset(txSet, ph.stages) + len(ph.stages)
		default:
			check(fmt.Errorf("tx set phase version %d is not supported", ph.v))
		}
	}
	return phases, off
}

// splitTail splits the fields that follow an input's txProcessing array.
func (in *mergeInput) splitTail(tail []byte) {
	next := func(raw []byte) []byte {
		tail = tail[len(raw):]
		return raw
	}
	in.upgrades = next(xdr.LedgerCloseMetaV2UpgradesProcessingView(tail).MustRaw())
	in.scpInfo = next(xdr.LedgerCloseMetaV2ScpInfoView(tail).MustRaw())
	in.sorobanSize = next(xdr.Uint64View(tail).MustRaw())
	in.evictedKeys = next(xdr.LedgerCloseMetaV2EvictedKeysView(tail).MustRaw())
}

func mergeInputs(inputs []mergeInput, opts MergeOptions, size int) ([]byte, xdr.Hash) {
	kept := &inputs[opts.HeaderFrom]
	prevHash := inputs[0].header.MustHeader().MustPreviousLedgerHash().MustValue()
	if opts.PreviousLedgerHash != nil {
		prevHash = *opts.PreviousLedgerHash
	}

	out := make([]byte, 0, size)
	out = binary.BigEndian.AppendUint32(out, 2) //nolint:mnd // LedgerCloseMeta version discriminant
	out = append(out, kept.ext...)

	headerOff := len(out)
	out = append(out, kept.header.MustRaw()...)

	txSetOff := len(out)
	out = appendTxSet(out, inputs, opts.HeaderFrom, prevHash)
	txSetHash := sha256.Sum256(out[txSetOff:])

	w := newMergedWalk(len(inputs), opts.RemapLedgerSeq)
	out = w.txProcessing(out, inputs)
	upgradesOff := len(out)
	out = appendArrays(out, collect(inputs, func(in mergeInput) []byte { return in.upgrades }))
	out = append(out, kept.scpInfo...)
	out = append(out, kept.sorobanSize...)
	out = appendArrays(out, collect(inputs, func(in mergeInput) []byte { return in.evictedKeys }))
	w.root = out
	w.upgradesProcessing(upgradesOff, inputs)
	var resultHash xdr.Hash
	w.results.Sum(resultHash[:0])

	hash := patchHeader(out[headerOff:], kept.header, opts.LedgerSeq, prevHash, txSetHash, resultHash)
	return repair(out, w), hash
}

// patchHeader rewrites the LedgerHeaderHistoryEntry at the start of dst (a
// copy of src) and returns the new header hash.
func patchHeader(
	dst []byte, src xdr.LedgerHeaderHistoryEntryView, ledgerSeq uint32,
	prevHash, txSetHash, resultHash xdr.Hash,
) xdr.Hash {
	entry, err := src.Fields()
	check(err)
	header, err := entry.Header.Fields()
	check(err)
	scp, err := header.ScpValue.Fields()
	check(err)

	copy(dst[offset(src, header.PreviousLedgerHash):], prevHash[:])
	copy(dst[offset(src, scp.TxSetHash):], txSetHash[:])
	copy(dst[offset(src, header.TxSetResultHash):], resultHash[:])
	if ledgerSeq != 0 {
		binary.BigEndian.PutUint32(dst[offset(src, header.LedgerSeq):], ledgerSeq)
	}
	start := offset(src, entry.Header)
	hash := sha256.Sum256(dst[start : start+len(entry.Header.MustRaw())])
	copy(dst[offset(src, entry.Hash):], hash[:])
	return hash
}

// appendTxSet appends a GeneralizedTransactionSet (v1) holding every input's
// transactions, phase by phase.
func appendTxSet(out []byte, inputs []mergeInput, headerFrom int, prevHash xdr.Hash) []byte {
	phases := len(inputs[0].phases)
	for i, in := range inputs {
		if len(in.phases) != phases {
			check(fmt.Errorf("ledger %d has %d tx set phases, ledger 0 has %d", i, len(in.phases), phases))
		}
	}
	out = binary.BigEndian.AppendUint32(out, 1)
	out = append(out, prevHash[:]...)
	out = binary.BigEndian.AppendUint32(out, uint32(phases)) //nolint:gosec // an XDR array count read from the input
	for p := range phases {
		v := inputs[0].phases[p].v
		for i, in := range inputs {
			if got := in.phases[p].v; got != v {
				check(fmt.Errorf("ledger %d tx set phase %d has version %d, ledger 0 has %d", i, p, got, v))
			}
		}
		out = binary.BigEndian.AppendUint32(out, uint32(v)) //nolint:gosec // a phase discriminant, 0 or 1
		if v == 0 {
			out = appendComponents(out, inputs, p)
			continue
		}
		out = append(out, inputs[headerFrom].phases[p].baseFee...)
		out = appendArrays(out, collect(inputs, func(in mergeInput) []byte { return in.phases[p].stages }))
	}
	return out
}

// appendComponents appends phase p's v0 components, one per distinct base
// fee (in order of first appearance), each holding its transactions in input
// order.
func appendComponents(out []byte, inputs []mergeInput, p int) []byte {
	type group struct {
		baseFee []byte
		txs     [][]byte
	}
	var groups []*group
	byFee := map[string]*group{}
	for _, in := range inputs {
		for _, c := range in.phases[p].components {
			g := byFee[string(c.baseFee)]
			if g == nil {
				g = &group{baseFee: c.baseFee}
				byFee[string(c.baseFee)] = g
				groups = append(groups, g)
			}
			g.txs = append(g.txs, c.txs)
		}
	}
	out = binary.BigEndian.AppendUint32(out, uint32(len(groups))) //nolint:gosec // at most the inputs' component count
	for _, g := range groups {
		out = binary.BigEndian.AppendUint32(out, uint32(xdr.TxSetComponentTypeTxsetCompTxsMaybeDiscountedFee))
		out = append(out, g.baseFee...)
		out = appendArrays(out, g.txs)
	}
	return out
}

// appendArrays appends one XDR variable-length array holding the elements of
// every raw array in arrays, in order.
func appendArrays(out []byte, arrays [][]byte) []byte {
	var n uint64
	for _, a := range arrays {
		n += uint64(binary.BigEndian.Uint32(a))
	}
	if n > math.MaxUint32 {
		check(fmt.Errorf("merged array would hold %d elements", n))
	}
	out = binary.BigEndian.AppendUint32(out, uint32(n))
	for _, a := range arrays {
		out = append(out, a[4:]...)
	}
	return out
}

func collect(inputs []mergeInput, field func(mergeInput) []byte) [][]byte {
	out := make([][]byte, len(inputs))
	for i, in := range inputs {
		out[i] = field(in)
	}
	return out
}

// walkErr carries an error out of a view walk that uses the panicking Must
// accessors.
type walkErr struct{ err error }

// tryWalk runs fn, returning as an error any *xdr.ViewError raised by a Must
// accessor or any error raised through check.
func tryWalk(fn func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch e := r.(type) {
			case *xdr.ViewError:
				err = e
			case walkErr:
				err = e.err
			default:
				panic(r)
			}
		}
	}()
	fn()
	return nil
}

func check(err error) {
	if err != nil {
		panic(walkErr{err})
	}
}
