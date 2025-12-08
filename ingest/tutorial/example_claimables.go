package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/stellar/go-stellar-sdk/historyarchive"
	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/support/storage"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func claimables() {
	// Open a history archive using our existing configuration details.
	historyArchive, err := historyarchive.Connect(
		config.HistoryArchiveURLs[0],
		historyarchive.ArchiveOptions{
			NetworkPassphrase: config.NetworkPassphrase,
			ConnectOptions: storage.ConnectOptions{
				S3Region:         "us-west-1",
				UnsignedRequests: false,
			},
		},
	)
	panicIf(err)

	// First, we need to establish a safe fallback in case of any problems
	// during the history archive download+processing, so we'll set a 30-second
	// timeout.
	//
	// NOTE: We're using the testnet here, whose archives are much smaller. For
	//       the pubnet, a 30 *minute* timeout may be more appropriate.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// We pass 123455 because given a checkpoint frequency of 64 ledgers (the
	// default in `ConnectOptions`, above), 123455+1 mod 64 == 0. Incompatible
	// sequence numbers will likely result in 404 errors.
	reader, err := ingest.NewCheckpointChangeReader(ctx, historyArchive, 123455)
	panicIf(err)

	entries, newCBs := 0, 0
	for {
		entry, err := reader.Read()
		if err == io.EOF {
			break
		}
		panicIf(err)

		entries++

		switch entry.Type {
		case xdr.LedgerEntryTypeClaimableBalance:
			newCBs++
		// these are included for completeness of the demonstration
		case xdr.LedgerEntryTypeAccount:
		case xdr.LedgerEntryTypeData:
		case xdr.LedgerEntryTypeTrustline:
		case xdr.LedgerEntryTypeOffer:
		default:
			panic(fmt.Errorf("Unknown type: %+v", entry.Type))
		}

		fmt.Printf("Processed %d ledger entry changes...\r", entries)
	}

	fmt.Println()
	fmt.Printf("%d/%d created entries were claimable balances\n", newCBs, entries)
}
