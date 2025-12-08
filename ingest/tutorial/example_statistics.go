package main

import (
	"context"
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
	"github.com/stellar/go-stellar-sdk/ingest"
	backends "github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
	"github.com/stellar/go-stellar-sdk/support/log"
)

func statistics() {
	ctx := context.Background()
	// Only log errors from the backend to keep output cleaner.
	lg := log.New()
	lg.SetLevel(logrus.ErrorLevel)
	config.Log = lg

	backend, err := backends.NewCaptive(config)
	panicIf(err)
	defer backend.Close()

	// Prepare a range to be ingested:
	var startingSeq uint32 = 2 // can't start with genesis ledger
	var ledgersToRead uint32 = 10000

	fmt.Printf("Preparing range (%d ledgers)...\n", ledgersToRead)
	ledgerRange := backends.BoundedRange(startingSeq, startingSeq+ledgersToRead)
	err = backend.PrepareRange(ctx, ledgerRange)
	panicIf(err)

	// These are the statistics that we're tracking.
	var successfulTransactions, failedTransactions int
	var operationsInSuccessful, operationsInFailed int

	for seq := startingSeq; seq <= startingSeq+ledgersToRead; seq++ {
		fmt.Printf("Processed ledger %d...\r", seq)

		txReader, err := ingest.NewLedgerTransactionReader(
			ctx, backend, config.NetworkPassphrase, seq,
		)
		panicIf(err)
		defer txReader.Close()

		// Read each transaction within the ledger, extract its operations, and
		// accumulate the statistics we're interested in.
		for {
			tx, err := txReader.Read()
			if err == io.EOF {
				break
			}
			panicIf(err)

			envelope := tx.Envelope
			operationCount := len(envelope.Operations())
			if tx.Result.Successful() {
				successfulTransactions++
				operationsInSuccessful += operationCount
			} else {
				failedTransactions++
				operationsInFailed += operationCount
			}
		}
	}

	fmt.Println("\nDone. Results:")
	fmt.Printf("  - total transactions: %d\n", successfulTransactions+failedTransactions)
	fmt.Printf("  - succeeded / failed: %d / %d\n", successfulTransactions, failedTransactions)
	fmt.Printf("  - total operations:   %d\n", operationsInSuccessful+operationsInFailed)
	fmt.Printf("  - succeeded / failed: %d / %d\n", operationsInSuccessful, operationsInFailed)
}
