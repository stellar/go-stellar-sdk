# PROJECT KNOWLEDGE BASE

**Generated:** 2026-02-12
**Commit:** 661961a5
**Branch:** main

## OVERVIEW

Official Go SDK for Stellar blockchain. Provides transaction building (`txnbuild`), API clients (`horizonclient`, `rpcclient`), ledger ingestion (`ingest`), and XDR type definitions. Previously a monorepo—services moved to separate repos (Oct 2025).

## STRUCTURE

```
go-stellar-sdk/
├── txnbuild/           # Transaction building API (83 Go files) - START HERE for tx construction
├── clients/            # Network clients
│   ├── horizonclient/  # Horizon REST API client (primary)
│   ├── rpcclient/      # Stellar RPC client
│   ├── stellarcore/    # Direct stellar-core client
│   └── stellartoml/    # stellar.toml parser
├── ingest/             # Ledger data ingestion library
│   └── ledgerbackend/  # Captive Core, RPC backend, buffered storage
├── xdr/                # XDR types (MOSTLY GENERATED - see xdr/AGENTS.md)
├── support/            # Internal utilities (db, http, config, errors, logging)
├── historyarchive/     # History archive access
├── protocols/          # Protocol response types (horizon, rpc, stellarcore)
├── processors/         # ETL-style data processors
├── strkey/             # Stellar key encoding (G*, S*, M*, P*, C* addresses)
├── keypair/            # Cryptographic key pair operations
├── tools/              # Standalone CLI utilities (not library code)
└── exp/                # DEPRECATED experimental packages
```

## WHERE TO LOOK

| Task | Location | Notes |
|------|----------|-------|
| Build transactions | `txnbuild/` | Start with `NewTransaction()` |
| Submit to Horizon | `clients/horizonclient/` | Use with txnbuild |
| Submit to RPC | `clients/rpcclient/` | Soroban smart contracts |
| Parse ledger data | `ingest/` | LedgerTransactionReader, CheckpointChangeReader |
| Run Captive Core | `ingest/ledgerbackend/` | CaptiveCoreConfig |
| XDR type helpers | `xdr/` | Hand-written helpers in non-generated files |
| Key encoding | `strkey/` | Decode/Encode/CheckValid |
| Sign transactions | `keypair/` | Full.Sign(), ParseAddress() |
| Network passphrases | `network/` | TestNetworkPassphrase, PublicNetworkPassphrase |
| Database utilities | `support/db/` | Session, batch insert builders |
| HTTP utilities | `support/http/` | Middleware, signed requests |

## CODE MAP

**Primary Public Interfaces:**

| Package | Key Types/Funcs | Role |
|---------|-----------------|------|
| `txnbuild` | `Transaction`, `NewTransaction()`, `Operation` | Construct and sign transactions |
| `horizonclient` | `Client`, `DefaultTestNetClient` | Query Horizon, submit transactions |
| `rpcclient` | `Client` | Interact with Stellar RPC |
| `ingest` | `LedgerTransactionReader`, `CheckpointChangeReader` | Parse ledger data |
| `ledgerbackend` | `CaptiveCoreConfig`, `NewCaptive()` | Run embedded stellar-core |
| `xdr` | All Stellar protocol types | Network data structures |
| `keypair` | `Full`, `FromAddress`, `Parse()`, `Random()` | Key management |
| `strkey` | `Decode()`, `Encode()`, `CheckValid()` | Address encoding |

## CONVENTIONS

**Deviations from standard Go:**
- No top-level `cmd/` or `pkg/`—CLIs under `tools/`, libs at root
- Nested `internal/` dirs (e.g., `strkey/internal/`) instead of single `/internal`
- Many `main.go` files are NOT executables—check `package` declaration
- Line length limit: 140 chars (relaxed)
- Function length: up to 100 lines allowed

**Error handling:**
- Mixed stdlib `fmt.Errorf("%w", err)` and `github.com/pkg/errors.Wrap()`
- Prefer `%w` for new code

**Formatting:**
- Run `goimports -w .` before PR (preferred over plain gofmt)
- CI enforces via `./gofmt.sh`

## ANTI-PATTERNS (THIS PROJECT)

| Don't | Do Instead |
|-------|------------|
| Edit `xdr/xdr_generated.go` | Modify `.x` files, run `make xdr` |
| Edit `*.pb.go` files | Modify `.proto` files, run `make generate-proto` |
| Use `xdr.UnmarshalBinary()` on MarshalBinaryBase64 output | Use matching decode method |
| Wrap `io.EOF` in xdr streaming | Return bare `io.EOF` |
| Use deprecated `PathPayment` | Use `PathPaymentStrictReceive` |
| Use deprecated `AllowTrust` | Use `SetTrustLineFlags` |
| Use `horizon` client | Use `horizonclient` |

**Generated code locations:**
- `xdr/xdr_generated.go` — XDR types (DO NOT EDIT)
- `gxdr/xdr_generated.go` — Alternative XDR (DO NOT EDIT)
- `*.pb.go` files — Protobuf (DO NOT EDIT)

## COMMANDS

```bash
# Format and lint (run before PR)
./gofmt.sh && ./gomod.sh && ./govet.sh && ./staticcheck.sh

# Run tests
go test ./...                    # All tests (some need postgres)
go test ./txnbuild              # Just txnbuild (safe)
go test -race -cover ./...      # CI command

# Regenerate XDR types
make xdr                        # Requires Docker

# Regenerate proto
make generate-proto

# Build all
go build ./...

# Run specific tool
go run ./tools/<tool>
```

## NOTES

**Test dependencies:**
- Some tests require PostgreSQL 12+ (set `PGHOST`, `PGUSER`, etc.)
- `ingest/ledgerbackend` tests may need stellar-core binary
- Fuzz tests under `xdr/fuzz/`

**Module path:** `github.com/stellar/go-stellar-sdk`

**Go version:** 1.24+ (tests against 1.24 and 1.25)

**Subdirectory AGENTS.md files exist for:**
- `txnbuild/AGENTS.md` — Transaction building details
- `xdr/AGENTS.md` — Generated vs manual code guidance
- `ingest/AGENTS.md` — Ingestion library patterns
- `clients/horizonclient/AGENTS.md` — Horizon client usage
