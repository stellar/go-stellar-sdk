package ingest_test

import (
	"io"
	"testing"

	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Benchmarks for the four public view extractors on a real pubnet ledger,
// each with the eager-decode reference path as the A/B baseline. These
// supersede the hand-rolled prototypes that lived in xdr
// (extract_tx_bench_test.go, event_extraction_bench_test.go,
// selective_decode_bench_test.go) — same fixture, but measuring the
// production code paths instead of inlined loops.

func benchRefTransactions(b *testing.B, raw []byte) []ingest.LedgerTransaction {
	b.Helper()
	var lcm xdr.LedgerCloseMeta
	if err := xdr.SafeUnmarshal(raw, &lcm); err != nil {
		b.Fatal(err)
	}
	reader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(network.PublicNetworkPassphrase, lcm)
	if err != nil {
		b.Fatal(err)
	}
	defer reader.Close()
	var out []ingest.LedgerTransaction
	for {
		tx, rerr := reader.Read()
		if rerr == io.EOF {
			return out
		}
		if rerr != nil {
			b.Fatal(rerr)
		}
		out = append(out, tx)
	}
}

func BenchmarkExtractTxHashes(b *testing.B) {
	raw := loadRealLedger(b)

	b.Run("full_decode", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			txs := benchRefTransactions(b, raw)
			hashes := make([]xdr.Hash, 0, len(txs))
			for _, tx := range txs {
				hashes = append(hashes, tx.Hash)
			}
			_ = hashes
		}
	})
	// The hashes-only extractor is gone (no RPC consumer); the streaming
	// extractor's Hash field covers the need. This bench row measures the
	// full-decode reference only.
}

func BenchmarkExtractLedgerEvents(b *testing.B) {
	raw := loadRealLedger(b)

	b.Run("full_decode", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			txs := benchRefTransactions(b, raw)
			for j := range txs {
				events, err := txs[j].GetTransactionEvents()
				if err != nil {
					b.Fatal(err)
				}
				_ = events
			}
		}
	})
	b.Run("view", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			events, err := collectLedgerEvents(xdr.ParseLedgerCloseMetaView(raw))
			if err != nil {
				b.Fatal(err)
			}
			_ = events
		}
	})
}

func BenchmarkLedgerTransactionViewRange(b *testing.B) {
	raw := loadRealLedger(b)

	b.Run("full_ledger/full_decode", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = benchRefTransactions(b, raw)
		}
	})
	b.Run("full_ledger/view", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			txs, err := ingest.LedgerTransactionViewRange(
				xdr.ParseLedgerCloseMetaView(raw), 0, 0, network.PublicNetworkPassphrase)
			if err != nil {
				b.Fatal(err)
			}
			_ = txs
		}
	})
	// The small page is where the early stop in envelope pairing shows up:
	// enumeration of the TxSet halts once the page's 10 hashes resolve.
	b.Run("page_10/view", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			txs, err := ingest.LedgerTransactionViewRange(
				xdr.ParseLedgerCloseMetaView(raw), 0, 10, network.PublicNetworkPassphrase)
			if err != nil {
				b.Fatal(err)
			}
			_ = txs
		}
	})
}

func BenchmarkLedgerTransactionViewByHash(b *testing.B) {
	raw := loadRealLedger(b)
	refTxs := benchRefTransactions(b, raw)

	// Early/mid/late apply positions (carried over from the deleted xdr
	// prototype): the TxProcessing scan cost scales with apply position,
	// while the TxSet envelope-pairing early stop depends on the hash's
	// agreed-set position — together the spread brackets the range.
	positions := []struct {
		name string
		idx  int
	}{
		{"early", min(len(refTxs)-1, 10)},
		{"mid", len(refTxs) / 2},
		{"late", len(refTxs) - 1},
	}
	for _, pos := range positions {
		target := refTxs[pos.idx].Hash
		b.Run("full_decode/"+pos.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				txs := benchRefTransactions(b, raw)
				for j := range txs {
					if txs[j].Hash == target {
						break
					}
				}
			}
		})
		b.Run("view/"+pos.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				tx, found, err := ingest.LedgerTransactionViewByHash(
					xdr.ParseLedgerCloseMetaView(raw), target, network.PublicNetworkPassphrase)
				if err != nil || !found {
					b.Fatalf("found=%v err=%v", found, err)
				}
				_ = tx
			}
		})
	}
}
