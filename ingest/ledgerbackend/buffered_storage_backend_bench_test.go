package ledgerbackend

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/support/datastore"
)

// Benchmarks GetLedger / RawLedgers stream throughput against a real GCS-backed
// data store. Off by default — set BSB_BENCH_BUCKET to a valid GCS bucket
// path and BSB_BENCH_FROM/BSB_BENCH_TO to the ledger range. Requires GCS
// application-default credentials.
//
// Sweeps NumWorkers across {5, 10, 20, 50, 100} to identify the throughput
// knee. BufferSize is fixed at 10000 — newLedgerBuffer caps it to the range
// size internally, so the effective buffer is min(10000, range). This sizes
// the pipeline so workers always see all queued tasks upfront and never
// throttle on the per-consumer-pull pushTaskQueue feedback loop. Use
// -benchtime=1x for a single pass per config.
//
// Example:
//
//	BSB_BENCH_BUCKET=sdf-ledger-close-meta/ledgers/pubnet \
//	BSB_BENCH_FROM=58752000 BSB_BENCH_TO=58752999 \
//	go test -run='^$' -bench=BenchmarkBSB -benchtime=1x \
//	  ./ingest/ledgerbackend/

const (
	bsbBenchBucketEnv = "BSB_BENCH_BUCKET"
	bsbBenchFromEnv   = "BSB_BENCH_FROM"
	bsbBenchToEnv     = "BSB_BENCH_TO"
	bsbBenchLPFEnv    = "BSB_BENCH_LEDGERS_PER_FILE"
)

var bsbBenchWorkerCounts = []uint32{5, 10, 20, 50, 100}

type bsbBenchEnv struct {
	bucket         string
	fromLedger     uint32
	toLedger       uint32
	ledgersPerFile uint32
}

func loadBenchEnv(b *testing.B) bsbBenchEnv {
	b.Helper()
	bucket := os.Getenv(bsbBenchBucketEnv)
	fromStr := os.Getenv(bsbBenchFromEnv)
	toStr := os.Getenv(bsbBenchToEnv)
	if bucket == "" || fromStr == "" || toStr == "" {
		b.Skipf("set %s, %s, %s to run", bsbBenchBucketEnv, bsbBenchFromEnv, bsbBenchToEnv)
	}
	var from, to uint32
	if _, err := fmt.Sscan(fromStr, &from); err != nil {
		b.Fatalf("parse %s: %v", bsbBenchFromEnv, err)
	}
	if _, err := fmt.Sscan(toStr, &to); err != nil {
		b.Fatalf("parse %s: %v", bsbBenchToEnv, err)
	}
	if to < from {
		b.Fatalf("%s (%d) must be >= %s (%d)", bsbBenchToEnv, to, bsbBenchFromEnv, from)
	}
	lpf := uint32(1)
	if v := os.Getenv(bsbBenchLPFEnv); v != "" {
		if _, err := fmt.Sscan(v, &lpf); err != nil {
			b.Fatalf("parse %s: %v", bsbBenchLPFEnv, err)
		}
	}
	return bsbBenchEnv{bucket: bucket, fromLedger: from, toLedger: to, ledgersPerFile: lpf}
}

func setupBenchBSB(b *testing.B, env bsbBenchEnv, numWorkers uint32) *BufferedStorageBackend {
	b.Helper()

	schema := datastore.DataStoreSchema{
		LedgersPerFile:    env.ledgersPerFile,
		FilesPerPartition: 64000 / env.ledgersPerFile,
		FileExtension:     "zstd",
	}

	store, err := datastore.NewDataStore(context.Background(), datastore.DataStoreConfig{
		Type:   "GCS",
		Params: map[string]string{"destination_bucket_path": env.bucket},
		Schema: schema,
	})
	require.NoError(b, err)

	bsb, err := NewBufferedStorageBackend(BufferedStorageBackendConfig{
		BufferSize: 10000,
		NumWorkers: numWorkers,
		RetryLimit: 0,
		RetryWait:  time.Microsecond,
	}, store, schema)
	require.NoError(b, err)

	require.NoError(b, bsb.PrepareRange(context.Background(), BoundedRange(env.fromLedger, env.toLedger)))
	return bsb
}

// runBSBBench drives `consume` over the full configured range for each
// NumWorkers value in bsbBenchWorkerCounts. b.SetBytes is set to the ledger
// count so the standard MB/s output column reads as ledgers/sec.
func runBSBBench(b *testing.B, consume func(ctx context.Context, bsb *BufferedStorageBackend, seq uint32) error) {
	env := loadBenchEnv(b)
	for _, nw := range bsbBenchWorkerCounts {
		b.Run(fmt.Sprintf("Workers=%d", nw), func(b *testing.B) {
			ctx := context.Background()
			count := env.toLedger - env.fromLedger + 1
			b.SetBytes(int64(count))
			for i := 0; i < b.N; i++ {
				bsb := setupBenchBSB(b, env, nw)
				b.StartTimer()
				for seq := env.fromLedger; seq <= env.toLedger; seq++ {
					if err := consume(ctx, bsb, seq); err != nil {
						b.Fatalf("consume(%d): %v", seq, err)
					}
				}
				b.StopTimer()
				require.NoError(b, bsb.Close())
			}
		})
	}
}

func BenchmarkBSBGetLedger(b *testing.B) {
	runBSBBench(b, func(ctx context.Context, bsb *BufferedStorageBackend, seq uint32) error {
		_, err := bsb.GetLedger(ctx, seq)
		return err
	})
}

func BenchmarkBufferedStorageStream(b *testing.B) {
	env := loadBenchEnv(b)
	for _, nw := range bsbBenchWorkerCounts {
		b.Run(fmt.Sprintf("Workers=%d", nw), func(b *testing.B) {
			ctx := context.Background()
			count := env.toLedger - env.fromLedger + 1
			b.SetBytes(int64(count))
			schema := datastore.DataStoreSchema{
				LedgersPerFile:    env.ledgersPerFile,
				FilesPerPartition: 64000 / env.ledgersPerFile,
				FileExtension:     "zstd",
			}
			dsConfig := datastore.DataStoreConfig{
				Type:   "GCS",
				Params: map[string]string{"destination_bucket_path": env.bucket},
				Schema: schema,
			}
			cfg := BufferedStorageBackendConfig{BufferSize: 10000, NumWorkers: nw, RetryLimit: 0, RetryWait: time.Microsecond}
			for i := 0; i < b.N; i++ {
				s := NewBufferedStorageStream(cfg, dsConfig, nil)
				b.StartTimer()
				for _, err := range s.RawLedgers(ctx, BoundedRange(env.fromLedger, env.toLedger)) {
					if err != nil {
						b.Fatalf("stream: %v", err)
					}
				}
				b.StopTimer()
			}
		})
	}
}

// BenchmarkBSBPipelineNoDecode measures the BSB worker→pipeline→queue overhead
// alone — workers fetch compressed bytes, the consumer drains them from
// ledgerQueue WITHOUT decompression or view-parsing. This isolates the
// pipeline cost from XDR/zstd work, giving a clean comparison vs the parallel
// store.GetFile probe (same underlying network work, different pipeline shape).
func BenchmarkBSBPipelineNoDecode(b *testing.B) {
	runBSBBench(b, func(ctx context.Context, bsb *BufferedStorageBackend, _ uint32) error {
		compressed, err := bsb.ledgerBuffer.getFromLedgerQueue(ctx)
		if err != nil {
			return err
		}
		bsb.ledgerBuffer.compressedPool.Put(compressed)
		return nil
	})
}
