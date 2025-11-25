package commands

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/tyler-smith/go-bip39"
)

const DefaultEntropySize = 256

var NewCmd = &cobra.Command{
	Use:   "new",
	Short: "Generates a new mnemonic code",
	Long:  "",
	RunE: func(cmd *cobra.Command, args []string) error {
		entropy, err := bip39.NewEntropy(DefaultEntropySize)
		if err != nil {
			return errors.Wrap(err, "Error generating entropy")
		}

		mnemonic, err := bip39.NewMnemonic(entropy)
		if err != nil {
			return errors.Wrap(err, "Error generating mnemonic code")
		}

		words := strings.Split(mnemonic, " ")
		for i := 0; i < len(words); i++ {
			printf("word %02d/24: %10s", i+1, words[i])
			readString()
		}

		println("WARNING! Store the words above in a safe place!")
		println("WARNING! If you lose your words, you will lose access to funds in all derived accounts!")
		println("WARNING! Anyone who has access to these words can spend your funds!")
		println("")
		println("Use: `stellar-hd-wallet accounts` command to see generated accounts.")

		return nil
	},
}
