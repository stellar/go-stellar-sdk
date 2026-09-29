package loadtest

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"

	"github.com/stellar/go-stellar-sdk/xdr"
)

const arrayCountSize = 4

type accountKey [36]byte // an XDR AccountID

// accountUpdate is one update to an account in the merged ledger: a CREATED
// change, or a STATE change and the UPDATED or REMOVED change right after it.
type accountUpdate struct {
	typ    xdr.LedgerEntryChangeType // CREATED, UPDATED or REMOVED when paired
	acct   int32                     // index into mergedWalk.accounts
	moved  bool                      // the repair moves it into its transaction's meta
	state  []byte                    // the STATE change, nil for a CREATED
	change []byte                    // the CREATED, UPDATED or REMOVED change
	// The images after the repair, nil where the ledger's stand.
	newPre, newPost []byte
}

// pre is the image before the update, nil for a CREATED.
func (u *accountUpdate) pre() []byte {
	if u.state == nil {
		return nil
	}
	return xdr.LedgerEntryChangeView(u.state).MustState()
}

// post is the image after the update, nil for a REMOVED.
func (u *accountUpdate) post() []byte {
	c := xdr.LedgerEntryChangeView(u.change)
	switch u.typ {
	case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
		return c.MustCreated()
	case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
		return c.MustUpdated()
	default:
		return nil
	}
}

// paired reports whether the update has the shape stellar-core emits: a
// CREATED, or a STATE followed by its UPDATED or REMOVED.
func (u *accountUpdate) paired() bool {
	return u.typ == xdr.LedgerEntryChangeTypeLedgerEntryCreated || u.state != nil
}

// changes is the update's changes in the ledger, where they are adjacent.
func (u *accountUpdate) changes() []byte {
	if u.state == nil {
		return u.change
	}
	return u.state[:len(u.state)+len(u.change)]
}

// span is a range of indices into mergedWalk.updates.
type span struct{ start, end int }

// txRecord is what the walk records about one transaction: its element in
// the ledger and its account updates by phase.
type txRecord struct {
	input             int
	elem              []byte
	fee, meta, refund span
}

// account is what the merge tracks about one account the inputs change.
type account struct {
	firstInput  int
	lastTxInput int  // the last input whose transactions change it
	shared      bool // more than one input changes it
	created     int  // the first input whose transactions create it, or -1
	removed     int  // the last input whose transactions remove it, or -1
	seen        bool // initial is set
	initial     []byte
	hasCurrent  bool
	current     []byte
}

// changePhase is the part of a ledger a LedgerEntryChanges array belongs to.
type changePhase int

const (
	phaseFee changePhase = iota
	phaseMeta
	phaseRefund
	phaseUpgrade
)

// mergedWalk walks a merged ledger's transactions and upgrades once, sizing
// each part once. It rewrites entry ledger sequences in place (when remap is
// set), hashes the result pairs, and records every account and account update
// for the repair.
type mergedWalk struct {
	root     []byte
	remaps   []func(uint32) uint32 // per input; nil without renumbering
	results  hash.Hash
	index    map[accountKey]int32
	accounts []account
	updates  []accountUpdate
	txs      []txRecord
	shared   int
	scratch  []byte // rebased images, which must not alias root
}

func newMergedWalk(inputs int, remap func(input int, seq uint32) uint32) *mergedWalk {
	w := &mergedWalk{results: sha256.New(), index: map[accountKey]int32{}}
	if remap != nil {
		w.remaps = make([]func(uint32) uint32, inputs)
		for i := range inputs {
			w.remaps[i] = func(s uint32) uint32 { return remap(i, s) }
		}
	}
	return w
}

func (w *mergedWalk) account(key accountKey, input int) int32 {
	if i, ok := w.index[key]; ok {
		if a := &w.accounts[i]; !a.shared && a.firstInput != input {
			a.shared = true
			w.shared++
		}
		return i
	}
	i := int32(len(w.accounts)) //nolint:gosec // bounded by the ledger's change count
	w.index[key] = i
	w.accounts = append(w.accounts, account{firstInput: input, created: -1, removed: -1})
	return i
}

// keep returns a copy of b that survives the in-place edits of root.
func (w *mergedWalk) keep(b []byte) []byte {
	w.scratch = append(w.scratch, b...)
	return w.scratch[len(w.scratch)-len(b) : len(w.scratch) : len(w.scratch)]
}

func (w *mergedWalk) u32(off int) int {
	if off+arrayCountSize > len(w.root) {
		check(fmt.Errorf("ledger ends inside the array count at offset %d", off))
	}
	return int(binary.BigEndian.Uint32(w.root[off:]))
}

// txProcessing appends the inputs' merged txProcessing array to out and walks
// it. Only walking an input's array sizes it, so each input's ledger from its
// array on is appended and walked in place, the bytes after the array are
// dropped, and the input's later fields are split from the rest of its ledger.
func (w *mergedWalk) txProcessing(out []byte, inputs []mergeInput) []byte {
	n := 0
	for _, in := range inputs {
		n += txCount(in)
	}
	out = appendArrays(out, collect(inputs, func(in mergeInput) []byte { return in.txProcessing[:arrayCountSize] }))
	_, _ = w.results.Write(out[len(out)-arrayCountSize:])
	w.txs = make([]txRecord, 0, n)
	w.updates = make([]accountUpdate, 0, 4*n) //nolint:mnd // a typical transaction's account updates
	for i := range inputs {
		in := &inputs[i]
		start := len(out)
		out = append(out, in.txProcessing[arrayCountSize:]...)
		w.root = out
		off := start
		for range txCount(*in) {
			off = w.transaction(off, i)
		}
		out = out[:off]
		in.splitTail(in.txProcessing[arrayCountSize+off-start:])
	}
	w.root = out
	return out
}

func txCount(in mergeInput) int {
	return xdr.LedgerCloseMetaV2TxProcessingView(in.txProcessing).MustCount()
}

func (w *mergedWalk) transaction(off, input int) int {
	t := txRecord{input: input, elem: w.root[off:]}
	off += len(xdr.ExtensionPointView(w.root[off:]).MustRaw())
	pair := xdr.TransactionResultPairView(w.root[off:]).MustRaw()
	_, _ = w.results.Write(pair)
	off += len(pair)
	t.fee.start = len(w.updates)
	off = w.walkChanges(off, input, phaseFee)
	t.fee.end = len(w.updates)
	t.meta.start = len(w.updates)
	off = w.txMeta(off, input)
	t.meta.end = len(w.updates)
	t.refund.start = len(w.updates)
	off = w.walkChanges(off, input, phaseRefund)
	t.refund.end = len(w.updates)
	w.txs = append(w.txs, t)
	return off
}

func (w *mergedWalk) txMeta(off, input int) int {
	v := w.u32(off)
	off += arrayCountSize
	switch v {
	case 0:
		return w.operations(off, input, false)
	case 1:
		off = w.walkChanges(off, input, phaseMeta)
		return w.operations(off, input, false)
	case 2, 3, 4: //nolint:mnd // TransactionMeta version discriminants
		if v >= 3 { //nolint:mnd // TransactionMeta V3 and V4 open with an ext
			off += len(xdr.ExtensionPointView(w.root[off:]).MustRaw())
		}
		off = w.walkChanges(off, input, phaseMeta)
		off = w.operations(off, input, v == 4) //nolint:mnd // TransactionMeta V4 carries OperationMetaV2
		off = w.walkChanges(off, input, phaseMeta)
		switch v {
		case 3: //nolint:mnd // TransactionMeta version discriminant
			off += len(xdr.TransactionMetaV3SorobanMetaOptView(w.root[off:]).MustRaw())
		case 4: //nolint:mnd // TransactionMeta version discriminant
			off += len(xdr.TransactionMetaV4SorobanMetaOptView(w.root[off:]).MustRaw())
			off += len(xdr.TransactionMetaV4EventsView(w.root[off:]).MustRaw())
			off += len(xdr.TransactionMetaV4DiagnosticEventsView(w.root[off:]).MustRaw())
		}
		return off
	default:
		check(fmt.Errorf("unsupported TransactionMeta version %d", v))
		return off
	}
}

func (w *mergedWalk) operations(off, input int, v2 bool) int {
	n := w.u32(off)
	off += arrayCountSize
	for range n {
		if v2 {
			off += len(xdr.ExtensionPointView(w.root[off:]).MustRaw())
		}
		off = w.walkChanges(off, input, phaseMeta)
		if v2 {
			off += len(xdr.OperationMetaV2EventsView(w.root[off:]).MustRaw())
		}
	}
	return off
}

// walkChanges walks the LedgerEntryChanges at off, which input holds in
// phase. It renumbers the entries and records the account updates, pairing
// each account STATE with the UPDATED or REMOVED right after it.
func (w *mergedWalk) walkChanges(off, input int, phase changePhase) int {
	n := w.u32(off)
	off += arrayCountSize
	var state []byte // an account STATE change awaiting its UPDATED or REMOVED
	var stateKey accountKey
	unpaired := func() {
		if state != nil {
			w.record(xdr.LedgerEntryChangeTypeLedgerEntryState, w.account(stateKey, input), input, phase, nil, state)
			state = nil
		}
	}
	for range n {
		c := xdr.LedgerEntryChangeView(w.root[off:])
		raw := c.MustRaw()
		off += len(raw)
		typ, id := w.change(c, input)
		if id == nil {
			unpaired()
			continue
		}
		key := accountKey(id)
		switch {
		case typ == xdr.LedgerEntryChangeTypeLedgerEntryState:
			unpaired()
			state, stateKey = raw, key
		case state != nil && key == stateKey &&
			(typ == xdr.LedgerEntryChangeTypeLedgerEntryUpdated || typ == xdr.LedgerEntryChangeTypeLedgerEntryRemoved):
			w.record(typ, w.account(key, input), input, phase, state, raw)
			state = nil
		default:
			unpaired()
			w.record(typ, w.account(key, input), input, phase, nil, raw)
		}
	}
	unpaired()
	return off
}

// change renumbers the entry of the LedgerEntryChange c and returns its type
// with its account ID, or nil when it changes another kind of entry. Only
// accounts need tracking: fee and refund changes touch nothing else, so every
// other entry's transaction changes are read in their real order.
func (w *mergedWalk) change(c xdr.LedgerEntryChangeView, input int) (xdr.LedgerEntryChangeType, xdr.AccountIdView) {
	typ := c.MustType()
	var entry xdr.LedgerEntryView
	switch typ {
	case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
		if key := c.MustRemoved(); key.MustType() == xdr.LedgerEntryTypeAccount {
			return typ, key.MustAccount().MustAccountId()
		}
		return typ, nil
	case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
		entry = c.MustCreated()
	case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
		entry = c.MustUpdated()
	case xdr.LedgerEntryChangeTypeLedgerEntryState:
		entry = c.MustState()
	case xdr.LedgerEntryChangeTypeLedgerEntryRestored:
		entry = c.MustRestored()
	}
	if w.remaps != nil {
		renumber(entry, w.remaps[input])
	}
	if data := entry.MustData(); data.MustType() == xdr.LedgerEntryTypeAccount {
		return typ, data.MustAccount().MustAccountId()
	}
	return typ, nil
}

// renumber rewrites, in place, the ledger sequences entry stores: its
// lastModifiedLedgerSeq, an account's seqLedger and a TTL's liveUntilLedgerSeq.
func renumber(entry xdr.LedgerEntryView, remap func(uint32) uint32) {
	patch := func(v xdr.Uint32View) { binary.BigEndian.PutUint32(v, remap(v.MustValue())) }
	patch(entry.MustLastModifiedLedgerSeq())
	data := entry.MustData()
	switch data.MustType() {
	case xdr.LedgerEntryTypeAccount:
		ext := data.MustAccount().MustExt()
		if ext.MustV() != 1 {
			return
		}
		ext1 := ext.MustV1().MustExt()
		if ext1.MustV() != 2 { //nolint:mnd // AccountEntryExtensionV1 ext arm
			return
		}
		ext2 := ext1.MustV2().MustExt()
		if ext2.MustV() != 3 { //nolint:mnd // AccountEntryExtensionV2 ext arm
			return
		}
		patch(ext2.MustV3().MustSeqLedger())
	case xdr.LedgerEntryTypeTtl:
		patch(data.MustTtl().MustLiveUntilLedgerSeq())
	}
}

// record notes one account update in phase. An upgrade is only checked: the
// change reader reads upgrades after every transaction, so an upgrade to an
// account that a later input's transactions change cannot be reordered.
func (w *mergedWalk) record(
	typ xdr.LedgerEntryChangeType, acct int32, input int, phase changePhase, state, change []byte,
) {
	a := &w.accounts[acct]
	switch phase {
	case phaseUpgrade:
		if a.lastTxInput > input {
			check(fmt.Errorf("ledger %d upgrades an account ledger %d changes, which merging cannot reorder",
				input, a.lastTxInput))
		}
		return
	case phaseMeta:
		if typ == xdr.LedgerEntryChangeTypeLedgerEntryCreated && a.created < 0 {
			a.created = input
		}
		if typ == xdr.LedgerEntryChangeTypeLedgerEntryRemoved {
			a.removed = input
		}
	}
	a.lastTxInput = input
	w.updates = append(w.updates, accountUpdate{typ: typ, acct: acct, state: state, change: change})
}

func (w *mergedWalk) upgradesProcessing(off int, inputs []mergeInput) {
	off += arrayCountSize
	for i, in := range inputs {
		for range binary.BigEndian.Uint32(in.upgrades) {
			off += len(xdr.LedgerUpgradeView(w.root[off:]).MustRaw())
			off = w.walkChanges(off, i, phaseUpgrade)
		}
	}
}
