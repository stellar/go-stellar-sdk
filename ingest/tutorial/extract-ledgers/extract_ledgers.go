package main

import (
	"context"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/xdr"

	ledgerbackend "github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
)

func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	// Command-line arguments: output filename and space-separated ledger numbers
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run extract_ledgers.go <output_file> <ledger_numbers...>")
		return
	}

	// Get the output file name and ledger numbers from arguments
	outputFile := os.Args[1]
	ledgerNumbers := os.Args[2:]

	archiveURLs := network.PublicNetworkhistoryArchiveURLs
	networkPassphrase := network.PublicNetworkPassphrase
	captiveCoreToml, err := ledgerbackend.NewCaptiveCoreToml(ledgerbackend.CaptiveCoreTomlParams{
		NetworkPassphrase:  networkPassphrase,
		HistoryArchiveURLs: archiveURLs,
	})
	panicIf(err)

	conf := ledgerbackend.CaptiveCoreConfig{
		// Change these based on your environment:
		BinaryPath:         "/usr/local/bin/stellar-core",
		NetworkPassphrase:  networkPassphrase,
		HistoryArchiveURLs: archiveURLs,
		Toml:               captiveCoreToml,
	}

	// Prepare logging
	lg := log.New()
	lg.SetLevel(logrus.ErrorLevel)
	conf.Log = lg

	// Open or create the output file for appending
	file, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	panicIf(err)
	defer file.Close()

	// Prepare backend connection
	ctx := context.Background()
	backend, err := ledgerbackend.NewCaptive(conf)
	panicIf(err)
	defer backend.Close()

	// Loop through the provided ledger numbers
	for _, ledgerSeqStr := range ledgerNumbers {
		// Convert ledger number to int
		var ledgerSeq int
		_, err := fmt.Sscanf(ledgerSeqStr, "%d", &ledgerSeq)
		panicIf(err)

		fmt.Printf("Fetching ledgerSequence: %v\n", ledgerSeq)
		// Prepare and retrieve the ledger
		err = backend.PrepareRange(ctx, ledgerbackend.BoundedRange(uint32(ledgerSeq), uint32(ledgerSeq)))
		panicIf(err)

		ledger, err := backend.GetLedger(ctx, uint32(ledgerSeq))
		panicIf(err)

		// Marshal ledger into XDR
		ledgerBase64Str, err := xdr.MarshalBase64(ledger)
		panicIf(err)

		// Write the XDR string to the output file
		_, err = file.WriteString(ledgerBase64Str + "\n")
		panicIf(err)
	}

	fmt.Println("Finished extracting and saving ledgers to:", outputFile)
}
