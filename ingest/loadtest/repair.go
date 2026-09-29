package loadtest

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"fmt"
	"math"
	"slices"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// repair rewrites the changes of the accounts that several inputs change.
// ingest.LedgerChangeReader reads every fee change before any transaction's
// changes and every refund after them. So if input 0 creates account A and
// input 1 charges A a fee, the reader would see the fee before A exists; the
// merge moves that fee to the front of its own transaction's txChangesBefore.
// Likewise a refund to an account a later input removes moves to the end of
// its own transaction's txChangesAfter. Every other change of such an account
// stays where it is, and its images are rebased so each one follows from the
// one before, which relies on fee and refund changes touching only balance
// and lastModifiedLedgerSeq.
func repair(root []byte, w *mergedWalk) []byte {
	if w.shared == 0 {
		return root
	}
	for _, u := range w.updates {
		if w.accounts[u.acct].shared && !u.paired() {
			check(fmt.Errorf("an account several ledgers change has a %s change outside a STATE and UPDATED or REMOVED pair", u.typ))
		}
	}
	w.firstImages()
	w.markMoves()
	w.rebase(w.readerOrder())
	return w.applyRepair(root)
}

// firstImages sets each shared account's image before its first update in
// real order, which is each input's fees, then its transactions' changes,
// then its refunds; nil for an account that did not exist yet.
func (w *mergedWalk) firstImages() {
	see := func(s span) {
		for i := s.start; i < s.end; i++ {
			u := &w.updates[i]
			if a := &w.accounts[u.acct]; a.shared && !a.seen {
				a.seen, a.initial = true, u.pre()
			}
		}
	}
	for start := 0; start < len(w.txs); {
		end := start
		for end < len(w.txs) && w.txs[end].input == w.txs[start].input {
			end++
		}
		for _, t := range w.txs[start:end] {
			see(t.fee)
		}
		for _, t := range w.txs[start:end] {
			see(t.meta)
		}
		for _, t := range w.txs[start:end] {
			see(t.refund)
		}
		start = end
	}
}

func (w *mergedWalk) markMoves() {
	for _, t := range w.txs {
		for i := t.fee.start; i < t.fee.end; i++ {
			a := &w.accounts[w.updates[i].acct]
			w.updates[i].moved = a.created >= 0 && a.created < t.input
		}
		for i := t.refund.start; i < t.refund.end; i++ {
			a := &w.accounts[w.updates[i].acct]
			w.updates[i].moved = a.removed > t.input
		}
	}
}

// readerOrder lists the shared accounts' updates, as indices into w.updates,
// in the order the change reader will see them once the moves are applied.
func (w *mergedWalk) readerOrder() []int {
	out := make([]int, 0, len(w.updates))
	add := func(s span, moved bool) {
		for i := s.start; i < s.end; i++ {
			if u := &w.updates[i]; w.accounts[u.acct].shared && u.moved == moved {
				out = append(out, i)
			}
		}
	}
	for _, t := range w.txs {
		add(t.fee, false)
	}
	for _, t := range w.txs {
		add(t.fee, true)
		add(t.meta, false)
		add(t.refund, true)
	}
	for _, t := range w.txs {
		add(t.refund, false)
	}
	return out
}

// rebase rewrites the shared accounts' images in the order the change reader
// sees their updates, starting each account from its image before its first
// update in real order (see firstImages).
func (w *mergedWalk) rebase(order []int) {
	n := 0 // room for one copy of each image
	for _, i := range order {
		n += len(w.updates[i].pre()) + len(w.updates[i].post())
	}
	w.scratch = make([]byte, 0, n)
	for _, i := range order {
		u := &w.updates[i]
		a := &w.accounts[u.acct]
		if u.typ == xdr.LedgerEntryChangeTypeLedgerEntryCreated {
			a.current, a.hasCurrent = u.post(), true
			continue
		}
		pre := u.pre()
		cur := a.current
		if !a.hasCurrent {
			cur = a.initial
		}
		if cur == nil {
			cur = pre
		}
		if !bytes.Equal(pre, cur) {
			u.newPre = w.keep(cur)
		}
		if u.typ == xdr.LedgerEntryChangeTypeLedgerEntryRemoved {
			a.current, a.hasCurrent = nil, true
			continue
		}
		post := w.rebasePost(pre, u.post(), cur)
		if !bytes.Equal(post, u.post()) {
			u.newPost = post
		}
		a.current, a.hasCurrent = post, true
	}
}

// rebasePost returns the image an update from pre to post leaves when it
// starts from cur instead. An update that changes only the balance, like every
// fee and refund, applies its balance change to cur; any other update keeps
// post, with its balance moved by cur's offset from pre. Either way the image
// keeps the later of the two lastModifiedLedgerSeq values.
func (w *mergedWalk) rebasePost(pre, post, cur []byte) []byte {
	if bytes.Equal(pre, cur) && lastModified(post) >= lastModified(cur) {
		return post
	}
	base := post
	if onlyBalanceDiffers(pre, post) {
		base = cur
	}
	out := w.keep(base)
	setBalance(out, addBalance(balance(cur), balance(post)-balance(pre)))
	setLastModified(out, max(lastModified(cur), lastModified(post)))
	return out
}

// applyRepair applies the rebased images and the moves to root. Images that
// keep their size are written in place, and only the moves and the images
// that change size splice.
func (w *mergedWalk) applyRepair(root []byte) []byte {
	for i := range w.updates {
		u := &w.updates[i]
		u.newPre = inPlace(u.pre(), u.newPre)
		u.newPost = inPlace(u.post(), u.newPost)
	}
	var edits []edit
	for i := range w.txs {
		edits = w.txEdits(edits, i)
	}
	return applyEdits(root, edits)
}

// inPlace writes img over old when they are the same size and returns nil,
// or returns img for the splice.
func inPlace(old, img []byte) []byte {
	if img == nil || len(img) != len(old) {
		return img
	}
	copy(old, img)
	return nil
}

func (w *mergedWalk) txEdits(edits []edit, i int) []edit {
	t := &w.txs[i]
	edits, feeMoves, nFee := w.phaseEdits(edits, t.fee)
	edits, _, _ = w.phaseEdits(edits, t.meta)
	edits, refundMoves, nRefund := w.phaseEdits(edits, t.refund)
	if nFee == 0 && nRefund == 0 {
		return edits
	}
	arrays := txArraysOf(t.elem)
	if nFee > 0 {
		addCount(arrays.fee, -nFee)
		addCount(arrays.before, nFee)
		edits = append(edits, w.edit(arrays.before[arrayCountSize:arrayCountSize], feeMoves))
	}
	if nRefund > 0 {
		addCount(arrays.refund, -nRefund)
		addCount(arrays.after, nRefund)
		edits = append(edits, w.edit(arrays.after[len(arrays.after):], refundMoves))
	}
	return edits
}

// phaseEdits appends the edits for the updates in s and returns them with the
// bytes of the moved changes and how many changes moved.
func (w *mergedWalk) phaseEdits(edits []edit, s span) ([]edit, []byte, int) {
	var moved []byte
	n := 0
	for i := s.start; i < s.end; i++ {
		u := &w.updates[i]
		if u.moved {
			edits = append(edits, w.edit(u.changes(), nil))
			if u.state != nil {
				moved = appendChange(moved, u.state, u.pre(), u.newPre)
				n++
			}
			moved = appendChange(moved, u.change, u.post(), u.newPost)
			n++
			continue
		}
		if u.newPre != nil {
			edits = append(edits, w.edit(u.pre(), u.newPre))
		}
		if u.newPost != nil {
			edits = append(edits, w.edit(u.post(), u.newPost))
		}
	}
	return edits, moved, n
}

// appendChange appends a change to buf, with img in place of its entry when
// img is not nil.
func appendChange(buf, change, entry, img []byte) []byte {
	if img == nil {
		return append(buf, change...)
	}
	buf = append(buf, change[:len(change)-len(entry)]...)
	return append(buf, img...)
}

// txArrays are the LedgerEntryChanges arrays of one transaction that a move
// edits.
type txArrays struct {
	fee, before, after, refund xdr.LedgerEntryChangesView
}

func txArraysOf(elem []byte) txArrays {
	f, err := xdr.TransactionResultMetaV1View(elem).Fields()
	check(err)
	if v := f.TxApplyProcessing.MustV(); v != 4 { //nolint:mnd // TransactionMeta version discriminant
		check(fmt.Errorf("moving a change into TransactionMeta version %d is not supported", v))
	}
	m, err := f.TxApplyProcessing.MustV4().Fields()
	check(err)
	return txArrays{fee: f.FeeProcessing, before: m.TxChangesBefore, after: m.TxChangesAfter, refund: f.PostTxApplyFeeProcessing}
}

func addCount(array xdr.LedgerEntryChangesView, delta int) {
	binary.BigEndian.PutUint32(array, uint32(array.MustCount()+delta)) //nolint:gosec // moved by at most its own length
}

// edit replaces root[start:end] with repl, which must not alias root.
type edit struct {
	start, end int
	repl       []byte
}

// edit returns the edit that replaces old, a slice of root, with repl; an
// empty old inserts.
func (w *mergedWalk) edit(old, repl []byte) edit {
	start := offset(w.root, old)
	return edit{start, start + len(old), repl}
}

// applyEdits applies edits, which must not overlap, to root in place and
// returns the result. The runs of bytes between edits shift by the size
// change before them and keep their order, so moving the runs that move left
// from the start and the runs that move right from the end never overwrites a
// run before it moves; the replacements are written last.
func applyEdits(root []byte, edits []edit) []byte {
	// An insertion goes before an edit at the same offset.
	slices.SortFunc(edits, func(a, b edit) int { return cmp.Or(a.start-b.start, a.end-b.end) })
	n := len(root)
	shift := make([]int, len(edits)+1) // shift[k] moves the run before edits[k]; the last, the run after them all
	for k, e := range edits {
		shift[k+1] = shift[k] + len(e.repl) - (e.end - e.start)
	}
	grow := shift[len(edits)]
	if grow > 0 {
		root = slices.Grow(root, grow)
	}
	buf := root[:n+max(grow, 0)]
	move := func(k int) {
		start, end := 0, n
		if k > 0 {
			start = edits[k-1].end
		}
		if k < len(edits) {
			end = edits[k].start
		}
		copy(buf[start+shift[k]:], buf[start:end])
	}
	for k := range shift {
		if shift[k] < 0 {
			move(k)
		}
	}
	for k := len(shift) - 1; k >= 0; k-- {
		if shift[k] > 0 {
			move(k)
		}
	}
	for k, e := range edits {
		copy(buf[e.start+shift[k]:], e.repl)
	}
	return buf[:n+grow]
}

func lastModified(entry []byte) uint32 {
	return xdr.LedgerEntryView(entry).MustLastModifiedLedgerSeq().MustValue()
}

func setLastModified(entry []byte, v uint32) {
	binary.BigEndian.PutUint32(xdr.LedgerEntryView(entry).MustLastModifiedLedgerSeq(), v)
}

func balanceView(entry []byte) xdr.Int64View {
	return xdr.LedgerEntryView(entry).MustData().MustAccount().MustBalance()
}

func balance(entry []byte) int64 { return balanceView(entry).MustValue() }

func setBalance(entry []byte, v int64) {
	binary.BigEndian.PutUint64(balanceView(entry), uint64(v)) //nolint:gosec // an XDR int64
}

// onlyBalanceDiffers reports whether a and b differ in nothing but their
// lastModifiedLedgerSeq and balance: the data up to the balance and the
// bytes after it are equal.
func onlyBalanceDiffers(a, b []byte) bool {
	da, db := xdr.LedgerEntryView(a).MustData(), xdr.LedgerEntryView(b).MustData()
	ba, bb := da.MustAccount().MustBalance(), db.MustAccount().MustBalance()
	return bytes.Equal(da[:len(da)-len(ba)], db[:len(db)-len(bb)]) &&
		bytes.Equal(ba[len(ba.MustRaw()):], bb[len(bb.MustRaw()):])
}

func addBalance(a, b int64) int64 {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		check(fmt.Errorf("rebased balance overflows"))
	}
	return a + b
}
