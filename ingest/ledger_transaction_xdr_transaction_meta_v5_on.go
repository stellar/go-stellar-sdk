//go:build xdr_transaction_meta_v5

package ingest

import "github.com/stellar/go-stellar-sdk/xdr"

func (t *LedgerTransaction) getChangesForXdrTransactionMetaV5() ([]Change, bool) {
	switch t.UnsafeMeta.V {
	case 5:
		v5Meta := t.UnsafeMeta.MustV5()
		txBeforeChanges := v5Meta.TxChangesBefore
		txAfterChanges := v5Meta.TxChangesAfter
		meta := operationsMetaV2(v5Meta.Operations)

		var changes []Change
		txChangesBefore := t.getTransactionChanges(txBeforeChanges)
		changes = append(changes, txChangesBefore...)

		// Ignore operations meta and txChangesAfter if txInternalError
		// https://github.com/stellar/go-stellar-sdk/issues/2111
		if t.txInternalError() && t.LedgerVersion <= 12 {
			return changes, true
		}

		for opIdx := 0; opIdx < meta.len(); opIdx++ {
			opChanges := t.operationChanges(meta, uint32(opIdx))
			changes = append(changes, opChanges...)
		}

		txChangesAfter := t.getTransactionChanges(txAfterChanges)
		changes = append(changes, txChangesAfter...)
		return changes, true
	default:
		return nil, false
	}
}

func (t *LedgerTransaction) getOperationChangesMetaForXdrTransactionMetaV5() (operationsMeta, error, bool) {
	switch t.UnsafeMeta.V {
	case 5:
		return operationsMetaV2(t.UnsafeMeta.MustV5().Operations), nil, true
	default:
		return nil, nil, false
	}
}

func (t *LedgerTransaction) getTransactionEventsForXdrTransactionMetaV5() (TransactionEvents, error, bool) {
	txEvents := TransactionEvents{}
	switch t.UnsafeMeta.V {
	case 5:
		txMeta := t.UnsafeMeta.MustV5()
		txEvents.TransactionEvents = txMeta.Events
		txEvents.DiagnosticEvents = txMeta.DiagnosticEvents
		txEvents.OperationEvents = make([][]xdr.ContractEvent, len(txMeta.Operations))
		for i, op := range txMeta.Operations {
			txEvents.OperationEvents[i] = op.Events
		}
		return txEvents, nil, true
	default:
		return txEvents, nil, false
	}
}

func (t *LedgerTransaction) sorobanResourceFeeRefundTxChangesAfterForXdrTransactionMetaV5() (xdr.LedgerEntryChanges, error, bool) {
	switch t.UnsafeMeta.V {
	case 5:
		return t.UnsafeMeta.MustV5().TxChangesAfter, nil, true
	default:
		return nil, nil, false
	}
}
