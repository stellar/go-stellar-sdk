/*
Package ingest provides primitives for building custom ingestion engines.

Very often developers need features that are outside of Horizon's scope. While
it provides APIs for building the most common apps, it's not possible to add all
possible features. This is why this package was created.

# Ledger Backend

Ledger backends are sources of information about Stellar network ledgers. This
can be, for example: Captive Stellar-Core instances. Please consult the "ledgerbackend"
package docs for more information about each backend.

Warning: Ledger backends provide low-level xdr.LedgerCloseMeta that should not

	be used directly unless the developer really understands this data
	structure. Read on to understand how to use ledger backend in higher
	level objects.

# Readers

Readers are objects that wrap ledger backend and provide higher level, developer
friendly APIs for reading ledger data.

Currently there are three types of readers:
  - CheckpointChangeReader reads ledger entries from history buckets for a given
    checkpoint ledger. Allow building state (all accounts, trust lines etc.) at
    any checkpoint ledger.
  - LedgerTransactionReader reads transactions for a given ledger sequence.
  - LedgerChangeReader reads all changes to ledger entries created as a result of
    transactions (fees and meta) and protocol upgrades in a given ledger.

Warning: Readers stream BOTH successful and failed transactions; check
transactions status in your application if required.

# Tutorial

Refer to the examples below for simple use cases, or check out the README (and
its corresponding tutorial/ subfolder) in the repository for a Getting Started
guide: https://github.com/stellar/go-stellar-sdk/blob/master/ingest/README.md
*/
package ingest
