# txnbuild

Transaction building API for Stellar network. Construct, sign, and serialize transactions.

## OVERVIEW

Primary SDK entry point for building transactions. Use `NewTransaction()` with operations, sign with keypairs, submit via `horizonclient`.

## KEY TYPES

| Type | Purpose |
|------|---------|
| `Transaction` | Immutable signed transaction; call `Sign()` to add signatures |
| `TransactionParams` | Input to `NewTransaction()`: source, ops, fee, preconditions |
| `FeeBumpTransaction` | Wrapper to increase fee on existing transaction |
| `Operation` | Interface implemented by all 20+ operation types |
| `Account` | Interface for source account (sequence number management) |
| `Preconditions` | TimeBounds, LedgerBounds, MinSequence, etc. |

## COMMON PATTERNS

```go
// Build transaction
tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
    SourceAccount:        &account,
    IncrementSequenceNum: true,
    Operations:           []txnbuild.Operation{&op},
    BaseFee:              txnbuild.MinBaseFee,
    Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
})

// Sign (returns NEW Transaction - immutable)
tx, err = tx.Sign(network.TestNetworkPassphrase, keypair)

// Serialize for submission
xdrBase64, err := tx.Base64()
```

## WHERE TO LOOK

| Task | File |
|------|------|
| Create transaction | `transaction.go` — `NewTransaction()` |
| Payment operation | `payment.go` |
| Path payments | `path_payment_strict_send.go`, `path_payment.go` |
| Account creation | `create_account.go` |
| Trust lines | `change_trust.go`, `set_trust_line_flags.go` |
| Offers/DEX | `manage_offer.go`, `manage_buy_offer.go` |
| Soroban invoke | `invoke_host_function.go` |
| Fee bumps | `transaction.go` — `NewFeeBumpTransaction()` |

## ANTI-PATTERNS

| Don't | Do Instead |
|-------|------------|
| Use `PathPayment` | Use `PathPaymentStrictReceive` (renamed) |
| Use `AllowTrust` | Use `SetTrustLineFlags` |
| Mutate Transaction after Sign | Transaction is immutable; Sign returns new instance |
| Forget `IncrementSequenceNum` | Set `true` unless managing sequence manually |

## TESTING

```bash
go test ./txnbuild           # Unit tests (no external deps)
go test ./txnbuild -v        # Verbose
```

Demo available: `go run ./txnbuild/cmd/demo`
