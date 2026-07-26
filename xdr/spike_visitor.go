package xdr

// SPIKE — two-tier visitor, stage 1 (go/no-go). A hand-written
// LedgerCloseMeta walk over the thin sizing engine with position-keyed
// callbacks, modeling the generated tier-2 Walk: unsubscribed subtrees are
// pruned via thin size skips, subscribed positions fire indirect calls
// through func values (exactly the call shape generated code will have).
//
// The Spike* symbols are exported ONLY so the frozen oracle differential and
// the benchmark harness in package ingest can gate them; they are NOT API —
// they will be deleted when the generated Walk lands. Everything here is
// spike-quality by design: hand-tracked offsets mirroring the generated thin
// size bodies, validated by byte-identity against the frozen oracle over the
// full corpus and its truncation sweep.

import "encoding/binary"

// spikeLCMCallbacks is the position-keyed subscription set. nil = position
// not subscribed; subtrees containing no subscribed positions are skipped
// via the thin engine. Group construction is version-discriminating by
// construction: OpEventsBegin/OpEventsRaw fire once per V4 op (count from
// nOps), and at most once for a V3 meta, gated on SorobanMeta presence —
// never derived from an arm-merged op count.
type spikeLCMCallbacks struct {
	// ResultPair delivers TxProcessing[txIdx].Result's exact wire bytes.
	ResultPair func(txIdx int, pair []byte) error
	// MetaVersion delivers the tx's TransactionMeta discriminant.
	MetaVersion func(txIdx int, v int32) error
	// V4Ops fires before a V4 meta's operations with the op count (spine
	// presizing; V0-V3 never fire it).
	V4Ops func(txIdx, nOps int) error

	// Variant (a): per-event delivery. *_Begin fires once per event group
	// with the group's validated count (V3: opIdx 0, iff SorobanMeta present;
	// V4: per op, empty groups included).
	OpEventsBegin func(txIdx, opIdx, count int) error
	OpEvent       func(txIdx, opIdx, evIdx int, ev ContractEventView) error
	TxEventsBegin func(txIdx, count int) error
	TxEvent       func(txIdx, evIdx int, ev TransactionEventView) error

	// Variant (b): per-array delivery, AllRaw-style — one indirect call per
	// group, raws presized and exact-extent (ownership passes to the callee).
	OpEventsRaw func(txIdx, opIdx int, raws [][]byte) error
	TxEventsRaw func(txIdx int, raws [][]byte) error
}

func (cb *spikeLCMCallbacks) wantOpEvents() bool {
	return cb.OpEvent != nil || cb.OpEventsBegin != nil || cb.OpEventsRaw != nil
}
func (cb *spikeLCMCallbacks) wantTxEvents() bool {
	return cb.TxEvent != nil || cb.TxEventsBegin != nil || cb.TxEventsRaw != nil
}
func (cb *spikeLCMCallbacks) wantMeta() bool {
	return cb.MetaVersion != nil || cb.V4Ops != nil || cb.wantOpEvents() || cb.wantTxEvents()
}

// spikeWalkLedgerCloseMeta drives cb over one LedgerCloseMeta. A zero
// subscription returns immediately, validating nothing (panel decision).
// Truncation stops round up to element/array advance boundaries: the walk
// errors exactly where a thin size or count read fails, which is the same
// boundary the frozen oracle path errors on.
func spikeWalkLedgerCloseMeta(d []byte, cb *spikeLCMCallbacks) error {
	if cb.ResultPair == nil && !cb.wantMeta() {
		return nil
	}
	if len(d) < 4 {
		return viewErrShortBuffer(0, "need 4 bytes for discriminant")
	}
	disc := int32(binary.BigEndian.Uint32(d[:4]))
	off := int64(4)
	var minElemW int
	switch disc {
	case 0:
		// LedgerCloseMetaV0: LedgerHeader, TxSet, TxProcessing, ...
		sz, err := sizeLedgerHeaderHistoryEntryView(d[off:], 1)
		if err != nil {
			return err
		}
		off += int64(sz)
		if sz, err = sizeTransactionSetView(d[off:], 1); err != nil {
			return err
		}
		off += int64(sz)
		minElemW = 60
	case 1, 2:
		// LedgerCloseMetaV1/V2: Ext, LedgerHeader, TxSet, TxProcessing, ...
		sz, err := sizeLedgerCloseMetaExtView(d[off:], 1)
		if err != nil {
			return err
		}
		off += int64(sz)
		if sz, err = sizeLedgerHeaderHistoryEntryView(d[off:], 1); err != nil {
			return err
		}
		off += int64(sz)
		if sz, err = sizeGeneralizedTransactionSetView(d[off:], 1); err != nil {
			return err
		}
		off += int64(sz)
		minElemW = 60
		if disc == 2 {
			minElemW = 68
		}
	default:
		return viewErrUnknownDiscriminant(0, disc)
	}
	if off > int64(len(d)) {
		return viewErrShortBuffer(uint32(off), "field offset exceeds data")
	}
	count, err := arrayViewCountChecked(d[off:], 0, minElemW)
	if err != nil {
		return err
	}
	off += 4
	for tx := 0; tx < count; tx++ {
		if off > int64(len(d)) {
			return viewErrShortBuffer(uint32(off), "element offset exceeds data")
		}
		if disc == 2 {
			// TransactionResultMetaV1's leading ExtensionPoint (fixed 4).
			off += 4
			if off > int64(len(d)) {
				return viewErrShortBuffer(uint32(off), "field offset exceeds data")
			}
		}
		// Result pair.
		sz, err := sizeTransactionResultPairView(d[off:], 2)
		if err != nil {
			return err
		}
		if cb.ResultPair != nil {
			if err := cb.ResultPair(tx, d[off:off+int64(sz)]); err != nil {
				return err
			}
		}
		off += int64(sz)
		// FeeProcessing.
		if sz, err = sizeLedgerEntryChangesView(d[off:], 2); err != nil {
			return err
		}
		off += int64(sz)
		// TxApplyProcessing.
		if !cb.wantMeta() {
			if sz, err = sizeTransactionMetaView(d[off:], 2); err != nil {
				return err
			}
			off += int64(sz)
		} else {
			msz, err := spikeWalkTransactionMeta(d[off:], tx, cb)
			if err != nil {
				return err
			}
			off += msz
		}
		if disc == 2 {
			// Trailing PostTxApplyFeeProcessing.
			if off > int64(len(d)) {
				return viewErrShortBuffer(uint32(off), "field offset exceeds data")
			}
			if sz, err = sizeLedgerEntryChangesView(d[off:], 2); err != nil {
				return err
			}
			off += int64(sz)
		}
		if off > int64(len(d)) {
			return viewErrShortBuffer(uint32(off), "element exceeds data")
		}
	}
	return nil
}

// spikeWalkTransactionMeta walks one TransactionMeta with subscribed meta
// positions, returning its wire size.
func spikeWalkTransactionMeta(d []byte, tx int, cb *spikeLCMCallbacks) (int64, error) {
	if len(d) < 4 {
		return 0, viewErrShortBuffer(0, "need 4 bytes for discriminant")
	}
	v := int32(binary.BigEndian.Uint32(d[:4]))
	if cb.MetaVersion != nil {
		if err := cb.MetaVersion(tx, v); err != nil {
			return 0, err
		}
	}
	off := int64(4)
	switch v {
	case 0:
		sz, err := sizeTransactionMetaOperationsView(d[off:], 3)
		if err != nil {
			return 0, err
		}
		off += int64(sz)
	case 1:
		sz, err := sizeTransactionMetaV1View(d[off:], 3)
		if err != nil {
			return 0, err
		}
		off += int64(sz)
	case 2:
		sz, err := sizeTransactionMetaV2View(d[off:], 3)
		if err != nil {
			return 0, err
		}
		off += int64(sz)
	case 3:
		// TransactionMetaV3: Ext(4), TxChangesBefore, Operations,
		// TxChangesAfter, SorobanMeta optional.
		off += 4
		if off > int64(len(d)) {
			return 0, viewErrShortBuffer(uint32(off), "field offset exceeds data")
		}
		sz, err := sizeLedgerEntryChangesView(d[off:], 4)
		if err != nil {
			return 0, err
		}
		off += int64(sz)
		if sz, err = sizeTransactionMetaV3OperationsView(d[off:], 4); err != nil {
			return 0, err
		}
		off += int64(sz)
		if sz, err = sizeLedgerEntryChangesView(d[off:], 4); err != nil {
			return 0, err
		}
		off += int64(sz)
		if off+4 > int64(len(d)) {
			return 0, viewErrShortBuffer(uint32(off), "need 4 bytes for optional flag")
		}
		flag := binary.BigEndian.Uint32(d[off : off+4])
		off += 4
		switch flag {
		case 0:
			// SorobanMeta absent: no event group (version-discriminating
			// construction — presence, not op count, gates the group).
		case 1:
			// SorobanTransactionMeta: Ext, Events, ReturnValue, DiagnosticEvents.
			sz, err := sizeSorobanTransactionMetaExtView(d[off:], 5)
			if err != nil {
				return 0, err
			}
			off += int64(sz)
			evsz, err := spikeWalkContractEvents(d[off:], tx, 0, cb)
			if err != nil {
				return 0, err
			}
			off += evsz
			if sz, err = sizeScValView(d[off:], 5); err != nil {
				return 0, err
			}
			off += int64(sz)
			if sz, err = sizeSorobanTransactionMetaDiagnosticEventsView(d[off:], 5); err != nil {
				return 0, err
			}
			off += int64(sz)
		default:
			return 0, viewErrBadBoolValue(uint32(off-4), flag)
		}
	case 4:
		// TransactionMetaV4: Ext(4), TxChangesBefore, Operations,
		// TxChangesAfter, SorobanMeta opt, Events, DiagnosticEvents.
		off += 4
		if off > int64(len(d)) {
			return 0, viewErrShortBuffer(uint32(off), "field offset exceeds data")
		}
		sz, err := sizeLedgerEntryChangesView(d[off:], 4)
		if err != nil {
			return 0, err
		}
		off += int64(sz)
		// Operations: version-discriminating group construction — one group
		// per op, count from nOps.
		if off > int64(len(d)) {
			return 0, viewErrShortBuffer(uint32(off), "field offset exceeds data")
		}
		nOps, err := arrayViewCountChecked(d[off:], 0, 12)
		if err != nil {
			return 0, err
		}
		if cb.V4Ops != nil {
			if err := cb.V4Ops(tx, nOps); err != nil {
				return 0, err
			}
		}
		off += 4
		for op := 0; op < nOps; op++ {
			// OperationMetaV2: Ext(4), Changes, Events.
			off += 4
			if off > int64(len(d)) {
				return 0, viewErrShortBuffer(uint32(off), "field offset exceeds data")
			}
			if sz, err = sizeLedgerEntryChangesView(d[off:], 5); err != nil {
				return 0, err
			}
			off += int64(sz)
			evsz, err := spikeWalkContractEvents(d[off:], tx, op, cb)
			if err != nil {
				return 0, err
			}
			off += evsz
		}
		if sz, err = sizeLedgerEntryChangesView(d[off:], 4); err != nil {
			return 0, err
		}
		off += int64(sz)
		if sz, err = sizeTransactionMetaV4SorobanMetaOptView(d[off:], 4); err != nil {
			return 0, err
		}
		off += int64(sz)
		evsz, err := spikeWalkTransactionEvents(d[off:], tx, cb)
		if err != nil {
			return 0, err
		}
		off += evsz
		if sz, err = sizeTransactionMetaV4DiagnosticEventsView(d[off:], 4); err != nil {
			return 0, err
		}
		off += int64(sz)
	default:
		return 0, viewErrUnknownDiscriminant(0, v)
	}
	if off > int64(len(d)) {
		return 0, viewErrShortBuffer(uint32(off), "meta exceeds data")
	}
	return off, nil
}

// spikeWalkContractEvents walks one contract-event array (a V3 soroban group
// or a V4 op group), delivering per the subscribed variant, and returns its
// wire size.
func spikeWalkContractEvents(d []byte, tx, op int, cb *spikeLCMCallbacks) (int64, error) {
	if !cb.wantOpEvents() {
		sz, err := sizeSorobanTransactionMetaEventsView(d, 5)
		return int64(sz), err
	}
	count, err := arrayViewCountChecked(d, 0, 24)
	if err != nil {
		return 0, err
	}
	off := int64(4)
	if cb.OpEventsRaw != nil {
		// Variant (b): AllRaw-style — one indirect call per group.
		raws := make([][]byte, 0, count)
		for k := 0; k < count; k++ {
			if off > int64(len(d)) {
				return 0, viewErrShortBuffer(uint32(off), "element offset exceeds data")
			}
			sz, err := sizeContractEventView(d[off:], 6)
			if err != nil {
				return 0, err
			}
			raws = append(raws, d[off:off+int64(sz)])
			off += int64(sz)
		}
		return off, cb.OpEventsRaw(tx, op, raws)
	}
	// Variant (a): per-event callbacks.
	if cb.OpEventsBegin != nil {
		if err := cb.OpEventsBegin(tx, op, count); err != nil {
			return 0, err
		}
	}
	for k := 0; k < count; k++ {
		if off > int64(len(d)) {
			return 0, viewErrShortBuffer(uint32(off), "element offset exceeds data")
		}
		sz, err := sizeContractEventView(d[off:], 6)
		if err != nil {
			return 0, err
		}
		if cb.OpEvent != nil {
			if err := cb.OpEvent(tx, op, k, ContractEventView{view{d: d[off : off+int64(sz)]}}); err != nil {
				return 0, err
			}
		}
		off += int64(sz)
	}
	return off, nil
}

// spikeWalkTransactionEvents walks a V4 meta's top-level transaction-event
// array, delivering per the subscribed variant, and returns its wire size.
func spikeWalkTransactionEvents(d []byte, tx int, cb *spikeLCMCallbacks) (int64, error) {
	if !cb.wantTxEvents() {
		sz, err := sizeTransactionMetaV4EventsView(d, 5)
		return int64(sz), err
	}
	count, err := arrayViewCountChecked(d, 0, 28)
	if err != nil {
		return 0, err
	}
	off := int64(4)
	if cb.TxEventsRaw != nil {
		raws := make([][]byte, 0, count)
		for k := 0; k < count; k++ {
			if off > int64(len(d)) {
				return 0, viewErrShortBuffer(uint32(off), "element offset exceeds data")
			}
			sz, err := sizeTransactionEventView(d[off:], 6)
			if err != nil {
				return 0, err
			}
			raws = append(raws, d[off:off+int64(sz)])
			off += int64(sz)
		}
		return off, cb.TxEventsRaw(tx, raws)
	}
	if cb.TxEventsBegin != nil {
		if err := cb.TxEventsBegin(tx, count); err != nil {
			return 0, err
		}
	}
	for k := 0; k < count; k++ {
		if off > int64(len(d)) {
			return 0, viewErrShortBuffer(uint32(off), "element offset exceeds data")
		}
		sz, err := sizeTransactionEventView(d[off:], 6)
		if err != nil {
			return 0, err
		}
		if cb.TxEvent != nil {
			if err := cb.TxEvent(tx, k, TransactionEventView{view{d: d[off : off+int64(sz)]}}); err != nil {
				return 0, err
			}
		}
		off += int64(sz)
	}
	return off, nil
}

// ---------------------------------------------------------------------------
// Spike collectors (the tier-3 extractor shapes, driven through callbacks).
// ---------------------------------------------------------------------------

// SpikeLedgerTransactionEvents mirrors ingest.LedgerTransactionEvents.
type SpikeLedgerTransactionEvents struct {
	Hash              [32]byte
	InnerHash         [32]byte
	FeeBump           bool
	TransactionEvents [][]byte
	OperationEvents   [][][]byte
}

// spikeEventsCollector builds per-tx event bundles from walk callbacks.
type spikeEventsCollector struct {
	out []SpikeLedgerTransactionEvents
}

func (c *spikeEventsCollector) resultPair(tx int, pair []byte) error {
	e := SpikeLedgerTransactionEvents{
		TransactionEvents: [][]byte{},
		OperationEvents:   [][][]byte{},
	}
	copy(e.Hash[:], pair[:32])
	// TransactionResult: FeeCharged(8), then the result union's code.
	code := int32(binary.BigEndian.Uint32(pair[40:44]))
	if code == int32(TransactionResultCodeTxFeeBumpInnerSuccess) ||
		code == int32(TransactionResultCodeTxFeeBumpInnerFailed) {
		copy(e.InnerHash[:], pair[44:76])
		e.FeeBump = true
	}
	c.out = append(c.out, e)
	return nil
}

func (c *spikeEventsCollector) v4Ops(tx, nOps int) error {
	c.out[tx].OperationEvents = make([][][]byte, 0, nOps)
	return nil
}

func (c *spikeEventsCollector) opEventsBegin(tx, op, count int) error {
	c.out[tx].OperationEvents = append(c.out[tx].OperationEvents, make([][]byte, 0, count))
	return nil
}

func (c *spikeEventsCollector) opEvent(tx, op, ev int, e ContractEventView) error {
	g := c.out[tx].OperationEvents
	g[len(g)-1] = append(g[len(g)-1], e.d)
	return nil
}

func (c *spikeEventsCollector) opEventsRaw(tx, op int, raws [][]byte) error {
	c.out[tx].OperationEvents = append(c.out[tx].OperationEvents, raws)
	return nil
}

func (c *spikeEventsCollector) txEvent(tx, ev int, e TransactionEventView) error {
	c.out[tx].TransactionEvents = append(c.out[tx].TransactionEvents, e.d)
	return nil
}

func (c *spikeEventsCollector) txEventsRaw(tx int, raws [][]byte) error {
	c.out[tx].TransactionEvents = raws
	return nil
}

// SpikeExtractLedgerEvents is the visitor-driven twin of
// ingest.ExtractLedgerEvents, in both delivery variants. SPIKE — not API.
func SpikeExtractLedgerEvents(d []byte, perArray bool) ([]SpikeLedgerTransactionEvents, error) {
	var c spikeEventsCollector
	cb := spikeLCMCallbacks{ResultPair: c.resultPair, V4Ops: c.v4Ops}
	if perArray {
		cb.OpEventsRaw = c.opEventsRaw
		cb.TxEventsRaw = c.txEventsRaw
	} else {
		cb.OpEventsBegin = c.opEventsBegin
		cb.OpEvent = c.opEvent
		cb.TxEvent = c.txEvent
	}
	if err := spikeWalkLedgerCloseMeta(d, &cb); err != nil {
		return nil, err
	}
	return c.out, nil
}

// SpikeExtractTxHashes is the visitor-driven twin of ingest.ExtractTxHashes.
// SPIKE — not API.
func SpikeExtractTxHashes(d []byte) ([]Hash, error) {
	var out []Hash
	cb := spikeLCMCallbacks{ResultPair: func(_ int, pair []byte) error {
		var h Hash
		copy(h[:], pair[:32])
		out = append(out, h)
		return nil
	}}
	if err := spikeWalkLedgerCloseMeta(d, &cb); err != nil {
		return nil, err
	}
	return out, nil
}

// SpikeWalkTxEventsOnly subscribes ONLY the last meta position (V4 top-level
// transaction events): on a classic-heavy ledger everything is pruned via
// thin size skips, so this measures that the mask/prune concept costs
// nothing over a blind size. Returns the delivered event count. SPIKE.
func SpikeWalkTxEventsOnly(d []byte) (int, error) {
	n := 0
	cb := spikeLCMCallbacks{TxEvent: func(_, _ int, _ TransactionEventView) error {
		n++
		return nil
	}}
	if err := spikeWalkLedgerCloseMeta(d, &cb); err != nil {
		return 0, err
	}
	return n, nil
}

// SpikeWalkNothing is the zero-subscription walk: returns immediately,
// validating nothing (panel decision, pinned by test). SPIKE.
func SpikeWalkNothing(d []byte) error {
	return spikeWalkLedgerCloseMeta(d, &spikeLCMCallbacks{})
}
