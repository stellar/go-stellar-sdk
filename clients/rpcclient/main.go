package rpcclient

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/jhttp"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
)

const (
	DefaultPollTransactionInitialInterval = 500 * time.Millisecond
	DefaultPollTransactionMaxInterval     = 3500 * time.Millisecond
)

// Client is a client for the Stellar RPC JSON-RPC API. It provides methods
// to query network state, submit transactions, and interact with Soroban
// smart contracts. The client is safe for concurrent use.
type Client struct {
	url        string
	cli        *jrpc2.Client
	mx         sync.RWMutex // to protect cli writes in refreshes
	httpClient *http.Client
}

// NewClient creates a new RPC client connected to the given URL. If httpClient
// is nil, http.DefaultClient is used. The client should be closed with Close
// when no longer needed.
func NewClient(url string, httpClient *http.Client) *Client {
	c := &Client{url: url, httpClient: httpClient}
	c.refreshClient()
	return c
}

// Close closes the client connection. After Close is called, the client
// should not be used.
func (c *Client) Close() error {
	c.mx.RLock()
	defer c.mx.RUnlock()
	return c.cli.Close()
}

func (c *Client) refreshClient() {
	var opts *jhttp.ChannelOptions
	if c.httpClient != nil {
		opts = &jhttp.ChannelOptions{
			Client: c.httpClient,
		}
	}
	ch := jhttp.NewChannel(c.url, opts)
	cli := jrpc2.NewClient(ch, nil)

	c.mx.Lock()
	defer c.mx.Unlock()
	if c.cli != nil {
		c.cli.Close()
	}
	c.cli = cli
}

func (c *Client) callResult(ctx context.Context, method string, params, result any) error {
	c.mx.RLock()
	err := c.cli.CallResult(ctx, method, params, result)
	c.mx.RUnlock()
	if err != nil {
		// This is needed because of https://github.com/creachadair/jrpc2/issues/118
		c.refreshClient()
	}
	return err
}

// GetEvents retrieves contract events matching the specified filters. Events
// can be filtered by ledger range, event type, contract ID, and topics. Use
// pagination options to control the number of results returned.
func (c *Client) GetEvents(ctx context.Context,
	request protocol.GetEventsRequest,
) (protocol.GetEventsResponse, error) {
	var result protocol.GetEventsResponse
	err := c.callResult(ctx, protocol.GetEventsMethodName, request, &result)
	if err != nil {
		return protocol.GetEventsResponse{}, err
	}
	return result, nil
}

// GetFeeStats returns statistics about network fees, including percentile data
// for both Soroban and classic transactions. Use this to estimate appropriate
// fees for transaction submission.
func (c *Client) GetFeeStats(ctx context.Context) (protocol.GetFeeStatsResponse, error) {
	var result protocol.GetFeeStatsResponse
	err := c.callResult(ctx, protocol.GetFeeStatsMethodName, nil, &result)
	if err != nil {
		return protocol.GetFeeStatsResponse{}, err
	}
	return result, nil
}

// GetHealth checks the health status of the RPC server. It returns the server
// status, the latest and oldest ledger sequences available, and the ledger
// retention window.
func (c *Client) GetHealth(ctx context.Context) (protocol.GetHealthResponse, error) {
	var result protocol.GetHealthResponse
	err := c.callResult(ctx, protocol.GetHealthMethodName, nil, &result)
	if err != nil {
		return protocol.GetHealthResponse{}, err
	}
	return result, nil
}

// GetLatestLedger returns information about the most recent ledger closed by
// the network. This includes the ledger sequence, hash, close time, and
// protocol version.
func (c *Client) GetLatestLedger(ctx context.Context) (protocol.GetLatestLedgerResponse, error) {
	var result protocol.GetLatestLedgerResponse
	err := c.callResult(ctx, protocol.GetLatestLedgerMethodName, nil, &result)
	if err != nil {
		return protocol.GetLatestLedgerResponse{}, err
	}
	return result, nil
}

// GetLedgerEntries retrieves ledger entries by their keys. Keys must be
// base64-encoded XDR LedgerKey values. This method is used to read account
// state, contract data, trustlines, and other ledger entries.
func (c *Client) GetLedgerEntries(ctx context.Context,
	request protocol.GetLedgerEntriesRequest,
) (protocol.GetLedgerEntriesResponse, error) {
	var result protocol.GetLedgerEntriesResponse
	err := c.callResult(ctx, protocol.GetLedgerEntriesMethodName, request, &result)
	if err != nil {
		return protocol.GetLedgerEntriesResponse{}, err
	}
	return result, nil
}

// GetLedgers retrieves metadata for a range of ledgers. Use pagination to
// control the range and number of results.
func (c *Client) GetLedgers(ctx context.Context,
	request protocol.GetLedgersRequest,
) (protocol.GetLedgersResponse, error) {
	var result protocol.GetLedgersResponse
	err := c.callResult(ctx, protocol.GetLedgersMethodName, request, &result)
	if err != nil {
		return protocol.GetLedgersResponse{}, err
	}
	return result, nil
}

// GetNetwork returns information about the network, including the network
// passphrase, protocol version, and friendbot URL (if available on testnet).
func (c *Client) GetNetwork(ctx context.Context,
) (protocol.GetNetworkResponse, error) {
	var request protocol.GetNetworkRequest
	var result protocol.GetNetworkResponse
	err := c.callResult(ctx, protocol.GetNetworkMethodName, request, &result)
	if err != nil {
		return protocol.GetNetworkResponse{}, err
	}
	return result, nil
}

// GetTransaction retrieves a transaction by its hash. The response includes
// the transaction status (SUCCESS, FAILED, or NOT_FOUND), and if found, the
// full transaction details including envelope, result, and metadata.
func (c *Client) GetTransaction(ctx context.Context,
	request protocol.GetTransactionRequest,
) (protocol.GetTransactionResponse, error) {
	var result protocol.GetTransactionResponse
	err := c.callResult(ctx, protocol.GetTransactionMethodName, request, &result)
	if err != nil {
		return protocol.GetTransactionResponse{}, err
	}
	return result, nil
}

// PollTransactionOptions configures the polling behavior for PollTransaction.
type PollTransactionOptions struct {
	initialInterval time.Duration
	maxInterval     time.Duration
}

// NewPollTransactionOptions returns PollTransactionOptions with default values:
// initial interval of 500ms and max interval of 3500ms.
func NewPollTransactionOptions() PollTransactionOptions {
	return PollTransactionOptions{
		initialInterval: DefaultPollTransactionInitialInterval,
		maxInterval:     DefaultPollTransactionMaxInterval,
	}
}

// WithInitialInterval sets the initial backoff interval between polling attempts.
func (o PollTransactionOptions) WithInitialInterval(d time.Duration) PollTransactionOptions {
	o.initialInterval = d
	return o
}

// WithMaxInterval sets the maximum backoff interval between polling attempts.
func (o PollTransactionOptions) WithMaxInterval(d time.Duration) PollTransactionOptions {
	o.maxInterval = d
	return o
}

// InitialInterval returns the initial backoff interval.
func (o PollTransactionOptions) InitialInterval() time.Duration {
	return o.initialInterval
}

// MaxInterval returns the maximum backoff interval.
func (o PollTransactionOptions) MaxInterval() time.Duration {
	return o.maxInterval
}

// PollTransaction polls GetTransaction until the transaction reaches a terminal
// state (SUCCESS or FAILED) or the context is canceled/times out. It uses
// exponential backoff between polling attempts with default options.
//
// Note: PollTransaction returns the last transaction response even when the status is FAILED.
// Callers must check result.Status to determine if the transaction succeeded or failed,
// since both are valid terminal states that return without error.
func (c *Client) PollTransaction(ctx context.Context,
	txHash string,
) (protocol.GetTransactionResponse, error) {
	return c.PollTransactionWithOptions(ctx, txHash, NewPollTransactionOptions())
}

// PollTransactionWithOptions polls GetTransaction until the transaction reaches a terminal
// state (SUCCESS or FAILED) or the context is canceled/times out. It uses
// exponential backoff between polling attempts.
//
// Note: PollTransactionWithOptions returns the last transaction response even when the status is FAILED.
// Callers must check result.Status to determine if the transaction succeeded or failed,
// since both are valid terminal states that return without error.
func (c *Client) PollTransactionWithOptions(ctx context.Context,
	txHash string,
	opts PollTransactionOptions,
) (protocol.GetTransactionResponse, error) {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = opts.InitialInterval()
	b.MaxInterval = opts.MaxInterval()
	b.MaxElapsedTime = 0 // Disable default 15m limit; rely on context timeout.
	// Note: MaxElapsedTime is now set to 0 because the context timeout
	// controls the overall timeout.

	var result protocol.GetTransactionResponse
	err := backoff.Retry(func() error {
		var err error
		result, err = c.GetTransaction(ctx, protocol.GetTransactionRequest{Hash: txHash})
		if err != nil {
			return backoff.Permanent(err)
		}

		switch result.Status {
		case protocol.TransactionStatusSuccess, protocol.TransactionStatusFailed:
			return nil // Terminal state reached
		default:
			// Transaction not yet finalized (including NOT_FOUND status).
			return fmt.Errorf("transaction not yet finalized: %s", result.Status)
		}
	}, backoff.WithContext(b, ctx))

	if err != nil {
		return protocol.GetTransactionResponse{}, err
	}
	return result, nil
}

// GetTransactions retrieves transactions within a ledger range. Use pagination
// to control the starting ledger and number of results.
func (c *Client) GetTransactions(ctx context.Context,
	request protocol.GetTransactionsRequest,
) (protocol.GetTransactionsResponse, error) {
	var result protocol.GetTransactionsResponse
	err := c.callResult(ctx, protocol.GetTransactionsMethodName, request, &result)
	if err != nil {
		return protocol.GetTransactionsResponse{}, err
	}
	return result, nil
}

// GetVersionInfo returns version information about the RPC server, including
// the software version, commit hash, build timestamp, and supported protocol
// version.
func (c *Client) GetVersionInfo(ctx context.Context) (protocol.GetVersionInfoResponse, error) {
	var result protocol.GetVersionInfoResponse
	err := c.callResult(ctx, protocol.GetVersionInfoMethodName, nil, &result)
	if err != nil {
		return protocol.GetVersionInfoResponse{}, err
	}
	return result, nil
}

// SendTransaction submits a signed transaction to the network. The transaction
// must be a base64-encoded TransactionEnvelope XDR. The response includes
// the transaction hash and submission status. Possible statuses are PENDING
// (accepted for processing), DUPLICATE (already submitted), TRY_AGAIN_LATER
// (server busy), or ERROR (validation failed). Use GetTransaction to poll
// for the final result.
func (c *Client) SendTransaction(ctx context.Context,
	request protocol.SendTransactionRequest,
) (protocol.SendTransactionResponse, error) {
	var result protocol.SendTransactionResponse
	err := c.callResult(ctx, protocol.SendTransactionMethodName, request, &result)
	if err != nil {
		return protocol.SendTransactionResponse{}, err
	}
	return result, nil
}

// SimulateTransaction simulates a transaction without submitting it to the
// network. This is used to estimate resource usage and fees, obtain required
// authorization entries, and check for errors before actual submission.
func (c *Client) SimulateTransaction(ctx context.Context,
	request protocol.SimulateTransactionRequest,
) (protocol.SimulateTransactionResponse, error) {
	var result protocol.SimulateTransactionResponse
	err := c.callResult(ctx, protocol.SimulateTransactionMethodName, request, &result)
	if err != nil {
		return protocol.SimulateTransactionResponse{}, err
	}
	return result, nil
}

// LoadAccount loads an account from the network by its address. The returned
// value implements [txnbuild.Account] and can be used directly with the
// txnbuild package to construct transactions. This is a convenience method
// that fetches the account's current sequence number.
func (c *Client) LoadAccount(ctx context.Context, address string) (txnbuild.Account, error) {
	if !strkey.IsValidEd25519PublicKey(address) {
		return nil, fmt.Errorf("address %s is not a valid Stellar account", address)
	}

	accountID, err := xdr.AddressToAccountId(address)
	if err != nil {
		return nil, err
	}

	lk, err := accountID.LedgerKey()
	if err != nil {
		return nil, err
	}

	accountKey, err := xdr.MarshalBase64(lk)
	if err != nil {
		return nil, err
	}

	resp, err := c.GetLedgerEntries(ctx, protocol.GetLedgerEntriesRequest{
		Keys: []string{accountKey},
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Entries) != 1 {
		return nil, fmt.Errorf("failed to find ledger entry for account %s", address)
	}

	var entry xdr.LedgerEntryData
	if err := xdr.SafeUnmarshalBase64(resp.Entries[0].DataXDR, &entry); err != nil {
		return nil, err
	}

	seqNum := entry.Account.SeqNum
	return &txnbuild.SimpleAccount{AccountID: address, Sequence: int64(seqNum)}, nil
}
