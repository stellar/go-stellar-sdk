# Changelog
This repository adheres to [Go module Versioning](https://go.dev/doc/modules/version-numbers).

This monorepo contains a number of sdk's:

* `horizonclient` ([changelog](./clients/horizonclient/CHANGELOG.md))
* `txnbuild` ([changelog](./txnbuild/CHANGELOG.md))
* `rpcclient` ([changelog](./clients/rpcclient/CHANGELOG.md))
* `corelient` ([changelog](./clients/stellarcore/CHANGELOG.md))


Official project releases may be found here: https://github.com/stellar/go-stellar-sdk/releases
## Pending

## [0.6.1] - 2026-08-10

Backport release for Horizon 27.0.1, cut from `v0.6.0`.

### Bug Fixes
* xdr: `Asset.LessThan` now orders assets the way the protocol does — by the raw 32-byte issuer key — instead of by base32 strkey text, and `xdr.NewPoolId` requires strictly `a < b`, rejecting reversed and identical pairs ([#5974](https://github.com/stellar/go-stellar-sdk/pull/5974))
* txnbuild: liquidity pool operations reject asset pairs that are not strictly ordered — previously these built transactions that stellar-core rejects; see the [txnbuild changelog](./txnbuild/CHANGELOG.md) ([#5974](https://github.com/stellar/go-stellar-sdk/pull/5974))
* processors/token_transfer: trustline revocation now compares liquidity pool assets by value instead of pointer identity, fixing wrong-leg selection when burning pool shares ([#5974](https://github.com/stellar/go-stellar-sdk/pull/5974))
* clients/stellartoml: `GetStellarToml` now validates the domain before issuing the request, matching `GetStellarTomlByAddress` ([#5970](https://github.com/stellar/go-stellar-sdk/pull/5970))

### Updates
* ingest/ledgerbackend: Updated the embedded `captive-core-pubnet.cfg`, replacing SatoshiPay's validators with Obsrvr's ([#5963](https://github.com/stellar/go-stellar-sdk/pull/5963))
* go.mod: Bumped github.com/stellar/go-xdr to dc590f1 ([#5974](https://github.com/stellar/go-stellar-sdk/pull/5974))

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
