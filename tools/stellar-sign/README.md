# Stellar Sign

This folder contains `stellar-sign` a simple utility to make it easy to add your signature to a transaction envelope or to verify a transaction signature with a public key.  
When run on the terminal it:

1.  Prompts your for a base64-encoded envelope
2.  
    - If `-verify` is used
        - Asks for your public key
        - Outputs if the transaction has a valid signature or not
    - If in signature mode (default)
        - Asks for your private seed
        - Outputs a new envelope with your signature added

## Installing

```bash
$ go get -u github.com/stellar/go-stellar-sdk/tools/stellar-sign
```

## Running

```bash
$ stellar-sign --help
Usage of ./stellar-sign:
  -infile string
    	transaction envelope
  -testnet
    	Sign or verify the transaction using Testnet passphrase instead of Public
  -verify
    	Verify the transaction instead of signing
```

```bash
$ stellar-sign
```
