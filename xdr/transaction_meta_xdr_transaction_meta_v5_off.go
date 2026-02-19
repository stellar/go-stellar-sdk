//go:build !xdr_transaction_meta_v5

package xdr

func transactionMetaContractEventsForOperation(_ *TransactionMeta, _ uint32) ([]ContractEvent, error, bool) {
	return nil, nil, false
}

func transactionMetaDiagnosticEvents(_ *TransactionMeta) ([]DiagnosticEvent, error, bool) {
	return nil, nil, false
}

func transactionMetaTransactionEvents(_ *TransactionMeta) ([]TransactionEvent, error, bool) {
	return nil, nil, false
}
