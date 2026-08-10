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
func endlessServer(t *testing.T, served *atomic.Int64) *httptest.Server {
	t.Helper()
	stop := make(chan struct{})
	chunk := make([]byte, 256*1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not-a-gzip-header-xx"))
		w.(http.Flusher).Flush()
		for {
			select {
			case <-stop:
				return
			default:
			}
			n, err := w.Write(chunk)
			if served != nil {
				served.Add(int64(n))
			}
			if err != nil {
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

func cacheDirSize(t *testing.T, dir string) int64 {
	t.Helper()
	var total int64
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
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
	settled := cacheDirSize(t, cachePath)
	time.Sleep(1 * time.Second)

	if grew := cacheDirSize(t, cachePath) - settled; grew > 0 {
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
	time.Sleep(300 * time.Millisecond)
	rdr.Close()

	time.Sleep(300 * time.Millisecond)
	settled := cacheDirSize(t, cachePath)
	time.Sleep(1 * time.Second)

	if grew := cacheDirSize(t, cachePath) - settled; grew > 0 {
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

func TestArchivePoolCachesEachFileOnce(t *testing.T) {
	var upstreamRequests atomic.Int64
	body := bytes.Repeat([]byte("payload"), 1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamRequests.Add(1)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)

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

func TestCachedGetHonoursPerFileSizeLimit(t *testing.T) {
	srv := endlessServer(t, nil)
	cachePath := t.TempDir()

	const limit = 8 << 20
	arch, err := Connect(srv.URL, ArchiveOptions{CachePath: cachePath, MaxCacheFileSize: limit})
	if err != nil {
		t.Fatal(err)
	}

	hash := MustDecodeHash("aabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabbccddeeffaabb")
	rdr, err := arch.cachedGet(arch.GetBucketPathForHash(hash))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, rdr)
	rdr.Close()

	// Allow slack for the chunk in flight when the limit tripped.
	if got := cacheDirSize(t, cachePath); got > limit+(8<<20) {
		t.Fatalf("wrote %d bytes past a %d byte limit", got, limit)
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
	rdr, err := arch.cachedGet(arch.GetBucketPathForHash(hash))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, rdr)
	rdr.Close()

	// Allow slack for the chunk in flight when the budget was reached.
	if got := cacheDirSize(t, cachePath); got > budget+(8<<20) {
		t.Fatalf("wrote %d bytes past a %d byte budget", got, budget)
	}
}
