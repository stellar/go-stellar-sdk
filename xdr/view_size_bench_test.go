package xdr

// BenchmarkBlindSizeRealLedger pins the raw throughput of the BLIND sizing
// engine (size(), detached, no walk) over a full production ledger — the
// A/B instrument for the struct-view re-skin cost: on identical bytes the
// named-[]byte engine (views-fused-locate) ran ~420µs; the struct-view
// engine measured ~660µs at the stage-2.5 port (+57%), uniform per-node
// overhead concentrated in tiny leaves (VarOpaque/ScSymbol/Uint32 reads).
// This is the number PLAN B (spec §Escape/alloc discipline) exists to
// recover; the frontier stack cannot help here (nothing is consumed).

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
