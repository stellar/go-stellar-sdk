package ingest_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// allocLCMRaw builds a deterministic representative LCM V2 ledger: one V3
// soroban tx (2 contract events, 1 diagnostic) and one V4 tx (2 ops with 1+0
// events, 1 top-level event, 1 diagnostic), with fee processing populated —
// every arm the extract walk has to size past or drain.
func allocLCMRaw(t testing.TB) []byte {
	t.Helper()
	rv := xdr.ScVal{Type: xdr.ScValTypeScvVoid}
	ev := func(topic string) xdr.ContractEvent {
		sym := xdr.ScSymbol(topic)
		return xdr.ContractEvent{
			Type: xdr.ContractEventTypeContract,
			Body: xdr.ContractEventBody{V: 0, V0: &xdr.ContractEventV0{
				Topics: []xdr.ScVal{{Type: xdr.ScValTypeScvSymbol, Sym: &sym}},
				Data:   xdr.ScVal{Type: xdr.ScValTypeScvVoid},
			}},
		}
	}
	metaV3 := xdr.TransactionMeta{V: 3, V3: &xdr.TransactionMetaV3{
		SorobanMeta: &xdr.SorobanTransactionMeta{
			Events:           []xdr.ContractEvent{ev("v3a"), ev("v3b")},
			DiagnosticEvents: []xdr.DiagnosticEvent{{InSuccessfulContractCall: true, Event: ev("v3d")}},
			ReturnValue:      rv,
		},
	}}
	metaV4 := xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{
		Operations: []xdr.OperationMetaV2{
			{Events: []xdr.ContractEvent{ev("op0")}},
			{Events: []xdr.ContractEvent{}},
		},
		Events: []xdr.TransactionEvent{
			{Stage: xdr.TransactionEventStageTransactionEventStageAfterTx, Event: ev("top")},
		},
		DiagnosticEvents: []xdr.DiagnosticEvent{{InSuccessfulContractCall: true, Event: ev("v4d")}},
	}}
	result := func() xdr.TransactionResultPair {
		return xdr.TransactionResultPair{
			TransactionHash: xdr.Hash{1, 2, 3},
			Result: xdr.TransactionResult{
				Result: xdr.TransactionResultResult{Code: xdr.TransactionResultCodeTxInternalError},
			},
		}
	}
	fee := xdr.LedgerEntryChanges{{
		Type: xdr.LedgerEntryChangeTypeLedgerEntryRemoved,
		Removed: &xdr.LedgerKey{Type: xdr.LedgerEntryTypeAccount, Account: &xdr.LedgerKeyAccount{
			AccountId: xdr.MustAddress(keypair.MustRandom().Address()),
		}},
	}}
	lcm := xdr.LedgerCloseMeta{V: 2, V2: &xdr.LedgerCloseMetaV2{
		LedgerHeader: xdr.LedgerHeaderHistoryEntry{Header: xdr.LedgerHeader{
			LedgerSeq: 777, ScpValue: xdr.StellarValue{CloseTime: 1_700_000_000},
		}},
		TxSet: xdr.GeneralizedTransactionSet{V: 1, V1TxSet: &xdr.TransactionSetV1{}},
		TxProcessing: []xdr.TransactionResultMetaV1{
			{Result: result(), FeeProcessing: fee, TxApplyProcessing: metaV3, PostTxApplyFeeProcessing: fee},
			{Result: result(), FeeProcessing: fee, TxApplyProcessing: metaV4, PostTxApplyFeeProcessing: fee},
		},
	}}
	raw, err := lcm.MarshalBinary()
	require.NoError(t, err)
	return raw
}

// TestExtractLedgerEvents_Allocs pins the extract path's allocation budget on
// a representative ledger (spec §Escape/alloc discipline): the walk, the
// per-parse iterator/dispatch closures, one iterator closure per range loop
// reached, and the extractor's OUTPUT (the result slice and its per-tx event
// spines) — and nothing else. Per-element access, bundle resolution, and leaf
// reads must contribute zero.
func TestExtractLedgerEvents_Allocs(t *testing.T) {
	raw := allocLCMRaw(t)

	// Sanity: the fixture exercises both meta arms and yields events.
	events, err := ingest.ExtractLedgerEvents(xdr.ParseLedgerCloseMetaView(raw))
	require.NoError(t, err)
	require.Len(t, events, 2)
	require.Len(t, events[0].OperationEvents, 1)    // V3: single op group
	require.Len(t, events[0].OperationEvents[0], 2) // with 2 events
	require.Len(t, events[1].OperationEvents, 2)    // V4: per-op groups
	require.Len(t, events[1].TransactionEvents, 1)  // V4 top-level

	allocs := testing.AllocsPerRun(200, func() {
		out, err := ingest.ExtractLedgerEvents(xdr.ParseLedgerCloseMetaView(raw))
		if err != nil {
			panic(err)
		}
		if len(out) != 2 {
			panic("tx count")
		}
	})
	t.Logf("ExtractLedgerEvents allocs/run: %v", allocs)

	// Budget for this fixture, itemized:
	//   1  walk (ParseLedgerCloseMetaView)
	//   4  dispatch: TxProcessing All() closure, txPartsSeqV1 closure, and the
	//      generalizedEnvelopes closure pair (built even though extract never
	//      iterates envelopes)
	//   1  out slice (presized from txCount)
	//   V3 tx: events All() closure + raws spine + OperationEvents group append
	//   V4 tx: ops All() closure + 2 per-op events All() closures + 2 op raws
	//      spines + OperationEvents spine + top events All() closure + top
	//      events spine
	// plus the range-over-func yield closures the compiler materializes for
	// the nested loops. The exact split between "iterator closure" and "yield
	// closure" is a compiler detail; what this test pins is the TOTAL staying
	// at the closures+outputs level measured at port time — a per-element or
	// per-byte allocation would blow it up by orders of magnitude.
	require.LessOrEqual(t, allocs, 16.0,
		"extract path must allocate only the walk, per-loop closures, and outputs")
}
