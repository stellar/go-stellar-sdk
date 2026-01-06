package rpcclient_test

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/stellar/go-stellar-sdk/clients/rpcclient"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
	"github.com/stellar/go-stellar-sdk/txnbuild"
)

// This example demonstrates how to create an RPC client.
func ExampleNewClient() {
	// Create a client with default HTTP settings
	client := rpcclient.NewClient("https://soroban-testnet.stellar.org", nil)
	defer client.Close()

	// Or with custom HTTP client for timeouts, proxies, etc.
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}
	clientWithCustomHTTP := rpcclient.NewClient("https://soroban-testnet.stellar.org", httpClient)
	defer clientWithCustomHTTP.Close()
}

// This example demonstrates checking the health of an RPC server.
func ExampleClient_GetHealth() {
	client := rpcclient.NewClient("https://soroban-testnet.stellar.org", nil)
	defer client.Close()

	ctx := context.Background()
	health, err := client.GetHealth(ctx)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("Status: %s\n", health.Status)
	fmt.Printf("Latest Ledger: %d\n", health.LatestLedger)
	fmt.Printf("Oldest Ledger: %d\n", health.OldestLedger)
	fmt.Printf("Ledger Retention: %d ledgers\n", health.LedgerRetentionWindow)
}

// This example demonstrates getting network information.
func ExampleClient_GetNetwork() {
	client := rpcclient.NewClient("https://soroban-testnet.stellar.org", nil)
	defer client.Close()

	ctx := context.Background()
	network, err := client.GetNetwork(ctx)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("Network Passphrase: %s\n", network.Passphrase)
	fmt.Printf("Protocol Version: %d\n", network.ProtocolVersion)
	if network.FriendbotURL != "" {
		fmt.Printf("Friendbot URL: %s\n", network.FriendbotURL)
	}
}

// This example demonstrates getting the latest ledger.
func ExampleClient_GetLatestLedger() {
	client := rpcclient.NewClient("https://soroban-testnet.stellar.org", nil)
	defer client.Close()

	ctx := context.Background()
	ledger, err := client.GetLatestLedger(ctx)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("Ledger Sequence: %d\n", ledger.Sequence)
	fmt.Printf("Ledger Hash: %s\n", ledger.Hash)
	fmt.Printf("Protocol Version: %d\n", ledger.ProtocolVersion)
}

// This example demonstrates fetching ledger entries.
func ExampleClient_GetLedgerEntries() {
	client := rpcclient.NewClient("https://soroban-testnet.stellar.org", nil)
	defer client.Close()

	// Ledger entry keys are base64-encoded XDR LedgerKey values.
	// This example uses a placeholder; real usage requires constructing
	// valid keys using the xdr package.
	keys := []string{
		"AAAAB...", // base64-encoded LedgerKey XDR
	}

	ctx := context.Background()
	resp, err := client.GetLedgerEntries(ctx, protocol.GetLedgerEntriesRequest{
		Keys: keys,
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("Latest Ledger: %d\n", resp.LatestLedger)
	for _, entry := range resp.Entries {
		fmt.Printf("Entry Key: %s\n", entry.KeyXDR)
		fmt.Printf("Entry XDR: %s\n", entry.DataXDR)
		fmt.Printf("Last Modified: Ledger %d\n", entry.LastModifiedLedger)
	}
}

// This example demonstrates querying contract events.
func ExampleClient_GetEvents() {
	client := rpcclient.NewClient("https://soroban-testnet.stellar.org", nil)
	defer client.Close()

	// Query contract events starting from a specific ledger
	ctx := context.Background()
	resp, err := client.GetEvents(ctx, protocol.GetEventsRequest{
		StartLedger: 1000000,
		Filters: []protocol.EventFilter{
			{
				// Filter by contract ID (Stellar contract address)
				ContractIDs: []string{
					"CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
				},
			},
		},
		Pagination: &protocol.PaginationOptions{
			Limit: 10,
		},
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("Found %d events\n", len(resp.Events))
	for _, event := range resp.Events {
		fmt.Printf("Event ID: %s, Type: %s, Ledger: %d\n",
			event.ID, event.EventType, event.Ledger)
	}
}

// This example demonstrates simulating a transaction.
func ExampleClient_SimulateTransaction() {
	client := rpcclient.NewClient("https://soroban-testnet.stellar.org", nil)
	defer client.Close()

	// Build a transaction using txnbuild (simplified example)
	// In practice, you would build a real transaction with operations
	sourceKey := keypair.MustRandom()
	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount: &txnbuild.SimpleAccount{
				AccountID: sourceKey.Address(),
				Sequence:  1,
			},
			Operations: []txnbuild.Operation{
				// Add your operations here
			},
			BaseFee:       txnbuild.MinBaseFee,
			Preconditions: txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
		},
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Get the transaction envelope as base64 XDR
	txXDR, err := tx.Base64()
	if err != nil {
		fmt.Println(err)
		return
	}

	// Simulate the transaction
	ctx := context.Background()
	resp, err := client.SimulateTransaction(ctx, protocol.SimulateTransactionRequest{
		Transaction: txXDR,
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	if resp.Error != "" {
		fmt.Printf("Simulation error: %s\n", resp.Error)
		return
	}

	fmt.Printf("Min Resource Fee: %d stroops\n", resp.MinResourceFee)
	fmt.Printf("Latest Ledger: %d\n", resp.LatestLedger)
}

// This example demonstrates submitting a transaction.
func ExampleClient_SendTransaction() {
	client := rpcclient.NewClient("https://soroban-testnet.stellar.org", nil)
	defer client.Close()

	// In practice, you would:
	// 1. Build a transaction with txnbuild
	// 2. Simulate it to get resource requirements
	// 3. Sign it
	// 4. Submit it

	// This example shows the submission step with a pre-signed transaction
	signedTxXDR := "AAAAAgAAAA..." // base64-encoded signed TransactionEnvelope

	ctx := context.Background()
	resp, err := client.SendTransaction(ctx, protocol.SendTransactionRequest{
		Transaction: signedTxXDR,
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("Transaction Hash: %s\n", resp.Hash)
	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Latest Ledger: %d\n", resp.LatestLedger)
}

// This example demonstrates looking up a transaction by hash.
func ExampleClient_GetTransaction() {
	client := rpcclient.NewClient("https://soroban-testnet.stellar.org", nil)
	defer client.Close()

	// Look up a transaction by its hash
	txHash := "abc123..." // hex-encoded transaction hash
	ctx := context.Background()
	resp, err := client.GetTransaction(ctx, protocol.GetTransactionRequest{
		Hash: txHash,
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("Status: %s\n", resp.Status)
	if resp.Status == protocol.TransactionStatusSuccess {
		fmt.Printf("Ledger: %d\n", resp.Ledger)
		fmt.Printf("Application Order: %d\n", resp.ApplicationOrder)
	}
}

// This example demonstrates loading an account for transaction building.
func ExampleClient_LoadAccount() {
	client := rpcclient.NewClient("https://soroban-testnet.stellar.org", nil)
	defer client.Close()

	// Load an account to get its current sequence number
	// The returned type implements txnbuild.Account
	ctx := context.Background()
	account, err := client.LoadAccount(ctx,
		"GAIH3ULLFQ4DGSECF2AR555KZ4KNDGEKN4AFI4SU2M7B43MGK3QJZNSR")
	if err != nil {
		fmt.Println(err)
		return
	}

	// Use the account with txnbuild to construct transactions
	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount: account,
			Operations:    []txnbuild.Operation{
				// Add your operations here
			},
			BaseFee:       txnbuild.MinBaseFee,
			Preconditions: txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
		},
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Sign the transaction
	tx, err = tx.Sign(network.TestNetworkPassphrase, keypair.MustRandom())
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Transaction built successfully")
}
