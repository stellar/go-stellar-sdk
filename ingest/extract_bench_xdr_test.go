package ingest_test

// Walk-backed extractor benchmarks (post-spike-retirement rows).

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func BenchmarkXdrExtractLedgerEvents(b *testing.B) {
	raw := loadRealLedger(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		events, err := xdr.ExtractLedgerEvents(xdr.ParseLedgerCloseMetaView(raw))
		if err != nil {
			b.Fatal(err)
		}
		_ = events
	}
}

func BenchmarkXdrExtractTxHashes(b *testing.B) {
	raw := loadRealLedger(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		hashes, err := xdr.ExtractTxHashes(xdr.ParseLedgerCloseMetaView(raw))
		if err != nil {
			b.Fatal(err)
		}
		_ = hashes
	}
}
