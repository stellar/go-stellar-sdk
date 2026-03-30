# XDR Views

## The Problem

Today, reading any field from a `LedgerCloseMeta` requires decoding the entire message into Go structs — every transaction, every operation, every ledger change. A typical pubnet ledger is ~1.2MB of XDR. Decoding it allocates ~7MB across ~91,000 Go objects (the decoded representation is larger than the wire format due to pointers, slice headers, etc.), even if you only need one transaction hash.

## The Idea

XDR's wire format is prefix-deterministic — given the schema, you can compute the byte offset of any field by reading length prefixes and discriminants, without decoding the full message. Views provide this: typed, read-only windows into raw XDR bytes that parse lazily on access.

```go
// Before: decode everything, use one field
var data []byte = getXDRBytes()
var lcm xdr.LedgerCloseMeta
err := lcm.UnmarshalBinary(data)          // ~7MB allocated, ~91K objects
seq := lcm.V1.LedgerHeader.Header.LedgerSeq

// After: navigate directly to the field
view := xdr.LedgerCloseMetaView(data)     // zero cost — just a type cast

v1, err := view.V1()                        // read 4-byte discriminant, return sub-view at V1 arm
hdr, err := v1.LedgerHeader()               // read preceding field sizes to find offset, return sub-view
header, err := hdr.Header()                 // read preceding field sizes to find offset, return sub-view
seqView, err := header.LedgerSeq()          // read preceding field sizes to find offset, return sub-view
seq, err := seqView.Value()                 // read 4 bytes, decode as uint32
```

The view path reads the union discriminant (4 bytes), reads a few length prefixes to skip past preceding struct fields, then reads the 4-byte sequence number. Only a small fraction of the ~1.2MB buffer is touched. Everything else is skipped entirely.

## How It Works

A view is a named `[]byte` type. Creating one is a type cast — no copies, no allocations:

```go
type LedgerCloseMetaView []byte

var data []byte = getXDRBytes()
view := LedgerCloseMetaView(data)
```

Every XDR struct, union, enum, and typedef has a corresponding view type. Each field of a struct becomes a method that returns a sub-view — a `[]byte` slice starting at that field's byte offset. Sub-views are "fat slices": they extend to the end of the parent buffer, not just the field's own extent. This avoids computing the field's size during navigation. The exact bytes for a view can be extracted later with `Raw()`.

```go
v1, err := view.V1()                  // LedgerCloseMetaV1View
if err != nil { return err }
hdr, err := v1.LedgerHeader()         // LedgerHeaderHistoryEntryView
if err != nil { return err }
header, err := hdr.Header()           // LedgerHeaderView
if err != nil { return err }
seqView, err := header.LedgerSeq()    // Uint32View
if err != nil { return err }
```

Each accessor returns `(T, error)`. The error is non-nil if the data is truncated or malformed. Each call computes the byte offset of the requested field and returns a sub-view starting there. No intermediate Go structs are created. No heap allocations occur.

At the leaves of the type hierarchy are primitive views like `Uint32View`, `Int64View`, `BoolView`. These have no sub-fields to navigate into. Instead, they expose `Value()` which decodes the raw bytes into a Go type:

```go
seq, err := seqView.Value()           // uint32
```

## Navigating Structs

Struct fields become typed methods. Each returns a sub-view that you can navigate further or extract a value from. Error checks omitted for brevity in the remaining examples:

```go
// Given a LedgerEntryView:
entryData, err := ledgerEntry.Data()       // LedgerEntryDataView (a union)
account, err := entryData.Account()        // AccountEntryView (a struct)
balance, err := account.Balance()          // Int64View (a leaf)
val, err := balance.Value()                // int64
```

## Navigating Unions

Unions have a discriminant accessor and one method per arm:

```go
// Given a LedgerEntryDataView:
disc, err := entryData.Type()              // LedgerEntryTypeView
discVal, err := disc.Value()               // LedgerEntryType enum value

account, err := entryData.Account()        // works if disc == ACCOUNT
trustline, err := entryData.Trustline()    // works if disc == TRUSTLINE
// calling the wrong arm returns ViewErrWrongDiscriminant
```

## Leaf Types

Leaf views have no sub-fields. Instead of returning sub-views, they expose `Value()` which decodes the raw bytes into a Go type:

| View Type | `Value()` returns |
|-----------|-------------------|
| `Int32View` | `int32` |
| `Uint32View` | `uint32` |
| `Int64View` | `int64` |
| `Uint64View` | `uint64` |
| `BoolView` | `bool` (strict 0 or 1) |
| `Float32View` | `float32` |
| `Float64View` | `float64` |
| Enum views (e.g., `LedgerEntryTypeView`) | The Go enum type (e.g., `LedgerEntryType`) |
| Fixed opaque views (e.g., `HashView`) | `[]byte` (exact size, e.g., 32 bytes) |
| Variable opaque / string views (e.g., `VarOpaqueView`) | `[]byte` (variable length) |
| Bounded opaque / string views (e.g., `String32View`) | `[]byte` (enforces max length) |

For example:

```go
// Given a TransactionResultPairView:
hashView, err := txResultPair.TransactionHash() // HashView (fixed opaque[32])
hashBytes, err := hashView.Value()              // []byte, the raw 32 bytes

// Given an AccountEntryView:
domainView, err := account.HomeDomain()         // String32View (bounded string<32>)
domainBytes, err := domainView.Value()          // []byte, up to 32 bytes
```

## Arrays

Variable-length arrays support count, random access, and iteration:

```go
count, err := arr.Count()            // (int, error) — reads count from wire

elem, err := arr.At(5)               // random access to element 5

for elem, err := range arr.Iter() {  // sequential iteration (requires Go 1.23+)
    // process each element
}
```

Fixed-length arrays (where the count is a schema constant, not in the wire data):

```go
n := arr.Len()                       // int — compile-time constant, never fails

elem, err := arr.At(2)               // random access
for elem, err := range arr.Iter() {  // iteration
    // ...
}
```

Note: fixed arrays use `Len()` (returns `int`) while variable arrays use `Count()` (returns `(int, error)`). The difference is that variable arrays read the count from the wire data (which can fail on truncated input), while fixed arrays know their count from the schema.

Bounded arrays (`T<100>`) enforce their max count in `Count()`, `At()`, and `Iter()`.

Sequential iteration via `Iter()` is O(N). Random access via `At(i)` is O(i) for variable-size elements because preceding elements must be scanned to compute offsets. Prefer `Iter()` for sequential access.

## Optionals

```go
inner, present, err := opt.Unwrap()
if present {
    // use inner (another view)
}
```

## Extracting Raw Bytes

To get the exact XDR wire bytes for any view, use `Raw()`:

```go
raw, err := txResult.Raw()   // the exact bytes, no trailing data
```

This is how you extract a sub-message for storage or forwarding without decoding it. Do not use `[]byte(v)` — as noted above, views are fat slices that include trailing bytes. `Raw()` trims to the exact wire extent.

## Copying

Views alias the original buffer. If you need an independent copy that outlives the original:

```go
copied, err := view.Copy()   // new allocation, safe to use after original is freed
```

## Validation

Views do not validate data on construction — `LedgerCloseMetaView(data)` always succeeds, even on corrupt data. To verify the data is well-formed:

```go
err := view.Valid()                    // default depth limit (200)
err := view.Valid(WithMaxDepth(500))   // custom depth limit
err := view.Valid(NoDepthLimit())      // no depth limit
```

`Valid()` traverses the **entire** structure checking bounds, schema constraints (max lengths, known enum values, bool 0/1), and recursion depth. This means it checks every field, including fields you may never access. After `Valid()` succeeds, all field accessors on that view are guaranteed to succeed.

Without calling `Valid()`, accessors still return errors on malformed data — they never panic. The trade-off is: `Valid()` pays the cost of full traversal once and gives a blanket guarantee, while skipping it means you handle errors at each access point but only pay for the fields you touch.

## Errors

All accessors return `(T, error)`. Errors are `*ViewError`:

```go
type ViewError struct {
    Kind   ViewErrorKind
    Offset uint32
    Detail string
}
```

| Kind | Meaning |
|------|---------|
| `ViewErrShortBuffer` | Data truncated |
| `ViewErrWrongDiscriminant` | Accessed wrong union arm |
| `ViewErrUnknownDiscriminant` | Discriminant not in schema |
| `ViewErrIndexOutOfRange` | Array index out of bounds |
| `ViewErrArrayCountExceedsData` | Array count exceeds remaining data |
| `ViewErrArrayCountExceedsMax` | Array count exceeds schema bound |
| `ViewErrOpaqueExceedsMax` | Opaque/string exceeds schema max length |
| `ViewErrBadBoolValue` | Bool is not 0 or 1 |
| `ViewErrMaxDepth` | Recursion depth exceeded |
| `ViewErrNonZeroPadding` | Padding byte is not zero |

For convenience, `Must()` panics on error:

```go
// Given a LedgerHeaderView:
seq := Must(Must(header.LedgerSeq()).Value())
```

## Performance

Benchmarked across 1,000 randomly sampled pubnet ledgers from ledgers 60,160,002–60,170,001 (December 5–6, 2025). Ledger size distribution: p25=1.3MB, p50=1.5MB, p75=1.8MB, p99=2.3MB. Transaction count: p25=234, p50=296, p75=411, p99=920.

| Operation | Full Decode | View | Speedup |
|-----------|------------|------|---------|
| Find tx by hash (early match) | 5.0ms | 56us | 89x |
| Find tx by hash (mid match) | 5.0ms | 161us | 31x |
| Find tx by hash (late match) | 5.0ms | 368us | 14x |
| Extract events by tx hash | 5.3ms | 394us | 13x |
| Extract all tx hashes | 5.2ms | 395us | 13x |
| Extract all events | 4.9ms | 464us | 11x |
| Extract all transactions | 6.9ms | 814us | 8x |
| Validate | 5.0ms | 538us | 9x |

Full decode allocates ~8.8MB across ~111,000 objects per ledger. Views: 0 heap allocations for navigation. Allocations occur only when calling `Raw()` or `Copy()`.

Full decode time is constant regardless of which fields are accessed — it always decodes everything. View time scales with how much data is touched: finding a transaction by hash near the start of the array (56us) is 7x faster than scanning to the end (368us).

## Security

Views are designed to safely handle untrusted input. Here is what the implementation guarantees, what callers should be aware of, and known failure modes.

### Guaranteed by the implementation

**No panics on malformed input.** Every slice operation is preceded by a bounds check. All accessors return `(T, error)` — they never panic, even on truncated, corrupt, or adversarial data.

**No unbounded memory allocation.** View construction is a zero-cost type cast. Navigation allocates nothing on the heap. `Raw()` and `Copy()` allocate exactly the bytes needed.

**Recursion depth limits.** XDR allows recursive types (e.g., `ClaimPredicate`, `SCVal`). Two independent limits prevent stack overflow:
- All internal traversal (field navigation, `Raw()`, etc.) enforces a hardcoded limit of 10,000 nesting levels. This cannot be disabled.
- `Valid()` defaults to a limit of 200 (matching `go-xdr`'s decode limit), configurable via `WithMaxDepth()`.

**Integer overflow safety.** All offset accumulation uses `int64` arithmetic internally — in struct field traversal, array iteration, size computation, and validation. This prevents overflow on both 32-bit and 64-bit platforms. Wire counts are capped at 2^31.

**No amplification attacks.** Processing time is proportional to the data actually present in the buffer, not to wire-declared counts. A small buffer with a large declared array count is rejected in O(1) for fixed-size elements and in O(data size) for variable-size elements. **Limiting the input payload size is sufficient to bound both CPU and memory usage** — views allocate nothing during navigation, and `Raw()`/`Copy()` allocate at most the payload size.

### Known failure modes

**Deeply nested structures may be rejected.** There are two independent depth limits, and their interaction requires care:

1. **`Valid()` limit (default 200, configurable).** `Valid()` rejects structures deeper than its configured limit. Use `WithMaxDepth(n)` or `NoDepthLimit()` to raise this. This limit exists to match `go-xdr`'s decode depth limit.

2. **Internal limit (default 10,000).** All other operations — field accessors, `Raw()`, array iteration — enforce a limit of `MaxViewDepth()` nesting levels. This exists as a safety net against stack overflow on adversarial data. It can be changed at program init via `SetMaxViewDepth(n)` (must not be called concurrently with view operations).

**These limits are independent.** Calling `Valid(WithMaxDepth(100000))` can succeed (because `Valid()` uses its own configurable limit), but subsequent operations — field accessors, `Raw()`, array iteration — may fail with `ViewErrMaxDepth` because the internal limit defaults to 10,000. This includes field accessors that must compute offsets past deeply nested preceding fields. To raise the internal limit, call `SetMaxViewDepth(n)` at program startup.

This limit may need to be raised in the future if the Stellar XDR schema evolves to include more deeply nested types. Current real-world nesting is under 20 levels.

**Concurrent mutation is unsafe.** Views alias the underlying buffer. If the buffer is modified while a view is being read, the view may return corrupt data or errors. Views are safe for concurrent reads from multiple goroutines, but the underlying buffer must not be written to concurrently.

### Caller responsibilities

**Check errors or call `Valid()`.** Views do not validate on construction. `LedgerCloseMetaView(data)` always succeeds, even on garbage. For untrusted input, either:
- Call `view.Valid()` once upfront for a blanket guarantee that all subsequent accessors succeed, or
- Check the `error` return from every accessor call.

For trusted input (e.g., from captive core or a verified ledger archive), `Valid()` is not necessary — accessors will succeed on well-formed data, and the validation cost can be avoided.

**Use `Raw()`, not `[]byte(v)`.** Views are fat slices that extend beyond the value's wire extent. Converting a view to `[]byte` directly includes trailing bytes from sibling fields. Always use `Raw()` to extract the exact wire bytes.
