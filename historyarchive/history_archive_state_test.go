// Copyright 2016 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package historyarchive

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalState(t *testing.T) {
	var jsonBlob = []byte(`{
		"version": 1,
		"server": "v0.4.0-34-g2f015f6",
		"currentLedger": 2113919,
		"currentBuckets": [
			{
				"curr": "0000000000000000000000000000000000000000000000000000000000000000",
				"next": {
					"state": 0
				},
				"snap": "0000000000000000000000000000000000000000000000000000000000000000"
			},
			{
				"curr": "0000000000000000000000000000000000000000000000000000000000000000",
				"next": {
					"state": 1,
					"output": "0000000000000000000000000000000000000000000000000000000000000000"
				},
				"snap": "0000000000000000000000000000000000000000000000000000000000000000"
			}
		 ]
	}`)

	var state HistoryArchiveState
	err := json.Unmarshal(jsonBlob, &state)
	if err != nil {
		t.Error(err)
	} else if state.CurrentLedger != 2113919 {
		t.Error(state)
	}
}

func TestHashValidation(t *testing.T) {
	for _, testCase := range []struct {
		inputFile    string
		expectedHash string
	}{
		{
			// This is real bucket hash list for pubnet's ledger: 24088895
			// https://horizon.stellar.org/ledgers/24088895
			// http://history.stellar.org/prd/core-live/core_live_001/history/01/6f/91/history-016f913f.json
			inputFile:    "testdata/historyV1.json",
			expectedHash: "fc5fe47af3f5a9b18b278f2a7edbbc641e1934bf68131d9aa5ab7aebb4aa8aa3",
		},
		{
			// taken from https://github.com/stellar/stellar-core/pull/4623/files#diff-f0bf7c78e09501debc1cd55b61977f9328f7f7b872138ccbd7e0e0a2b1cb91bc
			inputFile:    "testdata/historyV2.json",
			expectedHash: "136fa2ef979150e7d5fdbeeaf36f61c9af6016fe5b0f34ebc842301dfca95ebc",
		},
	} {
		t.Run(testCase.inputFile, func(t *testing.T) {
			inputBytes, err := os.ReadFile(testCase.inputFile)
			require.NoError(t, err)

			var state HistoryArchiveState
			require.NoError(t, json.Unmarshal(inputBytes, &state))

			expectedHash, err := hex.DecodeString(testCase.expectedHash)
			require.NoError(t, err)

			hash, err := state.BucketListHash()
			require.NoError(t, err)
			assert.Equal(t, expectedHash, hash[:])
		})
	}
}

func countBuckets(buckets BucketList) int {
	count := 0
	for _, level := range buckets {
		for _, bs := range []string{level.Curr, level.Snap, level.Next.Output} {
			if bs != "" && bs != "0000000000000000000000000000000000000000000000000000000000000000" {
				count++
			}
		}
	}
	return count
}

func TestBucketsIncludesHotArchiveBuckets(t *testing.T) {
	for _, testCase := range []struct {
		inputFile       string
		expectedVersion int
	}{
		{
			inputFile:       "testdata/historyV1.json",
			expectedVersion: 1,
		},
		{
			inputFile:       "testdata/historyV2.json",
			expectedVersion: HistoryArchiveStateVersionForProtocol23,
		},
	} {
		t.Run(testCase.inputFile, func(t *testing.T) {
			inputBytes, err := os.ReadFile(testCase.inputFile)
			require.NoError(t, err)

			var state HistoryArchiveState
			require.NoError(t, json.Unmarshal(inputBytes, &state))

			// Verify the version
			assert.Equal(t, testCase.expectedVersion, state.Version)

			// Verify that the Buckets() method includes hot archive buckets for V2
			buckets, err := state.Buckets()
			require.NoError(t, err)

			currentBucketCount := countBuckets(state.CurrentBuckets)
			hotArchiveBucketCount := countBuckets(state.HotArchiveBuckets)

			if testCase.expectedVersion >= HistoryArchiveStateVersionForProtocol23 {
				expectedTotal := currentBucketCount + hotArchiveBucketCount
				assert.Equal(t, expectedTotal, len(buckets),
					"Buckets() should include both currentBuckets and hotArchiveBuckets for V2+")
			} else {
				assert.Equal(t, 0, hotArchiveBucketCount,
					"V1 should have 0 hot archive buckets")
				assert.Equal(t, currentBucketCount, len(buckets),
					"Buckets() should only include currentBuckets for V1")
			}
		})
	}
}

func TestBucketListHashRejectsHashesOfTheWrongLength(t *testing.T) {
	inputBytes, err := os.ReadFile("testdata/historyV1.json")
	require.NoError(t, err)
	var state HistoryArchiveState
	require.NoError(t, json.Unmarshal(inputBytes, &state))

	for _, testCase := range []struct {
		name string
		curr string
	}{
		{"31 bytes", "2a4416e7f3e301c2fc1078dce0e1dd109b8ae6d3958942b91b447f24014a7b"},
		{"33 bytes", "2a4416e7f3e301c2fc1078dce0e1dd109b8ae6d3958942b91b447f24014a7b5c00"},
		{"1 byte", "ff"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			buckets := state.CurrentBuckets
			buckets[0].Curr = testCase.curr
			_, err := buckets.Hash()
			require.ErrorContains(t, err, "unexpected hash size")
		})
	}

	t.Run("empty", func(t *testing.T) {
		buckets := state.CurrentBuckets
		buckets[0].Curr = ""
		_, err := buckets.Hash()
		require.NoError(t, err)
	})
}
