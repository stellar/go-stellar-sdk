// Copyright 2021 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package historyarchive

import (
	"context"
	"math/rand"
	"time"

	"github.com/pkg/errors"

	log "github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/xdr"

	backoff "github.com/cenkalti/backoff/v4"
)

// An ArchivePool is just a collection of `ArchiveInterface`s so that we can
// distribute requests fairly throughout the pool.
type ArchivePool struct {
	logger  *log.Entry
	backoff backoff.BackOff
	pool    []ArchiveInterface
	curr    int
}

// NewArchivePool tries connecting to each of the provided history archive URLs,
// returning a pool of valid archives.
//
// If none of the archives work, this returns the error message of the last
// failed archive. Note that the errors for each individual archive are hard to
// track if there's success overall.
func NewArchivePool(archiveURLs []string, opts ArchiveOptions) (ArchiveInterface, error) {
	return NewArchivePoolWithBackoff(
		archiveURLs,
		opts,
		backoff.WithMaxRetries(backoff.NewConstantBackOff(250*time.Millisecond), 3),
	)
}

func NewArchivePoolWithBackoff(archiveURLs []string, opts ArchiveOptions, strategy backoff.BackOff) (ArchiveInterface, error) {
	if len(archiveURLs) <= 0 {
		return nil, errors.New("No history archives provided")
	}

	ap := ArchivePool{
		pool:    make([]ArchiveInterface, 0, len(archiveURLs)),
		backoff: strategy,
		logger:  opts.Logger,
	}
	var lastErr error

	// Try connecting to all of the listed archives, but only store valid ones.
	for _, url := range archiveURLs {
		archive, err := Connect(url, opts)
		if err != nil {
			lastErr = errors.Wrapf(err, "Error connecting to history archive (%s)", url)
			continue
		}

		ap.pool = append(ap.pool, archive)
	}

	if len(ap.pool) == 0 {
		return nil, lastErr
	}

	ap.curr = rand.Intn(len(ap.pool)) // don't necessarily start at zero
	return &ap, nil
}

func (pa *ArchivePool) GetStats() []ArchiveStats {
	stats := []ArchiveStats{}
	for _, archive := range pa.pool {
		stats = append(stats, archive.GetStats()...)
	}
	return stats
}

// Ensure the pool conforms to the ArchiveInterface
var _ ArchiveInterface = &ArchivePool{}

//
// These are helpers to round-robin calls through archives.
//

// getNextArchive statefully round-robins through the pool
func (pa *ArchivePool) getNextArchive() ArchiveInterface {
	// Round-robin through the archives
	pa.curr = (pa.curr + 1) % len(pa.pool)
	return pa.pool[pa.curr]
}

// runRoundRobin is a helper method that will run a particular action on every
// archive in the pool until it succeeds or the pool is exhausted (whichever
// comes first), repeating with a constant 500ms backoff.
func (pa *ArchivePool) runRoundRobin(runner func(ai ArchiveInterface) error) error {
	return backoff.Retry(func() error {
		var err error
		ai := pa.getNextArchive()
		if err = runner(ai); err == nil {
			return nil
		}

		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {
			return backoff.Permanent(err)
		}

		// Intentionally avoid logging context errors
		if stats := ai.GetStats(); len(stats) > 0 && pa.logger != nil {
			pa.logger.WithField("error", err).Warnf(
				"Encountered an error with archive '%s'",
				stats[0].GetBackendName())
		}

		return err
	}, pa.backoff)
}

//
// Below are the ArchiveInterface method implementations.
//

func (pa *ArchivePool) GetPathHAS(path string) (HistoryArchiveState, error) {
	has := HistoryArchiveState{}
	err := pa.runRoundRobin(func(ai ArchiveInterface) error {
		var innerErr error
		has, innerErr = ai.GetPathHAS(path)
		return innerErr
	})
	return has, err
}

func (pa *ArchivePool) PutPathHAS(path string, has HistoryArchiveState, opts *CommandOptions) error {
	return pa.runRoundRobin(func(ai ArchiveInterface) error {
		return ai.PutPathHAS(path, has, opts)
	})
}

func (pa *ArchivePool) BucketExists(bucket Hash) (bool, error) {
	status := false
	return status, pa.runRoundRobin(func(ai ArchiveInterface) error {
		var err error
		status, err = ai.BucketExists(bucket)
		return err
	})
}

func (pa *ArchivePool) BucketSize(bucket Hash) (int64, error) {
	var bsize int64
	return bsize, pa.runRoundRobin(func(ai ArchiveInterface) error {
		var err error
		bsize, err = ai.BucketSize(bucket)
		return err
	})
}

func (pa *ArchivePool) CategoryCheckpointExists(cat string, chk uint32) (bool, error) {
	var ok bool
	return ok, pa.runRoundRobin(func(ai ArchiveInterface) error {
		var err error
		ok, err = ai.CategoryCheckpointExists(cat, chk)
		return err
	})
}

func (pa *ArchivePool) GetLedgerHeader(chk uint32) (xdr.LedgerHeaderHistoryEntry, error) {
	var entry xdr.LedgerHeaderHistoryEntry
	return entry, pa.runRoundRobin(func(ai ArchiveInterface) error {
		var err error
		entry, err = ai.GetLedgerHeader(chk)
		return err
	})
}

func (pa *ArchivePool) GetRootHAS() (HistoryArchiveState, error) {
	var state HistoryArchiveState
	return state, pa.runRoundRobin(func(ai ArchiveInterface) error {
		var err error
		state, err = ai.GetRootHAS()
		return err
	})
}

func (pa *ArchivePool) GetLedgers(start, end uint32) (map[uint32]*Ledger, error) {
	var dict map[uint32]*Ledger

	return dict, pa.runRoundRobin(func(ai ArchiveInterface) error {
		var err error
		dict, err = ai.GetLedgers(start, end)
		return err
	})
}

func (pa *ArchivePool) GetCheckpointHAS(chk uint32) (HistoryArchiveState, error) {
	var state HistoryArchiveState
	return state, pa.runRoundRobin(func(ai ArchiveInterface) error {
		var err error
		state, err = ai.GetCheckpointHAS(chk)
		return err
	})
}

func (pa *ArchivePool) GetLatestLedgerSequence() (uint32, error) {
	has, err := pa.GetRootHAS()
	if err != nil {
		log.Error("Error getting root HAS from archive", err)
		return 0, errors.Wrap(err, "failed to retrieve the latest ledger sequence from history archive")
	}

	return has.CurrentLedger, nil
}

func (pa *ArchivePool) PutCheckpointHAS(chk uint32, has HistoryArchiveState, opts *CommandOptions) error {
	return pa.runRoundRobin(func(ai ArchiveInterface) error {
		return ai.PutCheckpointHAS(chk, has, opts)
	})
}

func (pa *ArchivePool) PutRootHAS(has HistoryArchiveState, opts *CommandOptions) error {
	return pa.runRoundRobin(func(ai ArchiveInterface) error {
		return ai.PutRootHAS(has, opts)
	})
}

func (pa *ArchivePool) GetXdrStreamForHash(hash Hash) (*xdr.Stream, error) {
	var stream *xdr.Stream
	return stream, pa.runRoundRobin(func(ai ArchiveInterface) error {
		var err error
		stream, err = ai.GetXdrStreamForHash(hash)
		return err
	})
}

func (pa *ArchivePool) GetXdrStream(pth string) (*xdr.Stream, error) {
	var stream *xdr.Stream
	return stream, pa.runRoundRobin(func(ai ArchiveInterface) error {
		var err error
		stream, err = ai.GetXdrStream(pth)
		return err
	})
}

func (pa *ArchivePool) GetCheckpointManager() CheckpointManager {
	return pa.getNextArchive().GetCheckpointManager()
}

//
// The channel-based methods do not have automatic retries.
//

func (pa *ArchivePool) ListBucket(dp DirPrefix) (chan string, chan error) {
	return pa.getNextArchive().ListBucket(dp)
}

func (pa *ArchivePool) ListAllBuckets() (chan string, chan error) {
	return pa.getNextArchive().ListAllBuckets()
}

func (pa *ArchivePool) ListAllBucketHashes() (chan Hash, chan error) {
	return pa.getNextArchive().ListAllBucketHashes()
}

func (pa *ArchivePool) ListCategoryCheckpoints(cat string, pth string) (chan uint32, chan error) {
	return pa.getNextArchive().ListCategoryCheckpoints(cat, pth)
}
