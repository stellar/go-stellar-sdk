package main

import (
	"fmt"
	"io"
	"os"

	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/processors/token_transfer"
	"github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/xdr"
	"google.golang.org/protobuf/encoding/protojson"
)

func main() {
	// Ensure we have the correct number of arguments
	if len(os.Args) < 2 {
		log.Fatalf("Usage: go run extract_events_from_ledger.go <input_file> [<output_file>]")
		return
	}

	networkPassphrase := network.PublicNetworkPassphrase

	inputFileName := os.Args[1]
	var outputFile *os.File
	var err error

	//  Open input file
	inputFile, err := os.Open(inputFileName)
	if err != nil {
		log.Fatalf("Error opening input file: %v", err)
	}
	defer inputFile.Close()

	// If output file is provided, create or overwrite it
	if len(os.Args) > 2 {
		outputFileName := os.Args[2]
		outputFile, err = os.Create(outputFileName) // Create the file fresh (overwrite if exists)
		if err != nil {
			log.Fatalf("Error opening output file: %v", err)
		}
		// Inform the user that output will be saved to the file
		fmt.Printf("Saving events to file: %s\n", outputFileName)
		defer outputFile.Close()
	} else {
		// If no output file is provided, use os.Stdout to print to the screen
		outputFile = os.Stdout
	}

	// Process each line in the input file
	var lineNum int
	for {
		// Read each line of the input file (ledgerXdr base64 encoded string)
		ledgerXdrEncodedString, err := readLine(inputFile)
		if err == io.EOF {
			break // End of file, done reading
		}
		if err != nil {
			log.Fatalf("Error reading input file: %v", err)
		}

		// Unmarshal the bytes into the ledger structure
		var ledger xdr.LedgerCloseMeta
		err = xdr.SafeUnmarshalBase64(ledgerXdrEncodedString, &ledger)
		if err != nil {
			log.Errorf("Error unmarshaling XDR into LedgerCloseMeta at line %d: %v", lineNum, err)
			continue // Skip to the next line in case of error
		}

		// Process the ledger to extract token transfer events
		ttp := token_transfer.NewEventsProcessor(networkPassphrase)
		events, err := ttp.EventsFromLedger(ledger)
		if err != nil {
			log.Errorf("Error processing ledger at line %d: %v", lineNum, err)
			continue // Skip to the next line in case of error
		}

		//  Write events to output file or print to screen
		writeEventsToOutput(events, outputFile)

		fmt.Printf("Processing ledger Seq: %d, ClosedAt: %v, Protocol Version: %v\n",
			ledger.LedgerSequence(), ledger.ClosedAt(), ledger.ProtocolVersion())

		verificationResult := token_transfer.VerifyEvents(ledger, networkPassphrase, false)

		verificationStatus := "success"
		if verificationResult != nil {
			verificationStatus = fmt.Sprintf("FAILED: %v", verificationResult)
		}

		fmt.Printf("Processed ledger Seq: %d, ClosedAt: %v, Protocol Version: %v, num events: %v, verification: %s\n",
			ledger.LedgerSequence(), ledger.ClosedAt(), ledger.ProtocolVersion(), len(events), verificationStatus)

		lineNum++
	}

	fmt.Println("Finished processing ledgers")
}

// Helper function to read a single line from the input file
func readLine(file *os.File) (string, error) {
	var line []byte
	// Read a line from the file
	_, err := fmt.Fscanln(file, &line)
	return string(line), err
}

// Helper function to write events to output (file or screen)
func writeEventsToOutput(events []*token_transfer.TokenTransferEvent, outputFile io.Writer) {
	for _, event := range events {
		jsonBytes, _ := protojson.MarshalOptions{
			Multiline:         true,
			EmitDefaultValues: true,
			Indent:            "  ",
		}.Marshal(event)

		_, err := fmt.Fprintf(outputFile, "### Event Type: %v\n", event.GetEventType())
		if err != nil {
			log.Errorf("Error writing event type to output: %v", err)
			continue
		}

		_, err = fmt.Fprintf(outputFile, "%s\n", string(jsonBytes))
		if err != nil {
			log.Errorf("Error writing event to output: %v", err)
			continue
		}

		_, err = fmt.Fprintf(outputFile, "###\n")
		if err != nil {
			log.Errorf("Error writing separator to output: %v", err)
			continue
		}
	}
}
