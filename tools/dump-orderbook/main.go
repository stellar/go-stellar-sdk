package main

import (
	"bytes"
	"context"
	"flag"
	"io"
	"os"
	"strings"

	"github.com/stellar/go-stellar-sdk/historyarchive"
	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/support/storage"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// This program will dump all the offers from a history archive checkpoint.
// The offers dump can then be fed in to the path finding benchmarks in exp/orderbook/graph_benchmark_test.go
func main() {
	testnet := flag.Bool("testnet", false, "connect to the Stellar test network")
	checkpointLedger := flag.Int(
		"checkpoint-ledger",
		0,
		"checkpoint ledger sequence to ingest, if omitted will use latest checkpoint ledger.",
	)
	offersOutput := flag.String("offers-output", "offers.dump", "output file which will be populated with offerss")
	poolsOutput := flag.String("pools-output", "pools.dump", "output file which will be populated with pools")

	flag.Parse()

	archive, err := archive(*testnet)
	if err != nil {
		panic(err)
	}
	log.SetLevel(log.InfoLevel)

	sequence := uint32(*checkpointLedger)
	if sequence == 0 {
		var root historyarchive.HistoryArchiveState
		root, err = archive.GetRootHAS()
		if err != nil {
			log.WithField("err", err).Fatal("could not fetch root has")
		}
		sequence = root.CurrentLedger
	}
	log.WithField("ledger", sequence).
		Info("Processing entries from History Archive Snapshot")

	var changeReader ingest.ChangeReader
	changeReader, err = ingest.NewCheckpointChangeReader(
		context.Background(),
		archive,
		sequence,
	)
	if err != nil {
		log.WithField("err", err).Fatal("cannot construct change reader")
	}
	defer changeReader.Close()
	var offersFile, poolsFile *os.File

	if offersFile, err = os.Create(*offersOutput); err != nil {
		log.WithField("err", err).Fatal("could not create offers file")
	}
	if poolsFile, err = os.Create(*poolsOutput); err != nil {
		log.WithField("err", err).Fatal("could not create pools file")
	}

	var offerXDRs []string
	var poolXDRs []string

	for {
		var change ingest.Change
		change, err = changeReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.WithField("err", err).Fatal("could not read change")
		}

		switch change.Type {
		case xdr.LedgerEntryTypeOffer:
			var serialized string
			serialized, err = xdr.MarshalBase64(change.Post.Data.MustOffer())
			if err != nil {
				log.WithField("err", err).Fatal("could not marshall offer")
			}
			offerXDRs = append(offerXDRs, serialized)
		case xdr.LedgerEntryTypeLiquidityPool:
			var serialized string
			serialized, err = xdr.MarshalBase64(change.Post.Data.MustLiquidityPool())
			if err != nil {
				log.WithField("err", err).Fatal("could not marshall liquidity pool")
			}
			poolXDRs = append(poolXDRs, serialized)
		}
	}

	if _, err = io.Copy(offersFile, bytes.NewBufferString(strings.Join(offerXDRs, "\n"))); err != nil {
		log.WithField("err", err).Fatal("could not write offer dump file")
	}
	if _, err = io.Copy(poolsFile, bytes.NewBufferString(strings.Join(poolXDRs, "\n"))); err != nil {
		log.WithField("err", err).Fatal("could not write pool dump file")
	}
}

func archive(testnet bool) (*historyarchive.Archive, error) {
	if testnet {
		return historyarchive.Connect(
			"https://history.stellar.org/prd/core-testnet/core_testnet_001",
			historyarchive.ArchiveOptions{
				ConnectOptions: storage.ConnectOptions{
					UserAgent: "dump-orderbook",
				},
			},
		)
	}

	return historyarchive.Connect(
		"https://history.stellar.org/prd/core-live/core_live_001/",
		historyarchive.ArchiveOptions{
			ConnectOptions: storage.ConnectOptions{
				UserAgent: "dump-orderbook",
			},
		},
	)
}
