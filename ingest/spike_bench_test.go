package ingest_test

// Stage-1 spike benchmarks (two-tier visitor go/no-go), same fixture and
// methodology as every prior stage: real pubnet ledger, 100-200x, count >= 3,
// session medians side-by-side with the views-fused-locate baselines.

import (
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func BenchmarkSpikeExtractLedgerEvents(b *testing.B) {
	raw := loadRealLedger(b)
	b.Run("per_event", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			events, err := xdr.SpikeExtractLedgerEvents(raw, false)
			if err != nil {
				b.Fatal(err)
			}
			_ = events
		}
	})
	b.Run("per_array", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			events, err := xdr.SpikeExtractLedgerEvents(raw, true)
			if err != nil {
				b.Fatal(err)
			}
			_ = events
		}
	})
}

func BenchmarkSpikeExtractTxHashes(b *testing.B) {
	raw := loadRealLedger(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		hashes, err := xdr.SpikeExtractTxHashes(raw)
		if err != nil {
			b.Fatal(err)
		}
		_ = hashes
	}
}

// BenchmarkSpikeWalkPruned subscribes only the LAST meta position (V4
// top-level tx events — absent from this classic-heavy ledger), so the walk
// prunes everything via thin size skips: parity with a blind size proves the
// mask/prune concept costs nothing.
func BenchmarkSpikeWalkPruned(b *testing.B) {
	raw := loadRealLedger(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		n, err := xdr.SpikeWalkTxEventsOnly(raw)
		if err != nil {
			b.Fatal(err)
		}
		_ = n
	}
}
