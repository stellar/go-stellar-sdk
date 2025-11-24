// Copyright 2019 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package historyarchive

import (
	"io"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func (has *HistoryArchiveState) GetChangedBuckets(arch *Archive, prevHas *HistoryArchiveState) (string, int, int64) {
	var (
		nChangedBytes   int64
		nChangedBuckets int
		changedBuckets  string
	)

	for i, b := range has.CurrentBuckets {
		if prevHas.CurrentBuckets[i].Curr != b.Curr {
			nChangedBuckets += 1
			changedBuckets += "#"
			nChangedBytes += arch.MustGetBucketSize(MustDecodeHash(b.Curr))
		} else {
			changedBuckets += "_"
		}
		if prevHas.CurrentBuckets[i].Snap != b.Snap {
			nChangedBuckets += 1
			changedBuckets += "#"
			nChangedBytes += arch.MustGetBucketSize(MustDecodeHash(b.Snap))
		} else {
			changedBuckets += "_"
		}
	}
	return changedBuckets, nChangedBuckets, nChangedBytes
}

func (arch *Archive) MustGetBucketSize(hash Hash) int64 {
	sz, err := arch.backend.Size(arch.GetBucketPathForHash(hash))
	if err != nil {
		panic(err)
	}
	return sz
}

func (arch *Archive) MustGetLedgerHeaderHistoryEntries(chk uint32) []xdr.LedgerHeaderHistoryEntry {
	path := CategoryCheckpointPath("ledger", chk)
	rdr, err := arch.GetXdrStream(path)
	if err != nil {
		panic(err)
	}
	defer rdr.Close()
	var lhes []xdr.LedgerHeaderHistoryEntry
	for {
		lhe := xdr.LedgerHeaderHistoryEntry{}
		if err = rdr.ReadOne(&lhe); err != nil {
			if err == io.EOF {
				break
			} else {
				panic(err)
			}
		}
		lhes = append(lhes, lhe)
	}
	return lhes
}

func (arch *Archive) MustGetTransactionHistoryEntries(chk uint32) []xdr.TransactionHistoryEntry {
	path := CategoryCheckpointPath("transactions", chk)
	rdr, err := arch.GetXdrStream(path)
	if err != nil {
		panic(err)
	}
	defer rdr.Close()
	var thes []xdr.TransactionHistoryEntry
	for {
		the := xdr.TransactionHistoryEntry{}
		if err = rdr.ReadOne(&the); err != nil {
			if err == io.EOF {
				break
			} else {
				panic(err)
			}
		}
		thes = append(thes, the)
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

		changedBuckets, nChangedBuckets, nChangedBytes := has.GetChangedBuckets(arch, &prevHas)
		prevHas = has

		lhes := arch.MustGetLedgerHeaderHistoryEntries(chk)
		lastlhe := lhes[len(lhes)-1]
		closeTime := time.Unix(int64(lastlhe.Header.ScpValue.CloseTime), 0)

		nTxs := 0
		thes := arch.MustGetTransactionHistoryEntries(chk)
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
