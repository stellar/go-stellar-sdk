package commands

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/stellar/go-stellar-sdk/tools/stellar-hd-wallet/crypto/derivation"

	"github.com/spf13/cobra"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/tyler-smith/go-bip39"
)

var wordsRegexp = regexp.MustCompile(`^[a-z]+$`)
var count, startID uint32

var allowedNumbers = map[uint32]bool{12: true, 15: true, 18: true, 21: true, 24: true}

var AccountsCmd = &cobra.Command{
	Use:   "accounts",
	Short: "Display accounts for a given mnemonic code",
	Long:  "",
	RunE: func(cmd *cobra.Command, args []string) error {
		printf("How many words? ")
		wordsCount := readUint()
		if _, exist := allowedNumbers[wordsCount]; !exist {
			return errors.New("Invalid value, allowed values: 12, 15, 18, 21, 24")
		}

		words := make([]string, wordsCount)
		for i := uint32(0); i < wordsCount; i++ {
			printf("Enter word #%-4d", i+1)
			words[i] = readString()
			if !wordsRegexp.MatchString(words[i]) {
				println("Invalid word, try again.")
				i--
			}
		}

		printf("Enter password (leave empty if none): ")
		password := readString()

		mnemonic := strings.Join(words, " ")
		println("Mnemonic:", mnemonic)

		seed, err := bip39.NewSeedWithErrorChecking(mnemonic, password)
		if err != nil {
			return errors.New("Invalid words or checksum")
		}

		println("BIP39 Seed:", hex.EncodeToString(seed))

		masterKey, err := derivation.DeriveForPath(derivation.StellarAccountPrefix, seed)
		if err != nil {
			return errors.Wrap(err, "Error deriving master key")
		}

		println("m/44'/148' key:", hex.EncodeToString(masterKey.Key))

		println("")

		for i := uint32(startID); i < startID+count; i++ {
			key, err := masterKey.Derive(derivation.FirstHardenedIndex + i)
			if err != nil {
				return errors.Wrap(err, "Error deriving child key")
			}

			kp, err := keypair.FromRawSeed(key.RawSeed())
			if err != nil {
				return errors.Wrap(err, "Error creating key pair")
			}

			println(fmt.Sprintf(derivation.StellarAccountPathFormat, i), kp.Address(), kp.Seed())
		}

		return nil
	},
}

func init() {
	AccountsCmd.Flags().Uint32VarP(&count, "count", "c", 10, "number of accounts to display")
	AccountsCmd.Flags().Uint32VarP(&startID, "start", "s", 0, "ID of the first wallet to display")
}
