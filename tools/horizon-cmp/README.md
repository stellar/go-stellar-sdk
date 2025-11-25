# Horizon cmp

Tool that compares the responses of two Horizon servers and shows the diffs.
Useful for checking for regressions.

## Install

Compile the `horizon-cmp` binary:

```bash
go install ./tools/horizon-cmp
```

## Usage

`horizon-cmp` can be run in two modes:

- Crawling: start with a set of paths (defined in [init_paths.go](https://github.com/stellar/go-stellar-sdk/blob/master/tools/horizon-cmp/init_paths.go)) and then uses `_links` to find new paths.
- ELB access log: send requests found in a provided ELB access log.

### Crawling mode

To run in crawling mode specify a `base` and `test` URL, where `base` is the current version of Horizon and `test` is the version you want to test.

```bash
horizon-cmp -t https://new-horizon.host.org -b https://horizon.stellar.org
```

The paths to be tested can be found in [init_paths.go](https://github.com/stellar/go-stellar-sdk/blob/master/tools/horizon-cmp/init_paths.go).

### ELB access log

To run using an ELB access log, use the flag `-a`.

```bash
horizon-cmp -t https://new-horizon.host.org -b https://horizon.stellar.org -a ./elb_access.log
```

Additionally you can specify which line to start in by using the flag `-s`.

### History

You can use the `history` command to compare the history endpoints for a given range of ledgers.

```
horizon-cmp history -t https://new-horizon.domain.org -b https://base-horizon.domain.org
```

By default this command will check the last 120 ledgers (~10 minutes), but you can specify `--from` and `--to`.

```
horizon-cmp history -t https://new-horizon.domain.org -b https://base-horizon.domain.org --count 20
```

or

```
horizon-cmp history -t https://new-horizon.domain.org -b https://base-horizon.domain.org --from 10 --to 20
```


### Request per second

By default `horizon-cmp` will send 1 request per second, however, you can change this value using the `--rps` flag.  The following will run `10` request per second. Please note that sending too many requests to a production server can result in rate limiting of requests.

```bash
horizon-cmp -t https://new-horizon.host.org -b https://horizon.stellar.org --rps 10
```
