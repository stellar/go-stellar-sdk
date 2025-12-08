# Ingestion Library
The `ingest` package provides primitives for building custom ingestion engines.

Very often, developers need features that are outside of Horizon's scope. While it provides APIs for building the most common applications, it's not possible to add all possible features. That's why this package was created.



# Architecture
From a high level, the ingestion library is broken down into a few modular components:

```
                  [ Processors ]
                        |
                       / \
                      /   \
                     /     \
              [Change]      [Transaction]
                 |               |
             |---+---|           |
       Checkpoint Ledger      Ledger
         Change   Change    Transaction
         Reader   Reader      Reader

                [ Ledger Backend ]
                        |
                    |---+---|
                 Captive Buffered
                  Core   Storage
```

This is described in a little more detail in [`doc.go`](./doc.go), its accompanying examples, the documentation within this package, and the rest of this tutorial.



# Hello, World!
As is tradition, we'll start with a simplistic example that ingests a single ledger from the network. We'll use the **Captive Stellar-Core backend** to ingest the ledger:

```go
package main

import (
	"context"
	"fmt"

	backends "github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
)

func main() {
	ctx := context.Background()
	backend, err := backends.NewCaptive(config)
	panicIf(err)
	defer backend.Close()

	// Prepare a single ledger to be ingested,
	err = backend.PrepareRange(ctx, backends.BoundedRange(123456, 123456))
	panicIf(err)

	// then retrieve it:
	ledger, err := backend.GetLedger(ctx, 123456)
	panicIf(err)

	// Now `ledger` is a raw `xdr.LedgerCloseMeta` object containing the
	// transactions contained within this ledger.
	fmt.Printf("\nHello, Sequence %d.\n", ledger.LedgerSequence())
}
```

_(The `panicIf` function is defined in the [footnotes](#footnotes); it's used here for error-checking brevity.)_

Notice that the mysterious `config` variable above isn't defined. This will be environment-specific and refer to the code [here](./ledgerbackend/captive_core_backend.go) for the complete list of configuration parameters. For now, we'll use the [default](../network/main.go) values defined for the SDF testnet:

```go
archiveURLs := network.TestNetworkhistoryArchiveURLs
networkPassphrase := network.TestNetworkPassphrase
captiveCoreToml, err := ledgerbackend.NewCaptiveCoreToml(
	ledgerbackend.CaptiveCoreTomlParams{
		NetworkPassphrase:  networkPassphrase,
		HistoryArchiveURLs: archiveURLs,
		},
	})
panicIf(err)

config := ledgerbackend.CaptiveCoreConfig{
	// Change these based on your environment:
	BinaryPath:         "/usr/bin/stellar-core",
	NetworkPassphrase:  networkPassphrase,
	HistoryArchiveURLs: archiveURLs,
	Toml:               captiveCoreToml,
}
```

(Again, see the format of the stub file, etc. in the linked docs.)

Running this should dump a ton of logs while Captive Core boots up, downloads a history archive, and ultimately pops up the ledger sequence number we ingested:

```
$ go run ./example.go
INFO[...] default: Config from /tmp/captive-stellar-core365405852/stellar-core.conf  pid=20574
INFO[...] default: RUN_STANDALONE enabled in configuration file - node will not function properly with most networks  pid=20574
INFO[...] default: Generated QUORUM_SET: {              pid=20574
INFO[...] "t" : 2,                                      pid=20574
INFO[...] "v" : [ "sdf_testnet_2", "sdf_testnet_3", "sdf_testnet_1" ]  pid=20574
INFO[...] }                                             pid=20574
INFO[...] default: Assigning calculated value of 1 to FAILURE_SAFETY  pid=20574
INFO[...] Database: Connecting to: sqlite3://:memory:   pid=20574
INFO[...] SCP: LocalNode::LocalNode@GCVAA qSet: 59d361  pid=20574
INFO[...] default: *                                    pid=20574
INFO[...] default: * The database has been initialized  pid=20574
INFO[...] default: *                                    pid=20574
INFO[...] Database: Applying DB schema upgrade to version 13  pid=20574
INFO[...] Database: Adding column 'ledgerext' to table 'accounts'  pid=20574
...
INFO[...] Ledger: Established genesis ledger, closing   pid=20574
INFO[...] Ledger: Root account seed: SDHOAMBNLGCE2MV5ZKIVZAQD3VCLGP53P3OBSBI6UN5L5XZI5TKHFQL4  pid=20574
INFO[...] default: *                                    pid=20574
INFO[...] default: * The next launch will catchup from the network afresh.  pid=20574
INFO[...] default: *                                    pid=20574
INFO[...] default: Application destructing              pid=20574
INFO[...] default: Application destroyed                pid=20574
...
INFO[...] History: Starting catchup with configuration: pid=20574
INFO[...] lastClosedLedger: 1                           pid=20574
INFO[...] toLedger: 123457                              pid=20574
INFO[...] count: 2                                      pid=20574
INFO[...] History: Catching up to ledger 123457: Downloading state file history/00/01/e2/history-0001e27f.json for ledger 123519  pid=20574
...
INFO[...] History: Catching up to ledger 123457: downloading and verifying buckets: 16/17 (94%)  pid=20574
INFO[...] History: Verifying bucket d4db982884941c0b82422996e26ae0778b4a85385ef657ffacee9b11adf72882  pid=20574
INFO[...] History: Catching up to ledger 123457: Succeeded: download-verify-buckets : 17/17 children completed  pid=20574
INFO[...] History: Applying buckets                     pid=20574
INFO[...] History: Catching up to ledger 123457: Applying buckets 0%. Currently on level 9  pid=20574
...
INFO[...] Bucket: Bucket-apply: 158366 entries in 17.12MB/17.12MB in 17/17 files (100%)  pid=20574
INFO[...] History: Catching up to ledger 123457: Applying buckets 100%. Currently on level 0  pid=20574
INFO[...] History: ApplyBuckets : done, restarting merges  pid=20574
INFO[...] History: Catching up to ledger 123457: Succeeded: download-verify-apply-buckets  pid=20574
INFO[...] History: Downloading, unzipping and applying transactions for checkpoint 123519  pid=20574
INFO[...] History: Catching up to ledger 123457: Download & apply checkpoints: num checkpoints left to apply:1 (0% done)  pid=20574

Hello, Ledger #123456.
```

There's obviously much, *much* more we can do with the ingestion library. Let's work through some more comprehensive examples.



# **Example**: Ledger Statistics
In this section, we'll demonstrate how to combine a backend with a reader to actually learn something meaningful about the Stellar network. Again, we'll use a specific backend here (Captive Core, again), but the processing can be done with any of them.

More specifically, we're going to analyze the ledgers and track some statistics about the success/failure of transactions and their relative operations using `LedgerTransactionReader`. While this is technically doable by manipulating the Horizon API and some fancy JSON parsing, it serves as a useful yet concise demonstration of the ingestion library's features.


## Preamble
Let's get the boilerplate out of the way first. Again, we presume `config` is some sensible Captive Core configuration.

```go
package main

import (
	"context"
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
	"github.com/stellar/go-stellar-sdk/ingest"
	backends "github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
	"github.com/stellar/go-stellar-sdk/support/log"
)

func statistics() {
	ctx := context.Background()
	// Only log errors from the backend to keep output cleaner.
	lg := log.New()
	lg.SetLevel(logrus.ErrorLevel)
	config.Log = lg

	backend, err := backends.NewCaptive(config)
	panicIf(err)
	defer backend.Close()

	// ...
```

## Reading Transactions
Now, let's identify a range of ledgers we wish to process. For simplicity, let's work on the first 10,000 ledgers on the network.

```go
	// Prepare a range to be ingested:
	var startingSeq uint32 = 2 // can't start with genesis ledger
	var ledgersToRead uint32 = 10000

	fmt.Printf("Preparing range (%d ledgers)...\n", ledgersToRead)
	ledgerRange := backends.BoundedRange(startingSeq, startingSeq+ledgersToRead)
	err = backend.PrepareRange(ctx, ledgerRange)
	panicIf(err)
```

This part will take a bit of time as Captive Core (or whatever backend) processes these ledgers and prepares them for ingestion.

Now, we'll actually use a `LedgerTransactionReader` object to use the backend and read the transactions ledger by ledger. It takes the backend, the network passphrase, and the ledger you'd like to process as parameters, giving you back an object that returns raw transaction objects row by row.

```go
	// These are the statistics that we're tracking.
	var successfulTransactions, failedTransactions int
	var operationsInSuccessful, operationsInFailed int

	for seq := startingSeq; seq <= startingSeq+ledgersToRead; seq++ {
		fmt.Printf("Processed ledger %d...\r", seq)

		txReader, err := ingest.NewLedgerTransactionReader(
			ctx, backend, config.NetworkPassphrase, seq,
		)
		panicIf(err)
		defer txReader.Close()
```

Each ledger likely has many transactions, so we nest in another loop to process them all:

```go
		// Read each transaction within the ledger, extract its operations, and
		// accumulate the statistics we're interested in.
		for {
			tx, err := txReader.Read()
			if err == io.EOF {
				break
			}
			panicIf(err)

			envelope := tx.Envelope
			operationCount := len(envelope.Operations())
			if tx.Result.Successful() {
				successfulTransactions++
				operationsInSuccessful += operationCount
			} else {
				failedTransactions++
				operationsInFailed += operationCount
			}
		}
	} // outer loop
```

And that's it! We can print the statistics out of interest:

```go
	fmt.Println("\nDone. Results:")
	fmt.Printf("  - total transactions: %d\n", successfulTransactions+failedTransactions)
	fmt.Printf("  - succeeded / failed: %d / %d\n", successfulTransactions, failedTransactions)
	fmt.Printf("  - total operations:   %d\n", operationsInSuccessful+operationsInFailed)
	fmt.Printf("  - succeeded / failed: %d / %d\n", operationsInSuccessful, operationsInFailed)
} // end of main
```

As of this writing, the stats are as follows:

    Results:
      - total transactions: 24159
      - succeeded / failed: 16037 / 8122
      - total operations:   33845
      - succeeded / failed: 25387 / 8458

The full, runnable example is available [here](./tutorial/example_statistics.go).


# **Example**: Feature Popularity
In this example, we'll leverage the `CheckpointChangeReader` to determine the popularity of a feature introduced in [Protocol 15](https://www.stellar.org/blog/protocol-14-improvements): claimable balances. Specifically, we'll be investigating how many claimable balances were created in an arbitrary ledger range.

Let's begin. As before, there's a bit of boilerplate necessary. There's only a single additional import necessary relative to the [previous Preamble](#preamble). Since we're working with checkpoint ledgers, history archives come into play:

```go
import "github.com/stellar/go-stellar-sdk/historyarchive"
```

This time, we don't need a `LedgerBackend` instance whatsoever. The ledger changes we want to process will be fed into the reader through a different means. In our example, the history archives have the ~droids~ ledgers that we are looking for.


## History Archive Connections
First thing's first: we need to establish a connection to a history archive.

```go
	// Open a history archive using our existing configuration details.
	historyArchive, err := historyarchive.Connect(
		config.HistoryArchiveURLs[0],	// assumes a CaptiveCoreConfig
		historyarchive.ConnectOptions{
			NetworkPassphrase: config.NetworkPassphrase,
			S3Region:          "us-west-1",
			UnsignedRequests:  false,
		},
	)
	panicIf(err)
```

## Tracking Changes
Each history archive contains the current cumulative state of the entire network. 

Now we can use the history archive to actually read in all of the changes that have accumulated in the entire network by a particular checkpoint.

```go
	// First, we need to establish a safe fallback in case of any problems
	// during the history archive download+processing, so we'll set a 30-second
	// timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	reader, err := ingest.NewCheckpointChangeReader(ctx, historyArchive, 123455)
	panicIf(err)
```

In our examples, we refer to the testnet, whose archives are much smaller. When using the pubnet, a 30 *minute* timeout may be more appropriate (depending on system specs): Horizon takes around 15-20 minutes to process pubnet history archives.

By default, checkpoints occur every 64 ledgers (see `historyarchive.ConnectOptions` for changing this). More specifically, given ledger `n`, if `n+1 mod 64 == 0`, then `n` is a checkpoint ledger. Alternatively, this is when `n*64 - 1` for `n = 1, 2, 3, ...` and so on. This is true above for `n == 123455`.

Since history archives store global cumulative state, our `ChangeReader` will report every entry as being "new", reading out a list of *all* ledger entries. We can then process them and establish how many claimable balances have been created in the testnet's lifetime:

```go
	entries, newCBs := 0, 0
	for {
		entry, err := reader.Read()
		if err == io.EOF {
			break
		}
		panicIf(err)

		entries++

		switch entry.Type {
		case xdr.LedgerEntryTypeClaimableBalance:
			newCBs++
		// these are included for completeness of the demonstration
		case xdr.LedgerEntryTypeAccount:
		case xdr.LedgerEntryTypeData:
		case xdr.LedgerEntryTypeTrustline:
		case xdr.LedgerEntryTypeOffer:
		default:
			panic(fmt.Errorf("Unknown type: %+v", entry.Type))
		}
	}

	fmt.Printf("%d/%d entries were claimable balances\n", newCBs, entries)
} // end of main()
```



# Snippets
This section outlines a brief collection of common things you may want to do with the library. We assume a very generic `backend` variable where necessary that is one of the aforementioned `LedgerBackend` instances to avoid boilerplate.


### Controlling `LedgerBackend` log verbosity
Certain backends (like Captive Core) can be very noisy; they will log to standard output by default at the "Info" level.

You can suppress many logs by changing the level to only print warnings and errors:

```go
package main

import (
  ingest "github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
  "github.com/stellar/go-stellar-sdk/support/log"
  "github.com/sirupsen/logrus"
)

func main() {
  lg := log.New()
  lg.SetLevel(logrus.WarnLevel)
  config.Log = lg // assume config is otherwise predefined

  backend, err := ingest.NewCaptive(config) // (or other backend)
  // ...
}
```

Or even disable output entirely by redirecting to `ioutil.Discard`:

```go
lg.Entry.Logger.Out = ioutil.Discard
```


#### Footnotes

  1. The minimalist error handler (if `panic`king counts as "handling" an error) `panicIf` used throughout this tutorial is defined simply as:

```go
func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}
```

  **Please don't use it in production code**; it's provided here for completeness, convenience, and brevity of examples.

  2. Since the Stellar testnet undergoes periodic resets, the example outputs from various sections (especially regarding network statistics) will not always be accurate.

  3. It's worth noting that even though the [second example](#example-feature-popularity) could *also* be done by using the `LedgerTransactionReader` and inspecting the individual operations, that'd be bit redundant as far as examples go.
