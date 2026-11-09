package historyarchive

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// Serves a body that never ends, so the test controls when copying stops.
func endlessServer(t *testing.T, requests *atomic.Int64) *httptest.Server {
	t.Helper()
	stop := make(chan struct{})
	chunk := make([]byte, 256*1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests != nil {
			requests.Add(1)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not-a-gzip-header-xx"))
		w.(http.Flusher).Flush()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := w.Write(chunk); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			time.Sleep(4 * time.Millisecond) // keep the test off the developer's disk
		}
	}))
	t.Cleanup(func() { srv.Close() })
	t.Cleanup(func() { close(stop) })
	return srv
}

func finiteServer(t *testing.T, body []byte, requests *atomic.Int64) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests != nil {
			requests.Add(1)
		}
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestGetXdrStreamStopsCachingWhenStreamIsRejected(t *testing.T) {
	srv := endlessServer(t, nil)
	cachePath := t.TempDir()

	arch, err := Connect(srv.URL, ArchiveOptions{CachePath: cachePath})
	if err != nil {
		t.Fatal(err)
	}

	hash := MustDecodeHash("aabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabb")
	if _, err := arch.GetXdrStreamForHash(hash); err == nil {
		t.Fatal("expected the malformed payload to be rejected")
	}

	time.Sleep(500 * time.Millisecond)
	settled := dirSize(cachePath)
	time.Sleep(1 * time.Second)

	if grew := dirSize(cachePath) - settled; grew > 0 {
		t.Fatalf("cache kept growing by %d bytes after the stream was rejected", grew)
	}
}

func TestCachedGetStopsWritingWhenReaderIsClosed(t *testing.T) {
	srv := endlessServer(t, nil)
	cachePath := t.TempDir()

	arch, err := Connect(srv.URL, ArchiveOptions{CachePath: cachePath})
	if err != nil {
		t.Fatal(err)
	}

	hash := MustDecodeHash("aabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabb")
	rdr, err := arch.cachedGet(arch.GetBucketPathForHash(hash))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.CopyN(io.Discard, rdr, 512*1024); err != nil {
		t.Fatal(err)
	}
	rdr.Close()

	time.Sleep(300 * time.Millisecond)
	settled := dirSize(cachePath)
	time.Sleep(1 * time.Second)

	if grew := dirSize(cachePath) - settled; grew > 0 {
		t.Fatalf("cache kept growing by %d bytes after the reader was closed", grew)
	}
}

func TestCachedGetReaderCloseIsIdempotent(t *testing.T) {
	srv := endlessServer(t, nil)
	cachePath := t.TempDir()

	arch, err := Connect(srv.URL, ArchiveOptions{CachePath: cachePath})
	if err != nil {
		t.Fatal(err)
	}

	hash := MustDecodeHash("aabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabb")
	rdr, err := arch.cachedGet(arch.GetBucketPathForHash(hash))
	if err != nil {
		t.Fatal(err)
	}

	rdr.Close()
	rdr.Close()
	rdr.Close()
}

func TestCachedGetHitReaderCloseIsIdempotent(t *testing.T) {
	srv := finiteServer(t, bytes.Repeat([]byte("payload"), 1024), nil)

	arch, err := Connect(srv.URL, ArchiveOptions{CachePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	hash := MustDecodeHash("aabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabb")
	path := arch.GetBucketPathForHash(hash)

	rdr, err := arch.cachedGet(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.Copy(io.Discard, rdr); err != nil {
		t.Fatal(err)
	}
	rdr.Close()

	hit, err := arch.cachedGet(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := arch.stats.GetCacheHits(); got != 1 {
		t.Fatalf("expected the second get to be a cache hit, hits=%d", got)
	}
	if _, err := io.Copy(io.Discard, hit); err != nil {
		t.Fatal(err)
	}

	hit.Close()
	hit.Close()
	hit.Close()
}

func TestArchivePoolCachesEachFileOnce(t *testing.T) {
	var upstreamRequests atomic.Int64
	srv := finiteServer(t, bytes.Repeat([]byte("payload"), 1024), &upstreamRequests)

	cachePath := t.TempDir()
	pool, err := NewArchivePool([]string{srv.URL, srv.URL, srv.URL}, ArchiveOptions{CachePath: cachePath})
	if err != nil {
		t.Fatal(err)
	}

	members := pool.(*ArchivePool).pool
	hash := MustDecodeHash("aabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabb")

	for _, member := range members {
		arch := member.(*Archive)
		rdr, err := arch.cachedGet(arch.GetBucketPathForHash(hash))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(io.Discard, rdr); err != nil {
			t.Fatal(err)
		}
		rdr.Close()
	}

	if got := upstreamRequests.Load(); got != 1 {
		t.Fatalf("fetched the same path %d times through a pool of 3", got)
	}
}

func TestArchivePoolFailedConnectLeavesCachePathAlone(t *testing.T) {
	cachePath := t.TempDir()
	sentinel := filepath.Join(cachePath, "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := NewArchivePool([]string{""}, ArchiveOptions{CachePath: cachePath}); err == nil {
		t.Fatal("expected pool construction to fail")
	}

	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("failed pool construction disturbed the cache directory: %v", err)
	}
}

func TestCachedGetHonoursPerFileSizeLimit(t *testing.T) {
	srv := endlessServer(t, nil)
	cachePath := t.TempDir()

	const limit = 8 << 20
	arch, err := Connect(srv.URL, ArchiveOptions{CachePath: cachePath, MaxCacheFileSize: limit})
	if err != nil {
		t.Fatal(err)
	}

	hash := MustDecodeHash("aabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabb")
	path := arch.GetBucketPathForHash(hash)
	rdr, err := arch.cachedGet(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.CopyN(io.Discard, rdr, limit+(4<<20)); err != nil {
		t.Fatalf("a file too big to cache must still be served in full, got %v", err)
	}

	// Measured before Close, which would otherwise clean up a fill that
	// the limit failed to stop.
	if got := dirSize(cachePath); got > limit {
		t.Fatalf("wrote %d bytes past a %d byte limit", got, limit)
	}
	rdr.Close()
	if arch.cache.hasCompleted(path) {
		t.Fatal("a file past the per-file limit must not be cached")
	}
}

func TestCachedGetHonoursTotalCacheSizeLimit(t *testing.T) {
	srv := endlessServer(t, nil)
	cachePath := t.TempDir()

	const budget = 8 << 20
	arch, err := Connect(srv.URL, ArchiveOptions{CachePath: cachePath})
	if err != nil {
		t.Fatal(err)
	}
	arch.cache.maxTotalSize = budget

	hash := MustDecodeHash("aabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabb")
	path := arch.GetBucketPathForHash(hash)
	rdr, err := arch.cachedGet(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.CopyN(io.Discard, rdr, budget+(8<<20)); err != nil {
		t.Fatalf("spending the cache budget must not cut the download short, got %v", err)
	}

	// Measured before Close, which would otherwise clean up a fill that
	// the budget failed to stop.
	if got := dirSize(cachePath); got > budget {
		t.Fatalf("wrote %d bytes past a %d byte budget", got, budget)
	}
	rdr.Close()
	if arch.cache.hasCompleted(path) {
		t.Fatal("a file that spent the budget must not be cached")
	}
}

func TestCachedGetServesFullFileWhenCacheIsFull(t *testing.T) {
	body := bytes.Repeat([]byte("payload"), 4096)
	srv := finiteServer(t, body, nil)

	cachePath := t.TempDir()
	arch, err := Connect(srv.URL, ArchiveOptions{CachePath: cachePath})
	if err != nil {
		t.Fatal(err)
	}
	arch.cache.maxTotalSize = 1 << 20

	// Push the directory past the budget so every fill aborts immediately.
	if err = os.WriteFile(filepath.Join(cachePath, "ballast"), make([]byte, 2<<20), 0644); err != nil {
		t.Fatal(err)
	}

	hash := MustDecodeHash("aabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabb")
	rdr, err := arch.cachedGet(arch.GetBucketPathForHash(hash))
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(rdr)
	if err != nil {
		t.Fatal(err)
	}
	rdr.Close()

	if !bytes.Equal(got, body) {
		t.Fatalf("got %d bytes, want %d: a full cache must not corrupt downloads", len(got), len(body))
	}
}

func TestCachedGetInFlightDownloadsBypassTheCache(t *testing.T) {
	var requests atomic.Int64
	srv := endlessServer(t, &requests)

	arch, err := Connect(srv.URL, ArchiveOptions{CachePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	hash := MustDecodeHash("aabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabb")
	path := arch.GetBucketPathForHash(hash)

	first, err := arch.cachedGet(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.CopyN(io.Discard, first, 128*1024); err != nil {
		t.Fatal(err)
	}

	second, err := arch.cachedGet(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.CopyN(io.Discard, second, 128*1024); err != nil {
		t.Fatal(err)
	}

	// Abandoning the first download (and with it the cache fill) must not
	// disturb the second reader.
	first.Close()
	if _, err := io.CopyN(io.Discard, second, 1<<20); err != nil {
		t.Fatalf("second reader broke when the cache fill was abandoned: %v", err)
	}
	second.Close()

	if got := requests.Load(); got != 2 {
		t.Fatalf("expected 2 separate upstream downloads, got %d", got)
	}
	if hits := arch.stats.GetCacheHits(); hits != 0 {
		t.Fatalf("an in-flight bypass must not count as a cache hit, hits=%d", hits)
	}
}

// Serves a short prefix and then stalls until the test ends.
func stallingServer(t *testing.T) *httptest.Server {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("prefix-before-the-stall"))
		w.(http.Flusher).Flush()
		<-release
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	return srv
}

func TestCachedGetCachesSmallFileClosedBeforeEOF(t *testing.T) {
	var requests atomic.Int64
	srv := finiteServer(t, bytes.Repeat([]byte("payload"), 4096), &requests)

	arch, err := Connect(srv.URL, ArchiveOptions{CachePath: t.TempDir(), MaxCacheFileSize: -1})
	if err != nil {
		t.Fatal(err)
	}

	hash := MustDecodeHash("aabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabb")
	path := arch.GetBucketPathForHash(hash)

	rdr, err := arch.cachedGet(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.CopyN(io.Discard, rdr, 1); err != nil {
		t.Fatal(err)
	}
	rdr.Close()

	hit, err := arch.cachedGet(path)
	if err != nil {
		t.Fatal(err)
	}
	defer hit.Close()
	if got := requests.Load(); got != 1 {
		t.Fatalf("a small file closed before EOF should be drained and cached; upstream requests=%d", got)
	}
	if got, _ := io.Copy(io.Discard, hit); got != 7*4096 {
		t.Fatalf("cached copy has %d bytes, want %d", got, 7*4096)
	}
}

func TestCachedGetCloseFromAnotherGoroutineUnblocksRead(t *testing.T) {
	srv := stallingServer(t)
	cachePath := t.TempDir()

	arch, err := Connect(srv.URL, ArchiveOptions{CachePath: cachePath})
	if err != nil {
		t.Fatal(err)
	}

	hash := MustDecodeHash("aabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabb")
	path := arch.GetBucketPathForHash(hash)
	rdr, err := arch.cachedGet(path)
	if err != nil {
		t.Fatal(err)
	}

	readErr := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, rdr)
		readErr <- err
	}()
	time.Sleep(100 * time.Millisecond) // let the copy block on the stalled body
	rdr.Close()

	select {
	case err := <-readErr:
		if err == nil {
			t.Fatal("a read interrupted by Close must return an error, not EOF")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not unblock the stalled Read")
	}
	if arch.cache.hasCompleted(path) {
		t.Fatal("an interrupted download must not be cached")
	}
	arch.cache.fillMu.Lock()
	filling := arch.cache.filling.Contains(path)
	arch.cache.fillMu.Unlock()
	if filling {
		t.Fatal("the in-flight marker must be cleared after the fill is abandoned")
	}
}
