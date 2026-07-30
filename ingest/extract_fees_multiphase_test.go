package ingest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// buildMultiPhaseLCM builds an LCM (V1 or V2 — both carry a
// GeneralizedTransactionSet) whose TxSet holds BOTH phase shapes in one
// ledger: txs[:split] sit in a V=0 components phase and txs[split:] in a V=1
// parallel-txs phase (one stage, one cluster per tx). Envelopes are listed in
// reverse within each phase, so nothing about the TxSet ordering matches the
// TxProcessing apply order — pairing is by hash.
func buildMultiPhaseLCM(t testing.TB, version int32, ledgerSeq uint32, closeTime int64, txs []txWithHash, split int) xdr.LedgerCloseMeta {
	t.Helper()
	require.Less(t, split, len(txs))

	v0Envs := make([]xdr.TransactionEnvelope, split)
	for i, tx := range txs[:split] {
		v0Envs[split-1-i] = tx.env
	}
	comp := []xdr.TxSetComponent{{
		Type:                  xdr.TxSetComponentTypeTxsetCompTxsMaybeDiscountedFee,
		TxsMaybeDiscountedFee: &xdr.TxSetComponentTxsMaybeDiscountedFee{Txs: v0Envs},
	}}

	parallelTxs := txs[split:]
	clusters := make(xdr.ParallelTxExecutionStage, 0, len(parallelTxs))
	for i := range parallelTxs {
		clusters = append(clusters, xdr.DependentTxCluster{parallelTxs[len(parallelTxs)-1-i].env})
	}

	phases := []xdr.TransactionPhase{
		{V: 0, V0Components: &comp},
		{V: 1, ParallelTxsComponent: &xdr.ParallelTxsComponent{
			ExecutionStages: []xdr.ParallelTxExecutionStage{clusters},
		}},
	}

	header := fixtureHeader(ledgerSeq, closeTime)
	txSet := xdr.GeneralizedTransactionSet{V: 1, V1TxSet: &xdr.TransactionSetV1{Phases: phases}}

	switch version {
	case 1:
		return xdr.LedgerCloseMeta{V: 1, V1: &xdr.LedgerCloseMetaV1{
			LedgerHeader: header, TxSet: txSet, TxProcessing: resultMetas(txs),
		}}
	case 2:
		return xdr.LedgerCloseMeta{V: 2, V2: &xdr.LedgerCloseMetaV2{
			LedgerHeader: header, TxSet: txSet, TxProcessing: resultMetaV1s(txs),
		}}
	default:
		t.Fatalf("unsupported version %d", version)
		return xdr.LedgerCloseMeta{}
	}
}

func TestExtractFees_MultiPhaseTxSet(t *testing.T) {
	invoke := feeInvokeHostFunctionOp()
	bump := feeBumpSequenceOp()
	txs := []txWithHash{
		// V0-components phase:
		feeTxV1(t, []xdr.Operation{bump, bump}, false, feeMetaV1(), 101),      // classic 50
		feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext1(40, 10), 100), // soroban 50
		feeTxV1(t, []xdr.Operation{invoke}, true, feeMetaV3Ext0(), 100),       // skipped
		// V1 parallel phase:
		feeBumpFeeTx(t, []xdr.Operation{invoke}, true, feeMetaV4Ext1(30, 20), 300), // soroban 250
		feeTxV1(t, []xdr.Operation{bump}, false, feeMetaV2(), 100),                 // classic 100
		feeTxV0(t, []xdr.Operation{bump, bump, bump}, feeMetaV0(), 100),            // classic 33
	}
	for _, version := range []int32{1, 2} {
		t.Run(fmt.Sprintf("lcmV%d", version), func(t *testing.T) {
			lcm := buildMultiPhaseLCM(t, version, 8952, 1_700_086_000, txs, 3)
			got := extractFeesFromLCM(t, lcm)
			assert.Equal(t, []uint64{50, 100, 33}, got.ClassicFeesPerOp)
			assert.Equal(t, []uint64{50, 250}, got.SorobanInclusionFees)
		})
	}
}
