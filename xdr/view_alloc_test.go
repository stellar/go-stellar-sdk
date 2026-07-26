package xdr

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Escape/alloc discipline gates (spec §Escape/alloc discipline): struct views
// must stay allocation-free on every traversal path. The entire per-parse
// allocation budget is one *walk (in ParseXView) plus at most one iterator
// closure per range loop; bundle access, element access, Fields() calls, and
// leaf reads allocate NOTHING.

// Package-level sinks defeat dead-code elimination without introducing
// closure captures (globals are not captured, so loop-body closures stay
// capture-free and allocation-free).
var (
	sinkU32 uint32
	sinkI64 int64
	sinkInt int
	sinkB   byte
)

// resultPair returns a marshalable TransactionResultPair (void result arm).
func resultPair() TransactionResultPair {
	return TransactionResultPair{
		Result: TransactionResult{
			Result: TransactionResultResult{Code: TransactionResultCodeTxInternalError},
		},
	}
}

// allocLCM builds a deterministic two-tx LedgerCloseMeta V1 whose metas carry
// V3 interiors with operations and changes — enough structure to exercise
// bundles, iterators, and the walk on every path below.
func allocLCM(t *testing.T) []byte {
	t.Helper()
	rv := ScVal{Type: ScValTypeScvVoid}
	meta := func() TransactionMeta {
		return TransactionMeta{V: 3, V3: &TransactionMetaV3{
			Operations: []OperationMeta{{}, {}},
			SorobanMeta: &SorobanTransactionMeta{
				ReturnValue: rv,
			},
		}}
	}
	lcm := LedgerCloseMeta{
		V: 1,
		V1: &LedgerCloseMetaV1{
			LedgerHeader: LedgerHeaderHistoryEntry{
				Header: LedgerHeader{LedgerSeq: 4242, ScpValue: StellarValue{CloseTime: 1234567}},
			},
			TxSet: GeneralizedTransactionSet{
				V:       1,
				V1TxSet: &TransactionSetV1{},
			},
			TxProcessing: []TransactionResultMeta{
				{Result: resultPair(), TxApplyProcessing: meta()},
				{Result: resultPair(), TxApplyProcessing: meta()},
			},
		},
	}
	data, err := lcm.MarshalBinary()
	require.NoError(t, err)
	return data
}

// sinkView forces the parsed view (and so its walk) to escape, measuring the
// worst-case per-parse budget. A non-escaping parse stack-allocates the walk
// and costs nothing at all (asserted separately).
var sinkView LedgerCloseMetaView

// TestAllocs_Parse pins the per-parse budget: at most one allocation — the
// shared *walk — even when the view escapes; zero when it does not.
func TestAllocs_Parse(t *testing.T) {
	data := allocLCM(t)

	escaping := testing.AllocsPerRun(200, func() {
		sinkView = ParseLedgerCloseMetaView(data)
	})
	require.Equal(t, 1.0, escaping, "an escaping ParseXView must allocate exactly the walk")

	local := testing.AllocsPerRun(200, func() {
		v := ParseLedgerCloseMetaView(data)
		sinkInt += len(v.d)
	})
	require.Zero(t, local, "a non-escaping ParseXView must stack-allocate the walk")

	// Detached parse carries no walk at all: zero allocations even when the
	// view escapes.
	detached := testing.AllocsPerRun(200, func() {
		sinkView = ParseLedgerCloseMetaViewDetached(data)
	})
	require.Zero(t, detached, "ParseXViewDetached must never allocate")
}

// TestAllocs_FieldsChain pins zero allocations for a deep Fields() chain:
// arm entry, three nested bundles, and a leaf Value() read.
func TestAllocs_FieldsChain(t *testing.T) {
	data := allocLCM(t)
	v := ParseLedgerCloseMetaView(data)
	chain := func() {
		seq, err := v.LedgerSequence() // Arm + Fields chain + leaf Value
		if err != nil {
			panic(err)
		}
		sinkU32 += seq
		ct, err := v.LedgerCloseTime()
		if err != nil {
			panic(err)
		}
		sinkI64 += ct
	}
	chain() // warm the walk: record-hit paths must also be allocation-free
	allocs := testing.AllocsPerRun(200, chain)
	require.Zero(t, allocs, "bundle chain access must not allocate")
}

// TestAllocs_FieldsChain_Detached is TestAllocs_FieldsChain without a walk:
// the w == nil paths must be equally allocation-free.
func TestAllocs_FieldsChain_Detached(t *testing.T) {
	data := allocLCM(t)
	v := LedgerCloseMetaView{view{d: data}}
	allocs := testing.AllocsPerRun(200, func() {
		seq, err := v.LedgerSequence()
		if err != nil {
			panic(err)
		}
		sinkU32 += seq
	})
	require.Zero(t, allocs, "detached bundle chain access must not allocate")
}

// TestAllocs_FullIteration pins the full-extract loop shape: iterate the tx
// array, open each element's bundle, descend into the meta's operations and
// consume each op's raw extent. Per-element and per-bundle work must be free;
// the only tolerated allocations are the iterator closures (one per range
// loop reached: 1 outer + 2 metas × 1 ops loop = 3 per run).
func TestAllocs_FullIteration(t *testing.T) {
	data := allocLCM(t)
	v := ParseLedgerCloseMetaView(data)
	v1, err := v.ArmV1()
	require.NoError(t, err)
	f, err := v1.Fields()
	require.NoError(t, err)
	tp, err := f.TxProcessing()
	require.NoError(t, err)

	run := func() {
		for elem, err := range tp.All() {
			if err != nil {
				panic(err)
			}
			ef, _ := elem.Fields()
			m, err := ef.TxApplyProcessing()
			if err != nil {
				panic(err)
			}
			v3, err := m.ArmV3()
			if err != nil {
				panic(err)
			}
			mf, _ := v3.Fields()
			ops, err := mf.Operations()
			if err != nil {
				panic(err)
			}
			for op, err := range ops.All() {
				if err != nil {
					panic(err)
				}
				r, err := op.Raw()
				if err != nil {
					panic(err)
				}
				sinkB += r[0]
			}
			r, err := elem.Raw()
			if err != nil {
				panic(err)
			}
			sinkInt += len(r)
		}
	}
	run()
	allocs := testing.AllocsPerRun(100, run)
	require.LessOrEqual(t, allocs, 3.0,
		"full iteration may allocate at most one iterator closure per range loop")
	t.Logf("full iteration allocs/run: %v", allocs)
}
