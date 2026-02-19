//go:build xdr_transaction_meta_v5

package xdr

func transactionMetaContractEventsForOperation(t *TransactionMeta, opIndex uint32) ([]ContractEvent, error, bool) {
	switch t.V {
	case 5:
		txMeta := t.MustV5()
		if len(txMeta.Operations) == 0 {
			return nil, nil, true
		}
		return txMeta.Operations[opIndex].Events, nil, true
	default:
		return nil, nil, false
	}
}

func transactionMetaDiagnosticEvents(t *TransactionMeta) ([]DiagnosticEvent, error, bool) {
	switch t.V {
	case 5:
		return t.MustV5().DiagnosticEvents, nil, true
	default:
		return nil, nil, false
	}
}

func transactionMetaTransactionEvents(t *TransactionMeta) ([]TransactionEvent, error, bool) {
	switch t.V {
	case 5:
		return t.MustV5().Events, nil, true
	default:
		return nil, nil, false
	}
}
