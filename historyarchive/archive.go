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
	"regexp"
	"strconv"
	"strings"
	"sync"
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
	dec := json.NewDecoder(rdr)
	err = dec.Decode(&has)
	if err != nil {
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

	rdr, wrtr, err := a.cache.Get(pth)
	if err != nil {
		L.WithError(err).
			WithField("remove", a.cache.Remove(pth)).
			Warn("On-disk cache retrieval failed")
		a.stats.incrementDownloads()
		return a.backend.GetFile(pth)
	}

	// If a NEW key is being retrieved, it returns a writer to which
	// you're expected to write your upstream as well as a reader that
	// will read directly from it.
	if wrtr != nil {
		log.WithField("path", pth).Info("Caching file...")
		a.stats.incrementDownloads()
		upstreamReader, err := a.backend.GetFile(pth)
		if err != nil {
			writeErr := wrtr.Close()
			readErr := rdr.Close()
			removeErr := a.cache.Remove(pth)
			// Execution order isn't guaranteed w/in a function call expression
			// so we close them with explicit order first.
			L.WithError(err).WithFields(log.Fields{
				"write-close": writeErr,
				"read-close":  readErr,
				"cache-rm":    removeErr,
			}).Warn("Download failed, purging from cache")
			return nil, err
		}

		// Start a goroutine to slurp up the upstream and feed
		// it directly to the cache.
		go func() {
			written, err := io.Copy(wrtr, upstreamReader)
			writeErr := wrtr.Close()
			readErr := upstreamReader.Close()
			fields := log.Fields{
				"wr-close": writeErr,
				"rd-close": readErr,
			}

			if err != nil {
				L.WithFields(fields).WithError(err).
					Warn("Failed to download and cache file")

				// Removal must happen *after* handles close.
				if removalErr := a.cache.Remove(pth); removalErr != nil {
					L.WithError(removalErr).Warn("Removing cached file failed")
				}
			} else {
				L.WithFields(fields).Infof("Cached %dKiB file", written/1024)

				// Track how much bandwidth we've saved from caching by saving
				// the size of the file we just downloaded.
				a.cache.sizes.Store(pth, written)
			}
		}()
	} else {
		// Best-effort check to track bandwidth metrics
		if written, found := a.cache.sizes.Load(pth); found {
			a.stats.incrementCacheBandwidth(written.(int64))
		}
		a.stats.incrementCacheHits()
	}

	return rdr, nil
}

func (a *Archive) cachedExists(pth string) (bool, error) {
	if a.cache != nil && a.cache.Exists(pth) {
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
		// Set up a <= ~10GiB LRU cache for history archives files
		haunter := fscache.NewLRUHaunterStrategy(
			fscache.NewLRUHaunter(0, 10<<30, time.Minute /* frequency check */),
		)

		// Wipe any existing cache on startup
		os.RemoveAll(opts.CachePath)
		fs, err := fscache.NewFs(opts.CachePath, 0755 /* drwxr-xr-x */)

		if err != nil {
			return &arch, errors.Wrapf(err,
				"creating cache at '%s' with mode 0755 failed",
				opts.CachePath)
		}

		cache, err := fscache.NewCacheWithHaunter(fs, haunter)
		if err != nil {
			return &arch, errors.Wrapf(err,
				"creating cache at '%s' failed",
				opts.CachePath)
		}

		arch.cache = &archiveBucketCache{cache, opts.CachePath, sync.Map{}}
	}

	arch.stats = archiveStats{backendName: u}
	return &arch, nil
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
