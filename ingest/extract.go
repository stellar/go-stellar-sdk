package ingest

// The extraction tier's single public home: one streaming function over the
// generated Walk. Consumers wanting a slice write the collect loop:
//
//	var evs []ingest.LedgerTransactionEvents
//	err := ingest.StreamLedgerEvents(lcm,
//		func(n int) error { evs = make([]ingest.LedgerTransactionEvents, 0, n); return nil },
//		func(_ int, ev ingest.LedgerTransactionEvents) error { evs = append(evs, ev); return nil })

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// LedgerTransactionEvents is one transaction's contract events plus its
// hashes — the per-transaction delivery type of StreamLedgerEvents.
//
// ALIASING CONTRACT: every byte slice ([]byte leaf) aliases the source
// LedgerCloseMetaView buffer — valid exactly as long as the caller keeps
// those bytes alive and unmodified; copy to retain beyond them. The spines
// (the slices of slices) are freshly allocated per transaction, so the value
// itself may be retained across callbacks.
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

// streamCollector assembles one transaction at a time from Walk positions,
// flushing at the TxMeta position (the last to fire for each transaction).
type streamCollector struct {
	begin func(txCount int) error
	tx    func(txIdx int, ev LedgerTransactionEvents) error
	cur   LedgerTransactionEvents
}

func (c *streamCollector) txProcessingBegin(count int) error {
	if c.begin != nil {
		return c.begin(count)
	}
	return nil
}

func (c *streamCollector) resultPair(_ int, pair xdr.TransactionResultPairView) error {
	c.cur = LedgerTransactionEvents{
		TransactionEvents: [][]byte{},
		OperationEvents:   [][][]byte{},
	}
	h, inner, feeBump, err := txProcessingHashes(txPartViews{Result: pair})
	if err != nil {
		return err
	}
	c.cur.Hash, c.cur.InnerHash, c.cur.FeeBump = h, inner, feeBump
	return nil
}

func (c *streamCollector) v4Ops(_ int, nOps int) error {
	c.cur.OperationEvents = make([][][]byte, 0, nOps)
	return nil
}

func (c *streamCollector) opEventsBegin(_, _, count int) error {
	c.cur.OperationEvents = append(c.cur.OperationEvents, make([][]byte, 0, count))
	return nil
}

func (c *streamCollector) opEvent(_, _, _ int, e xdr.ContractEventView) error {
	g := c.cur.OperationEvents
	g[len(g)-1] = append(g[len(g)-1], e.MustRaw()) // exact-extent: slice op
	return nil
}

func (c *streamCollector) txEvent(_, _ int, e xdr.TransactionEventView) error {
	c.cur.TransactionEvents = append(c.cur.TransactionEvents, e.MustRaw())
	return nil
}

func (c *streamCollector) txMeta(txIdx int, _ xdr.TransactionMetaView) error {
	// TxMeta is the last position to fire for a transaction: deliver.
	return c.tx(txIdx, c.cur)
}

// StreamLedgerEvents delivers every transaction of the ledger, in apply
// (TxProcessing) order, to tx — one generated Walk over the buffer, no
// intermediate slice. begin, when non-nil, is called once first with the
// validated transaction count (for caller presizing). Returning ErrStopWalk
// from either callback stops the stream cleanly (StreamLedgerEvents returns
// nil); any other error aborts and is returned. See LedgerTransactionEvents
// for the aliasing contract.
//
// It does NOT gate V3 SorobanMeta events on whether the transaction is
// soroban (SorobanMeta presence is the trusted-input gate); the read path
// applies that gate where the paired envelope is in hand.
//
// Experimental: the view-based extractors are new in this release and their
// signatures may still change.
func StreamLedgerEvents(
	lcm xdr.LedgerCloseMetaView,
	begin func(txCount int) error,
	tx func(txIdx int, ev LedgerTransactionEvents) error,
) error {
	c := streamCollector{begin: begin, tx: tx}
	w := xdr.LedgerCloseMetaWalk{
		TxProcessingBegin: c.txProcessingBegin,
		ResultPair:        c.resultPair,
		V4Ops:             c.v4Ops,
		OpEventsBegin:     c.opEventsBegin,
		OpEvent:           c.opEvent,
		TxEvent:           c.txEvent,
		TxMeta:            c.txMeta,
	}
	if err := xdr.WalkLedgerCloseMeta(lcm, &w); err != nil {
		return fmt.Errorf("ingest: %w", err)
	}
	return nil
}
