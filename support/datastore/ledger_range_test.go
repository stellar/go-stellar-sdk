package datastore

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var schema = DataStoreSchema{FilesPerPartition: 10, LedgersPerFile: 1}

func TestFindLatestLedger(t *testing.T) {
	ctx := context.Background()
	mds := new(MockDataStore)

	end := uint32(10)
	name := schema.GetObjectKeyFromSequenceNumber(end)

	mds.On("ListFilePaths", ctx, ListFileOptions{}).
		Return([]string{name}, nil).Once()

	got, err := findLatestLedger(ctx, mds, ListFileOptions{})
	assert.NoError(t, err)
	assert.Equal(t, end, got)

	mds.AssertExpectations(t)
}

func TestFindLatestLedger_Success(t *testing.T) {
	ctx := context.Background()
	mds := new(MockDataStore)

	end := uint32(10)
	latestKey := schema.GetObjectKeyFromSequenceNumber(end)
	key1 := schema.GetObjectKeyFromSequenceNumber(5)
	key2 := schema.GetObjectKeyFromSequenceNumber(2)

	// First entry does not match ledgerFilenameRe; should be skipped.
	nonMatching := "misc/README.txt"

	mds.On("ListFilePaths", ctx, ListFileOptions{}).
		Return([]string{nonMatching, latestKey, key1, key2}, nil).Once()

	got, err := findLatestLedger(ctx, mds, ListFileOptions{})
	require.NoError(t, err)
	require.Equal(t, end, got)

	mds.AssertExpectations(t)
}

func TestFindLatestLedger_ListError(t *testing.T) {
	ctx := context.Background()
	mds := new(MockDataStore)

	mds.On("ListFilePaths", ctx, ListFileOptions{}).
		Return(nil, errors.New("boom")).Once()

	_, err := findLatestLedger(ctx, mds, ListFileOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "boom")

	mds.AssertExpectations(t)
}

func TestFindLatestLedger_NoMatchingFiles(t *testing.T) {
	ctx := context.Background()
	mds := new(MockDataStore)

	// Non-matching base names are skipped; ErrNoValidLedgerFiles
	mds.On("ListFilePaths", ctx, ListFileOptions{}).
		Return([]string{"/bucket/ledgers/README.txt", "/bucket/foo/bar.txt"}, nil).Once()
	mds.On("ListFilePaths", ctx, ListFileOptions{StartAfter: "/bucket/foo/bar.txt"}).
		Return([]string{}, nil).Once()

	_, err := FindLatestLedgerSequence(ctx, mds)
	assert.ErrorIs(t, err, ErrNoValidLedgerFiles)

	mds.AssertExpectations(t)
}

func TestFindLatestLedgerUpToSequence(t *testing.T) {
	ctx := context.Background()
	mds := new(MockDataStore)

	end := uint32(50)
	name := schema.GetObjectKeyFromSequenceNumber(50)

	mds.On("ListFilePaths", ctx, ListFileOptions{StartAfter: schema.GetObjectKeyFromSequenceNumber(end + 1)}).
		Return([]string{name}, nil).Once()

	got, err := FindLatestLedgerUpToSequence(ctx, mds, end, schema)
	assert.NoError(t, err)
	assert.Equal(t, uint32(50), got)

	mds.AssertExpectations(t)
}

func TestFindLatestLedgerUpToSequence_MultipleLedgersPerFile(t *testing.T) {
	ctx := context.Background()
	mds := new(MockDataStore)

	testSchema := DataStoreSchema{
		LedgersPerFile:    10,
		FilesPerPartition: 10,
	}

	end := uint32(50)
	name := schema.GetObjectKeyFromSequenceNumber(50)

	mds.On("ListFilePaths", ctx, ListFileOptions{StartAfter: "FFFFFFFF--0-99/FFFFFFC3--60-69.xdr.zst"}).
		Return([]string{name}, nil).Once()

	got, err := FindLatestLedgerUpToSequence(ctx, mds, end, testSchema)
	assert.NoError(t, err)
	assert.Equal(t, uint32(50), got)

	mds.AssertExpectations(t)
}

func TestFindLatestLedgerUpToSequence_InvalidEnd(t *testing.T) {
	ctx := context.Background()
	mds := new(MockDataStore)

	// ledger sequence < 2 is invalid
	_, err := FindLatestLedgerUpToSequence(ctx, mds, 1, schema)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "end sequence must be greater")

	mds.AssertExpectations(t)
}

func TestFindLatestLedgerUpToSequence_MaxSequence(t *testing.T) {
	ctx := context.Background()
	mds := new(MockDataStore)

	end := uint32(math.MaxUint32)
	name := schema.GetObjectKeyFromSequenceNumber(end)

	mds.On("ListFilePaths", ctx, ListFileOptions{StartAfter: ""}).
		Return([]string{name}, nil).Once()

	got, err := FindLatestLedgerUpToSequence(ctx, mds, end, schema)
	assert.NoError(t, err)
	assert.Equal(t, uint32(math.MaxUint32), got)

	mds.AssertExpectations(t)
}

func TestFindOldestLedgerSequence_InvalidLatest(t *testing.T) {
	ctx := context.Background()

	type tc struct {
		name        string
		latest      uint32
		exists      map[uint32]bool
		want        uint32
		errContains string
		errIs       error
	}

	tests := []tc{
		{
			name:   "latest=1_invalid",
			latest: 1,
			errIs:  ErrNoValidLedgerFiles,
		},
		{
			name:   "latest=2_valid",
			latest: 2,
			exists: map[uint32]bool{2: true},
			want:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mds := new(MockDataStore)

			latestKey := schema.GetObjectKeyFromSequenceNumber(tt.latest)

			mds.On("ListFilePaths", ctx, ListFileOptions{}).
				Return([]string{latestKey}, nil).Once()

			for seq, ok := range tt.exists {
				key := schema.GetObjectKeyFromSequenceNumber(seq)
				mds.On("Exists", ctx, key).Return(ok, nil).Maybe()
			}

			got, err := FindOldestLedgerSequence(ctx, mds, schema)

			switch {
			case tt.errContains != "":
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errContains)
			case tt.errIs != nil:
				require.ErrorIs(t, err, tt.errIs)
			default:
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			}

			mds.AssertExpectations(t)
		})
	}
}

func TestFindOldestLedgerSequence_FindsFirstExisting(t *testing.T) {
	ctx := context.Background()
	mds := new(MockDataStore)

	// Latest = 6 (so the search range is [2..6]); we expect oldest existing = 5.
	latest := uint32(6)
	latestKey := schema.GetObjectKeyFromSequenceNumber(latest)

	mds.On("ListFilePaths", ctx, ListFileOptions{}).
		Return([]string{latestKey}, nil).Once()

	for seq := uint32(2); seq <= 6; seq++ {
		key := schema.GetObjectKeyFromSequenceNumber(seq)
		exists := seq == 5 || seq == 6 // only 5 and 6 exist
		mds.On("Exists", ctx, key).Return(exists, nil).Maybe()
	}

	got, err := FindOldestLedgerSequence(ctx, mds, schema)

	assert.NoError(t, err)
	assert.Equal(t, uint32(5), got)

	mds.AssertExpectations(t)
}

func TestFindOldestLedgerSequence_ExistsError(t *testing.T) {
	ctx := context.Background()
	mds := new(MockDataStore)

	latest := uint32(3)
	latestName := schema.GetObjectKeyFromSequenceNumber(latest)

	mds.On("ListFilePaths", ctx, ListFileOptions{}).
		Return([]string{latestName}, nil).Once()

	mds.On("Exists", ctx, mock.Anything).
		Return(false, errors.New("check failed")).Once()

	_, err := FindOldestLedgerSequence(ctx, mds, schema)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error while checking existence")

	mds.AssertExpectations(t)
}

func TestFindOldestLedgerSequence_NoLedgersExist(t *testing.T) {
	ctx := context.Background()
	mds := new(MockDataStore)

	// Latest = 2 (so search range is [2..2]); Exists returns false.
	latest := uint32(2)
	latestName := schema.GetObjectKeyFromSequenceNumber(latest)

	mds.On("ListFilePaths", ctx, ListFileOptions{}).
		Return([]string{latestName}, nil).Once()

	mds.On("Exists", ctx, mock.Anything).
		Return(false, nil).Maybe()

	_, err := FindOldestLedgerSequence(ctx, mds, schema)
	assert.ErrorIs(t, err, ErrNoValidLedgerFiles)

	mds.AssertExpectations(t)
}

func TestFindOldestLedgerSequence_EmptyListFilePaths(t *testing.T) {
	ctx := context.Background()
	mds := new(MockDataStore)

	mds.On("ListFilePaths", ctx, ListFileOptions{}).
		Return([]string{}, nil).Once()

	_, err := FindOldestLedgerSequence(ctx, mds, schema)
	assert.ErrorIs(t, err, ErrNoValidLedgerFiles)

	mds.AssertExpectations(t)
}

func TestFindOldestLedgerSequence_InvalidLatestLedger(t *testing.T) {
	ctx := context.Background()
	mds := new(MockDataStore)

	latest := uint32(1)
	latestName := schema.GetObjectKeyFromSequenceNumber(latest)

	mds.On("ListFilePaths", ctx, ListFileOptions{}).
		Return([]string{latestName}, nil).Once()

	mds.On("Exists", ctx, mock.Anything).
		Return(false, nil).Maybe()

	_, err := FindOldestLedgerSequence(ctx, mds, schema)
	assert.ErrorIs(t, err, ErrNoValidLedgerFiles)

	mds.AssertExpectations(t)
}

func TestFindOldestLedgerSequence_LargeRange(t *testing.T) {
	ctx := context.Background()
	mds := new(MockDataStore)

	// latest = 150; exists 37..50; oldest = 37
	latest := uint32(150)
	latestKey := schema.GetObjectKeyFromSequenceNumber(latest)

	mds.On("ListFilePaths", ctx, ListFileOptions{}).
		Return([]string{latestKey}, nil).Once()

	for seq := uint32(2); seq <= latest; seq++ {
		key := schema.GetObjectKeyFromSequenceNumber(seq)
		exists := seq >= 37
		mds.On("Exists", ctx, key).Return(exists, nil).Maybe()
	}

	got, err := FindOldestLedgerSequence(ctx, mds, schema)
	assert.NoError(t, err)
	assert.Equal(t, uint32(37), got)

	mds.AssertExpectations(t)
}
