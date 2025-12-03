package datastore

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
)

var ErrNoValidLedgerFiles = errors.New("no valid ledger files found on the data store")

// findLatestLedger is a helper function that returns the latest ledger
// found within a list of files. This implementation assumes the
// datastore returns files in reverse lexicographical order, so the first
// matching file in the list is the latest.
func findLatestLedger(ctx context.Context, ds DataStore, options ListFileOptions) (uint32, error) {
	it := LedgerFileIter(ctx, ds, options.StartAfter, "")
	for lf, err := range it {
		if err != nil {
			return 0, err
		}
		return lf.High, nil
	}
	return 0, ErrNoValidLedgerFiles

}

// FindLatestLedgerSequence returns the absolute latest ledger sequence number
// stored in the datastore.
func FindLatestLedgerSequence(ctx context.Context, datastore DataStore) (uint32, error) {
	return findLatestLedger(ctx, datastore, ListFileOptions{})
}

// FindLatestLedgerUpToSequence finds the latest ledger sequence number
// that is less than or equal to a given 'end' sequence.
func FindLatestLedgerUpToSequence(ctx context.Context, datastore DataStore,
	end uint32, schema DataStoreSchema) (uint32, error) {
	if end < 2 {
		return 0, errors.New("end sequence must be greater than or equal to 2")
	}

	startAfter := ""
	boundary := schema.GetSequenceNumberEndBoundary(end)

	// If boundary == math.MaxUint32, startAfter is left as "". Empty string lists all
	// keys and since keys are in reverse numeric order, the highest sequence is listed
	// first, allowing 'findLatestLedger' to return the latest ledger.
	if boundary < math.MaxUint32 {
		startAfter = schema.GetObjectKeyFromSequenceNumber(boundary + 1)
	}

	return findLatestLedger(ctx, datastore, ListFileOptions{StartAfter: startAfter})
}

// FindOldestLedgerSequence finds the oldest existing ledger in the datastore.
// It uses a binary search on the range of all known ledgers (from sequence 2 to the latest)
// to efficiently locate the first existing ledger file.
func FindOldestLedgerSequence(ctx context.Context, datastore DataStore, schema DataStoreSchema) (uint32, error) {
	start := uint32(2)
	end, err := FindLatestLedgerSequence(ctx, datastore)
	if err != nil {
		return 0, err
	}

	if end < start {
		return 0, ErrNoValidLedgerFiles
	}

	var lookupError error
	// The binary search returns the index of the first element for which the function returns true.
	// In this case, we are searching for the first ledger that exists.
	i := sort.Search(int(end-start+1), func(index int) bool {
		if lookupError != nil {
			return false
		}

		ledgerSequence := start + uint32(index)
		objectKey := schema.GetObjectKeyFromSequenceNumber(ledgerSequence)

		if exists, err := datastore.Exists(ctx, objectKey); err != nil {
			lookupError = fmt.Errorf("error while checking existence of object key %v: %w", objectKey, err)
			return false
		} else {
			return exists
		}
	})

	if lookupError != nil {
		return 0, lookupError
	}

	if i < int(end-start+1) {
		return start + uint32(i), nil
	}

	return 0, ErrNoValidLedgerFiles
}
