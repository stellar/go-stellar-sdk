// Package demo is an interactive demonstration of the Go SDK using the Stellar TestNet.
package demo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
	"github.com/stellar/go-stellar-sdk/network"
	hProtocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"

	"github.com/stellar/go-stellar-sdk/keypair"
)

// The account address of the TestNet "friendbot"
const friendbotAddress = "GAIH3ULLFQ4DGSECF2AR555KZ4KNDGEKN4AFI4SU2M7B43MGK3QJZNSR"

// The local file where your generated demo account keys will be stored
// For convenience, the address is also stored so you can look up accounts on the network
const accountsFile = "demo.keys"

// Account represents a Stellar account for this demo.
type Account struct {
	Seed     string             `json:"name"`
	Address  string             `json:"address"`
	HAccount *hProtocol.Account `json:"account"`
	Keypair  *keypair.Full      `json:"keypair"`
	Exists   bool               `json:"exists"`
}

// InitKeys creates n random new keypairs, storing them in a local file. If the file exists,
// InitKeys reads the file instead to construct the keypairs (and ignores n).
func InitKeys(n int) []Account {
	accounts := make([]Account, n)

	fh, err := os.Open(accountsFile)
	if os.IsNotExist(err) {
		// Create the accounts and record them in a file
		log.Info("Accounts file not found - creating new keypairs...")
		for i := 0; i < n; i++ {
			accounts[i] = createKeypair()
		}

		jsonAccounts, err2 := json.MarshalIndent(accounts, "", "  ")
		dieIfError("problem marshaling json accounts", err2)
		err = ioutil.WriteFile(accountsFile, jsonAccounts, 0644)
		dieIfError("problem writing json accounts file", err)
		log.Info("Wrote keypairs to local file ", accountsFile)

		return accounts
	}

	// Read the file and create keypairs
	log.Infof("Found accounts file %s...", accountsFile)
	defer fh.Close()
	bytes, err := ioutil.ReadAll(fh)
	dieIfError("problem converting json file to bytes", err)
	json.Unmarshal(bytes, &accounts)

	// Create the keypair objects
	for i, k := range accounts {
		kp, err := keypair.Parse(k.Seed)
		dieIfError("keypair didn't parse!", err)
		accounts[i].Keypair = kp.(*keypair.Full)
	}

	return accounts
}

// Reset is a command that removes all test accounts created by this demo. All funds are
// transferred back to Friendbot using the account merge operation.
func Reset(client *horizonclient.Client, keys []Account) {
	keys = loadAccounts(client, keys)
	for _, k := range keys {
		if !k.Exists {
			log.Infof("Account %s not found - skipping further operations on it...", k.Address)
			continue
		}

		// It exists - so we will proceed to deconstruct any existing account entries, and then merge it
		// See https://developers.stellar.org/docs/glossary/ledger/#ledger-entries
		log.Info("Found testnet account with ID:", k.HAccount.ID)

		// Find any offers that need deleting...
		offerRequest := horizonclient.OfferRequest{
			ForAccount: k.Address,
			Order:      horizonclient.OrderDesc,
		}
		offers, err := client.Offers(offerRequest)
		dieIfError("error while getting offers", err)
		log.Infof("Account %s has %v offers...", k.Address, len(offers.Embedded.Records))

		// ...and delete them
		for _, o := range offers.Embedded.Records {
			log.Info("    ", o)
			txe, err := deleteOffer(k.HAccount, o.ID, k)
			dieIfError("problem building deleteOffer op", err)
			log.Infof("Deleting offer %d...", o.ID)
			resp := submit(client, txe)
			log.Debug(resp)
		}

		// Find any authorised trustlines on this account...
		log.Infof("Account %s has %d balances...", k.Address, len(k.HAccount.Balances))

		// ...and delete them
		for _, b := range k.HAccount.Balances {
			// Native balances don't have trustlines
			if b.Type == "native" {
				continue
			}
			asset := txnbuild.CreditAsset{
				Code:   b.Code,
				Issuer: b.Issuer,
			}

			// Send the asset back to the issuer...
			log.Infof("Sending %v of surplus asset %s:%s back to issuer...", b.Balance, asset.Code, asset.Issuer)
			txe, err := payment(k.HAccount, asset.Issuer, b.Balance, asset, k)
			dieIfError("problem building payment op", err)
			resp := submit(client, txe)
			log.Debug(resp)

			// Delete the now-empty trustline...
			log.Infof("Deleting trustline for asset %s:%s...", b.Code, b.Issuer)
			ctAsset, err := asset.ToChangeTrustAsset()
			dieIfError("problem building deleteTrustline op", err)
			txe, err = deleteTrustline(k.HAccount, ctAsset, k)
			dieIfError("problem building deleteTrustline op", err)
			resp = submit(client, txe)
			log.Debug(resp)
		}

		// Find any data entries on this account...
		log.Infof("Account %s has %d data entries...", k.Address, len(k.HAccount.Data))
		for dataKey := range k.HAccount.Data {
			decodedV, _ := k.HAccount.GetData(dataKey)
			log.Infof("Deleting data entry '%s' -> '%s'...", dataKey, decodedV)
			txe, err := deleteData(k.HAccount, dataKey, k)
			dieIfError("problem building manageData op", err)
			resp := submit(client, txe)
			log.Debug(resp)
		}
	}

	// Finally, the accounts may be merged
	for _, k := range keys {
		if !k.Exists {
			continue
		}
		log.Infof("Merging account %s back to friendbot (%s)...", k.Address, friendbotAddress)
		txe, err := mergeAccount(k.HAccount, friendbotAddress, k)
		dieIfError("problem building mergeAccount op", err)
		resp := submit(client, txe)
		log.Debug(resp)
	}
}

// Initialise is a command that funds an initial set of accounts for use with other demo operations.
// The first account is funded from Friendbot; subseqeuent accounts are created and funded from this
// first account.
func Initialise(client *horizonclient.Client, keys []Account) {
	// Fund the first account from friendbot
	log.Infof("Funding account %s from friendbot...", keys[0].Address)
	_, err := client.Fund(keys[0].Address)
	dieIfError(fmt.Sprintf("couldn't fund account %s from friendbot", keys[0].Address), err)
	keys[0].HAccount = loadAccount(client, keys[0].Address)
	keys[0].Exists = true

	// Fund the others using the create account operation
	for i := 1; i < len(keys); i++ {
		log.Infof("Funding account %s from account %s...", keys[i].Address, keys[0].Address)
		txe, err := createAccount(keys[0].HAccount, keys[i].Address, keys[0])
		dieIfError("problem building createAccount op", err)
		resp := submit(client, txe)
		log.Debug(resp)
	}
}

// TXError is a command that deliberately creates a bad transaction to trigger an error response
// from Horizon. This code demonstrates how to retrieve and inspect the error.
func TXError(client *horizonclient.Client, keys []Account) {
	keys = loadAccounts(client, keys)
	// Create a bump seq operation
	// Set the seq number to -1 (invalid)
	// Create the transaction
	txe, err := bumpSequence(keys[0].HAccount, -1, keys[0])
	dieIfError("problem building createAccount op", err)

	// Submit
	resp := submit(client, txe)

	// Inspect and print error
	log.Info(resp)
}

/***** Examples of operation building follow *****/

func bumpSequence(source *hProtocol.Account, seqNum int64, signer Account) (string, error) {
	bumpSequenceOp := txnbuild.BumpSequence{
		BumpTo: seqNum,
	}

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        source,
			IncrementSequenceNum: true,
			Operations:           []txnbuild.Operation{&bumpSequenceOp},
			BaseFee:              txnbuild.MinBaseFee,
			Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
		},
	)
	if err != nil {
		return "", errors.Wrap(err, "couldn't build transaction")
	}
	tx, err = tx.Sign(network.TestNetworkPassphrase, signer.Keypair)
	if err != nil {
		return "", errors.Wrap(err, "couldn't sign transaction")
	}

	txeBase64, err := tx.Base64()
	if err != nil {
		return txeBase64, errors.Wrap(err, "couldn't serialise transaction")
	}
	return txeBase64, nil
}

func createAccount(source *hProtocol.Account, dest string, signer Account) (string, error) {
	createAccountOp := txnbuild.CreateAccount{
		Destination: dest,
		Amount:      "100",
	}

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        source,
			IncrementSequenceNum: true,
			Operations:           []txnbuild.Operation{&createAccountOp},
			BaseFee:              txnbuild.MinBaseFee,
			Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
		},
	)
	if err != nil {
		return "", errors.Wrap(err, "couldn't build transaction")
	}
	tx, err = tx.Sign(network.TestNetworkPassphrase, signer.Keypair)
	if err != nil {
		return "", errors.Wrap(err, "couldn't sign transaction")
	}

	txeBase64, err := tx.Base64()
	if err != nil {
		return txeBase64, errors.Wrap(err, "couldn't serialise transaction")
	}
	return txeBase64, nil
}

func deleteData(source *hProtocol.Account, dataKey string, signer Account) (string, error) {
	manageDataOp := txnbuild.ManageData{
		Name: dataKey,
	}

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        source,
			IncrementSequenceNum: true,
			Operations:           []txnbuild.Operation{&manageDataOp},
			BaseFee:              txnbuild.MinBaseFee,
			Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
		},
	)
	if err != nil {
		return "", errors.Wrap(err, "couldn't build transaction")
	}
	tx, err = tx.Sign(network.TestNetworkPassphrase, signer.Keypair)
	if err != nil {
		return "", errors.Wrap(err, "couldn't sign transaction")
	}

	txeBase64, err := tx.Base64()
	if err != nil {
		return txeBase64, errors.Wrap(err, "couldn't serialise transaction")
	}
	return txeBase64, nil
}

func payment(source *hProtocol.Account, dest, amount string, asset txnbuild.Asset, signer Account) (string, error) {
	paymentOp := txnbuild.Payment{
		Destination: dest,
		Amount:      amount,
		Asset:       asset,
	}

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        source,
			IncrementSequenceNum: true,
			Operations:           []txnbuild.Operation{&paymentOp},
			BaseFee:              txnbuild.MinBaseFee,
			Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
		},
	)
	if err != nil {
		return "", errors.Wrap(err, "couldn't build transaction")
	}
	tx, err = tx.Sign(network.TestNetworkPassphrase, signer.Keypair)
	if err != nil {
		return "", errors.Wrap(err, "couldn't sign transaction")
	}

	txeBase64, err := tx.Base64()
	if err != nil {
		return txeBase64, errors.Wrap(err, "couldn't serialise transaction")
	}
	return txeBase64, nil
}

func deleteTrustline(source *hProtocol.Account, asset txnbuild.ChangeTrustAsset, signer Account) (string, error) {
	deleteTrustline := txnbuild.RemoveTrustlineOp(asset)

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        source,
			IncrementSequenceNum: true,
			Operations:           []txnbuild.Operation{&deleteTrustline},
			BaseFee:              txnbuild.MinBaseFee,
			Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
		},
	)
	if err != nil {
		return "", errors.Wrap(err, "couldn't build transaction")
	}
	tx, err = tx.Sign(network.TestNetworkPassphrase, signer.Keypair)
	if err != nil {
		return "", errors.Wrap(err, "couldn't sign transaction")
	}

	txeBase64, err := tx.Base64()
	if err != nil {
		return txeBase64, errors.Wrap(err, "couldn't serialise transaction")
	}
	return txeBase64, nil
}

func deleteOffer(source *hProtocol.Account, offerID int64, signer Account) (string, error) {
	deleteOffer, err := txnbuild.DeleteOfferOp(offerID)
	if err != nil {
		return "", errors.Wrap(err, "building offer")
	}

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        source,
			IncrementSequenceNum: true,
			Operations:           []txnbuild.Operation{&deleteOffer},
			BaseFee:              txnbuild.MinBaseFee,
			Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
		},
	)
	if err != nil {
		return "", errors.Wrap(err, "couldn't build transaction")
	}
	tx, err = tx.Sign(network.TestNetworkPassphrase, signer.Keypair)
	if err != nil {
		return "", errors.Wrap(err, "couldn't sign transaction")
	}

	txeBase64, err := tx.Base64()
	if err != nil {
		return txeBase64, errors.Wrap(err, "couldn't serialise transaction")
	}
	return txeBase64, nil
}

func mergeAccount(source *hProtocol.Account, destAddress string, signer Account) (string, error) {
	accountMerge := txnbuild.AccountMerge{
		Destination: destAddress,
	}

	tx, err := txnbuild.NewTransaction(
		txnbuild.TransactionParams{
			SourceAccount:        source,
			IncrementSequenceNum: true,
			Operations:           []txnbuild.Operation{&accountMerge},
			BaseFee:              txnbuild.MinBaseFee,
			Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
		},
	)
	if err != nil {
		return "", errors.Wrap(err, "couldn't build transaction")
	}
	tx, err = tx.Sign(network.TestNetworkPassphrase, signer.Keypair)
	if err != nil {
		return "", errors.Wrap(err, "couldn't sign transaction")
	}

	txeBase64, err := tx.Base64()
	if err != nil {
		return txeBase64, errors.Wrap(err, "couldn't serialise transaction")
	}
	return txeBase64, nil
}

// createKeypair constructs a new random keypair, and returns it in a DemoAccount.
func createKeypair() Account {
	pair, err := keypair.Random()
	if err != nil {
		log.Fatal(err)
	}
	log.Info("Seed:", pair.Seed())
	log.Info("Address:", pair.Address())

	return Account{
		Seed:    pair.Seed(),
		Address: pair.Address(),
		Keypair: pair,
	}
}

// loadAccounts looks up each account in the provided list and stores the returned information.
func loadAccounts(client *horizonclient.Client, accounts []Account) []Account {
	for i, a := range accounts {
		accounts[i].HAccount = loadAccount(client, a.Address)
		accounts[i].Exists = true
	}

	return accounts
}

// loadAccount is an example of how to get an account's details from Horizon.
func loadAccount(client *horizonclient.Client, address string) *hProtocol.Account {
	accountRequest := horizonclient.AccountRequest{AccountID: address}
	horizonSourceAccount, err := client.AccountDetail(accountRequest)
	if err != nil {
		dieIfError(fmt.Sprintf("couldn't get account detail for %s", address), err)
	}

	return &horizonSourceAccount
}

func submit(client *horizonclient.Client, txeBase64 string) (resp hProtocol.Transaction) {
	resp, err := client.SubmitTransactionXDR(txeBase64)
	if err != nil {
		hError := err.(*horizonclient.Error)
		err = printHorizonError(hError)
		dieIfError("couldn't print Horizon eror", err)
		os.Exit(1)
	}

	return
}

func dieIfError(desc string, err error) {
	if err != nil {
		log.Fatalf("Fatal error (%s): %s", desc, err)
	}
}

// printHorizonError is an example of how to inspect the error returned from Horizon.
func printHorizonError(hError *horizonclient.Error) error {
	problem := hError.Problem
	log.Println("Error type:", problem.Type)
	log.Println("Error title:", problem.Title)
	log.Println("Error status:", problem.Status)
	log.Println("Error detail:", problem.Detail)

	resultCodes, err := hError.ResultCodes()
	if err != nil {
		return errors.Wrap(err, "Couldn't read ResultCodes")
	}
	log.Println("TransactionCode:", resultCodes.TransactionCode)
	log.Println("OperationCodes:")
	for _, code := range resultCodes.OperationCodes {
		log.Println("    ", code)
	}

	resultString, err := hError.ResultString()
	if err != nil {
		return errors.Wrap(err, "Couldn't read ResultString")
	}
	log.Println("TransactionResult XDR (base 64):", resultString)

	envelope, err := hError.Envelope()
	if err != nil {
		return errors.Wrap(err, "Couldn't read Envelope")
	}

	aid := envelope.SourceAccount().MustEd25519()
	decodedAID, err := strkey.Encode(strkey.VersionByteAccountID, aid[:])
	if err != nil {
		log.Println("Couldn't decode account ID:", err)
	} else {
		log.Printf("SourceAccount (%s): %s", envelope.SourceAccount().Type, decodedAID)
	}
	log.Println("Fee:", envelope.Fee())
	log.Println("SequenceNumber:", envelope.SeqNum())
	log.Println("TimeBounds:", envelope.TimeBounds())
	log.Println("Memo:", envelope.Memo())
	log.Println("Memo.Type:", envelope.Memo().Type)
	if envelope.Memo().Type != xdr.MemoTypeMemoNone {
		log.Println("Memo.Text:", envelope.Memo().Text)
		log.Println("Memo.Id:", envelope.Memo().Id)
		log.Println("Memo.Hash:", envelope.Memo().Hash)
		log.Println("Memo.RetHash:", envelope.Memo().RetHash)
	}
	log.Println("Operations:", envelope.Operations())

	for _, op := range envelope.Operations() {
		log.Println("Operations.SourceAccount:", op.SourceAccount)
		log.Println("Operations.Body.Type:", op.Body.Type)
	}
	// TODO is Ext a useful field which we should print?
	// log.Println("Ext:", txe.Ext)

	return nil
}
