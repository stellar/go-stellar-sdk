# XDR views: the two-tier API

**Read a little → views. Consume a lot → Walk.** That single sentence is the
doctrine; everything below is its elaboration.

Both tiers operate zero-copy over untrusted XDR bytes: nothing is validated up
front, every access bounds-checks and validates exactly what it reads, and
malformed input yields a `*ViewError` — never a panic (except through the
`Must*` discipline below, which panics with that same `*ViewError` for
`Try`-recovery).

## Tier 1 — plain thin views (read a little)

`NewXView(b []byte) XView` wraps bytes allocation-free. Views are thin
windows (`{data, exact}`) navigated by:

- **Structs**: one lazy accessor per field (`v.TxProcessing()`). Accessing
  field *k* locates it by sizing the preceding variable-size fields on every
  access — there is no memo and no hidden state. Repeated deep access re-sizes
  the prefix; that is the tier-1 trade, and the cue to move to Walk.
- **Unions**: `V()` (or the typed discriminant accessor) reads the
  discriminant O(1); `ArmX()` validates it and enters the arm.
- **Optionals**: `Unwrap() (Inner, bool, error)`.
- **Arrays**: `Len()` (O(1), validated count), plus three iteration forms:
  - `All() iter.Seq2[Elem, error]` — sizes each element **before** yielding
    and delivers **trimmed** views (`Raw()` on an element is a slice
    operation). On a malformed element it yields `(zero, err)` once and
    stops. NOTE: like any `iter.Seq2`, ranging with a single variable
    silently discards the error — use `MustAll` for that form.
  - `MustAll() iter.Seq[Elem]` — the blessed single-variable form: converts
    the in-band error into a `Must` panic (the `*ViewError` sentinel).
  - `Scan()` — the scanner idiom for hot loops: `sc := arr.Scan(); for
    sc.Next() { use sc.Cur() }; if sc.Err() != nil { ... }`. Per-iterator
    local position, no closures. `sc.Rest()` is the power-tool escape hatch:
    the UNVALIDATED remainder of the array's window from the current
    position, for consumers that advance by externally computed extents
    (e.g. `network.TransactionViewHasher.HashSized`, whose returned consumed
    size equals the envelope's `len(Raw())` — one traversal shared between
    hashing and iteration). Bytes past the scan position are neither sized
    nor validated; bound every read yourself.
- **Everything**: `Raw()` (exact wire bytes; a slice op on trimmed views),
  `MustRaw()`, `Copy()` (detached deep copy), `ValidateFull()`.

Element validation is **eager at iteration boundaries**: an iterator sizes an
element in full before handing it over, so corruption anywhere in an element
fails every path that reaches it.

### Must*/Try discipline

Navigational accessors have `Must*` twins panicking with the `*ViewError`
sentinel (field/arm/discriminant accessors, `Unwrap`, `Value`, `Raw`, `All`
via `MustAll`; NOT `Len` or domain helpers like the LCM header conveniences
and `Successful`, which return errors normally).
`Try0(func())`, `Try[T]`, and `Try2[A, B]` run a function and convert exactly
that sentinel back into an ordinary error; **any other panic value is
re-panicked untouched**. Chained navigation reads naturally:

```go
seq, err := xdr.Try(func() uint32 {
    return v.MustArmV1().MustLedgerHeader().MustHeader().MustLedgerSeq().MustValue()
})
```

**Goroutine caveat**: a `Must*` panic is recoverable only by a `Try*` in the
SAME goroutine. Do not call `Must*` in goroutines spawned inside a `Try`
function — Go panics do not cross goroutine boundaries, and the process will
crash.

## Tier 2 — generated Walk (consume a lot)

When a workload touches most of a buffer (extraction, indexing, analytics),
per-access sizing is the wrong shape: use the generated visitor. Per root
type there is a `WalkX(view, *XWalk) error` driver and a position-keyed
callback struct. `WalkTransactionMeta` is the meta sub-root and shares
`LedgerCloseMetaWalk` (there is no separate TransactionMetaWalk; txIdx is 0
and the TxMeta position does not fire for the root itself). The extraction functions in package `ingest`
(`ExtractLedgerEvents`, `ExtractTxHashes`) are its blessed consumers and its
usage models — each operation has exactly ONE public home in the module.

### Hooks vs callbacks — the distinction that matters

The retired designs let hooks *participate in traversal* (report sizes, steer
the walk); a lying hook could corrupt it. Walk callbacks are **observers
only**: the generated driver owns all sizing and advancement through the thin
engine, and a callback can influence the walk in exactly one way — its return
value (see stopping). Callbacks receive path context (`txIdx`, `opIdx`,
`evIdx`) and exact-extent views whose `Raw()` is a slice operation.

### Order

Positions fire in **wire order**, with dense, ordered indices: nondecreasing
`txIdx`; within a transaction, groups in `opIdx` order and events in `evIdx`
order; every `...Begin(count)` position fires before its elements (counts are
validated, for output presizing). Group construction is
version-discriminating: a V3 meta yields at most one operation-event group,
gated only on SorobanMeta presence; a V4 meta yields one group per operation.

### Stopping and errors

- Return `ErrStopWalk` from any callback: the walk stops cleanly and `WalkX`
  returns nil.
- Return any other error: the walk aborts and returns it verbatim.
- Malformed input: the walk returns the engine's `*ViewError` at the point
  the thin engine could not advance. Truncation stops round **up** to
  element/array advance boundaries, and the walk validates nothing past the
  last field it owes a subscriber (scope-derived: e.g. the LedgerCloseMeta
  walk owes nothing past TxProcessing).
- A **zero subscription returns immediately, validating nothing**.
  Unsubscribed subtrees are pruned via thin size skips — pruning costs
  nothing beyond the sizing the wire format demands.

### Position naming and upgrade policy

Positions are named for schema positions (`ResultPair`, `OpEvent`,
`TxEventsBegin`, `TxMeta`, ...) and enumerated in the generated
`XWalkPositions` manifest. The manifest is a compatibility surface: adding a
position (a protocol upgrade adding schema) is backward compatible — existing
subscriptions simply never fire it; renaming or removing one is breaking.
Two tripwires guard upgrades: generation fails if a walker's interior types
disappear from the schema, and the corpus test asserts every manifest
position is exercised — a new protocol shape must extend the corpus before it
can ship.

## Errors

All failures are `*ViewError` with a kind — short buffer, unknown
discriminant, wrong discriminant, array count exceeds data, opaque exceeds
max, bad bool, non-zero padding, max depth, value overflow — and an offset.
Error text is not a compatibility surface; error-vs-success is.
