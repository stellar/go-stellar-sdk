# ingest

Ledger data ingestion library. Parse raw ledger data from Captive Core, RPC, or history archives.

## OVERVIEW

Build custom ingestion pipelines. Read transactions, state changes, and ledger metadata. Used by Horizon and custom analytics tools.

## ARCHITECTURE

```
              [ Your Code ]
                    |
               [ Readers ]
              /           \
     [ChangeReader]    [TransactionReader]
           |                    |
    CheckpointChange      LedgerTransaction
       Reader                 Reader
                    |
            [ LedgerBackend ]
                    |
         |---------|---------|
      Captive    Buffered     RPC
       Core      Storage    Backend
```

## KEY TYPES

| Type | Purpose |
|------|---------|
| `LedgerTransactionReader` | Read transactions from a ledger |
| `CheckpointChangeReader` | Read state changes at checkpoints |
| `LedgerChangeReader` | Read changes within a single ledger |
| `Change` | Represents a ledger entry change (created/updated/removed) |
| `LedgerTransaction` | Transaction with result and metadata |

## BACKENDS (`ledgerbackend/`)

| Backend | Use Case |
|---------|----------|
| `CaptiveCoreBackend` | Run embedded stellar-core (most common) |
| `BufferedStorageBackend` | Read from pre-exported data lake |
| `RPCBackend` | Read from Stellar RPC |

## COMMON PATTERNS

```go
// Create Captive Core backend
backend, err := ledgerbackend.NewCaptive(ledgerbackend.CaptiveCoreConfig{
    BinaryPath:         "/usr/bin/stellar-core",
    NetworkPassphrase:  network.TestNetworkPassphrase,
    HistoryArchiveURLs: network.TestNetworkhistoryArchiveURLs,
    Toml:               captiveCoreToml,
})
defer backend.Close()

// Prepare ledger range
err = backend.PrepareRange(ctx, ledgerbackend.BoundedRange(1000, 2000))

// Read transactions
reader, err := ingest.NewLedgerTransactionReader(ctx, backend, passphrase, seq)
for {
    tx, err := reader.Read()
    if err == io.EOF { break }
    // Process tx
}
```

## WHERE TO LOOK

| Task | File |
|------|------|
| Read transactions | `ledger_transaction_reader.go` |
| Read state changes | `checkpoint_change_reader.go`, `ledger_change_reader.go` |
| Configure Captive Core | `ledgerbackend/captive_core_backend.go` |
| TOML config generation | `ledgerbackend/toml.go` |
| Buffered/lake backend | `ledgerbackend/buffered_storage_backend.go` |

## ANTI-PATTERNS

| Don't | Do Instead |
|-------|------------|
| Use `FeeChanges` directly | Use helper methods on `LedgerTransaction` |
| Ignore `PrepareRange` | Always prepare range before `GetLedger` |
| Skip `Close()` on backend | Defer close to avoid resource leaks |

## TESTING

```bash
go test ./ingest              # Unit tests
go test ./ingest/ledgerbackend  # Backend tests (may need stellar-core)
```

**Note:** Some tests require stellar-core binary or external services.

## TUTORIALS

Working examples in `ingest/tutorial/`:
- `expiring-sac-balances/` — Track expiring Soroban balances
