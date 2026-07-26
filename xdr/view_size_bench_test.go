package xdr

// BenchmarkBlindSizeRealLedger pins the raw throughput of the BLIND sizing
// engine (size(), detached, no walk) over a full production ledger — the
// A/B instrument for the engine's call shape: on identical bytes the
// named-[]byte engine (views-fused-locate) ran ~420µs; the struct-view
// methods measured ~660µs at the stage-2.5 port (+57%, uniform per-node
// receiver/construction overhead concentrated in tiny leaves), and the
// PLAN A.5 thin engine (package-level size functions over bare []byte)
// recovered parity (~410µs). Regressions here mean the thin engine grew
// fat calls; the frontier stack cannot help on this path (nothing is
// consumed).

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
