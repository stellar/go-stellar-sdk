package ingest

import (
	"encoding/binary"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// offsetIn returns the offset of v within base; v must reslice base.
func offsetIn(t *testing.T, base, v []byte) int {
	t.Helper()
	off := cap(base) - cap(v)
	require.True(t, off >= 0 && off+len(v) <= len(base), "view is not a slice of the buffer")
	if len(v) > 0 {
		require.True(t, &base[off] == &v[0], "view is not a slice of the buffer")
	}
	return off
}

// wantEntry derives, with the generated accessors alone, the entry locate must
// record for meta.
func wantEntry(t *testing.T, meta xdr.TransactionMetaView) []uint32 {
	t.Helper()
	entry := []uint32{uint32(meta.MustV()), uint32(len(meta.MustRaw())), 0, 0} //nolint:gosec // small test values
	switch meta.MustV() {
	case 3:
		sm, ok := meta.MustV3().MustSorobanMeta().MustUnwrap()
		if !ok {
			return entry
		}
		entry[entrySoroban] = offset32(t, meta, sm)
		entry[entryDiagnostics] = offset32(t, meta, sm.MustDiagnosticEvents())
		entry = wantArray(t, meta, entry, sm.MustEvents(), sm.MustEvents().MustAll())
	case 4:
		v4 := meta.MustV4()
		if sm, ok := v4.MustSorobanMeta().MustUnwrap(); ok {
			entry[entrySoroban] = offset32(t, meta, sm)
		}
		entry[entryDiagnostics] = offset32(t, meta, v4.MustDiagnosticEvents())
		ops := v4.MustOperations()
		entry = append(entry, uint32(ops.MustCount())) //nolint:gosec // test buffers are small
		for k := range ops.MustCount() {
			evs := ops.MustAt(k).MustEvents()
			entry = wantArray(t, meta, entry, evs, evs.MustAll())
		}
		entry = wantArray(t, meta, entry, v4.MustEvents(), v4.MustEvents().MustAll())
	default:
	}
	return entry
}

func offset32(t *testing.T, base, v []byte) uint32 {
	t.Helper()
	return uint32(offsetIn(t, base, v)) //nolint:gosec // test buffers are small
}

// wantArray appends the record of the array arr with elements elems.
func wantArray[E ~[]byte](t *testing.T, meta []byte, entry []uint32, arr []byte, elems []E) []uint32 {
	t.Helper()
	first := offset32(t, meta, arr) + uint32(len(xdr.Uint32View(arr).MustRaw())) //nolint:gosec // test buffers are small
	entry = append(entry, uint32(len(elems)), first)                             //nolint:gosec // test buffers are small
	for _, e := range elems {
		entry = append(entry, offset32(t, meta, e)+uint32(len(e))) //nolint:gosec // test buffers are small
	}
	return entry
}

func TestOffsetRecord_MatchesGeneratedAccessors(t *testing.T) {
	for i, meta := range equivalenceMetas() {
		t.Run(fmt.Sprintf("meta%d-v%d", i, meta.V), func(t *testing.T) {
			raw, err := meta.MarshalBinary()
			require.NoError(t, err)
			// Trailing bytes must not change what is located.
			padded := append(append([]byte{}, raw...), 0xff, 0xff, 0xff, 0xff)

			var rec offsetRecord
			var got xdr.TransactionMetaView
			require.NoError(t, xdr.TryVoid(func() { got = rec.locate(padded) }))
			assert.Len(t, got, len(raw), "located meta must be trimmed to its wire extent")
			assert.Equal(t, wantEntry(t, xdr.TransactionMetaView(padded)), rec.offsets)
		})
	}
}

// The walk hand-writes the field order of these generated structs; a schema
// change must fail here before it can mis-locate a field.
func TestMirroredLayouts_MatchGeneratedTypes(t *testing.T) {
	fields := func(v any) []string {
		typ := reflect.TypeOf(v)
		var out []string
		for i := range typ.NumField() {
			out = append(out, typ.Field(i).Name+" "+typ.Field(i).Type.Name())
		}
		return out
	}
	assert.Equal(t, []string{
		"View TransactionResultPairView", "TransactionHash HashView", "Result TransactionResultView",
	}, fields(xdr.TransactionResultPairFields{}))
	assert.Equal(t, []string{
		"View TransactionResultMetaView", "Result TransactionResultPairView",
		"FeeProcessing LedgerEntryChangesView", "TxApplyProcessing TransactionMetaView",
	}, fields(xdr.TransactionResultMetaFields{}))
	assert.Equal(t, []string{
		"View TransactionResultMetaV1View", "Ext ExtensionPointView", "Result TransactionResultPairView",
		"FeeProcessing LedgerEntryChangesView", "TxApplyProcessing TransactionMetaView",
		"PostTxApplyFeeProcessing LedgerEntryChangesView",
	}, fields(xdr.TransactionResultMetaV1Fields{}))
	assert.Equal(t, []string{
		"View TransactionMetaV3View", "Ext ExtensionPointView", "TxChangesBefore LedgerEntryChangesView",
		"Operations TransactionMetaV3OperationsView", "TxChangesAfter LedgerEntryChangesView",
		"SorobanMeta TransactionMetaV3SorobanMetaOptView",
	}, fields(xdr.TransactionMetaV3Fields{}))
	assert.Equal(t, []string{
		"View SorobanTransactionMetaView", "Ext SorobanTransactionMetaExtView",
		"Events SorobanTransactionMetaEventsView", "ReturnValue ScValView",
		"DiagnosticEvents SorobanTransactionMetaDiagnosticEventsView",
	}, fields(xdr.SorobanTransactionMetaFields{}))
	assert.Equal(t, []string{
		"View TransactionMetaV4View", "Ext ExtensionPointView", "TxChangesBefore LedgerEntryChangesView",
		"Operations TransactionMetaV4OperationsView", "TxChangesAfter LedgerEntryChangesView",
		"SorobanMeta TransactionMetaV4SorobanMetaOptView", "Events TransactionMetaV4EventsView",
		"DiagnosticEvents TransactionMetaV4DiagnosticEventsView",
	}, fields(xdr.TransactionMetaV4Fields{}))
	assert.Equal(t, []string{
		"View OperationMetaV2View", "Ext ExtensionPointView", "Changes LedgerEntryChangesView",
		"Events OperationMetaV2EventsView",
	}, fields(xdr.OperationMetaV2Fields{}))

	returns := func(v any, method string) string {
		m, ok := reflect.TypeOf(v).MethodByName(method)
		require.True(t, ok, method)
		return m.Type.Out(0).Name()
	}
	assert.Equal(t, "TransactionMetaV3View", returns(xdr.TransactionMetaView{}, "V3"))
	assert.Equal(t, "TransactionMetaV4View", returns(xdr.TransactionMetaView{}, "V4"))
	assert.Equal(t, "TransactionResultMetaView", returns(xdr.LedgerCloseMetaV0TxProcessingView{}, "At"))
	assert.Equal(t, "TransactionResultMetaView", returns(xdr.LedgerCloseMetaV1TxProcessingView{}, "At"))
	assert.Equal(t, "TransactionResultMetaV1View", returns(xdr.LedgerCloseMetaV2TxProcessingView{}, "At"))
}

// Each V4 operation's events are the ones the generated random-access
// accessor finds at that index, in slices allocated at their final length.
func TestEventsFromTxParts_V4OperationsAtTheirOffsets(t *testing.T) {
	for _, counts := range [][]int{{1}, {0}, {3, 0, 5, 1, 0, 2}, {0, 0, 0}, {7, 7}, {}} {
		t.Run(fmt.Sprintf("ops=%v", counts), func(t *testing.T) {
			lcm := buildEventsLCM(t, 9100, 1_700_000_000, []xdr.TransactionMeta{equivalenceV4Meta(counts, 3, 2, false)})
			raw, err := lcm.MarshalBinary()
			require.NoError(t, err)
			parts, err := ExtractLedgerTxParts(xdr.LedgerCloseMetaView(raw))
			require.NoError(t, err)
			events, err := EventsFromTxParts(parts)
			require.NoError(t, err)

			got := events[0].OperationEvents
			require.Len(t, got, len(counts))
			assert.Equal(t, len(got), cap(got))
			ops := parts[0].Meta.MustV4().MustOperations()
			for k, n := range counts {
				assertSameElements(t, ops.MustAt(k).MustEvents().MustAll(), got[k], fmt.Sprintf("operation %d", k))
				assert.Len(t, got[k], n)
				assert.Equal(t, len(got[k]), cap(got[k]), "operation %d", k)
			}
		})
	}
}

// Every per-ledger and per-transaction slice the extractors return has its
// final length as its capacity, which append growth cannot produce for these
// counts.
func TestViewExtractorsPresizeTheirOutput(t *testing.T) {
	metas := []xdr.TransactionMeta{
		equivalenceV4Meta([]int{3, 0, 5, 1, 0, 2}, 3, 3, true),
		vMetaV3SorobanWithDiag(
			[]xdr.ContractEvent{vContractEvent("a"), vContractEvent("b"), vContractEvent("c")},
			[]xdr.DiagnosticEvent{vDiagEvent("d0"), vDiagEvent("d1"), vDiagEvent("d2")},
		),
		equivalenceV4Meta([]int{5}, 0, 0, false),
		equivalenceV4Meta([]int{3, 3}, 5, 0, false),
		vMetaV3Soroban([]xdr.ContractEvent{vContractEvent("x"), vContractEvent("y"), vContractEvent("z")}),
	}
	lcm := buildEventsLCM(t, 9110, 1_700_000_000, metas)
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	view := xdr.LedgerCloseMetaView(raw)

	parts, err := ExtractLedgerTxParts(view)
	require.NoError(t, err)
	require.Len(t, parts, len(metas))
	assert.Equal(t, len(parts), cap(parts))

	events, err := EventsFromTxParts(parts)
	require.NoError(t, err)
	assert.Equal(t, len(events), cap(events))
	for i, te := range events {
		assert.Equal(t, len(te.TransactionEvents), cap(te.TransactionEvents), "tx %d", i)
		assert.Equal(t, len(te.OperationEvents), cap(te.OperationEvents), "tx %d", i)
		for k, group := range te.OperationEvents {
			assert.Equal(t, len(group), cap(group), "tx %d operation %d", i, k)
		}
	}

	txs, err := LedgerTransactionViewRange(view, 0, 0, viewTestPassphrase)
	require.NoError(t, err)
	for i, tx := range txs {
		assert.Equal(t, len(tx.DiagnosticEvents), cap(tx.DiagnosticEvents), "tx %d", i)
		assert.Equal(t, len(tx.TransactionEvents), cap(tx.TransactionEvents), "tx %d", i)
		for k, group := range tx.ContractEvents {
			assert.Equal(t, len(group), cap(group), "tx %d operation %d", i, k)
		}
	}
}

// txProcessingElements returns the ledger's TxProcessing elements with the
// generated accessors.
func txProcessingElements(t *testing.T, view xdr.LedgerCloseMetaView) [][]byte {
	t.Helper()
	var out [][]byte
	switch view.MustV() {
	case 0:
		for _, e := range view.MustV0().MustTxProcessing().MustAll() {
			out = append(out, e)
		}
	case 1:
		for _, e := range view.MustV1().MustTxProcessing().MustAll() {
			out = append(out, e)
		}
	case 2:
		for _, e := range view.MustV2().MustTxProcessing().MustAll() {
			out = append(out, e)
		}
	default:
		t.Fatalf("unsupported LCM version")
	}
	return out
}

func TestExtractLedgerTxParts_ElementSpans(t *testing.T) {
	for _, version := range []int32{0, 1, 2} {
		t.Run(fmt.Sprintf("lcmV%d", version), func(t *testing.T) {
			v4 := sorobanTx(t, "x")
			v4.meta = equivalenceV4Meta([]int{2, 0}, 1, 1, true)
			txs := []txWithHash{
				sorobanTx(t, "a"),
				feeBumpTx(t, vMetaV3Soroban([]xdr.ContractEvent{vContractEvent("fb")})),
				v4,
				txV0(t, nil, 5),
			}
			raw, err := buildLCM(t, version, 9120, 1_700_000_000, txs, true).MarshalBinary()
			require.NoError(t, err)
			view := xdr.LedgerCloseMetaView(raw)
			parts, err := ExtractLedgerTxParts(view)
			require.NoError(t, err)

			elems := txProcessingElements(t, view)
			require.Len(t, parts, len(elems))
			hashAt := 0
			if version == 2 {
				hashAt = xdrWord
			}
			for i, part := range parts {
				assert.Equal(t, offsetIn(t, raw, elems[i]), part.ElemStart, "tx %d", i)
				assert.Equal(t, part.ElemStart+len(elems[i]), part.ElemEnd, "tx %d", i)
				if i > 0 {
					assert.Equal(t, parts[i-1].ElemEnd, part.ElemStart, "elements %d and %d must tile", i-1, i)
				}
				elem := raw[part.ElemStart:part.ElemEnd]
				assert.Equal(t, part.Hash[:], elem[hashAt:hashAt+32], "tx %d", i)
				for name, v := range map[string][]byte{"Result": part.Result, "Meta": part.Meta} {
					off := offsetIn(t, raw, v)
					assert.GreaterOrEqual(t, off, part.ElemStart, "tx %d %s", i, name)
					assert.LessOrEqual(t, off+len(v), part.ElemEnd, "tx %d %s", i, name)
				}
			}
		})
	}
}

// assertSameBytes asserts two element slices hold equal bytes, wherever they
// live.
func assertSameBytes[W, G ~[]byte](t *testing.T, want []W, got []G, ctx string) {
	t.Helper()
	require.Len(t, got, len(want), ctx)
	for i := range want {
		assert.Equal(t, []byte(want[i]), []byte(got[i]), "%s[%d]", ctx, i)
	}
}

// Parts built by hand, and walked parts whose Meta was copied out of the
// ledger, give the same products as the walked parts; a walked part whose Meta
// was shortened or replaced by another meta is an error.
func TestProducts_PartsNotStraightFromTheWalk(t *testing.T) {
	for name, raw := range equivalenceLedgers(t) {
		t.Run(name, func(t *testing.T) {
			parts, err := ExtractLedgerTxParts(xdr.LedgerCloseMetaView(raw))
			require.NoError(t, err)
			wantEvents, err := EventsFromTxParts(parts)
			require.NoError(t, err)
			wantFees, err := FeesFromTxParts(parts)
			require.NoError(t, err)

			byHand := make([]LedgerTxParts, len(parts))
			untrimmed := make([]LedgerTxParts, len(parts))
			copied := make([]LedgerTxParts, len(parts))
			for i, p := range parts {
				byHand[i] = LedgerTxParts{Hash: p.Hash, InnerHash: p.InnerHash, FeeBump: p.FeeBump, Result: p.Result, Meta: p.Meta}
				untrimmed[i] = byHand[i]
				untrimmed[i].Meta = xdr.TransactionMetaView(raw[offsetIn(t, raw, p.Meta):])
				copied[i] = p
				copied[i].Meta = p.Meta.MustCopy()
			}
			for label, in := range map[string][]LedgerTxParts{"by hand": byHand, "by hand, untrimmed": untrimmed, "copied meta": copied} {
				gotEvents, err := EventsFromTxParts(in)
				require.NoError(t, err, label)
				require.Len(t, gotEvents, len(wantEvents), label)
				for i := range wantEvents {
					ctx := fmt.Sprintf("%s tx %d", label, i)
					assertSameBytes(t, wantEvents[i].TransactionEvents, gotEvents[i].TransactionEvents, ctx)
					require.Len(t, gotEvents[i].OperationEvents, len(wantEvents[i].OperationEvents), ctx)
					for k := range wantEvents[i].OperationEvents {
						assertSameBytes(t, wantEvents[i].OperationEvents[k], gotEvents[i].OperationEvents[k], ctx)
					}
				}
				gotFees, err := FeesFromTxParts(in)
				require.NoError(t, err, label)
				assert.Equal(t, wantFees, gotFees, label)
			}

			other := map[int32]xdr.TransactionMetaView{}
			for _, p := range parts {
				other[p.Meta.MustV()] = p.Meta
			}
			for i, p := range parts {
				entry, err := p.entry(nil)
				require.NoError(t, err)
				var changed []LedgerTxParts
				if at := entry[entryDiagnostics]; at != 0 {
					shortened := p
					shortened.Meta = p.Meta[:at-1]
					changed = append(changed, shortened)
				}
				for v, meta := range other {
					if v != p.Meta.MustV() {
						replaced := p
						replaced.Meta = meta
						changed = append(changed, replaced)
					}
				}
				for _, c := range changed {
					_, err = EventsFromTxParts([]LedgerTxParts{c})
					require.ErrorContains(t, err, "not the meta the walk located", "tx %d", i)
					_, err = FeesFromTxParts([]LedgerTxParts{c})
					require.ErrorContains(t, err, "not the meta the walk located", "tx %d", i)
				}
			}
		})
	}
}

// corruptWord overwrites the big-endian uint32 at the given offset.
func corruptWord(raw []byte, at int, v uint32) []byte {
	out := append([]byte{}, raw...)
	binary.BigEndian.PutUint32(out[at:], v)
	return out
}

func TestExtractors_MalformedInputIsAnError(t *testing.T) {
	v3 := vMetaV3SorobanWithDiag([]xdr.ContractEvent{vContractEvent("a")}, []xdr.DiagnosticEvent{vDiagEvent("d")})
	v4 := equivalenceV4Meta([]int{2}, 1, 1, true)
	raw, err := buildEventsLCM(t, 9130, 1_700_000_000, []xdr.TransactionMeta{v3, v4}).MarshalBinary()
	require.NoError(t, err)
	view := xdr.LedgerCloseMetaView(raw)
	parts, err := ExtractLedgerTxParts(view)
	require.NoError(t, err)
	metaAt := func(i int) int { return offsetIn(t, raw, parts[i].Meta) }
	v3Meta, v4Meta := parts[0].Meta.MustV3(), parts[1].Meta.MustV4()

	txProcessing := offsetIn(t, raw, view.MustV2().MustTxProcessing())
	cases := map[string][]byte{
		"v3 SorobanMeta flag 2": corruptWord(raw, metaAt(0)+offsetIn(t, parts[0].Meta, v3Meta.MustSorobanMeta()), 2),
		"v4 SorobanMeta flag 2": corruptWord(raw, metaAt(1)+offsetIn(t, parts[1].Meta, v4Meta.MustSorobanMeta()), 2),
		"v3 event count": corruptWord(raw, metaAt(0)+offsetIn(t, parts[0].Meta,
			mustUnwrapped(v3Meta.MustSorobanMeta().MustUnwrap()).MustEvents()), 1<<30),
		"v4 operation count": corruptWord(raw, metaAt(1)+offsetIn(t, parts[1].Meta, v4Meta.MustOperations()), 1<<30),
		"v4 operation event count": corruptWord(raw, metaAt(1)+offsetIn(t, parts[1].Meta,
			v4Meta.MustOperations().MustAt(0).MustEvents()), 1<<30),
		"v4 event count":            corruptWord(raw, metaAt(1)+offsetIn(t, parts[1].Meta, v4Meta.MustEvents()), 1<<30),
		"TxProcessing count 2^31":   corruptWord(raw, txProcessing, 1<<31),
		"TxProcessing count 2^30":   corruptWord(raw, txProcessing, 1<<30),
		"TxProcessing count +1":     corruptWord(raw, txProcessing, uint32(len(parts)+1)), //nolint:gosec // small
		"meta version 5":            corruptWord(raw, metaAt(1), 5),
		"diagnostic event count v4": corruptWord(raw, metaAt(1)+offsetIn(t, parts[1].Meta, v4Meta.MustDiagnosticEvents()), 1<<30),
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			badView := xdr.LedgerCloseMetaView(bad)
			_, err := ExtractLedgerTxParts(badView)
			require.Error(t, err)
			_, err = LedgerTransactionViewRange(badView, 0, 0, viewTestPassphrase)
			require.Error(t, err)
			// An absent hash makes the lookup walk every element.
			_, _, err = LedgerTransactionViewByHash(badView, [32]byte{0xde, 0xad}, viewTestPassphrase)
			require.Error(t, err)
		})
	}
}

func mustUnwrapped[V any](v V, ok bool) V {
	if !ok {
		panic("absent optional")
	}
	return v
}

// Every truncation of a ledger is either an error or, past TxProcessing, the
// same parts; nothing panics.
func TestExtractors_TruncatedLedgers(t *testing.T) {
	for _, version := range []int32{0, 1, 2} {
		t.Run(fmt.Sprintf("lcmV%d", version), func(t *testing.T) {
			v4 := sorobanTx(t, "x")
			v4.meta = equivalenceV4Meta([]int{1, 0}, 1, 1, true)
			txs := []txWithHash{
				v4,
				feeBumpTx(t, vMetaV3SorobanWithDiag(
					[]xdr.ContractEvent{vContractEvent("e")}, []xdr.DiagnosticEvent{vDiagEvent("d")})),
			}
			raw, err := buildLCM(t, version, 9140, 1_700_000_000, txs, false).MarshalBinary()
			require.NoError(t, err)
			whole, err := ExtractLedgerTxParts(xdr.LedgerCloseMetaView(raw))
			require.NoError(t, err)
			end := whole[len(whole)-1].ElemEnd

			for n := range len(raw) {
				cut := xdr.LedgerCloseMetaView(raw[:n:n])
				parts, err := ExtractLedgerTxParts(cut)
				if n < end {
					require.Error(t, err, "cut at %d", n)
				} else {
					require.NoError(t, err, "cut at %d", n)
					_, err = EventsFromTxParts(parts)
					require.NoError(t, err, "cut at %d", n)
					_, err = FeesFromTxParts(parts)
					require.NoError(t, err, "cut at %d", n)
				}
				_, _ = LedgerTransactionViewRange(cut, 0, 0, viewTestPassphrase)
				_, _, _ = LedgerTransactionViewByHash(cut, whole[1].Hash, viewTestPassphrase)
			}
		})
	}
}

// ExtractLedgerTxParts allocates the parts, the record and its backing, and
// nothing per transaction.
func TestExtractLedgerTxParts_Allocations(t *testing.T) {
	allocs := func(raw []byte) float64 {
		view := xdr.LedgerCloseMetaView(raw)
		return testing.AllocsPerRun(20, func() {
			if _, err := ExtractLedgerTxParts(view); err != nil {
				panic(err)
			}
		})
	}
	for _, n := range []int{4, 256} {
		txs := make([]txWithHash, n)
		for i := range txs {
			txs[i] = sorobanTx(t, "x")
			txs[i].meta = equivalenceV4Meta([]int{1}, 1, 1, true)
		}
		raw, err := buildLCM(t, 2, 9150, 1_700_000_000, txs, false).MarshalBinary()
		require.NoError(t, err)
		assert.InDelta(t, 3, allocs(raw), 0, "%d transactions", n)
	}
	pubnet, err := os.ReadFile("../xdr/testdata/ledger_58752000.bin")
	require.NoError(t, err)
	assert.InDelta(t, 3, allocs(pubnet), 0, "pubnet ledger")
}

func FuzzExtractors(f *testing.F) {
	for _, version := range []int32{0, 1, 2} {
		v4 := sorobanTx(f, "x")
		v4.meta = equivalenceV4Meta([]int{1, 0}, 1, 1, true)
		txs := []txWithHash{
			v4,
			feeBumpTx(f, vMetaV3SorobanWithDiag(
				[]xdr.ContractEvent{vContractEvent("e")}, []xdr.DiagnosticEvent{vDiagEvent("d")})),
			feeTxV1(f, []xdr.Operation{feeBumpSequenceOp()}, false, feeMetaV2(), 100),
		}
		raw, err := buildLCM(f, version, 9160, 1_700_000_000, txs, false).MarshalBinary()
		require.NoError(f, err)
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		view := xdr.LedgerCloseMetaView(raw)
		_, _ = LedgerTransactionViewRange(view, 0, 0, viewTestPassphrase)
		parts, err := ExtractLedgerTxParts(view)
		if err != nil {
			return
		}
		ref := refLedger(t, raw)
		require.Len(t, parts, len(ref))
		events, err := EventsFromTxParts(parts)
		require.NoError(t, err)
		for i, tx := range ref {
			assert.Equal(t, tx.hash, parts[i].Hash)
			assertSameView(t, tx.meta, parts[i].Meta, "Meta")
			assertSameElements(t, tx.txEvents, events[i].TransactionEvents, "TransactionEvents")
			assertSameGroups(t, tx.opEvents, events[i].OperationEvents, "OperationEvents")
		}
		_, _ = FeesFromTxParts(parts)
	})
}
