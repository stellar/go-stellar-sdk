package ingest

// The extraction tier's single public home. Collectors here drive the PUBLIC
// xdr Walk surface (WalkLedgerCloseMeta / WalkTransactionMeta) exactly as any
// third party would — no privileged access.

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// LedgerTransactionEvents is one transaction's contract events plus its hash,
// in the flat raw-bytes shape an events indexer consumes. Every byte slice
// ALIASES the source LedgerCloseMetaView buffer (zero-copy); callers copy
// what they retain.
//
//   - TransactionEvents holds the V4 top-level transaction events, each a raw
//     xdr.TransactionEvent.
//   - OperationEvents holds, per operation, the raw xdr.ContractEvent bytes.
//     For V3 SorobanMeta there is a single operation group (present iff
//     SorobanMeta is present); for V4 there is one group per operation.
//
// V0/V1/V2 meta carry no contract events, so both event fields are empty.
type LedgerTransactionEvents struct {
	Hash              [32]byte
	InnerHash         [32]byte   // the inner transaction's hash; meaningful iff FeeBump
	FeeBump           bool       // the transaction is a fee-bump
	TransactionEvents [][]byte   // raw xdr.TransactionEvent (V4 top-level)
	OperationEvents   [][][]byte // raw xdr.ContractEvent, per operation
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

func (c *ledgerEventsCollector) resultPair(tx int, pair xdr.TransactionResultPairView) error {
	e := LedgerTransactionEvents{
		TransactionEvents: [][]byte{},
		OperationEvents:   [][][]byte{},
	}
	h, inner, feeBump, err := txProcessingHashes(txPartViews{Result: pair})
	if err != nil {
		return err
	}
	e.Hash, e.InnerHash, e.FeeBump = h, inner, feeBump
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

func (c *ledgerEventsCollector) opEvent(tx, op, ev int, e xdr.ContractEventView) error {
	g := c.out[tx].OperationEvents
	g[len(g)-1] = append(g[len(g)-1], e.MustRaw()) // exact-extent: slice op
	return nil
}

func (c *ledgerEventsCollector) txEvent(tx, ev int, e xdr.TransactionEventView) error {
	c.out[tx].TransactionEvents = append(c.out[tx].TransactionEvents, e.MustRaw())
	return nil
}

// ExtractLedgerEvents returns the contract events of every transaction in the
// ledger, in apply order, each paired with its transaction hash — one
// generated Walk over the buffer. It does NOT gate V3 SorobanMeta events on
// whether the transaction is soroban (SorobanMeta presence is the
// trusted-input gate); the read path applies that gate where the paired
// envelope is in hand.
//
// Experimental: the view-based extractors are new in this release and their
// signatures may still change.
func ExtractLedgerEvents(lcm xdr.LedgerCloseMetaView) ([]LedgerTransactionEvents, error) {
	var c ledgerEventsCollector
	w := xdr.LedgerCloseMetaWalk{
		TxProcessingBegin: c.txProcessingBegin,
		ResultPair:        c.resultPair,
		V4Ops:             c.v4Ops,
		OpEventsBegin:     c.opEventsBegin,
		OpEvent:           c.opEvent,
		TxEvent:           c.txEvent,
	}
	if err := xdr.WalkLedgerCloseMeta(lcm, &w); err != nil {
		return nil, fmt.Errorf("ingest: %w", err)
	}
	return c.out, nil
}

// ExtractTxHashes returns every transaction hash of the ledger in apply
// (TxProcessing) order — the cheapest per-ledger hash listing; everything
// else is pruned by the Walk.
//
// Experimental: the view-based extractors are new in this release and their
// signatures may still change.
func ExtractTxHashes(lcm xdr.LedgerCloseMetaView) ([]xdr.Hash, error) {
	var out []xdr.Hash
	w := xdr.LedgerCloseMetaWalk{
		TxProcessingBegin: func(count int) error {
			if count > 0 {
				out = make([]xdr.Hash, 0, count)
			}
			return nil
		},
		ResultPair: func(_ int, pair xdr.TransactionResultPairView) error {
			hv, err := pair.TransactionHash()
			if err != nil {
				return err
			}
			h, err := hv.Value()
			if err != nil {
				return err
			}
			out = append(out, h)
			return nil
		},
	}
	if err := xdr.WalkLedgerCloseMeta(lcm, &w); err != nil {
		return nil, fmt.Errorf("ingest: %w", err)
	}
	return out, nil
}
