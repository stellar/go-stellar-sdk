package ingest_test

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// BenchmarkLedgerTxPartsStressDensity measures the parts/products composition
// on a crafted ledger far denser than the pubnet fixture: 6,000 transactions
// (stellar-rpc's hot-ingest stress shape), ~10 contract events each, cycling
// V3-soroban / V4-soroban / V4-classic metas so every extractor arm runs, in
// an LCM V2 (TransactionResultMetaV1 TxProcessing — the modern element shape).
// Each operation also carries a couple of ledger-entry changes and each tx a
// couple of diagnostic events, so the walk pays a realistic spine, not just
// event bytes. This is the density where per-element size passes dominate; a
// TRUE production-shape measurement (real stress ledgers) belongs to the RPC
// repo and its fixtures.
func BenchmarkLedgerTxPartsStressDensity(b *testing.B) {
	const txCount = 6000
	raw := stressDensityLCM(b, txCount)
	view := xdr.LedgerCloseMetaView(raw)

	b.Run("parts", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			txParts, err := ingest.ExtractLedgerTxParts(view)
			if err != nil {
				b.Fatal(err)
			}
			if len(txParts) != txCount {
				b.Fatal("bad fixture")
			}
		}
	})
	b.Run("parts_events", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			txParts, err := ingest.ExtractLedgerTxParts(view)
			if err != nil {
				b.Fatal(err)
			}
			txEvents, err := ingest.EventsFromTxParts(txParts)
			if err != nil {
				b.Fatal(err)
			}
			if len(txEvents) != txCount {
				b.Fatal("bad fixture")
			}
		}
	})
	b.Run("all_products", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			txParts, err := ingest.ExtractLedgerTxParts(view)
			if err != nil {
				b.Fatal(err)
			}
			txEvents, err := ingest.EventsFromTxParts(txParts)
			if err != nil {
				b.Fatal(err)
			}
			fees, err := ingest.FeesFromTxParts(txParts)
			if err != nil {
				b.Fatal(err)
			}
			if len(txEvents) != txCount || len(fees.ClassicFeesPerOp) == 0 || len(fees.SorobanInclusionFees) == 0 {
				b.Fatal("bad fixture")
			}
		}
	})
}

// The stress fixture's building blocks. Kept as package-level helpers rather
// than closures so the ledger builder below stays readable at a glance.

// stressEvent is one contract event with a symbol topic and an i64 payload —
// small enough to be realistic, big enough that sizing it is real work.
func stressEvent(topic string) xdr.ContractEvent {
	sym := xdr.ScSymbol(topic)
	amount := xdr.Int64(1_000_000)
	return xdr.ContractEvent{
		Type: xdr.ContractEventTypeContract,
		Body: xdr.ContractEventBody{V: 0, V0: &xdr.ContractEventV0{
			Topics: []xdr.ScVal{{Type: xdr.ScValTypeScvSymbol, Sym: &sym}},
			Data:   xdr.ScVal{Type: xdr.ScValTypeScvI64, I64: &amount},
		}},
	}
}

func stressEvents(n int) []xdr.ContractEvent {
	out := make([]xdr.ContractEvent, n)
	for i := range out {
		out[i] = stressEvent("stress-topic-payload")
	}
	return out
}

// stressChanges is the ledger-entry spine each operation carries, so the walk
// pays for an operation interior rather than for event bytes alone.
func stressChanges(seed byte) xdr.LedgerEntryChanges {
	key := func(k byte) xdr.LedgerKey {
		var ed xdr.Uint256
		ed[0], ed[31] = seed, k
		return xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeAccount,
			Account: &xdr.LedgerKeyAccount{AccountId: xdr.AccountId{
				Type: xdr.PublicKeyTypePublicKeyTypeEd25519, Ed25519: &ed,
			}},
		}
	}
	removed0, removed1 := key(0), key(1)
	return xdr.LedgerEntryChanges{
		{Type: xdr.LedgerEntryChangeTypeLedgerEntryRemoved, Removed: &removed0},
		{Type: xdr.LedgerEntryChangeTypeLedgerEntryRemoved, Removed: &removed1},
	}
}

// stressFlavor is one meta flavor together with the number of per-operation
// results a transaction carrying it must report, so the fee product sees a
// result shape that matches the meta.
type stressFlavor struct {
	meta xdr.TransactionMeta
	nOps int
}

// stressFlavors returns the three flavors the ledger cycles through, in the
// order stressDensityLCM assigns them:
//
//	V3 soroban  (SorobanMeta: fee ext, 10 events, 2 diagnostics)
//	V4 soroban  (1 op: 2 changes + 10 events; 1 tx-level event, SorobanMeta
//	             with fee ext, 2 diagnostics)
//	V4 classic  (2 ops: 2 changes + 5 unified events each)
func stressFlavors() []stressFlavor {
	diags := []xdr.DiagnosticEvent{
		{InSuccessfulContractCall: true, Event: stressEvent("diag-a")},
		{InSuccessfulContractCall: true, Event: stressEvent("diag-b")},
	}
	feeExt := xdr.SorobanTransactionMetaExt{V: 1, V1: &xdr.SorobanTransactionMetaExtV1{
		TotalNonRefundableResourceFeeCharged: 40_000,
		TotalRefundableResourceFeeCharged:    10_000,
	}}
	rv := xdr.ScVal{Type: xdr.ScValTypeScvVoid}

	return []stressFlavor{
		{nOps: 1, meta: xdr.TransactionMeta{V: 3, V3: &xdr.TransactionMetaV3{
			TxChangesBefore: stressChanges(3),
			SorobanMeta: &xdr.SorobanTransactionMeta{
				Ext: feeExt, Events: stressEvents(10), ReturnValue: rv, DiagnosticEvents: diags,
			},
		}}},
		{nOps: 1, meta: xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{
			Operations:  []xdr.OperationMetaV2{{Changes: stressChanges(4), Events: stressEvents(10)}},
			SorobanMeta: &xdr.SorobanTransactionMetaV2{Ext: feeExt, ReturnValue: &rv},
			Events: []xdr.TransactionEvent{
				{Stage: xdr.TransactionEventStageTransactionEventStageAfterTx, Event: stressEvent("fee")},
			},
			DiagnosticEvents: diags,
		}}},
		{nOps: 2, meta: xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{
			Operations: []xdr.OperationMetaV2{
				{Changes: stressChanges(5), Events: stressEvents(5)},
				{Changes: stressChanges(6), Events: stressEvents(5)},
			},
		}}},
	}
}

// stressOpResults is a transaction result's per-operation result list — what
// the classic fee arm divides the fee by, so the fee product classifies.
func stressOpResults(n int) *[]xdr.OperationResult {
	out := make([]xdr.OperationResult, n)
	for i := range out {
		out[i] = xdr.OperationResult{
			Code: xdr.OperationResultCodeOpInner,
			Tr: &xdr.OperationResultTr{
				Type:          xdr.OperationTypePayment,
				PaymentResult: &xdr.PaymentResult{Code: xdr.PaymentResultCodePaymentSuccess},
			},
		}
	}
	return &out
}

// stressDensityLCM builds the stress ledger: txCount transactions cycling the
// three stressFlavors, each with per-operation results (so the classic fee arm
// classifies) and a distinct hash, in an LCM V2.
func stressDensityLCM(b *testing.B, txCount int) []byte {
	b.Helper()

	flavors := stressFlavors()
	proc := make([]xdr.TransactionResultMetaV1, txCount)
	for i := range proc {
		flavor := flavors[i%len(flavors)]
		var hash xdr.Hash
		hash[0], hash[1], hash[2] = byte(i), byte(i>>8), byte(i>>16)
		proc[i] = xdr.TransactionResultMetaV1{
			Result: xdr.TransactionResultPair{
				TransactionHash: hash,
				Result: xdr.TransactionResult{
					FeeCharged: 150_000,
					Result: xdr.TransactionResultResult{
						Code:    xdr.TransactionResultCodeTxSuccess,
						Results: stressOpResults(flavor.nOps),
					},
				},
			},
			TxApplyProcessing: flavor.meta,
		}
	}

	lcm := xdr.LedgerCloseMeta{V: 2, V2: &xdr.LedgerCloseMetaV2{
		TxSet:        xdr.GeneralizedTransactionSet{V: 1, V1TxSet: &xdr.TransactionSetV1{}},
		TxProcessing: proc,
	}}
	raw, err := lcm.MarshalBinary()
	if err != nil {
		b.Fatal(err)
	}
	return raw
}
