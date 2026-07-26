# XDR Views

## The Problem

Today, reading any field from a `LedgerCloseMeta` requires decoding the entire message into Go structs — every transaction, every operation, every ledger change. A typical pubnet ledger is ~1.5MB of XDR (median). Decoding it allocates ~8.5MB across ~107,000 Go objects (the decoded representation is larger than the wire format due to pointers, slice headers, etc.), even if you only need one transaction hash.

## The Idea

XDR's wire format is prefix-deterministic — given the schema, you can compute the byte offset of any field by reading length prefixes and discriminants, without decoding the full message. Views provide this: typed, read-only windows into raw XDR bytes that parse lazily on access.

```go
// Before: decode everything, use one field
var data []byte = getXDRBytes()
var lcm xdr.LedgerCloseMeta
err := lcm.UnmarshalBinary(data)          // ~8.5MB allocated, ~107K objects
seq := lcm.MustV1().LedgerHeader.Header.LedgerSeq

// After: navigate directly to the field
view := xdr.LedgerCloseMetaView(data)     // zero cost — just a type cast
seq := view.MustV1().MustLedgerHeader().MustHeader().MustLedgerSeq().MustValue()

// Or with error handling:
v1, err := view.V1()                        // read 4-byte discriminant, return sub-view at V1 arm
hdr, err := v1.LedgerHeader()               // read preceding field sizes to find offset, return sub-view
header, err := hdr.Header()                 // read preceding field sizes to find offset, return sub-view
seqView, err := header.LedgerSeq()          // read preceding field sizes to find offset, return sub-view
seq, err := seqView.Value()                 // read 4 bytes, decode as uint32
```

The view path reads the union discriminant (4 bytes), reads a few length prefixes to skip past preceding struct fields, then reads the 4-byte sequence number. Only a small fraction of the buffer is touched. Everything else is skipped entirely.

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

### Fields(): every field in one walk

Each field accessor locates its field by walking the bytes of the fields before it, so reading several fields of the same struct re-walks the shared prefix each time. `Fields()` locates **every** field of a struct in a single pass and returns a bundle of sub-views, each **trimmed to its exact wire extent**:

```go
f, err := txResultMeta.Fields()            // one walk
result := f.Result                         // TransactionResultPairView (trimmed)
meta := f.TxApplyProcessing                // TransactionMetaView (trimmed)
```

Because each bundle field is trimmed, `[]byte(f.TxApplyProcessing)` is that field's exact wire bytes with no further sizing walk — unlike a plain field accessor (whose result is a fat slice; see Extracting Raw Bytes). The bundle also carries the whole node as `View` (trimmed for free, since the walk already computed the node's total extent), so `len(f.View)` is the node's wire size. That makes `Fields()` the building block for fused iteration over an array of structs: walk element *i* with `Fields()`, then advance by `len(f.View)` to reach element *i+1* — a single walk per element that both locates its fields and yields its size for the next step.

## Navigating Unions

Unions have a discriminant accessor and one method per arm. The discriminant accessor returns the **decoded** discriminant directly — the Go enum for enum discriminants, or `int32`/`bool` for non-enum ones — not a leaf view:

```go
// Given a LedgerEntryDataView:
disc, err := entryData.Type()              // LedgerEntryType (decoded enum value)

account, err := entryData.Account()        // works if disc == ACCOUNT
trustline, err := entryData.TrustLine()    // works if disc == TRUSTLINE
// calling the wrong arm returns ViewErrWrongDiscriminant
```

Enum discriminants are validated against the known case set (an unknown value returns `ViewErrUnknownDiscriminant`). `int`/`bool` discriminants are decoded without case validation, so a `default` arm stays reachable for forward-compatible values.

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
| Fixed opaque views (e.g., `HashView`) | the typed fixed array **by value** (e.g., `Hash` / `[N]byte`) |
| Variable opaque / string views (e.g., `VarOpaqueView`) | `[]byte` (variable length) |
| Bounded opaque / string views (e.g., `String32View`) | `[]byte` (enforces max length) |

For example:

```go
// Given a TransactionResultPairView:
hashView, err := txResultPair.TransactionHash() // HashView (fixed opaque[32])
hash, err := hashView.Value()                   // xdr.Hash, a [32]byte by value (a copy)

// Given an AccountEntryView:
domainView, err := account.HomeDomain()         // String32View (bounded string<32>)
domainBytes, err := domainView.Value()          // []byte, up to 32 bytes
```

A fixed-opaque `Value()` returns a copy of the bytes as a typed array. For the zero-copy bytes that alias the source buffer, use `Raw()` instead.

## Arrays

Variable-length arrays support count, random access, iteration, and materialization:

```go
count, err := arr.Count()            // (int, error) — reads + validates count against the buffer

elem, err := arr.At(5)               // random access to element 5

for elem, err := range arr.Iter() {  // lazy sequential iteration
    // process each element
}

elems, err := arr.All()              // eager: materialize all elements

raws, err := arr.AllRaw()            // eager: every element's exact wire bytes
```

Fixed-length arrays (where the count is a schema constant, not in the wire data):

```go
n := arr.Len()                       // int — compile-time constant, never fails

elem, err := arr.At(2)               // random access
for elem, err := range arr.Iter() {  // iteration
    // ...
}
elems, err := arr.All()              // materialize all elements
raws, err := arr.AllRaw()            // every element's exact wire bytes
```

Note: fixed arrays use `Len()` (returns `int`) while variable arrays use `Count()` (returns `(int, error)`). The difference is that variable arrays read the count from the wire data (which can fail on truncated input), while fixed arrays know their count from the schema.

Bounded arrays (`T<100>`) enforce their max count in `Count()`, `At()`, `Iter()`, `All()`, and `AllRaw()`.

`Count()`, `Iter()`, `All()`, and `AllRaw()` validate the wire element count against the remaining buffer up front, using a per-element minimum wire size — a tiny buffer declaring a huge count is rejected before any allocation (see Security). The validation lives at these allocation/iteration entry points; the internal size/validate walks rely on their own per-element bounds checks instead.

Sequential iteration via `Iter()` is O(N). Random access via `At(i)` is O(i) for variable-size elements because preceding elements must be scanned to compute offsets. Prefer `Iter()` for sequential access.

### Iter vs All

Both traverse all elements, but they differ in semantics:

- `Iter()` yields fat slices (like other view accessors) — views extending to the end of the array. This is optimized for lazy navigation with early break: if you find what you need and break out of the loop, you skip size computation for remaining elements. Use `Raw()` on the yielded element to get its exact wire bytes.

- `All()` returns a materialized slice where each element is trimmed to its exact wire extent — `[]byte(elems[i])` returns the element's exact bytes. Useful when you need to index into all elements or hold references to them. Same total work as `Iter()` over all elements (one `size()` call each).

- `AllRaw()` returns each element's exact wire bytes as a `[][]byte`, presized from the validated count — an empty array yields an empty, non-nil slice. It does the same single `size()` walk as `All()` (fixed-size elements stride-slice with no `size()` calls) but skips the view wrappers, and it replaces the `Iter()`+`Raw()` pattern, which sizes every element twice (once to advance, once to trim). All slices alias the source buffer (zero-copy).

Use `Iter()` for search-and-break patterns. Use `All()` when you need random access to all elements or want to hold onto element byte extents. Use `AllRaw()` when you drain an array purely for its elements' raw bytes.

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

This is how you extract a sub-message for storage or forwarding without decoding it. Do not use `[]byte(v)` on a view returned by a field/arm/`At()`/`Iter()` accessor — those are fat slices that include trailing bytes; `Raw()` trims to the exact wire extent. The exceptions are views that are *already* trimmed — the elements of `All()` and the fields of a `Fields()` bundle — for which `[]byte(v)` is exactly the wire bytes (no walk).

## Copying

Views alias the original buffer. If you need an independent copy that outlives the original:

```go
copied, err := view.Copy()   // new allocation, safe to use after original is freed
```

## Validation

Views validate incrementally during navigation — every field accessor checks bounds before reading and returns `(T, error)`. There is no way to get a value from a view without error checking. This means navigating a view on well-formed data always succeeds, and navigating on malformed data returns errors at the point of access.

For an upfront guarantee, `ValidateFull()` traverses the **entire** structure checking bounds, schema constraints (max lengths, known enum values, bool 0/1, zero padding bytes), and nesting depth:

```go
err := view.ValidateFull()
```

After `ValidateFull()` succeeds, all field accessors on that view are guaranteed to succeed, provided the underlying buffer is not modified.

The name `ValidateFull` communicates that the normal navigation path already does validation — `ValidateFull` just does it exhaustively and upfront rather than incrementally. This follows the same pattern as [Cap'n Proto](https://capnproto.org/encoding.html#security-considerations), which validates lazily on each pointer traversal rather than upfront.

For trusted input (e.g., from captive core or a verified ledger archive), `ValidateFull()` is not necessary — the per-access validation is sufficient, and the full traversal cost (~730µs per 1.5 MB ledger) can be avoided.

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
| `ViewErrMaxDepth` | Nesting depth exceeded internal limit |
| `ViewErrNonZeroPadding` | Padding byte is not zero |

### Must methods

Every accessor has a `Must` variant that panics on error instead of returning it:

```go
// Error-checked:
seqView, err := header.LedgerSeq()
seq, err := seqView.Value()

// Must (panics on error):
seq := header.MustLedgerSeq().MustValue()
```

Must methods are safe after `ValidateFull()` succeeds, or on trusted input. They also work inside `Try` blocks (see below).

Arrays have `MustCount()`, `MustAt(i)`, `MustIter()`, `MustAll()`, and `MustAllRaw()`:

```go
for elem := range arr.MustIter() {   // iter.Seq[T] — yields values, panics on error
    // process elem
}
```

### Try / TryVoid

`Try` and `TryVoid` recover panics from Must methods and return them as errors. This enables clean navigation without per-field error checks:

```go
result, err := xdr.Try(func() uint32 {
    view := xdr.LedgerCloseMetaView(data)
    return view.MustV1().MustLedgerHeader().MustHeader().MustLedgerSeq().MustValue()
})

err := xdr.TryVoid(func() {
    for tx := range view.MustV1().MustTxProcessing().MustIter() {
        hash := tx.MustTransactionHash().MustValue()
        // ...
    }
})
```

Only `*ViewError` panics are caught — other panics propagate normally. Must methods must be called in the same goroutine as Try.

## Performance

Benchmarked across 1,000 randomly sampled pubnet ledgers (avg ~1.5 MB per ledger) on Apple M1 Max, Go 1.25.

| Operation | Full Decode | View | Speedup |
|-----------|------------|------|---------|
| Find tx by hash (early match) | 6.5ms | 48µs | **137x** |
| Find tx by hash (mid match) | 7.2ms | 125µs | **58x** |
| Find tx by hash (late match) | 8.5ms | 699µs | **12x** |
| Extract events by tx hash | 8.4ms | 723µs | **12x** |
| Extract all tx hashes | 8.0ms | 542µs | **15x** |
| Extract all events | 6.8ms | 388µs | **18x** |
| Extract all transactions | 10.9ms | 978µs | **11x** |
| ValidateFull | — | 729µs | — |

Full decode allocates ~8.5MB across ~107,000 objects per ledger. Views: 0 heap allocations for navigation. Allocations occur only when calling `Copy()`. `Raw()` returns a subslice of the original buffer (zero allocation).

Full decode time is constant regardless of which fields are accessed — it always decodes everything. View time scales with how much data is touched: finding a transaction by hash near the start of the array (48µs) is ~15x faster than scanning to the end (699µs).

## Security

Views are designed to safely handle untrusted input. Here is what the implementation guarantees, what callers should be aware of, and known failure modes.

### Guaranteed by the implementation

**No panics on malformed input (error-returning API).** Every slice operation is preceded by a bounds check. All error-returning accessors (`Field()`, `Value()`, `At()`, `Iter()`, etc.) never panic, even on truncated, corrupt, or adversarial data. Must methods (`MustField()`, `MustValue()`, etc.) panic on error by design — use them inside `Try` blocks or after `ValidateFull()` succeeds.

**No unbounded memory allocation.** View construction is a zero-cost type cast. Navigation allocates nothing on the heap. `Raw()` returns a subslice of the original buffer (zero allocation). `Copy()` allocates exactly the bytes needed.

**Nesting depth limit.** XDR allows recursive types (e.g., `ClaimPredicate`, `SCVal`). All view operations — field navigation, `Raw()`, `ValidateFull()`, array iteration — enforce a fixed recursion depth limit of 1,500, matching stellar-core's `xdr::marshaling_stack_limit`. Real-world Stellar XDR nests under 20 levels; 1,500 provides ample headroom for future schema evolution.

**Padding byte validation.** Both `ValidateFull()` and `Value()` reject non-zero XDR padding bytes with `ViewErrNonZeroPadding`, matching the behavior of the `go-xdr` decoder used by `SafeUnmarshal`.

**Integer overflow safety.** All offset accumulation uses `int64` arithmetic internally — in struct field traversal, array iteration, size computation, and validation. This prevents overflow on both 32-bit and 64-bit platforms. Wire-level element counts are validated as signed 32-bit integers (max 2,147,483,647).

**No amplification attacks.** Processing time is proportional to the data actually present in the buffer, not to wire-declared counts. A small buffer with a large declared array count is rejected in O(1) for fixed-size elements and in O(data size) for variable-size elements. **Limiting the input payload size is sufficient to bound both CPU and memory usage** — views allocate nothing during navigation, and `Copy()` allocates at most the payload size.

### Known failure modes

**Extremely deep nesting is rejected.** The recursion depth limit is 1,500, matching stellar-core's `xdr::marshaling_stack_limit`. XDR data nested deeper than this returns `ViewErrMaxDepth`. This limit is fixed and not configurable. Current real-world Stellar XDR nests under 20 levels.

**All mutation of the underlying buffer is unsafe.** Views alias the underlying buffer and assume the bytes are immutable for the view's lifetime. Any modification to the buffer (serial or concurrent) may cause views to return corrupt data or errors. Views are safe for concurrent reads from multiple goroutines.

### Caller responsibilities

**Check errors, use Try, or call `ValidateFull()`.** Views validate incrementally — every accessor returns `(T, error)` and checks bounds before reading. Three styles:
1. Check each error individually.
2. Use Must methods inside `Try`/`TryVoid` for clean chaining.
3. Call `ValidateFull()` once upfront, then use Must methods freely.

For trusted input (e.g., from captive core or a verified ledger archive), `ValidateFull()` is not necessary — the per-access validation is sufficient, and the full traversal cost can be avoided.

**Use `Raw()`, not `[]byte(v)`.** Views are fat slices that extend beyond the value's wire extent. Converting a view to `[]byte` directly includes trailing bytes from sibling fields. Always use `Raw()` to extract the exact wire bytes.

