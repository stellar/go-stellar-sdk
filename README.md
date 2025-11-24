<div align="center">
<a href="https://stellar.org"><img alt="Stellar" src="https://github.com/stellar/.github/raw/master/stellar-logo.png" width="558" /></a>
<br/>
</div>
<p align="center">
 
<a href="https://github.com/stellar/go-stellar-sdk/actions/workflows/go.yml?query=branch%3Amaster+event%3Apush">![master GitHub workflow](https://github.com/stellar/go-stellar-sdk/actions/workflows/go.yml/badge.svg)</a>
<a href="https://godoc.org/github.com/stellar/go-stellar-sdk"><img alt="GoDoc" src="https://godoc.org/github.com/stellar/go-stellar-sdk?status.svg" /></a>
<a href="https://goreportcard.com/report/github.com/stellar/go-stellar-sdk"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/stellar/go-stellar-sdk" /></a>
</p>

This repository contains the *official Go SDK for Stellar*, maintained by the [Stellar Development Foundation].

It provides all the tools developers need to build applications that integrate with the Stellar network, including APIs for Horizon and Stellar RPC, as well as foundational utilities and ingestion libraries for working with raw ledger data.

This repo previously served as the [Go Monorepo](https://github.com/stellar/go) for all SDF Go projects. As of October 2025, it has been refactored to focus exclusively on developer-facing SDK packages. Services that are still maintained have been moved to their own dedicated repositories.

This repo follows [Go Module Versioning](https://go.dev/doc/modules/version-numbers).

## Usage

Run the following command inside your Go project.

```
go get github.com/stellar/go-stellar-sdk@latest
```

## Package Index

| Package | Description |
|-----------|-----------|
| [Transaction Builder](txnbuild) | Builder for creating Stellar transactions and operations |
| [Horizon API Client](clients/horizonclient) | Client for querying and submitting transactions via a Horizon instance |
| [RPC API Client](clients/rpcclient) | Client for interacting with a Stellar RPC instance |
| [Ingest SDK](ingest) | Library for parsing raw ledger data from Captive Core, a Galexie Data Lake, or RPC |
| [xdr](xdr) / [strkey](strkey) | Core network primitives and encoding helpers |
| [Processors Library](processors) | Reusable data abstractions and ETL-style processors |

## Relocated

The following services have been moved to their own repositories and are no longer built from this SDK:

| Service | New Repository | Description |
|-----------|-----------|-----------|
| Horizon | [stellar-horizon](https://github.com/stellar/stellar-horizon) | Full-featured API server for querying Stellar network data |
| Galexie | [stellar-galexie](https://github.com/stellar/stellar-galexie) | Ledger exporter that writes network data to external data stores |
| Friendbot | [friendbot](https://github.com/stellar/friendbot) | Stellar's test network native asset faucet |

If you build or deploy these services from source, please use their new repositories. Pre-built Debian packages and Docker images continue to be distributed through the same channels.

## Deprecated Services

As of tag [**stellar-go-2025-10-29_10-56-50**](https://github.com/stellar/go-stellar-sdk/releases/tag/stellar-go-2025-10-29_10-56-50), several legacy services have been deprecated and removed. They remain available in Git history for archival or fork purposes.

| Service | Description |
|-----------|-----------|
| [Ticker](https://github.com/stellar/go-stellar-sdk/tree/stellar-go-2025-10-29_10-56-50/services/ticker) | API server providing asset and market statistics |
| [Keystore](https://github.com/stellar/go-stellar-sdk/tree/stellar-go-2025-10-29_10-56-50/services/keystore) | Encrypted key-management API server |
| [Federation Server](https://github.com/stellar/go-stellar-sdk/tree/stellar-go-2025-10-29_10-56-50/services/federation) | Address-lookup service for anchors and financial institutions | 

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for more details.

## Go Versions Supported

The packages in this repository are tested against the two most recent major versions of Go, because only [the two most recent major versions of Go receive security updates](https://go.dev/doc/security/policy).

[Stellar Development Foundation]: https://stellar.org
