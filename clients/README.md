# Clients package

Packages here provide client libraries for accessing the ecosystem of Stellar services.

* `horizonclient` - programmatic client access to Horizon (use in conjunction with [txnbuild](../txnbuild))
* `stellartoml` - parse Stellar.toml files from the internet
* `federation` - resolve federation addresses into stellar account IDs, suitable for use within a transaction
* `horizon` (DEPRECATED) - the original Horizon client, now superceded by `horizonclient`

See [GoDoc](https://godoc.org/github.com/stellar/go-stellar-sdk/clients) for more details.

## For developers: Adding new client packages

Ideally, each one of our client packages will have commonalities in their API to ease the cost of learning each.  It's recommended that we follow a pattern similar to the `net/http` package's client shape:

A type, `Client`, is the central type of any client package, and its methods should provide the bulk of the functionality for the package.  A `DefaultClient` var is provided for consumers that don't need client-level customization of behavior.  Each method on the `Client` type should have a corresponding func at the package level that proxies a call through to the default client.  For example, `http.Get()` is the equivalent of `http.DefaultClient.Get()`.
