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
This is first release of the newly [restructured GO SDK](https://github.com/stellar/go-stellar-sdk/issues/5666). 
Prior releases are retained for historical reference on [stellar/go](https://github.com/stellar/go)

### Fixed
- ingest: captive core ledger backend doesn't replay ledger sequence 2 when inclusive of an unbounded prepare range([#5866](https://github.com/stellar/go-stellar-sdk/issues/5866))

### Added
- historyarchive: add time->ledger lookup using binary search[#5874](https://github.com/stellar/go-stellar-sdk/pull/5874)
