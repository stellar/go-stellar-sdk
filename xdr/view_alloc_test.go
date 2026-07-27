package xdr

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Escape/alloc discipline gates: views must stay allocation-free on every
// traversal path. NewXView allocates NOTHING; accessor chains, element
// access, and leaf reads allocate nothing; iteration may cost at most the
// per-range-loop closures the compiler materializes.

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
		sinkView = NewLedgerCloseMetaView(data)
	})
	require.Zero(t, escaping, "tier-1 parse is allocation-free (no walk exists to allocate)")
}

// TestAllocs_FieldsChain pins zero allocations for a deep accessor chain:
// arm entry, nested field accessors, and a leaf Value() read.
func TestAllocs_FieldsChain(t *testing.T) {
	data := allocLCM(t)
	v := NewLedgerCloseMetaView(data)
	chain := func() {
		seq, err := v.LedgerSequence() // Arm + accessor chain + leaf Value
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
	chain()
	allocs := testing.AllocsPerRun(200, chain)
	require.Zero(t, allocs, "accessor chain access must not allocate")
}

// TestAllocs_FullIteration pins the full-extract loop shape: iterate the tx
// array, open each element's bundle, descend into the meta's operations and
// consume each op's raw extent. Per-element and per-bundle work must be free;
// the only tolerated allocations are the iterator closures (one per range
// loop reached: 1 outer + 2 metas × 1 ops loop = 3 per run).
func TestAllocs_FullIteration(t *testing.T) {
	data := allocLCM(t)
	v := NewLedgerCloseMetaView(data)
	v1, err := v.ArmV1()
	require.NoError(t, err)
	tp, err := v1.TxProcessing()
	require.NoError(t, err)

	run := func() {
		sc1 := tp.Scan()
		for elem, err := range scanSeq2(sc1.Next, sc1.Cur, sc1.Err) {
			if err != nil {
				panic(err)
			}
			m, err := elem.TxApplyProcessing()
			if err != nil {
				panic(err)
			}
			v3, err := m.ArmV3()
			if err != nil {
				panic(err)
			}
			ops, err := v3.Operations()
			if err != nil {
				panic(err)
			}
			sc2 := ops.Scan()
			for op, err := range scanSeq2(sc2.Next, sc2.Cur, sc2.Err) {
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
