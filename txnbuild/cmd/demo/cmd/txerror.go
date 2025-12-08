package cmd

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
	demo "github.com/stellar/go-stellar-sdk/txnbuild/cmd/demo/operations"
)

// txerrorCmd represents the txerror command
var txerrorCmd = &cobra.Command{
	Use:   "txerror",
	Short: "Submit a purposefully invalid transaction",
	Long:  `This command submits an invalid transaction, in order to demonstrate a Horizon error return.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("Demonstrating a bad transaction response...")
		keys := demo.InitKeys(4)
		client := horizonclient.DefaultTestNetClient

		demo.TXError(client, keys)
		fmt.Println("Transaction complete.")
	},
}

func init() {
	rootCmd.AddCommand(txerrorCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// txerrorCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// txerrorCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
