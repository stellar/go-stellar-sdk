//go:build xdr_transaction_meta_v5

package token_transfer

import (
	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func unifiedEventsStreamOperationsForXdrTransactionMetaV5(tx ingest.LedgerTransaction) ([]xdr.OperationMetaV2, error, bool) {
	switch tx.UnsafeMeta.V {
	case 5:
		return tx.UnsafeMeta.MustV5().Operations, nil, true
	default:
		return nil, nil, false
	}
}

func isValidUnifiedEventsStreamVersionForXdrTransactionMetaV5(v int32) bool {
	return v == 5
}

func parseCustomTokenEventForXdrTransactionMetaV5(fn string, tx ingest.LedgerTransaction, opIndex *uint32, contractEvent xdr.ContractEvent, txMetaVersion int32) (*TokenTransferEvent, error, bool) {
	switch txMetaVersion {
	case 5:
		event, err := parseCustomTokenEventV4(fn, tx, opIndex, contractEvent)
		return event, err, true
	default:
		return nil, nil, false
	}
}
