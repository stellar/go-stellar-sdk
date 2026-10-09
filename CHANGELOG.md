# Changelog
This repository adheres to [Go module Versioning](https://go.dev/doc/modules/version-numbers).

This monorepo contains a number of sdk's:

* `horizonclient` ([changelog](./clients/horizonclient/CHANGELOG.md))
* `txnbuild` ([changelog](./txnbuild/CHANGELOG.md))
* `rpcclient` ([changelog](./clients/rpcclient/CHANGELOG.md))
* `corelient` ([changelog](./clients/stellarcore/CHANGELOG.md))


Official project releases may be found here: https://github.com/stellar/go-stellar-sdk/releases
## Pending

### Breaking Changes
* xdr: Protocol 30 support (CAP-0084, CAP-0087, CAP-0088). Regenerated from stellar-xdr@4f524bba, where all three CAPs are ungated, so `XDR_FEATURES` is now empty. Contract spec UDT and event names are widened to `SC_SPEC_TYPE_NAME_LIMIT` (1024 bytes), and `ScSpecEventV0.Name` is now a `string` instead of an `ScSymbol`.
* support/storage, historyarchive: Remove `OnDiskCache`, `MakeOnDiskCache`, `ConnectOptions.Wrap`, `ArchiveBucketCache` and `CacheOptions`. They have had no callers since lighthorizon was removed ([#5334](https://github.com/stellar/go-stellar-sdk/pull/5334)) and Horizon moved to fscache ([#5197](https://github.com/stellar/go-stellar-sdk/pull/5197)). An early exit from a bucket read left a partial file in `OnDiskCache` that later reads served as complete ([#6017](https://github.com/stellar/go-stellar-sdk/pull/6017))

### New Features
* xdr: Add the CAP-0084 `ScAddressTypeScAddressTypeMuxedContract` arm of `ScAddress`. `ScAddress.String()` renders it as a `W...` muxed contract strkey.
* xdr: Add the CAP-0088 millisecond close-time arms of `StellarValue`: `StellarValueTypeStellarValueSignedMs` (`SignedMsValue`) and `StellarValueTypeStellarValueEmptyTxSetMs` (`ProposedMsValue`), and the `TimePointMs` type. Every ledger after the Protocol 30 upgrade ledger uses one of these arms, so earlier SDK versions cannot decode them.
* xdr: Add the CAP-0087 ML-DSA `ContractCostType` values 86–94 (`MlDsa{44,65,87}DecodeVerifyingKey`, `MlDsa{44,65,87}DecodeSignature`, `VerifyMlDsa{44,65,87}Sig`). These are names only; cost params are positional, so they decoded before.
* strkey: Add `VersionByteMuxedContract` (`W...`), `MuxedContract`, `DecodeMuxedContract`, and `IsValidMuxedContractAddress`.
* xdr: Added `LedgerCloseMetaView.LedgerHeader()`, exposing the version-resolving header accessor that already backs `LedgerSequence`, `LedgerCloseTime`, `LedgerHash`, and `PreviousLedgerHash` ([#5982](https://github.com/stellar/go-stellar-sdk/pull/5982))
* xdr: Add `Stream.ResumeFrom`, which positions a new stream at the last record boundary of a failed one and continues its SHA-256 from there, so `ValidateHash` on the new stream covers both ([#6017](https://github.com/stellar/go-stellar-sdk/pull/6017))
* rpcclient: Add `Client.URL()` to expose the configured RPC server URL ([#5885](https://github.com/stellar/go-stellar-sdk/issues/5885))
* protocols/rpc: Add queryEvents wire types and request validation (`QueryEventsRequest`/`QueryEventsResponse`, `QueryEventsFilter`, scan statuses, typed `error.data` payloads), transcribed from the [accepted proposal](https://github.com/orgs/stellar/discussions/1872) ([#5971](https://github.com/stellar/go-stellar-sdk/pull/5971))

### Bug Fixes
* support/contractevents: Read the amount from map event data (`{amount, to_muxed_id}`). A SAC transfer to a muxed destination (CAP-0067 `M...`, CAP-0084 `W...`) previously parsed as a transfer with `Amount` zero and no error. Event data with no amount now returns `ErrNotTransferEvent` instead of a zero-amount event.
* support/datastore: Fix infinite loop in `LedgerFileIter` when the datastore contains directory placeholder objects, and reject a bucket path with empty, `.` or `..` segments at startup ([#6014](https://github.com/stellar/go-stellar-sdk/pull/6014))
* ingest: `CheckpointChangeReader` now resumes a failed bucket download from the last record boundary and continues the first download's SHA-256 from there, so the final hash check covers every record the reader returned. Before, the second download had its own hash and the records already returned from the first were not covered. A closed read channel now always means the producer has finished, so concurrent `Read()` callers cannot see `io.EOF` after a failure. `xdr.Stream.Close` is idempotent; a second close used to panic with the fscache-backed archive. `NewHotArchiveIterator` no longer blocks when the consumer stops early. `Progress()` returns 0 instead of NaN before the first bucket size is known ([#6017](https://github.com/stellar/go-stellar-sdk/pull/6017))
* processors/token_transfer: Accept a `to_muxed_id` bound to `Void` in V4 event data. CAP-0067 specifies that the key is simply absent when there is no muxed destination, and that form already parsed. `Void` is what a contract emits instead if it publishes its event data as a `#[contracttype]` struct with an `Option` field — the natural way to write it before CAP-0086's sparse maps, which omit the key. Such an event previously failed to parse and was dropped from the event stream entirely, silently ([#5983](https://github.com/stellar/go-stellar-sdk/pull/5983))
* processors/token_transfer: Report a `to_muxed_id` of type `ScvBytes` at the length the contract emitted. It was previously copied into a fixed 32-byte buffer, so a shorter value was right-padded with zeroes and a longer one truncated, reporting a muxed id that was never emitted. Only a classic transaction memo maps to a fixed 32 bytes here; a contract may put any byte string in `to_muxed_id` ([#5984](https://github.com/stellar/go-stellar-sdk/issues/5984))

## [0.7.2] - 2026-08-14

### Breaking Changes
* strkey: `Decode` and `DecodeAny` now validate the payload length against the version byte per [SEP-23](https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0023.md). Fixed-length keys (account ID, seed, muxed account, contract, liquidity pool, claimable balance, hashTx, hashX) must decode to their exact canonical size, and signed payloads must carry a declared payload length of 1–64 bytes matched by their zero padding. Inputs with a valid checksum but a wrong-length payload — previously accepted by `Decode`, `DecodeAny`, and every `IsValid*` helper — are now rejected. ([#5977](https://github.com/stellar/go-stellar-sdk/pull/5977))
  * `NewSignedPayload` rejects empty payloads, matching CAP-40 (the protocol fails such signers with `SET_OPTIONS_BAD_SIGNER`/`txMALFORMED`) and keeping `SignedPayload.Encode` output decodable.
  * Length re-checks that `Decode` now subsumes were removed from `xdr.AccountId.SetAddress`, `xdr.MuxedAccount.SetAddress`, `xdr.SignerKey.SetAddress`, `xdr.ClaimableBalanceId.DecodeFromStrkey`, `strkey.DecodeMuxedAccount`, `strkey.MuxedAccount.SetAccountID`, and txnbuild contract-address parsing. For wrong-length inputs, the xdr and txnbuild callers now return strkey's `invalid payload length` error instead of their own; the strkey muxed-account helpers keep their generic `invalid muxed account` / `invalid ed25519 public key` errors; `xdr.MuxedAccount.SetAddress` is unchanged (it rejects on encoded string length before decoding).
  * `keypair.ParseAddress` wraps every decode failure with `ErrInvalidKey`, so `errors.Is(err, ErrInvalidKey)` keeps matching wrong-length keys (and now also matches checksum/encoding failures, which previously returned the bare strkey error).
  * `DecodeSignedPayload` delegates structure validation to `Decode`; structurally invalid inputs now uniformly error with `invalid signed payload` (previously `signed payload too short: ...` or `invalid signed payload padding`).
* xdr: `Asset.LessThan` now orders assets the way the protocol does — by the raw 32-byte issuer key — instead of by base32 strkey text, and `xdr.NewPoolId` requires strictly `a < b`, rejecting reversed and identical pairs ([#5974](https://github.com/stellar/go-stellar-sdk/pull/5974))
* txnbuild: liquidity pool operations reject asset pairs that are not strictly ordered; see the [txnbuild changelog](./txnbuild/CHANGELOG.md) ([#5974](https://github.com/stellar/go-stellar-sdk/pull/5974), [#5978](https://github.com/stellar/go-stellar-sdk/pull/5978))

### Bug Fixes
* processors/token_transfer: trustline revocation now compares liquidity pool assets by value instead of pointer identity, fixing wrong-leg selection when burning pool shares ([#5974](https://github.com/stellar/go-stellar-sdk/pull/5974))
* ingest: `ApplyLedgerMetadata` now closes the datastore and the ledger backend, and returns the `PrepareRange` error instead of discarding it — an early return previously stranded one goroutine per configured worker and a failed prepare went unnoticed ([#5974](https://github.com/stellar/go-stellar-sdk/pull/5974))

### Updates
* go.mod: Bumped github.com/stellar/go-xdr to dc590f1, normalizing the schema bound in `mergeInputLenAndMaxSize` so a variable-length field whose length prefix ends the input keeps both its schema and input-length bounds. Every generated type decodes through this decoder ([#5974](https://github.com/stellar/go-stellar-sdk/pull/5974))

## [0.7.1] - 2026-08-04

### New Features
* ingest: Added `ExtractLedgerTxParts` plus `EventsFromTxParts` and `FeesFromTxParts` (**experimental**), a single-walk zero-copy extraction API that supersedes the earlier extractor bundles; see the [ingest changelog](./ingest/CHANGELOG.md) ([#5966](https://github.com/stellar/go-stellar-sdk/pull/5966))

### Bug Fixes
* clients/stellartoml: `GetStellarToml` validates the domain before requesting it, for parity with its sibling client ([#5970](https://github.com/stellar/go-stellar-sdk/pull/5970))

## [0.7.0]

### New Features
* xdr: Protocol 28 support (CAP-0083, CAP-0085). XDR regenerated from
  [stellar-xdr@9c9c1459](https://github.com/stellar/stellar-xdr/commit/9c9c145953e80990d6ff1ae3a6a973a0ce6d0694),
  the commit stellar-core 28.0.0 pins; both CAPs are ungated upstream so
  `XDR_FEATURES` is now empty.
* protocols/rpc: Add `LatestLedgerCloseTime` and `OldestLedgerCloseTime` to `GetHealthResponse`, exposing the latest and oldest ledgers' close times (unix seconds) on the `getHealth` response ([#5958](https://github.com/stellar/go-stellar-sdk/pull/5958))

## [0.6.0] - 2026-06-09

Adds support for Protocol 27 (CAP-0071).

### New Features
* xdr: Protocol 27 (CAP-0071) XDR ([#5945](https://github.com/stellar/go-stellar-sdk/pull/5945), [#5947](https://github.com/stellar/go-stellar-sdk/pull/5947))
* xdr: Added zero-copy XDR view types and code generator ([#5937](https://github.com/stellar/go-stellar-sdk/pull/5937))
* ingest/ledgerbackend: Integrated XDR views into the buffered storage backend and added `GetLedgerRaw` ([#5941](https://github.com/stellar/go-stellar-sdk/pull/5941))
* ingest/ledgerbackend: Added `LedgerStream` streaming ingestion API ([#5944](https://github.com/stellar/go-stellar-sdk/pull/5944))
* ingest/loadtest: Added stellar-core apply-load tooling ([#5940](https://github.com/stellar/go-stellar-sdk/pull/5940))
* apiclient: Support non-JSON responses via `ResponseType` ([#5939](https://github.com/stellar/go-stellar-sdk/pull/5939))

### Bug Fixes
* services/stellar-archivist: Fixed nil pointer panics in `S3Storage.ListFiles` ([#5934](https://github.com/stellar/go-stellar-sdk/pull/5934))
* strkey: Bounded decode input length to avoid unnecessary allocation ([#5935](https://github.com/stellar/go-stellar-sdk/pull/5935))
* txnbuild: Validate payload length in contract address decoding ([#5943](https://github.com/stellar/go-stellar-sdk/pull/5943))

### Updates
* go.mod: Bumped github.com/stellar/go-xdr to a87d4d0 ([#5938](https://github.com/stellar/go-stellar-sdk/pull/5938))

## [0.5.0] - 2026-04-07

### Bug Fixes
* ingest: Fixed `VerifyEvents` to handle amounts exceeding the int64 range ([#5932](https://github.com/stellar/go-stellar-sdk/pull/5932))

## [0.4.0] - 2026-04-01

Adds support for Protocol 26.

### New Features
* xdr: Protocol 26 support, merged from protocol-next ([#5930](https://github.com/stellar/go-stellar-sdk/pull/5930))

### Bug Fixes
* support/datastore: Fixed `ListFilePath` for a datastore bucket with no prefix ([#5923](https://github.com/stellar/go-stellar-sdk/pull/5923))

## [0.3.0]

### Security Fixes
* historyarchive: Added size bound to `GetPathHAS` to prevent resource exhaustion ([#5918](https://github.com/stellar/go-stellar-sdk/pull/5918))

### New Features
* xdr: Added `SafeUnmarshalBase64WithOptions` and regenerated with output size tracking ([#5916](https://github.com/stellar/go-stellar-sdk/pull/5916))

## [0.2.0]

### Breaking Changes
* Replaced `SetExpectedHash`/`Close` hash validation pattern with explicit `ValidateHash` method; `Close` now only releases resources. Added `SetMaxRecordSize` to configure per-record allocation limit (default 64MB) ([#5900](https://github.com/stellar/go-stellar-sdk/pull/5900))

### Security Fixes
* Fixed `InputLen()` guard bypass in streaming XDR decoders ([#5905](https://github.com/stellar/go-stellar-sdk/pull/5905))
* strkey: Fixed panic on invalid payload length in `DecodeSignedPayload` ([#5909](https://github.com/stellar/go-stellar-sdk/pull/5909))
* keypair: Fixed panic on invalid payload length in `ParseAddress` ([#5908](https://github.com/stellar/go-stellar-sdk/pull/5908))

### New Features
* rpcclient: Added `PollTransaction` with exponential backoff ([#5876](https://github.com/stellar/go-stellar-sdk/pull/5876))
* support/datastore: Added filesystem datastore support ([#5892](https://github.com/stellar/go-stellar-sdk/pull/5892))


## [0.1.0]
- ingest: captive core ledger backend doesn't replay ledger sequence 2 when inclusive of an unbounded prepare range([#5866](https://github.com/stellar/go-stellar-sdk/issues/5866))
