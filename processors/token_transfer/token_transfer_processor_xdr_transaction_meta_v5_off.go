//go:build !xdr_transaction_meta_v5

package token_transfer

import (
	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func unifiedEventsStreamOperationsForXdrTransactionMetaV5(_ ingest.LedgerTransaction) ([]xdr.OperationMetaV2, error, bool) {
	return nil, nil, false
}

func isValidUnifiedEventsStreamVersionForXdrTransactionMetaV5(_ int32) bool {
	return false
}

func parseCustomTokenEventForXdrTransactionMetaV5(_ string, _ ingest.LedgerTransaction, _ *uint32, _ xdr.ContractEvent, _ int32) (*TokenTransferEvent, error, bool) {
	return nil, nil, false
}
