package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	slog "github.com/stellar/go-stellar-sdk/support/log"
	cmp "github.com/stellar/go-stellar-sdk/tools/horizon-cmp/internal"
)

var (
	count uint32
	from  uint32
	to    uint32
)

func init() {
	historyCmd.Flags().Uint32Var(&count, "count", 0, "number of last ledgers to check (if from/to not set)")
	historyCmd.Flags().Uint32Var(&from, "from", 0, "start of the range")
	historyCmd.Flags().Uint32Var(&to, "to", 0, "end of the range")
}

func runHistoryCmp(cmd *cobra.Command) {
	if count == 0 && from == 0 && to == 0 {
		// Defaults to checking the last 120 ledgers = ~10 minutes.
		count = 120
	}

	if count != 0 && (from != 0 || to != 0) {
		log.Error("--count and --from/--to are mutually exclusive")
		cmd.Help()
		os.Exit(1)
	}

	if count != 0 {
		ledger := getLatestLedger(horizonBase)
		to = uint32(ledger.Sequence)
		from = uint32(ledger.Sequence) - count + 1
	}

	// Check this after all calculations to catch possible underflow
	if from > to || from == 0 || to == 0 {
		log.Error("Invalid --from/--to range")
		cmd.Help()
		os.Exit(1)
	}

	for cur := from; cur <= to; cur++ {
		log.Infof("Getting paths for %d...", cur)
		paths := getAllPathsForLedger(cur)
		checkPaths(paths)
	}
}

func checkPaths(paths []string) {
	for _, path := range paths {
		a := cmp.NewResponse(horizonBase, path, false)
		b := cmp.NewResponse(horizonTest, path, false)

		log = log.WithFields(slog.F{
			"status_code": a.StatusCode,
			"size_base":   a.Size(),
			"size_test":   b.Size(),
		})

		if a.Equal(b) {
			log.Info(path)
		} else {
			log.Error("DIFF " + path)
			os.Exit(1)
		}
	}
}

func getAllPathsForLedger(sequence uint32) []string {
	var paths []string
	ledgerPath := fmt.Sprintf("/ledgers/%d", sequence)
	paths = append(paths, ledgerPath)
	paths = append(paths, getAllPagesPaths(ledgerPath+"/transactions?limit=200")...)
	paths = append(paths, getAllPagesPaths(ledgerPath+"/operations?limit=200")...)
	paths = append(paths, getAllPagesPaths(ledgerPath+"/payments?limit=200")...)
	paths = append(paths, getAllPagesPaths(ledgerPath+"/effects?limit=200")...)
	return paths
}

func getAllPagesPaths(page string) []string {
	pageBody := struct {
		Links struct {
			Self struct {
				Href string `json:"href"`
			} `json:"self"`
			Next struct {
				Href string `json:"href"`
			} `json:"next"`
		} `json:"_links"`
		Embedded struct {
			Records []interface{} `json:"records"`
		} `json:"_embedded"`
	}{}

	var paths []string

	for {
		resp, err := http.Get(horizonBase + page)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}
		err = json.Unmarshal(body, &pageBody)
		if err != nil {
			panic(err)
		}

		// Add current page
		page = pageBody.Links.Self.Href
		page = strings.Replace(page, horizonBase, "", -1)
		paths = append(paths, page)

		// Check next page
		page = pageBody.Links.Next.Href
		page = strings.Replace(page, horizonBase, "", -1)

		// We always add the last empty page (above) to check if there are
		// no extra objects in the Horizon we are testing.
		if len(pageBody.Embedded.Records) == 0 {
			break
		}
	}

	return paths
}
