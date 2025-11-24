package main

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"io"
	"log"
	"os"
	"strconv"

	"github.com/stellar/go-stellar-sdk/historyarchive"
	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/ingest/sac"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

var (
	archiveURLs = []string{
		"https://history.stellar.org/prd/core-live/core_live_001",
		"https://history.stellar.org/prd/core-live/core_live_002",
		"https://history.stellar.org/prd/core-live/core_live_003",
	}
	targetAssets = map[string]string{
		"CDTKPWPLOURQA2SGTKTUQOWRCBZEORB4BWBOMJ3D3ZTQQSGE5F6JBQLV": "EURC",
		"CCW67TSZV3SSS2HXMBQ5JFGCKJNXKZM7UQUWUZPUTHXSTZLEO7SJMI75": "USDC",
	}
	cutoffLedger uint32 = 60739011
)

type balance struct {
	ledgerKey     string
	asset         string
	holderAddress string
	amount        string
	ttl           string
}

func findEvictedBalances(ctx context.Context, arch historyarchive.ArchiveInterface, checkpointLedger uint32, targetAssets map[string]string) []balance {
	var balances []balance
	for ledgerEntry, err := range ingest.NewHotArchiveIterator(ctx, arch, checkpointLedger) {
		if err != nil {
			log.Fatalf("error while reading hot archive ledger entries: %v", err)
		}
		if ledgerEntry.Data.Type != xdr.LedgerEntryTypeContractData {
			continue
		}

		contractID, ok := ledgerEntry.Data.MustContractData().Contract.GetContractId()
		if !ok {
			continue
		}
		sacAddress := strkey.MustEncode(strkey.VersionByteContract, contractID[:])
		asset, ok := targetAssets[sacAddress]
		if !ok {
			continue
		}

		holder, amt, ok := sac.ContractBalanceFromContractData(ledgerEntry, network.PublicNetworkPassphrase)
		if !ok {
			continue
		}

		ledgerKey, err := ledgerEntry.LedgerKey()
		if err != nil {
			log.Fatalf("error while extracting ledger key: %v", err)
		}
		ledgerKeyBase64, err := ledgerKey.MarshalBinaryBase64()
		if err != nil {
			log.Fatalf("error while marshaling ledger key: %v", err)
		}

		balances = append(balances, balance{
			ledgerKey:     ledgerKeyBase64,
			asset:         asset,
			holderAddress: strkey.MustEncode(strkey.VersionByteContract, holder[:]),
			amount:        amt.String(),
			ttl:           "evicted",
		})
	}
	return balances
}

func findExpiringBalances(ctx context.Context, arch historyarchive.ArchiveInterface, checkpointLedger, expirationLedger uint32, targetAssets map[string]string) []balance {
	var balances []balance
	byKeyHash := map[xdr.Hash]balance{}
	ttls := map[xdr.Hash]uint32{}

	reader, err := ingest.NewCheckpointChangeReader(ctx, arch, checkpointLedger)
	if err != nil {
		log.Fatalf("failed to create checkpoint change reader: %v", err)
	}
	defer reader.Close()

	for {
		change, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("error while reading checkpoint changes: %v", err)
		}

		if change.Type == xdr.LedgerEntryTypeTtl {
			ttlEntry := change.Post.Data.MustTtl()
			ttls[ttlEntry.KeyHash] = uint32(ttlEntry.LiveUntilLedgerSeq)
		}

		if change.Type != xdr.LedgerEntryTypeContractData || change.Post == nil {
			continue
		}

		le := *change.Post
		contractID, ok := le.Data.MustContractData().Contract.GetContractId()
		if !ok {
			continue
		}
		sacAddress := strkey.MustEncode(strkey.VersionByteContract, contractID[:])
		asset, ok := targetAssets[sacAddress]
		if !ok {
			continue
		}

		holder, amt, ok := sac.ContractBalanceFromContractData(le, network.PublicNetworkPassphrase)
		if !ok {
			continue
		}

		ledgerKey, err := le.LedgerKey()
		if err != nil {
			log.Fatalf("error while extracting ledger key: %v", err)
		}
		bin, err := ledgerKey.MarshalBinary()
		if err != nil {
			log.Fatalf("error while marshaling ledger key: %v", err)
		}
		keyHash := xdr.Hash(sha256.Sum256(bin))
		if _, ok := byKeyHash[keyHash]; ok {
			log.Fatalf("duplicate key hash: %v", keyHash)
		}

		ledgerKeyBase64, err := ledgerKey.MarshalBinaryBase64()
		if err != nil {
			log.Fatalf("error while marshaling ledger key: %v", err)
		}

		byKeyHash[keyHash] = balance{
			ledgerKey:     ledgerKeyBase64,
			asset:         asset,
			holderAddress: strkey.MustEncode(strkey.VersionByteContract, holder[:]),
			amount:        amt.String(),
		}
	}

	for keyHash, b := range byKeyHash {
		ttl, ok := ttls[keyHash]
		if !ok {
			log.Fatalf("missing ttl for key hash: %v", keyHash)
		}
		if ttl > expirationLedger {
			continue
		}
		b.ttl = strconv.FormatUint(uint64(ttl), 10)
		balances = append(balances, b)
	}
	return balances
}

// finds all SAC balances for a given list of assets that are either archived
// or close to being archived
func main() {
	arch, err := historyarchive.NewArchivePool(archiveURLs, historyarchive.ArchiveOptions{})
	if err != nil {
		log.Fatalf("failed to connect to pubnet archives: %v", err)
	}

	// Determine the latest published checkpoint ledger sequence
	seq, err := arch.GetLatestLedgerSequence()
	if err != nil {
		log.Fatalf("failed to get latest checkpoint sequence: %v", err)
	}

	// Always write to stdout
	csvw := csv.NewWriter(os.Stdout)
	// header
	if err = csvw.Write([]string{"asset", "ledger_key", "holder_contract_id", "amount", "ttl"}); err != nil {
		log.Fatalf("failed writing CSV: %v", err)
	}

	for _, b := range findExpiringBalances(context.Background(), arch, seq, cutoffLedger, targetAssets) {
		if err = csvw.Write([]string{
			b.asset,
			b.ledgerKey,
			b.holderAddress,
			b.amount,
			b.ttl,
		}); err != nil {
			log.Fatalf("failed writing CSV: %v", err)
		}
	}

	for _, b := range findEvictedBalances(context.Background(), arch, seq, targetAssets) {
		if err = csvw.Write([]string{
			b.asset,
			b.ledgerKey,
			b.holderAddress,
			b.amount,
			b.ttl,
		}); err != nil {
			log.Fatalf("failed writing CSV: %v", err)
		}
	}

	csvw.Flush()
	if err := csvw.Error(); err != nil {
		log.Fatalf("failed writing CSV: %v", err)
	}
}
