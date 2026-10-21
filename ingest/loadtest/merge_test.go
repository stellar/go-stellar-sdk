package loadtest

import (
	"crypto/sha256"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/gxdr"
	"github.com/stellar/go-stellar-sdk/randxdr"
	"github.com/stellar/go-stellar-sdk/xdr"
)

var testBaseFees = []*xdr.Int64{nil, fee(100), fee(200)}

func fee(v int64) *xdr.Int64 { f := xdr.Int64(v); return &f }

func marshal(t *testing.T, v interface{ MarshalBinary() ([]byte, error) }) []byte {
	raw, err := v.MarshalBinary()
	require.NoError(t, err)
	return raw
}

func deepCopy(t *testing.T, lcm xdr.LedgerCloseMeta) xdr.LedgerCloseMeta {
	var out xdr.LedgerCloseMeta
	require.NoError(t, out.UnmarshalBinary(marshal(t, lcm)))
	return out
}

func v1TxSet(lcm xdr.LedgerCloseMeta) *xdr.TransactionSetV1 { return lcm.V2.TxSet.V1TxSet }

// randomLedger returns a random LedgerCloseMeta.
func randomLedger(t *testing.T, gen randxdr.Generator) xdr.LedgerCloseMeta {
	shape := &gxdr.LedgerCloseMeta{}
	gen.Next(shape, randxdr.LedgerCloseMetaPresets)
	var lcm xdr.LedgerCloseMeta
	require.NoError(t, gxdr.Convert(shape, &lcm))
	return lcm
}

func scrambleSeq(s uint32) uint32 { return s*2654435761 + 12345 }

// normalizedLedger returns a random V2 LedgerCloseMeta whose tx set has the
// real phase layout, a component phase and a parallel Soroban phase, with
// component base fees drawn from testBaseFees so that merging regroups them.
func normalizedLedger(t *testing.T, gen randxdr.Generator) xdr.LedgerCloseMeta {
	for {
		lcm := randomLedger(t, gen)
		if lcm.V != 2 {
			continue
		}
		ts := v1TxSet(lcm)
		var comps []xdr.TxSetComponent
		var par *xdr.ParallelTxsComponent
		for _, ph := range ts.Phases {
			switch ph.V {
			case 0:
				comps = append(comps, *ph.V0Components...)
			case 1:
				if par == nil {
					p := *ph.ParallelTxsComponent
					par = &p
				}
			}
		}
		for i := range comps {
			comps[i].TxsMaybeDiscountedFee.BaseFee = testBaseFees[i%len(testBaseFees)]
		}
		if par == nil {
			par = &xdr.ParallelTxsComponent{}
		}
		ts.Phases = []xdr.TransactionPhase{{V: 0, V0Components: &comps}, {V: 1, ParallelTxsComponent: par}}
		return deepCopy(t, lcm)
	}
}

func feeKey(f *xdr.Int64) string {
	if f == nil {
		return "nil"
	}
	return strconv.FormatInt(int64(*f), 10)
}

// referenceMerge is MergeLedgerBytes written against decoded values.
func referenceMerge(t *testing.T, inputs []xdr.LedgerCloseMeta, opts MergeOptions) xdr.LedgerCloseMeta {
	ins := make([]xdr.LedgerCloseMeta, len(inputs))
	for i, in := range inputs {
		ins[i] = deepCopy(t, in)
		if opts.RemapLedgerSeq != nil {
			require.NoError(t, UpdateLedgerSeqInLedgerEntries(&ins[i], func(s uint32) uint32 { return opts.RemapLedgerSeq(i, s) }))
		}
	}
	out := deepCopy(t, ins[opts.HeaderFrom])
	prevHash := ins[0].PreviousLedgerHash()
	if opts.PreviousLedgerHash != nil {
		prevHash = *opts.PreviousLedgerHash
	}

	phases := referencePhases(ins, opts.HeaderFrom)
	txSet := xdr.GeneralizedTransactionSet{V: 1, V1TxSet: &xdr.TransactionSetV1{PreviousLedgerHash: prevHash, Phases: phases}}

	results := xdr.TransactionResultSet{Results: []xdr.TransactionResultPair{}}
	v2 := out.V2
	v2.TxSet = txSet
	v2.TxProcessing, v2.UpgradesProcessing, v2.EvictedKeys = nil, nil, nil
	for _, in := range ins {
		v2.TxProcessing = append(v2.TxProcessing, in.V2.TxProcessing...)
		v2.UpgradesProcessing = append(v2.UpgradesProcessing, in.V2.UpgradesProcessing...)
		v2.EvictedKeys = append(v2.EvictedKeys, in.V2.EvictedKeys...)
		for _, tx := range in.V2.TxProcessing {
			results.Results = append(results.Results, tx.Result)
		}
	}
	header := &v2.LedgerHeader
	header.Header.PreviousLedgerHash = prevHash
	header.Header.ScpValue.TxSetHash = sha256.Sum256(marshal(t, txSet))
	header.Header.TxSetResultHash = sha256.Sum256(marshal(t, results))
	if opts.LedgerSeq != 0 {
		header.Header.LedgerSeq = xdr.Uint32(opts.LedgerSeq)
	}
	header.Hash = sha256.Sum256(marshal(t, header.Header))
	return out
}

// referencePhases merges the inputs' tx set phases the way MergeLedgerBytes does.
func referencePhases(ins []xdr.LedgerCloseMeta, headerFrom int) []xdr.TransactionPhase {
	var phases []xdr.TransactionPhase
	for p, first := range v1TxSet(ins[0]).Phases {
		switch first.V {
		case 0:
			type group struct {
				fee *xdr.Int64
				txs []xdr.TransactionEnvelope
			}
			var groups []*group
			byFee := map[string]*group{}
			for _, in := range ins {
				for _, c := range *v1TxSet(in).Phases[p].V0Components {
					k := feeKey(c.TxsMaybeDiscountedFee.BaseFee)
					if byFee[k] == nil {
						byFee[k] = &group{fee: c.TxsMaybeDiscountedFee.BaseFee}
						groups = append(groups, byFee[k])
					}
					byFee[k].txs = append(byFee[k].txs, c.TxsMaybeDiscountedFee.Txs...)
				}
			}
			comps := []xdr.TxSetComponent{}
			for _, g := range groups {
				comps = append(comps, xdr.TxSetComponent{
					Type:                  xdr.TxSetComponentTypeTxsetCompTxsMaybeDiscountedFee,
					TxsMaybeDiscountedFee: &xdr.TxSetComponentTxsMaybeDiscountedFee{BaseFee: g.fee, Txs: g.txs},
				})
			}
			phases = append(phases, xdr.TransactionPhase{V: 0, V0Components: &comps})
		case 1:
			par := xdr.ParallelTxsComponent{
				BaseFee:         v1TxSet(ins[headerFrom]).Phases[p].ParallelTxsComponent.BaseFee,
				ExecutionStages: []xdr.ParallelTxExecutionStage{},
			}
			for _, in := range ins {
				par.ExecutionStages = append(par.ExecutionStages, v1TxSet(in).Phases[p].ParallelTxsComponent.ExecutionStages...)
			}
			phases = append(phases, xdr.TransactionPhase{V: 1, ParallelTxsComponent: &par})
		}
	}
	return phases
}

func TestMergeLedgerBytesMatchesReference(t *testing.T) {
	gen := randxdr.NewGenerator()
	for i := range 24 {
		inputs := make([]xdr.LedgerCloseMeta, 1+i%4)
		raws := make([][]byte, len(inputs))
		for j := range inputs {
			inputs[j] = normalizedLedger(t, gen)
			raws[j] = marshal(t, inputs[j])
		}
		opts := MergeOptions{HeaderFrom: i % len(inputs)}
		if i%3 == 0 {
			opts.LedgerSeq = uint32(1000 + i)
			prev := xdr.Hash{byte(i)}
			opts.PreviousLedgerHash = &prev
		}
		if i%2 == 0 {
			opts.RemapLedgerSeq = func(input int, s uint32) uint32 { return scrambleSeq(s) + uint32(input) } //nolint:gosec // input < 4
		}
		want := referenceMerge(t, inputs, opts)
		got, hash, err := MergeLedgerBytes(raws, opts)
		require.NoError(t, err, "iteration %d", i)
		require.Equal(t, marshal(t, want), got, "iteration %d", i)
		require.Equal(t, want.LedgerHash(), hash, "iteration %d", i)
		for j := range inputs {
			require.Equal(t, marshal(t, inputs[j]), raws[j], "iteration %d: input %d was modified", i, j)
		}
	}
}

func TestMergeLedgerBytesRejects(t *testing.T) {
	gen := randxdr.NewGenerator()
	v2 := marshal(t, normalizedLedger(t, gen))

	_, _, err := MergeLedgerBytes(nil, MergeOptions{})
	require.ErrorContains(t, err, "no ledgers")

	_, _, err = MergeLedgerBytes([][]byte{v2}, MergeOptions{HeaderFrom: 1})
	require.ErrorContains(t, err, "out of range")

	txSet := xdr.GeneralizedTransactionSet{V: 1, V1TxSet: &xdr.TransactionSetV1{}}
	for _, old := range []xdr.LedgerCloseMeta{{V: 0, V0: &xdr.LedgerCloseMetaV0{}}, {V: 1, V1: &xdr.LedgerCloseMetaV1{TxSet: txSet}}} {
		_, _, err = MergeLedgerBytes([][]byte{v2, marshal(t, old)}, MergeOptions{})
		require.ErrorContains(t, err, "not supported")
	}

	threePhases := normalizedLedger(t, gen)
	ts := v1TxSet(threePhases)
	ts.Phases = append(ts.Phases, ts.Phases[0])
	_, _, err = MergeLedgerBytes([][]byte{v2, marshal(t, threePhases)}, MergeOptions{})
	require.ErrorContains(t, err, "tx set phases")

	swapped := normalizedLedger(t, gen)
	ts = v1TxSet(swapped)
	ts.Phases[0], ts.Phases[1] = ts.Phases[1], ts.Phases[0]
	_, _, err = MergeLedgerBytes([][]byte{v2, marshal(t, swapped)}, MergeOptions{})
	require.ErrorContains(t, err, "phase 0 has version 1")
}

// realLedger is testnet ledger 4,930,015 as stellar-core wrote it.
func realLedger(t *testing.T) []byte {
	raw, err := os.ReadFile("testdata/testnet_ledger_4930015.bin")
	require.NoError(t, err)
	return raw
}

// TestMergeLedgerBytesRealLedger checks the recomputed tx set, result set and
// header hashes against stellar-core's on a real ledger: merging it alone must
// return its exact bytes.
func TestMergeLedgerBytesRealLedger(t *testing.T) {
	raw := realLedger(t)
	var lcm xdr.LedgerCloseMeta
	require.NoError(t, lcm.UnmarshalBinary(raw))

	merged, hash, err := MergeLedgerBytes([][]byte{raw}, MergeOptions{})
	require.NoError(t, err)
	require.Equal(t, raw, merged)
	require.Equal(t, lcm.LedgerHash(), hash)
}

// TestMergeLedgerBytesTruncated checks that a ledger cut short anywhere from
// its txProcessing on is rejected with an error.
func TestMergeLedgerBytesTruncated(t *testing.T) {
	raw := realLedger(t)
	in, err := splitLedger(raw)
	require.NoError(t, err)
	for cut := len(raw) - len(in.txProcessing); cut < len(raw); cut++ {
		_, _, err := MergeLedgerBytes([][]byte{raw[:cut]}, MergeOptions{})
		require.Error(t, err, "cut at %d", cut)
	}
}
