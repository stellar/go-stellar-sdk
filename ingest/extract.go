package ingest

import (
	"github.com/stellar/go-stellar-sdk/xdr"
)

// LedgerTransactionEvents is one transaction's contract events plus its hash;
// see xdr.LedgerTransactionEvents (the extraction lives in package xdr since
// the two-tier visitor redesign; the alias keeps the historical ingest name).
type LedgerTransactionEvents = xdr.LedgerTransactionEvents

// ExtractTxHashes returns every transaction hash of the ledger in apply
// (TxProcessing) order — a thin delegation to xdr.ExtractTxHashes.
//
// Experimental: the view-based extractors are new in this release and their
// signatures may still change.
func ExtractTxHashes(lcm xdr.LedgerCloseMetaView) ([]xdr.Hash, error) {
	return xdr.ExtractTxHashes(lcm)
}

// ExtractLedgerEvents returns the contract events of every transaction in
// the ledger, in apply order, each paired with its transaction hash — a thin
// delegation to xdr.ExtractLedgerEvents (one generated Walk over the buffer).
//
// Experimental: the view-based extractors are new in this release and their
// signatures may still change.
func ExtractLedgerEvents(lcm xdr.LedgerCloseMetaView) ([]LedgerTransactionEvents, error) {
	return xdr.ExtractLedgerEvents(lcm)
}
