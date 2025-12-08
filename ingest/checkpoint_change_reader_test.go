package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/stellar/go-stellar-sdk/historyarchive"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestCheckpointChangeReaderTestSuite(t *testing.T) {
	suite.Run(t, new(CheckpointChangeReaderTestSuite))
}

type CheckpointChangeReaderTestSuite struct {
	suite.Suite
	mockArchive          *historyarchive.MockArchive
	reader               *CheckpointChangeReader
	has                  historyarchive.HistoryArchiveState
	parentCtxCancel      context.CancelFunc
	mockBucketExistsCall *mock.Call
	mockBucketSizeCall   *mock.Call
}

func (s *CheckpointChangeReaderTestSuite) SetupTest() {
	s.mockArchive = &historyarchive.MockArchive{}

	err := json.Unmarshal([]byte(hasExample), &s.has)
	s.Require().NoError(err)

	ledgerSeq := uint32(24123007)

	s.mockArchive.
		On("GetCheckpointHAS", ledgerSeq).
		Return(s.has, nil)

	// BucketExists should be called 21 times (11 levels, last without `snap`)
	s.mockBucketExistsCall = s.mockArchive.
		On("BucketExists", mock.AnythingOfType("historyarchive.Hash")).
		Return(true, nil).Times(21)

	// BucketSize should be called 21 times (11 levels, last without `snap`)
	s.mockBucketSizeCall = s.mockArchive.
		On("BucketSize", mock.AnythingOfType("historyarchive.Hash")).
		Return(int64(100), nil).Times(21)

	s.mockArchive.
		On("GetCheckpointManager").
		Return(historyarchive.NewCheckpointManager(
			historyarchive.DefaultCheckpointFrequency))

	var ctx context.Context
	ctx, s.parentCtxCancel = context.WithCancel(context.Background())
	s.reader, err = NewCheckpointChangeReader(
		ctx,
		s.mockArchive,
		ledgerSeq,
		DisableBucketListValidation,
	)
	s.Require().NotNil(s.reader)
	s.Require().NoError(err)
	s.Assert().Equal(ledgerSeq, s.reader.sequence)
}

func (s *CheckpointChangeReaderTestSuite) TearDownTest() {
	s.mockArchive.AssertExpectations(s.T())
}

// TestSimple test reading buckets with a single live entry.
func (s *CheckpointChangeReaderTestSuite) TestSimple() {
	meta := metaEntry(23)
	liveType := xdr.BucketListTypeLive
	meta.MetaEntry.Ext = xdr.BucketMetadataExt{
		V:              1,
		BucketListType: &liveType,
	}
	curr1 := createXdrStream(
		meta,
		entryAccount(xdr.BucketEntryTypeLiveentry, "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 1),
	)

	nextBucket := createBucketChannel(s.has.CurrentBuckets)

	// Return curr1 stream for the first bucket...
	s.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(curr1, nil).Once()

	// ...and empty streams for the rest of the buckets.
	for hash := range nextBucket {
		s.mockArchive.
			On("GetXdrStreamForHash", hash).
			Return(createXdrStream(), nil).Once()
	}

	var e Change
	var err error
	e, err = s.reader.Read()
	s.Require().NoError(err)

	id := e.Post.Data.MustAccount().AccountId
	s.Assert().Equal("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", id.Address())

	_, err = s.reader.Read()
	s.Require().Equal(err, io.EOF)
}

func (s *CheckpointChangeReaderTestSuite) TestReadAfterClose() {
	meta := metaEntry(23)
	liveType := xdr.BucketListTypeLive
	meta.MetaEntry.Ext = xdr.BucketMetadataExt{
		V:              1,
		BucketListType: &liveType,
	}
	curr1 := createXdrStream(
		meta,
		entryAccount(xdr.BucketEntryTypeLiveentry, "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 1),
		entryAccount(xdr.BucketEntryTypeLiveentry, "GCMNSW2UZMSH3ZFRLWP6TW2TG4UX4HLSYO5HNIKUSFMLN2KFSF26JKWF", 10),
	)

	nextBucket := createBucketChannel(s.has.CurrentBuckets)

	// Return curr1 stream for the first bucket...
	s.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(curr1, nil).Once()

	// ...and empty streams for the rest of the buckets.
	for hash := range nextBucket {
		s.mockArchive.
			On("GetXdrStreamForHash", hash).
			Return(createXdrStream(), nil).Maybe()
	}

	var e Change
	var err error
	e, err = s.reader.Read()
	s.Require().NoError(err)

	id := e.Post.Data.MustAccount().AccountId
	s.Assert().Equal("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", id.Address())

	s.Require().NoError(s.reader.Close())

	// there is a race condition because we are selecting on either
	// the read channel or the context done channel so it is possible
	// we receive the 2nd ledger entry before the context cancel error
	for i := 0; i < 2; i++ {
		e, err = s.reader.Read()
		if err == nil {
			id = e.Post.Data.MustAccount().AccountId
			s.Assert().Equal("GCMNSW2UZMSH3ZFRLWP6TW2TG4UX4HLSYO5HNIKUSFMLN2KFSF26JKWF", id.Address())
			continue
		}
		break
	}
	s.Require().ErrorContains(err, "reader is closed")
}

func (s *CheckpointChangeReaderTestSuite) TestContextCanceled() {
	meta := metaEntry(23)
	liveType := xdr.BucketListTypeLive
	meta.MetaEntry.Ext = xdr.BucketMetadataExt{
		V:              1,
		BucketListType: &liveType,
	}
	curr1 := createXdrStream(
		meta,
		entryAccount(xdr.BucketEntryTypeLiveentry, "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 1),
		entryAccount(xdr.BucketEntryTypeLiveentry, "GCMNSW2UZMSH3ZFRLWP6TW2TG4UX4HLSYO5HNIKUSFMLN2KFSF26JKWF", 10),
	)

	nextBucket := createBucketChannel(s.has.CurrentBuckets)

	// Return curr1 stream for the first bucket...
	s.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(curr1, nil).Once()

	// ...and empty streams for the rest of the buckets.
	for hash := range nextBucket {
		s.mockArchive.
			On("GetXdrStreamForHash", hash).
			Return(createXdrStream(), nil).Maybe()
	}

	var e Change
	var err error
	e, err = s.reader.Read()
	s.Require().NoError(err)

	id := e.Post.Data.MustAccount().AccountId
	s.Assert().Equal("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", id.Address())

	s.parentCtxCancel()

	// there is a race condition because we are selecting on either
	// the read channel or the context done channel so it is possible
	// we receive the 2nd ledger entry before the context cancel error
	for i := 0; i < 2; i++ {
		e, err = s.reader.Read()
		if err == nil {
			id = e.Post.Data.MustAccount().AccountId
			s.Assert().Equal("GCMNSW2UZMSH3ZFRLWP6TW2TG4UX4HLSYO5HNIKUSFMLN2KFSF26JKWF", id.Address())
			continue
		}
		break
	}
	s.Require().ErrorContains(err, "context canceled")
}

// TestRemoved test reading buckets with a single live entry that was removed.
func (s *CheckpointChangeReaderTestSuite) TestRemoved() {
	curr1 := createXdrStream(
		entryAccount(xdr.BucketEntryTypeDeadentry, "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 1),
	)

	snap1 := createXdrStream(
		entryAccount(xdr.BucketEntryTypeLiveentry, "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 1),
	)

	nextBucket := createBucketChannel(s.has.CurrentBuckets)

	// Return curr1 and snap1 stream for the first two bucket...
	s.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(curr1, nil).Once()

	s.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(snap1, nil).Once()

	// ...and empty streams for the rest of the buckets.
	for hash := range nextBucket {
		s.mockArchive.
			On("GetXdrStreamForHash", hash).
			Return(createXdrStream(), nil).Once()
	}

	_, err := s.reader.Read()
	s.Require().Equal(err, io.EOF)
}

// TestConcurrentRead test concurrent reads for race conditions
func (s *CheckpointChangeReaderTestSuite) TestConcurrentRead() {
	curr1 := createXdrStream(
		entryAccount(xdr.BucketEntryTypeDeadentry, "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 1),
	)

	snap1 := createXdrStream(
		entryAccount(xdr.BucketEntryTypeLiveentry, "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 1),
		entryAccount(xdr.BucketEntryTypeLiveentry, "GCMNSW2UZMSH3ZFRLWP6TW2TG4UX4HLSYO5HNIKUSFMLN2KFSF26JKWF", 1),
		entryAccount(xdr.BucketEntryTypeLiveentry, "GB6IPC7LIOSRY26MXHQ3QJ32MTELYAA6YFIRBXZVVGTU7AOI4KUFOQ54", 1),
		entryAccount(xdr.BucketEntryTypeLiveentry, "GCK45YKCFNIOICB4TWPCOPWLQYNUKCJVV7OMMHH55AB3DD67K4E54STO", 1),
	)

	nextBucket := createBucketChannel(s.has.CurrentBuckets)

	// Return curr1 and snap1 stream for the first two bucket...
	s.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(curr1, nil).Once()

	s.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(snap1, nil).Once()

	// ...and empty streams for the rest of the buckets.
	for hash := range nextBucket {
		s.mockArchive.
			On("GetXdrStreamForHash", hash).
			Return(createXdrStream(), nil).Once()
	}

	// 3 live entries
	var wg sync.WaitGroup

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			_, err := s.reader.Read()
			s.Assert().Nil(err)
			wg.Done()
		}()
	}

	wg.Wait()

	// Next call should return io.EOF
	_, err := s.reader.Read()
	s.Require().Equal(err, io.EOF)
}

// TestEnsureLatestLiveEntry tests if a live entry overrides an older initentry
func (s *CheckpointChangeReaderTestSuite) TestEnsureLatestLiveEntry() {
	curr1 := createXdrStream(
		metaEntry(11),
		entryAccount(xdr.BucketEntryTypeLiveentry, "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 1),
		entryAccount(xdr.BucketEntryTypeInitentry, "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 2),
	)

	nextBucket := createBucketChannel(s.has.CurrentBuckets)

	// Return curr1 stream, rest won't be read due to an error
	s.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(curr1, nil).Once()

	// ...and empty streams for the rest of the buckets.
	for hash := range nextBucket {
		s.mockArchive.
			On("GetXdrStreamForHash", hash).
			Return(createXdrStream(), nil).Once()
	}

	entry, err := s.reader.Read()
	s.Require().Nil(err)
	// Latest entry balance is 1
	s.Assert().Equal(xdr.Int64(1), entry.Post.Data.Account.Balance)

	_, err = s.reader.Read()
	s.Require().Equal(err, io.EOF)
}

func (s *CheckpointChangeReaderTestSuite) TestUniqueInitEntryOptimization() {
	curr1 := createXdrStream(
		metaEntry(20),
		entryAccount(xdr.BucketEntryTypeLiveentry, "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 1),
		entryCB(xdr.BucketEntryTypeDeadentry, xdr.Hash{1, 2, 3}, 100),
		entryOffer(xdr.BucketEntryTypeDeadentry, "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 20),
	)

	snap1 := createXdrStream(
		metaEntry(20),
		entryAccount(xdr.BucketEntryTypeInitentry, "GALPCCZN4YXA3YMJHKL6CVIECKPLJJCTVMSNYWBTKJW4K5HQLYLDMZTB", 1),
		entryAccount(xdr.BucketEntryTypeInitentry, "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 1),
		entryAccount(xdr.BucketEntryTypeInitentry, "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H", 1),
		entryCB(xdr.BucketEntryTypeInitentry, xdr.Hash{1, 2, 3}, 100),
		entryOffer(xdr.BucketEntryTypeInitentry, "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 20),
		entryAccount(xdr.BucketEntryTypeInitentry, "GAP2KHWUMOHY7IO37UJY7SEBIITJIDZS5DRIIQRPEUT4VUKHZQGIRWS4", 1),
		entryAccount(xdr.BucketEntryTypeInitentry, "GAIH3ULLFQ4DGSECF2AR555KZ4KNDGEKN4AFI4SU2M7B43MGK3QJZNSR", 1),
	)

	nextBucket := createBucketChannel(s.has.CurrentBuckets)

	// Return curr1 and snap1 stream for the first two bucket...
	s.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(curr1, nil).Once()

	s.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(snap1, nil).Once()

	// ...and empty streams for the rest of the buckets.
	for hash := range nextBucket {
		s.mockArchive.
			On("GetXdrStreamForHash", hash).
			Return(createXdrStream(), nil).Once()
	}

	// replace readChan with an unbuffered channel so we can test behavior of when items are added / removed
	// from visitedLedgerKeys
	s.reader.readChan = make(chan xdr.LedgerEntry, 0)

	change, err := s.reader.Read()
	s.Require().NoError(err)
	key, err := change.Post.Data.LedgerKey()
	s.Require().NoError(err)
	s.Require().True(
		key.Equals(xdr.LedgerKey{
			Type:    xdr.LedgerEntryTypeAccount,
			Account: &xdr.LedgerKeyAccount{AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML")},
		}),
	)

	change, err = s.reader.Read()
	s.Require().NoError(err)
	key, err = change.Post.Data.LedgerKey()
	s.Require().NoError(err)
	s.Require().True(
		key.Equals(xdr.LedgerKey{
			Type:    xdr.LedgerEntryTypeAccount,
			Account: &xdr.LedgerKeyAccount{AccountId: xdr.MustAddress("GALPCCZN4YXA3YMJHKL6CVIECKPLJJCTVMSNYWBTKJW4K5HQLYLDMZTB")},
		}),
	)
	s.Require().Equal(len(s.reader.visitedLedgerKeys), 3)
	s.assertVisitedLedgerKeysContains(xdr.LedgerKey{
		Type:    xdr.LedgerEntryTypeAccount,
		Account: &xdr.LedgerKeyAccount{AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML")},
	})
	s.assertVisitedLedgerKeysContains(xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeOffer,
		Offer: &xdr.LedgerKeyOffer{
			SellerId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
			OfferId:  20,
		},
	})
	s.assertVisitedLedgerKeysContains(xdr.LedgerKey{
		Type: xdr.LedgerEntryTypeClaimableBalance,
		ClaimableBalance: &xdr.LedgerKeyClaimableBalance{
			BalanceId: xdr.ClaimableBalanceId{
				Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
				V0:   &xdr.Hash{1, 2, 3},
			},
		},
	})

	change, err = s.reader.Read()
	s.Require().NoError(err)
	key, err = change.Post.Data.LedgerKey()
	s.Require().NoError(err)
	s.Require().True(
		key.Equals(xdr.LedgerKey{
			Type:    xdr.LedgerEntryTypeAccount,
			Account: &xdr.LedgerKeyAccount{AccountId: xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")},
		}),
	)

	change, err = s.reader.Read()
	s.Require().NoError(err)
	key, err = change.Post.Data.LedgerKey()
	s.Require().NoError(err)
	s.Require().True(
		key.Equals(xdr.LedgerKey{
			Type:    xdr.LedgerEntryTypeAccount,
			Account: &xdr.LedgerKeyAccount{AccountId: xdr.MustAddress("GAP2KHWUMOHY7IO37UJY7SEBIITJIDZS5DRIIQRPEUT4VUKHZQGIRWS4")},
		}),
	)
	// the offer and cb ledger keys should now be removed from visitedLedgerKeys
	// because we encountered the init entries in the bucket
	s.Require().Equal(len(s.reader.visitedLedgerKeys), 1)
	s.assertVisitedLedgerKeysContains(xdr.LedgerKey{
		Type:    xdr.LedgerEntryTypeAccount,
		Account: &xdr.LedgerKeyAccount{AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML")},
	})

	change, err = s.reader.Read()
	s.Require().NoError(err)
	key, err = change.Post.Data.LedgerKey()
	s.Require().NoError(err)
	s.Require().True(
		key.Equals(xdr.LedgerKey{
			Type:    xdr.LedgerEntryTypeAccount,
			Account: &xdr.LedgerKeyAccount{AccountId: xdr.MustAddress("GAIH3ULLFQ4DGSECF2AR555KZ4KNDGEKN4AFI4SU2M7B43MGK3QJZNSR")},
		}),
	)

	_, err = s.reader.Read()
	s.Require().Equal(err, io.EOF)
}

func (s *CheckpointChangeReaderTestSuite) assertVisitedLedgerKeysContains(key xdr.LedgerKey) {
	encodingBuffer := xdr.NewEncodingBuffer()
	keyBytes, err := encodingBuffer.LedgerKeyUnsafeMarshalBinaryCompress(key)
	s.Require().NoError(err)
	s.Require().True(s.reader.visitedLedgerKeys.Contains(string(keyBytes)))
}

// TestMalformedProtocol11Bucket tests a buggy protocol 11 bucket (meta not the first entry)
func (s *CheckpointChangeReaderTestSuite) TestMalformedProtocol11Bucket() {
	curr1 := createXdrStream(
		entryAccount(xdr.BucketEntryTypeLiveentry, "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 1),
		metaEntry(11),
	)

	nextBucket := createBucketChannel(s.has.CurrentBuckets)

	// Return curr1 stream, rest won't be read due to an error
	s.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(curr1, nil).Once()

	// Account entry
	_, err := s.reader.Read()
	s.Require().Nil(err)

	// Meta entry
	_, err = s.reader.Read()
	s.Require().NotNil(err)
	s.Assert().Equal("METAENTRY not the first entry (n=1) in the bucket hash '517bea4c6627a688a8ce501febd8c562e737e3d86b29689d9956217640f3c74b'", err.Error())
}

// TestMalformedProtocol11BucketNoMeta tests a buggy protocol 11 bucket (no meta entry)
func (s *CheckpointChangeReaderTestSuite) TestMalformedProtocol11BucketNoMeta() {
	curr1 := createXdrStream(
		entryAccount(xdr.BucketEntryTypeInitentry, "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 1),
	)

	nextBucket := createBucketChannel(s.has.CurrentBuckets)

	// Return curr1 stream, rest won't be read due to an error
	s.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(curr1, nil).Once()

	// Init entry without meta
	_, err := s.reader.Read()
	s.Require().NotNil(err)
	s.Assert().Equal("Read INITENTRY from version <11 bucket: 0@517bea4c6627a688a8ce501febd8c562e737e3d86b29689d9956217640f3c74b", err.Error())
}

// TestMalformedBucketListType ensures the checkpoint change reader asserts its reading from the live bucketlist
func (s *CheckpointChangeReaderTestSuite) TestMalformedBucketListType() {
	meta := metaEntry(23)
	hotArchiveType := xdr.BucketListTypeHotArchive
	meta.MetaEntry.Ext = xdr.BucketMetadataExt{
		V:              1,
		BucketListType: &hotArchiveType,
	}
	curr1 := createXdrStream(
		meta,
		entryAccount(xdr.BucketEntryTypeLiveentry, "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 1),
	)

	nextBucket := createBucketChannel(s.has.CurrentBuckets)

	// Return curr1 stream, rest won't be read due to an error
	s.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(curr1, nil).Once()

	// Meta entry
	_, err := s.reader.Read()
	s.Require().NotNil(err)
	s.Assert().EqualError(err, "expected bucket list type to be live (instead got BucketListTypeHotArchive) in the bucket hash '517bea4c6627a688a8ce501febd8c562e737e3d86b29689d9956217640f3c74b'")
}

// TestFilter exercises the WithFilter functionality by ignoring a DEADENTRY
// for a specific account in a newer bucket so that an older LIVEENTRY for
// that account is yielded.
func (s *CheckpointChangeReaderTestSuite) TestFilter() {
	// Prepare streams: newer bucket has a DEADENTRY for A; older bucket has LIVEENTRY for A.
	curr1 := createXdrStream(
		entryAccount(xdr.BucketEntryTypeDeadentry, "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 1),
		entryCB(xdr.BucketEntryTypeInitentry, xdr.Hash{1, 2, 3}, 100),
	)
	snap1 := createXdrStream(
		entryAccount(xdr.BucketEntryTypeLiveentry, "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 1),
	)

	nextBucket := createBucketChannel(s.has.CurrentBuckets)

	// Mock to return curr1 then snap1 for the first two buckets...
	s.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(curr1, nil).Once()

	s.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(snap1, nil).Once()

	// ...and empty streams for the rest of the buckets.
	for hash := range nextBucket {
		s.mockArchive.
			On("GetXdrStreamForHash", hash).
			Return(createXdrStream(), nil).Once()
	}

	// Recreate reader with a filter that ignores the DEADENTRY for account A.
	ctx := context.Background()
	var err error
	s.reader, err = NewCheckpointChangeReader(
		ctx,
		s.mockArchive,
		s.reader.sequence,
		DisableBucketListValidation,
		WithFilter(
			// accept all live entries
			func(le xdr.LedgerEntry) bool {
				return le.Data.Type != xdr.LedgerEntryTypeClaimableBalance
			},
			// ignore DEADENTRY for our target account
			func(key xdr.LedgerKey) bool {
				if key.Type != xdr.LedgerEntryTypeAccount {
					return true
				}
				return key.Account.AccountId.Address() != "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"
			},
		),
	)
	s.Require().NoError(err)

	// We should now receive the older LIVEENTRY for the account because the
	// newer DEADENTRY was filtered out.
	change, err := s.reader.Read()
	s.Require().NoError(err)
	id := change.Post.Data.MustAccount().AccountId
	s.Assert().Equal("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", id.Address())

	// Then EOF
	_, err = s.reader.Read()
	s.Require().Equal(err, io.EOF)
}

func TestBucketExistsTestSuite(t *testing.T) {
	suite.Run(t, new(BucketExistsTestSuite))
}

type BucketExistsTestSuite struct {
	suite.Suite
	mockArchive    *historyarchive.MockArchive
	reader         *CheckpointChangeReader
	cancel         context.CancelFunc
	expectedSleeps []time.Duration
}

func (s *BucketExistsTestSuite) SetupTest() {
	s.mockArchive = &historyarchive.MockArchive{}

	ledgerSeq := uint32(24123007)
	s.mockArchive.
		On("GetCheckpointHAS", ledgerSeq).
		Return(historyarchive.HistoryArchiveState{}, nil)

	s.mockArchive.
		On("GetCheckpointManager").
		Return(historyarchive.NewCheckpointManager(
			historyarchive.DefaultCheckpointFrequency))

	ctx, cancel := context.WithCancel(context.Background())
	var err error
	s.reader, err = NewCheckpointChangeReader(
		ctx,
		s.mockArchive,
		ledgerSeq,
	)
	s.cancel = cancel
	s.Require().NoError(err)
	s.reader.sleep = func(d time.Duration) {
		if len(s.expectedSleeps) == 0 {
			s.Assert().Fail("unexpected call to sleep()")
			return
		}
		s.Assert().Equal(s.expectedSleeps[0], d)
		s.expectedSleeps = s.expectedSleeps[1:]
	}
}

func (s *BucketExistsTestSuite) TearDownTest() {
	s.mockArchive.AssertExpectations(s.T())
}

func (s *BucketExistsTestSuite) TestBucketExists() {
	for _, expected := range []bool{true, false} {
		hash := historyarchive.Hash{1, 2, 3}
		s.mockArchive.On("BucketExists", hash).
			Return(expected, nil).Once()
		exists, err := s.reader.bucketExists(hash)
		s.Assert().Equal(expected, exists)
		s.Assert().NoError(err)
	}
}

func TestReadBucketEntryTestSuite(t *testing.T) {
	suite.Run(t, new(ReadBucketEntryTestSuite))
}

type ReadBucketEntryTestSuite struct {
	suite.Suite
	mockArchive *historyarchive.MockArchive
	reader      *CheckpointChangeReader
	cancel      context.CancelFunc
}

func (s *ReadBucketEntryTestSuite) SetupTest() {
	s.mockArchive = &historyarchive.MockArchive{}

	ledgerSeq := uint32(24123007)
	s.mockArchive.
		On("GetCheckpointHAS", ledgerSeq).
		Return(historyarchive.HistoryArchiveState{}, nil)

	s.mockArchive.
		On("GetCheckpointManager").
		Return(historyarchive.NewCheckpointManager(
			historyarchive.DefaultCheckpointFrequency))

	ctx, cancel := context.WithCancel(context.Background())
	var err error
	s.reader, err = NewCheckpointChangeReader(
		ctx,
		s.mockArchive,
		ledgerSeq,
	)
	s.cancel = cancel
	s.Require().NoError(err)
}

func (s *ReadBucketEntryTestSuite) TearDownTest() {
	s.mockArchive.AssertExpectations(s.T())
}

func (s *ReadBucketEntryTestSuite) TestNewXDRStream() {
	emptyHash := historyarchive.EmptyXdrArrayHash()
	expectedStream := createXdrStream(metaEntry(1), metaEntry(2))

	hash, ok := expectedStream.ExpectedHash()
	s.Require().NotEqual(historyarchive.Hash(hash), emptyHash)
	s.Require().False(ok)

	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(expectedStream, nil).Once()

	stream, err := s.reader.newXDRStream(emptyHash)
	s.Require().NoError(err)
	s.Require().True(stream == expectedStream)

	hash, ok = stream.ExpectedHash()
	s.Require().Equal(historyarchive.Hash(hash), emptyHash)
	s.Require().True(ok)
}

func (s *ReadBucketEntryTestSuite) TestReadAllEntries() {
	emptyHash := historyarchive.EmptyXdrArrayHash()
	firstEntry := metaEntry(1)
	secondEntry := metaEntry(2)
	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(createXdrStream(firstEntry, secondEntry), nil).Once()

	stream, err := s.reader.newXDRStream(emptyHash)
	s.Require().NoError(err)

	var entry xdr.BucketEntry
	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().NoError(err)
	s.Require().Equal(entry, firstEntry)

	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().NoError(err)
	s.Require().Equal(entry, secondEntry)

	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().Equal(io.EOF, err)
}

func (s *ReadBucketEntryTestSuite) TestFirstReadFailsWithContextError() {
	emptyHash := historyarchive.EmptyXdrArrayHash()
	firstEntry := metaEntry(1)
	secondEntry := metaEntry(2)
	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(createXdrStream(firstEntry, secondEntry), nil).Once()

	stream, err := s.reader.newXDRStream(emptyHash)
	s.Require().NoError(err)
	s.cancel()

	var entry xdr.BucketEntry
	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().Equal(context.Canceled, err)
}

func (s *ReadBucketEntryTestSuite) TestSecondReadFailsWithContextError() {
	emptyHash := historyarchive.EmptyXdrArrayHash()
	firstEntry := metaEntry(1)
	secondEntry := metaEntry(2)
	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(createXdrStream(firstEntry, secondEntry), nil).Once()

	stream, err := s.reader.newXDRStream(emptyHash)
	s.Require().NoError(err)

	var entry xdr.BucketEntry
	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().NoError(err)
	s.Require().Equal(entry, firstEntry)
	s.cancel()

	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().Equal(context.Canceled, err)
}

func (s *ReadBucketEntryTestSuite) TestReadEntryAllRetriesFail() {
	emptyHash := historyarchive.EmptyXdrArrayHash()

	for i := 0; i < 4; i++ {
		s.mockArchive.
			On("GetXdrStreamForHash", emptyHash).
			Return(createInvalidXdrStream(nil), nil).Once()
	}

	stream, err := s.reader.newXDRStream(emptyHash)
	s.Require().NoError(err)

	var entry xdr.BucketEntry
	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().EqualError(err, "Read wrong number of bytes from XDR")
}

func (s *ReadBucketEntryTestSuite) TestReadEntryRetryIgnoresProtocolCloseError() {
	emptyHash := historyarchive.EmptyXdrArrayHash()

	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(
			createInvalidXdrStream(errors.New("stream error: stream ID 75; PROTOCOL_ERROR")),
			nil,
		).Once()

	expectedEntry := metaEntry(1)
	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(createXdrStream(expectedEntry), nil).Once()

	stream, err := s.reader.newXDRStream(emptyHash)
	s.Require().NoError(err)

	var entry xdr.BucketEntry
	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().NoError(err)
	s.Require().Equal(entry, expectedEntry)

	hash, ok := stream.ExpectedHash()
	s.Require().Equal(historyarchive.Hash(hash), emptyHash)
	s.Require().True(ok)

	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().Equal(err, io.EOF)
}

func (s *ReadBucketEntryTestSuite) TestReadEntryRetryFailsToCreateNewStream() {
	emptyHash := historyarchive.EmptyXdrArrayHash()

	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(createInvalidXdrStream(nil), nil).Once()

	var nilStream *xdr.Stream
	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(nilStream, errors.New("cannot create new stream")).Times(3)

	stream, err := s.reader.newXDRStream(emptyHash)
	s.Require().NoError(err)

	var entry xdr.BucketEntry
	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().EqualError(err, "Error creating new xdr stream: cannot create new stream")
}

func (s *ReadBucketEntryTestSuite) TestReadEntryRetrySucceedsAfterFailsToCreateNewStream() {
	emptyHash := historyarchive.EmptyXdrArrayHash()

	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(createInvalidXdrStream(nil), nil).Once()

	var nilStream *xdr.Stream
	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(nilStream, errors.New("cannot create new stream")).Once()

	firstEntry := metaEntry(1)

	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(createXdrStream(firstEntry), nil).Once()

	stream, err := s.reader.newXDRStream(emptyHash)
	s.Require().NoError(err)

	var entry xdr.BucketEntry
	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().NoError(err)
	s.Require().Equal(entry, firstEntry)

	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().Equal(io.EOF, err)
}

func (s *ReadBucketEntryTestSuite) TestReadEntryRetrySucceeds() {
	emptyHash := historyarchive.EmptyXdrArrayHash()

	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(createInvalidXdrStream(nil), nil).Once()

	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(createInvalidXdrStream(nil), nil).Once()

	expectedEntry := metaEntry(1)
	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(createXdrStream(expectedEntry), nil).Once()

	stream, err := s.reader.newXDRStream(emptyHash)
	s.Require().NoError(err)

	var entry xdr.BucketEntry
	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().NoError(err)
	s.Require().Equal(entry, expectedEntry)

	hash, ok := stream.ExpectedHash()
	s.Require().Equal(historyarchive.Hash(hash), emptyHash)
	s.Require().True(ok)

	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().Equal(err, io.EOF)
}

func (s *ReadBucketEntryTestSuite) TestReadEntryRetrySucceedsWithDiscard() {
	emptyHash := historyarchive.EmptyXdrArrayHash()

	firstEntry := metaEntry(1)
	secondEntry := metaEntry(2)

	b := &bytes.Buffer{}
	s.Require().NoError(xdr.MarshalFramed(b, firstEntry))
	writeInvalidFrame(b)

	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(xdrStreamFromBuffer(b), nil).Once()

	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(createXdrStream(firstEntry, secondEntry), nil).Once()

	stream, err := s.reader.newXDRStream(emptyHash)
	s.Require().NoError(err)

	var entry xdr.BucketEntry
	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().NoError(err)
	s.Require().Equal(entry, firstEntry)

	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().NoError(err)
	s.Require().Equal(entry, secondEntry)

	hash, ok := stream.ExpectedHash()
	s.Require().Equal(historyarchive.Hash(hash), emptyHash)
	s.Require().True(ok)

	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().Equal(err, io.EOF)
}

func (s *ReadBucketEntryTestSuite) TestReadEntryRetryFailsWithDiscardError() {
	emptyHash := historyarchive.EmptyXdrArrayHash()

	firstEntry := metaEntry(1)

	b := &bytes.Buffer{}
	s.Require().NoError(xdr.MarshalFramed(b, firstEntry))
	writeInvalidFrame(b)

	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(xdrStreamFromBuffer(b), nil).Times(4)

	b = &bytes.Buffer{}
	b.WriteString("a")

	stream, err := s.reader.newXDRStream(emptyHash)
	s.Require().NoError(err)

	var entry xdr.BucketEntry
	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().NoError(err)
	s.Require().Equal(entry, firstEntry)

	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().EqualError(err, "Error discarding from xdr stream: EOF")
}

func (s *ReadBucketEntryTestSuite) TestReadEntryRetrySucceedsAfterDiscardError() {
	emptyHash := historyarchive.EmptyXdrArrayHash()

	firstEntry := metaEntry(1)
	secondEntry := metaEntry(2)

	b := &bytes.Buffer{}
	s.Require().NoError(xdr.MarshalFramed(b, firstEntry))
	writeInvalidFrame(b)

	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(xdrStreamFromBuffer(b), nil).Once()

	b = &bytes.Buffer{}
	b.WriteString("a")

	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(xdrStreamFromBuffer(b), nil).Once()

	s.mockArchive.
		On("GetXdrStreamForHash", emptyHash).
		Return(createXdrStream(firstEntry, secondEntry), nil).Once()

	stream, err := s.reader.newXDRStream(emptyHash)
	s.Require().NoError(err)

	var entry xdr.BucketEntry
	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().NoError(err)
	s.Require().Equal(entry, firstEntry)

	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().NoError(err)
	s.Require().Equal(entry, secondEntry)

	err = s.reader.readBucketRecord(stream, emptyHash, &entry)
	s.Require().Equal(io.EOF, err)
}

func TestCheckpointLedgersTestSuite(t *testing.T) {
	suite.Run(t, new(CheckpointLedgersTestSuite))
}

type CheckpointLedgersTestSuite struct {
	suite.Suite
}

// TestNonCheckpointLedger ensures that the reader errors on a non-checkpoint ledger
func (s *CheckpointLedgersTestSuite) TestNonCheckpointLedger() {
	mockArchive := &historyarchive.MockArchive{}
	ledgerSeq := uint32(123456)

	for _, freq := range []uint32{historyarchive.DefaultCheckpointFrequency, 5} {
		mockArchive.
			On("GetCheckpointManager").
			Return(historyarchive.NewCheckpointManager(freq))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		_, err := NewCheckpointChangeReader(ctx, mockArchive, ledgerSeq)
		s.Require().Error(err)
	}
}

func metaEntry(version uint32) xdr.BucketEntry {
	return xdr.BucketEntry{
		Type: xdr.BucketEntryTypeMetaentry,
		MetaEntry: &xdr.BucketMetadata{
			LedgerVersion: xdr.Uint32(version),
		},
	}
}

func entryAccount(t xdr.BucketEntryType, id string, balance uint32) xdr.BucketEntry {
	switch t {
	case xdr.BucketEntryTypeLiveentry, xdr.BucketEntryTypeInitentry:
		return xdr.BucketEntry{
			Type: t,
			LiveEntry: &xdr.LedgerEntry{
				Data: xdr.LedgerEntryData{
					Type: xdr.LedgerEntryTypeAccount,
					Account: &xdr.AccountEntry{
						AccountId: xdr.MustAddress(id),
						Balance:   xdr.Int64(balance),
					},
				},
			},
		}
	case xdr.BucketEntryTypeDeadentry:
		return xdr.BucketEntry{
			Type: xdr.BucketEntryTypeDeadentry,
			DeadEntry: &xdr.LedgerKey{
				Type:    xdr.LedgerEntryTypeAccount,
				Account: &xdr.LedgerKeyAccount{AccountId: xdr.MustAddress(id)},
			},
		}
	default:
		panic("Unknown entry type")
	}
}

func entryCB(t xdr.BucketEntryType, id xdr.Hash, balance xdr.Int64) xdr.BucketEntry {
	switch t {
	case xdr.BucketEntryTypeLiveentry, xdr.BucketEntryTypeInitentry:
		return xdr.BucketEntry{
			Type: t,
			LiveEntry: &xdr.LedgerEntry{
				Data: xdr.LedgerEntryData{
					Type: xdr.LedgerEntryTypeClaimableBalance,
					ClaimableBalance: &xdr.ClaimableBalanceEntry{
						BalanceId: xdr.ClaimableBalanceId{
							Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
							V0:   &id,
						},
						Asset:  xdr.MustNewNativeAsset(),
						Amount: balance,
					},
				},
			},
		}
	case xdr.BucketEntryTypeDeadentry:
		return xdr.BucketEntry{
			Type: xdr.BucketEntryTypeDeadentry,
			DeadEntry: &xdr.LedgerKey{
				Type: xdr.LedgerEntryTypeClaimableBalance,
				ClaimableBalance: &xdr.LedgerKeyClaimableBalance{
					BalanceId: xdr.ClaimableBalanceId{
						Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
						V0:   &id,
					},
				},
			},
		}
	default:
		panic("Unknown entry type")
	}
}

func entryOffer(t xdr.BucketEntryType, seller string, id xdr.Int64) xdr.BucketEntry {
	switch t {
	case xdr.BucketEntryTypeLiveentry, xdr.BucketEntryTypeInitentry:
		return xdr.BucketEntry{
			Type: t,
			LiveEntry: &xdr.LedgerEntry{
				Data: xdr.LedgerEntryData{
					Type: xdr.LedgerEntryTypeOffer,
					Offer: &xdr.OfferEntry{
						OfferId:  id,
						SellerId: xdr.MustAddress(seller),
						Selling:  xdr.MustNewNativeAsset(),
						Buying:   xdr.MustNewCreditAsset("USD", "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7"),
						Amount:   100,
						Price:    xdr.Price{1, 1},
					},
				},
			},
		}
	case xdr.BucketEntryTypeDeadentry:
		return xdr.BucketEntry{
			Type: xdr.BucketEntryTypeDeadentry,
			DeadEntry: &xdr.LedgerKey{
				Type: xdr.LedgerEntryTypeOffer,
				Offer: &xdr.LedgerKeyOffer{
					OfferId:  id,
					SellerId: xdr.MustAddress(seller),
				},
			},
		}
	default:
		panic("Unknown entry type")
	}
}

type errCloser struct {
	io.Reader
	err error
}

func (e errCloser) Close() error { return e.err }

func createInvalidXdrStream(closeError error) *xdr.Stream {
	b := &bytes.Buffer{}
	writeInvalidFrame(b)

	return xdr.NewStream(errCloser{b, closeError})
}

func writeInvalidFrame(b *bytes.Buffer) {
	bufferSize := b.Len()
	err := xdr.MarshalFramed(b, metaEntry(1))
	if err != nil {
		panic(err)
	}
	frameSize := b.Len() - bufferSize
	b.Truncate(bufferSize + frameSize/2)
}

func createXdrStream(entries ...interface{}) *xdr.Stream {
	b := &bytes.Buffer{}
	for _, e := range entries {
		err := xdr.MarshalFramed(b, e)
		if err != nil {
			panic(err)
		}
	}

	return xdrStreamFromBuffer(b)
}

func xdrStreamFromBuffer(b *bytes.Buffer) *xdr.Stream {
	return xdr.NewStream(ioutil.NopCloser(b))
}

// createBucketChannel is a helper that returns next bucket hash in the order of processing.
// This allows to write simpler test code that ensures that mocked calls are in a
// correct order.
func createBucketChannel(buckets historyarchive.BucketList) <-chan (historyarchive.Hash) {
	// 11 levels with 2 buckets each = buffer of 22
	c := make(chan (historyarchive.Hash), 22)

	for i := 0; i < len(buckets); i++ {
		b := buckets[i]

		curr := historyarchive.MustDecodeHash(b.Curr)
		if !curr.IsZero() {
			c <- curr
		}

		snap := historyarchive.MustDecodeHash(b.Snap)
		if !snap.IsZero() {
			c <- snap
		}
	}

	close(c)
	return c
}

var hasExample = `{
    "version": 1,
    "server": "v11.1.0",
    "currentLedger": 24123007,
    "currentBuckets": [
        {
            "curr": "517bea4c6627a688a8ce501febd8c562e737e3d86b29689d9956217640f3c74b",
            "next": {
                "state": 0
            },
            "snap": "75c8c5540a825da61e05ae23d0b0be9d29f2bdb8fdfa550a3f3496f030f62ffd"
        },
        {
            "curr": "5bca6165dbf6832ff4550e67d0e564eca56494acfc9b7fd46c740f4d66c74609",
            "next": {
                "state": 1,
                "output": "75c8c5540a825da61e05ae23d0b0be9d29f2bdb8fdfa550a3f3496f030f62ffd"
            },
            "snap": "b6bad6183a3394087aae3d05ed393c5dcb80e35ed557e2c8935cba855f20dfa5"
        },
        {
            "curr": "56b70bb56fcb27dd05759b00b07bc3c9dc7cc6dbfc9d409cfec2a41d9fd4a1e8",
            "next": {
                "state": 1,
                "output": "cfa973ce4ba1fbdf3b5767e398a5b7b86e30461855d24b7b50dc499eb313b4d0"
            },
            "snap": "974a089a6980bf25d8ad1a6a71370bac2663e9bb14702ba90b9db657464c0b3a"
        },
        {
            "curr": "16742c8e61a4dde3b35179bedbdd7c56e67d03a5faf8973a6094c57e430322df",
            "next": {
                "state": 1,
                "output": "ef39804657a928139750e801c63d1d911532d7d126c80f151ba362f49147972e"
            },
            "snap": "b415a283c5e33d8c425cbb003a86c780f73e8d2016fb5dcc6ba1477e551a2c66"
        },
        {
            "curr": "b081e1c075c9114a6c74cf87a0767ee877f02e88e18a8bf97b8f268ff120ad0d",
            "next": {
                "state": 1,
                "output": "162b859558c7c51c6416f659dbd8d70236c75540196e5d7a5dee2a66744ebbf5"
            },
            "snap": "66f8fb3f36bbe328bbbe14151951891d455ad0fba1d19d05531226c0909a84c7"
        },
        {
            "curr": "822b766e755e83d4ad08a38e86466f47452a2d7c4702295ebd3235332db76a05",
            "next": {
                "state": 1,
                "output": "1c04dc66c3410efc535044f4250c02490627b549f99a8873e4857b2cec4d51c8"
            },
            "snap": "163a49fa560761217710f6bbbf85179514aa7714d373337dde7f200f8d6c623a"
        },
        {
            "curr": "75b77814875529876258760ed6b6f37d81b5a39183812c684b9e3014bb6b8cf6",
            "next": {
                "state": 1,
                "output": "444088f447eb7ea3d397e7098d57c4f63b66912d24c4a26a29bf1dde7a4fdecc"
            },
            "snap": "35472156c463eaf62867c9b229b92e8192e2fe40cf86e269cab65fd0045c996f"
        },
        {
            "curr": "b331675d693bdb4456f409083a1b8cbadbcef977df765ba7d4ddd787800bdc84",
            "next": {
                "state": 1,
                "output": "3d9627fa5ef81486688dc584f52445560a55496d3b961a7664b0e536655179bb"
            },
            "snap": "5a7996730755a90ea5cbd2d726a982f3f3703c3db8bc2a2217bd496b9c0cf3d1"
        },
        {
            "curr": "11f8c2f8e1cb0d47576f74d9e2fa838f5f3a37180907a24a85d0ad8b647862e4",
            "next": {
                "state": 1,
                "output": "6c0133dfd0411f9975c74d792911bb80fc1555830a943249cea6c2a80e5064d1"
            },
            "snap": "48f435285dd96511d0822f7ae1a20e28c6c28019e385313713655fc76fe3bc03"
        },
        {
            "curr": "5f351041761b45f3e725f98bb8b6713873e30ab6c8aee56ba0823d357c7ebd0d",
            "next": {
                "state": 1,
                "output": "264d3a93bc5fff47a968cc53f0f2f50297e5f9015300bbc357cfb8dec30899c6"
            },
            "snap": "4100ad3b1085bd14d1c808ece3b38db97171532d0d11ed5edd57aff0e416e06a"
        },
        {
            "curr": "a4811c9ba9505e421f0015e5fcfd9f5d204ae85b584766759e844ef85db10d47",
            "next": {
                "state": 1,
                "output": "be4ecc289998a40319be24662c88f161f5e78d4be846b083923614573aa17336"
            },
            "snap": "0000000000000000000000000000000000000000000000000000000000000000"
        }
    ]
}`
