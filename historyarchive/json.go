// Copyright 2016 Stellar Development Foundation and contributors. Licensed
// under the Apache License, Version 2.0. See the COPYING file at the root
// of this distribution or at http://www.apache.org/licenses/LICENSE-2.0

package historyarchive

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func DumpXdrAsJson(args []string) error {
	var tmp xdr.DecoderFrom
	var rdr io.ReadCloser
	var err error

	for _, arg := range args {
		rdr, err = os.Open(arg)
		if err != nil {
			return err
		}

		if strings.HasSuffix(arg, ".gz") {
			rdr, err = gzip.NewReader(rdr)
			if err != nil {
				return err
			}
		}

		base := path.Base(arg)
		xr := xdr.NewStream(rdr)
		n := 0
		for {
			var lhe xdr.LedgerHeaderHistoryEntry
			var the xdr.TransactionHistoryEntry
			var thre xdr.TransactionHistoryResultEntry
			var bke xdr.BucketEntry
			var scp xdr.ScpHistoryEntry

			if strings.HasPrefix(base, "bucket") {
				tmp = &bke
			} else if strings.HasPrefix(base, "ledger") {
				tmp = &lhe
			} else if strings.HasPrefix(base, "transactions") {
				tmp = &the
			} else if strings.HasPrefix(base, "results") {
				tmp = &thre
			} else if strings.HasPrefix(base, "scp") {
				tmp = &scp
			} else {
				return fmt.Errorf("Error: unrecognized XDR file type %s", base)
			}

			if err = xr.ReadOne(tmp); err != nil {
				if err == io.EOF {
					break
				} else {
					return fmt.Errorf("Error on XDR record %d of %s",
						n, arg)
				}
			}
			n++
			buf, err := json.MarshalIndent(tmp, "", "    ")
			if err != nil {
				return err
			}
			os.Stdout.Write(buf)
		}
		xr.Close()
	}
	return nil
}
