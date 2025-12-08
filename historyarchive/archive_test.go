// Copyright 2016 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package historyarchive

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stellar/go-stellar-sdk/support/storage"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var cachePath = filepath.Join(os.TempDir(), "history-archive-test-cache")

func GetTestS3Archive() *Archive {
	mx := big.NewInt(0xffffffff)
	r, e := rand.Int(rand.Reader, mx)
	if e != nil {
		panic(e)
	}
	bucket := fmt.Sprintf("s3://history-stg.stellar.org/dev/archivist/test-%s", r)
	region := "eu-west-1"
	if env_bucket := os.Getenv("ARCHIVIST_TEST_S3_BUCKET"); env_bucket != "" {
		bucket = fmt.Sprintf(env_bucket+"/archivist/test-%s", r)
	}
	if env_region := os.Getenv("ARCHIVIST_TEST_S3_REGION"); env_region != "" {
		region = env_region
	}
	return MustConnect(
		bucket,
		ArchiveOptions{
			CheckpointFrequency: DefaultCheckpointFrequency,
			ConnectOptions:      storage.ConnectOptions{S3Region: region},
		},
	)
}

func GetTestMockArchive() *Archive {
	return MustConnect("mock://test", ArchiveOptions{
		CheckpointFrequency: 64,
		CachePath:           cachePath,
	})
}

var tmpdirs []string

func GetTestFileArchive() *Archive {
	d, e := ioutil.TempDir("/tmp", "archivist")
	if e != nil {
		panic(e)
	}
	if tmpdirs == nil {
		tmpdirs = []string{d}
	} else {
		tmpdirs = append(tmpdirs, d)
	}
	return MustConnect("file://"+d, ArchiveOptions{CheckpointFrequency: DefaultCheckpointFrequency})
}

func cleanup() {
	for _, d := range tmpdirs {
		os.RemoveAll(d)
	}
}

func GetTestArchive() *Archive {
	ty := os.Getenv("ARCHIVIST_TEST_TYPE")
	if ty == "file" {
		return GetTestFileArchive()
	} else if ty == "s3" {
		return GetTestS3Archive()
	} else {
		return GetTestMockArchive()
	}
}

func (arch *Archive) AddRandomBucket() (Hash, error) {
	var h Hash
	buf := make([]byte, 1024)
	_, e := rand.Read(buf)
	if e != nil {
		return h, e
	}
	h = sha256.Sum256(buf)
	pth := BucketPath(h)
	e = arch.backend.PutFile(pth, ioutil.NopCloser(bytes.NewReader(buf)))
	return h, e
}

func (arch *Archive) AddRandomCheckpointFile(cat string, chk uint32) error {
	buf := make([]byte, 1024)
	_, e := rand.Read(buf)
	if e != nil {
		return e
	}
	pth := CategoryCheckpointPath(cat, chk)
	return arch.backend.PutFile(pth, ioutil.NopCloser(bytes.NewReader(buf)))
}

func (arch *Archive) AddRandomCheckpoint(chk uint32) error {
	opts := &CommandOptions{Force: true}
	for _, cat := range Categories() {
		if cat == "history" {
			var has HistoryArchiveState
			has.CurrentLedger = chk
			for i := 0; i < NumLevels; i++ {
				curr, e := arch.AddRandomBucket()
				if e != nil {
					return e
				}
				snap, e := arch.AddRandomBucket()
				if e != nil {
					return e
				}
				next, e := arch.AddRandomBucket()
				if e != nil {
					return e
				}
				has.CurrentBuckets[i].Curr = curr.String()
				has.CurrentBuckets[i].Snap = snap.String()
				has.CurrentBuckets[i].Next.Output = next.String()
			}
			arch.PutCheckpointHAS(chk, has, opts)
			arch.PutRootHAS(has, opts)
		} else {
			arch.AddRandomCheckpointFile(cat, chk)
		}
	}
	return nil
}

func (arch *Archive) PopulateRandomRange(rng Range) error {
	for chk := range rng.GenerateCheckpoints(arch.checkpointManager) {
		if e := arch.AddRandomCheckpoint(chk); e != nil {
			return e
		}
	}
	return nil
}

func (arch *Archive) PopulateRandomRangeWithGap(rng Range, gap uint32) error {
	for chk := range rng.GenerateCheckpoints(arch.checkpointManager) {
		if chk == gap {
			continue
		}
		if e := arch.AddRandomCheckpoint(chk); e != nil {
			return e
		}
	}
	return nil
}

func testRange() Range {
	return Range{Low: 63, High: 0x3bf}
}

func testOptions() *CommandOptions {
	return &CommandOptions{Range: testRange(), Concurrency: 16}
}

func GetRandomPopulatedArchive() *Archive {
	a := GetTestArchive()
	a.PopulateRandomRange(testRange())
	return a
}

func GetRandomPopulatedArchiveWithGapAt(gap uint32) *Archive {
	a := GetTestArchive()
	a.PopulateRandomRangeWithGap(testRange(), gap)
	return a
}

func TestScan(t *testing.T) {
	defer cleanup()
	opts := testOptions()
	GetRandomPopulatedArchive().Scan(opts)
}

func TestConfiguresHttpUserAgent(t *testing.T) {
	var userAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent = r.Header["User-Agent"][0]
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	archive, err := Connect(server.URL, ArchiveOptions{
		ConnectOptions: storage.ConnectOptions{
			UserAgent: "uatest",
		},
	})
	assert.NoError(t, err)

	ok, err := archive.BucketExists(EmptyXdrArrayHash())
	assert.True(t, ok)
	assert.NoError(t, err)
	assert.Equal(t, userAgent, "uatest")
}

func TestScanSize(t *testing.T) {
	defer cleanup()
	opts := testOptions()
	arch := GetRandomPopulatedArchive()
	arch.Scan(opts)
	assert.Equal(t, opts.Range.SizeInCheckPoints(arch.checkpointManager),
		len(arch.checkpointFiles["history"]))
}

func TestScanSizeSubrange(t *testing.T) {
	defer cleanup()
	opts := testOptions()
	arch := GetRandomPopulatedArchive()
	opts.Range.Low = arch.checkpointManager.NextCheckpoint(opts.Range.Low)
	opts.Range.High = arch.checkpointManager.PrevCheckpoint(opts.Range.High)
	arch.Scan(opts)
	assert.Equal(t, opts.Range.SizeInCheckPoints(arch.checkpointManager),
		len(arch.checkpointFiles["history"]))
}

func TestScanSizeSubrangeFewBuckets(t *testing.T) {
	defer cleanup()
	opts := testOptions()
	arch := GetRandomPopulatedArchive()
	opts.Range.Low = 0x1ff
	opts.Range.High = 0x1ff
	arch.Scan(opts)
	// We should only scan one checkpoint worth of buckets.
	assert.Less(t, len(arch.allBuckets), 40)
}

func TestScanSizeSubrangeAllBuckets(t *testing.T) {
	defer cleanup()
	opts := testOptions()
	arch := GetRandomPopulatedArchive()
	arch.Scan(opts)
	// We should scan all checkpoints worth of buckets.
	assert.Less(t, 300, len(arch.allBuckets))
}

func countMissing(arch *Archive, opts *CommandOptions) int {
	n := 0
	arch.Scan(opts)
	for cat, missing := range arch.CheckCheckpointFilesMissing(opts) {
		if categoryRequired(cat) || !opts.SkipOptional {
			n += len(missing)
		}
	}
	n += len(arch.CheckBucketsMissing())
	return n
}

func TestScanSlowMissing(t *testing.T) {
	defer cleanup()
	opts := testOptions()
	arch := GetRandomPopulatedArchiveWithGapAt(0x1bf)
	arch.ScanCheckpointsSlow(opts)
	n := 0
	for _, missing := range arch.CheckCheckpointFilesMissing(opts) {
		n += len(missing)
	}
	assert.Equal(t, 5, n)
}

func TestMirror(t *testing.T) {
	defer cleanup()
	opts := testOptions()
	src := GetRandomPopulatedArchive()
	dst := GetTestArchive()
	Mirror(src, dst, opts)
	assert.Equal(t, 0, countMissing(dst, opts))
}

func TestOnlyRequired(t *testing.T) {
	defer cleanup()
	optsWithOptional := testOptions()

	optsOnlyRequired := testOptions()
	optsOnlyRequired.SkipOptional = true

	src := GetRandomPopulatedArchive()
	dstWithOptional := GetTestArchive()
	dstOnlyRequired := GetTestArchive()

	// First mirror copies optional files, second does not.
	Mirror(src, dstWithOptional, optsWithOptional)
	Mirror(src, dstOnlyRequired, optsOnlyRequired)

	// Scanning the mirror with optional files under either
	// scan (ignore or require optional) is ok.
	assert.Equal(t, 0, countMissing(dstWithOptional, optsWithOptional))
	assert.Equal(t, 0, countMissing(dstWithOptional, optsOnlyRequired))

	// Scanning the mirror without optional files produces
	// a missing count when requiring optional files.
	assert.NotEqual(t, 0, countMissing(dstOnlyRequired, optsWithOptional))
	assert.Equal(t, 0, countMissing(dstOnlyRequired, optsOnlyRequired))

	// First repair ignores optional files, second fills them in.
	Repair(src, dstOnlyRequired, optsOnlyRequired)
	assert.NotEqual(t, 0, countMissing(dstOnlyRequired, optsWithOptional))
	Repair(src, dstOnlyRequired, optsWithOptional)
	assert.Equal(t, 0, countMissing(dstOnlyRequired, optsWithOptional))
}

func copyFile(category string, checkpoint uint32, src *Archive, dst *Archive) {
	pth := CategoryCheckpointPath(category, checkpoint)
	rdr, err := src.backend.GetFile(pth)
	if err != nil {
		panic(err)
	}
	if err = dst.backend.PutFile(pth, rdr); err != nil {
		panic(err)
	}
}

func TestMirrorThenRepair(t *testing.T) {
	defer cleanup()
	opts := testOptions()
	src := GetRandomPopulatedArchive()
	dst := GetTestArchive()
	Mirror(src, dst, opts)
	assert.Equal(t, 0, countMissing(dst, opts))
	bad := opts.Range.Low + uint32(opts.Range.SizeInCheckPoints(src.checkpointManager)/2)
	src.AddRandomCheckpoint(bad)
	copyFile("history", bad, src, dst)
	assert.NotEqual(t, 0, countMissing(dst, opts))
	Repair(src, dst, opts)
	assert.Equal(t, 0, countMissing(dst, opts))
}

func (a *Archive) MustGetRootHAS() HistoryArchiveState {
	has, e := a.GetRootHAS()
	if e != nil {
		panic("failed to get root HAS")
	}
	return has
}

func TestMirrorSubsetDoPointerUpdate(t *testing.T) {
	defer cleanup()
	opts := testOptions()
	src := GetRandomPopulatedArchive()
	dst := GetTestArchive()
	Mirror(src, dst, opts)
	oldHigh := opts.Range.High
	assert.Equal(t, oldHigh, dst.MustGetRootHAS().CurrentLedger)
	opts.Range.High = src.checkpointManager.NextCheckpoint(oldHigh)
	src.AddRandomCheckpoint(opts.Range.High)
	Mirror(src, dst, opts)
	assert.Equal(t, opts.Range.High, dst.MustGetRootHAS().CurrentLedger)
}

func TestMirrorSubsetNoPointerUpdate(t *testing.T) {
	defer cleanup()
	opts := testOptions()
	src := GetRandomPopulatedArchive()
	dst := GetTestArchive()
	Mirror(src, dst, opts)
	oldHigh := opts.Range.High
	assert.Equal(t, oldHigh, dst.MustGetRootHAS().CurrentLedger)
	src.AddRandomCheckpoint(src.checkpointManager.NextCheckpoint(oldHigh))
	opts.Range.Low = 0x7f
	opts.Range.High = 0xff
	Mirror(src, dst, opts)
	assert.Equal(t, oldHigh, dst.MustGetRootHAS().CurrentLedger)
}

func TestDryRunNoRepair(t *testing.T) {
	defer cleanup()
	opts := testOptions()
	src := GetRandomPopulatedArchive()
	dst := GetTestArchive()
	Mirror(src, dst, opts)
	assert.Equal(t, 0, countMissing(dst, opts))
	bad := opts.Range.Low + uint32(opts.Range.SizeInCheckPoints(src.checkpointManager)/2)
	src.AddRandomCheckpoint(bad)
	copyFile("history", bad, src, dst)
	assert.NotEqual(t, 0, countMissing(dst, opts))
	opts.DryRun = true
	Repair(src, dst, opts)
	assert.NotEqual(t, 0, countMissing(dst, opts))
}

func TestNetworkPassphrase(t *testing.T) {
	makeHASReader := func() io.ReadCloser {
		return ioutil.NopCloser(strings.NewReader(`
{
	"version": 1,
	"server": "v14.1.0rc2",
	"currentLedger": 31883135,
	"networkPassphrase": "Public Global Stellar Network ; September 2015"
}`))
	}

	makeHASReaderNoNetwork := func() io.ReadCloser {
		return ioutil.NopCloser(strings.NewReader(`
{
	"version": 1,
	"server": "v14.1.0rc2",
	"currentLedger": 31883135
}`))
	}

	// No network passphrase set in options
	archive := MustConnect("mock://test", ArchiveOptions{CheckpointFrequency: DefaultCheckpointFrequency})
	err := archive.backend.PutFile("has.json", makeHASReader())
	assert.NoError(t, err)
	_, err = archive.GetPathHAS("has.json")
	assert.NoError(t, err)

	// No network passphrase set in HAS
	archive = MustConnect("mock://test", ArchiveOptions{
		NetworkPassphrase:   "Public Global Stellar Network ; September 2015",
		CheckpointFrequency: DefaultCheckpointFrequency,
	})
	err = archive.backend.PutFile("has.json", makeHASReaderNoNetwork())
	assert.NoError(t, err)
	_, err = archive.GetPathHAS("has.json")
	assert.NoError(t, err)

	// Correct network passphrase set in options
	archive = MustConnect("mock://test", ArchiveOptions{
		NetworkPassphrase:   "Public Global Stellar Network ; September 2015",
		CheckpointFrequency: DefaultCheckpointFrequency,
	})
	err = archive.backend.PutFile("has.json", makeHASReader())
	assert.NoError(t, err)
	_, err = archive.GetPathHAS("has.json")
	assert.NoError(t, err)

	// Incorrect network passphrase set in options
	archive = MustConnect("mock://test", ArchiveOptions{
		NetworkPassphrase:   "Test SDF Network ; September 2015",
		CheckpointFrequency: DefaultCheckpointFrequency,
	})
	err = archive.backend.PutFile("has.json", makeHASReader())
	assert.NoError(t, err)
	_, err = archive.GetPathHAS("has.json")
	assert.EqualError(t, err, "Network passphrase does not match! expected=Test SDF Network ; September 2015 actual=Public Global Stellar Network ; September 2015")
}

func TestXdrDecode(t *testing.T) {

	xdrbytes := []byte{

		0, 0, 0, 0, // entry type 0, liveentry

		0, 32, 223, 100, // lastmodified 2154340

		0, 0, 0, 0, // entry type 0, account

		0, 0, 0, 0, // key type 0
		23, 140, 68, 253, // ed25519 key (32 bytes)
		184, 162, 186, 195,
		118, 239, 158, 210,
		100, 241, 174, 254,
		108, 110, 165, 140,
		75, 76, 83, 141,
		104, 212, 227, 80,
		1, 214, 157, 7,

		0, 0, 0, 29, // 64bit balance: 125339976000
		46, 216, 65, 64,

		0, 0, 129, 170, // 64bit seqnum: 142567144423475
		0, 0, 0, 51,

		0, 0, 0, 1, // numsubentries: 1

		0, 0, 0, 1, // inflationdest type, populated

		0, 0, 0, 0, // key type 0
		87, 240, 19, 71, // ed25519 key (32 bytes)
		52, 91, 9, 62,
		213, 239, 178, 85,
		161, 119, 108, 251,
		168, 90, 76, 116,
		12, 48, 134, 248,
		115, 255, 117, 50,
		19, 18, 170, 203,

		0, 0, 0, 0, // flags

		0, 0, 0, 19, // homedomain: 19 bytes + 1 null padding
		99, 101, 110, 116, // "centaurus.xcoins.de"
		97, 117, 114, 117,
		115, 46, 120, 99,
		111, 105, 110, 115,
		46, 100, 101, 0,

		1, 0, 0, 0, // thresholds
		0, 0, 0, 0, // signers (null)

		0, 0, 0, 0, // entry.account.ext.v: 0

		0, 0, 0, 0, // entry.ext.v: 0
	}

	assert.Equal(t, len(xdrbytes), 152)

	var tmp xdr.BucketEntry
	n, err := xdr.Unmarshal(bytes.NewReader(xdrbytes[:]), &tmp)
	fmt.Printf("Decoded %d bytes\n", n)
	if err != nil {
		panic(err)
	}
	assert.Equal(t, len(xdrbytes), n)

	var out bytes.Buffer
	n, err = xdr.Marshal(&out, &tmp)
	fmt.Printf("Encoded %d bytes\n", n)
	if err != nil {
		panic(err)
	}

	assert.Equal(t, out.Len(), n)
	assert.Equal(t, out.Bytes(), xdrbytes)
}

type xdrEntry interface {
	MarshalBinary() ([]byte, error)
}

func writeCategoryFile(t *testing.T, backend storage.Storage, path string, entries []xdrEntry) {
	file := &bytes.Buffer{}
	writer := gzip.NewWriter(file)

	for _, entry := range entries {
		assert.NoError(t, xdr.MarshalFramed(writer, entry))
	}
	assert.NoError(t, writer.Close())

	assert.NoError(t, backend.PutFile(path, ioutil.NopCloser(file)))
}

func assertXdrEquals(t *testing.T, a, b xdrEntry) {
	b64, err := xdr.MarshalBase64(a)
	assert.NoError(t, err)
	other, err := xdr.MarshalBase64(b)
	assert.NoError(t, err)
	assert.Equal(t, b64, other)
}

func TestGetLedgers(t *testing.T) {
	archive := GetTestMockArchive()
	_, err := archive.GetLedgers(1000, 1002)
	assert.Equal(t, uint32(1), archive.GetStats()[0].GetRequests())
	assert.Equal(t, uint32(0), archive.GetStats()[0].GetDownloads())
	assert.EqualError(t, err, "checkpoint 1023 is not published")
	ledgerHeaders, transactions, results := makeFakeArchive(t, archive)

	stats := archive.GetStats()[0]
	ledgers, err := archive.GetLedgers(1000, 1002)

	assert.NoError(t, err)
	assert.Len(t, ledgers, 3)
	// it started at 1, incurred 6 requests total: 3 queries + 3 downloads
	assert.EqualValues(t, 7, stats.GetRequests())
	// started 0, incurred 3 file downloads
	assert.EqualValues(t, 3, stats.GetDownloads())
	assert.EqualValues(t, 0, stats.GetCacheHits())
	for i, seq := range []uint32{1000, 1001, 1002} {
		ledger := ledgers[seq]
		assertXdrEquals(t, ledgerHeaders[i], ledger.Header)
		assertXdrEquals(t, transactions[i], ledger.Transaction)
		assertXdrEquals(t, results[i], ledger.TransactionResult)
	}

	// Repeat the same check but ensure the cache was used
	ledgers, err = archive.GetLedgers(1000, 1002) // all cached
	assert.NoError(t, err)
	assert.Len(t, ledgers, 3)

	// downloads should not change because of the cache
	assert.EqualValues(t, 3, stats.GetDownloads())
	// but requests increase because of 3 fetches to categories
	assert.EqualValues(t, 10, stats.GetRequests())
	assert.EqualValues(t, 3, stats.GetCacheHits())
	for i, seq := range []uint32{1000, 1001, 1002} {
		ledger := ledgers[seq]
		assertXdrEquals(t, ledgerHeaders[i], ledger.Header)
		assertXdrEquals(t, transactions[i], ledger.Transaction)
		assertXdrEquals(t, results[i], ledger.TransactionResult)
	}

	// remove the cached files without informing it and ensure it fills up again
	require.NoError(t, os.RemoveAll(cachePath))
	ledgers, err = archive.GetLedgers(1000, 1002) // uncached, refetch
	assert.NoError(t, err)
	assert.Len(t, ledgers, 3)

	// downloads should increase again
	assert.EqualValues(t, 6, stats.GetDownloads())
	assert.EqualValues(t, 3, stats.GetCacheHits())
}

func TestStressfulGetLedgers(t *testing.T) {
	archive := GetTestMockArchive()
	ledgerHeaders, transactions, results := makeFakeArchive(t, archive)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)

		go func() {
			time.Sleep(time.Millisecond) // encourage interleaved execution
			ledgers, err := archive.GetLedgers(1000, 1002)
			assert.NoError(t, err)
			assert.Len(t, ledgers, 3)
			for i, seq := range []uint32{1000, 1001, 1002} {
				ledger := ledgers[seq]
				assertXdrEquals(t, ledgerHeaders[i], ledger.Header)
				assertXdrEquals(t, transactions[i], ledger.Transaction)
				assertXdrEquals(t, results[i], ledger.TransactionResult)
			}

			wg.Done()
		}()
	}

	require.Eventually(t, func() bool { wg.Wait(); return true }, time.Minute, time.Second)
}

func TestCacheDeadlocks(t *testing.T) {
	archive := MustConnect("fmock://test", ArchiveOptions{
		CheckpointFrequency: 64,
		CachePath:           cachePath,
	})
	makeFakeArchive(t, archive)
	_, err := archive.GetLedgers(1000, 1002)
	require.Error(t, err)
}

func makeFakeArchive(t *testing.T, archive *Archive) (
	[]xdr.LedgerHeaderHistoryEntry,
	[]xdr.TransactionHistoryEntry,
	[]xdr.TransactionHistoryResultEntry,
) {
	ledgerHeaders := []xdr.LedgerHeaderHistoryEntry{
		{
			Hash: xdr.Hash{1},
			Header: xdr.LedgerHeader{
				LedgerSeq: 1000,
			},
		},
		{
			Hash: xdr.Hash{2},
			Header: xdr.LedgerHeader{
				LedgerSeq: 1001,
			},
		},
		{
			Hash: xdr.Hash{3},
			Header: xdr.LedgerHeader{
				LedgerSeq: 1002,
			},
		},
	}
	writeCategoryFile(
		t, archive.backend, "ledger/00/00/03/ledger-000003ff.xdr.gz",
		[]xdrEntry{ledgerHeaders[0], ledgerHeaders[1], ledgerHeaders[2]},
	)

	transactions := []xdr.TransactionHistoryEntry{
		{
			LedgerSeq: 1000,
			TxSet: xdr.TransactionSet{
				PreviousLedgerHash: xdr.Hash{10},
			},
		},
		{
			LedgerSeq: 1001,
			TxSet: xdr.TransactionSet{
				PreviousLedgerHash: xdr.Hash{11},
			},
		},
		{
			LedgerSeq: 1002,
			TxSet: xdr.TransactionSet{
				PreviousLedgerHash: xdr.Hash{12},
			},
		},
	}
	writeCategoryFile(
		t, archive.backend, "transactions/00/00/03/transactions-000003ff.xdr.gz",
		[]xdrEntry{transactions[0], transactions[1], transactions[2]},
	)

	result := xdr.TransactionResult{Result: xdr.TransactionResultResult{Code: xdr.TransactionResultCodeTxBadSeq}}
	results := []xdr.TransactionHistoryResultEntry{
		{
			LedgerSeq: 1000,
			TxResultSet: xdr.TransactionResultSet{
				Results: []xdr.TransactionResultPair{
					{TransactionHash: xdr.Hash{213}, Result: result},
				},
			},
		},
		{
			LedgerSeq: 1001,
			TxResultSet: xdr.TransactionResultSet{
				Results: []xdr.TransactionResultPair{
					{TransactionHash: xdr.Hash{198}, Result: result},
				},
			},
		},
		{
			LedgerSeq: 1002,
			TxResultSet: xdr.TransactionResultSet{
				Results: []xdr.TransactionResultPair{
					{TransactionHash: xdr.Hash{131}, Result: result},
				},
			},
		},
	}
	writeCategoryFile(
		t, archive.backend, "results/00/00/03/results-000003ff.xdr.gz",
		[]xdrEntry{results[0], results[1], results[2]},
	)

	return ledgerHeaders, transactions, results
}
