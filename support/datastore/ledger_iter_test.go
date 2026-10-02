package datastore

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestParseRangeFromFilename_SingleAndRange(t *testing.T) {
	low, high, err := ParseRangeFromObjectKey("0000000A--10.xdr.zst")
	require.NoError(t, err)
	require.Equal(t, uint32(10), low)
	require.Equal(t, uint32(10), high)

	low, high, err = ParseRangeFromObjectKey("00000014--11-20.xdr.zst")
	require.NoError(t, err)
	require.Equal(t, uint32(11), low)
	require.Equal(t, uint32(20), high)
}

func TestObjectKeyRoundTrip_SimpleSchemas(t *testing.T) {
	cases := []DataStoreSchema{
		{LedgersPerFile: 1, FilesPerPartition: 1},
		{LedgersPerFile: 16, FilesPerPartition: 1},
		{LedgersPerFile: 64, FilesPerPartition: 4},
	}

	ledgerSeqs := []uint32{1, 2, 15, 16, 17, 63, 64, 65, 1000}

	for _, schema := range cases {
		t.Run(fmt.Sprintf("lpf=%d,fpp=%d", schema.LedgersPerFile, schema.FilesPerPartition), func(t *testing.T) {
			for _, seq := range ledgerSeqs {
				start := schema.GetSequenceNumberStartBoundary(seq)
				end := schema.GetSequenceNumberEndBoundary(seq)

				key := schema.GetObjectKeyFromSequenceNumber(seq)
				low, high, err := ParseRangeFromObjectKey(key)
				require.NoError(t, err)
				require.Equal(t, start, low)
				require.Equal(t, end, high)
			}
		})
	}
}

func TestLedgerKeyIter_Pagination(t *testing.T) {
	ctx := context.Background()
	ds := new(MockDataStore)

	// Page 1
	ds.On("ListFilePaths", mock.Anything, mock.MatchedBy(func(o ListFileOptions) bool {
		return o.StartAfter == ""
	})).Return([]string{
		"0000000A--10.xdr.zst",
		"00000014--11-20.xdr.zst",
	}, nil).Once()

	// Page 2 (StartAfter = last of page 1)
	ds.On("ListFilePaths", mock.Anything, mock.MatchedBy(func(o ListFileOptions) bool {
		return o.StartAfter == "00000014--11-20.xdr.zst"
	})).Return([]string{
		"0000001E--21-30.xdr.zst",
	}, nil).Once()

	// Empty page terminates
	ds.On("ListFilePaths", mock.Anything, mock.Anything).
		Return([]string{}, nil).Maybe()

	var all []LedgerFile
	for cur, err := range LedgerFileIter(ctx, ds, "", "") {
		require.NoError(t, err)
		all = append(all, cur)
	}

	require.Len(t, all, 3)
	keys := []string{all[0].Key, all[1].Key, all[2].Key}
	exp := []string{"0000000A--10.xdr.zst", "00000014--11-20.xdr.zst", "0000001E--21-30.xdr.zst"}
	require.Equal(t, exp, keys)

	ds.AssertExpectations(t)
}

func TestParseRangeFromFilename(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		low     uint32
		high    uint32
		wantErr bool
	}{
		{"single ledger", "ABCDEF01--7.xdr.foo", 7, 7, false},
		{"range ok", "ABCDEF01--42-100.xdr.bar", 42, 100, false},
		{"low==high ok", "00000000--5-5.xdr.baz", 5, 5, false},
		{"invalid no marker", "weirdname.xdr", 0, 0, true},
		{"low>high", "ABCDEF01--10-9.xdr.foo", 0, 0, true},
		{"bad low", "ABCDEF01--notnum-9.xdr.foo", 0, 0, true},
		{"bad high", "ABCDEF01--10-notnum.xdr.foo", 0, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lo, hi, err := ParseRangeFromObjectKey(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if lo != tc.low || hi != tc.high {
				t.Fatalf("got %d-%d, want %d-%d", lo, hi, tc.low, tc.high)
			}
		})
	}
}

func TestLedgerFileIter_EmptyDatastore(t *testing.T) {
	ctx := context.Background()
	ds := new(MockDataStore)

	ds.On("ListFilePaths", mock.Anything, mock.Anything).
		Return([]string{}, nil).Once()

	var count int
	for _, err := range LedgerFileIter(ctx, ds, "", "") {
		require.NoError(t, err)
		count++
	}

	require.Equal(t, 0, count)
	ds.AssertExpectations(t)
}

func TestLedgerKeyIter_StopAfter(t *testing.T) {
	ctx := context.Background()
	ds := new(MockDataStore)

	// Single page returned; iterator will stop early when p > StopAfter
	page := []string{
		"0000000A--10.xdr.zst",
		"00000014--11-20.xdr.zst",
		"0000001E--21-30.xdr.zst",
	}
	sort.Strings(page)
	ds.On("ListFilePaths", mock.Anything, mock.Anything).
		Return(page, nil).Once()
	ds.On("ListFilePaths", mock.Anything, mock.Anything).
		Return([]string{}, nil).Maybe()

	stopAfter := "00000014--11-20.xdr.zst" // stop once > this
	var all []LedgerFile
	for cur, err := range LedgerFileIter(ctx, ds, "", stopAfter) {
		require.NoError(t, err)
		all = append(all, cur)
	}
	// Should include entries <= StopAfter
	require.Len(t, all, 2)

	ds.AssertExpectations(t)
}

func TestLedgerFileIter_InvalidRange_YieldsError(t *testing.T) {
	ctx := context.Background()
	ds := new(MockDataStore)
	iter := LedgerFileIter(ctx, ds, "zzz", "aaa") // startAfter > stopAfter
	for lf, err := range iter {
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid range")
		require.Zero(t, lf)
	}
	ds.AssertExpectations(t)
}

func TestLedgerFileIter_NonAdvancingCursor_YieldsError(t *testing.T) {
	ctx := context.Background()
	ds := new(MockDataStore)

	// A directory placeholder object trims to an empty key. If the backend
	// returned it, the cursor would never advance; the iterator must error
	// out instead of looping.
	ds.On("ListFilePaths", mock.Anything, mock.MatchedBy(func(o ListFileOptions) bool {
		return o.StartAfter == ""
	})).Return([]string{""}, nil).Once()

	var count int
	var gotErr error
	for _, err := range LedgerFileIter(ctx, ds, "", "") {
		if err != nil {
			gotErr = err
			break
		}
		count++
	}

	require.Equal(t, 0, count)
	require.ErrorContains(t, gotErr, "did not advance")
	ds.AssertExpectations(t)
}
