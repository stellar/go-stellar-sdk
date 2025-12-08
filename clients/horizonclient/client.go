package horizonclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/manucorporat/sse"

	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stellar/go-stellar-sdk/protocols/horizon/effects"
	"github.com/stellar/go-stellar-sdk/protocols/horizon/operations"
	"github.com/stellar/go-stellar-sdk/support/errors"
)

// sendRequest builds the URL for the given horizon request and sends the url to a horizon server
func (c *Client) sendRequest(hr HorizonRequest, resp interface{}) (err error) {
	req, err := hr.HTTPRequest(c.fixHorizonURL())
	if err != nil {
		return err
	}

	return c.sendHTTPRequest(req, resp)
}

// checkMemoRequired implements a memo required check as defined in
// https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0029.md
func (c *Client) checkMemoRequired(transaction *txnbuild.Transaction) error {
	destinations := map[string]bool{}

	for i, op := range transaction.Operations() {
		var destination string

		if err := op.Validate(); err != nil {
			return err
		}

		switch p := op.(type) {
		case *txnbuild.Payment:
			destination = p.Destination
		case *txnbuild.PathPaymentStrictReceive:
			destination = p.Destination
		case *txnbuild.PathPaymentStrictSend:
			destination = p.Destination
		case *txnbuild.AccountMerge:
			destination = p.Destination
		default:
			continue
		}

		muxed, err := xdr.AddressToMuxedAccount(destination)
		if err != nil {
			return errors.Wrapf(err, "destination %v is not a valid address", destination)
		}
		// Skip destination addresses with a memo id because the address has a memo
		// encoded within it
		destinationHasMemoID := muxed.Type == xdr.CryptoKeyTypeKeyTypeMuxedEd25519

		if destinations[destination] || destinationHasMemoID {
			continue
		}
		destinations[destination] = true

		request := AccountRequest{
			AccountID: destination,
			DataKey:   "config.memo_required",
		}

		data, err := c.AccountData(request)
		if err != nil {
			horizonError := GetError(err)

			if horizonError == nil || horizonError.Response.StatusCode != 404 {
				return err
			}

			continue
		}

		if data.Value == accountRequiresMemo {
			return errors.Wrap(
				ErrAccountRequiresMemo,
				fmt.Sprintf("operation[%d]", i),
			)
		}
	}

	return nil
}

// sendGetRequest sends a HTTP GET request to a horizon server.
// It can be used for requests that do not implement the HorizonRequest interface.
func (c *Client) sendGetRequest(requestURL string, a interface{}) error {
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return errors.Wrap(err, "error creating HTTP request")
	}
	return c.sendHTTPRequest(req, a)
}

func (c *Client) sendHTTPRequest(req *http.Request, a interface{}) error {
	c.setClientAppHeaders(req)
	c.setDefaultClient()

	if c.horizonTimeout == 0 {
		c.horizonTimeout = HorizonTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.horizonTimeout)
	defer cancel()

	if resp, err := c.HTTP.Do(req.WithContext(ctx)); err != nil {
		return err
	} else {
		return decodeResponse(resp, a, c.HorizonURL, c.clock)
	}
}

// stream handles connections to endpoints that support streaming on a horizon server
func (c *Client) stream(
	ctx context.Context,
	streamURL string,
	handler func(data []byte) error,
) error {
	su, err := url.Parse(streamURL)
	if err != nil {
		return errors.Wrap(err, "error parsing stream url")
	}

	query := su.Query()
	if query.Get("cursor") == "" {
		query.Set("cursor", "now")
	}

	for {
		// Check if ctx is not canceled
		select {
		case <-ctx.Done():
			return nil
		default:
			// Continue
		}

		// If err is nil, reconnect and try again.
		if err = c.streamConnection(ctx, su, query, handler); err != nil {
			return err
		}
	}
}

func (c *Client) streamConnection(
	ctx context.Context,
	su *url.URL,
	query url.Values,
	handler func(data []byte) error,
) error {
	// updates the url with a new cursor
	su.RawQuery = query.Encode()
	req, err := http.NewRequest("GET", su.String(), nil)
	if err != nil {
		return errors.Wrap(err, "error creating HTTP request")
	}
	req.Header.Set("Accept", "text/event-stream")
	c.setDefaultClient()
	c.setClientAppHeaders(req)

	// We can use c.HTTP here because we set Timeout per request not on the client. See sendRequest()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return errors.Wrap(err, "error sending HTTP request")
	}
	defer resp.Body.Close()

	// Expected statusCode are 200-299
	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		return fmt.Errorf("got bad HTTP status code %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)

	// Read events one by one. Break this loop when there is no more data to be
	// read from resp.Body (io.EOF).
Events:
	for {
		// Read until empty line = event delimiter. The perfect solution would be to read
		// as many bytes as possible and forward them to sse.Decode. However this
		// requires much more complicated code.
		// We could also write our own `sse` package that works fine with streams directly
		// (github.com/manucorporat/sse is just using io/ioutils.ReadAll).
		var buffer bytes.Buffer
		nonEmptylinesRead := 0
		for {
			// Check if ctx is not canceled
			select {
			case <-ctx.Done():
				return nil
			default:
				// Continue
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					// We catch EOF errors to handle two possible situations:
					// - The last line before closing the stream was not empty. This should never
					//   happen in Horizon as it always sends an empty line after each event.
					// - The stream was closed by the server/proxy because the connection was idle.
					//
					// In the former case, that (again) should never happen in Horizon, we need to
					// check if there are any events we need to decode. We do this in the `if`
					// statement below just in case if Horizon behavior changes in a future.
					//
					// From spec:
					// > Once the end of the file is reached, the user agent must dispatch the
					// > event one final time, as defined below.
					if nonEmptylinesRead == 0 {
						break Events
					}
				} else {
					return errors.Wrap(err, "error reading line")
				}
			}
			buffer.WriteString(line)

			if strings.TrimRight(line, "\n\r") == "" {
				break
			}

			nonEmptylinesRead++
		}

		events, err := sse.Decode(strings.NewReader(buffer.String()))
		if err != nil {
			return errors.Wrap(err, "error decoding event")
		}

		// Right now len(events) should always be 1. This loop will be helpful after writing
		// new SSE decoder that can handle io.Reader without using ioutils.ReadAll().
		for _, event := range events {
			if event.Event != "message" {
				continue
			}

			// Update cursor with event ID
			if event.Id != "" {
				query.Set("cursor", event.Id)
			}

			switch data := event.Data.(type) {
			case string:
				err = handler([]byte(data))
				err = errors.Wrap(err, "handler error")
			case []byte:
				err = handler(data)
				err = errors.Wrap(err, "handler error")
			default:
				err = errors.New("invalid event.Data type")
			}
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Client) setClientAppHeaders(req *http.Request) {
	for key, value := range c.Headers {
		req.Header.Set(key, value)
	}

	req.Header.Set("X-Client-Name", "go-stellar-sdk")
	req.Header.Set("X-Client-Version", c.Version())
	req.Header.Set("X-App-Name", c.AppName)
	req.Header.Set("X-App-Version", c.AppVersion)
}

// setDefaultClient sets the default HTTP client when none is provided.
func (c *Client) setDefaultClient() {
	if c.HTTP == nil {
		c.HTTP = http.DefaultClient
	}
}

// fixHorizonURL strips all slashes(/) at the end of HorizonURL if any, then adds a single slash
func (c *Client) fixHorizonURL() string {
	c.fixHorizonURLOnce.Do(func() {
		// TODO: we shouldn't happily edit data provided by the user,
		//       better store it in an internal variable or, even better,
		//       just parse it every time (what if the url changes during the life of the client?).
		c.HorizonURL = strings.TrimRight(c.HorizonURL, "/") + "/"
	})
	return c.HorizonURL
}

// SetHorizonTimeout allows users to set the timeout before a horizon request is canceled.
// The timeout is specified as a time.Duration which is in nanoseconds.
func (c *Client) SetHorizonTimeout(t time.Duration) *Client {
	c.horizonTimeout = t
	return c
}

// HorizonTimeout returns the current timeout for a horizon client
func (c *Client) HorizonTimeout() time.Duration {
	return c.horizonTimeout
}

// Accounts returns accounts who have a given signer or
// have a trustline to an asset.
// See https://developers.stellar.org/api/resources/accounts/
func (c *Client) Accounts(request AccountsRequest) (accounts hProtocol.AccountsPage, err error) {
	err = c.sendRequest(request, &accounts)
	return
}

// AccountDetail returns information for a single account.
// See https://developers.stellar.org/api/resources/accounts/single/
func (c *Client) AccountDetail(request AccountRequest) (account hProtocol.Account, err error) {
	if request.AccountID == "" {
		err = errors.New("no account ID provided")
	}

	if err != nil {
		return
	}

	err = c.sendRequest(request, &account)
	return
}

// AccountData returns a single data associated with a given account
// See https://developers.stellar.org/api/resources/accounts/data/
func (c *Client) AccountData(request AccountRequest) (accountData hProtocol.AccountData, err error) {
	if request.AccountID == "" || request.DataKey == "" {
		err = errors.New("too few parameters")
	}

	if err != nil {
		return
	}

	err = c.sendRequest(request, &accountData)
	return
}

// Effects returns effects (https://developers.stellar.org/api/resources/effects/)
// It can be used to return effects for an account, a ledger, an operation, a transaction and all effects on the network.
func (c *Client) Effects(request EffectRequest) (effects effects.EffectsPage, err error) {
	err = c.sendRequest(request, &effects)
	return
}

// Assets returns asset information.
// See https://developers.stellar.org/api/resources/assets/list/
func (c *Client) Assets(request AssetRequest) (assets hProtocol.AssetsPage, err error) {
	err = c.sendRequest(request, &assets)
	return
}

// Ledgers returns information about all ledgers.
// See https://developers.stellar.org/api/resources/ledgers/list/
func (c *Client) Ledgers(request LedgerRequest) (ledgers hProtocol.LedgersPage, err error) {
	err = c.sendRequest(request, &ledgers)
	return
}

// LedgerDetail returns information about a particular ledger for a given sequence number
// See https://developers.stellar.org/api/resources/ledgers/single/
func (c *Client) LedgerDetail(sequence uint32) (ledger hProtocol.Ledger, err error) {
	if sequence == 0 {
		err = errors.New("invalid sequence number provided")
	}

	if err != nil {
		return
	}

	request := LedgerRequest{forSequence: sequence}
	err = c.sendRequest(request, &ledger)
	return
}

// FeeStats returns information about fees in the last 5 ledgers.
// See https://developers.stellar.org/api/aggregations/fee-stats/
func (c *Client) FeeStats() (feestats hProtocol.FeeStats, err error) {
	request := feeStatsRequest{endpoint: "fee_stats"}
	err = c.sendRequest(request, &feestats)
	return
}

// Offers returns information about offers made on the SDEX.
// See https://developers.stellar.org/api/resources/offers/list/
func (c *Client) Offers(request OfferRequest) (offers hProtocol.OffersPage, err error) {
	err = c.sendRequest(request, &offers)
	return
}

// OfferDetails returns information for a single offer.
// See https://developers.stellar.org/api/resources/offers/single/
func (c *Client) OfferDetails(offerID string) (offer hProtocol.Offer, err error) {
	if len(offerID) == 0 {
		err = errors.New("no offer ID provided")
		return
	}

	if _, err = strconv.ParseInt(offerID, 10, 64); err != nil {
		err = errors.New("invalid offer ID provided")
		return
	}

	err = c.sendRequest(OfferRequest{OfferID: offerID}, &offer)
	return
}

// Operations returns stellar operations (https://developers.stellar.org/api/resources/operations/list/)
// It can be used to return operations for an account, a ledger, a transaction and all operations on the network.
func (c *Client) Operations(request OperationRequest) (ops operations.OperationsPage, err error) {
	err = c.sendRequest(request.SetOperationsEndpoint(), &ops)
	return
}

// OperationDetail returns a single stellar operation for a given operation id
// See https://developers.stellar.org/api/resources/operations/single/
func (c *Client) OperationDetail(id string) (ops operations.Operation, err error) {
	if id == "" {
		return ops, errors.New("invalid operation id provided")
	}

	request := OperationRequest{forOperationID: id, endpoint: "operations"}

	var record interface{}

	err = c.sendRequest(request, &record)
	if err != nil {
		return ops, errors.Wrap(err, "sending request to horizon")
	}

	var baseRecord operations.Base
	dataString, err := json.Marshal(record)
	if err != nil {
		return ops, errors.Wrap(err, "marshaling json")
	}
	if err = json.Unmarshal(dataString, &baseRecord); err != nil {
		return ops, errors.Wrap(err, "unmarshaling json")
	}

	ops, err = operations.UnmarshalOperation(baseRecord.GetTypeI(), dataString)
	if err != nil {
		return ops, errors.Wrap(err, "unmarshaling to the correct operation type")
	}
	return ops, nil
}

// validateFeeBumpTx checks if the inner transaction has a memo or not and converts the transaction object to
// base64 string.
func (c *Client) validateFeeBumpTx(transaction *txnbuild.FeeBumpTransaction, opts SubmitTxOpts) (string, error) {
	var err error
	if inner := transaction.InnerTransaction(); !opts.SkipMemoRequiredCheck && inner.Memo() == nil {
		err = c.checkMemoRequired(inner)
		if err != nil {
			return "", err
		}
	}

	txeBase64, err := transaction.Base64()
	if err != nil {
		err = errors.Wrap(err, "Unable to convert transaction object to base64 string")
		return "", err
	}
	return txeBase64, nil
}

// validateTx checks if the transaction has a memo or not and converts the transaction object to
// base64 string.
func (c *Client) validateTx(transaction *txnbuild.Transaction, opts SubmitTxOpts) (string, error) {
	var err error
	if !opts.SkipMemoRequiredCheck && transaction.Memo() == nil {
		err = c.checkMemoRequired(transaction)
		if err != nil {
			return "", err
		}
	}

	txeBase64, err := transaction.Base64()
	if err != nil {
		err = errors.Wrap(err, "Unable to convert transaction object to base64 string")
		return "", err
	}
	return txeBase64, nil
}

// SubmitTransactionXDR submits a transaction represented as a base64 XDR string to the network. err can be either error object or horizon.Error object.
// See https://developers.stellar.org/api/resources/transactions/post/
func (c *Client) SubmitTransactionXDR(transactionXdr string) (tx hProtocol.Transaction,
	err error) {
	request := submitRequest{endpoint: "transactions", transactionXdr: transactionXdr}
	err = c.sendRequest(request, &tx)
	return
}

// SubmitFeeBumpTransaction submits a fee bump transaction to the network. err can be either an
// error object or a horizon.Error object.
//
// This function will always check if the destination account requires a memo in the transaction as
// defined in SEP0029: https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0029.md
//
// If you want to skip this check, use SubmitTransactionWithOptions.
//
// See https://developers.stellar.org/api/resources/transactions/post/
func (c *Client) SubmitFeeBumpTransaction(transaction *txnbuild.FeeBumpTransaction) (tx hProtocol.Transaction, err error) {
	return c.SubmitFeeBumpTransactionWithOptions(transaction, SubmitTxOpts{})
}

// SubmitFeeBumpTransactionWithOptions submits a fee bump transaction to the network, allowing
// you to pass SubmitTxOpts. err can be either an error object or a horizon.Error object.
//
// See https://developers.stellar.org/api/resources/transactions/post/
func (c *Client) SubmitFeeBumpTransactionWithOptions(transaction *txnbuild.FeeBumpTransaction, opts SubmitTxOpts) (tx hProtocol.Transaction, err error) {
	// only check if memo is required if skip is false and the inner transaction
	// doesn't have a memo.
	txeBase64, err := c.validateFeeBumpTx(transaction, opts)
	if err != nil {
		return
	}

	return c.SubmitTransactionXDR(txeBase64)
}

// SubmitTransaction submits a transaction to the network. err can be either an
// error object or a horizon.Error object.
//
// This function will always check if the destination account requires a memo in the transaction as
// defined in SEP0029: https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0029.md
//
// If you want to skip this check, use SubmitTransactionWithOptions.
//
// See https://developers.stellar.org/api/resources/transactions/post/
func (c *Client) SubmitTransaction(transaction *txnbuild.Transaction) (tx hProtocol.Transaction, err error) {
	return c.SubmitTransactionWithOptions(transaction, SubmitTxOpts{})
}

// SubmitTransactionWithOptions submits a transaction to the network, allowing
// you to pass SubmitTxOpts. err can be either an error object or a horizon.Error object.
//
// See https://developers.stellar.org/api/resources/transactions/post/
func (c *Client) SubmitTransactionWithOptions(transaction *txnbuild.Transaction, opts SubmitTxOpts) (tx hProtocol.Transaction, err error) {
	// only check if memo is required if skip is false and the transaction
	// doesn't have a memo.
	txeBase64, err := c.validateTx(transaction, opts)
	if err != nil {
		return
	}

	return c.SubmitTransactionXDR(txeBase64)
}

// AsyncSubmitTransactionXDR submits a base64 XDR transaction using the transactions_async endpoint. err can be either error object or horizon.Error object.
func (c *Client) AsyncSubmitTransactionXDR(transactionXdr string) (txResp hProtocol.AsyncTransactionSubmissionResponse,
	err error) {
	request := submitRequest{endpoint: "transactions_async", transactionXdr: transactionXdr}
	err = c.sendRequest(request, &txResp)
	return
}

// AsyncSubmitFeeBumpTransaction submits an async fee bump transaction to the network. err can be either an
// error object or a horizon.Error object.
//
// This function will always check if the destination account requires a memo in the transaction as
// defined in SEP0029: https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0029.md
//
// If you want to skip this check, use SubmitTransactionWithOptions.
func (c *Client) AsyncSubmitFeeBumpTransaction(transaction *txnbuild.FeeBumpTransaction) (txResp hProtocol.AsyncTransactionSubmissionResponse, err error) {
	return c.AsyncSubmitFeeBumpTransactionWithOptions(transaction, SubmitTxOpts{})
}

// AsyncSubmitFeeBumpTransactionWithOptions submits an async fee bump transaction to the network, allowing
// you to pass SubmitTxOpts. err can be either an error object or a horizon.Error object.
func (c *Client) AsyncSubmitFeeBumpTransactionWithOptions(transaction *txnbuild.FeeBumpTransaction, opts SubmitTxOpts) (txResp hProtocol.AsyncTransactionSubmissionResponse, err error) {
	// only check if memo is required if skip is false and the inner transaction
	// doesn't have a memo.
	txeBase64, err := c.validateFeeBumpTx(transaction, opts)
	if err != nil {
		return
	}

	return c.AsyncSubmitTransactionXDR(txeBase64)
}

// AsyncSubmitTransaction submits an async transaction to the network. err can be either an
// error object or a horizon.Error object.
//
// This function will always check if the destination account requires a memo in the transaction as
// defined in SEP0029: https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0029.md
//
// If you want to skip this check, use SubmitTransactionWithOptions.
func (c *Client) AsyncSubmitTransaction(transaction *txnbuild.Transaction) (txResp hProtocol.AsyncTransactionSubmissionResponse, err error) {
	return c.AsyncSubmitTransactionWithOptions(transaction, SubmitTxOpts{})
}

// AsyncSubmitTransactionWithOptions submits an async transaction to the network, allowing
// you to pass SubmitTxOpts. err can be either an error object or a horizon.Error object.
func (c *Client) AsyncSubmitTransactionWithOptions(transaction *txnbuild.Transaction, opts SubmitTxOpts) (txResp hProtocol.AsyncTransactionSubmissionResponse, err error) {
	// only check if memo is required if skip is false and the transaction
	// doesn't have a memo.
	txeBase64, err := c.validateTx(transaction, opts)
	if err != nil {
		return
	}

	return c.AsyncSubmitTransactionXDR(txeBase64)
}

// Transactions returns stellar transactions (https://developers.stellar.org/api/resources/transactions/list/)
// It can be used to return transactions for an account, a ledger,and all transactions on the network.
func (c *Client) Transactions(request TransactionRequest) (txs hProtocol.TransactionsPage, err error) {
	err = c.sendRequest(request, &txs)
	return
}

// TransactionDetail returns information about a particular transaction for a given transaction hash
// See https://developers.stellar.org/api/resources/transactions/single/
func (c *Client) TransactionDetail(txHash string) (tx hProtocol.Transaction, err error) {
	if txHash == "" {
		return tx, errors.New("no transaction hash provided")
	}

	request := TransactionRequest{forTransactionHash: txHash}
	err = c.sendRequest(request, &tx)
	return
}

// OrderBook returns the orderbook for an asset pair (https://developers.stellar.org/api/aggregations/order-books/single/)
func (c *Client) OrderBook(request OrderBookRequest) (obs hProtocol.OrderBookSummary, err error) {
	err = c.sendRequest(request, &obs)
	return
}

// Paths returns the available paths to make a strict receive path payment. See https://developers.stellar.org/api/aggregations/paths/strict-receive/
// This function is an alias for `client.StrictReceivePaths` and will be deprecated, use `client.StrictReceivePaths` instead.
func (c *Client) Paths(request PathsRequest) (paths hProtocol.PathsPage, err error) {
	paths, err = c.StrictReceivePaths(request)
	return
}

// StrictReceivePaths returns the available paths to make a strict receive path payment. See https://developers.stellar.org/api/aggregations/paths/strict-receive/
func (c *Client) StrictReceivePaths(request PathsRequest) (paths hProtocol.PathsPage, err error) {
	err = c.sendRequest(request, &paths)
	return
}

// StrictSendPaths returns the available paths to make a strict send path payment. See https://developers.stellar.org/api/aggregations/paths/strict-send/
func (c *Client) StrictSendPaths(request StrictSendPathsRequest) (paths hProtocol.PathsPage, err error) {
	err = c.sendRequest(request, &paths)
	return
}

// Payments returns stellar account_merge, create_account, path payment and payment operations.
// It can be used to return payments for an account, a ledger, a transaction and all payments on the network.
func (c *Client) Payments(request OperationRequest) (ops operations.OperationsPage, err error) {
	err = c.sendRequest(request.SetPaymentsEndpoint(), &ops)
	return
}

// Trades returns stellar trades (https://developers.stellar.org/api/resources/trades/list/)
// It can be used to return trades for an account, an offer and all trades on the network.
func (c *Client) Trades(request TradeRequest) (tds hProtocol.TradesPage, err error) {
	err = c.sendRequest(request, &tds)
	return
}

// Fund creates a new account funded from friendbot. It only works on test networks. See
// https://developers.stellar.org/docs/tutorials/create-account/ for more information.
func (c *Client) Fund(addr string) (tx hProtocol.Transaction, err error) {
	friendbotURL := fmt.Sprintf("%sfriendbot?addr=%s", c.fixHorizonURL(), addr)
	err = c.sendGetRequest(friendbotURL, &tx)
	if IsNotFoundError(err) {
		return tx, errors.Wrap(err, "funding is only available on test networks and may not be supported by "+c.fixHorizonURL())
	}
	return
}

// StreamTrades streams executed trades. It can be used to stream all trades, trades for an account and
// trades for an offer. Use context.WithCancel to stop streaming or context.Background() if you want
// to stream indefinitely. TradeHandler is a user-supplied function that is executed for each streamed trade received.
func (c *Client) StreamTrades(ctx context.Context, request TradeRequest, handler TradeHandler) (err error) {
	err = request.StreamTrades(ctx, c, handler)
	return
}

// TradeAggregations returns stellar trade aggregations (https://developers.stellar.org/api/aggregations/trade-aggregations/list/)
func (c *Client) TradeAggregations(request TradeAggregationRequest) (tds hProtocol.TradeAggregationsPage, err error) {
	err = c.sendRequest(request, &tds)
	return
}

// StreamTransactions streams processed transactions. It can be used to stream all transactions and
// transactions for an account. Use context.WithCancel to stop streaming or context.Background()
// if you want to stream indefinitely. TransactionHandler is a user-supplied function that is executed for each streamed transaction received.
func (c *Client) StreamTransactions(ctx context.Context, request TransactionRequest, handler TransactionHandler) error {
	return request.StreamTransactions(ctx, c, handler)
}

// StreamEffects streams horizon effects. It can be used to stream all effects or account specific effects.
// Use context.WithCancel to stop streaming or context.Background() if you want to stream indefinitely.
// EffectHandler is a user-supplied function that is executed for each streamed transaction received.
func (c *Client) StreamEffects(ctx context.Context, request EffectRequest, handler EffectHandler) error {
	return request.StreamEffects(ctx, c, handler)
}

// StreamOperations streams stellar operations. It can be used to stream all operations or operations
// for an account. Use context.WithCancel to stop streaming or context.Background() if you want to
// stream indefinitely. OperationHandler is a user-supplied function that is executed for each streamed
// operation received.
func (c *Client) StreamOperations(ctx context.Context, request OperationRequest, handler OperationHandler) error {
	return request.SetOperationsEndpoint().StreamOperations(ctx, c, handler)
}

// StreamPayments streams stellar payments. It can be used to stream all payments or payments
// for an account. Payments include create_account, payment, path_payment and account_merge operations.
// Use context.WithCancel to stop streaming or context.Background() if you want to
// stream indefinitely. OperationHandler is a user-supplied function that is executed for each streamed
// operation received.
func (c *Client) StreamPayments(ctx context.Context, request OperationRequest, handler OperationHandler) error {
	return request.SetPaymentsEndpoint().StreamOperations(ctx, c, handler)
}

// StreamOffers streams offers processed by the Stellar network for an account. Use context.WithCancel
// to stop streaming or context.Background() if you want to stream indefinitely.
// OfferHandler is a user-supplied function that is executed for each streamed offer received.
func (c *Client) StreamOffers(ctx context.Context, request OfferRequest, handler OfferHandler) error {
	return request.StreamOffers(ctx, c, handler)
}

// StreamLedgers streams stellar ledgers. It can be used to stream all ledgers. Use context.WithCancel
// to stop streaming or context.Background() if you want to stream indefinitely.
// LedgerHandler is a user-supplied function that is executed for each streamed ledger received.
func (c *Client) StreamLedgers(ctx context.Context, request LedgerRequest, handler LedgerHandler) error {
	return request.StreamLedgers(ctx, c, handler)
}

// StreamOrderBooks streams the orderbook for a given asset pair. Use context.WithCancel
// to stop streaming or context.Background() if you want to stream indefinitely.
// OrderBookHandler is a user-supplied function that is executed for each streamed order received.
func (c *Client) StreamOrderBooks(ctx context.Context, request OrderBookRequest, handler OrderBookHandler) error {
	return request.StreamOrderBooks(ctx, c, handler)
}

// FetchTimebounds provides timebounds for N seconds from now using the server time of the horizon instance.
// It defaults to localtime when the server time is not available.
// Note that this will generate your timebounds when you init the transaction, not when you build or submit
// the transaction! So give yourself enough time to get the transaction built and signed before submitting.
func (c *Client) FetchTimebounds(seconds int64) (txnbuild.TimeBounds, error) {
	serverURL, err := url.Parse(c.HorizonURL)
	if err != nil {
		return txnbuild.TimeBounds{}, errors.Wrap(err, "unable to parse horizon url")
	}
	currentTime := currentServerTime(serverURL.Hostname(), c.clock.Now().UTC().Unix())
	if currentTime != 0 {
		return txnbuild.NewTimebounds(0, currentTime+seconds), nil
	}

	// return a timebounds based on local time if no server time has been recorded
	// to do: query an endpoint to get the most current time. Implement this after we add retry logic to client.
	return txnbuild.NewTimeout(seconds), nil
}

// Root loads the root endpoint of horizon
func (c *Client) Root() (root hProtocol.Root, err error) {
	err = c.sendGetRequest(c.fixHorizonURL(), &root)
	return
}

// Version returns the current version.
func (c *Client) Version() string {
	return version
}

// NextAccountsPage returns the next page of accounts.
func (c *Client) NextAccountsPage(page hProtocol.AccountsPage) (accounts hProtocol.AccountsPage, err error) {
	err = c.sendGetRequest(page.Links.Next.Href, &accounts)
	return
}

// NextAssetsPage returns the next page of assets.
func (c *Client) NextAssetsPage(page hProtocol.AssetsPage) (assets hProtocol.AssetsPage, err error) {
	err = c.sendGetRequest(page.Links.Next.Href, &assets)
	return
}

// PrevAssetsPage returns the previous page of assets.
func (c *Client) PrevAssetsPage(page hProtocol.AssetsPage) (assets hProtocol.AssetsPage, err error) {
	err = c.sendGetRequest(page.Links.Prev.Href, &assets)
	return
}

// NextLedgersPage returns the next page of ledgers.
func (c *Client) NextLedgersPage(page hProtocol.LedgersPage) (ledgers hProtocol.LedgersPage, err error) {
	err = c.sendGetRequest(page.Links.Next.Href, &ledgers)
	return
}

// PrevLedgersPage returns the previous page of ledgers.
func (c *Client) PrevLedgersPage(page hProtocol.LedgersPage) (ledgers hProtocol.LedgersPage, err error) {
	err = c.sendGetRequest(page.Links.Prev.Href, &ledgers)
	return
}

// NextEffectsPage returns the next page of effects.
func (c *Client) NextEffectsPage(page effects.EffectsPage) (efp effects.EffectsPage, err error) {
	err = c.sendGetRequest(page.Links.Next.Href, &efp)
	return
}

// PrevEffectsPage returns the previous page of effects.
func (c *Client) PrevEffectsPage(page effects.EffectsPage) (efp effects.EffectsPage, err error) {
	err = c.sendGetRequest(page.Links.Prev.Href, &efp)
	return
}

// NextTransactionsPage returns the next page of transactions.
func (c *Client) NextTransactionsPage(page hProtocol.TransactionsPage) (transactions hProtocol.TransactionsPage, err error) {
	err = c.sendGetRequest(page.Links.Next.Href, &transactions)
	return
}

// PrevTransactionsPage returns the previous page of transactions.
func (c *Client) PrevTransactionsPage(page hProtocol.TransactionsPage) (transactions hProtocol.TransactionsPage, err error) {
	err = c.sendGetRequest(page.Links.Prev.Href, &transactions)
	return
}

// NextOperationsPage returns the next page of operations.
func (c *Client) NextOperationsPage(page operations.OperationsPage) (operations operations.OperationsPage, err error) {
	err = c.sendGetRequest(page.Links.Next.Href, &operations)
	return
}

// PrevOperationsPage returns the previous page of operations.
func (c *Client) PrevOperationsPage(page operations.OperationsPage) (operations operations.OperationsPage, err error) {
	err = c.sendGetRequest(page.Links.Prev.Href, &operations)
	return
}

// NextPaymentsPage returns the next page of payments.
func (c *Client) NextPaymentsPage(page operations.OperationsPage) (operations.OperationsPage, error) {
	return c.NextOperationsPage(page)
}

// PrevPaymentsPage returns the previous page of payments.
func (c *Client) PrevPaymentsPage(page operations.OperationsPage) (operations.OperationsPage, error) {
	return c.PrevOperationsPage(page)
}

// NextOffersPage returns the next page of offers.
func (c *Client) NextOffersPage(page hProtocol.OffersPage) (offers hProtocol.OffersPage, err error) {
	err = c.sendGetRequest(page.Links.Next.Href, &offers)
	return
}

// PrevOffersPage returns the previous page of offers.
func (c *Client) PrevOffersPage(page hProtocol.OffersPage) (offers hProtocol.OffersPage, err error) {
	err = c.sendGetRequest(page.Links.Prev.Href, &offers)
	return
}

// NextTradesPage returns the next page of trades.
func (c *Client) NextTradesPage(page hProtocol.TradesPage) (trades hProtocol.TradesPage, err error) {
	err = c.sendGetRequest(page.Links.Next.Href, &trades)
	return
}

// PrevTradesPage returns the previous page of trades.
func (c *Client) PrevTradesPage(page hProtocol.TradesPage) (trades hProtocol.TradesPage, err error) {
	err = c.sendGetRequest(page.Links.Prev.Href, &trades)
	return
}

// HomeDomainForAccount returns the home domain for a single account.
func (c *Client) HomeDomainForAccount(aid string) (string, error) {
	if aid == "" {
		return "", errors.New("no account ID provided")
	}

	accountDetail, err := c.AccountDetail(AccountRequest{AccountID: aid})
	if err != nil {
		return "", errors.Wrap(err, "get account detail failed")
	}

	return accountDetail.HomeDomain, nil
}

// NextTradeAggregationsPage returns the next page of trade aggregations from the current
// trade aggregations response.
func (c *Client) NextTradeAggregationsPage(page hProtocol.TradeAggregationsPage) (ta hProtocol.TradeAggregationsPage, err error) {
	err = c.sendGetRequest(page.Links.Next.Href, &ta)
	return
}

// PrevTradeAggregationsPage returns the previous page of trade aggregations from the current
// trade aggregations response.
func (c *Client) PrevTradeAggregationsPage(page hProtocol.TradeAggregationsPage) (ta hProtocol.TradeAggregationsPage, err error) {
	err = c.sendGetRequest(page.Links.Prev.Href, &ta)
	return
}

// ClaimableBalances returns details about available claimable balances,
// possibly filtered to a specific sponsor or other parameters.
func (c *Client) ClaimableBalances(cbr ClaimableBalanceRequest) (cb hProtocol.ClaimableBalances, err error) {
	err = c.sendRequest(cbr, &cb)
	return
}

// ClaimableBalance returns details about a *specific*, unique claimable balance.
func (c *Client) ClaimableBalance(id string) (cb hProtocol.ClaimableBalance, err error) {
	cbr := ClaimableBalanceRequest{ID: id}
	err = c.sendRequest(cbr, &cb)
	return
}

func (c *Client) LiquidityPoolDetail(request LiquidityPoolRequest) (lp hProtocol.LiquidityPool, err error) {
	err = c.sendRequest(request, &lp)
	return
}

func (c *Client) LiquidityPools(request LiquidityPoolsRequest) (lp hProtocol.LiquidityPoolsPage, err error) {
	err = c.sendRequest(request, &lp)
	return
}

func (c *Client) NextLiquidityPoolsPage(page hProtocol.LiquidityPoolsPage) (lp hProtocol.LiquidityPoolsPage, err error) {
	err = c.sendGetRequest(page.Links.Next.Href, &lp)
	return
}

func (c *Client) PrevLiquidityPoolsPage(page hProtocol.LiquidityPoolsPage) (lp hProtocol.LiquidityPoolsPage, err error) {
	err = c.sendGetRequest(page.Links.Prev.Href, &lp)
	return
}

// ensure that the horizon client implements ClientInterface
var _ ClientInterface = &Client{}
