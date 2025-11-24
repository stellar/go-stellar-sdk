package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/stellar/go-stellar-sdk/keypair"
)

var prefix string

const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

func main() {

	if len(os.Args) != 2 {
		usage()
		os.Exit(1)
	}

	prefix = strings.ToUpper(os.Args[1])
	checkPlausible()

	for {
		kp, err := keypair.Random()

		if err != nil {
			log.Fatal(err)
		}

		// NOTE: the first letter of an address will always be G, and the second letter will be one of only a few
		// possibilities in the base32 alphabet, so we are actually searching for the vanity value after this 2
		// character prefix.
		if strings.HasPrefix(kp.Address()[2:], prefix) {
			fmt.Println("Found!")
			fmt.Printf("Secret seed: %s\n", kp.Seed())
			fmt.Printf("Public: %s\n", kp.Address())
			os.Exit(0)
		}
	}
}

func usage() {
	fmt.Printf("Usage:\n\tstellar-vanity-gen PREFIX\n")
}

// aborts the attempt if a desired character is not a valid base32 digit
func checkPlausible() {
	for _, r := range prefix {
		if !strings.ContainsRune(alphabet, r) {
			fmt.Printf("Invalid prefix: %s is not in the base32 alphabet\n", strconv.QuoteRune(r))
			os.Exit(1)
		}
	}
}
