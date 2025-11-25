// stellar-sign is a small interactive utility to help you contribute a
// signature to a transaction envelope or verify a transaction.
//
// It prompts you for a key, public (verify) or private (sign)
package main

import (
	"bufio"
	b64 "encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"

	"github.com/howeyc/gopass"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"
)

type SignOrVerify struct {
	verify            bool
	networkPassphrase string
	privKey           *keypair.Full
	pubKey            *keypair.FromAddress
}

func (router *SignOrVerify) setKey(input string) {
	var err error
	if router.verify {
		var inputPubKey keypair.KP
		inputPubKey, err = keypair.Parse(input)
		if err != nil {
			log.Fatal(err)
		}
		router.pubKey = inputPubKey.FromAddress()
	} else {
		router.privKey, err = keypair.ParseFull(input)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func (router *SignOrVerify) doVerify(gentx *txnbuild.GenericTransaction) error {
	var err error
	var gentxHash [32]byte
	var gentxEnv xdr.TransactionEnvelope
	gentxEnv, err = gentx.ToXDR()
	if err != nil {
		log.Fatal(err)
	}
	signatures := gentxEnv.Signatures()
	gentxHash, err = gentx.Hash(router.networkPassphrase)
	if err != nil {
		log.Fatal(err)
	}
	if gentxEnv.IsFeeBump() {
		signatures = gentxEnv.FeeBumpSignatures()
		gentxFeeBump, ok := gentx.FeeBump()
		if !ok {
			log.Fatal("Cannot find the correct transaction type")
		}
		gentxHash, err = gentxFeeBump.Hash(router.networkPassphrase)
		if err != nil {
			log.Fatal(err)
		}
	}
	if len(signatures) == 0 {
		log.Fatal("The transaction does not contain any signature")
	}
	// We try all signature and stop if we find a correct one
	for _, signature := range signatures {
		err = router.pubKey.Verify(gentxHash[:], signature.Signature)
		if err == nil {
			break
		}
	}
	return err
}

func (router *SignOrVerify) doSign(gentx *txnbuild.GenericTransaction) string {
	var newEnv string
	var err error
	if tx, ok := gentx.Transaction(); ok {
		tx, err = tx.Sign(router.networkPassphrase, router.privKey)
		if err != nil {
			log.Fatal(err)
		}
		newEnv, err = tx.Base64()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		var txFeeBump *txnbuild.FeeBumpTransaction
		txFeeBump, ok = gentx.FeeBump()
		if !ok {
			log.Fatal("Cannot find the correct transaction type")
		}
		txFeeBump, err = txFeeBump.Sign(router.networkPassphrase, router.privKey)
		if err != nil {
			log.Fatal(err)
		}
		newEnv, err = txFeeBump.Base64()
		if err != nil {
			log.Fatal(err)
		}
	}
	return newEnv
}

var in *bufio.Reader

var infile = flag.String("infile", "", "transaction envelope")
var verify = flag.Bool("verify", false, "Verify the transaction instead of signing")
var testnet = flag.Bool("testnet", false, "Sign or verify the transaction using Testnet passphrase instead of Public")

func main() {
	flag.Parse()
	in = bufio.NewReader(os.Stdin)

	var (
		env string
		err error
	)

	if *infile == "" {
		// read envelope
		env, err = readLine("Enter envelope (base64): ", false)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		var file *os.File
		file, err = os.Open(*infile)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()

		var raw []byte
		raw, err = ioutil.ReadAll(file)
		if err != nil {
			log.Fatal(err)
		}

		env = string(raw)
	}

	// parse the envelope
	var txe xdr.TransactionEnvelope
	err = xdr.SafeUnmarshalBase64(env, &txe)
	if err != nil {
		log.Fatal(err)
	}

	isFeeBump := txe.IsFeeBump()

	fmt.Println("")
	fmt.Println("Transaction Summary:")
	sourceAccount := txe.SourceAccount().ToAccountId()
	fmt.Printf("  type: %s\n", txe.Type.String())
	if isFeeBump {
		fmt.Printf("  fee bump source: %s\n", txe.FeeBumpAccount().ToAccountId().Address())
	}
	fmt.Printf("  source: %s\n", sourceAccount.Address())
	fmt.Printf("  ops: %d\n", len(txe.Operations()))
	fmt.Printf("  sigs: %d\n", len(txe.Signatures()))
	for _, signature := range txe.Signatures() {
		fmt.Printf("    %s\n", b64.StdEncoding.EncodeToString(signature.Signature))
	}
	if isFeeBump {
		fmt.Printf("  fee bump sigs: %d\n", len(txe.FeeBumpSignatures()))
		for _, feeBumpSignature := range txe.FeeBumpSignatures() {
			fmt.Printf("    %s\n", b64.StdEncoding.EncodeToString(feeBumpSignature.Signature))
		}
	}
	fmt.Println("")

	// TODO: add operation details

	passPhrase := network.PublicNetworkPassphrase
	if *testnet {
		passPhrase = network.TestNetworkPassphrase
	}

	// read seed/public key
	key, err := readLine("Enter key", true)
	if err != nil {
		log.Fatal(err)
	}

	flowRouter := &SignOrVerify{verify: *verify, networkPassphrase: passPhrase}
	flowRouter.setKey(key)

	parsed, err := txnbuild.TransactionFromXDR(env)
	if err != nil {
		log.Fatal(err)
	}

	if *verify {
		err := flowRouter.doVerify(parsed)
		if err != nil {
			fmt.Print("\nSignature is INVALID\n")
		} else {
			fmt.Print("\nSignature is VALID\n")
		}
	} else {
		newEnv := flowRouter.doSign(parsed)
		fmt.Print("\n==== Result ====\n\n")
		fmt.Print("```\n")
		fmt.Println(newEnv)
		fmt.Print("```\n")
	}

}

func readLine(prompt string, private bool) (string, error) {
	fmt.Println(prompt)
	var line string
	var err error

	if private {
		var str []byte
		str, err = gopass.GetPasswdMasked()
		if err != nil {
			return "", err
		}
		line = string(str)
	} else {
		line, err = in.ReadString('\n')
		if err != nil {
			return "", err
		}
	}
	return strings.Trim(line, "\n"), nil
}
