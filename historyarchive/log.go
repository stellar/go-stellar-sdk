// Copyright 2019 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package historyarchive

import (
	"io"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// GetChangedBuckets compares the live bucket list of has against prevHas and
// returns a per-level change map (one '#' or '_' for each level's curr and
// snap), the number of changed buckets and their total size in bytes. A curr
// or snap that is not a hash, or whose size the archive cannot report, is an
// error.
func (has *HistoryArchiveState) GetChangedBuckets(arch *Archive, prevHas *HistoryArchiveState) (string, int, int64, error) {
	var (
		nChangedBytes   int64
		nChangedBuckets int
		changedBuckets  string
	)

	for i, b := range has.CurrentBuckets {
		for _, side := range []struct{ curr, prev string }{
			{b.Curr, prevHas.CurrentBuckets[i].Curr},
			{b.Snap, prevHas.CurrentBuckets[i].Snap},
		} {
			if side.curr == side.prev {
				changedBuckets += "_"
				continue
			}
			hash, err := DecodeHash(side.curr)
			if err != nil {
				return "", 0, 0, errors.Wrapf(err, "decoding bucket hash at level %d", i)
			}
			size, err := arch.BucketSize(hash)
			if err != nil {
				return "", 0, 0, errors.Wrapf(err, "sizing bucket %s at level %d", hash, i)
			}
			nChangedBuckets += 1
			changedBuckets += "#"
			nChangedBytes += size
		}
	}
	return changedBuckets, nChangedBuckets, nChangedBytes, nil
}

func (arch *Archive) MustGetBucketSize(hash Hash) int64 {
	sz, err := arch.BucketSize(hash)
	if err != nil {
		panic(err)
	}
	return sz
}

func (arch *Archive) GetLedgerHeaderHistoryEntries(chk uint32) ([]xdr.LedgerHeaderHistoryEntry, error) {
	path := CategoryCheckpointPath("ledger", chk)
	rdr, err := arch.GetXdrStream(path)
	if err != nil {
		return nil, err
	}
	defer rdr.Close()
	var lhes []xdr.LedgerHeaderHistoryEntry
	for {
		lhe := xdr.LedgerHeaderHistoryEntry{}
		if err = rdr.ReadOne(&lhe); err != nil {
			if err == io.EOF {
				break
			}
			return nil, errors.Wrapf(err, "reading %s", path)
		}
		lhes = append(lhes, lhe)
	}
	return lhes, nil
}

func (arch *Archive) MustGetLedgerHeaderHistoryEntries(chk uint32) []xdr.LedgerHeaderHistoryEntry {
	lhes, err := arch.GetLedgerHeaderHistoryEntries(chk)
	if err != nil {
		panic(err)
	}
	return lhes
}

func (arch *Archive) GetTransactionHistoryEntries(chk uint32) ([]xdr.TransactionHistoryEntry, error) {
	path := CategoryCheckpointPath("transactions", chk)
	rdr, err := arch.GetXdrStream(path)
	if err != nil {
		return nil, err
	}
	defer rdr.Close()
	var thes []xdr.TransactionHistoryEntry
	for {
		the := xdr.TransactionHistoryEntry{}
		if err = rdr.ReadOne(&the); err != nil {
			if err == io.EOF {
				break
			}
			return nil, errors.Wrapf(err, "reading %s", path)
		}
		thes = append(thes, the)
	}
	return thes, nil
}

func (arch *Archive) MustGetTransactionHistoryEntries(chk uint32) []xdr.TransactionHistoryEntry {
	thes, err := arch.GetTransactionHistoryEntries(chk)
	if err != nil {
		panic(err)
	}
	return thes
}

func (arch *Archive) Log(opts *CommandOptions) error {
	state, e := arch.GetRootHAS()
	if e != nil {
		return e
	}
	opts.Range = opts.Range.clamp(state.Range(), arch.checkpointManager)

	log.Printf("Log of checkpoint files in range: %s", opts.Range)
	log.Printf("\n")
	log.Printf("%10s | %10s | %20s | %5s | %s",
		"ledger", "hex", "close time", "txs", "buckets changed")

	prevHas, err := arch.GetCheckpointHAS(arch.checkpointManager.PrevCheckpoint(opts.Range.Low))
	if err != nil {
		return err
	}

	for chk := range opts.Range.GenerateCheckpoints(arch.checkpointManager) {
		has, err := arch.GetCheckpointHAS(chk)
		if err != nil {
			return err
		}

		changedBuckets, nChangedBuckets, nChangedBytes, err := has.GetChangedBuckets(arch, &prevHas)
		if err != nil {
			return err
		}
		prevHas = has

		lhes, err := arch.GetLedgerHeaderHistoryEntries(chk)
		if err != nil {
			return err
		}
		if len(lhes) == 0 {
			return errors.Errorf("checkpoint %d has no ledger headers", chk)
		}
		lastlhe := lhes[len(lhes)-1]
		closeTime := time.Unix(int64(lastlhe.Header.ScpValue.CloseTime), 0)

		nTxs := 0
		thes, err := arch.GetTransactionHistoryEntries(chk)
		if err != nil {
			return err
		}
		for _, tx := range thes {
			nTxs += len(tx.TxSet.Txs)
		}

		log.Printf("%10d | 0x%08x | %20s | %5d | %2d buckets, %10d bytes, %s",
			has.CurrentLedger, has.CurrentLedger,
			closeTime.UTC().Format(time.RFC3339),
			nTxs, nChangedBuckets, nChangedBytes, changedBuckets)
	}
	return nil
}
