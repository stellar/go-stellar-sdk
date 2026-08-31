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

// stressDensityLCM builds the stress ledger: txCount transactions cycling
// three flavors —
//
//	i%3 == 0: V3 soroban  (SorobanMeta: fee ext, 10 events, 2 diagnostics)
//	i%3 == 1: V4 soroban  (1 op: 2 changes + 10 events; 1 tx-level event,
//	          SorobanMeta with fee ext, 2 diagnostics)
//	i%3 == 2: V4 classic  (2 ops: 2 changes + 5 unified events each)
//
// with per-tx results carrying per-operation results (so the classic fee arm
// classifies) and distinct hashes.
func stressDensityLCM(b *testing.B, txCount int) []byte {
	b.Helper()

	ev := func(s string) xdr.ContractEvent {
		sym := xdr.ScSymbol(s)
		amount := xdr.Int64(1_000_000)
		return xdr.ContractEvent{
			Type: xdr.ContractEventTypeContract,
			Body: xdr.ContractEventBody{V: 0, V0: &xdr.ContractEventV0{
				Topics: []xdr.ScVal{{Type: xdr.ScValTypeScvSymbol, Sym: &sym}},
				Data:   xdr.ScVal{Type: xdr.ScValTypeScvI64, I64: &amount},
			}},
		}
	}
	events := func(n int) []xdr.ContractEvent {
		out := make([]xdr.ContractEvent, n)
		for i := range out {
			out[i] = ev("stress-topic-payload")
		}
		return out
	}
	diags := []xdr.DiagnosticEvent{
		{InSuccessfulContractCall: true, Event: ev("diag-a")},
		{InSuccessfulContractCall: true, Event: ev("diag-b")},
	}
	changes := func(seed byte) xdr.LedgerEntryChanges {
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
	feeExt := xdr.SorobanTransactionMetaExt{V: 1, V1: &xdr.SorobanTransactionMetaExtV1{
		TotalNonRefundableResourceFeeCharged: 40_000,
		TotalRefundableResourceFeeCharged:    10_000,
	}}
	rv := xdr.ScVal{Type: xdr.ScValTypeScvVoid}

	metaV3 := xdr.TransactionMeta{V: 3, V3: &xdr.TransactionMetaV3{
		TxChangesBefore: changes(3),
		SorobanMeta: &xdr.SorobanTransactionMeta{
			Ext: feeExt, Events: events(10), ReturnValue: rv, DiagnosticEvents: diags,
		},
	}}
	metaV4Soroban := xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{
		Operations:       []xdr.OperationMetaV2{{Changes: changes(4), Events: events(10)}},
		SorobanMeta:      &xdr.SorobanTransactionMetaV2{Ext: feeExt, ReturnValue: &rv},
		Events:           []xdr.TransactionEvent{{Stage: xdr.TransactionEventStageTransactionEventStageAfterTx, Event: ev("fee")}},
		DiagnosticEvents: diags,
	}}
	metaV4Classic := xdr.TransactionMeta{V: 4, V4: &xdr.TransactionMetaV4{
		Operations: []xdr.OperationMetaV2{
			{Changes: changes(5), Events: events(5)},
			{Changes: changes(6), Events: events(5)},
		},
	}}

	opResults := func(n int) *[]xdr.OperationResult {
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

	proc := make([]xdr.TransactionResultMetaV1, txCount)
	for i := range proc {
		var meta xdr.TransactionMeta
		nOps := 1
		switch i % 3 {
		case 0:
			meta = metaV3
		case 1:
			meta = metaV4Soroban
		case 2:
			meta = metaV4Classic
			nOps = 2
		}
		var hash xdr.Hash
		hash[0], hash[1], hash[2] = byte(i), byte(i>>8), byte(i>>16)
		proc[i] = xdr.TransactionResultMetaV1{
			Result: xdr.TransactionResultPair{
				TransactionHash: hash,
				Result: xdr.TransactionResult{
					FeeCharged: 150_000,
					Result: xdr.TransactionResultResult{
						Code:    xdr.TransactionResultCodeTxSuccess,
						Results: opResults(nOps),
					},
				},
			},
			TxApplyProcessing: meta,
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
