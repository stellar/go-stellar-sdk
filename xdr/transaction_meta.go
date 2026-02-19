package xdr

import "fmt"

func (t *TransactionMeta) GetContractEventsForOperation(opIndex uint32) ([]ContractEvent, error) {
	switch t.V {
	case 1, 2:
		return nil, nil
	// For TxMetaV3, events appear in the TxMetaV3.SorobanMeta.Events, and we dont need to rely on opIndex
	case 3:
		sorobanMeta := t.MustV3().SorobanMeta
		if sorobanMeta == nil {
			return nil, nil
		}
		return sorobanMeta.Events, nil
	case 4:
		txMeta := t.MustV4()
		// For a failed transaction, txMeta.Operations slice will be nil.
		// This length check helps prevent an index out of range error and you dont have to explicitly check if it's a failed tx
		if len(txMeta.Operations) == 0 {
			return nil, nil
		}
		return txMeta.Operations[opIndex].Events, nil
	default:
		if events, err, ok := transactionMetaContractEventsForOperation(t, opIndex); ok {
			return events, err
		}
		return nil, fmt.Errorf("unsupported TransactionMeta version: %v", t.V)
	}
}

// GetDiagnosticEvents returns the diagnostic events as they appear in the TransactionMeta
// Please note that, depending on the configuration with which txMeta may be generated,
// it is possible that, for smart contract transactions, the list of generated diagnostic events MAY include contract events as well
// Users of this function (horizon, rpc, etc) should be careful not to double count diagnostic events and contract events in that case
func (t *TransactionMeta) GetDiagnosticEvents() ([]DiagnosticEvent, error) {
	switch t.V {
	case 1, 2:
		return nil, nil
	case 3:
		sorobanMeta := t.MustV3().SorobanMeta
		if sorobanMeta == nil {
			return nil, nil
		}
		return sorobanMeta.DiagnosticEvents, nil
	case 4:
		return t.MustV4().DiagnosticEvents, nil
	default:
		if events, err, ok := transactionMetaDiagnosticEvents(t); ok {
			return events, err
		}
		return nil, fmt.Errorf("unsupported TransactionMeta version: %v", t.V)
	}
}

// GetTransactionEvents returns the xdr.transactionEvents present in the ledger.
// For TxMetaVersions < 4, they will be empty
func (t *TransactionMeta) GetTransactionEvents() ([]TransactionEvent, error) {
	switch t.V {
	case 1, 2, 3:
		return nil, nil
	case 4:
		return t.MustV4().Events, nil
	default:
		if events, err, ok := transactionMetaTransactionEvents(t); ok {
			return events, err
		}
		return nil, fmt.Errorf("unsupported TransactionMeta version: %v", t.V)
	}
}
