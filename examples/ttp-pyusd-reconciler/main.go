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
	"math/big"
	"os"
	"os/signal"
	"sort"
	"strings"
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

type outlierEvent struct {
	eventType string
	amount    string // display string
	ledger    uint32
	txHash    string
}

// workerResult holds per-worker mint/burn totals (in stroops).
// Uses big.Int because Soroban contract events can have Int128 amounts
// that exceed int64 when converted to stroops.
type workerResult struct {
	minted   *big.Int
	burned   *big.Int
	ledgers  uint32
	outliers []outlierEvent
	err      error
}

func main() {
	start := flag.Uint("start", 0, "Start ledger sequence (inclusive)")
	end := flag.Uint("end", 0, "End ledger sequence (inclusive)")
	workers := flag.Int("workers", 20, "Number of parallel workers")
	bufferSize := flag.Uint("buffer-size", 1000, "BSB prefetch buffer size per worker")
	bsbWorkers := flag.Uint("bsb-workers", 20, "BSB internal fetch workers per worker")
	retryLimit := flag.Uint("retry-limit", 3, "BSB retry limit on fetch failures")
	outlierThreshold := flag.Float64("outlier", 1_000_000, "Flag mints/burns above this amount (in PYUSD)")
	bucket := flag.String("bucket", "sdf-ledger-close-meta/v1/ledgers/pubnet", "GCS bucket path for pubnet ledger data")
	flag.Parse()

	if *start == 0 || *end == 0 || *start >= *end {
		fmt.Fprintf(os.Stderr, "PYUSD Reconciler — track mints, burns, and circulating supply on pubnet\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  go run main.go --start <ledger> --end <ledger> [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  go run main.go --start 55000000 --end 55100000 --workers 10\n")
		os.Exit(1)
	}

	startSeq := uint32(*start)
	endSeq := uint32(*end)
	numWorkers := *workers
	if numWorkers < 1 {
		numWorkers = 1
	}

	// Convert outlier threshold from PYUSD to stroops (big.Int).
	thresholdStroops := new(big.Int).SetInt64(int64(*outlierThreshold * 10_000_000))

	bsbCfg := ledgerbackend.BufferedStorageBackendConfig{
		BufferSize: uint32(*bufferSize),
		NumWorkers: uint32(*bsbWorkers),
		RetryLimit: uint32(*retryLimit),
		RetryWait:  30 * time.Second,
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
	fmt.Printf("  BSB/worker  : buffer=%d, fetchers=%d, retries=%d\n", bsbCfg.BufferSize, bsbCfg.NumWorkers, bsbCfg.RetryLimit)
	fmt.Printf("  Outlier     : >= %s PYUSD\n", stroopsToDisplay(thresholdStroops))
	fmt.Printf("  GCS bucket  : %s\n", *bucket)
	fmt.Println()

	// Progress tracking: total ledgers processed across all workers.
	var processedLedgers atomic.Uint64
	overallStart := time.Now()

	// Launch workers.
	results := make([]workerResult, len(chunks))
	var wg sync.WaitGroup
	var printMu sync.Mutex

	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, from, to uint32) {
			defer wg.Done()
			results[idx] = processChunk(ctx, from, to, *bucket, bsbCfg, pyusdAsset, thresholdStroops, &printMu, &processedLedgers)
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
	totalMinted := new(big.Int)
	totalBurned := new(big.Int)
	var totalProcessed uint32
	var errs []error

	var allOutliers []outlierEvent
	for i, r := range results {
		if r.err != nil {
			errs = append(errs, fmt.Errorf("worker %d (ledgers %d–%d): %w", i, chunks[i][0], chunks[i][1], r.err))
			continue
		}
		totalMinted.Add(totalMinted, r.minted)
		totalBurned.Add(totalBurned, r.burned)
		totalProcessed += r.ledgers
		allOutliers = append(allOutliers, r.outliers...)
	}

	circulating := new(big.Int).Sub(totalMinted, totalBurned)
	elapsed := time.Since(overallStart)

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════")
	fmt.Println("  PYUSD Reconciliation Results")
	fmt.Println("═══════════════════════════════════════════════════")
	fmt.Printf("  Ledger range    : %d — %d\n", startSeq, endSeq)
	fmt.Printf("  Ledgers scanned : %d\n", totalProcessed)
	fmt.Printf("  Total minted    : %s PYUSD\n", stroopsToDisplay(totalMinted))
	fmt.Printf("  Total burned    : %s PYUSD\n", stroopsToDisplay(totalBurned))
	fmt.Printf("  Circulating     : %s PYUSD  (minted - burned)\n", stroopsToDisplay(circulating))
	fmt.Printf("  Elapsed         : %s\n", elapsed.Round(time.Second))
	fmt.Printf("  Throughput      : %.0f ledgers/sec\n", float64(totalProcessed)/elapsed.Seconds())
	fmt.Println("═══════════════════════════════════════════════════")

	if len(allOutliers) > 0 {
		// Sort by ledger sequence for chronological output.
		sort.Slice(allOutliers, func(i, j int) bool {
			return allOutliers[i].ledger < allOutliers[j].ledger
		})
		fmt.Printf("\n  Outlier events (>= %s PYUSD): %d\n", stroopsToDisplay(thresholdStroops), len(allOutliers))
		fmt.Println("  ─────────────────────────────────────────────────")
		for _, o := range allOutliers {
			fmt.Printf("    %-5s %s PYUSD | ledger %d | tx %s\n", o.eventType, o.amount, o.ledger, o.txHash)
		}
	}

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
	bsbConfig ledgerbackend.BufferedStorageBackendConfig,
	targetAsset *assetProto.Asset,
	threshold *big.Int,
	printMu *sync.Mutex,
	processed *atomic.Uint64,
) workerResult {
	res := workerResult{
		minted: new(big.Int),
		burned: new(big.Int),
	}

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

			stroops, ok := parseStroops(event.GetAmount())
			if !ok {
				res.err = fmt.Errorf("bad amount %q at ledger %d", event.GetAmount(), seq)
				return res
			}

			var evType string
			switch event.GetEventType() {
			case token_transfer.MintEvent:
				res.minted.Add(res.minted, stroops)
				evType = "MINT"
			case token_transfer.BurnEvent:
				res.burned.Add(res.burned, stroops)
				evType = "BURN"
			default:
				continue
			}

			if stroops.Cmp(threshold) >= 0 {
				displayAmt := stroopsToDisplay(stroops)
				txHash := event.GetMeta().GetTxHash()
				o := outlierEvent{
					eventType: evType,
					amount:    displayAmt,
					ledger:    seq,
					txHash:    txHash,
				}
				res.outliers = append(res.outliers, o)
				printMu.Lock()
				fmt.Printf("  ** %s %s PYUSD | ledger %d | tx %s\n", evType, displayAmt, seq, txHash)
				printMu.Unlock()
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

var bigOne = big.NewInt(10_000_000)

// parseStroops converts a decimal amount string (e.g. "3330000000000.0000000") to stroops as big.Int.
func parseStroops(s string) (*big.Int, bool) {
	dot := strings.IndexByte(s, '.')
	if dot < 0 {
		// Whole number — multiply by 10^7.
		v, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return nil, false
		}
		return v.Mul(v, bigOne), true
	}
	intPart := s[:dot]
	fracPart := s[dot+1:]
	// Pad or truncate to exactly 7 digits.
	for len(fracPart) < 7 {
		fracPart += "0"
	}
	fracPart = fracPart[:7]
	v, ok := new(big.Int).SetString(intPart+fracPart, 10)
	if !ok {
		return nil, false
	}
	return v, true
}

// stroopsToDisplay formats a big.Int stroops value as a decimal string with 7 fractional digits.
// Uses amount.IntStringToAmount which handles values exceeding int64.
func stroopsToDisplay(v *big.Int) string {
	s, err := amount.IntStringToAmount(v.String())
	if err != nil {
		return v.String() + " (raw stroops)"
	}
	return s
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
