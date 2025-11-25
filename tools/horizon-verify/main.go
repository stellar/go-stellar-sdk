package main

import (
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/spf13/cobra"
	"github.com/stellar/go-stellar-sdk/clients/horizonclient"
	protocol "github.com/stellar/go-stellar-sdk/protocols/horizon"
	"github.com/stellar/go-stellar-sdk/xdr"
)

var horizonURL string
var startSequence uint32
var count uint

func init() {
	rootCmd.PersistentFlags().StringVarP(&horizonURL, "url", "u", "", "Horizon server URL")
	rootCmd.PersistentFlags().Uint32VarP(&startSequence, "start", "s", 0, "Sequence number of the ledger to start with (follows descending order, defaults to the latest ledger)")
	rootCmd.PersistentFlags().UintVarP(&count, "count", "c", 10000, "Number of ledgers to check")
}

var rootCmd = &cobra.Command{
	Use:   "horizon-verify",
	Short: "tool to check horizon data consistency",
	Run: func(cmd *cobra.Command, args []string) {
		if horizonURL == "" {
			cmd.Help()
			return
		}

		client := horizonclient.Client{
			HorizonURL: horizonURL,
			HTTP:       http.DefaultClient,
		}

		ledgerCursor := ""

		if startSequence != 0 {
			startSequence++

			ledger, err := client.LedgerDetail(startSequence)
			if err != nil {
				panic(err)
			}

			ledgerCursor = ledger.PagingToken()
		}

		fmt.Printf("%s: Checking %d ledgers starting from cursor \"%s\"\n\n", horizonURL, count, ledgerCursor)

		for {
			ledgersPage, err := client.Ledgers(horizonclient.LedgerRequest{
				Limit:  200,
				Order:  horizonclient.OrderDesc,
				Cursor: ledgerCursor,
			})

			if err != nil {
				panic(err)
			}

			if len(ledgersPage.Embedded.Records) == 0 {
				fmt.Println("Done")
				return
			}

			for _, ledger := range ledgersPage.Embedded.Records {
				fmt.Printf("Checking ledger: %d (successful=%d failed=%d)\n", ledger.Sequence, ledger.SuccessfulTransactionCount, *ledger.FailedTransactionCount)

				ledgerCursor = ledger.PagingToken()

				transactionsPage, err := client.Transactions(horizonclient.TransactionRequest{
					ForLedger:     uint(ledger.Sequence),
					Limit:         200,
					IncludeFailed: true,
				})

				if err != nil {
					panic(err)
				}

				var (
					wg                 sync.WaitGroup
					successful, failed int32
				)

				for _, transaction := range transactionsPage.Embedded.Records {
					wg.Add(1)

					if transaction.Successful {
						successful++
					} else {
						failed++
					}

					go func(transaction protocol.Transaction) {
						defer wg.Done()

						var resultXDR xdr.TransactionResult
						err = xdr.SafeUnmarshalBase64(transaction.ResultXdr, &resultXDR)
						if err != nil {
							panic(err)
						}

						if (transaction.Successful && !resultXDR.Successful()) ||
							(!transaction.Successful && resultXDR.Successful()) {
							panic(fmt.Sprintf("Corrupted data! %s %s", transaction.Hash, transaction.ResultXdr))
						}

						operationsPage, err := client.Operations(horizonclient.OperationRequest{
							ForTransaction: transaction.Hash,
							Limit:          200,
						})

						if err != nil {
							panic(err)
						}

						if len(operationsPage.Embedded.Records) != int(transaction.OperationCount) {
							panic(fmt.Sprintf("Corrupted data! %s operations count %d vs %d", transaction.Hash, len(operationsPage.Embedded.Records), transaction.OperationCount))
						}
					}(transaction)
				}

				wg.Wait()

				if successful != ledger.SuccessfulTransactionCount || failed != *ledger.FailedTransactionCount {
					panic(fmt.Sprintf("Invalid ledger counters %d", ledger.Sequence))
				}

				count--
				if count == 0 {
					fmt.Println("Done")
					return
				}
			}
		}
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// func getBody(url string) []byte {
// 	resp, err := http.Get(url)
// 	if err != nil {
// 		panic(err)
// 	}

// 	if resp.StatusCode != http.StatusOK {
// 		panic(fmt.Sprintf("%d response for %s", resp.StatusCode, url))
// 	}

// 	body, err := ioutil.ReadAll(resp.Body)
// 	if err != nil {
// 		panic(err)
// 	}

// 	return body
// }
