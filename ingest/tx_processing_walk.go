package ingest

import (
	"fmt"
	"math"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// xdrWord is the width of an XDR array count.
const xdrWord = 4

// txProcessing is a ledger's TxProcessing array.
type txProcessing struct {
	arr   []byte // from the count prefix; runs past the array's end
	count int
	lcmV2 bool // elements are TransactionResultMetaV1, not TransactionResultMeta
}

func newTxProcessing[A interface {
	~[]byte
	Count() (int, error)
}](arr A, lcmV2 bool) (txProcessing, error) {
	// Offsets into the array's metas are recorded as uint32.
	if uint64(len(arr)) > math.MaxUint32 {
		return txProcessing{}, fmt.Errorf("%d bytes exceed 4 GiB", len(arr))
	}
	count, err := arr.Count()
	if err != nil {
		return txProcessing{}, err
	}
	return txProcessing{arr: arr, count: count, lcmV2: lcmV2}, nil
}

// walk sizes each element once, in apply order, and appends the parts of the
// elements keep selects, locating their metas into rec. keep sees each
// element's index and result pair before its meta; the walk ends after the
// element keep stops at.
func (tp txProcessing) walk(
	out []LedgerTxParts, rec *offsetRecord, keep func(i int, result xdr.TransactionResultPairView) (kept, stop bool),
) ([]LedgerTxParts, error) {
	i := 0
	err := xdr.TryVoid(func() {
		b, p := tp.arr, xdrWord
		for ; i < tp.count; i++ {
			if tp.lcmV2 {
				p += len(xdr.ExtensionPointView(b[p:]).MustRaw())
			}
			result := xdr.TransactionResultPairView(xdr.TransactionResultPairView(b[p:]).MustRaw())
			p += len(result)
			p += len(xdr.LedgerEntryChangesView(b[p:]).MustRaw()) // FeeProcessing
			kept, stop := keep(i, result)
			if !kept {
				p += len(xdr.TransactionMetaView(b[p:]).MustRaw())
			} else {
				out = append(out, LedgerTxParts{})
				part := &out[len(out)-1]
				part.Result = result
				part.Hash, part.InnerHash, part.FeeBump = resultHashes(result)
				part.rec, part.recStart = rec, rec.len()
				part.Meta = rec.locate(b[p:])
				part.recEnd = rec.len()
				p += len(part.Meta)
			}
			if tp.lcmV2 {
				p += len(xdr.LedgerEntryChangesView(b[p:]).MustRaw()) // PostTxApplyFeeProcessing
			}
			if stop {
				return
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("ingest: TxProcessing element %d: %w", i, err)
	}
	return out, nil
}

// resultHashes returns a TransactionResultPair's transaction hash and, for a
// fee-bump result, the inner transaction's.
func resultHashes(result xdr.TransactionResultPairView) (hash, inner [32]byte, feeBump bool) {
	hash = [32]byte(result.MustTransactionHash().MustValue())
	switch res := result.MustResult().MustResult(); res.MustCode() {
	case xdr.TransactionResultCodeTxFeeBumpInnerSuccess, xdr.TransactionResultCodeTxFeeBumpInnerFailed:
		return hash, [32]byte(res.MustInnerResultPair().MustTransactionHash().MustValue()), true
	default:
		return hash, [32]byte{}, false
	}
}

// maxPresizedTxs caps the transactions presized for, so a crafted
// TxProcessing count cannot force a large allocation before the walk fails.
const maxPresizedTxs = 1 << 13

// presized returns parts and a record with room for n transactions.
func presized(n int) ([]LedgerTxParts, *offsetRecord) {
	n = min(n, maxPresizedTxs)
	return make([]LedgerTxParts, 0, n), &offsetRecord{offsets: make([]uint32, 0, offsetsPerTx*n)}
}

// offsetRecord holds, for each meta located into it, an entry saying where the
// products' fields sit, as offsets from the meta's first byte:
//
//	the meta's version
//	the meta's size
//	the SorobanMeta body, 0 when there is none
//	the DiagnosticEvents array, 0 when there is none
//	V3 with SorobanMeta: its Events array
//	V4: the operation count, each operation's Events array, then Events
//
// An array is its element count followed by count+1 offsets: the start of
// each element, then the end of the last. A meta starts with its 4-byte
// version, so 0 is never a field's offset.
type offsetRecord struct {
	offsets []uint32
}

// Entry header positions.
const (
	entryVersion = iota
	entrySize
	entrySoroban
	entryDiagnostics
	entryArrays
)

func (r *offsetRecord) len() uint32 {
	return uint32(len(r.offsets)) //nolint:gosec // callers bound buffers below 4 GiB
}

// locate walks the TransactionMeta at b, sizing every field once, appends its
// entry, and returns the meta trimmed to its wire extent.
func (r *offsetRecord) locate(b []byte) xdr.TransactionMetaView {
	first := len(r.offsets)
	v := xdr.TransactionMetaView(b).MustV()
	r.offsets = append(r.offsets, uint32(v), 0, 0, 0) //nolint:gosec // a union discriminant
	var end int
	switch v {
	case 3: //nolint:mnd // TransactionMeta version
		end = r.v3(b, first)
	case 4: //nolint:mnd // TransactionMeta version
		end = r.v4(b, first)
	default:
		end = len(xdr.TransactionMetaView(b).MustRaw())
	}
	r.set(first+entrySize, end)
	return xdr.TransactionMetaView(b[:end])
}

// v3 walks a TransactionMetaV3 from its SorobanMeta, the last field: Ext,
// Events, ReturnValue, DiagnosticEvents.
func (r *offsetRecord) v3(b []byte, first int) int {
	opt := xdr.TransactionMetaView(b).MustV3().MustSorobanMeta()
	sm, present := opt.MustUnwrap()
	if !present {
		return offsetOf(b, opt) + len(opt.MustRaw())
	}
	r.set(first+entrySoroban, offsetOf(b, sm))
	p := recordArray[xdr.SorobanTransactionMetaEventsView, xdr.ContractEventView](r, b, sm.MustEvents())
	p += len(xdr.ScValView(b[p:]).MustRaw())
	r.set(first+entryDiagnostics, p)
	return p + len(xdr.SorobanTransactionMetaDiagnosticEventsView(b[p:]).MustRaw())
}

// v4 walks a TransactionMetaV4 from its Operations on: Operations (each ending
// with its Events), TxChangesAfter, SorobanMeta, Events, DiagnosticEvents.
func (r *offsetRecord) v4(b []byte, first int) int {
	ops := xdr.TransactionMetaView(b).MustV4().MustOperations()
	n := ops.MustCount()
	r.offsets = append(r.offsets, uint32(n)) //nolint:gosec // bounded by the buffer, below 4 GiB
	p := offsetOf(b, ops) + xdrWord
	for range n {
		events := xdr.OperationMetaV2View(b[p:]).MustEvents()
		p = recordArray[xdr.OperationMetaV2EventsView, xdr.ContractEventView](r, b, events)
	}
	p += len(xdr.LedgerEntryChangesView(b[p:]).MustRaw())
	opt := xdr.TransactionMetaV4SorobanMetaOptView(b[p:])
	if sm, present := opt.MustUnwrap(); present {
		r.set(first+entrySoroban, offsetOf(b, sm))
	}
	p += len(opt.MustRaw())
	p = recordArray[xdr.TransactionMetaV4EventsView, xdr.TransactionEventView](r, b, xdr.TransactionMetaV4EventsView(b[p:]))
	r.set(first+entryDiagnostics, p)
	return p + len(xdr.TransactionMetaV4DiagnosticEventsView(b[p:]).MustRaw())
}

// offsetOf returns the offset in b of v, a view resliced from b.
func offsetOf(b, v []byte) int {
	return cap(b) - cap(v)
}

func (r *offsetRecord) set(i, offset int) {
	r.offsets[i] = uint32(offset) //nolint:gosec // callers bound buffers below 4 GiB
}

// recordArray walks arr, an array of E resliced from b, appending its count
// and element boundaries to r, and returns the offset in b past it.
func recordArray[A interface {
	~[]byte
	MustCount() int
}, E interface {
	~[]byte
	MustRaw() []byte
}](r *offsetRecord, b []byte, arr A) int {
	n := arr.MustCount()
	p := offsetOf(b, arr) + xdrWord
	r.offsets = append(r.offsets, uint32(n), uint32(p)) //nolint:gosec // callers bound buffers below 4 GiB
	for range n {
		p += len(E(b[p:]).MustRaw())
		r.offsets = append(r.offsets, uint32(p)) //nolint:gosec // callers bound buffers below 4 GiB
	}
	return p
}

// entry returns the part's record entry, locating Meta into scratch for a part
// built by hand (the entry is then valid until scratch is reused).
func (p *LedgerTxParts) entry(scratch *offsetRecord) ([]uint32, error) {
	if p.rec == nil {
		if uint64(len(p.Meta)) > math.MaxUint32 {
			return nil, fmt.Errorf("ingest: tx %x: meta of %d bytes exceeds 4 GiB", p.Hash, len(p.Meta))
		}
		scratch.offsets = scratch.offsets[:0]
		if err := xdr.TryVoid(func() { scratch.locate(p.Meta) }); err != nil {
			return nil, fmt.Errorf("ingest: tx %x: meta: %w", p.Hash, err)
		}
		return scratch.offsets, nil
	}
	e := p.walkedEntry()
	v, err := p.Meta.V()
	if err != nil || uint32(v) != e[entryVersion] || int(e[entrySize]) != len(p.Meta) { //nolint:gosec // a union discriminant
		return nil, fmt.Errorf("ingest: tx %x: Meta is not the meta the walk located", p.Hash)
	}
	return e, nil
}

// walkedEntry returns the entry the walk recorded for the part.
func (p *LedgerTxParts) walkedEntry() []uint32 {
	return p.rec.offsets[p.recStart:p.recEnd]
}

// entryReader reads an entry's arrays in the order locate wrote them.
type entryReader []uint32

func (r *entryReader) next() uint32 {
	v := (*r)[0]
	*r = (*r)[1:]
	return v
}

// skip passes over the next recorded array and returns its element count.
func (r *entryReader) skip() int {
	n := r.next()
	*r = (*r)[n+1:]
	return int(n)
}

// eventSlabs are backing arrays that many transactions' event slices are
// carved from, one allocation per element type.
type eventSlabs struct {
	groups   [][]xdr.ContractEventView
	contract []xdr.ContractEventView
	tx       []xdr.TransactionEventView
}

// newEventSlabs sizes slabs for the events of the walked parts in txParts.
func newEventSlabs(txParts []LedgerTxParts) eventSlabs {
	var groups, contract, tx int
	for i := range txParts {
		if txParts[i].rec == nil {
			continue
		}
		entry := txParts[i].walkedEntry()
		r := entryReader(entry[entryArrays:])
		switch entry[entryVersion] {
		case 3: //nolint:mnd // TransactionMeta version
			if entry[entrySoroban] != 0 {
				groups++
				contract += r.skip()
			}
		case 4: //nolint:mnd // TransactionMeta version
			m := int(r.next())
			groups += m
			for range m {
				contract += r.skip()
			}
			tx += r.skip()
		default:
		}
	}
	return eventSlabs{
		groups:   make([][]xdr.ContractEventView, groups),
		contract: make([]xdr.ContractEventView, contract),
		tx:       make([]xdr.TransactionEventView, tx),
	}
}

// carve returns the first n elements of slab, capped at n, and advances slab
// past them; with no slab it allocates.
func carve[E any](slab *[]E, n int) []E {
	if slab == nil {
		return make([]E, n)
	}
	out := (*slab)[:n:n]
	*slab = (*slab)[n:]
	return out
}

// elements returns the next recorded array as views into meta.
func elements[E ~[]byte](r *entryReader, meta []byte, slab *[]E) []E {
	n := r.next()
	bounds := (*r)[:n+1]
	*r = (*r)[n+1:]
	out := carve(slab, int(n))
	for i := range out {
		out[i] = E(meta[bounds[i]:bounds[i+1]])
	}
	return out
}

// metaEvents returns the contract events the entry points at, carved from s
// when s is not nil.
func metaEvents(meta xdr.TransactionMetaView, entry []uint32, s *eventSlabs) (TxEvents, error) {
	var groups *[][]xdr.ContractEventView
	var contract *[]xdr.ContractEventView
	var tx *[]xdr.TransactionEventView
	if s != nil {
		groups, contract, tx = &s.groups, &s.contract, &s.tx
	}
	events := TxEvents{TransactionEvents: []xdr.TransactionEventView{}, OperationEvents: [][]xdr.ContractEventView{}}
	r := entryReader(entry[entryArrays:])
	switch v := entry[entryVersion]; v {
	case 0, 1, 2: //nolint:mnd // TransactionMeta versions
	case 3: //nolint:mnd // TransactionMeta version
		if entry[entrySoroban] != 0 {
			events.OperationEvents = carve(groups, 1)
			events.OperationEvents[0] = elements(&r, meta, contract)
		}
	case 4: //nolint:mnd // TransactionMeta version
		events.OperationEvents = carve(groups, int(r.next()))
		for k := range events.OperationEvents {
			events.OperationEvents[k] = elements(&r, meta, contract)
		}
		events.TransactionEvents = elements(&r, meta, tx)
	default:
		return TxEvents{}, fmt.Errorf("ingest: unsupported TransactionMeta V=%d", v)
	}
	return events, nil
}

// metaDiagnostics returns the diagnostic events of a located meta.
func metaDiagnostics(meta xdr.TransactionMetaView, entry []uint32) ([]xdr.DiagnosticEventView, error) {
	at := entry[entryDiagnostics]
	if at == 0 {
		return []xdr.DiagnosticEventView{}, nil
	}
	var diag []xdr.DiagnosticEventView
	var err error
	if entry[entryVersion] == 3 { //nolint:mnd // TransactionMeta version
		diag, err = xdr.SorobanTransactionMetaDiagnosticEventsView(meta[at:]).All()
	} else {
		diag, err = xdr.TransactionMetaV4DiagnosticEventsView(meta[at:]).All()
	}
	if err != nil {
		return nil, fmt.Errorf("ingest: diagnostic events: %w", err)
	}
	return diag, nil
}
