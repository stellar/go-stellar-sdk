package ingest_test

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// BenchmarkXdrExtractLedgerEvents_StressDensity is the SDK-level stress-shape
// PROXY: a crafted ledger far denser in events than pubnet (200 soroban txs,
// 10 contract events each, V3/V4 alternating with diagnostics), extracting
// through the same public path. A TRUE stress-shape measurement (production
// stress ledgers) requires the stellar-rpc side and its fixtures — that A/B
// belongs to the RPC repo, not the SDK.
func BenchmarkXdrExtractLedgerEvents_StressDensity(b *testing.B) {
	raw := stressDensityLCM(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		events, err := collectLedgerEvents(xdr.ParseLedgerCloseMetaView(raw))
		if err != nil {
			b.Fatal(err)
		}
		if len(events) != 200 {
			b.Fatal("bad fixture")
		}
	}
}

func stressDensityLCM(b *testing.B) []byte {
	b.Helper()
	ev := func(s string) xdr.ContractEvent {
		sym := xdr.ScSymbol(s)
		return xdr.ContractEvent{Type: xdr.ContractEventTypeContract,
			Body: xdr.ContractEventBody{V: 0, V0: &xdr.ContractEventV0{
				Topics: []xdr.ScVal{{Type: xdr.ScValTypeScvSymbol, Sym: &sym}},
				Data:   xdr.ScVal{Type: xdr.ScValTypeScvVoid}}}}
	}
	evs := make([]xdr.ContractEvent, 10)
	for i := range evs {
		evs[i] = ev("stress-topic-payload")
	}
	rv := xdr.ScVal{Type: xdr.ScValTypeScvVoid}
	metaV3 := xdr.TransactionMeta{V: 3, V3: &xdr.TransactionMetaV3{
		SorobanMeta: &xdr.SorobanTransactionMeta{Events: evs, ReturnValue: rv,
			DiagnosticEvents: []xdr.DiagnosticEvent{{InSuccessfulContractCall: true, Event: ev("diag")}}}}}
	metaV4 := xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{
		Operations: []xdr.OperationMetaV2{{Events: evs[:5]}, {Events: evs[5:]}},
		Events:     []xdr.TransactionEvent{{Stage: xdr.TransactionEventStageTransactionEventStageAfterTx, Event: ev("top")}}}}
	tr := xdr.TransactionResult{Result: xdr.TransactionResultResult{Code: xdr.TransactionResultCodeTxInternalError}}
	proc := make([]xdr.TransactionResultMeta, 200)
	for i := range proc {
		m := metaV3
		if i%2 == 1 {
			m = metaV4
		}
		proc[i] = xdr.TransactionResultMeta{
			Result:            xdr.TransactionResultPair{TransactionHash: xdr.Hash{byte(i)}, Result: tr},
			TxApplyProcessing: m,
		}
	}
	lcm := xdr.LedgerCloseMeta{V: 1, V1: &xdr.LedgerCloseMetaV1{
		TxSet:        xdr.GeneralizedTransactionSet{V: 1, V1TxSet: &xdr.TransactionSetV1{}},
		TxProcessing: proc,
	}}
	raw, err := lcm.MarshalBinary()
	if err != nil {
		b.Fatal(err)
	}
	return raw
}
