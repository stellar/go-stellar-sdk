//go:build !xdr_transaction_meta_v5

package ingest

import "github.com/stellar/go-stellar-sdk/xdr"

func (t *LedgerTransaction) getChangesForXdrTransactionMetaV5() ([]Change, bool) {
	return nil, false
}

func (t *LedgerTransaction) getOperationChangesMetaForXdrTransactionMetaV5() (operationsMeta, error, bool) {
	return nil, nil, false
}

func (t *LedgerTransaction) getTransactionEventsForXdrTransactionMetaV5() (TransactionEvents, error, bool) {
	return TransactionEvents{}, nil, false
}

func (t *LedgerTransaction) sorobanResourceFeeRefundTxChangesAfterForXdrTransactionMetaV5() (xdr.LedgerEntryChanges, error, bool) {
	return nil, nil, false
}
