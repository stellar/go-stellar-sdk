# horizonclient


`horizonclient` is a [Stellar Go SDK](https://developers.stellar.org/api/) package that provides client access to a horizon server. It supports all endpoints exposed by the [horizon API](https://developers.stellar.org/api/introduction/).

This project is maintained by the Stellar Development Foundation.

## Getting Started
This library is aimed at developers building Go applications that interact with the [Stellar network](https://www.stellar.org/). It allows users to query the network and submit transactions to the network. The recommended transaction builder for Go programmers is [txnbuild](https://github.com/stellar/go-stellar-sdk/tree/master/txnbuild). Together, these two libraries provide a complete Stellar SDK.

* The [horizonclient API reference](https://godoc.org/github.com/stellar/go-stellar-sdk/clients/horizonclient).
* The [txnbuild API reference](https://godoc.org/github.com/stellar/go-stellar-sdk/txnbuild).

### Prerequisites
* Go (this repository is officially supported on the last two releases of Go)
* [Modules](https://github.com/golang/go/wiki/Modules) to manage dependencies

### Installing
* `go get github.com/stellar/go-stellar-sdk/clients/horizonclient`

### Usage

``` golang
    ...
    import hClient "github.com/stellar/go-stellar-sdk/clients/horizonclient"
    ...

    // Use the default pubnet client
    client := hClient.DefaultPublicNetClient

    // Create an account request
    accountRequest := hClient.AccountRequest{AccountID: "GCLWGQPMKXQSPF776IU33AH4PZNOOWNAWGGKVTBQMIC5IMKUNP3E6NVU"}

    // Load the account detail from the network
    account, err := client.AccountDetail(accountRequest)
    if err != nil {
        fmt.Println(err)
        return
    }
    // Account contains information about the stellar account
    fmt.Print(account)
```
For more examples, refer to the [documentation](https://godoc.org/github.com/stellar/go-stellar-sdk/clients/horizonclient).

## Running the tests
Run the unit tests from the package directory: `go test`

## Contributing
Please read [Code of Conduct](https://github.com/stellar/.github/blob/master/CODE_OF_CONDUCT.md) to understand this project's communication rules.

To submit improvements and fixes to this library, please see [CONTRIBUTING](../CONTRIBUTING.md).

## License
This project is licensed under the Apache License - see the [LICENSE](../../LICENSE) file for details.
