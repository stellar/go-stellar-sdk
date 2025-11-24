// Copyright 2016 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package historyarchive

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const NumLevels = 11

const HistoryArchiveStateVersionForProtocol23 = 2

type BucketList [NumLevels]struct {
	Curr string `json:"curr"`
	Snap string `json:"snap"`
	Next struct {
		State  uint32 `json:"state"`
		Output string `json:"output,omitempty"`
	} `json:"next"`
}

func (b BucketList) Hash() (xdr.Hash, error) {
	var total []byte

	for i, level := range b {
		curr, err := hex.DecodeString(level.Curr)
		if err != nil {
			return xdr.Hash{}, errors.Wrap(err, fmt.Sprintf("Error decoding hex of %d.curr", i))
		}
		snap, err := hex.DecodeString(level.Snap)
		if err != nil {
			return xdr.Hash{}, errors.Wrap(err, fmt.Sprintf("Error decoding hex of %d.snap", i))
		}
		both := append(curr, snap...)
		bothHash := sha256.Sum256(both)
		total = append(total, bothHash[:]...)
	}

	return sha256.Sum256(total), nil
}

type HistoryArchiveState struct {
	Version       int    `json:"version"`
	Server        string `json:"server"`
	CurrentLedger uint32 `json:"currentLedger"`
	// NetworkPassphrase was added in Stellar-Core v14.1.0. Can be missing
	// in HAS created by previous versions.
	NetworkPassphrase string     `json:"networkPassphrase"`
	CurrentBuckets    BucketList `json:"currentBuckets"`
	HotArchiveBuckets BucketList `json:"hotArchiveBuckets"`
}

func summarizeBucketList(buckets BucketList) (string, int, error) {
	summ := ""
	nz := 0
	for _, b := range buckets {
		state := '_'
		for _, bs := range []string{
			b.Curr, b.Snap, b.Next.Output,
		} {
			// Ignore empty values
			if bs == "" {
				continue
			}

			h, err := DecodeHash(bs)
			if err != nil {
				return summ, nz, err
			}

			if !h.IsZero() {
				state = '#'
			}
		}
		if state != '_' {
			nz += 1
		}
		summ += string(state)
	}
	return summ, nz, nil
}

func (h *HistoryArchiveState) LevelSummary() (string, int, error) {
	currentSummary, currentNz, err := summarizeBucketList(h.CurrentBuckets)
	if err != nil {
		return "", 0, err
	}

	// For V2+, include hot archive buckets
	if h.Version >= HistoryArchiveStateVersionForProtocol23 {
		hotSummary, hotNz, err := summarizeBucketList(h.HotArchiveBuckets)
		if err != nil {
			return "", 0, err
		}
		return currentSummary + "|" + hotSummary, currentNz + hotNz, nil
	}

	return currentSummary, currentNz, nil
}

func extractBucketHashes(buckets BucketList) ([]Hash, error) {
	var result []Hash
	for _, b := range buckets {
		for _, bs := range []string{b.Curr, b.Snap, b.Next.Output} {
			// Ignore empty values
			if bs == "" {
				continue
			}

			h, err := DecodeHash(bs)
			if err != nil {
				return result, err
			}
			if !h.IsZero() {
				result = append(result, h)
			}
		}
	}
	return result, nil
}

// Returns all Buckets reference by the HistoryArchiveState. This includes
// both the live Buckets and hot archive Buckets (for HAS version 2+).
func (h *HistoryArchiveState) Buckets() ([]Hash, error) {
	r := []Hash{}

	// Extract current buckets
	currentBuckets, err := extractBucketHashes(h.CurrentBuckets)
	if err != nil {
		return r, err
	}
	r = append(r, currentBuckets...)

	// Include hot archive buckets for version 2+ (protocol 23+)
	if h.Version >= HistoryArchiveStateVersionForProtocol23 {
		hotArchiveBuckets, err := extractBucketHashes(h.HotArchiveBuckets)
		if err != nil {
			return r, err
		}
		r = append(r, hotArchiveBuckets...)
	}

	return r, nil
}

// BucketListHash calculates the hash of bucket list in the HistoryArchiveState.
// This can be later compared with LedgerHeader.BucketListHash of the checkpoint
// ledger to ensure data in history archive has not been changed by a malicious
// actor.
// Warning: Ledger header should be fetched from a trusted (!) stellar-core
// instead of ex. history archives!
func (h *HistoryArchiveState) BucketListHash() (xdr.Hash, error) {
	hash, err := h.CurrentBuckets.Hash()
	if err != nil {
		return xdr.Hash{}, err
	}
	if h.Version < HistoryArchiveStateVersionForProtocol23 {
		return hash, nil
	}
	// Protocol 23 introduced another bucketlist for archived entries.
	// From protocol 23 onwards, the bucket list hash in the ledger header
	// is computed by hashing both the live bucket list and the hot archive
	// bucket list. See:
	// https://github.com/stellar/stellar-protocol/blob/master/core/cap-0062.md#changes-to-ledgerheader
	archiveListHash, err := h.HotArchiveBuckets.Hash()
	if err != nil {
		return xdr.Hash{}, err
	}

	total := append(hash[:], archiveListHash[:]...)
	return sha256.Sum256(total), nil
}

func (h *HistoryArchiveState) Range() Range {
	return Range{Low: 63, High: h.CurrentLedger}
}
