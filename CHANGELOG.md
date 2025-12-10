# Changelog
This repository adheres to [Go module Versioning](https://go.dev/doc/modules/version-numbers).

This monorepo contains a number of sdk's:

* `horizonclient` ([changelog](./clients/horizonclient/CHANGELOG.md))
* `txnbuild` ([changelog](./txnbuild/CHANGELOG.md))
* `rpcclient` ([changelog](./clients/rpcclient/CHANGELOG.md))
* `corelient` ([changelog](./clients/stellarcore/CHANGELOG.md))


Official project releases may be found here: https://github.com/stellar/go-stellar-sdk/releases

### Pending

- ingest: captive core ledger backend doesn't replay ledger sequence 2 when inclusive of an unbounded prepare range([#5866](https://github.com/stellar/go-stellar-sdk/issues/5866))
