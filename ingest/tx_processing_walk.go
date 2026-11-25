package ingest

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// XDR puts a 4-byte word in front of an array (its element count), an optional
// value (its present flag) and a union's arm (its discriminant).
const (
	arrayCountSize   = 4
	optionalFlagSize = 4
	discriminantSize = 4
)

// txProcessing is a ledger's TxProcessing array.
type txProcessing struct {
	arr   []byte // from the count prefix; runs past the array's end
	count int
	lcmV2 bool // elements are TransactionResultMetaV1, not TransactionResultMeta
}

func newTxProcessing[Array interface {
	~[]byte
	Count() (int, error)
}](arr Array, lcmV2 bool) (txProcessing, error) {
	count, err := arr.Count()
	if err != nil {
		return txProcessing{}, err
	}
	return txProcessing{arr: arr, count: count, lcmV2: lcmV2}, nil
}

// walkedTxs is what a walk kept.
type walkedTxs struct {
	parts []LedgerTxParts
	// diagnosticsAt[i] is the offset in parts[i].Meta of its DiagnosticEvents
	// array, 0 when the meta has none.
	diagnosticsAt []int
}

// maxPresizedTxs caps the transactions presized for, so a crafted
// TxProcessing count cannot force a large allocation before the walk fails.
const maxPresizedTxs = 1 << 15

// txWalker carries one walk's state from element to element.
type txWalker struct {
	eventSlabs
	keep  func(index int, result xdr.TransactionResultPairView) (kept, stop bool)
	out   walkedTxs
	index int            // of the element being walked
	part  *LedgerTxParts // the element's part, nil when keep skipped it
	stop  bool           // keep stopped at the element
}

// walk sizes each element once, in apply order, and returns the elements keep
// selects, with room for presize of them up front. keep sees each element's
// index and result pair before its meta; the walk ends after the element keep
// stops at.
//
// XDR records no struct lengths, so the walk measures every field to find the
// next one, keeping each transaction's hash, result, events and SorobanMeta
// extension on the way.
func (processing txProcessing) walk(
	presize int, keep func(index int, result xdr.TransactionResultPairView) (kept, stop bool),
) (walkedTxs, error) {
	presize = min(presize, maxPresizedTxs)
	walker := txWalker{
		eventSlabs: newEventSlabs(presize),
		keep:       keep,
		out: walkedTxs{
			parts:         make([]LedgerTxParts, 0, presize),
			diagnosticsAt: make([]int, 0, presize),
		},
	}
	err := xdr.TryVoid(func() {
		buf, pos := processing.arr, arrayCountSize
		for ; walker.index < processing.count && !walker.stop; walker.index++ {
			if processing.lcmV2 {
				pos = walker.resultMetaV1(buf, pos)
			} else {
				pos = walker.resultMeta(buf, pos)
			}
		}
	})
	if err != nil {
		return walkedTxs{}, fmt.Errorf("ingest: TxProcessing element %d: %w", walker.index, err)
	}
	return walker.out, nil
}

// resultMeta walks the TransactionResultMeta at pos in buf, an element of an
// LCM V0 or V1 ledger's TxProcessing:
//
//	struct TransactionResultMeta
//	{
//	    TransactionResultPair result;
//	    LedgerEntryChanges feeProcessing;
//	    TransactionMeta txApplyProcessing;
//	};
func (walker *txWalker) resultMeta(buf []byte, pos int) int {
	pos = walker.result(buf, pos)                       // result
	pos += sizeOf[xdr.LedgerEntryChangesView](buf, pos) // feeProcessing
	return walker.meta(buf, pos)                        // txApplyProcessing
}

// resultMetaV1 walks the TransactionResultMetaV1 at pos in buf, an element of
// an LCM V2 ledger's TxProcessing:
//
//	struct TransactionResultMetaV1
//	{
//	    ExtensionPoint ext;
//	    TransactionResultPair result;
//	    LedgerEntryChanges feeProcessing;
//	    TransactionMeta txApplyProcessing;
//	    LedgerEntryChanges postTxApplyFeeProcessing;
//	};
func (walker *txWalker) resultMetaV1(buf []byte, pos int) int {
	pos += sizeOf[xdr.ExtensionPointView](buf, pos)           // ext
	pos = walker.result(buf, pos)                             // result
	pos += sizeOf[xdr.LedgerEntryChangesView](buf, pos)       // feeProcessing
	pos = walker.meta(buf, pos)                               // txApplyProcessing
	return pos + sizeOf[xdr.LedgerEntryChangesView](buf, pos) // postTxApplyFeeProcessing
}

// result reads the element's TransactionResultPair at pos in buf and asks keep
// whether to keep the element, starting its part if so.
func (walker *txWalker) result(buf []byte, pos int) int {
	result := valueAt[xdr.TransactionResultPairView](buf, pos)
	kept, stop := walker.keep(walker.index, result)
	walker.part, walker.stop = nil, stop
	if kept {
		hash, innerHash, feeBump := resultHashes(result)
		walker.out.parts = append(walker.out.parts,
			LedgerTxParts{Hash: hash, InnerHash: innerHash, FeeBump: feeBump, Result: result})
		walker.part = &walker.out.parts[len(walker.out.parts)-1]
	}
	return pos + len(result)
}

// meta walks the element's TransactionMeta at pos in buf: field by field into
// its part when the element is kept, sized whole when it is not.
func (walker *txWalker) meta(buf []byte, pos int) int {
	if walker.part == nil {
		return pos + sizeOf[xdr.TransactionMetaView](buf, pos)
	}
	walker.out.diagnosticsAt = append(walker.out.diagnosticsAt, walker.locate(buf[pos:], walker.part))
	return pos + len(walker.part.Meta)
}

// resultHashes returns a TransactionResultPair's transaction hash and, for a
// fee-bump result, the inner transaction's. Only the two fee-bump codes carry
// an inner result pair.
func resultHashes(result xdr.TransactionResultPairView) (hash, innerHash [32]byte, feeBump bool) {
	hash = [32]byte(result.MustTransactionHash().MustValue())
	switch txResult := result.MustResult().MustResult(); txResult.MustCode() {
	case xdr.TransactionResultCodeTxFeeBumpInnerSuccess, xdr.TransactionResultCodeTxFeeBumpInnerFailed:
		return hash, [32]byte(txResult.MustInnerResultPair().MustTransactionHash().MustValue()), true
	default:
		return hash, [32]byte{}, false
	}
}

// eventSlabs back the event slices of the transactions a walk keeps.
type eventSlabs struct {
	opEvents       slab[[]xdr.ContractEventView]
	contractEvents slab[xdr.ContractEventView]
	txEvents       slab[xdr.TransactionEventView]
}

// newEventSlabs returns slabs whose arrays hold presize elements, so a walk
// usually allocates one array per slab.
func newEventSlabs(presize int) eventSlabs {
	chunk := max(presize, minSlabChunk)
	return eventSlabs{
		opEvents:       newSlab[[]xdr.ContractEventView](chunk),
		contractEvents: newSlab[xdr.ContractEventView](chunk),
		txEvents:       newSlab[xdr.TransactionEventView](chunk),
	}
}

const minSlabChunk = 16

// locate walks the TransactionMeta at the start of buf, sizing every field
// once, and sets part's Meta, SorobanMetaExt and Events. It returns the offset
// in the meta of its DiagnosticEvents array, 0 when it has none.
//
//	union TransactionMeta switch (int v)
//	{
//	case 0:
//	    OperationMeta operations<>;
//	case 1:
//	    TransactionMetaV1 v1;
//	case 2:
//	    TransactionMetaV2 v2;
//	case 3:
//	    TransactionMetaV3 v3;
//	case 4:
//	    TransactionMetaV4 v4;
//	};
func (slabs *eventSlabs) locate(buf []byte, part *LedgerTxParts) (diagnosticsAt int) {
	var end int
	switch version := xdr.TransactionMetaView(buf).MustV(); version {
	case 0, 1, 2: //nolint:mnd // TransactionMeta versions without contract events
		end = sizeOf[xdr.TransactionMetaView](buf, 0)
	case 3: //nolint:mnd // TransactionMeta version
		end, diagnosticsAt = slabs.v3(buf, discriminantSize, part)
	case 4: //nolint:mnd // TransactionMeta version
		end, diagnosticsAt = slabs.v4(buf, discriminantSize, part)
	default:
		panic(&xdr.ViewError{
			Kind:   xdr.ViewErrUnknownDiscriminant,
			Detail: fmt.Sprintf("unsupported TransactionMeta V=%d", version),
		})
	}
	part.Meta = xdr.TransactionMetaView(buf[:end])
	part.Events = TxEvents{TransactionEvents: slabs.txEvents.cut(), OperationEvents: slabs.opEvents.cut()}
	return diagnosticsAt
}

// v3 walks the TransactionMetaV3 at pos in buf:
//
//	struct TransactionMetaV3
//	{
//	    ExtensionPoint ext;
//	    LedgerEntryChanges txChangesBefore;
//	    OperationMeta operations<>;
//	    LedgerEntryChanges txChangesAfter;
//	    SorobanTransactionMeta* sorobanMeta;
//	};
func (slabs *eventSlabs) v3(buf []byte, pos int, part *LedgerTxParts) (end, diagnosticsAt int) {
	pos += sizeOf[xdr.ExtensionPointView](buf, pos)                      // ext
	pos += sizeOf[xdr.LedgerEntryChangesView](buf, pos)                  // txChangesBefore
	pos += sizeOf[xdr.TransactionMetaV3OperationsView](buf, pos)         // operations
	pos += sizeOf[xdr.LedgerEntryChangesView](buf, pos)                  // txChangesAfter
	sorobanMetaOpt := xdr.TransactionMetaV3SorobanMetaOptView(buf[pos:]) // sorobanMeta
	pos += optionalFlagSize
	if _, present := sorobanMetaOpt.MustUnwrap(); !present {
		return pos, 0
	}
	return slabs.sorobanMeta(buf, pos, part)
}

// sorobanMeta walks the SorobanTransactionMeta at pos in buf:
//
//	struct SorobanTransactionMeta
//	{
//	    SorobanTransactionMetaExt ext;
//	    ContractEvent events<>;
//	    SCVal returnValue;
//	    DiagnosticEvent diagnosticEvents<>;
//	};
func (slabs *eventSlabs) sorobanMeta(buf []byte, pos int, part *LedgerTxParts) (end, diagnosticsAt int) {
	part.SorobanMetaExt = valueAt[xdr.SorobanTransactionMetaExtView](buf, pos) // ext, kept for FeesFromTxParts
	pos += len(part.SorobanMetaExt)
	pos = appendElements[xdr.SorobanTransactionMetaEventsView](&slabs.contractEvents, buf, pos) // events
	slabs.opEvents.append(slabs.contractEvents.cut())
	pos += sizeOf[xdr.ScValView](buf, pos) // returnValue
	diagnosticsAt = pos
	pos += sizeOf[xdr.SorobanTransactionMetaDiagnosticEventsView](buf, pos) // diagnosticEvents
	return pos, diagnosticsAt
}

// v4 walks the TransactionMetaV4 at pos in buf:
//
//	struct TransactionMetaV4
//	{
//	    ExtensionPoint ext;
//	    LedgerEntryChanges txChangesBefore;
//	    OperationMetaV2 operations<>;
//	    LedgerEntryChanges txChangesAfter;
//	    SorobanTransactionMetaV2* sorobanMeta;
//	    TransactionEvent events<>;
//	    DiagnosticEvent diagnosticEvents<>;
//	};
func (slabs *eventSlabs) v4(buf []byte, pos int, part *LedgerTxParts) (end, diagnosticsAt int) {
	pos += sizeOf[xdr.ExtensionPointView](buf, pos)                       // ext
	pos += sizeOf[xdr.LedgerEntryChangesView](buf, pos)                   // txChangesBefore
	opCount := xdr.TransactionMetaV4OperationsView(buf[pos:]).MustCount() // operations
	pos += arrayCountSize
	for range opCount {
		pos = slabs.operation(buf, pos)
	}
	pos += sizeOf[xdr.LedgerEntryChangesView](buf, pos)                  // txChangesAfter
	sorobanMetaOpt := xdr.TransactionMetaV4SorobanMetaOptView(buf[pos:]) // sorobanMeta
	pos += optionalFlagSize
	if _, present := sorobanMetaOpt.MustUnwrap(); present {
		pos = slabs.sorobanMetaV2(buf, pos, part)
	}
	pos = appendElements[xdr.TransactionMetaV4EventsView](&slabs.txEvents, buf, pos) // events
	diagnosticsAt = pos
	pos += sizeOf[xdr.TransactionMetaV4DiagnosticEventsView](buf, pos) // diagnosticEvents
	return pos, diagnosticsAt
}

// operation walks the OperationMetaV2 at pos in buf:
//
//	struct OperationMetaV2
//	{
//	    ExtensionPoint ext;
//	    LedgerEntryChanges changes;
//	    ContractEvent events<>;
//	};
func (slabs *eventSlabs) operation(buf []byte, pos int) int {
	pos += sizeOf[xdr.ExtensionPointView](buf, pos)                                      // ext
	pos += sizeOf[xdr.LedgerEntryChangesView](buf, pos)                                  // changes
	pos = appendElements[xdr.OperationMetaV2EventsView](&slabs.contractEvents, buf, pos) // events
	slabs.opEvents.append(slabs.contractEvents.cut())
	return pos
}

// sorobanMetaV2 walks the SorobanTransactionMetaV2 at pos in buf:
//
//	struct SorobanTransactionMetaV2
//	{
//	    SorobanTransactionMetaExt ext;
//	    SCVal* returnValue;
//	};
func (slabs *eventSlabs) sorobanMetaV2(buf []byte, pos int, part *LedgerTxParts) int {
	part.SorobanMetaExt = valueAt[xdr.SorobanTransactionMetaExtView](buf, pos) // ext, kept for FeesFromTxParts
	pos += len(part.SorobanMetaExt)
	return pos + sizeOf[xdr.SorobanTransactionMetaV2ReturnValueOptView](buf, pos) // returnValue
}

// xdrView is a generated XDR view type.
type xdrView interface {
	~[]byte
	MustRaw() []byte
}

// valueAt returns the value of type View at pos in buf. buf[pos:] runs to the
// end of buf; MustRaw trims it to exactly the value's bytes, by reading the
// value as its XDR type, and panics on malformed bytes, which walk returns as
// an error.
func valueAt[View xdrView](buf []byte, pos int) View {
	return View(View(buf[pos:]).MustRaw())
}

// sizeOf returns the size in bytes of the value of type View at pos in buf.
func sizeOf[View xdrView](buf []byte, pos int) int {
	return len(valueAt[View](buf, pos))
}

// appendElements appends to slab each element of the array of type Array at
// pos in buf, and returns the offset in buf past the array.
func appendElements[Array interface {
	~[]byte
	MustCount() int
}, Element xdrView](slab *slab[Element], buf []byte, pos int) int {
	count := Array(buf[pos:]).MustCount()
	pos += arrayCountSize
	for range count {
		element := valueAt[Element](buf, pos)
		slab.append(element)
		pos += len(element)
	}
	return pos
}

// slab carves many small slices out of a shared backing array. When the array
// is full it starts a new one; slices already handed out stay in the old one.
type slab[Element any] struct {
	buf   []Element // current backing array
	start int       // where the slice being built starts in buf
	chunk int       // size of each new backing array
}

func newSlab[Element any](chunk int) slab[Element] {
	return slab[Element]{buf: []Element{}, chunk: chunk} // not nil, so an empty cut is not nil either
}

func (slab *slab[Element]) append(element Element) {
	if len(slab.buf) == cap(slab.buf) {
		// The slices already cut keep the full array; the one being built moves.
		building := slab.buf[slab.start:]
		slab.buf = make([]Element, len(building), max(slab.chunk, 2*len(building)))
		copy(slab.buf, building)
		slab.start = 0
	}
	slab.buf = append(slab.buf, element)
}

// cut returns the slice built since the last cut. Its capacity equals its
// length, so appending to it cannot overwrite the next slice.
func (slab *slab[Element]) cut() []Element {
	out := slab.buf[slab.start:len(slab.buf):len(slab.buf)]
	slab.start = len(slab.buf)
	return out
}
