// Copyright 2016 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package historyarchive

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"sort"

	log "github.com/sirupsen/logrus"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// Transaction sets are sorted in two different orders: one for hashing and
// one for applying. Hash order is just the lexicographic order of the
// hashes of the txs themselves. Apply order is built on top, by xoring
// each tx hash with the set-hash (to defeat anyone trying to force a given
// apply sequence), and sub-ordering by account sequence number.
//
// TxSets are stored in the XDR file in apply-order, but we want to sort
// them here back in simple hash order so we can confirm the hash value
// agreed-on by SCP.
//
// Moreover, txsets (when sorted) are _not_ hashed by simply hashing the
// XDR; they have a slightly-more-manual hashing process.

type byHash struct {
	txe []xdr.TransactionEnvelope
	hsh []Hash
}

func (h *byHash) Len() int { return len(h.hsh) }
func (h *byHash) Swap(i, j int) {
	h.txe[i], h.txe[j] = h.txe[j], h.txe[i]
	h.hsh[i], h.hsh[j] = h.hsh[j], h.hsh[i]
}
func (h *byHash) Less(i, j int) bool {
	return bytes.Compare(h.hsh[i][:], h.hsh[j][:]) < 0
}

func SortTxsForHash(txset *xdr.TransactionSet) error {
	bh := &byHash{
		txe: txset.Txs,
		hsh: make([]Hash, len(txset.Txs)),
	}
	for i, tx := range txset.Txs {
		h, err := xdr.HashXdr(&tx)
		if err != nil {
			return err
		}
		bh.hsh[i] = Hash(h)
	}
	sort.Sort(bh)
	return nil
}

func HashTxSet(txset *xdr.TransactionSet) (Hash, error) {
	err := SortTxsForHash(txset)
	var h Hash
	if err != nil {
		return h, err
	}
	hsh := sha256.New()
	hsh.Write(txset.PreviousLedgerHash[:])

	for _, env := range txset.Txs {
		_, err := xdr.Marshal(hsh, &env)
		if err != nil {
			return h, err
		}
	}
	sum := hsh.Sum([]byte{})
	copy(h[:], sum[:])
	return h, nil
}

func HashEmptyTxSet(previousLedgerHash Hash) Hash {
	return Hash(sha256.Sum256(previousLedgerHash[:]))
}

func (arch *Archive) VerifyLedgerHeaderHistoryEntry(entry *xdr.LedgerHeaderHistoryEntry) error {
	h, err := xdr.HashXdr(&entry.Header)
	if err != nil {
		return err
	}
	if h != entry.Hash {
		return fmt.Errorf("Ledger %d expected hash %s, got %s",
			entry.Header.LedgerSeq, Hash(entry.Hash), Hash(h))
	}
	arch.mutex.Lock()
	defer arch.mutex.Unlock()
	seq := uint32(entry.Header.LedgerSeq)
	arch.actualLedgerHashes[seq] = Hash(h)
	arch.expectLedgerHashes[seq-1] = Hash(entry.Header.PreviousLedgerHash)
	arch.expectTxSetHashes[seq] = Hash(entry.Header.ScpValue.TxSetHash)
	arch.expectTxResultSetHashes[seq] = Hash(entry.Header.TxSetResultHash)

	return nil
}

func (arch *Archive) VerifyTransactionHistoryEntry(entry *xdr.TransactionHistoryEntry) error {
	var h Hash
	if entry.Ext.V == 0 {
		h0, err := HashTxSet(&entry.TxSet)
		if err != nil {
			return err
		}
		h = h0
	} else if entry.Ext.V == 1 {
		h1, err := xdr.HashXdr(entry.Ext.GeneralizedTxSet)
		if err != nil {
			return err
		}
		h = Hash(h1)
	} else {
		return fmt.Errorf("unknown TransactionHistoryEntry ext version %d", entry.Ext.V)
	}
	log.Tracef("Found actual v%d TxSet hash %s", entry.Ext.V, h)
	arch.mutex.Lock()
	defer arch.mutex.Unlock()
	arch.actualTxSetHashes[uint32(entry.LedgerSeq)] = h
	return nil
}

func (arch *Archive) VerifyTransactionHistoryResultEntry(entry *xdr.TransactionHistoryResultEntry) error {
	h, err := xdr.HashXdr(&entry.TxResultSet)
	if err != nil {
		return err
	}
	arch.mutex.Lock()
	defer arch.mutex.Unlock()
	arch.actualTxResultSetHashes[uint32(entry.LedgerSeq)] = Hash(h)
	return nil
}

func (arch *Archive) VerifyCategoryCheckpoint(cat string, chk uint32) error {

	if cat == "history" {
		return nil
	}

	rdr, err := arch.GetXdrStream(CategoryCheckpointPath(cat, chk))
	if err != nil {
		return err
	}
	defer rdr.Close()

	var tmp xdr.DecoderFrom
	var step func() error
	var reset func()

	var lhe xdr.LedgerHeaderHistoryEntry
	var the xdr.TransactionHistoryEntry
	var thre xdr.TransactionHistoryResultEntry

	switch cat {
	case "ledger":
		tmp = &lhe
		step = func() error {
			return arch.VerifyLedgerHeaderHistoryEntry(&lhe)
		}
		reset = func() {
			lhe = xdr.LedgerHeaderHistoryEntry{}
		}
	case "transactions":
		tmp = &the
		step = func() error {
			return arch.VerifyTransactionHistoryEntry(&the)
		}
		reset = func() {
			the = xdr.TransactionHistoryEntry{}
		}
	case "results":
		tmp = &thre
		step = func() error {
			return arch.VerifyTransactionHistoryResultEntry(&thre)
		}
		reset = func() {
			thre = xdr.TransactionHistoryResultEntry{}
		}
	default:
		return nil
	}

	for {
		reset()
		if err = rdr.ReadOne(tmp); err != nil {
			if err == io.EOF {
				break
			} else {
				return err
			}
		}
		if err = step(); err != nil {
			return err
		}
	}
	return nil
}

func checkBucketHash(hasher hash.Hash, expect Hash) error {
	var actual Hash
	sum := hasher.Sum([]byte{})
	copy(actual[:], sum[:])
	if actual != expect {
		return fmt.Errorf("bucket hash mismatch: expected %s, got %s",
			expect, actual)
	}
	return nil
}

func (arch *Archive) VerifyBucketHash(h Hash) error {
	rdr, err := arch.backend.GetFile(BucketPath(h))
	if err != nil {
		return err
	}
	defer rdr.Close()
	hsh := sha256.New()
	rdr, err = gzip.NewReader(bufReadCloser(rdr))
	if err != nil {
		return err
	}
	io.Copy(hsh, bufReadCloser(rdr))
	return checkBucketHash(hsh, h)
}

func (arch *Archive) VerifyBucketEntries(h Hash) error {
	rdr, err := arch.GetXdrStream(BucketPath(h))
	if err != nil {
		return err
	}
	defer rdr.Close()
	hsh := sha256.New()
	for {
		var entry xdr.BucketEntry
		err = rdr.ReadOne(&entry)
		if err == nil {
			err2 := xdr.MarshalFramed(hsh, &entry)
			if err2 != nil {
				return err2
			}
		}
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
	}
	return checkBucketHash(hsh, h)
}

func reportValidity(ty string, nbad int, total int) {
	if nbad == 0 {
		log.Printf("Verified %d %ss have expected hashes", total, ty)
	} else {
		log.Errorf("Error: %d %ss (of %d checked) have unexpected hashes", nbad, ty, total)
	}
}

func compareHashMaps(expect map[uint32]Hash, actual map[uint32]Hash, ty string,
	passOn func(eledger uint32, ehash Hash) bool) int {
	n := 0
	for eledger, ehash := range expect {
		ahash, ok := actual[eledger]
		if !ok && passOn(eledger, ehash) {
			continue
		}
		if ahash != ehash {
			n++
			log.Errorf("Error: mismatched hash on %s 0x%8.8x: expected %s, got %s",
				ty, eledger, ehash, ahash)
		}
	}
	reportValidity(ty, n, len(expect))
	return n
}

func makeEmptyGeneralizedSequentialTxSet(previousLedgerHash Hash) xdr.GeneralizedTransactionSet {
	var emptyGenTxSet = xdr.GeneralizedTransactionSet{
		V: 1,
		V1TxSet: &xdr.TransactionSetV1{
			PreviousLedgerHash: xdr.Hash(previousLedgerHash),
			Phases:             make([]xdr.TransactionPhase, 2),
		},
	}
	emptyGenTxSet.V1TxSet.Phases[0].V = 0
	emptyGenTxSet.V1TxSet.Phases[0].V0Components = new([]xdr.TxSetComponent)
	emptyGenTxSet.V1TxSet.Phases[1].V = 0
	emptyGenTxSet.V1TxSet.Phases[1].V0Components = new([]xdr.TxSetComponent)
	return emptyGenTxSet
}

func makeEmptyGeneralizedParallelTxSet(previousLedgerHash Hash) xdr.GeneralizedTransactionSet {
	var emptyGenTxSet = xdr.GeneralizedTransactionSet{
		V: 1,
		V1TxSet: &xdr.TransactionSetV1{
			PreviousLedgerHash: xdr.Hash(previousLedgerHash),
			Phases:             make([]xdr.TransactionPhase, 2),
		},
	}
	emptyGenTxSet.V1TxSet.Phases[0].V = 0
	emptyGenTxSet.V1TxSet.Phases[0].V0Components = new([]xdr.TxSetComponent)
	emptyGenTxSet.V1TxSet.Phases[1].V = 1
	emptyGenTxSet.V1TxSet.Phases[1].ParallelTxsComponent = new(xdr.ParallelTxsComponent)
	return emptyGenTxSet
}

func (arch *Archive) ReportInvalid(opts *CommandOptions) (bool, error) {
	if !opts.Verify {
		return false, nil
	}

	arch.mutex.Lock()
	defer arch.mutex.Unlock()

	lowest := uint32(0xffffffff)
	for i := range arch.expectLedgerHashes {
		if i < lowest {
			lowest = i
		}
	}

	arch.invalidLedgers = compareHashMaps(arch.expectLedgerHashes,
		arch.actualLedgerHashes, "ledger header",
		func(eledger uint32, ehash Hash) bool {
			// We will never have the lowest expected ledger, because
			// it's one-before the first checkpoint we scanned.
			return eledger == lowest
		})

	arch.invalidTxSets = compareHashMaps(arch.expectTxSetHashes,
		arch.actualTxSetHashes, "transaction set",
		func(eledger uint32, ehash Hash) bool {
			// When there was an empty txset v0, it produces just the hash of
			// the previous ledger header followed by nothing.
			log.Tracef("Checking for empty v1 TxSet hash %s", ehash)
			previousLedgerHash := arch.expectLedgerHashes[eledger-1]
			if ehash == HashEmptyTxSet(previousLedgerHash) {
				log.Tracef("Found expected empty v0 TxSet hash %s", ehash)
				return true
			}

			// When it's a v1, it produces an empty GeneralizedTxSet hash,
			// which might be either sequential or parallel.
			emptySeqGenTxSet := makeEmptyGeneralizedSequentialTxSet(previousLedgerHash)
			emptySeqGenTxSetHash, err := xdr.HashXdr(&emptySeqGenTxSet)
			if err != nil {
				log.Errorf("error hashing empty sequential GeneralizedTxSet for ledger 0x%8.8x: %s",
					eledger, err)
				return false
			}
			if ehash == Hash(emptySeqGenTxSetHash) {
				log.Tracef("Found expected empty sequential GeneralizedTxSet hash %s",
					ehash)
				return true
			}

			// Try parallel version
			emptyParGenTxSet := makeEmptyGeneralizedParallelTxSet(previousLedgerHash)
			emptyParGenTxSetHash, err := xdr.HashXdr(&emptyParGenTxSet)
			if err != nil {
				log.Errorf("error hashing empty parallel GeneralizedTxSet for ledger 0x%8.8x: %s",
					eledger, err)
				return false
			}
			if ehash == Hash(emptyParGenTxSetHash) {
				log.Tracef("Found expected empty parallel GeneralizedTxSet hash %s",
					ehash)
				return true
			}
			return false
		})

	emptyXdrArrayHash := EmptyXdrArrayHash()
	arch.invalidTxResultSets = compareHashMaps(arch.expectTxResultSetHashes,
		arch.actualTxResultSetHashes, "transaction result set",
		func(eledger uint32, ehash Hash) bool {
			// When there was an empty txresultset, it produces just the hash of
			// the 4-zero-byte "0 entries" XDR array.
			return ehash == emptyXdrArrayHash
		})

	reportValidity("bucket", arch.invalidBuckets, len(arch.referencedBuckets))

	totalInvalid := arch.invalidBuckets
	totalInvalid += arch.invalidLedgers
	totalInvalid += arch.invalidTxSets
	totalInvalid += arch.invalidTxResultSets

	if totalInvalid != 0 {
		return true, fmt.Errorf("detected %d objects with unexpected hashes", totalInvalid)
	}
	return false, nil
}
