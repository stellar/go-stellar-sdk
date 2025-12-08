package cmd

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
	demo "github.com/stellar/go-stellar-sdk/txnbuild/cmd/demo/operations"
)

// resetCmd represents the reset command
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset the state of all demo accounts on the TestNet",
	Long: `Run this command before trying other commands in order to have a clean slate
for testing.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Info("Resetting TestNet state...")
		keys := demo.InitKeys(4)
		client := horizonclient.DefaultTestNetClient

		demo.Reset(client, keys)
		fmt.Println("Reset complete.")
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// resetCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// resetCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
