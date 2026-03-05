# Changelog
This repository adheres to [Go module Versioning](https://go.dev/doc/modules/version-numbers).

This monorepo contains a number of sdk's:

* `horizonclient` ([changelog](./clients/horizonclient/CHANGELOG.md))
* `txnbuild` ([changelog](./txnbuild/CHANGELOG.md))
* `rpcclient` ([changelog](./clients/rpcclient/CHANGELOG.md))
* `corelient` ([changelog](./clients/stellarcore/CHANGELOG.md))


Official project releases may be found here: https://github.com/stellar/go-stellar-sdk/releases
## Pending

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
