package client

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/jhttp"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"

	protocol "github.com/stellar/go-stellar-sdk/protocols/rpc"
)

type Client struct {
	url        string
	cli        *jrpc2.Client
	mx         sync.RWMutex // to protect cli writes in refreshes
	httpClient *http.Client
}

func NewClient(url string, httpClient *http.Client) *Client {
	c := &Client{url: url, httpClient: httpClient}
	c.refreshClient()
	return c
}

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

func (c *Client) GetFeeStats(ctx context.Context) (protocol.GetFeeStatsResponse, error) {
	var result protocol.GetFeeStatsResponse
	err := c.callResult(ctx, protocol.GetFeeStatsMethodName, nil, &result)
	if err != nil {
		return protocol.GetFeeStatsResponse{}, err
	}
	return result, nil
}

func (c *Client) GetHealth(ctx context.Context) (protocol.GetHealthResponse, error) {
	var result protocol.GetHealthResponse
	err := c.callResult(ctx, protocol.GetHealthMethodName, nil, &result)
	if err != nil {
		return protocol.GetHealthResponse{}, err
	}
	return result, nil
}

func (c *Client) GetLatestLedger(ctx context.Context) (protocol.GetLatestLedgerResponse, error) {
	var result protocol.GetLatestLedgerResponse
	err := c.callResult(ctx, protocol.GetLatestLedgerMethodName, nil, &result)
	if err != nil {
		return protocol.GetLatestLedgerResponse{}, err
	}
	return result, nil
}

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

func (c *Client) GetNetwork(ctx context.Context,
) (protocol.GetNetworkResponse, error) {
	// phony
	var request protocol.GetNetworkRequest
	var result protocol.GetNetworkResponse
	err := c.callResult(ctx, protocol.GetNetworkMethodName, request, &result)
	if err != nil {
		return protocol.GetNetworkResponse{}, err
	}
	return result, nil
}

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

func (c *Client) GetVersionInfo(ctx context.Context) (protocol.GetVersionInfoResponse, error) {
	var result protocol.GetVersionInfoResponse
	err := c.callResult(ctx, protocol.GetVersionInfoMethodName, nil, &result)
	if err != nil {
		return protocol.GetVersionInfoResponse{}, err
	}
	return result, nil
}

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
