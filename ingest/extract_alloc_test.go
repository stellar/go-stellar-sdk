package ingest_test

import (
	"os"
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

// TestViewExtract_Deterministic pins that repeated extraction over the same
// bytes is deterministic and byte-identical (no hidden state anywhere in the
// view or Walk machinery).
func TestViewExtract_Deterministic(t *testing.T) {
	raws := [][]byte{allocLCMRaw(t)}
	if real, err := os.ReadFile("../xdr/testdata/ledger_58752000.bin"); err == nil {
		raws = append(raws, real)
	}
	for i, raw := range raws {
		// Tier-1 has a single Parse constructor (no modes); repeated runs
		// must be deterministic and byte-identical.
		a, aerr := collectLedgerEvents(xdr.ParseLedgerCloseMetaView(raw))
		b, berr := collectLedgerEvents(xdr.ParseLedgerCloseMetaView(raw))
		require.Equal(t, aerr != nil, berr != nil, "fixture %d: error parity", i)
		require.Equal(t, a, b, "fixture %d: extraction must be deterministic", i)
	}
}

// TestStreamLedgerEvents_Allocs pins the streaming extraction budget on a
// representative ledger: with no slice materialization the only allocations
// are the per-transaction event spines —
//
//	V3 tx: OperationEvents growth (1) + one group make
//	V4 tx: OperationEvents presize + two group makes
//
// plus the walk-callback closures bound once per call. Streaming must
// allocate strictly less than the old slice-returning extractor did (6/run
// on this fixture).
func TestStreamLedgerEvents_Allocs(t *testing.T) {
	raw := allocLCMRaw(t)
	lcm := xdr.ParseLedgerCloseMetaView(raw)

	// Sanity: fixture exercises both meta arms.
	evs, err := collectLedgerEvents(lcm)
	require.NoError(t, err)
	require.Len(t, evs, 2)

	n := 0
	// The counting callback is hoisted so the measurement sees only the
	// extractor's own allocations, not the harness closure.
	count := func(_ int, ev ingest.LedgerTransactionEvents) error {
		n += len(ev.OperationEvents)
		return nil
	}
	stream := func() {
		if err := ingest.StreamLedgerEvents(lcm, nil, count); err != nil {
			panic(err)
		}
	}
	stream()
	require.Equal(t, 3, n)
	n = 0
	allocs := testing.AllocsPerRun(200, stream)
	t.Logf("StreamLedgerEvents allocs/run: %v", allocs)
	require.LessOrEqual(t, allocs, 5.0,
		"streaming must allocate strictly less than the slice extractor's 6/run")
}

// collectLedgerEvents is the three-line collect loop over the streaming API,
// shared by tests and benches (the slice-returning extractor is gone).
func collectLedgerEvents(lcm xdr.LedgerCloseMetaView) ([]ingest.LedgerTransactionEvents, error) {
	var out []ingest.LedgerTransactionEvents
	err := ingest.StreamLedgerEvents(lcm,
		func(n int) error {
			if n > 0 {
				out = make([]ingest.LedgerTransactionEvents, 0, n)
			}
			return nil
		},
		func(_ int, ev ingest.LedgerTransactionEvents) error {
			out = append(out, ev)
			return nil
		})
	if err != nil {
		return nil, err
	}
	return out, nil
}
