package ingest

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/stellar/go-stellar-sdk/historyarchive"
	"github.com/stellar/go-stellar-sdk/xdr"
)

var hasWithHotArchiveExample = `{
    "version": 1,
    "server": "v11.1.0",
    "currentLedger": 24123007,
    "hotArchiveBuckets": [
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

func hotArchiveMetaEntry(version uint32) xdr.HotArchiveBucketEntry {
	listType := xdr.BucketListTypeHotArchive
	return xdr.HotArchiveBucketEntry{
		Type: xdr.HotArchiveBucketEntryTypeHotArchiveMetaentry,
		MetaEntry: &xdr.BucketMetadata{
			LedgerVersion: xdr.Uint32(version),
			Ext: xdr.BucketMetadataExt{
				V:              1,
				BucketListType: &listType,
			},
		},
	}
}

func archivedBucketEntry(id string, balance uint32) xdr.HotArchiveBucketEntry {
	return xdr.HotArchiveBucketEntry{
		Type: xdr.HotArchiveBucketEntryTypeHotArchiveArchived,
		ArchivedEntry: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress(id),
					Balance:   xdr.Int64(balance),
				},
			},
		},
	}
}

func archivedLiveEntry(id string) xdr.HotArchiveBucketEntry {
	return xdr.HotArchiveBucketEntry{
		Type: xdr.HotArchiveBucketEntryTypeHotArchiveLive,
		Key: &xdr.LedgerKey{
			Type: xdr.LedgerEntryTypeAccount,
			Account: &xdr.LedgerKeyAccount{
				AccountId: xdr.MustAddress(id),
			},
		},
	}
}

func TestHotArchiveIteratorTestSuite(t *testing.T) {
	suite.Run(t, new(HotArchiveIteratorTestSuite))
}

type HotArchiveIteratorTestSuite struct {
	suite.Suite
	mockArchive *historyarchive.MockArchive
	has         historyarchive.HistoryArchiveState
	ledgerSeq   uint32
}

func (h *HotArchiveIteratorTestSuite) SetupTest() {
	require.NoError(h.T(), json.Unmarshal([]byte(hasWithHotArchiveExample), &h.has))

	h.mockArchive = &historyarchive.MockArchive{}
	h.ledgerSeq = 24123007

	h.mockArchive.
		On("GetCheckpointHAS", h.ledgerSeq).
		Return(h.has, nil)

	// BucketExists should be called 21 times (11 levels, last without `snap`)
	h.mockArchive.
		On("BucketExists", mock.AnythingOfType("historyarchive.Hash")).
		Return(true, nil).Times(21)

	// BucketSize should be called 21 times (11 levels, last without `snap`)
	h.mockArchive.
		On("BucketSize", mock.AnythingOfType("historyarchive.Hash")).
		Return(int64(100), nil).Times(21)

	h.mockArchive.
		On("GetCheckpointManager").
		Return(historyarchive.NewCheckpointManager(
			historyarchive.DefaultCheckpointFrequency))
}

func (h *HotArchiveIteratorTestSuite) TearDownTest() {
	h.mockArchive.AssertExpectations(h.T())
}

func (h *HotArchiveIteratorTestSuite) TestIteration() {
	curr1 := createXdrStream(
		hotArchiveMetaEntry(24),
		archivedBucketEntry("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 1),
		archivedBucketEntry("GALPCCZN4YXA3YMJHKL6CVIECKPLJJCTVMSNYWBTKJW4K5HQLYLDMZTB", 100),
		archivedLiveEntry("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H"),
	)

	snap1 := createXdrStream(
		hotArchiveMetaEntry(24),
		archivedBucketEntry("GALPCCZN4YXA3YMJHKL6CVIECKPLJJCTVMSNYWBTKJW4K5HQLYLDMZTB", 50),
		archivedLiveEntry("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		archivedBucketEntry("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H", 600),
	)

	nextBucket := createBucketChannel(h.has.HotArchiveBuckets)

	// Return curr1 and snap1 stream for the first two bucket...
	h.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(curr1, nil).Once()

	h.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(snap1, nil).Once()

	// ...and empty streams for the rest of the buckets.
	for hash := range nextBucket {
		h.mockArchive.
			On("GetXdrStreamForHash", hash).
			Return(createXdrStream(), nil).Once()
	}

	expectedAccounts := []string{
		"GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML",
		"GALPCCZN4YXA3YMJHKL6CVIECKPLJJCTVMSNYWBTKJW4K5HQLYLDMZTB",
	}
	expectedBalances := []uint32{1, 100}

	i := 0
	for ledgerEntry, err := range NewHotArchiveIterator(
		context.Background(),
		h.mockArchive,
		h.ledgerSeq,
		DisableBucketListValidation,
	) {
		h.Require().NoError(err)
		h.Require().Less(i, len(expectedAccounts))
		h.Require().Equal(expectedAccounts[i], ledgerEntry.Data.Account.AccountId.Address())
		h.Require().Equal(expectedBalances[i], uint32(ledgerEntry.Data.Account.Balance))
		i++
	}
}

// TestFilter exercises WithFilter for the hot archive iterator by ignoring a
// HotArchiveLive key in a newer bucket so that an older ArchivedEntry is yielded.
func (h *HotArchiveIteratorTestSuite) TestFilter() {
	curr1 := createXdrStream(
		hotArchiveMetaEntry(24),
		// Newest bucket contains only a live key for A
		archivedLiveEntry("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
		archivedBucketEntry("GALPCCZN4YXA3YMJHKL6CVIECKPLJJCTVMSNYWBTKJW4K5HQLYLDMZTB", 100),
	)

	snap1 := createXdrStream(
		hotArchiveMetaEntry(24),
		// Older bucket contains the archived entry we expect to receive
		archivedBucketEntry("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 50),
	)

	nextBucket := createBucketChannel(h.has.HotArchiveBuckets)

	// Return curr1 and snap1 for the first two buckets...
	h.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(curr1, nil).Once()
	h.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(snap1, nil).Once()

	// ...and empty streams for the rest of the buckets.
	for hash := range nextBucket {
		h.mockArchive.
			On("GetXdrStreamForHash", hash).
			Return(createXdrStream(), nil).Once()
	}

	i := 0
	for ledgerEntry, err := range NewHotArchiveIterator(
		context.Background(),
		h.mockArchive,
		h.ledgerSeq,
		DisableBucketListValidation,
		WithFilter(
			// accept all archived entries
			func(le xdr.LedgerEntry) bool {
				if le.Data.Type != xdr.LedgerEntryTypeAccount {
					return true
				}
				return le.Data.Account.AccountId.Address() != "GALPCCZN4YXA3YMJHKL6CVIECKPLJJCTVMSNYWBTKJW4K5HQLYLDMZTB"
			},
			// ignore the live key for our target account so the older archived entry is not suppressed
			func(key xdr.LedgerKey) bool {
				if key.Type != xdr.LedgerEntryTypeAccount {
					return true
				}
				return key.Account.AccountId.Address() != "GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"
			},
		),
	) {
		h.Require().NoError(err)
		h.Require().Zero(i)
		h.Require().Equal("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", ledgerEntry.Data.Account.AccountId.Address())
		h.Require().Equal(uint32(50), uint32(ledgerEntry.Data.Account.Balance))
		i++
	}
}

func (h *HotArchiveIteratorTestSuite) TestMetaEntryNotFirst() {
	curr1 := createXdrStream(
		archivedBucketEntry("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 1),
		hotArchiveMetaEntry(24),
	)

	nextBucket := createBucketChannel(h.has.HotArchiveBuckets)

	// Return curr1 stream for the first bucket...
	h.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(curr1, nil).Once()

	// ...and empty streams for the rest of the buckets.
	for hash := range nextBucket {
		h.mockArchive.
			On("GetXdrStreamForHash", hash).
			Return(createXdrStream(), nil).Maybe()
	}

	i := 0
	for _, err := range NewHotArchiveIterator(
		context.Background(),
		h.mockArchive,
		h.ledgerSeq,
		DisableBucketListValidation,
	) {
		if i == 0 {
			h.Require().ErrorContains(err, "METAENTRY not the first entry in the bucket hash")
		} else {
			h.Require().FailNow("expected at most 1 element")
		}
		i++
	}
}

func (h *HotArchiveIteratorTestSuite) TestMissingBucketListType() {
	curr1 := createXdrStream(
		xdr.HotArchiveBucketEntry{
			Type: xdr.HotArchiveBucketEntryTypeHotArchiveMetaentry,
			MetaEntry: &xdr.BucketMetadata{
				LedgerVersion: xdr.Uint32(24),
			},
		},
		archivedBucketEntry("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 1),
	)

	nextBucket := createBucketChannel(h.has.HotArchiveBuckets)

	// Return curr1 stream for the first bucket...
	h.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(curr1, nil).Once()

	// ...and empty streams for the rest of the buckets.
	for hash := range nextBucket {
		h.mockArchive.
			On("GetXdrStreamForHash", hash).
			Return(createXdrStream(), nil).Maybe()
	}

	i := 0
	for _, err := range NewHotArchiveIterator(
		context.Background(),
		h.mockArchive,
		h.ledgerSeq,
		DisableBucketListValidation,
	) {
		h.Require().Zero(i)
		h.Require().ErrorContains(err, "METAENTRY missing bucket list type")
		i++
	}
}

func (h *HotArchiveIteratorTestSuite) TestInvalidBucketListType() {
	listType := xdr.BucketListTypeLive
	curr1 := createXdrStream(
		xdr.HotArchiveBucketEntry{
			Type: xdr.HotArchiveBucketEntryTypeHotArchiveMetaentry,
			MetaEntry: &xdr.BucketMetadata{
				LedgerVersion: xdr.Uint32(24),
				Ext: xdr.BucketMetadataExt{
					V:              1,
					BucketListType: &listType,
				},
			},
		},
		archivedBucketEntry("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML", 1),
	)

	nextBucket := createBucketChannel(h.has.HotArchiveBuckets)

	// Return curr1 stream for the first bucket...
	h.mockArchive.
		On("GetXdrStreamForHash", <-nextBucket).
		Return(curr1, nil).Once()

	// ...and empty streams for the rest of the buckets.
	for hash := range nextBucket {
		h.mockArchive.
			On("GetXdrStreamForHash", hash).
			Return(createXdrStream(), nil).Maybe()
	}

	i := 0
	for _, err := range NewHotArchiveIterator(
		context.Background(),
		h.mockArchive,
		h.ledgerSeq,
		DisableBucketListValidation,
	) {
		h.Require().Zero(i)
		h.Require().ErrorContains(err, "expected bucket list type to be hot-")
		i++
	}
}
