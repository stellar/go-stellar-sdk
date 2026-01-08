/*
Package rpcclient provides a client for interacting with Stellar RPC servers.

This package provides a Go client for the Stellar RPC JSON-RPC API. It enables
applications to simulate transactions, submit transactions, query ledger data,
submit transactions, and monitor network state.

# Creating a Client

Create a client by providing the RPC server URL:

	client := rpcclient.NewClient("https://soroban-testnet.stellar.org", nil)
	defer client.Close()

For custom HTTP settings (timeouts, proxies, etc.), provide your own http.Client:

	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := rpcclient.NewClient("https://soroban-testnet.stellar.org", httpClient)

# Network Information

These methods retrieve information about the RPC server and network:

  - [Client.GetHealth] checks if the RPC server is healthy and returns ledger
    retention information.
  - [Client.GetNetwork] returns the network passphrase and protocol version.
  - [Client.GetFeeStats] returns fee statistics for estimating transaction costs.
  - [Client.GetVersionInfo] returns the RPC server version.
  - [Client.GetLatestLedger] returns the most recent ledger sequence and metadata.

# Querying Ledger Data

These methods query data stored on the ledger:

  - [Client.GetLedgerEntries] fetches specific ledger entries by their keys.
    This is used to read account balances, contract data, and other state.
  - [Client.GetLedgers] retrieves ledger metadata for a range of ledgers.

# Transactions

These methods work with transactions:

  - [Client.SimulateTransaction] simulates a transaction to estimate fees, check
    for errors, and obtain authorization requirements before submission.
  - [Client.SendTransaction] submits a signed transaction to the network.
  - [Client.GetTransaction] retrieves a transaction by its hash.
  - [Client.GetTransactions] queries transactions within a ledger range.
  - [Client.PollTransaction] polls for transaction completion using exponential
    backoff until it reaches a terminal state (SUCCESS or FAILED).
  - [Client.PollTransactionWithOptions] polls for transaction completion with
    custom backoff intervals.

# Events

  - [Client.GetEvents] queries contract events with filters for event type,
    contract ID, and topics. Events are used to track contract activity.

# Transaction Building

The client integrates with the txnbuild package for transaction construction:

  - [Client.LoadAccount] retrieves an account's current sequence number,
    returning a type that implements [txnbuild.Account] for use with txnbuild.

# Request and Response Types

All request and response types are defined in the [protocol] package
(github.com/stellar/go-stellar-sdk/protocols/rpc). Import it to construct
requests and access response fields:

	import protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"

	resp, err := client.GetEvents(ctx, protocol.GetEventsRequest{
		StartLedger: 1000,
		Filters: []protocol.EventFilter{
			{ContractIDs: []string{contractID}},
		},
	})

# Error Handling

All methods return errors from the underlying JSON-RPC transport. Network errors,
invalid requests, and RPC-level errors are all returned as Go errors. Some
response types include an Error field for application-level errors (e.g.,
[protocol.SimulateTransactionResponse.Error]).

[protocol]: https://pkg.go.dev/github.com/stellar/go-stellar-sdk/protocols/rpc
*/
package rpcclient
