// Copyright 2016 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package historyarchive

import (
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
	"sync/atomic"
	"time"

	fscache "github.com/djherbis/fscache"
	log "github.com/sirupsen/logrus"

	"github.com/stellar/go-stellar-sdk/support/errors"
	supportlog "github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/support/storage"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const hexPrefixPat = "/[0-9a-f]{2}/[0-9a-f]{2}/[0-9a-f]{2}/"
const rootHASPath = ".well-known/stellar-history.json"
const maxHASSize = 10 * 1024 * 1024 // 10 MB

const (
	// cacheTotalSizeBudget is the hard ceiling on the cache directory's size.
	// A download that would push the directory past it stops being written to
	// the cache; the caller still receives the whole file.
	cacheTotalSizeBudget = 10 << 30 // 10 GiB

	// cacheEvictionTarget is the size the once-a-minute LRU eviction pass
	// trims the cache directory down to. It sits well below
	// cacheTotalSizeBudget on purpose: the eviction pass measures the
	// directory differently than the budget check does (it skips files that
	// are open, and its own metadata files), so if the two limits shared one
	// number, a cache sitting at the budget could look small enough to the
	// eviction pass that it never evicts anything — and then no download
	// would ever be cached again.
	cacheEvictionTarget = 8 << 30 // 8 GiB

	// The biggest bucket the pubnet archive serves today is around 2.2GiB, so
	// this leaves plenty of room for state growth while staying below
	// cacheTotalSizeBudget, which remains the binding limit.
	defaultMaxCacheFileSize = 8 << 30 // 8 GiB

	// How many bytes a fill writes to the cache between two checks of
	// cacheTotalSizeBudget.
	cacheCopyChunkSize = 4 << 20 // 4 MiB

	cacheUsageScanInterval = time.Second

	// How long the probe read in cacheFillingReader.Close may block before
	// the upstream is closed under it.
	cacheCommitProbeTimeout = time.Second
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
	// served in full; it just isn't cached. Zero selects
	// defaultMaxCacheFileSize.
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

	// filling holds the paths whose cache entries are being written right
	// now. Only completed entries are ever served from the cache; see acquire.
	fillMu  sync.Mutex
	filling map[string]struct{}
}

// acquire routes a request for pth to one of three outcomes: a reader over a
// fully cached file (rdr != nil), a writer the caller must fill with the
// upstream's bytes (wrtr != nil), or inFlight=true, meaning another goroutine
// is filling pth right now and this request should bypass the cache and
// download directly.
//
// The bypass is what keeps concurrent readers safe. The cache library would
// happily hand out a reader over the half-written file, but if the fill is
// then abandoned (size limits, upstream error, the filling caller closing
// early), that reader sees the file end with a clean EOF — silently truncated
// data, with no error. Serving only completed files makes an abandoned fill
// invisible to everyone but the caller that started it.
func (c *archiveBucketCache) acquire(pth string) (rdr io.ReadCloser, wrtr io.WriteCloser, inFlight bool, err error) {
	c.fillMu.Lock()
	defer c.fillMu.Unlock()

	if _, filling := c.filling[pth]; filling {
		return nil, nil, true, nil
	}

	rdr, wrtr, err = c.Get(pth)
	if err != nil {
		return nil, nil, false, err
	}
	if wrtr == nil {
		return rdr, nil, false, nil
	}

	// A new entry: the caller fills it through wrtr while streaming the
	// upstream to its own consumer. The reader half is unused — nothing reads
	// the cache file until the fill completes.
	rdr.Close()
	c.filling[pth] = struct{}{}
	return nil, wrtr, false, nil
}

// finishFill closes the writer of a fill started by acquire and, unless the
// fill completed cleanly, deletes the cache entry. Only after both steps does
// pth leave the in-flight set, so no other caller can observe the entry
// half-written or mid-removal.
func (c *archiveBucketCache) finishFill(pth string, wrtr io.WriteCloser, complete bool) (closeErr, removeErr error) {
	closeErr = wrtr.Close()
	if !complete || closeErr != nil {
		removeErr = c.Remove(pth)
	}

	c.fillMu.Lock()
	delete(c.filling, pth)
	c.fillMu.Unlock()
	return closeErr, removeErr
}

func (c *archiveBucketCache) hasCompleted(pth string) bool {
	c.fillMu.Lock()
	_, filling := c.filling[pth]
	c.fillMu.Unlock()
	return !filling && c.Exists(pth)
}

// cacheUsage answers "how many bytes does the cache directory hold right now?".
//
// The underlying cache library keeps its own running total, but it leaves out
// any file that is still open, which is exactly what a download in progress is.
// So the number is measured here instead, by adding up the files in the cache
// directory.
//
// Measuring means a stat per file, which is too slow to repeat on every write,
// so the directory is only re-measured once every cacheUsageScanInterval, by
// whichever caller of total() finds the last measurement expired — and that
// caller walks the directory without holding a lock, so concurrent downloads
// are never stalled behind the walk. In between measurements, the copies that
// are running hand the bytes they write to add(), and those are added to the
// last measurement.
type cacheUsage struct {
	scanned atomic.Int64
	written atomic.Int64

	mu       sync.Mutex
	scanning bool
	at       time.Time
}

func (u *cacheUsage) total(dir string) int64 {
	u.mu.Lock()
	rescan := !u.scanning && time.Since(u.at) >= cacheUsageScanInterval
	if rescan {
		u.scanning = true
	}
	u.mu.Unlock()

	if rescan {
		n := dirSize(dir)
		u.mu.Lock()
		u.scanned.Store(n)
		u.written.Store(0)
		u.at = time.Now()
		u.scanning = false
		u.mu.Unlock()
	}

	return u.scanned.Load() + u.written.Load()
}

func (u *cacheUsage) add(n int64) {
	u.written.Add(n)
}

func dirSize(dir string) int64 {
	var total int64
	filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// cacheFillingReader streams a file from the upstream archive to the caller
// and, as a side effect, writes the same bytes into a cache entry. The caller
// always receives exactly what the upstream sent: when caching has to stop —
// the file outgrows the per-file limit, the cache directory hits its budget,
// or a cache write fails — the half-written entry is deleted and this reader
// keeps streaming; it just stops caching.
//
// A fill is committed once the upstream reports EOF — either during a Read,
// or by the probe read in Close for callers that consume the whole file
// without ever seeing the EOF themselves. A caller that closes mid-file gets
// its incomplete entry deleted instead.
type cacheFillingReader struct {
	upstream io.ReadCloser
	cache    *archiveBucketCache
	pth      string
	log      *log.Entry

	wrtr      io.WriteCloser // nil once the fill has been committed or abandoned
	written   int64
	lastCheck int64

	closeOnce sync.Once
	closeErr  error

	upstreamCloseOnce sync.Once
	upstreamCloseErr  error
}

func (r *cacheFillingReader) Read(p []byte) (int, error) {
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
	if r.cache.maxFileSize > 0 && r.written+int64(len(b)) > r.cache.maxFileSize {
		r.abandon(errCacheFileTooLarge)
		return
	}
	if r.written == 0 || r.written-r.lastCheck >= cacheCopyChunkSize {
		r.lastCheck = r.written
		if r.cache.maxTotalSize > 0 && r.cache.usage.total(r.cache.path) >= r.cache.maxTotalSize {
			r.abandon(errCacheBudgetSpent)
			return
		}
	}

	w, err := r.wrtr.Write(b)
	r.written += int64(w)
	r.cache.usage.add(int64(w))
	if err != nil {
		r.abandon(err)
	} else if w < len(b) {
		r.abandon(io.ErrShortWrite)
	}
}

func (r *cacheFillingReader) commit() {
	if r.wrtr == nil {
		return
	}
	wrtr := r.wrtr
	r.wrtr = nil

	closeErr, removeErr := r.cache.finishFill(r.pth, wrtr, true)
	if closeErr != nil {
		r.log.WithError(closeErr).WithField("cache-rm", removeErr).
			Warn("Committing cached file failed")
		return
	}
	r.log.Infof("Cached %dKiB file", r.written/1024)

	// Track how much bandwidth we've saved from caching by saving
	// the size of the file we just downloaded.
	r.cache.sizes.Store(r.pth, r.written)
}

func (r *cacheFillingReader) abandon(reason error) {
	if r.wrtr == nil {
		return
	}
	wrtr := r.wrtr
	r.wrtr = nil

	closeErr, removeErr := r.cache.finishFill(r.pth, wrtr, false)
	r.log.WithError(reason).WithFields(log.Fields{
		"wr-close": closeErr,
		"cache-rm": removeErr,
	}).Warn("Stopped caching file")
}

// Close can be called more than once. If the fill is still running, one probe
// read decides its fate: gzip consumers stop reading right after the gzip
// trailer, without the extra Read that would report io.EOF, so a Close does
// not necessarily mean the download is incomplete. An immediate EOF from the
// probe means the whole file was consumed, and the fill is committed;
// anything else discards it.
func (r *cacheFillingReader) Close() error {
	r.closeOnce.Do(func() {
		if r.wrtr != nil {
			r.settleFillOnClose()
		}
		r.closeErr = r.closeUpstream()
	})
	return r.closeErr
}

func (r *cacheFillingReader) settleFillOnClose() {
	// The probe must not block forever on a stalled connection: closing the
	// upstream is what unblocks a pending read, so a timer does exactly that
	// if the probe outlives cacheCommitProbeTimeout.
	timer := time.AfterFunc(cacheCommitProbeTimeout, func() { r.closeUpstream() })
	defer timer.Stop()

	var buf [1]byte
	n, err := r.upstream.Read(buf[:])
	if n == 0 && err == io.EOF {
		r.commit()
		return
	}
	r.abandon(errors.New("reader closed before EOF"))
}

func (r *cacheFillingReader) closeUpstream() error {
	r.upstreamCloseOnce.Do(func() {
		r.upstreamCloseErr = r.upstream.Close()
	})
	return r.upstreamCloseErr
}

// closeOnceReader makes Close safe to call more than once. The cache reader
// underneath panics on a second Close, because its handle count goes negative,
// and a reader handed out to callers has no business being that fragile.
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

	// Another goroutine is filling the cache entry for pth right now. Only
	// completed entries are ever served from the cache (see acquire), so this
	// request downloads its own copy instead of waiting.
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

		return &cacheFillingReader{
			upstream: upstream,
			cache:    a.cache,
			pth:      pth,
			log:      L,
			wrtr:     wrtr,
		}, nil
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

func Connect(u string, opts ArchiveOptions) (*Archive, error) {
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

	if opts.CachePath != "" {
		arch.cache, err = newArchiveBucketCache(opts)
		if err != nil {
			return &arch, err
		}
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
	if maxFileSize == 0 {
		maxFileSize = defaultMaxCacheFileSize
	}

	// Wipe any existing cache on startup
	os.RemoveAll(opts.CachePath)
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
		filling:      make(map[string]struct{}),
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
