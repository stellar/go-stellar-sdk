package main

import (
	"context"
	"flag"
	"io"
	"log"

	"github.com/stellar/go-stellar-sdk/historyarchive"
	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/support/storage"
)

func main() {
	ledgerPtr := flag.Uint64("ledger", 0, "`ledger to analyze` (tip: has to be of the form `ledger = 64*n - 1`, where n is > 0)")
	flag.Parse()
	seqNum := uint32(*ledgerPtr)

	if seqNum == 0 {
		flag.Usage()
		return
	}

	archive, e := archive()
	if e != nil {
		panic(e)
	}

	sr, e := ingest.NewCheckpointChangeReader(context.Background(), archive, seqNum)
	if e != nil {
		panic(e)
	}

	accounts := map[string]bool{}
	var i uint64 = 0
	var count uint64 = 0
	for {
		le, e := sr.Read()
		if e != nil {
			panic(e)
		}
		if e == io.EOF {
			log.Printf("total seen %d entries of which %d were accounts", i, count)
			return
		}

		if ae, valid := le.Post.Data.GetAccount(); valid {
			addr := ae.AccountId.Address()
			if _, exists := accounts[addr]; exists {
				log.Fatalf("error, total seen %d entries of which %d were unique accounts; repeated account: %s", i, count, addr)
			}

			accounts[addr] = true
			count += 1
		}
		i += 1

		if i%1000 == 0 {
			log.Printf("seen %d entries of which %d were accounts", i, count)
		}
	}
}

func archive() (*historyarchive.Archive, error) {
	return historyarchive.Connect(
		"s3://history.stellar.org/prd/core-live/core_live_001/",
		historyarchive.ArchiveOptions{
			ConnectOptions: storage.ConnectOptions{
				S3Region:         "eu-west-1",
				UserAgent:        "archive-reader",
				UnsignedRequests: true,
			},
		},
	)
}
