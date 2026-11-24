# horizonclient

HTTP client for Stellar Horizon API. Query network state, submit transactions.

## OVERVIEW

Primary client for interacting with Horizon servers. Use with `txnbuild` for complete transaction workflow.

## KEY TYPES

| Type | Purpose |
|------|---------|
| `Client` | Main client; set `HorizonURL` |
| `DefaultTestNetClient` | Pre-configured testnet client |
| `DefaultPublicNetClient` | Pre-configured pubnet client |
| `AccountRequest` | Query account details |
| `TransactionRequest` | Query transactions |
| `Error` | Horizon error with problem details |

## COMMON PATTERNS

```go
// Use default testnet client
client := horizonclient.DefaultTestNetClient

// Or configure custom
client := &horizonclient.Client{HorizonURL: "https://horizon.stellar.org"}

// Get account
account, err := client.AccountDetail(horizonclient.AccountRequest{
    AccountID: "GABC...",
})

// Submit transaction (built with txnbuild)
resp, err := client.SubmitTransaction(tx)
// or from XDR
resp, err := client.SubmitTransactionXDR(base64XDR)

// Stream ledgers
client.StreamLedgers(ctx, horizonclient.LedgerRequest{}, func(ledger horizon.Ledger) {
    // handle ledger
})
```

## REQUEST TYPES

| Request | Endpoint |
|---------|----------|
| `AccountRequest` | `/accounts/{id}` |
| `AccountsRequest` | `/accounts` (list) |
| `TransactionRequest` | `/transactions` |
| `OperationRequest` | `/operations` |
| `EffectRequest` | `/effects` |
| `LedgerRequest` | `/ledgers` |
| `OfferRequest` | `/offers` |
| `TradeRequest` | `/trades` |
| `PathsRequest` | `/paths` |
| `OrderBookRequest` | `/order_book` |
| `AssetRequest` | `/assets` |
| `ClaimableBalanceRequest` | `/claimable_balances` |

## ERROR HANDLING

```go
resp, err := client.SubmitTransaction(tx)
if err != nil {
    if hzErr, ok := err.(*horizonclient.Error); ok {
        // Access Horizon problem details
        fmt.Println(hzErr.Problem.Title)
        fmt.Println(hzErr.Problem.Extras["result_codes"])
    }
}
```

## ANTI-PATTERNS

| Don't | Do Instead |
|-------|------------|
| Use deprecated `horizon` package | Use `horizonclient` |
| Ignore `*horizonclient.Error` type | Type assert for detailed error info |
| Hardcode Horizon URLs | Use `DefaultTestNetClient` / `DefaultPublicNetClient` |

## TESTING

```bash
go test ./clients/horizonclient   # Unit tests (mocked)
```

Mock available: `horizonclient.MockClient` for testing without network.
