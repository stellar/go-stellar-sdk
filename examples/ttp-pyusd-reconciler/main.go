// PYUSD Reconciler — tracks mints, burns, and circulating supply for PYUSD on pubnet
// using the Token Transfer Processor (TTP) with BufferedStorageBackend.
//
// Usage:
//
//	go run main.go --start 55000000 --end 55100000 [--workers 20] [--bucket stellar-ledger-data-pubnet/ledgers/Jan-2025]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stellar/go-stellar-sdk/amount"
	assetProto "github.com/stellar/go-stellar-sdk/asset"
	"github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/processors/token_transfer"
	"github.com/stellar/go-stellar-sdk/support/datastore"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const (
	pyusdCode   = "PYUSD"
	pyusdIssuer = "GDQE7IXJ4HUHV6RQHIUPRJSEZE4DRS5WY577O2FY6YQ5LVWZ7JZTU2V5"
)

// workerResult holds per-worker mint/burn totals (in stroops).
type workerResult struct {
	minted uint64
	burned uint64
	ledgers uint32
	err    error
}

func main() {
	start := flag.Uint("start", 0, "Start ledger sequence (inclusive)")
	end := flag.Uint("end", 0, "End ledger sequence (inclusive)")
	workers := flag.Int("workers", 20, "Number of parallel workers")
	bucket := flag.String("bucket", "sdf-ledger-close-meta/v1/ledgers/pubnet", "GCS bucket path for pubnet ledger data")
	flag.Parse()

	if *start == 0 || *end == 0 || *start >= *end {
		fmt.Fprintf(os.Stderr, "PYUSD Reconciler — track mints, burns, and circulating supply on pubnet\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  go run main.go --start <ledger> --end <ledger> [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  go run main.go --start 55000000 --end 55100000 --workers 20\n")
		os.Exit(1)
	}

	startSeq := uint32(*start)
	endSeq := uint32(*end)
	numWorkers := *workers
	if numWorkers < 1 {
		numWorkers = 1
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Second Ctrl+C force-exits immediately.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		<-sigCh // first one is handled by NotifyContext above
		<-sigCh // second one = force exit
		fmt.Fprintf(os.Stderr, "\nForce exit.\n")
		os.Exit(1)
	}()

	// Build the target PYUSD proto asset for comparison.
	pyusdXdr := xdr.MustNewCreditAsset(pyusdCode, pyusdIssuer)
	pyusdAsset := assetProto.NewProtoAsset(pyusdXdr)

	// Split the ledger range into numWorkers chunks.
	totalLedgers := endSeq - startSeq + 1
	chunks := splitRange(startSeq, endSeq, numWorkers)

	fmt.Printf("PYUSD Reconciler\n")
	fmt.Printf("  Range       : %d — %d (%d ledgers)\n", startSeq, endSeq, totalLedgers)
	fmt.Printf("  Workers     : %d\n", len(chunks))
	fmt.Printf("  GCS bucket  : %s\n", *bucket)
	fmt.Println()

	// Progress tracking: total ledgers processed across all workers.
	var processedLedgers atomic.Uint64
	overallStart := time.Now()

	// Launch workers.
	results := make([]workerResult, len(chunks))
	var wg sync.WaitGroup

	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, from, to uint32) {
			defer wg.Done()
			results[idx] = processChunk(ctx, from, to, *bucket, pyusdAsset, &processedLedgers)
		}(i, chunk[0], chunk[1])
	}

	// Progress reporter goroutine.
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				printProgress(overallStart, processedLedgers.Load(), uint64(totalLedgers))
			case <-ctx.Done():
				return
			case <-done:
				return
			}
		}
	}()

	wg.Wait()
	close(done)

	// Aggregate results.
	var totalMinted, totalBurned uint64
	var totalProcessed uint32
	var errs []error

	for i, r := range results {
		if r.err != nil {
			errs = append(errs, fmt.Errorf("worker %d (ledgers %d–%d): %w", i, chunks[i][0], chunks[i][1], r.err))
			continue
		}
		totalMinted += r.minted
		totalBurned += r.burned
		totalProcessed += r.ledgers
	}

	elapsed := time.Since(overallStart)

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════")
	fmt.Println("  PYUSD Reconciliation Results")
	fmt.Println("═══════════════════════════════════════════════════")
	fmt.Printf("  Ledger range    : %d — %d\n", startSeq, endSeq)
	fmt.Printf("  Ledgers scanned : %d\n", totalProcessed)
	fmt.Printf("  Total minted    : %s PYUSD\n", amount.String(xdr.Int64(totalMinted)))
	fmt.Printf("  Total burned    : %s PYUSD\n", amount.String(xdr.Int64(totalBurned)))
	fmt.Printf("  Circulating     : %s PYUSD  (minted - burned)\n", amount.String(xdr.Int64(totalMinted-totalBurned)))
	fmt.Printf("  Elapsed         : %s\n", elapsed.Round(time.Second))
	fmt.Printf("  Throughput      : %.0f ledgers/sec\n", float64(totalProcessed)/elapsed.Seconds())
	fmt.Println("═══════════════════════════════════════════════════")

	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "\n%d worker(s) failed:\n", len(errs))
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  • %v\n", e)
		}
		os.Exit(1)
	}
}

// processChunk creates its own BSB instance and iterates [from, to] directly.
func processChunk(
	ctx context.Context,
	from, to uint32,
	bucket string,
	targetAsset *assetProto.Asset,
	processed *atomic.Uint64,
) workerResult {
	var res workerResult

	dsConfig := datastore.DataStoreConfig{
		Type: "GCS",
		Params: map[string]string{
			"destination_bucket_path": bucket,
		},
	}

	ds, err := datastore.NewDataStore(ctx, dsConfig)
	if err != nil {
		res.err = fmt.Errorf("create datastore: %w", err)
		return res
	}
	defer ds.Close()

	schema, err := datastore.LoadSchema(ctx, ds, dsConfig)
	if err != nil {
		res.err = fmt.Errorf("load schema: %w", err)
		return res
	}

	bsbConfig := ledgerbackend.BufferedStorageBackendConfig{
		BufferSize: 1000,
		NumWorkers: 20,
		RetryLimit: 3,
		RetryWait:  30 * time.Second,
	}

	backend, err := ledgerbackend.NewBufferedStorageBackend(bsbConfig, ds, schema)
	if err != nil {
		res.err = fmt.Errorf("create BSB: %w", err)
		return res
	}
	defer backend.Close()

	ledgerRange := ledgerbackend.BoundedRange(from, to)
	if err := backend.PrepareRange(ctx, ledgerRange); err != nil {
		res.err = fmt.Errorf("prepare range: %w", err)
		return res
	}

	ttp := token_transfer.NewEventsProcessor(
		network.PublicNetworkPassphrase,
		token_transfer.WithUnifiedEventsStreamEnabled(),
	)

	for seq := from; seq <= to; seq++ {
		lcm, err := backend.GetLedger(ctx, seq)
		if err != nil {
			res.err = fmt.Errorf("get ledger %d: %w", seq, err)
			return res
		}

		events, err := ttp.EventsFromLedger(lcm)
		if err != nil {
			res.err = fmt.Errorf("TTP error at ledger %d: %w", seq, err)
			return res
		}

		for _, event := range events {
			eventAsset := event.GetAsset()
			if eventAsset == nil || !eventAsset.Equals(targetAsset) {
				continue
			}

			stroops := uint64(amount.MustParse(event.GetAmount()))

			switch event.GetEventType() {
			case token_transfer.MintEvent:
				res.minted += stroops
			case token_transfer.BurnEvent:
				res.burned += stroops
			}
		}

		res.ledgers++
		processed.Add(1)
	}

	return res
}

// splitRange divides [start, end] into n roughly equal sub-ranges.
func splitRange(start, end uint32, n int) [][2]uint32 {
	total := end - start + 1
	if uint32(n) > total {
		n = int(total)
	}

	chunkSize := total / uint32(n)
	remainder := total % uint32(n)

	chunks := make([][2]uint32, 0, n)
	cursor := start
	for i := 0; i < n; i++ {
		size := chunkSize
		if uint32(i) < remainder {
			size++
		}
		chunkEnd := cursor + size - 1
		chunks = append(chunks, [2]uint32{cursor, chunkEnd})
		cursor = chunkEnd + 1
	}
	return chunks
}

// printProgress shows elapsed time, percent done, and ETA.
func printProgress(start time.Time, done, total uint64) {
	if done == 0 {
		fmt.Printf("  [progress] waiting for first ledgers...\n")
		return
	}
	elapsed := time.Since(start)
	pct := float64(done) / float64(total) * 100
	rate := float64(done) / elapsed.Seconds()
	remaining := time.Duration(float64(total-done)/rate) * time.Second
	fmt.Printf("  [progress] %d/%d ledgers (%.1f%%) | %.0f ledgers/sec | elapsed %s | ETA %s\n",
		done, total, pct, rate, elapsed.Round(time.Second), remaining.Round(time.Second))
}
