# xdr

Stellar XDR (External Data Representation) types. **MOSTLY GENERATED CODE.**

## OVERVIEW

Contains all Stellar protocol types: transactions, operations, ledger entries, assets, etc. The bulk is generated from `.x` IDL files—do NOT edit generated code.

## GENERATED vs MANUAL

| File | Type | Edit? |
|------|------|-------|
| `xdr_generated.go` | Generated | **NO** — 50k+ lines, regenerate via `make xdr` |
| `Stellar-*.x` | IDL source | Yes — modify these, then regenerate |
| All other `*.go` | Hand-written helpers | Yes |

**Hand-written helpers** (safe to edit):
- `asset.go`, `account_id.go`, `muxed_account.go` — type helpers
- `json.go` — JSON marshaling customization
- `scval.go` — Soroban value helpers
- `ledger_key.go`, `ledger_entry.go` — ledger data helpers
- `transaction_envelope.go` — envelope manipulation
- `xdrstream.go` — streaming XDR reader

## REGENERATION

```bash
# Requires Docker
make xdr

# Or step by step:
# 1. Edit xdr/Stellar-*.x files
# 2. Run make xdr (uses xdrgen via Docker)
# 3. Commit xdr_generated.go
```

## COMMON PATTERNS

```go
// Decode base64 XDR
var envelope xdr.TransactionEnvelope
err := xdr.SafeUnmarshalBase64(base64Str, &envelope)

// Encode to base64
base64Str, err := xdr.MarshalBase64(envelope)

// Stream reading (for large data)
reader := xdr.NewXdrStream(r)
for {
    var entry xdr.LedgerEntry
    err := reader.ReadOne(&entry)
    if err == io.EOF { break }
}
```

## ANTI-PATTERNS

| Don't | Do Instead |
|-------|------------|
| Edit `xdr_generated.go` | Edit `.x` files, run `make xdr` |
| `xdr.UnmarshalBinary()` on base64 data | Use `xdr.SafeUnmarshalBase64()` |
| Wrap `io.EOF` in stream reading | Return bare `io.EOF` for proper termination |
| Use deprecated `LedgerKey.LedgerKey()` | Use `LedgerEntryData.LedgerKey()` |

## FUZZ TESTING

Fuzz harnesses under `xdr/fuzz/`:
- `jsonclaimpredicate/` — JSON claim predicate fuzzing
- Corpus files for reproducible fuzzing

## NOTES

- XDR commit tracked in `xdr_commit_generated.txt`
- gxdr package (`../gxdr/`) is alternative XDR impl—also generated
- Many types have `MarshalBinary`/`UnmarshalBinary` for wire format
- JSON marshaling customized for Horizon API compatibility
