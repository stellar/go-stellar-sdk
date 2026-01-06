package xdr

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

const (
	// testLedgersPath is the path to the cached multi-ledger test fixture.
	testLedgersPath = "testdata/benchmark-ledgers.xdr.zst"
	// defaultRPCURL is used to fetch ledgers if no fixture exists.
	defaultRPCURL = "https://rpc.lightsail.network/"
	// numLedgersToFetch is the number of ledgers to download from RPC.
	numLedgersToFetch = 100
)

// loadTestLedgers loads multiple LedgerCloseMeta from test files.
// If no multi-ledger fixture exists and XDR_BENCHMARK_DOWNLOAD=1 is set,
// it downloads ledgers from a public RPC. Otherwise falls back to single ledger.
func loadTestLedgers(tb testing.TB) [][]byte {
	// Try to load from cached zstd file
	ledgers := loadLedgersFromZstd(tb, testLedgersPath)
	if len(ledgers) > 0 {
		return ledgers
	}

	// No fixture found - check if download is enabled
	if os.Getenv("XDR_BENCHMARK_DOWNLOAD") == "1" {
		tb.Logf("no fixture at %s, downloading from RPC...", testLedgersPath)
		if err := downloadLedgersFixture(tb, testLedgersPath); err != nil {
			tb.Logf("failed to download ledgers: %v", err)
		} else {
			// Try loading again after download
			ledgers = loadLedgersFromZstd(tb, testLedgersPath)
			if len(ledgers) > 0 {
				return ledgers
			}
		}
	}

	// Fall back to single ledger file
	data, err := os.ReadFile("testdata/ledger_close_meta.xdr")
	if err != nil {
		tb.Skipf("no test data available (set XDR_BENCHMARK_DOWNLOAD=1 to fetch from RPC)")
	}
	return [][]byte{data}
}

// rpcRequest is a JSON-RPC 2.0 request.
type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// rpcResponse is a JSON-RPC 2.0 response.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// getLedgersParams are the parameters for getLedgers RPC method.
type getLedgersParams struct {
	StartLedger uint32                `json:"startLedger,omitempty"`
	Pagination  *ledgerPaginationOpts `json:"pagination,omitempty"`
	Format      string                `json:"xdrFormat,omitempty"`
}

type ledgerPaginationOpts struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  uint   `json:"limit,omitempty"`
}

// getLedgersResult is the result of getLedgers RPC method.
type getLedgersResult struct {
	Ledgers []ledgerInfo `json:"ledgers"`
	Cursor  string       `json:"cursor"`
}

type ledgerInfo struct {
	Sequence       uint32 `json:"sequence"`
	LedgerMetadata string `json:"metadataXdr"`
}

// getLatestLedgerResult is the result of getLatestLedger RPC method.
type getLatestLedgerResult struct {
	Sequence uint32 `json:"sequence"`
}

// callRPC makes a JSON-RPC call to the given URL.
func callRPC(url, method string, params any, result any) error {
	reqBody, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return err
	}

	if rpcResp.Error != nil {
		return fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return json.Unmarshal(rpcResp.Result, result)
}

// downloadLedgersFixture fetches ledgers from a public RPC and saves them
// as a compressed LedgerCloseMetaBatch fixture.
func downloadLedgersFixture(tb testing.TB, path string) error {
	// Get latest ledger
	var latest getLatestLedgerResult
	if err := callRPC(defaultRPCURL, "getLatestLedger", nil, &latest); err != nil {
		return err
	}
	tb.Logf("latest ledger: %d", latest.Sequence)

	// Calculate start ledger
	startLedger := latest.Sequence - uint32(numLedgersToFetch)

	// Fetch ledgers in batches
	var allMetas []LedgerCloseMeta
	cursor := ""

	for len(allMetas) < numLedgersToFetch {
		params := getLedgersParams{
			Format: "base64",
			Pagination: &ledgerPaginationOpts{
				Limit: 20,
			},
		}
		if cursor != "" {
			params.Pagination.Cursor = cursor
		} else {
			params.StartLedger = startLedger
		}

		var result getLedgersResult
		if err := callRPC(defaultRPCURL, "getLedgers", params, &result); err != nil {
			return err
		}

		if len(result.Ledgers) == 0 {
			break
		}

		for _, ledger := range result.Ledgers {
			metaBytes, err := base64.StdEncoding.DecodeString(ledger.LedgerMetadata)
			if err != nil {
				return err
			}

			var lcm LedgerCloseMeta
			if err := SafeUnmarshal(metaBytes, &lcm); err != nil {
				return err
			}

			allMetas = append(allMetas, lcm)
		}

		cursor = result.Cursor
		if cursor == "" {
			break
		}
	}

	tb.Logf("fetched %d ledgers", len(allMetas))

	if len(allMetas) == 0 {
		return fmt.Errorf("no ledgers fetched from RPC")
	}

	// Create LedgerCloseMetaBatch
	batch := LedgerCloseMetaBatch{
		StartSequence:    Uint32(allMetas[0].LedgerSequence()),
		EndSequence:      Uint32(allMetas[len(allMetas)-1].LedgerSequence()),
		LedgerCloseMetas: allMetas,
	}

	// Marshal and compress
	batchBytes, err := batch.MarshalBinary()
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder, err := zstd.NewWriter(f, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	if err != nil {
		return err
	}

	if _, err := encoder.Write(batchBytes); err != nil {
		encoder.Close()
		return err
	}

	return encoder.Close()
}

// loadLedgersFromZstd reads LedgerCloseMetaBatch from a zstd file and
// returns individual ledgers as serialized bytes.
func loadLedgersFromZstd(tb testing.TB, path string) [][]byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	decoder, err := zstd.NewReader(f)
	if err != nil {
		return nil
	}
	defer decoder.Close()

	data, err := io.ReadAll(decoder)
	if err != nil {
		tb.Logf("error reading zstd data: %v", err)
		return nil
	}

	var batch LedgerCloseMetaBatch
	if err = SafeUnmarshal(data, &batch); err != nil {
		tb.Logf("error unmarshaling batch: %v", err)
		return nil
	}

	// Limit to numLedgersToFetch ledgers for benchmarks
	metas := batch.LedgerCloseMetas
	if len(metas) > numLedgersToFetch {
		metas = metas[:numLedgersToFetch]
	}

	ledgers := make([][]byte, len(metas))
	for i, lcm := range metas {
		ledgers[i], err = lcm.MarshalBinary()
		if err != nil {
			tb.Logf("error marshaling ledger %d: %v", i, err)
			return nil
		}
	}

	return ledgers
}

// BenchmarkDecodeWithoutPool measures allocations without pooling.
func BenchmarkDecodeWithoutPool(b *testing.B) {
	ledgers := loadTestLedgers(b)
	b.Logf("loaded %d ledgers for benchmark", len(ledgers))

	var totalBytes int64
	for _, data := range ledgers {
		totalBytes += int64(len(data))
	}
	avgBytes := totalBytes / int64(len(ledgers))

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(avgBytes)

	for i := 0; i < b.N; i++ {
		data := ledgers[i%len(ledgers)]
		var lcm LedgerCloseMeta
		if err := SafeUnmarshal(data, &lcm); err != nil {
			b.Fatalf("decode failed: %v", err)
		}
	}
}

// BenchmarkDecodeWithPool measures allocations with pooling.
func BenchmarkDecodeWithPool(b *testing.B) {
	ledgers := loadTestLedgers(b)
	b.Logf("loaded %d ledgers for benchmark", len(ledgers))

	pool := NewPool[LedgerCloseMeta]()

	// Warmup - cycle through different ledgers to populate internal structures
	for i := 0; i < len(ledgers)*2; i++ {
		obj := pool.Get()
		data := ledgers[i%len(ledgers)]
		if err := SafeUnmarshal(data, obj); err != nil {
			b.Fatalf("warmup decode failed: %v", err)
		}
		pool.Put(obj)
	}

	var totalBytes int64
	for _, data := range ledgers {
		totalBytes += int64(len(data))
	}
	avgBytes := totalBytes / int64(len(ledgers))

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(avgBytes)

	for i := 0; i < b.N; i++ {
		data := ledgers[i%len(ledgers)]
		lcm := pool.Get()
		if err := SafeUnmarshal(data, lcm); err != nil {
			b.Fatalf("decode failed: %v", err)
		}
		pool.Put(lcm)
	}
}

// BenchmarkDecodeWithPoolParallel measures concurrent pooled decoding.
func BenchmarkDecodeWithPoolParallel(b *testing.B) {
	ledgers := loadTestLedgers(b)
	b.Logf("loaded %d ledgers for benchmark", len(ledgers))

	pool := NewPool[LedgerCloseMeta]()

	var totalBytes int64
	for _, data := range ledgers {
		totalBytes += int64(len(data))
	}
	avgBytes := totalBytes / int64(len(ledgers))

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(avgBytes)

	var idx atomic.Int64
	b.RunParallel(func(pb *testing.PB) {
		// Per-goroutine warmup with varied ledgers
		for i := 0; i < len(ledgers); i++ {
			obj := pool.Get()
			data := ledgers[i%len(ledgers)]
			if err := SafeUnmarshal(data, obj); err != nil {
				b.Errorf("warmup decode failed: %v", err)
				return
			}
			pool.Put(obj)
		}

		localIdx := idx.Add(1)
		for pb.Next() {
			data := ledgers[localIdx%int64(len(ledgers))]
			localIdx++
			lcm := pool.Get()
			if err := SafeUnmarshal(data, lcm); err != nil {
				b.Errorf("decode failed: %v", err)
				return
			}
			pool.Put(lcm)
		}
	})
}

// TestMemoryStabilityWithPool verifies memory doesn't grow unbounded
// when decoding varied ledgers.
func TestMemoryStabilityWithPool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-running memory test")
	}

	ledgers := loadTestLedgers(t)
	if len(ledgers) == 0 {
		t.Skip("no test data found")
	}
	t.Logf("loaded %d ledgers for memory test", len(ledgers))

	pool := NewPool[LedgerCloseMeta]()

	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	initialAlloc := m.Alloc

	for i := 0; i < 10000; i++ {
		data := ledgers[i%len(ledgers)]
		lcm := pool.Get()
		if err := SafeUnmarshal(data, lcm); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		pool.Put(lcm)

		if i%1000 == 0 {
			runtime.GC()
		}
	}

	runtime.GC()
	runtime.ReadMemStats(&m)
	finalAlloc := m.Alloc

	// Memory should not grow significantly (allow 50MB variance)
	growth := int64(finalAlloc) - int64(initialAlloc)
	if growth > 50*1024*1024 {
		t.Errorf("memory grew by %d bytes, expected stable", growth)
	}
}
