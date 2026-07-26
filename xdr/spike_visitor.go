package xdr

// SPIKE gating surface (stage 3): the hand-written stage-1 walker has been
// replaced by the GENERATED WalkLedgerCloseMeta; the Spike* entry points
// survive only so the frozen-oracle differential, truncation sweep, and
// benchmark harness keep gating the generated Walk until stage 4 retires
// them in favor of the exported extractors.

import "encoding/binary"

// SpikeLedgerTransactionEvents mirrors ingest.LedgerTransactionEvents.
type SpikeLedgerTransactionEvents struct {
	Hash              [32]byte
	InnerHash         [32]byte
	FeeBump           bool
	TransactionEvents [][]byte
	OperationEvents   [][][]byte
}

type spikeEventsCollector struct {
	out []SpikeLedgerTransactionEvents
}

func (c *spikeEventsCollector) txProcessingBegin(count int) error {
	c.out = make([]SpikeLedgerTransactionEvents, 0, count)
	return nil
}

func (c *spikeEventsCollector) resultPair(tx int, pair TransactionResultPairView) error {
	d := pair.d
	e := SpikeLedgerTransactionEvents{
		TransactionEvents: [][]byte{},
		OperationEvents:   [][][]byte{},
	}
	copy(e.Hash[:], d[:32])
	code := int32(binary.BigEndian.Uint32(d[40:44]))
	if code == int32(TransactionResultCodeTxFeeBumpInnerSuccess) ||
		code == int32(TransactionResultCodeTxFeeBumpInnerFailed) {
		copy(e.InnerHash[:], d[44:76])
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

func (c *spikeEventsCollector) txEvent(tx, ev int, e TransactionEventView) error {
	c.out[tx].TransactionEvents = append(c.out[tx].TransactionEvents, e.d)
	return nil
}

// SpikeExtractLedgerEvents drives the generated Walk. perArray is retired
// (per-event granularity was adopted by the ladder); the parameter stays so
// the gating tests exercise one path under both flags.
func SpikeExtractLedgerEvents(d []byte, perArray bool) ([]SpikeLedgerTransactionEvents, error) {
	_ = perArray
	var c spikeEventsCollector
	w := LedgerCloseMetaWalk{
		TxProcessingBegin: c.txProcessingBegin,
		ResultPair:        c.resultPair,
		V4Ops:             c.v4Ops,
		OpEventsBegin:     c.opEventsBegin,
		OpEvent:           c.opEvent,
		TxEvent:           c.txEvent,
	}
	if err := WalkLedgerCloseMeta(d, &w); err != nil {
		return nil, err
	}
	return c.out, nil
}

// SpikeExtractTxHashes drives a hashes-only subscription of the generated Walk.
func SpikeExtractTxHashes(d []byte) ([]Hash, error) {
	var out []Hash
	w := LedgerCloseMetaWalk{
		TxProcessingBegin: func(count int) error {
			out = make([]Hash, 0, count)
			return nil
		},
		ResultPair: func(_ int, pair TransactionResultPairView) error {
			var h Hash
			copy(h[:], pair.d[:32])
			out = append(out, h)
			return nil
		},
	}
	if err := WalkLedgerCloseMeta(d, &w); err != nil {
		return nil, err
	}
	return out, nil
}

// SpikeWalkTxEventsOnly subscribes only the LAST meta position — the pruned
// walk benchmark (parity with blind size proves masks/prunes cost nothing).
func SpikeWalkTxEventsOnly(d []byte) (int, error) {
	n := 0
	w := LedgerCloseMetaWalk{TxEvent: func(_, _ int, _ TransactionEventView) error {
		n++
		return nil
	}}
	if err := WalkLedgerCloseMeta(d, &w); err != nil {
		return 0, err
	}
	return n, nil
}

// SpikeWalkNothing is the zero-subscription walk: returns immediately,
// validating nothing.
func SpikeWalkNothing(d []byte) error {
	return WalkLedgerCloseMeta(d, &LedgerCloseMetaWalk{})
}
