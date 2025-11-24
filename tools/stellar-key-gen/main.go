package main

import (
	"html/template"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/stellar/go-stellar-sdk/keypair"
)

func main() {
	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	cmd := &cobra.Command{
		Use:   "stellar-key-gen",
		Short: "Generate a Stellar key.",
	}
	cmd.SetArgs(args)
	cmd.SetOutput(stderr)

	outFormat := "{{.PublicKey}}\n{{.SecretKey}}\n"
	cmd.Flags().StringVarP(&outFormat, "format", "f", outFormat, "Format of output")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		tmpl, err := template.New("").Parse(outFormat)
		if err != nil {
			return err
		}

		key, err := keypair.Random()
		if err != nil {
			return err
		}

		data := outData{
			PublicKey: key.Address(),
			SecretKey: key.Seed(),
		}

		err = tmpl.Execute(stdout, data)
		if err != nil {
			return err
		}

		return nil
	}

	err := cmd.Execute()
	if err != nil {
		return 1
	}
	return 0
}

type outData struct {
	PublicKey string
	SecretKey string
}
