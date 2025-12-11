# Changelog
This repository adheres to [Go module Versioning](https://go.dev/doc/modules/version-numbers).

This monorepo contains a number of sdk's:

* `horizonclient` ([changelog](./clients/horizonclient/CHANGELOG.md))
* `txnbuild` ([changelog](./txnbuild/CHANGELOG.md))
* `rpcclient` ([changelog](./clients/rpcclient/CHANGELOG.md))
* `corelient` ([changelog](./clients/stellarcore/CHANGELOG.md))


Official project releases may be found here: https://github.com/stellar/go-stellar-sdk/releases

## [v0.1.0](https://github.com/stellar/go-stellar-sdk/compare/horizon-v24.0.0...v0.1.0)

**Inaugural release of restructured SDK.** 
This is first release of the newly [restructured GO SDK](https://stellar.org/blog/developers/introducing-the-golang-stellar-sdk). 
Prior releases are retained for historical reference on [stellar/go](https://github.com/stellar/go)

### Fixed
- ingest: captive core ledger backend doesn't replay ledger sequence 2 when inclusive of an unbounded prepare range([#5866](https://github.com/stellar/go-stellar-sdk/issues/5866))
- txnbuild: fix BumpSequence to validate sequence number error text[#5880](https://github.com/stellar/go-stellar-sdk/pull/5880)
- support: prevent overflow when calculating the file/partition boundary[#5871](https://github.com/stellar/go-stellar-sdk/pull/5871)

### Added
- historyarchive: add time->ledger lookup using binary search[#5874](https://github.com/stellar/go-stellar-sdk/pull/5874)
- ingest: Added `futurenet` for preconfigured network types. [#5863](https://github.com/stellar/go-stellar-sdk/pull/5863)
- rpcclient: Expanded GetLatestLedgerResponse struct fields[#5870](https://github.com/stellar/go-stellar-sdk/pull/5870)