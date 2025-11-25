package ingest

import (
	"context"
	"fmt"
	"io"

	"github.com/stellar/go-stellar-sdk/historyarchive"
	"github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/support/storage"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// Example_ledgerentrieshistoryarchive demonstrates how to stream all ledger
// entries live at specific checkpoint ledger from history archives.
func Example_ledgerentrieshistoryarchive() {
	archiveURL := "http://history.stellar.org/prd/core-live/core_live_001"

	archive, err := historyarchive.Connect(
		archiveURL,
		historyarchive.ArchiveOptions{
			ConnectOptions: storage.ConnectOptions{
				Context: context.TODO(),
			},
		},
	)
	if err != nil {
		panic(err)
	}

	// Ledger must be a checkpoint ledger: (100031+1) mod 64 == 0.
	reader, err := NewCheckpointChangeReader(context.TODO(), archive, 100031)
	if err != nil {
		panic(err)
	}

	var accounts, data, trustlines, offers int
	for {
		entry, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		switch entry.Type {
		case xdr.LedgerEntryTypeAccount:
			accounts++
		case xdr.LedgerEntryTypeData:
			data++
		case xdr.LedgerEntryTypeTrustline:
			trustlines++
		case xdr.LedgerEntryTypeOffer:
			offers++
		default:
			panic("Unknown type")
		}
	}

	fmt.Println("accounts", accounts)
	fmt.Println("data", data)
	fmt.Println("trustlines", trustlines)
	fmt.Println("offers", offers)
}

// Example_changes demonstrates how to stream ledger entry changes
// for a specific ledger using captive stellar-core. Please note that transaction
// meta IS available when using this backend.
func Example_changes() {
	ctx := context.Background()
	archiveURL := "http://history.stellar.org/prd/core-live/core_live_001"
	networkPassphrase := network.PublicNetworkPassphrase

	captiveCoreToml, err := ledgerbackend.NewCaptiveCoreToml(ledgerbackend.CaptiveCoreTomlParams{
		NetworkPassphrase:  networkPassphrase,
		HistoryArchiveURLs: []string{archiveURL},
	})
	if err != nil {
		panic(err)
	}

	// Requires Stellar-Core 13.2.0+
	backend, err := ledgerbackend.NewCaptive(
		ledgerbackend.CaptiveCoreConfig{
			BinaryPath:         "/bin/stellar-core",
			NetworkPassphrase:  networkPassphrase,
			HistoryArchiveURLs: []string{archiveURL},
			Toml:               captiveCoreToml,
		},
	)
	if err != nil {
		panic(err)
	}

	sequence := uint32(3)

	err = backend.PrepareRange(ctx, ledgerbackend.SingleLedgerRange(sequence))
	if err != nil {
		panic(err)
	}

	changeReader, err := NewLedgerChangeReader(ctx, backend, networkPassphrase, sequence)
	if err != nil {
		panic(err)
	}

	for {
		change, err := changeReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		var action string
		switch {
		case change.Pre == nil && change.Post != nil:
			action = "created"
		case change.Pre != nil && change.Post != nil:
			action = "updated"
		case change.Pre != nil && change.Post == nil:
			action = "removed"
		}

		switch change.Type {
		case xdr.LedgerEntryTypeAccount:
			var accountEntry xdr.AccountEntry
			if change.Pre != nil {
				accountEntry = change.Pre.Data.MustAccount()
			} else {
				accountEntry = change.Post.Data.MustAccount()
			}
			fmt.Println("account", accountEntry.AccountId.Address(), action)
		case xdr.LedgerEntryTypeData:
			fmt.Println("data", action)
		case xdr.LedgerEntryTypeTrustline:
			fmt.Println("trustline", action)
		case xdr.LedgerEntryTypeOffer:
			fmt.Println("offer", action)
		default:
			panic("Unknown type")
		}
	}
}
