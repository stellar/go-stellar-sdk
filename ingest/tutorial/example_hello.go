package main

import (
	"context"
	"fmt"

	backends "github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
)

func helloworld() {
	ctx := context.Background()
	backend, err := backends.NewCaptive(config)
	panicIf(err)
	defer backend.Close()

	// Prepare a single ledger to be ingested,
	err = backend.PrepareRange(ctx, backends.BoundedRange(123456, 123456))
	panicIf(err)

	// then retrieve it:
	ledger, err := backend.GetLedger(ctx, 123456)
	panicIf(err)

	// Now `ledger` is a raw `xdr.LedgerCloseMeta` object containing the
	// transactions contained within this ledger.
	fmt.Printf("\nHello, Sequence %d.\n", ledger.LedgerSequence())
}
