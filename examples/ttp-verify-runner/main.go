// TTP Verify Runner — runs TTP over ledgers, generates events (JSON), runs VerifyEvents,
// and prints per-ledger counts + verification results with a summary at the end.
//
// Modes:
//   - "cap-67" / "unified-events":  TTP reads events from the CAP-67 unified event stream
//     in LedgerCloseMeta (requires stellar-core EMIT_CLASSIC_EVENTS + BACKFILL_STELLAR_ASSET_EVENTS).
//   - "classic":  TTP derives events from operation types (pre-CAP-67 path).
//     VerifyEvents then compares derived events against actual ledger entry changes.
//
// Usage:
//
//	go run ./examples/ttp-verify-runner \
//	  --mode cap-67 \
//	  --start-ledger 61456768 --end-ledger 61457010
//
//	go run ./examples/ttp-verify-runner \
//	  --mode classic \
//	  --ledgers 61456768,61457010,61500000
//
//	go run ./examples/ttp-verify-runner \
//	  --mode cap-67 \
//	  --ledger-input-file ledgers.txt
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/processors/token_transfer"
	"github.com/stellar/go-stellar-sdk/support/datastore"
	"github.com/stellar/go-stellar-sdk/xdr"
	"google.golang.org/protobuf/encoding/protojson"
)

const defaultBucket = "sdf-ledger-close-meta/v1/ledgers/pubnet"

var protoMarshaler = protojson.MarshalOptions{
	Multiline:         true,
	EmitDefaultValues: true,
	Indent:            "  ",
}

type ledgerResult struct {
	LedgerSeq     uint32
	EventCount    int
	TransferCount int
	MintCount     int
	BurnCount     int
	ClawbackCount int
	FeeCount      int
	VerifyResult  string
	VerifyError   string
}

func usage() {
	fmt.Fprintf(os.Stderr, `TTP Verify Runner — generate TTP events and verify them against ledger entry changes.

Modes:
  cap-67 / unified-events   Read events from the CAP-67 unified event stream (default).
                             Requires stellar-core EMIT_CLASSIC_EVENTS + BACKFILL_STELLAR_ASSET_EVENTS.
  classic                    Derive events from operation types (pre-CAP-67 path).

Ledger input (pick one):
  --start-ledger + --end-ledger   Contiguous range (inclusive).
  --ledgers                       Comma-separated list.
  --ledger-input-file             File with one ledger per line (# comments ok).

Examples:
  go run ./examples/ttp-verify-runner --mode cap-67   --start-ledger 61456768 --end-ledger 61457010
  go run ./examples/ttp-verify-runner --mode classic  --ledgers 61456768,61457010
  go run ./examples/ttp-verify-runner --mode cap-67   --ledger-input-file ledgers.txt --quiet

Flags:
`)
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	startLedger := flag.Uint("start-ledger", 0, "Start ledger sequence (inclusive)")
	endLedger := flag.Uint("end-ledger", 0, "End ledger sequence (inclusive)")
	ledgerList := flag.String("ledgers", "", "Comma-separated list of ledger sequences")
	ledgerFile := flag.String("ledger-input-file", "", "File with one ledger sequence per line")
	mode := flag.String("mode", "cap-67", "TTP mode: 'cap-67' or 'classic'")
	bucket := flag.String("bucket", defaultBucket, "GCS bucket path for ledger data")
	bufferSize := flag.Uint("buffer-size", 500, "BSB prefetch buffer size")
	bsbWorkers := flag.Uint("bsb-workers", 10, "BSB internal fetch workers")
	retryLimit := flag.Uint("retry-limit", 3, "BSB retry limit")
	quiet := flag.Bool("quiet", false, "Suppress per-event JSON, only show per-ledger counts and summary")
	flag.Parse()

	useUnifiedEvents := true
	switch *mode {
	case "cap-67", "unified-events":
		useUnifiedEvents = true
	case "classic":
		useUnifiedEvents = false
	default:
		fmt.Fprintf(os.Stderr, "Error: invalid mode %q. Use 'cap-67' (unified events) or 'classic' (derive from ops)\n", *mode)
		os.Exit(1)
	}

	ledgers, err := parseLedgerInput(*startLedger, *endLedger, *ledgerList, *ledgerFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(ledgers) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no ledgers specified. Use --start-ledger/--end-ledger, --ledgers, or --ledger-input-file\n")
		flag.Usage()
		os.Exit(1)
	}

	sort.Slice(ledgers, func(i, j int) bool { return ledgers[i] < ledgers[j] })

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Fprintf(os.Stderr, "TTP Verify Runner\n")
	fmt.Fprintf(os.Stderr, "  Mode          : %s (unified-events=%v)\n", *mode, useUnifiedEvents)
	fmt.Fprintf(os.Stderr, "  Ledgers       : %d (first=%d, last=%d)\n", len(ledgers), ledgers[0], ledgers[len(ledgers)-1])
	fmt.Fprintf(os.Stderr, "  BSB           : buffer=%d, workers=%d, retries=%d\n", *bufferSize, *bsbWorkers, *retryLimit)
	fmt.Fprintf(os.Stderr, "  Bucket        : %s\n", *bucket)
	fmt.Fprintf(os.Stderr, "  Quiet         : %v\n", *quiet)
	fmt.Fprintf(os.Stderr, "\n")

	bsbCfg := ledgerbackend.BufferedStorageBackendConfig{
		BufferSize: uint32(*bufferSize),
		NumWorkers: uint32(*bsbWorkers),
		RetryLimit: uint32(*retryLimit),
		RetryWait:  30 * time.Second,
	}

	ranges := groupIntoRanges(ledgers)

	overallStart := time.Now()
	var results []ledgerResult

	for _, r := range ranges {
		if ctx.Err() != nil {
			break
		}
		rangeResults, err := processRange(ctx, r[0], r[1], *bucket, bsbCfg, useUnifiedEvents, *quiet)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Fatal error processing range %d-%d: %v\n", r[0], r[1], err)
			os.Exit(1)
		}
		results = append(results, rangeResults...)
	}

	// Print summary
	var totalEvents, totalTransfers, totalMints, totalBurns, totalClawbacks, totalFees int
	var passed, failed int
	var failedLedgers []uint32
	for _, r := range results {
		totalEvents += r.EventCount
		totalTransfers += r.TransferCount
		totalMints += r.MintCount
		totalBurns += r.BurnCount
		totalClawbacks += r.ClawbackCount
		totalFees += r.FeeCount
		if r.VerifyResult == "PASS" {
			passed++
		} else {
			failed++
			failedLedgers = append(failedLedgers, r.LedgerSeq)
		}
	}
	elapsed := time.Since(overallStart).Round(time.Millisecond)

	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "═══════════════════════════════════════════════════\n")
	fmt.Fprintf(os.Stderr, "  TTP Verify Summary\n")
	fmt.Fprintf(os.Stderr, "═══════════════════════════════════════════════════\n")
	fmt.Fprintf(os.Stderr, "  Mode            : %s\n", *mode)
	fmt.Fprintf(os.Stderr, "  Ledgers         : %d\n", len(results))
	fmt.Fprintf(os.Stderr, "  Total events    : %d\n", totalEvents)
	fmt.Fprintf(os.Stderr, "  Transfers       : %d\n", totalTransfers)
	fmt.Fprintf(os.Stderr, "  Mints           : %d\n", totalMints)
	fmt.Fprintf(os.Stderr, "  Burns           : %d\n", totalBurns)
	fmt.Fprintf(os.Stderr, "  Clawbacks       : %d\n", totalClawbacks)
	fmt.Fprintf(os.Stderr, "  Fees            : %d\n", totalFees)
	fmt.Fprintf(os.Stderr, "  Verify PASS     : %d\n", passed)
	fmt.Fprintf(os.Stderr, "  Verify FAIL     : %d\n", failed)
	if len(failedLedgers) > 0 {
		fmt.Fprintf(os.Stderr, "  Failed ledgers  : %v\n", failedLedgers)
	}
	fmt.Fprintf(os.Stderr, "  Elapsed         : %s\n", elapsed)
	fmt.Fprintf(os.Stderr, "═══════════════════════════════════════════════════\n")

	if failed > 0 {
		os.Exit(1)
	}
}

func processRange(
	ctx context.Context,
	from, to uint32,
	bucket string,
	bsbCfg ledgerbackend.BufferedStorageBackendConfig,
	useUnifiedEvents bool,
	quiet bool,
) ([]ledgerResult, error) {
	dsConfig := datastore.DataStoreConfig{
		Type: "GCS",
		Params: map[string]string{
			"destination_bucket_path": bucket,
		},
	}

	ds, err := datastore.NewDataStore(ctx, dsConfig)
	if err != nil {
		return nil, fmt.Errorf("create datastore: %w", err)
	}
	defer ds.Close()

	schema, err := datastore.LoadSchema(ctx, ds, dsConfig)
	if err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}

	backend, err := ledgerbackend.NewBufferedStorageBackend(bsbCfg, ds, schema)
	if err != nil {
		return nil, fmt.Errorf("create BSB: %w", err)
	}
	defer backend.Close()

	ledgerRange := ledgerbackend.BoundedRange(from, to)
	if err := backend.PrepareRange(ctx, ledgerRange); err != nil {
		return nil, fmt.Errorf("prepare range %d-%d: %w", from, to, err)
	}

	ttp := createTTP(useUnifiedEvents)
	var results []ledgerResult

	for seq := from; seq <= to; seq++ {
		if ctx.Err() != nil {
			return results, ctx.Err()
		}

		lcm, err := backend.GetLedger(ctx, seq)
		if err != nil {
			return results, fmt.Errorf("get ledger %d: %w", seq, err)
		}

		lr := processLedger(lcm, ttp, useUnifiedEvents, quiet)
		results = append(results, lr)

		status := "PASS"
		if lr.VerifyResult != "PASS" {
			status = "FAIL"
		}
		fmt.Fprintf(os.Stderr, "  [%s] ledger %d: %d events (T:%d M:%d B:%d C:%d F:%d)",
			status, lr.LedgerSeq, lr.EventCount,
			lr.TransferCount, lr.MintCount, lr.BurnCount, lr.ClawbackCount, lr.FeeCount)
		if lr.VerifyError != "" {
			fmt.Fprintf(os.Stderr, " err=%s", truncate(lr.VerifyError, 120))
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	return results, nil
}

func processLedger(
	lcm xdr.LedgerCloseMeta,
	ttp *token_transfer.EventsProcessor,
	useUnifiedEvents bool,
	quiet bool,
) ledgerResult {
	seq := lcm.LedgerSequence()
	lr := ledgerResult{LedgerSeq: seq}

	events, err := ttp.EventsFromLedger(lcm)
	if err != nil {
		lr.VerifyResult = "FAIL"
		lr.VerifyError = fmt.Sprintf("EventsFromLedger error: %v", err)
		return lr
	}

	for _, event := range events {
		lr.EventCount++
		switch event.GetEventType() {
		case token_transfer.TransferEvent:
			lr.TransferCount++
		case token_transfer.MintEvent:
			lr.MintCount++
		case token_transfer.BurnEvent:
			lr.BurnCount++
		case token_transfer.ClawbackEvent:
			lr.ClawbackCount++
		case token_transfer.FeeEvent:
			lr.FeeCount++
		}

		if !quiet {
			jsonBytes, err := protoMarshaler.Marshal(event)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  WARN: failed to marshal event to JSON: %v\n", err)
				continue
			}
			fmt.Printf("%s\n", jsonBytes)
		}
	}

	verifyErr := token_transfer.VerifyEvents(lcm, network.PublicNetworkPassphrase, useUnifiedEvents)
	if verifyErr != nil {
		lr.VerifyResult = "FAIL"
		lr.VerifyError = verifyErr.Error()
	} else {
		lr.VerifyResult = "PASS"
	}

	return lr
}

func createTTP(useUnifiedEvents bool) *token_transfer.EventsProcessor {
	if useUnifiedEvents {
		return token_transfer.NewEventsProcessor(
			network.PublicNetworkPassphrase,
			token_transfer.WithUnifiedEventsStreamEnabled(),
		)
	}
	return token_transfer.NewEventsProcessor(network.PublicNetworkPassphrase)
}

func parseLedgerInput(startLedger, endLedger uint, ledgerList, ledgerFile string) ([]uint32, error) {
	specified := 0
	if startLedger != 0 || endLedger != 0 {
		specified++
	}
	if ledgerList != "" {
		specified++
	}
	if ledgerFile != "" {
		specified++
	}
	if specified > 1 {
		return nil, fmt.Errorf("specify only one of: --start-ledger/--end-ledger, --ledgers, or --ledger-input-file")
	}

	var ledgers []uint32

	if startLedger != 0 || endLedger != 0 {
		if startLedger == 0 || endLedger == 0 {
			return nil, fmt.Errorf("both --start-ledger and --end-ledger are required for range mode")
		}
		if startLedger > endLedger {
			return nil, fmt.Errorf("--start-ledger (%d) must be <= --end-ledger (%d)", startLedger, endLedger)
		}
		for seq := uint32(startLedger); seq <= uint32(endLedger); seq++ {
			ledgers = append(ledgers, seq)
		}
		return ledgers, nil
	}

	if ledgerList != "" {
		parts := strings.Split(ledgerList, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			n, err := strconv.ParseUint(part, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid ledger number %q: %w", part, err)
			}
			ledgers = append(ledgers, uint32(n))
		}
		return dedup(ledgers), nil
	}

	if ledgerFile != "" {
		f, err := os.Open(ledgerFile)
		if err != nil {
			return nil, fmt.Errorf("open ledger input file: %w", err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			n, err := strconv.ParseUint(line, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid ledger number %q at line %d: %w", line, lineNum, err)
			}
			ledgers = append(ledgers, uint32(n))
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("reading ledger input file: %w", err)
		}
		return dedup(ledgers), nil
	}

	return nil, nil
}

func groupIntoRanges(ledgers []uint32) [][2]uint32 {
	if len(ledgers) == 0 {
		return nil
	}

	var ranges [][2]uint32
	from := ledgers[0]
	to := ledgers[0]

	for i := 1; i < len(ledgers); i++ {
		if ledgers[i] == to+1 {
			to = ledgers[i]
		} else {
			ranges = append(ranges, [2]uint32{from, to})
			from = ledgers[i]
			to = ledgers[i]
		}
	}
	ranges = append(ranges, [2]uint32{from, to})
	return ranges
}

func dedup(ledgers []uint32) []uint32 {
	sort.Slice(ledgers, func(i, j int) bool { return ledgers[i] < ledgers[j] })
	if len(ledgers) == 0 {
		return ledgers
	}
	result := []uint32{ledgers[0]}
	for i := 1; i < len(ledgers); i++ {
		if ledgers[i] != ledgers[i-1] {
			result = append(result, ledgers[i])
		}
	}
	return result
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
