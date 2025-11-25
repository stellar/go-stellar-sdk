# Changelog

All notable changes to this project will be documented in this
file.  This project adheres to [Semantic Versioning](http://semver.org/).

## Unreleased

## [11.0.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v11.0.0) - 2023-03-29

### Breaking changes

* Muxed accounts and ID memos can be used in the `BuildChallengeTx()` and `ReadChallengeTx()` SEP-10 utilitiy functions to identify users of shared Stellar accounts. ([#4746](https://github.com/stellar/go-stellar-sdk/pull/4746))
  * `BuildChallengeTx()`:
    * Muxed account addresses can be passed as the `clientAccountID`.
    * Adds an additional parameter of type `*txnbuild.MemoID`. Memos cannot be specified if the `clientAccoutID` id a muxed account address.
  * `ReadChallengeTx()`:
    * Muxed account addresses may be returned as the `clientAccountID`.
    * Adds an additional return value of type `*txnbuild.MemoID`.


## [10.0.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v10.0.0) - 2022-04-18

* Adds support for Protocol 19 transaction preconditions ([CAP-21](https://stellar.org/protocol/cap-21)).

### Breaking changes

* There are many new ways for a transaction to be (in)valid (see the new `Preconditions` structure), and the corresponding breaking change is in how transactions are built:

```diff
 tx, err := NewTransaction(TransactionParams{
     SourceAccount: someAccount,
     // ... other parameters ...
-    Timebounds:    NewTimeout(5),
+    Preconditions: Preconditions{TimeBounds: NewTimeout(5)},
 })
```

* `Timebounds` has been renamed to `TimeBounds`, though a type alias remains.

* A `*TimeBounds` structure is no longer considered valid (via `Validate()`) if it's `nil`. This further reinforces the fact that transactions need timebounds.


## [9.0.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v9.0.0) - 2022-01-10

* Enable Muxed Accounts ([SEP-23](https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0023.md)) by default ([#4169](https://github.com/stellar/go-stellar-sdk/pull/4169)):
  * Remove `TransactionParams.EnableMuxedAccounts`
  * Remove `TransactionFromXDROptionEnableMuxedAccounts`
  * Remove `FeeBumpTransactionParams.EnableMuxedAccounts`
  * Remove parameter `withMuxedAccounts bool` from all methods/functions.
  * Remove `options ...TransactionFromXDROption` parameter from `TransactionFromXDR()`
  * Rename `SetOpSourceMuxedAccount()` to (pre-existing) `SetOpSourceAccount()` which now accepts
    both `G` and `M` (muxed) account strkeys.
* Use xdr.Price to represent prices in txnbuild instead of strings ([#4167](https://github.com/stellar/go-stellar-sdk/pull/4167)).
  
## [8.0.0-beta.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v8.0.0-beta.0) - 2021-10-04

**This release adds support for Protocol 18.**

### New features
* `GenericTransaction`, `Transaction`, and `FeeBumpTransaction` now implement
`encoding.TextMarshaler` and `encoding.TextUnmarshaler`.
* New asset structures that conform to the new ChangeTrust and New assets: 
* Support for the core liquidity pool XDR types: `LiquidityPoolId`, `LiquidityPoolParameters`, `LiquidityPoolDeposit`, and `LiquidityPoolWithdraw`.
* Support for the new asset structures: `ChangeTrustAsset` and `TrustLineAsset`.

### Changes
* There's now a 5-minute grace period to `transaction.ReadChallengeTx`'s minimum time bound constraint ([#3824](https://github.com/stellar/go-stellar-sdk/pull/3824)).
* Assets can now be liquidity pool shares (`AssetTypePoolShare`).
* All asset objects can now be converted to the new `ChangeTrustAsset` and `TrustLineAsset` objects.
* Assets can now be compared in accordance with the protocol, see their respective `LessThan()` implementations.

### Breaking changes
* `ChangeTrust` requires a `ChangeTrustAsset`.
* `RevokeSponsorship` requires a `TrustLineAsset` when revoking trustlines.
* `RemoveTrustlineOp` helper now requires a `ChangeTrustAsset`
* `validate*Asset` helpers now require more-specific asset types.


## [v7.1.1](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v7.1.1) - 2021-06-25

### Bug Fixes

* Claimable balance IDs are now precomputed correctly (`Transaction.ClaimableBalanceID(int)`) even when the transaction's source account is a fully-muxed account ([#3678](https://github.com/stellar/go-stellar-sdk/pull/3678)).
* Fix muxed account address parsing for account merge operation ([#3722](https://github.com/stellar/go-stellar-sdk/pull/3722)).

## [v7.1.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v7.1.0) - 2021-06-01

### New features

* Add `Transaction.SequenceNumber()` helper function to make retrieving the underlying sequence number easier ([#3616](https://github.com/stellar/go-stellar-sdk/pull/3616)).

* Add `Transaction.AddSignatureDecorated()` helper function to make attaching decorated signatures to existing transactions easier ([#3640](https://github.com/stellar/go-stellar-sdk/pull/3640)).

### Bug Fix

* `BaseFee` in `TransactionParams` when calling `NewTransaction` is allowed to be zero because the fee can be paid by wrapping a `Transaction` in a `FeeBumpTransaction` ([#3622](https://github.com/stellar/go-stellar-sdk/pull/3622)).


## [v7.0.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v7.0.0) - 2021-05-15

### Breaking changes

* `AllowTrustOpAsset` was renamed to `AssetCode`, `{Must}NewAllowTrustAsset` was renamed to `{Must}NewAssetCodeFromString`.
* Some methods from the `Operation` interface (`BuildXDR()`,`FromXDR()` and `Validate()`) now take an additional `bool` parameter (`withMuxedAccounts`)
  to indicate whether [SEP23](https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0023.md) M-strkeys should be enabled.

### New features

* Add support for Stellar Protocol 17 (CAP35): `Clawback`, `ClawbackClaimableBalance` and `SetTrustlineFlags` operations.
* Add opt-in support for [SEP23](https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0023.md) M-strkeys for `MuxedAccount`s:
  * Some methods from the `Operation` interface (`BuildXDR()`,`FromXDR()` and `Validate()`) now take an additional `bool` parameter (`withMuxedAccounts`)
  * The parameters from `NewFeeBumpTransaction()` and `NewTransaction()` now include a new field (`EnableMuxedAccounts`) to enable M-strekeys.
  * `TransactionFromXDR()` now allows passing a `TransactionFromXDROptionEnableMuxedAccounts` option, to enable M-strkey parsing.

## [v6.0.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v6.0.0) - 2021-02-22

### Breaking changes

* Updates the SEP-10 helper function parameters to support [SEP-10 v3.1](https://github.com/stellar/stellar-protocol/commit/6c8c9cf6685c85509835188a136ffb8cd6b9c11c).
  * The following functions added the `webAuthDomain` parameter:
    * `BuildChallengeTx()`
    * `ReadChallengeTx()`
    * `VerifyChallengeTxThreshold()`
    * `VerifyChallengeTxSigners()`
  * The webAuthDomain parameter is verified in the `Read*` and `Verify*` functions if it is contained in the challenge transaction, and is ignored if the challenge transaction was generated by an older implementation that does not support the webAuthDomain.
  * The webAuthDomain parameter is included in challenge transactions generated in the `Build*` function, and the resulting challenge transaction is compatible with SEP-10 v2.1 or greater.
* Use strings to represent source accounts in Operation structs ([#3393](https://github.com/stellar/go-stellar-sdk/pull/3393)) see example below:
  ```go
  bumpSequenceOp := txnbuild.BumpSequence{BumpTo: 100, SourceAccount: "GB56OJGSA6VHEUFZDX6AL2YDVG2TS5JDZYQJHDYHBDH7PCD5NIQKLSDO"}
  ```
* Remove `TxEnvelope()` functions from `Transaction` and `FeeBumpTransaction` to simplify the API. `ToXDR()` should be used instead of `TxEnvelope()` ([#3377](https://github.com/stellar/go-stellar-sdk/pull/3377))

## [v5.0.1](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v5.0.1) - 2021-02-16

* Fix a bug in `ClaimableBalanceID()` where the wrong account was used to derive the claimable balance id ([#3406](https://github.com/stellar/go-stellar-sdk/pull/3406))

## [v5.0.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v5.0.0) - 2020-11-12

### Breaking changes

* Updates the SEP-10 helper function parameters and return values to support [SEP-10 v3.0](https://github.com/stellar/stellar-protocol/commit/9d121f98fd2201a5edfe0ed2befe92f4bf88bfe4)
  * The following functions replaced the `homeDomain` parameter with `homeDomains` (note: plural):
    * `ReadChallengeTx()`
    * `VerifyChallengeTxThreshold()`
    * `VerifyChallengeTxSigners()`
  * `ReadChallengeTx()` now returns a third non-error value: `matchedHomeDomain`

## [v4.2.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v4.2.0) - 2020-11-11

* Add `HashHex()`, `SignWithKeyString()`, `SignHashX()`, and `AddSignatureBase64()` functions back to `FeeBumpTransaction`  ([#3199](https://github.com/stellar/go-stellar-sdk/pull/3199)).

## [v4.1.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v4.1.0) - 2020-10-16

* Add helper function `ParseAssetString()`, making it easier to build an `Asset` structure from a string in [canonical form](https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0011.md#asset) and check its various properties ([#3105](https://github.com/stellar/go-stellar-sdk/pull/3105)).

* Add helper function `Transaction.ClaimableBalanceID()`, making it easier to calculate balance IDs for [claimable balances](https://developers.stellar.org/docs/glossary/claimable-balance/) without actually submitting the transaction ([#3122](https://github.com/stellar/go-stellar-sdk/pull/3122)).

* Add support for SEP-10 v2.1.0.
  * Remove verification of home domain. (Will be reintroduced and changed in a future release.)
  * Allow additional manage data operations that have source account as the server key.

## [v4.0.1](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v4.0.1) - 2020-10-02

* Fixed bug in `TransactionFromXDR()` which occurs when parsing transaction XDR envelopes which contain Protocol 14 operations.

## [v4.0.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v4.0.0) - 2020-09-29

Added support for the new operations in [Protocol 14](https://github.com/stellar/go-stellar-sdk/issues/3035). Now it is possible to:
* Create and claim claimable balance operations (see [CAP-23](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0023.md)) with the `[Create|Claim]ClaimableBalance` structures and their associated helpers
* Begin/end sponsoring future reserves for other accounts (see [CAP-33](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0033.md)) with the `[Begin|End]SponsoringFutureReserves` operations
* Revoke sponsorships of various objects with the `RevokeSponsorship` operation (see [CAP-33](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0033.md)).

Also:
* Added support for Go 1.15.
### Breaking changes

* Dropped support for Go 1.13.
* Add support for SEP-10 v2.0.0.
  * Replace `BuildChallengeTx`'s `anchorName` parameter with `homeDomain`.
  * Add `homeDomain` parameter to `ReadChallengeTx`, `VerifyChallengeTxThreshold`, and `VerifyChallengeTxSigners`.

## [v3.2.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v3.2.0) - 2020-06-18

* `txnbuild` now generates V1 transaction envelopes which are only supported by Protocol 13 ([#2640](https://github.com/stellar/go-stellar-sdk/pull/2640))
* Add `ToXDR()` functions for `Transaction` and `FeeBumpTransaction` instances which return xdr transaction envelopes without errors ([#2651](https://github.com/stellar/go-stellar-sdk/pull/2651))

## [v3.1.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v3.1.0) - 2020-05-14

* Fix bug which occurs when parsing xdr offers with prices that require more than 7 decimals of precision ([#2588](https://github.com/stellar/go-stellar-sdk/pull/2588))
* Add `AddSignatureBase64` function to both `Transaction` and `FeeBumpTransaction` objects for adding a base64-encoded signature. [#2586](https://github.com/stellar/go-stellar-sdk/pull/2586)

## [v3.0.1](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v3.0.1) - 2020-05-11

* Fix bug which occurs when parsing transactions with manage data operations containing nil values ([#2573](https://github.com/stellar/go-stellar-sdk/pull/2573))

## [v3.0.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v3.0.0) - 2020-04-28

### Breaking changes

* The `Account` interface has been extended to include `GetSequenceNumber() (int64, error)`. Also, `IncrementSequenceNumber()` now returns an `(int64, error)` pair instead of a `(xdr.SequenceNumber, error)` pair.
* Refactor workflow for creating and signing transactions. Previously, you could create a transaction envelope by populating a `Transaction` instance and calling the `Build()` function on the `Transaction` instance.

`Transaction` is now an opaque type which has accessor functions like `SourceAccount() SimpleAccount`, `Memo() Memo`, etc. The motivation behind this change is to make `Transaction` more immutable. Here is an example of how to use the new transaction type:
```go
	kp := keypair.MustParse("SBPQUZ6G4FZNWFHKUWC5BEYWF6R52E3SEP7R3GWYSM2XTKGF5LNTWW4R")
	client := horizonclient.DefaultTestNetClient
	ar := horizonclient.AccountRequest{AccountID: kp.Address()}
	sourceAccount, err := client.AccountDetail(ar)
	check(err)

	op := txnbuild.Payment{
		Destination: "GCCOBXW2XQNUSL467IEILE6MMCNRR66SSVL4YQADUNYYNUVREF3FIV2Z",
		Amount:      "10",
		Asset:       NativeAsset{},
	}

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        &sourceAccount,
			// If IncrementSequenceNum is true, NewTransaction() will call `sourceAccount.IncrementSequenceNumber()`
			// to obtain the sequence number for the transaction.
			// If IncrementSequenceNum is false, NewTransaction() will call `sourceAccount.GetSequenceNumber()`
			// to obtain the sequence number for the transaction.
			IncrementSequenceNum: true,
			Operations:           []Operation{&op},
			BaseFee:              MinBaseFee,
			Timebounds:           NewInfiniteTimeout(),
		},
	)
	check(err)

	tx, err = tx.Sign(network.TestNetworkPassphrase, kp.(*keypair.Full))
```

* `TransactionFromXDR` now has the following signature `TransactionFromXDR(txeB64 string) (*GenericTransaction, error)`. A `GenericTransaction` is a container which can be unpacked into either a `Transaction` or a `FeeBumpTransaction`.
* `BuildChallengeTx` now returns a `Transaction` instance instead of the base 64 string encoding of the SEP 10 challenge transaction.
* `VerifyChallengeTx` has been removed. Use `VerifyChallengeTxThreshold` or `VerifyChallengeTxSigners` instead.

### Add

* Add `NewFeeBumpTransaction(params FeeBumpTransactionParams) (*FeeBumpTransaction, error)` function for creating [fee bump transactions](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0015.md). Note that fee bump transactions will only be accepted by Stellar Core once Protocol 13 is enabled.

### Updates

* `AllowTrust` supports [CAP0018](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0018.md) Fine-Grained Control of Authorization by exposing a `AuthorizeToMaintainLiabilities` boolean field.
* `ReadChallengeTx` will reject any challenge transactions which are fee bump transactions.
* `ReadChallengeTx` will reject any challenge transactions which contain a [MuxedAccount](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0027.md) with a memo ID.

### Remove

* Dropped support for Go 1.12.

## [v1.5.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v1.5.0) - 2019-10-09

* Dropped support for Go 1.10, 1.11.
* Add support for stellar-core [protocol 12](https://github.com/stellar/stellar-core/releases/tag/v12.0.0), which implements [CAP-0024](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0024.md) ("Make PathPayment Symmetrical"). ([#1737](https://github.com/stellar/go-stellar-sdk/issues/1737)).
* **Deprecated:** Following CAP-0024, the operation `txnbuild.PathPayment` is now deprecated in favour of [`txnbuild.PathPaymentStrictReceive`](https://godoc.org/github.com/stellar/go-stellar-sdk/txnbuild#PathPaymentStrictReceive), and will be removed in a future release. This is a rename - the new operation behaves identically to the old one. Client code should be updated to use the new operation.
* **Add:** New operation [`txnbuild.PathPaymentStrictSend`](https://godoc.org/github.com/stellar/go-stellar-sdk/txnbuild#PathPaymentStrictSend) allows a path payment to be made where the amount sent is specified, and the amount received can vary.

## [v1.4.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v1.4.0) - 2019-08-09

* Add `BuildChallengeTx` function for building [SEP-10](https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0010.md) challenge transaction([#1466](https://github.com/stellar/go-stellar-sdk/issues/1466)).
* Add `VerifyChallengeTx` method for verifying [SEP-10](https://github.com/stellar/stellar-protocol/blob/master/ecosystem/sep-0010.md) challenge transaction([#1530](https://github.com/stellar/go-stellar-sdk/issues/1530)).
* Add `TransactionFromXDR` function for building `txnbuild.Transaction` struct from a  base64 XDR transaction envelope[#1329](https://github.com/stellar/go-stellar-sdk/issues/1329).
* Fix bug that allowed multiple calls to `Transaction.Build` increment the number of operations in a transaction [#1448](https://github.com/stellar/go-stellar-sdk/issues/1448).
* Add `Transaction.SignWithKeyString` helper method for signing transactions using secret keys as strings.([#1564](https://github.com/stellar/go-stellar-sdk/issues/1564))


## [v1.3.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v1.3.0) - 2019-07-08

* Add support for getting the hex-encoded transaction hash with `Transaction.HashHex` method.
* `TransactionEnvelope` is now available after building a transaction(`Transaction.Build`). Previously, this was only available after signing a transaction. ([#1376](https://github.com/stellar/go-stellar-sdk/pull/1376))
* Add support for getting the `TransactionEnvelope` struct with `Transaction.TxEnvelope` method ([#1415](https://github.com/stellar/go-stellar-sdk/issues/1415)).
* `AllowTrust` operations no longer requires the asset issuer, only asset code is required ([#1330](https://github.com/stellar/go-stellar-sdk/issues/1330)).
* `Transaction.SetDefaultFee` method is deprecated and will be removed in the next major release ([#1221](https://github.com/stellar/go-stellar-sdk/issues/1221)).
* `Transaction.TransactionFee` method has been added to get the fee that will be paid for a transaction.
* `Transaction.SignHashX` method adds support for signing transactions with hash(x) signature types.

## [v1.2.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v1.2.0) - 2019-05-16

* In addition to account responses from horizon, transactions and operations can now be built with txnbuild.SimpleAccount structs constructed locally ([#1266](https://github.com/stellar/go-stellar-sdk/issues/1266)).
* Added `MaxTrustlineLimit` which represents the maximum value for a trustline limit ([#1265](https://github.com/stellar/go-stellar-sdk/issues/1265)).
* ChangeTrust operation with no `Limit` field set now defaults to `MaxTrustlineLimit` ([#1265](https://github.com/stellar/go-stellar-sdk/issues/1265)).
* Add support for building `ManageBuyOffer` operation ([#1165](https://github.com/stellar/go-stellar-sdk/issues/1165)).
* Fix bug in ChangeTrust operation builder ([1296](https://github.com/stellar/go-stellar-sdk/issues/1296)).

## [v1.1.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v1.1.0) - 2019-02-02

* Support for multiple signatures ([#1198](https://github.com/stellar/go-stellar-sdk/pull/1198))

## [v1.0.0](https://github.com/stellar/go-stellar-sdk/releases/tag/horizonclient-v1.0) - 2019-04-26

* Initial release
