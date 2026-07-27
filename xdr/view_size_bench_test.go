package xdr

// BenchmarkBlindSizeRealLedger pins the raw throughput of the blind sizing
// engine (size(), no per-node state) over a full production ledger — the
// canary for regressions that would put fat calls back into the thin
// engine.

import (
	"os"
	"testing"
)

func BenchmarkBlindSizeRealLedger(b *testing.B) {
	raw, err := os.ReadFile("testdata/ledger_58752000.bin")
	if err != nil {
		b.Skip("no testdata")
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sz, err := LedgerCloseMetaView{view{d: raw}}.size(0)
		if err != nil || sz != len(raw) {
			b.Fatal(sz, err)
		}
	}
}
