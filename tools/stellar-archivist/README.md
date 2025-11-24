# stellar-archivist

This is a small tool, written in Go, for working with `stellar-core` history archives directly.
It is a standalone tool that does not require `stellar-core`, or any other programs.

It is much smaller and simpler than `stellar-core`, and is intended only for archive-maintenance tasks.

  - reporting the current state of an archive
  - mirroring archives, or portions of archives
  - scanning all or recent portions of archives for missing files
  - repairing archives by copying missing files from other archives
  - performing integrity checks on files

## Installation

```
$ go install github.com/stellar/go-stellar-sdk/tools/stellar-archivist
```

## Usage

```
inspect stellar history archive

Usage:
  stellar-archivist [flags]
  stellar-archivist [command]

Available Commands:
  dumpxdr
  mirror
  repair
  scan
  status

Flags:
  -c, --concurrency int   number of files to operate on concurrently (default 32)
  -n, --dryrun            describe file-writes, but do not perform any
  -f, --force             overwrite existing files
  -h, --help              help for stellar-archivist
      --high int          last ledger to act on (default 4294967295)
      --last int          number of recent ledgers to act on (default -1)
      --low int           first ledger to act on
      --profile           collect and serve profile locally
  -r, --recent            act on ledger-range difference between achives
      --s3region string   S3 region to connect to (default "us-east-1")
      --s3endpoint string S3 endpoint (default to AWS endpoint for selected region)
      --skip-optional     skip optional (SCP) checkpoint files
      --thorough          decode and re-encode all buckets
      --verify            verify file contents

Use "stellar-archivist [command] --help" for more information about a command.
```

## Specifying history archives

Unlike `stellar-core`, `stellar-archivist` does not run subprocesses to access history archives;
instead it operates directly on history archives given by URLs. Currently it understands URLs
of the following schemes:

  - `http://hostname/path/to/archive`
  - `s3://bucketname/prefix`
  - `file://path/to/archive`

Supporting an additional URL scheme requires writing a new archive backend implementation; see
for example [the S3 backend](s3_archive.go).

The disadvantage of this approach is that it requires special-purpose code to support each type of
archive; the advantage is that more operations are supported, and the tool can scan and operate on
archives much more quickly. This is necessary to handle bulk operations on archives with many
thousands of files efficiently.

### S3 backend

`stellar-archivist` supports reading from and writing to any S3-compatible storage.

The following options are specific to S3 backend:

 - `--s3region string` — AWS S3 region to connect to (default "us-east-1")
 - `--s3endpoint string` — S3-compatible endpoint (default to AWS S3 endpoint for selected region)

For example, to check the current status of an archive in DigitalOcean Spaces (ams3 region):

```
$ stellar-archivist status --s3endpoint ams3.digitaloceanspaces.com s3://bucketname/prefix
```

In order to use this backend with Google Cloud Storage, you need to enable interoperability access in
the [Cloud Storage Settings](https://console.cloud.google.com/storage/settings) and generate interoperable 
storage access keys.

```
$ export AWS_ACCESS_KEY_ID=<interoperable storage access key> AWS_SECRET_ACCESS_KEY=<interoperable storage secret key> 
$ stellar-archivist status --s3endpoint https://storage.googleapis.com s3://google-storage-bucketname
``` 

## Examples of use

### Reporting the current status of an archive:

```
$ stellar-archivist status http://s3-eu-west-1.amazonaws.com/history.stellar.org/prd/core-testnet/core_testnet_001

       Archive: http://s3-eu-west-1.amazonaws.com/history.stellar.org/prd/core-testnet/core_testnet_001
        Server: v0.4.0-34-g2f015f6
 CurrentLedger: 2470911 (0x0025b3ff)
CurrentBuckets: ____####### (7 nonzero levels)
 Newest bucket: b9d345d89ffe039edba65387dbe3770e16e7bd2095159213eb1c2920988e30dd

```

### Mirroring an archive
```
$ stellar-archivist mirror http://s3-eu-west-1.amazonaws.com/history.stellar.org/prd/core-testnet/core_testnet_001 file://local-archive

2016/02/10 18:27:09 mirroring http://s3-eu-west-1.amazonaws.com/history.stellar.org/prd/core-testnet/core_testnet_001 -> file://local-archive
2016/02/10 18:27:10 copying range [0x0000003f, 0x0025b3ff]
2016/02/10 18:31:20 Copied 4096/38607 checkpoints (10.609475%), 10386 buckets
2016/02/10 18:33:26 Copied 8192/38607 checkpoints (21.218950%), 11524 buckets
...
2016/02/10 18:49:39 Copied 36864/38607 checkpoints (95.485275%), 23168 buckets
2016/02/10 18:50:37 Copied 38607 checkpoints, 23715 buckets

```

### Incremental update to a mirror with --recent
```
$ stellar-archivist mirror --recent http://history.stellar.org/prd/core-live/core_live_001 file://local-archive

2019/08/21 13:53:15 mirroring http://history.stellar.org/prd/core-live/core_live_001 -> file://local-archive
2019/08/21 13:53:15 copying range [0x01843cbf, 0x01843d7f]
2019/08/21 13:53:19 skipping existing bucket/18/43/cc/bucket-1843cce32e1c4d6d0765858c9464a7435a6f46c25c8ab164a0d9a11b3da5098b.xdr.gz
2019/08/21 13:53:20 skipping existing bucket/ff/08/f4/bucket-ff08f47f9d90fdaed40c1d6f0fdb1ea20344cdfac18849690d52aab7125c303c.xdr.gz
...
2019/08/21 13:53:20 skipping existing bucket/1e/1e/97/bucket-1e1e97031a1829fe3dff1875abed509103f5fc0569eef0718e02392d411b9c0a.xdr.gz
2019/08/21 13:53:20 skipping existing bucket/36/e1/0c/bucket-36e10c7087567c3dd230e615851771062acc71cb3e8e7bc08605b1829f49f53c.xdr.gz
2019/08/21 13:53:22 copied 3 checkpoints, 49 buckets, range [0x01843cbf, 0x01843d7f]
```

### Incremental update to a mirror with --last N
```
$ stellar-archivist --last 1024 mirror http://history.stellar.org/prd/core-testnet/core_testnet_001 file://local-archive

2016/02/10 19:14:01 mirroring http://s3-eu-west-1.amazonaws.com/history.stellar.org/prd/core-testnet/core_testnet_001 -> file://local-archive
2016/02/10 19:14:02 copying range [0x0025b23f, 0x0025b6bf]
2016/02/10 19:14:02 skipping existing bucket/b9/d3/45/bucket-b9d345d89ffe039edba65387dbe3770e16e7bd2095159213eb1c2920988e30dd.xdr.gz
2016/02/10 19:14:02 skipping existing bucket/5c/c7/45/bucket-5cc745c8b08784c031e821f3a34f943f77e82018c9f5ffffa7f8f314170e0139.xdr.gz
...
2016/02/10 19:14:02 skipping existing bucket/1d/bb/14/bucket-1dbb140a3e8127ac99ee1169a1581ca37ad718c4f50200492a57201577070772.xdr.gz
2016/02/10 19:14:02 skipping existing history/00/25/b2/history-0025b23f.json
2016/02/10 19:14:02 skipping existing ledger/00/25/b2/ledger-0025b23f.xdr.gz
2016/02/10 19:14:02 skipping existing transactions/00/25/b2/transactions-0025b23f.xdr.gz
...
2016/02/10 19:14:02 skipping existing scp/00/25/b2/scp-0025b2ff.xdr.gz
2016/02/10 19:14:03 Copied 18 checkpoints, 18 buckets

```

### Scanning an entire archive (for missing files)

```
$ stellar-archivist scan file://local-archive

2016/02/10 19:01:57 Scanning checkpoint files in range: [0x0000003f, 0x0025b3ff]
2016/02/10 19:01:57 Archive: 4077 history, 0 ledger, 0 transactions, 0 results, 0 scp
2016/02/10 19:01:57 Archive: 8192 history, 0 ledger, 0 transactions, 0 results, 0 scp
...
2016/02/10 19:02:09 Archive: 23715 buckets total, 23715 referenced
2016/02/10 19:02:09 Examining checkpoint files for gaps
2016/02/10 19:02:10 Examining buckets referenced by checkpoints
2016/02/10 19:02:10 No checkpoint files missing in range [0x0000003f, 0x0025b3ff]
2016/02/10 19:02:10 No missing buckets referenced in range [0x0000003f, 0x0025b3ff]

```

### Scanning a range of an archive

```
$ stellar-archivist --last 4096 scan file://local-archive

2016/02/10 19:03:55 Scanning checkpoint files in range: [0x0025a37f, 0x0025b3ff]
2016/02/10 19:03:55 Checkpoint files scanned with 0 errors
2016/02/10 19:03:55 Archive: 67 history, 67 ledger, 67 transactions, 67 results, 67 scp
2016/02/10 19:03:55 Scanning all buckets, and those referenced by range
2016/02/10 19:03:55 Archive: 4097 buckets total, 0 referenced
2016/02/10 19:03:56 Archive: 8193 buckets total, 0 referenced
...
2016/02/10 19:03:57 Archive: 23715 buckets total, 33 referenced
2016/02/10 19:03:57 Examining checkpoint files for gaps
2016/02/10 19:03:57 Examining buckets referenced by checkpoints
2016/02/10 19:03:57 No checkpoint files missing in range [0x0025a37f, 0x0025b3ff]
2016/02/10 19:03:57 No missing buckets referenced in range [0x0025a37f, 0x0025b3ff]

$ stellar-archivist --low 1000000 --high 1006000 scan file://local-archive

2016/02/10 19:23:51 Scanning checkpoint files in range: [0x000f41ff, 0x000f59bf]
2016/02/10 19:23:51 Checkpoint files scanned with 0 errors
2016/02/10 19:23:51 Archive: 100 history, 100 ledger, 100 transactions, 100 results, 100 scp
2016/02/10 19:23:51 Scanning all buckets, and those referenced by range
2016/02/10 19:23:53 Archive: 4096 buckets total, 0 referenced
2016/02/10 19:23:54 Archive: 8193 buckets total, 0 referenced
...
2016/02/10 19:23:57 Archive: 23716 buckets total, 61 referenced
2016/02/10 19:23:57 Examining checkpoint files for gaps
2016/02/10 19:23:57 Examining buckets referenced by checkpoints
2016/02/10 19:23:57 No checkpoint files missing in range [0x000f41ff, 0x000f59bf]
2016/02/10 19:23:57 No missing buckets referenced in range [0x000f41ff, 0x000f59bf]

```

### Scanning and verifying contents of files

```
$ stellar-archivist --verify --last 4096 scan file://local-archive

2016/02/10 19:05:29 Scanning checkpoint files in range: [0x0025a37f, 0x0025b3ff]
2016/02/10 19:05:29 Checkpoint files scanned with 0 errors
2016/02/10 19:05:29 Archive: 67 history, 67 ledger, 67 transactions, 67 results, 67 scp
2016/02/10 19:05:29 Scanning all buckets, and those referenced by range
2016/02/10 19:05:29 Archive: 4097 buckets total, 0 referenced
2016/02/10 19:05:30 Archive: 8192 buckets total, 0 referenced
...
2016/02/10 19:05:31 Archive: 23715 buckets total, 33 referenced
2016/02/10 19:05:31 Examining checkpoint files for gaps
2016/02/10 19:05:31 Examining buckets referenced by checkpoints
2016/02/10 19:05:31 No checkpoint files missing in range [0x0025a37f, 0x0025b3ff]
2016/02/10 19:05:31 No missing buckets referenced in range [0x0025a37f, 0x0025b3ff]
2016/02/10 19:05:31 Verified 4288 ledger headers have expected hashes
2016/02/10 19:05:31 Verified 4288 transaction sets have expected hashes
2016/02/10 19:05:31 Verified 4288 transaction result sets have expected hashes
2016/02/10 19:05:31 Verified 33 buckets have expected hashes

```

### Repairing missing files

```
$ cp -a local-archive broken-archive

$ rm broken-archive/transactions/00/10/f7/*

$ stellar-archivist repair file://local-archive file://broken-archive

2016/02/10 19:10:53 repairing file://local-archive -> file://broken-archive
2016/02/10 19:10:53 Starting scan for repair
2016/02/10 19:10:53 Scanning checkpoint files in range: [0x0000003f, 0x0025b3ff]
2016/02/10 19:10:53 Archive: 4085 history, 0 ledger, 0 transactions, 0 results, 0 scp
2016/02/10 19:10:53 Archive: 8173 history, 0 ledger, 0 transactions, 0 results, 0 scp
...
2016/02/10 19:10:58 Archive: 38607 history, 38607 ledger, 38603 transactions, 38607 results, 38087 scp
2016/02/10 19:10:58 Checkpoint files scanned with 0 errors
2016/02/10 19:10:58 Archive: 38607 history, 38607 ledger, 38603 transactions, 38607 results, 38607 scp
2016/02/10 19:10:58 Examining checkpoint files for gaps
2016/02/10 19:10:58 Repairing transactions/00/01/f7/transactions-0001f73f.xdr.gz
2016/02/10 19:10:58 Repairing transactions/00/01/f7/transactions-0001f77f.xdr.gz
2016/02/10 19:10:58 Repairing transactions/00/01/f7/transactions-0001f7bf.xdr.gz
2016/02/10 19:10:58 Repairing transactions/00/01/f7/transactions-0001f7ff.xdr.gz
2016/02/10 19:10:58 Scanning all buckets, and those referenced by range
2016/02/10 19:10:59 Archive: 4096 buckets total, 0 referenced
2016/02/10 19:11:00 Archive: 8192 buckets total, 0 referenced
...
2016/02/10 19:11:07 Archive: 23715 buckets total, 23715 referenced
2016/02/10 19:11:07 Examining buckets referenced by checkpoints

```

### Dumping an XDR file from an archive as JSON

```
 stellar-archivist dumpxdr local-archive/transactions//00/20/de/transactions-0020de7f.xdr.gz

{
    "LedgerSeq": 2154109,
    "TxSet": {
        "PreviousLedgerHash": [...]
        "Txs": [
            {
                "Tx": {
                    "SourceAccount": {
                        "Type": 0,
                        "Ed25519": [...]
                    },
                    "Fee": 100,
                    "SeqNum": 2371491962290216,
                    "TimeBounds": null,
                    "Memo": {
                        "Type": 0,
                        "Text": null,
                        "Id": null,
                        "Hash": null,
                        "RetHash": null
                    },
                    "Operations": [
                        {
                            "SourceAccount": null,
                            "Body": {
                                "Type": 5,
                                ...
                                "SetOptionsOp": {
                                    "InflationDest": {
                                        "Type": 0,
                                        "Ed25519": [...]
                                    },
                                    "ClearFlags": null,
                                    "SetFlags": null,
                                    "MasterWeight": null,
                                    "LowThreshold": null,
                                    "MedThreshold": null,
                                    "HighThreshold": null,
                                    "HomeDomain": "centaurus.xcoins.de",
                                    "Signer": null
                                },
                                "ChangeTrustOp": null,
                                "AllowTrustOp": null,
                                "Destination": null
                            }
                        }
                    ],
                    "Ext": {
                        "V": 0
                    }
                },
                "Signatures": [
                    {
                        "Hint": [...]
                        "Signature": "..."
                    }
                ]
            }
        ]
    },
    "Ext": {
        "V": 0
    }
}

$
```
