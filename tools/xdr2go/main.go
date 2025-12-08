package main

import (
	"fmt"
	"go/format"

	"github.com/spf13/cobra"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

var (
	typ string
)

var rootCmd = &cobra.Command{
	Use:   "xdr2go [base64-encoded XDR object]",
	Short: "xdr2go transforms base64 encoded XDR objects into a pretty Go code",
	RunE:  run,
}

func main() {
	rootCmd.Flags().StringVarP(&typ, "type", "t", "TransactionEnvelope", "xdr type, currently only TransactionEnvelope is available")
	rootCmd.Execute()
}

func run(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errors.New("Exactly one command argument with XDR object is required.")
	}
	var object interface{}
	switch typ {
	case "TransactionEnvelope":
		object = &xdr.TransactionEnvelope{}
	default:
		return errors.New("Unknown type.")
	}
	err := xdr.SafeUnmarshalBase64(args[0], object)
	if err != nil {
		return errors.Wrap(err, "Error unmarshaling XDR stucture.")
	}

	source := fmt.Sprintf("%#v\n", object)
	formatted, err := format.Source([]byte(source))
	if err != nil {
		return errors.Wrap(err, "Error formatting code.")
	}
	fmt.Println(string(formatted))
	return nil
}
