// Copyright 2016 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package historyarchive

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	fscache "github.com/djherbis/fscache"
	log "github.com/sirupsen/logrus"

	"github.com/stellar/go-stellar-sdk/support/collections/set"
	"github.com/stellar/go-stellar-sdk/support/errors"
	supportlog "github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/support/storage"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const hexPrefixPat = "/[0-9a-f]{2}/[0-9a-f]{2}/[0-9a-f]{2}/"
const rootHASPath = ".well-known/stellar-history.json"
const maxHASSize = 10 * 1024 * 1024 // 10 MB

const (
	// Hard ceiling on the cache directory's size. A download that would push
	// the directory past it is served in full but no longer cached.
	cacheTotalSizeBudget = 10 << 30 // 10 GiB

	// The LRU eviction pass, run once a minute, trims the directory to this
	// low watermark. It sits below cacheTotalSizeBudget on purpose. Eviction
	// skips open files and its own metadata, so it measures less than the
	// budget check does. With one shared number a full cache could look small
	// enough to evict nothing.
	cacheEvictionTarget = 8 << 30 // 8 GiB

	// The biggest bucket the pubnet archive serves today is around 2.2 GiB.
	// This leaves room for growth while staying below cacheTotalSizeBudget,
	// which remains the binding limit.
	defaultMaxCacheFileSize = 8 << 30 // 8 GiB

	cacheUsageScanInterval = time.Second

	// Writes into a cache entry are buffered to this size. The consumer reads
	// in 4 KiB pieces, which would otherwise be one write call each.
	cacheFillBufferSize = 1 << 20 // 1 MiB

	// How much of a file Close reads past the point where the caller stopped,
	// so that a small file read only part way is still cached whole.
	cacheDrainOnCloseLimit = 4 << 20 // 4 MiB

	// How long that drain may take before the upstream is closed under it.
	cacheDrainOnCloseTimeout = time.Second
)

var (
	errCacheFileTooLarge = errors.New("file is larger than the per-file cache limit")
	errCacheBudgetSpent  = errors.New("cache has no room left within its size budget")
)

type CommandOptions struct {
	Concurrency  int
	Range        Range
	DryRun       bool
	Force        bool
	Verify       bool
	Thorough     bool
	SkipOptional bool
}

type ArchiveOptions struct {
	storage.ConnectOptions

	Logger *supportlog.Entry
	// NetworkPassphrase defines the expected network of history archive. It is
	// checked when getting HAS. If network passphrase does not match, error is
	// returned.
	NetworkPassphrase string
	// CheckpointFrequency is the number of ledgers between checkpoints
	// if unset, DefaultCheckpointFrequency will be used
	CheckpointFrequency uint32
	// CachePath controls where/if bucket files are cached on the disk.
	CachePath string
	// MaxCacheFileSize is the largest single file, in bytes, that will be
	// written to the on-disk cache. A larger file is still downloaded and
	// served in full; it just isn't cached. Zero or a negative value selects
	// the default of 8 GiB.
	MaxCacheFileSize int64
}

type Ledger struct {
	Header            xdr.LedgerHeaderHistoryEntry
	Transaction       xdr.TransactionHistoryEntry
	TransactionResult xdr.TransactionHistoryResultEntry
}

type ArchiveInterface interface {
	GetPathHAS(path string) (HistoryArchiveState, error)
	PutPathHAS(path string, has HistoryArchiveState, opts *CommandOptions) error
	BucketExists(bucket Hash) (bool, error)
	BucketSize(bucket Hash) (int64, error)
	CategoryCheckpointExists(cat string, chk uint32) (bool, error)
	GetLedgerHeader(chk uint32) (xdr.LedgerHeaderHistoryEntry, error)
	GetRootHAS() (HistoryArchiveState, error)
	GetLedgers(start, end uint32) (map[uint32]*Ledger, error)
	GetLatestLedgerSequence() (uint32, error)
	GetCheckpointHAS(chk uint32) (HistoryArchiveState, error)
	PutCheckpointHAS(chk uint32, has HistoryArchiveState, opts *CommandOptions) error
	PutRootHAS(has HistoryArchiveState, opts *CommandOptions) error
	ListBucket(dp DirPrefix) (chan string, chan error)
	ListAllBuckets() (chan string, chan error)
	ListAllBucketHashes() (chan Hash, chan error)
	ListCategoryCheckpoints(cat string, pth string) (chan uint32, chan error)
	GetXdrStreamForHash(hash Hash) (*xdr.Stream, error)
	GetXdrStream(pth string) (*xdr.Stream, error)
	GetCheckpointManager() CheckpointManager
	GetStats() []ArchiveStats
}

var _ ArchiveInterface = &Archive{}

type Archive struct {
	networkPassphrase string

	mutex             sync.Mutex
	checkpointFiles   map[string](map[uint32]bool)
	allBuckets        map[Hash]bool
	referencedBuckets map[Hash]bool

	expectLedgerHashes      map[uint32]Hash
	actualLedgerHashes      map[uint32]Hash
	expectTxSetHashes       map[uint32]Hash
	actualTxSetHashes       map[uint32]Hash
	expectTxResultSetHashes map[uint32]Hash
	actualTxResultSetHashes map[uint32]Hash

	invalidBuckets int

	invalidLedgers      int
	invalidTxSets       int
	invalidTxResultSets int

	checkpointManager CheckpointManager

	backend storage.Storage
	stats   archiveStats
	cache   *archiveBucketCache
}

type archiveBucketCache struct {
	fscache.Cache

	path  string
	sizes sync.Map

	maxTotalSize int64
	maxFileSize  int64
	usage        cacheUsage

	// Paths whose entries are being written right now. Only completed entries
	// are ever served; see acquire.
	fillMu  sync.Mutex
	filling set.Set[string]
}

// acquire returns a reader when pth is fully cached, a writer the caller must
// fill when it is not, or inFlight=true when another goroutine is filling it.
// In that last case the caller must bypass the cache and download directly.
//
// A half-written entry is never handed out. The cache library cannot report
// an error to the readers of an entry whose writer stopped early: they see a
// clean EOF and return a shorter, complete-looking file. A follow-up that
// writes entries to a temporary name and renames them on commit would make
// this bookkeeping unnecessary.
func (c *archiveBucketCache) acquire(pth string) (rdr io.ReadCloser, wrtr io.WriteCloser, inFlight bool, err error) {
	c.fillMu.Lock()
	defer c.fillMu.Unlock()

	if c.filling.Contains(pth) {
		return nil, nil, true, nil
	}

	rdr, wrtr, err = c.Get(pth)
	if err != nil {
		return nil, nil, false, err
	}
	if wrtr == nil {
		return rdr, nil, false, nil
	}

	// Nothing reads the cache entry until the fill completes.
	rdr.Close()
	c.filling.Add(pth)
	return nil, wrtr, false, nil
}

// finishFill closes a fill's writer and, unless the fill completed cleanly,
// deletes the entry. pth leaves the in-flight set only after both steps, so
// no other caller can observe the entry half-written or mid-removal. If the
// entry could not be deleted, pth stays in the set, so the partial file is
// bypassed instead of served.
func (c *archiveBucketCache) finishFill(pth string, wrtr io.WriteCloser, complete bool) (closeErr, removeErr error) {
	closeErr = wrtr.Close()
	if !complete || closeErr != nil {
		removeErr = c.Remove(pth)
		if removeErr != nil {
			return closeErr, removeErr
		}
	}

	c.fillMu.Lock()
	c.filling.Remove(pth)
	c.fillMu.Unlock()
	return closeErr, removeErr
}

func (c *archiveBucketCache) hasCompleted(pth string) bool {
	c.fillMu.Lock()
	filling := c.filling.Contains(pth)
	c.fillMu.Unlock()
	return !filling && c.Exists(pth)
}

// cacheUsage tracks how many bytes the cache directory holds. The cache
// library's own total leaves out files that are still open, which is exactly
// what a download in progress is. So the directory is measured here instead:
// a walk in the background at most once per cacheUsageScanInterval, plus the
// bytes the running fills have written since, plus the bytes sitting in their
// write buffers.
type cacheUsage struct {
	mu       sync.Mutex
	onDisk   int64 // directory size at the last walk
	flushed  int64 // bytes fills wrote to disk since that walk started
	reserved int64 // bytes fills accepted into their buffers, not yet on disk
	scanning bool
	at       time.Time
	// unmeasured is set when the last walk failed. The directory size is then
	// unknown, so nothing is cached until a walk succeeds.
	unmeasured bool
}

// reserve accepts n more bytes into a fill's buffer when the total stays
// within limit. The check and the accounting happen under one lock, so two
// fills cannot both claim the last of the budget.
func (u *cacheUsage) reserve(dir string, n, limit int64) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.scanLocked(dir)
	if u.unmeasured || u.onDisk+u.flushed+u.reserved+n > limit {
		return false
	}
	u.reserved += n
	return true
}

// flush moves n bytes from a fill's buffer to disk in the accounting.
func (u *cacheUsage) flush(n int64) {
	u.mu.Lock()
	u.reserved -= n
	u.flushed += n
	u.mu.Unlock()
}

// discard drops n buffered bytes that will never reach disk.
func (u *cacheUsage) discard(n int64) {
	u.mu.Lock()
	u.reserved -= n
	u.mu.Unlock()
}

func (u *cacheUsage) scanLocked(dir string) {
	if u.scanning || time.Since(u.at) < cacheUsageScanInterval {
		return
	}
	u.scanning = true
	// Bytes flushed before the walk starts are on disk, so the walk counts
	// them. Bytes flushed during the walk may or may not be counted; keeping
	// them errs on the side of a larger total.
	before := u.flushed
	go func() {
		n, err := dirSize(dir)
		if err != nil {
			log.WithError(err).WithField("cache", dir).
				Warn("Measuring the cache directory failed; caching paused until it succeeds")
		}
		u.mu.Lock()
		u.onDisk = n
		u.unmeasured = err != nil
		u.flushed -= before
		u.at = time.Now()
		u.scanning = false
		u.mu.Unlock()
	}()
}

// flushCounter reports to the usage accounting each write that the fill's
// buffer passes on to the cache entry.
type flushCounter struct {
	w     io.Writer
	usage *cacheUsage
}

func (f *flushCounter) Write(b []byte) (int, error) {
	n, err := f.w.Write(b)
	f.usage.flush(int64(n))
	return n, err
}

// dirSize returns the bytes held by the files under dir. A file that was
// removed while the walk ran is not an error; anything else is, and the
// total returned with it is a lower bound.
func dirSize(dir string) (int64, error) {
	var total int64
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// cacheFillingReader streams a file from the upstream archive to the caller
// and writes the same bytes into a cache entry on the side. The caller always
// receives exactly what the upstream sent. When caching has to stop, because
// of a size limit or a failed cache write, the half-written entry is deleted
// and the reader keeps streaming. A fill is kept only if it reached the
// upstream's EOF.
type cacheFillingReader struct {
	upstream io.ReadCloser
	cache    *archiveBucketCache
	path     string
	log      *log.Entry

	// mu serialises Read and Close. Close may run on another goroutine while
	// a Read is blocked; see Close.
	mu      sync.Mutex
	wrtr    io.WriteCloser // nil once the fill has been committed or abandoned
	buf     *bufio.Writer
	written int64

	upstreamCloseOnce sync.Once
	upstreamCloseErr  error
}

func newCacheFillingReader(upstream io.ReadCloser, cache *archiveBucketCache, pth string, wrtr io.WriteCloser, L *log.Entry) *cacheFillingReader {
	return &cacheFillingReader{
		upstream: upstream,
		cache:    cache,
		path:     pth,
		log:      L,
		wrtr:     wrtr,
		buf:      bufio.NewWriterSize(&flushCounter{w: wrtr, usage: &cache.usage}, cacheFillBufferSize),
	}
}

func (r *cacheFillingReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.read(p)
}

func (r *cacheFillingReader) read(p []byte) (int, error) {
	n, err := r.upstream.Read(p)
	if n > 0 {
		r.fill(p[:n])
	}
	if err == io.EOF {
		r.commit()
	} else if err != nil {
		r.abandon(err)
	}
	return n, err
}

func (r *cacheFillingReader) fill(b []byte) {
	if r.wrtr == nil {
		return
	}
	if r.written+int64(len(b)) > r.cache.maxFileSize {
		r.abandon(errCacheFileTooLarge)
		return
	}
	if !r.cache.usage.reserve(r.cache.path, int64(len(b)), r.cache.maxTotalSize) {
		r.abandon(errCacheBudgetSpent)
		return
	}

	w, err := r.buf.Write(b)
	r.written += int64(w)
	if w < len(b) {
		r.cache.usage.discard(int64(len(b) - w))
	}
	if err != nil {
		r.abandon(err)
	}
}

func (r *cacheFillingReader) commit() {
	if r.wrtr == nil {
		return
	}
	wrtr := r.wrtr
	r.wrtr = nil

	flushErr := r.buf.Flush()
	if flushErr != nil {
		// The buffer keeps what it could not write; those bytes never reach disk.
		r.cache.usage.discard(int64(r.buf.Buffered()))
	}
	closeErr, removeErr := r.cache.finishFill(r.path, wrtr, flushErr == nil)
	if flushErr != nil || closeErr != nil {
		r.log.WithFields(log.Fields{
			"flush":    flushErr,
			"wr-close": closeErr,
			"cache-rm": removeErr,
		}).Warn("Committing cached file failed")
		return
	}
	r.log.Infof("Cached %dKiB file", r.written/1024)

	// Replayed into the bandwidth-saved stat when this path is later a hit.
	r.cache.sizes.Store(r.path, r.written)
}

func (r *cacheFillingReader) abandon(reason error) {
	if r.wrtr == nil {
		return
	}
	wrtr := r.wrtr
	r.wrtr = nil
	r.cache.usage.discard(int64(r.buf.Buffered()))

	closeErr, removeErr := r.cache.finishFill(r.path, wrtr, false)
	r.log.WithError(reason).WithFields(log.Fields{
		"wr-close": closeErr,
		"cache-rm": removeErr,
	}).Warn("Stopped caching file")
}

// Close can be called more than once, and from a goroutine other than the
// one that reads. A fill that is still active when the caller stops is
// drained a bounded amount further so a small file is still cached whole;
// past that bound it is abandoned.
func (r *cacheFillingReader) Close() error {
	if !r.mu.TryLock() {
		// A Read is blocked on the upstream. Closing the upstream is what
		// unblocks it; that Read then returns an error and abandons the fill.
		r.closeUpstream()
		r.mu.Lock()
	} else if r.wrtr != nil {
		r.drainOnClose()
	}
	defer r.mu.Unlock()

	r.abandon(errors.New("reader closed before EOF"))
	return r.closeUpstream()
}

func (r *cacheFillingReader) drainOnClose() {
	// Closing the upstream is what unblocks a stalled read, so a timer does
	// exactly that if the drain outlives cacheDrainOnCloseTimeout.
	timer := time.AfterFunc(cacheDrainOnCloseTimeout, func() { r.closeUpstream() })
	defer timer.Stop()

	buf := make([]byte, 32<<10)
	for remaining := cacheDrainOnCloseLimit; remaining > 0 && r.wrtr != nil; {
		n, err := r.read(buf[:min(len(buf), remaining)])
		remaining -= n
		if err != nil {
			return
		}
	}
}

func (r *cacheFillingReader) closeUpstream() error {
	r.upstreamCloseOnce.Do(func() {
		r.upstreamCloseErr = r.upstream.Close()
	})
	return r.upstreamCloseErr
}

// closeOnceReader makes Close safe to call more than once; the fscache reader
// underneath panics on a second Close.
type closeOnceReader struct {
	io.ReadCloser
	once     sync.Once
	closeErr error
}

func (c *closeOnceReader) Close() error {
	c.once.Do(func() {
		c.closeErr = c.ReadCloser.Close()
	})
	return c.closeErr
}

func (arch *Archive) GetStats() []ArchiveStats {
	return []ArchiveStats{&arch.stats}
}

func (arch *Archive) GetCheckpointManager() CheckpointManager {
	return arch.checkpointManager
}

func (a *Archive) GetPathHAS(path string) (HistoryArchiveState, error) {
	var has HistoryArchiveState
	rdr, err := a.backend.GetFile(path)
	// this is a query on the HA server state, not a data/bucket file download
	a.stats.incrementRequests()
	if err != nil {
		return has, err
	}
	defer rdr.Close()
	lr := &io.LimitedReader{R: rdr, N: maxHASSize + 1}
	dec := json.NewDecoder(lr)
	err = dec.Decode(&has)
	if err != nil {
		if lr.N == 0 && (err == io.EOF || err == io.ErrUnexpectedEOF) {
			return has, errors.Errorf("history archive state response exceeds %d bytes limit", maxHASSize)
		}
		return has, err
	}

	// Compare network passphrase only when non empty. The field was added in
	// Stellar-Core 14.1.0.
	if has.NetworkPassphrase != "" && a.networkPassphrase != "" &&
		has.NetworkPassphrase != a.networkPassphrase {
		return has, errors.Errorf(
			"Network passphrase does not match! expected=%s actual=%s",
			a.networkPassphrase,
			has.NetworkPassphrase,
		)
	}

	return has, nil
}

func (a *Archive) PutPathHAS(path string, has HistoryArchiveState, opts *CommandOptions) error {
	exists, err := a.backend.Exists(path)
	a.stats.incrementRequests()
	if err != nil {
		return err
	}
	if exists && !opts.Force {
		log.Printf("skipping existing %s", path)
		return nil
	}
	buf, err := json.MarshalIndent(has, "", "    ")
	if err != nil {
		return err
	}
	a.stats.incrementUploads()
	return a.backend.PutFile(path, io.NopCloser(bytes.NewReader(buf)))
}

func (a *Archive) GetLatestLedgerSequence() (uint32, error) {
	has, err := a.GetRootHAS()
	if err != nil {
		log.Error("Error getting root HAS from archive", err)
		return 0, errors.Wrap(err, "failed to retrieve the latest ledger sequence from history archive")
	}

	return has.CurrentLedger, nil
}

func (a *Archive) BucketExists(bucket Hash) (bool, error) {
	return a.cachedExists(BucketPath(bucket))
}

func (a *Archive) BucketSize(bucket Hash) (int64, error) {
	a.stats.incrementRequests()
	return a.backend.Size(BucketPath(bucket))
}

func (a *Archive) CategoryCheckpointExists(cat string, chk uint32) (bool, error) {
	a.stats.incrementRequests()
	return a.backend.Exists(CategoryCheckpointPath(cat, chk))
}

func (a *Archive) GetLedgerHeader(ledger uint32) (xdr.LedgerHeaderHistoryEntry, error) {
	checkpoint := ledger
	if !a.checkpointManager.IsCheckpoint(checkpoint) {
		checkpoint = a.checkpointManager.NextCheckpoint(ledger)
	}
	path := CategoryCheckpointPath("ledger", checkpoint)
	xdrStream, err := a.GetXdrStream(path)
	if err != nil {
		return xdr.LedgerHeaderHistoryEntry{}, errors.Wrap(err, "error opening ledger stream")
	}
	defer xdrStream.Close()

	for {
		var ledgerHeader xdr.LedgerHeaderHistoryEntry
		err = xdrStream.ReadOne(&ledgerHeader)
		if err != nil {
			if err == io.EOF {
				break
			}
			return ledgerHeader, errors.Wrap(err, "error reading from ledger stream")
		}

		if uint32(ledgerHeader.Header.LedgerSeq) == ledger {
			return ledgerHeader, nil
		}
	}

	return xdr.LedgerHeaderHistoryEntry{}, errors.New("ledger header not found in checkpoint")
}

func (a *Archive) GetRootHAS() (HistoryArchiveState, error) {
	return a.GetPathHAS(rootHASPath)
}

func (a *Archive) GetLedgers(start, end uint32) (map[uint32]*Ledger, error) {
	if start > end {
		return nil, errors.Errorf("range is invalid, start: %d end: %d", start, end)
	}
	startCheckpoint := a.GetCheckpointManager().GetCheckpoint(start)
	endCheckpoint := a.GetCheckpointManager().GetCheckpoint(end)
	cache := map[uint32]*Ledger{}
	for cur := startCheckpoint; cur <= endCheckpoint; cur += a.GetCheckpointManager().GetCheckpointFrequency() {
		for _, category := range []string{"ledger", "transactions", "results"} {
			if exists, err := a.CategoryCheckpointExists(category, cur); err != nil {
				return nil, errors.Wrap(err, "could not check if category checkpoint exists")
			} else if !exists {
				return nil, errors.Errorf("checkpoint %d is not published", cur)
			}

			if err := a.fetchCategory(cache, category, cur); err != nil {
				return nil, errors.Wrap(err, "could not fetch category checkpoint")
			}
		}
	}

	return cache, nil
}

func (a *Archive) fetchCategory(cache map[uint32]*Ledger, category string, checkpointSequence uint32) error {
	checkpointPath := CategoryCheckpointPath(category, checkpointSequence)
	xdrStream, err := a.GetXdrStream(checkpointPath)
	if err != nil {
		return errors.Wrapf(err, "error opening %s stream", category)
	}
	defer xdrStream.Close()

	for {
		switch category {
		case "ledger":
			var object xdr.LedgerHeaderHistoryEntry
			if err = xdrStream.ReadOne(&object); err == nil {
				entry := cache[uint32(object.Header.LedgerSeq)]
				if entry == nil {
					entry = &Ledger{}
				}
				entry.Header = object
				cache[uint32(object.Header.LedgerSeq)] = entry
			}
		case "transactions":
			var object xdr.TransactionHistoryEntry
			if err = xdrStream.ReadOne(&object); err == nil {
				entry := cache[uint32(object.LedgerSeq)]
				if entry == nil {
					entry = &Ledger{}
				}
				entry.Transaction = object
				cache[uint32(object.LedgerSeq)] = entry
			}
		case "results":
			var object xdr.TransactionHistoryResultEntry
			if err = xdrStream.ReadOne(&object); err == nil {
				entry := cache[uint32(object.LedgerSeq)]
				if entry == nil {
					entry = &Ledger{}
				}
				entry.TransactionResult = object
				cache[uint32(object.LedgerSeq)] = entry
			}
		default:
			panic("unknown category")
		}

		if err == io.EOF {
			break
		} else if err != nil {
			return errors.Wrapf(err, "error reading from %s stream", category)
		}
	}

	return nil
}

func (a *Archive) GetCheckpointHAS(chk uint32) (HistoryArchiveState, error) {
	return a.GetPathHAS(CategoryCheckpointPath("history", chk))
}

func (a *Archive) PutCheckpointHAS(chk uint32, has HistoryArchiveState, opts *CommandOptions) error {
	return a.PutPathHAS(CategoryCheckpointPath("history", chk), has, opts)
}

func (a *Archive) PutRootHAS(has HistoryArchiveState, opts *CommandOptions) error {
	force := opts.Force
	opts.Force = true
	e := a.PutPathHAS(rootHASPath, has, opts)
	opts.Force = force
	return e
}

func (a *Archive) ListBucket(dp DirPrefix) (chan string, chan error) {
	a.stats.incrementRequests()
	return a.backend.ListFiles(path.Join("bucket", dp.Path()))
}

func (a *Archive) ListAllBuckets() (chan string, chan error) {
	a.stats.incrementRequests()
	return a.backend.ListFiles("bucket")
}

func (a *Archive) ListAllBucketHashes() (chan Hash, chan error) {
	a.stats.incrementRequests()
	sch, errs := a.backend.ListFiles("bucket")
	ch := make(chan Hash)
	rx := regexp.MustCompile("bucket" + hexPrefixPat + "bucket-([0-9a-f]{64})\\.xdr\\.gz$")
	errs = makeErrorPump(errs)
	go func() {
		for s := range sch {
			m := rx.FindStringSubmatch(s)
			if m != nil {
				ch <- MustDecodeHash(m[1])
			}
		}
		close(ch)
	}()
	return ch, errs
}

func (a *Archive) ListCategoryCheckpoints(cat string, pth string) (chan uint32, chan error) {
	ext := categoryExt(cat)
	rx := regexp.MustCompile(cat + hexPrefixPat + cat +
		"-([0-9a-f]{8})\\." + regexp.QuoteMeta(ext) + "$")
	a.stats.incrementRequests()
	sch, errs := a.backend.ListFiles(path.Join(cat, pth))
	ch := make(chan uint32)
	errs = makeErrorPump(errs)

	go func() {
		for s := range sch {
			m := rx.FindStringSubmatch(s)
			if m != nil {
				i, e := strconv.ParseUint(m[1], 16, 32)
				if e == nil {
					ch <- uint32(i)
				} else {
					errs <- errors.New("decoding checkpoint number in filename " + s)
				}
			}
		}
		close(ch)
	}()
	return ch, errs
}

func (a *Archive) GetBucketPathForHash(hash Hash) string {
	return fmt.Sprintf(
		"bucket/%s/bucket-%s.xdr.gz",
		HashPrefix(hash).Path(),
		hash.String(),
	)
}

func (a *Archive) GetXdrStreamForHash(hash Hash) (*xdr.Stream, error) {
	return a.GetXdrStream(a.GetBucketPathForHash(hash))
}

func (a *Archive) GetXdrStream(pth string) (*xdr.Stream, error) {
	if !strings.HasSuffix(pth, ".xdr.gz") {
		return nil, errors.New("File has non-.xdr.gz suffix: " + pth)
	}
	rdr, err := a.cachedGet(pth)
	if err != nil {
		return nil, err
	}
	return xdr.NewGzStream(rdr)
}

func (a *Archive) cachedGet(pth string) (io.ReadCloser, error) {
	if a.cache == nil {
		a.stats.incrementDownloads()
		return a.backend.GetFile(pth)
	}

	L := log.WithField("path", pth).WithField("cache", a.cache.path)

	rdr, wrtr, inFlight, err := a.cache.acquire(pth)
	if err != nil {
		L.WithError(err).
			WithField("remove", a.cache.Remove(pth)).
			Warn("On-disk cache retrieval failed")
		a.stats.incrementDownloads()
		return a.backend.GetFile(pth)
	}

	// Another goroutine is filling this entry — download independently
	// rather than read a file that may be abandoned mid-write (see acquire).
	if inFlight {
		a.stats.incrementDownloads()
		return a.backend.GetFile(pth)
	}

	if wrtr != nil {
		L.Info("Caching file...")
		a.stats.incrementDownloads()
		upstream, err := a.backend.GetFile(pth)
		if err != nil {
			closeErr, removeErr := a.cache.finishFill(pth, wrtr, false)
			L.WithError(err).WithFields(log.Fields{
				"write-close": closeErr,
				"cache-rm":    removeErr,
			}).Warn("Download failed, purging from cache")
			return nil, err
		}

		return newCacheFillingReader(upstream, a.cache, pth, wrtr, L), nil
	}

	// Best-effort check to track bandwidth metrics
	if written, found := a.cache.sizes.Load(pth); found {
		a.stats.incrementCacheBandwidth(written.(int64))
	}
	a.stats.incrementCacheHits()

	return &closeOnceReader{ReadCloser: rdr}, nil
}

func (a *Archive) cachedExists(pth string) (bool, error) {
	if a.cache != nil && a.cache.hasCompleted(pth) {
		return true, nil
	}

	a.stats.incrementRequests()
	return a.backend.Exists(pth)
}

// Connect opens the archive at u and, when opts.CachePath is set, gives it
// its own on-disk cache. NewArchivePool uses connect instead and attaches one
// cache shared by every member.
func Connect(u string, opts ArchiveOptions) (*Archive, error) {
	arch, err := connect(u, opts)
	if err != nil || opts.CachePath == "" {
		return arch, err
	}
	arch.cache, err = newArchiveBucketCache(opts)
	return arch, err
}

func connect(u string, opts ArchiveOptions) (*Archive, error) {
	arch := Archive{
		networkPassphrase:       opts.NetworkPassphrase,
		checkpointFiles:         make(map[string](map[uint32]bool)),
		allBuckets:              make(map[Hash]bool),
		referencedBuckets:       make(map[Hash]bool),
		expectLedgerHashes:      make(map[uint32]Hash),
		actualLedgerHashes:      make(map[uint32]Hash),
		expectTxSetHashes:       make(map[uint32]Hash),
		actualTxSetHashes:       make(map[uint32]Hash),
		expectTxResultSetHashes: make(map[uint32]Hash),
		actualTxResultSetHashes: make(map[uint32]Hash),
		checkpointManager:       NewCheckpointManager(opts.CheckpointFrequency),
	}
	for _, cat := range Categories() {
		arch.checkpointFiles[cat] = make(map[uint32]bool)
	}

	if opts.ConnectOptions.Context == nil {
		opts.ConnectOptions.Context = context.Background()
	}

	var err error
	arch.backend, err = ConnectBackend(u, opts.ConnectOptions)
	if err != nil {
		return &arch, err
	}

	arch.stats = archiveStats{backendName: u}
	return &arch, nil
}

func newArchiveBucketCache(opts ArchiveOptions) (*archiveBucketCache, error) {
	// Set up an LRU cache for history archive files
	haunter := fscache.NewLRUHaunterStrategy(
		fscache.NewLRUHaunter(0, cacheEvictionTarget, time.Minute /* frequency check */),
	)

	maxFileSize := opts.MaxCacheFileSize
	if maxFileSize <= 0 {
		maxFileSize = defaultMaxCacheFileSize
	}

	// The cache is rebuilt from scratch on every start: nothing records
	// which files on disk were completed, so a leftover directory could
	// only be trusted after re-reading every file.
	if err := os.RemoveAll(opts.CachePath); err != nil {
		return nil, errors.Wrapf(err, "clearing cache at '%s' failed", opts.CachePath)
	}
	fs, err := fscache.NewFs(opts.CachePath, 0755 /* drwxr-xr-x */)

	if err != nil {
		return nil, errors.Wrapf(err,
			"creating cache at '%s' with mode 0755 failed",
			opts.CachePath)
	}

	cache, err := fscache.NewCacheWithHaunter(fs, haunter)
	if err != nil {
		return nil, errors.Wrapf(err,
			"creating cache at '%s' failed",
			opts.CachePath)
	}

	return &archiveBucketCache{
		Cache:        cache,
		path:         opts.CachePath,
		maxTotalSize: cacheTotalSizeBudget,
		maxFileSize:  maxFileSize,
		filling:      set.NewSet[string](0),
	}, nil
}

func ConnectBackend(u string, opts storage.ConnectOptions) (storage.Storage, error) {
	if u == "" {
		return nil, errors.New("URL is empty")
	}

	var err error
	parsed, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	var backend storage.Storage

	if parsed.Scheme == "mock" {
		backend = makeMockBackend()
	} else if parsed.Scheme == "fmock" {
		backend = makeFailingMockBackend()
	} else {
		backend, err = storage.ConnectBackend(u, opts)
	}

	return backend, err
}

func MustConnect(u string, opts ArchiveOptions) *Archive {
	arch, err := Connect(u, opts)
	if err != nil {
		log.Fatal(err)
	}
	return arch
}
