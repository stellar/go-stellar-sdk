package xdr

// Exported extractors over the generated Walk (two-tier visitor design,
// stage 4): consume-a-lot workloads route through WalkLedgerCloseMeta;
// read-a-little workloads use the tier-1 views directly.

import "encoding/binary"

// LedgerTransactionEvents is one transaction's contract events plus its
// hashes, in the flat raw-bytes shape an events indexer consumes. Every byte
// slice ALIASES the source buffer (zero-copy); callers copy what they retain.
//
//   - TransactionEvents holds the V4 top-level transaction events, each a raw
//     TransactionEvent.
//   - OperationEvents holds, per operation, the raw ContractEvent bytes. For
//     V3 SorobanMeta there is a single operation group (present iff
//     SorobanMeta is present); for V4 there is one group per operation.
//
// V0/V1/V2 metas carry no contract events, so both event fields are empty.
type LedgerTransactionEvents struct {
	Hash              [32]byte
	InnerHash         [32]byte // the inner transaction's hash; meaningful iff FeeBump
	FeeBump           bool     // the transaction is a fee-bump
	TransactionEvents [][]byte
	OperationEvents   [][][]byte
}

// resultPairHashes reads (hash, innerHash, feeBump) straight off an
// exact-extent TransactionResultPair delivered by the Walk (the pair was
// fully sized, so the fixed offsets are in bounds: hash at 0, the result
// union's code at 40, the fee-bump inner pair's hash at 44).
func resultPairHashes(pair TransactionResultPairView) (h, inner [32]byte, feeBump bool) {
	d := pair.d
	copy(h[:], d[:32])
	code := int32(binary.BigEndian.Uint32(d[40:44]))
	if code == int32(TransactionResultCodeTxFeeBumpInnerSuccess) ||
		code == int32(TransactionResultCodeTxFeeBumpInnerFailed) {
		copy(inner[:], d[44:76])
		feeBump = true
	}
	return h, inner, feeBump
}

type ledgerEventsCollector struct {
	out []LedgerTransactionEvents
}

func (c *ledgerEventsCollector) txProcessingBegin(count int) error {
	// Presize from the validated count; keep nil for an empty ledger (the
	// historical contract — extraction returns nil, not empty, with no txs).
	if count > 0 {
		c.out = make([]LedgerTransactionEvents, 0, count)
	}
	return nil
}

func (c *ledgerEventsCollector) resultPair(tx int, pair TransactionResultPairView) error {
	e := LedgerTransactionEvents{
		TransactionEvents: [][]byte{},
		OperationEvents:   [][][]byte{},
	}
	e.Hash, e.InnerHash, e.FeeBump = resultPairHashes(pair)
	c.out = append(c.out, e)
	return nil
}

func (c *ledgerEventsCollector) v4Ops(tx, nOps int) error {
	c.out[tx].OperationEvents = make([][][]byte, 0, nOps)
	return nil
}

func (c *ledgerEventsCollector) opEventsBegin(tx, op, count int) error {
	c.out[tx].OperationEvents = append(c.out[tx].OperationEvents, make([][]byte, 0, count))
	return nil
}

func (c *ledgerEventsCollector) opEvent(tx, op, ev int, e ContractEventView) error {
	g := c.out[tx].OperationEvents
	g[len(g)-1] = append(g[len(g)-1], e.d)
	return nil
}

func (c *ledgerEventsCollector) txEvent(tx, ev int, e TransactionEventView) error {
	c.out[tx].TransactionEvents = append(c.out[tx].TransactionEvents, e.d)
	return nil
}

// ExtractLedgerEvents returns the contract events of every transaction in the
// ledger, in apply order, each paired with its transaction hash — one Walk
// over the buffer. It does NOT gate V3 SorobanMeta events on whether the
// transaction is soroban (SorobanMeta presence is the trusted-input gate);
// read paths that hold the paired envelope apply that gate themselves.
func ExtractLedgerEvents(v LedgerCloseMetaView) ([]LedgerTransactionEvents, error) {
	var c ledgerEventsCollector
	w := LedgerCloseMetaWalk{
		TxProcessingBegin: c.txProcessingBegin,
		ResultPair:        c.resultPair,
		V4Ops:             c.v4Ops,
		OpEventsBegin:     c.opEventsBegin,
		OpEvent:           c.opEvent,
		TxEvent:           c.txEvent,
	}
	if err := WalkLedgerCloseMeta(v, &w); err != nil {
		return nil, err
	}
	return c.out, nil
}

// ExtractTxHashes returns every transaction hash of the ledger in apply
// (TxProcessing) order — the cheapest per-ledger hash listing; everything
// else is pruned via thin size skips.
func ExtractTxHashes(v LedgerCloseMetaView) ([]Hash, error) {
	var out []Hash
	w := LedgerCloseMetaWalk{
		TxProcessingBegin: func(count int) error {
			if count > 0 {
				out = make([]Hash, 0, count)
			}
			return nil
		},
		ResultPair: func(_ int, pair TransactionResultPairView) error {
			var h Hash
			copy(h[:], pair.d[:32])
			out = append(out, h)
			return nil
		},
	}
	if err := WalkLedgerCloseMeta(v, &w); err != nil {
		return nil, err
	}
	return out, nil
}

// TransactionMetaEvents is one transaction meta's event content plus its
// version: the per-meta twin of LedgerTransactionEvents' event fields, with
// diagnostics (a read-path concern the ledger-level extractor omits).
type TransactionMetaEvents struct {
	V                 int32
	TransactionEvents [][]byte
	OperationEvents   [][][]byte
	DiagnosticEvents  [][]byte
}

// ExtractTransactionMetaEvents collects one TransactionMeta's contract
// events, top-level transaction events, and diagnostic events, returning the
// meta version. V0/V1/V2 metas return their version with empty spines
// WITHOUT walking the arm (the historical contract; their arms carry no
// events). Group construction is version-discriminating: V3 emits one group
// iff SorobanMeta is present; V4 emits one group per operation.
func ExtractTransactionMetaEvents(v TransactionMetaView) (TransactionMetaEvents, error) {
	out := TransactionMetaEvents{
		TransactionEvents: [][]byte{},
		OperationEvents:   [][][]byte{},
		DiagnosticEvents:  [][]byte{},
	}
	w := LedgerCloseMetaWalk{
		MetaVersion: func(_ int, mv int32) error {
			out.V = mv
			if mv >= 0 && mv <= 2 {
				return ErrStopWalk
			}
			return nil
		},
		V4Ops: func(_, nOps int) error {
			out.OperationEvents = make([][][]byte, 0, nOps)
			return nil
		},
		OpEventsBegin: func(_, _, count int) error {
			out.OperationEvents = append(out.OperationEvents, make([][]byte, 0, count))
			return nil
		},
		OpEvent: func(_, _, _ int, e ContractEventView) error {
			g := out.OperationEvents
			g[len(g)-1] = append(g[len(g)-1], e.d)
			return nil
		},
		TxEvent: func(_, _ int, e TransactionEventView) error {
			out.TransactionEvents = append(out.TransactionEvents, e.d)
			return nil
		},
		DiagEvent: func(_, _ int, e DiagnosticEventView) error {
			out.DiagnosticEvents = append(out.DiagnosticEvents, e.d)
			return nil
		},
	}
	// Drive the generated meta sub-walker directly (tx index 0); ErrStopWalk
	// implements the V0-V2 early return.
	if _, err := walkLCMTransactionMeta(v.d, 0, &w, w.mask()); err != nil && err != ErrStopWalk {
		return TransactionMetaEvents{}, err
	}
	return out, nil
}
