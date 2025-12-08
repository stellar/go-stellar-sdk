package ingest

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/toid"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// LedgerTransaction represents the data for a single transaction within a ledger.
type LedgerTransaction struct {
	Index    uint32 // this index is 1-indexed as opposed to zero. Refer Read() in ledger_transaction_reader.go
	Envelope xdr.TransactionEnvelope
	Result   xdr.TransactionResultPair

	// FeeChanges, UnsafeMeta, and PostTxApplyFeeChanges are low level values, do not use them directly unless
	// you know what you are doing.
	// Use LedgerTransaction.GetChanges() for higher level access to ledger
	// entry changes.
	FeeChanges            xdr.LedgerEntryChanges
	UnsafeMeta            xdr.TransactionMeta
	PostTxApplyFeeChanges xdr.LedgerEntryChanges

	LedgerVersion uint32
	Ledger        xdr.LedgerCloseMeta // This is read-only and not to be modified by downstream functions
	Hash          xdr.Hash
}

type TransactionEvents struct {
	TransactionEvents []xdr.TransactionEvent
	OperationEvents   [][]xdr.ContractEvent
	DiagnosticEvents  []xdr.DiagnosticEvent
}

func (t *LedgerTransaction) txInternalError() bool {
	return t.Result.Result.Result.Code == xdr.TransactionResultCodeTxInternalError
}

func (t *LedgerTransaction) FeeAccount() xdr.MuxedAccount {
	return t.Envelope.FeeAccount()
}

// GetFeeChanges returns a developer friendly representation of LedgerEntryChanges
// connected to fees.
func (t *LedgerTransaction) GetFeeChanges() []Change {
	changes := GetChangesFromLedgerEntryChanges(t.FeeChanges)
	for i := range changes {
		changes[i].Reason = LedgerEntryChangeReasonFee
		changes[i].Transaction = t
		changes[i].Ledger = &t.Ledger
	}
	return changes
}

// GetPostApplyFeeChanges returns a developer friendly representation of LedgerEntryChanges
// connected to fee refunds which are applied after all transactions are executed.
func (t *LedgerTransaction) GetPostApplyFeeChanges() []Change {
	changes := GetChangesFromLedgerEntryChanges(t.PostTxApplyFeeChanges)
	for i := range changes {
		changes[i].Reason = LedgerEntryChangeReasonFeeRefund
		changes[i].Transaction = t
		changes[i].Ledger = &t.Ledger
	}
	return changes
}

func (t *LedgerTransaction) getTransactionChanges(ledgerEntryChanges xdr.LedgerEntryChanges) []Change {
	changes := GetChangesFromLedgerEntryChanges(ledgerEntryChanges)
	for i := range changes {
		changes[i].Reason = LedgerEntryChangeReasonTransaction
		changes[i].Transaction = t
		changes[i].Ledger = &t.Ledger
	}
	return changes
}

// GetChanges returns a developer friendly representation of LedgerEntryChanges.
// It contains transaction changes and operation changes in that order. If the
// transaction failed with TxInternalError, operations and txChangesAfter are
// omitted. It doesn't support legacy TransactionMeta.V=0.
func (t *LedgerTransaction) GetChanges() ([]Change, error) {
	var changes []Change

	// Transaction meta
	switch t.UnsafeMeta.V {
	case 0:
		return changes, errors.New("TransactionMeta.V=0 not supported")
	case 1:
		v1Meta := t.UnsafeMeta.MustV1()
		// The var `txChanges` reflect the ledgerEntryChanges that are changed because of the transaction as a whole
		txChanges := t.getTransactionChanges(v1Meta.TxChanges)
		changes = append(changes, txChanges...)

		// Ignore operations meta if txInternalError https://github.com/stellar/go-stellar-sdk/issues/2111
		if t.txInternalError() && t.LedgerVersion <= 12 {
			return changes, nil
		}

		// These changes reflect the ledgerEntry changes that were caused by the operations in the transaction
		// Populate the operationInfo for these changes in the `Change` struct

		operationMeta := v1Meta.Operations
		//	operationMeta is a list of lists.
		//	Each element in operationMeta is a list of ledgerEntryChanges
		//	caused by the operation at that index of the element
		for opIdx := range operationMeta {
			opChanges := t.operationChanges(operationsMetaV1(v1Meta.Operations), uint32(opIdx))
			changes = append(changes, opChanges...)
		}
	case 2, 3, 4:
		var (
			txBeforeChanges, txAfterChanges xdr.LedgerEntryChanges
			meta                            operationsMeta
		)

		switch t.UnsafeMeta.V {
		case 2:
			v2Meta := t.UnsafeMeta.MustV2()
			txBeforeChanges = v2Meta.TxChangesBefore
			txAfterChanges = v2Meta.TxChangesAfter
			meta = operationsMetaV1(v2Meta.Operations)
		case 3:
			v3Meta := t.UnsafeMeta.MustV3()
			txBeforeChanges = v3Meta.TxChangesBefore
			txAfterChanges = v3Meta.TxChangesAfter
			meta = operationsMetaV1(v3Meta.Operations)
		case 4:
			v4Meta := t.UnsafeMeta.MustV4()
			txBeforeChanges = v4Meta.TxChangesBefore
			txAfterChanges = v4Meta.TxChangesAfter
			meta = operationsMetaV2(v4Meta.Operations)
		default:
			panic("Invalid meta version, expected 2, 3, or 4")
		}

		txChangesBefore := t.getTransactionChanges(txBeforeChanges)
		changes = append(changes, txChangesBefore...)

		// Ignore operations meta and txChangesAfter if txInternalError
		// https://github.com/stellar/go-stellar-sdk/issues/2111
		if t.txInternalError() && t.LedgerVersion <= 12 {
			return changes, nil
		}

		//	operationMeta is a list of lists.
		//	Each element in operationMeta is a list of ledgerEntryChanges
		//	caused by the operation at that index of the element
		for opIdx := 0; opIdx < meta.len(); opIdx++ {
			opChanges := t.operationChanges(meta, uint32(opIdx))
			changes = append(changes, opChanges...)
		}

		txChangesAfter := t.getTransactionChanges(txAfterChanges)
		changes = append(changes, txChangesAfter...)
	default:
		return changes, fmt.Errorf("unsupported TransactionMeta version: %v", t.UnsafeMeta.V)
	}

	return changes, nil
}

// GetOperation returns an operation by index.
func (t *LedgerTransaction) GetOperation(index uint32) (xdr.Operation, bool) {
	ops := t.Envelope.Operations()
	if int(index) >= len(ops) {
		return xdr.Operation{}, false
	}
	return ops[index], true
}

// GetOperationChanges returns a developer friendly representation of LedgerEntryChanges.
// It contains only operation changes.
func (t *LedgerTransaction) GetOperationChanges(operationIndex uint32) ([]Change, error) {
	if t.UnsafeMeta.V == 0 {
		return []Change{}, errors.New("TransactionMeta.V=0 not supported")
	}

	// Ignore operations meta if txInternalError https://github.com/stellar/go-stellar-sdk/issues/2111
	if t.txInternalError() && t.LedgerVersion <= 12 {
		return []Change{}, nil
	}

	var meta operationsMeta
	switch t.UnsafeMeta.V {
	case 1:
		meta = operationsMetaV1(t.UnsafeMeta.MustV1().Operations)
	case 2:
		meta = operationsMetaV1(t.UnsafeMeta.MustV2().Operations)
	case 3:
		meta = operationsMetaV1(t.UnsafeMeta.MustV3().Operations)
	case 4:
		meta = operationsMetaV2(t.UnsafeMeta.MustV4().Operations)
	default:
		return []Change{}, fmt.Errorf("unsupported TransactionMeta version: %v", t.UnsafeMeta.V)
	}
	return t.operationChanges(meta, operationIndex), nil
}

type operationsMeta interface {
	getChanges(op uint32) xdr.LedgerEntryChanges
	len() int
}

type operationsMetaV1 []xdr.OperationMeta

func (ops operationsMetaV1) getChanges(op uint32) xdr.LedgerEntryChanges {
	return ops[op].Changes
}

func (ops operationsMetaV1) len() int {
	return len(ops)
}

type operationsMetaV2 []xdr.OperationMetaV2

func (ops operationsMetaV2) getChanges(op uint32) xdr.LedgerEntryChanges {
	return ops[op].Changes
}

func (ops operationsMetaV2) len() int {
	return len(ops)
}

func (t *LedgerTransaction) operationChanges(ops operationsMeta, index uint32) []Change {
	if int(index) >= ops.len() {
		return []Change{}
	}

	changes := GetChangesFromLedgerEntryChanges(ops.getChanges(index))

	for i := range changes {
		changes[i].Reason = LedgerEntryChangeReasonOperation
		changes[i].Transaction = t
		changes[i].OperationIndex = index
		changes[i].Ledger = &t.Ledger
	}
	return changes
}

func (t *LedgerTransaction) GetContractEventsForOperation(opIndex uint32) ([]xdr.ContractEvent, error) {
	return t.UnsafeMeta.GetContractEventsForOperation(opIndex)
}

// GetContractEvents returns a []xdr.ContractEvent for pnly smart contract transaction.
// If it is not a smart contract transaction, it throws an error
// For getting events from classic operations/transaction, use GetContractEventsForOperation
// For getting soroban smart contract events,we rely on the fact that there will only be one operation present in the transaction
func (t *LedgerTransaction) GetContractEvents() ([]xdr.ContractEvent, error) {
	if !t.IsSorobanTx() {
		return nil, errors.New("not a soroban transaction")
	}
	return t.GetContractEventsForOperation(0)
}

// GetDiagnosticEvents returns strictly diagnostic events emitted by a given transaction.
// Please note that, depending on the configuration with which txMeta may be generated,
// it is possible that, for smart contract transactions, the list of generated diagnostic events MAY include contract events as well
// Users of this function (horizon, rpc, etc) should be careful not to double count diagnostic events and contract events in that case
func (t *LedgerTransaction) GetDiagnosticEvents() ([]xdr.DiagnosticEvent, error) {
	return t.UnsafeMeta.GetDiagnosticEvents()
}

// GetTransactionEvents gives the breakdown of xdr.ContractEvent, xdr.TransactionEvent, xdr.Disgnostic event as they appea in the TxMeta
// In TransactionMetaV3, for soroban transactions, contract events and diagnostic events appear in the SorobanMeta struct in TransactionMetaV3, i.e. at the transaction level
// In TransactionMetaV4 and onwards, there is a more granular breakdown, because of CAP-67 unified events
//   - Classic operations will also have contract events.
//   - Contract events will now be present in the "operation []OperationMetaV2" in the TransactionMetaV4 structure, instead of at the transaction level as in TxMetaV3.
//     This is true for soroban transactions as well, which will only have one operation and thus contract events will appear at index 0 in the []OperationMetaV2 structure
//   - Additionally, if its a soroban  transaction, the diagnostic events will also be included in the "DiagnosticEvents []DiagnosticEvent" structure
//   - Non soroban transactions will have an empty list for DiagnosticEvents
//
// It is preferred to use this function in horizon and rpc
func (t *LedgerTransaction) GetTransactionEvents() (TransactionEvents, error) {
	txEvents := TransactionEvents{}
	switch t.UnsafeMeta.V {
	case 1, 2:
		return txEvents, nil
	case 3:
		// There wont be any events for classic operations in TxMetaV3
		if !t.IsSorobanTx() {
			return txEvents, nil
		}
		contractEvents, err := t.GetContractEvents()
		if err != nil {
			return txEvents, err
		}
		diagnosticEvents, err := t.GetDiagnosticEvents()
		if err != nil {
			return txEvents, err
		}
		// There  will only ever be 1 smart contract operation per tx.
		txEvents.OperationEvents = make([][]xdr.ContractEvent, 1)
		txEvents.OperationEvents[0] = contractEvents
		txEvents.DiagnosticEvents = diagnosticEvents
	case 4:
		txMeta := t.UnsafeMeta.MustV4()
		txEvents.TransactionEvents = txMeta.Events
		txEvents.DiagnosticEvents = txMeta.DiagnosticEvents
		txEvents.OperationEvents = make([][]xdr.ContractEvent, len(txMeta.Operations))
		for i, op := range txMeta.Operations {
			txEvents.OperationEvents[i] = op.Events
		}
	default:
		return txEvents, fmt.Errorf("unsupported TransactionMeta version: %v", t.UnsafeMeta.V)
	}
	return txEvents, nil

}

func (t *LedgerTransaction) ID() int64 {
	return toid.New(int32(t.Ledger.LedgerSequence()), int32(t.Index), 0).ToInt64()
}

func (t *LedgerTransaction) Account() (string, error) {
	sourceAccount := t.Envelope.SourceAccount().ToAccountId()
	return sourceAccount.GetAddress()
}

func (t *LedgerTransaction) AccountSequence() int64 {
	return t.Envelope.SeqNum()
}

func (t *LedgerTransaction) MaxFee() uint32 {
	return t.Envelope.Fee()
}

func (t *LedgerTransaction) FeeCharged() (int64, bool) {
	// Any Soroban Fee Bump transactions before P21 will need the below logic to calculate the correct feeCharged
	// Protocol 20 contained a bug where the feeCharged was incorrectly calculated but was fixed for
	// Protocol 21 with https://github.com/stellar/stellar-core/issues/4188
	var ok bool
	_, ok = t.GetSorobanData()
	if ok {
		if uint32(t.Ledger.LedgerHeaderHistoryEntry().Header.LedgerVersion) < 21 && t.Envelope.Type == xdr.EnvelopeTypeEnvelopeTypeTxFeeBump {
			resourceFeeRefund := t.SorobanResourceFeeRefund()
			return int64(t.Result.Result.FeeCharged) - resourceFeeRefund, true
		}
	}

	return int64(t.Result.Result.FeeCharged), true

}

func (t *LedgerTransaction) OperationCount() uint32 {
	return t.Envelope.OperationsCount()
}

func (t *LedgerTransaction) Memo() string {
	var memoContents string
	memoObject := t.Envelope.Memo()

	switch xdr.MemoType(memoObject.Type) {
	case xdr.MemoTypeMemoNone:
		memoContents = ""
	case xdr.MemoTypeMemoText:
		memoContents = memoObject.MustText()
	case xdr.MemoTypeMemoId:
		memoContents = strconv.FormatUint(uint64(memoObject.MustId()), 10)
	case xdr.MemoTypeMemoHash:
		hash := memoObject.MustHash()
		memoContents = base64.StdEncoding.EncodeToString(hash[:])
	case xdr.MemoTypeMemoReturn:
		hash := memoObject.MustRetHash()
		memoContents = base64.StdEncoding.EncodeToString(hash[:])
	default:
		panic(fmt.Errorf("unknown MemoType %d", memoObject.Type))
	}

	return memoContents
}

func (t *LedgerTransaction) MemoType() string {
	memoObject := t.Envelope.Memo()
	return memoObject.Type.String()
}

func (t *LedgerTransaction) TimeBounds() (string, bool) {
	timeBounds := t.Envelope.TimeBounds()
	if timeBounds == nil {
		return "", false
	}

	if timeBounds.MaxTime == 0 {
		return fmt.Sprintf("[%d,)", timeBounds.MinTime), true
	}

	return fmt.Sprintf("[%d,%d)", timeBounds.MinTime, timeBounds.MaxTime), true
}

func (t *LedgerTransaction) LedgerBounds() (string, bool) {
	ledgerBounds := t.Envelope.LedgerBounds()
	if ledgerBounds == nil {
		return "", false
	}

	return fmt.Sprintf("[%d,%d)", int64(ledgerBounds.MinLedger), int64(ledgerBounds.MaxLedger)), true

}

func (t *LedgerTransaction) MinSequence() (int64, bool) {
	minSeqNum := t.Envelope.MinSeqNum()
	if minSeqNum == nil {
		return 0, false
	}

	return *minSeqNum, true
}

func (t *LedgerTransaction) MinSequenceAge() (int64, bool) {
	minSequenceAge := t.Envelope.MinSeqAge()
	if minSequenceAge == nil {
		return 0, false
	}

	return int64(*minSequenceAge), true
}

func (t *LedgerTransaction) MinSequenceLedgerGap() (int64, bool) {
	minSequenceLedgerGap := t.Envelope.MinSeqLedgerGap()
	if minSequenceLedgerGap == nil {
		return 0, false
	}

	return int64(*minSequenceLedgerGap), true
}

func (t *LedgerTransaction) GetSorobanData() (xdr.SorobanTransactionData, bool) {
	switch t.Envelope.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		return t.Envelope.V1.Tx.Ext.GetSorobanData()
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		return t.Envelope.FeeBump.Tx.InnerTx.V1.Tx.Ext.GetSorobanData()
	case xdr.EnvelopeTypeEnvelopeTypeTxV0:
		return xdr.SorobanTransactionData{}, false
	case xdr.EnvelopeTypeEnvelopeTypeScp:
		return xdr.SorobanTransactionData{}, false
	case xdr.EnvelopeTypeEnvelopeTypeAuth:
		return xdr.SorobanTransactionData{}, false
	case xdr.EnvelopeTypeEnvelopeTypeScpvalue:
		return xdr.SorobanTransactionData{}, false
	case xdr.EnvelopeTypeEnvelopeTypeOpId:
		return xdr.SorobanTransactionData{}, false
	case xdr.EnvelopeTypeEnvelopeTypePoolRevokeOpId:
		return xdr.SorobanTransactionData{}, false
	case xdr.EnvelopeTypeEnvelopeTypeContractId:
		return xdr.SorobanTransactionData{}, false
	case xdr.EnvelopeTypeEnvelopeTypeSorobanAuthorization:
		return xdr.SorobanTransactionData{}, false
	default:
		panic(fmt.Errorf("unknown EnvelopeType %d", t.Envelope.Type))
	}
}

func (t *LedgerTransaction) IsSorobanTx() bool {
	_, res := t.GetSorobanData()
	return res
}

func (t *LedgerTransaction) SorobanResourceFee() (int64, bool) {
	sorobanData, ok := t.GetSorobanData()
	if !ok {
		return 0, false
	}

	return int64(sorobanData.ResourceFee), true
}

func (t *LedgerTransaction) SorobanResourcesInstructions() (uint32, bool) {
	sorobanData, ok := t.GetSorobanData()
	if !ok {
		return 0, false
	}

	return uint32(sorobanData.Resources.Instructions), true
}

func (t *LedgerTransaction) SorobanResourcesDiskReadBytes() (uint32, bool) {
	sorobanData, ok := t.GetSorobanData()
	if !ok {
		return 0, false
	}

	return uint32(sorobanData.Resources.DiskReadBytes), true
}

func (t *LedgerTransaction) SorobanResourcesWriteBytes() (uint32, bool) {
	sorobanData, ok := t.GetSorobanData()
	if !ok {
		return 0, false
	}

	return uint32(sorobanData.Resources.WriteBytes), true
}

func (t *LedgerTransaction) SorobanInclusionFeeBid() (int64, bool) {
	resourceFee, ok := t.SorobanResourceFee()
	if !ok {
		return 0, false
	}

	return int64(t.Envelope.Fee()) - resourceFee, true
}

func (t *LedgerTransaction) FeeAccountAddress() (string, bool) {
	switch t.Envelope.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		sourceAccount := t.Envelope.SourceAccount()
		feeAccountAddress := sourceAccount.Address()
		return feeAccountAddress, true
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		feeBumpAccount := t.Envelope.FeeBumpAccount()
		feeAccountAddress := feeBumpAccount.Address()
		return feeAccountAddress, true
	case xdr.EnvelopeTypeEnvelopeTypeTxV0:
		return "", false
	case xdr.EnvelopeTypeEnvelopeTypeScp:
		return "", false
	case xdr.EnvelopeTypeEnvelopeTypeAuth:
		return "", false
	case xdr.EnvelopeTypeEnvelopeTypeScpvalue:
		return "", false
	case xdr.EnvelopeTypeEnvelopeTypeOpId:
		return "", false
	case xdr.EnvelopeTypeEnvelopeTypePoolRevokeOpId:
		return "", false
	case xdr.EnvelopeTypeEnvelopeTypeContractId:
		return "", false
	case xdr.EnvelopeTypeEnvelopeTypeSorobanAuthorization:
		return "", false
	default:
		panic(fmt.Errorf("unknown EnvelopeType %d", t.Envelope.Type))
	}
}

func (t *LedgerTransaction) SorobanInclusionFeeCharged() (int64, bool) {
	resourceFee, ok := t.SorobanResourceFee()
	if !ok {
		return 0, false
	}

	feeAccountAddress, ok := t.FeeAccountAddress()
	if !ok {
		return 0, false
	}

	accountBalanceStart, accountBalanceEnd := getAccountBalanceFromLedgerEntryChanges(t.FeeChanges, feeAccountAddress)
	initialFeeCharged := accountBalanceStart - accountBalanceEnd

	return initialFeeCharged - resourceFee, true
}

func (t *LedgerTransaction) InclusionFeeCharged() (int64, bool) {
	inclusionFee, ok := t.SorobanInclusionFeeCharged()
	if ok {
		return inclusionFee, ok
	}

	// Inclusion fee can be calculated from the equation:
	// Fee charged = inclusion fee * (operation count + if(feeBumpAccount is not null, 1, 0))
	// or inclusionFee = feeCharged/(operation count + if(feeBumpAccount is not null, 1, 0))
	operationCount := t.OperationCount()
	if t.Envelope.Type == xdr.EnvelopeTypeEnvelopeTypeTxFeeBump {
		operationCount += 1
	}

	var feeCharged int64
	feeCharged, ok = t.FeeCharged()
	if !ok {
		return 0, false
	}

	return feeCharged / int64(operationCount), true
}

func getAccountBalanceFromLedgerEntryChanges(changes xdr.LedgerEntryChanges, sourceAccountAddress string) (int64, int64) {
	var accountBalanceStart int64
	var accountBalanceEnd int64

	for _, change := range changes {
		switch change.Type {
		case xdr.LedgerEntryChangeTypeLedgerEntryUpdated:
			accountEntry, ok := change.Updated.Data.GetAccount()
			if !ok {
				continue
			}

			if accountEntry.AccountId.Address() == sourceAccountAddress {
				accountBalanceEnd = int64(accountEntry.Balance)
			}
		case xdr.LedgerEntryChangeTypeLedgerEntryState:
			accountEntry, ok := change.State.Data.GetAccount()
			if !ok {
				continue
			}

			if accountEntry.AccountId.Address() == sourceAccountAddress {
				accountBalanceStart = int64(accountEntry.Balance)
			}
		case xdr.LedgerEntryChangeTypeLedgerEntryCreated:
			continue
		case xdr.LedgerEntryChangeTypeLedgerEntryRemoved:
			continue
		default:
			panic(fmt.Errorf("unknown ChangeType %d", change.Type))
		}
	}

	return accountBalanceStart, accountBalanceEnd
}

func (t *LedgerTransaction) OriginalFeeCharged() int64 {
	startingBal, endingBal := getAccountBalanceFromLedgerEntryChanges(t.FeeChanges, t.FeeAccount().ToAccountId().Address())
	if endingBal > startingBal {
		panic("Invalid Fee")
	}
	return startingBal - endingBal
}

func (t *LedgerTransaction) SorobanResourceFeeRefund() int64 {
	if !t.IsSorobanTx() {
		return 0
	}
	var txChangesAfter xdr.LedgerEntryChanges
	switch t.UnsafeMeta.V {
	case 3:
		txChangesAfter = t.UnsafeMeta.MustV3().TxChangesAfter
	case 4:
		txChangesAfter = t.UnsafeMeta.MustV4().TxChangesAfter
	default:
		panic(fmt.Errorf("Invalid txMeta version: %d", t.UnsafeMeta.V))
	}

	// For soroban transactions before P23, the feeRefund changes will show up in TxMeta in the `TxChangesAfter` field
	// From P23 onwards, they will show up in the PostTxApplyFeeChanges field and not in TxChangesAfter
	// You can safely append the TxChangesAfter and PostTxApplyFeeChanges before passing it to getAccountBalanceFromLedgerEntryChanges,
	// since only one of them will reflect balance changes
	allChanges := append(txChangesAfter, t.PostTxApplyFeeChanges...)

	startingBal, endingBal := getAccountBalanceFromLedgerEntryChanges(allChanges, t.FeeAccount().ToAccountId().Address())
	if startingBal > endingBal {
		panic("Invalid Soroban Resource Refund")
	}
	return endingBal - startingBal
}

func (t *LedgerTransaction) SorobanTotalNonRefundableResourceFeeCharged() (int64, bool) {
	meta, ok := t.UnsafeMeta.GetV3()
	if !ok {
		return 0, false
	}

	switch meta.SorobanMeta.Ext.V {
	case 1:
		return int64(meta.SorobanMeta.Ext.V1.TotalNonRefundableResourceFeeCharged), true
	default:
		panic(fmt.Errorf("unknown SorobanMeta.Ext.V %d", meta.SorobanMeta.Ext.V))
	}
}

func (t *LedgerTransaction) SorobanTotalRefundableResourceFeeCharged() (int64, bool) {
	meta, ok := t.UnsafeMeta.GetV3()
	if !ok {
		return 0, false
	}

	switch meta.SorobanMeta.Ext.V {
	case 1:
		return int64(meta.SorobanMeta.Ext.V1.TotalRefundableResourceFeeCharged), true
	default:
		panic(fmt.Errorf("unknown SorobanMeta.Ext.V %d", meta.SorobanMeta.Ext.V))
	}
}

func (t *LedgerTransaction) SorobanRentFeeCharged() (int64, bool) {
	meta, ok := t.UnsafeMeta.GetV3()
	if !ok {
		return 0, false
	}

	switch meta.SorobanMeta.Ext.V {
	case 1:
		return int64(meta.SorobanMeta.Ext.V1.RentFeeCharged), true
	default:
		panic(fmt.Errorf("unknown SorobanMeta.Ext.V %d", meta.SorobanMeta.Ext.V))
	}
}

func (t *LedgerTransaction) ResultCode() string {
	return t.Result.Result.Result.Code.String()
}

func (t *LedgerTransaction) Signers() (signers []string, err error) {
	if t.Envelope.IsFeeBump() {
		return getTxSigners(t.Envelope.FeeBump.Signatures)
	}

	return getTxSigners(t.Envelope.Signatures())
}

func getTxSigners(xdrSignatures []xdr.DecoratedSignature) ([]string, error) {
	signers := make([]string, len(xdrSignatures))

	for i, sig := range xdrSignatures {
		signerAccount, err := strkey.Encode(strkey.VersionByteAccountID, sig.Signature)
		if err != nil {
			return nil, err
		}
		signers[i] = signerAccount
	}

	return signers, nil
}

func (t *LedgerTransaction) AccountMuxed() (string, bool) {
	sourceAccount := t.Envelope.SourceAccount()
	if sourceAccount.Type != xdr.CryptoKeyTypeKeyTypeMuxedEd25519 {
		return "", false
	}

	return sourceAccount.Address(), true

}

func (t *LedgerTransaction) InnerTransactionHash() (string, bool) {
	if !t.Envelope.IsFeeBump() {
		return "", false
	}

	innerHash := t.Result.InnerHash()

	return hex.EncodeToString(innerHash[:]), true
}

func (t *LedgerTransaction) NewMaxFee() (uint32, bool) {
	if !t.Envelope.IsFeeBump() {
		return 0, false
	}

	return uint32(t.Envelope.FeeBumpFee()), true
}

func (t *LedgerTransaction) Successful() bool {
	return t.Result.Successful()
}

func (t *LedgerTransaction) GetTransactionV1Envelope() (xdr.TransactionV1Envelope, bool) {
	switch t.Envelope.Type {
	case xdr.EnvelopeTypeEnvelopeTypeTx:
		switch t.Envelope.Type {
		case 2:
			return *t.Envelope.V1, true
		default:
			return xdr.TransactionV1Envelope{}, false
		}
	case xdr.EnvelopeTypeEnvelopeTypeTxFeeBump:
		return t.Envelope.MustFeeBump().Tx.InnerTx.MustV1(), true
	default:
		return xdr.TransactionV1Envelope{}, false
	}
}

func (t *LedgerTransaction) LedgerKeyHashesFromSorobanFootprint() []string {
	var ledgerKeyHash []string

	v1Envelope, ok := t.GetTransactionV1Envelope()
	if !ok {
		return ledgerKeyHash
	}

	for _, ledgerKey := range v1Envelope.Tx.Ext.SorobanData.Resources.Footprint.ReadOnly {
		ledgerKeyBase64, err := xdr.MarshalBase64(ledgerKey)
		if err != nil {
			panic(err)
		}
		if ledgerKeyBase64 != "" {
			ledgerKeyHash = append(ledgerKeyHash, ledgerKeyBase64)
		}
	}

	for _, ledgerKey := range v1Envelope.Tx.Ext.SorobanData.Resources.Footprint.ReadWrite {
		ledgerKeyBase64, err := xdr.MarshalBase64(ledgerKey)
		if err != nil {
			panic(err)
		}
		if ledgerKeyBase64 != "" {
			ledgerKeyHash = append(ledgerKeyHash, ledgerKeyBase64)
		}
	}

	return ledgerKeyHash
}

func (t *LedgerTransaction) ContractCodeHashFromSorobanFootprint() (string, bool) {
	v1Envelope, ok := t.GetTransactionV1Envelope()
	if !ok {
		return "", false
	}
	for _, ledgerKey := range v1Envelope.Tx.Ext.SorobanData.Resources.Footprint.ReadOnly {
		contractCode, ok := t.contractCodeFromContractData(ledgerKey)
		if !ok {
			return "", false
		}
		if contractCode != "" {
			return contractCode, true
		}
	}

	for _, ledgerKey := range v1Envelope.Tx.Ext.SorobanData.Resources.Footprint.ReadWrite {
		contractCode, ok := t.contractCodeFromContractData(ledgerKey)
		if !ok {
			return "", false
		}
		if contractCode != "" {
			return contractCode, true
		}
	}

	return "", true
}

func (t *LedgerTransaction) contractCodeFromContractData(ledgerKey xdr.LedgerKey) (string, bool) {
	contractCode, ok := ledgerKey.GetContractCode()
	if !ok {
		return "", false
	}

	codeHash, err := xdr.MarshalBase64(contractCode.Hash)
	if err != nil {
		panic(err)
	}

	return codeHash, true
}

func (t *LedgerTransaction) ContractIdFromTxEnvelope() (string, bool) {
	v1Envelope, ok := t.GetTransactionV1Envelope()
	if !ok {
		return "", false
	}
	for _, ledgerKey := range v1Envelope.Tx.Ext.SorobanData.Resources.Footprint.ReadWrite {
		contractId, ok := t.contractIdFromContractData(ledgerKey)
		if !ok {
			return "", false
		}

		if contractId != "" {
			return contractId, true
		}
	}

	for _, ledgerKey := range v1Envelope.Tx.Ext.SorobanData.Resources.Footprint.ReadOnly {
		contractId, ok := t.contractIdFromContractData(ledgerKey)
		if !ok {
			return "", false
		}

		if contractId != "" {
			return contractId, true
		}
	}

	return "", true
}

func (t *LedgerTransaction) contractIdFromContractData(ledgerKey xdr.LedgerKey) (string, bool) {
	contractData, ok := ledgerKey.GetContractData()
	if !ok {
		return "", false
	}
	contractIdHash, ok := contractData.Contract.GetContractId()
	if !ok {
		return "", false
	}

	var contractIdByte []byte
	var contractId string
	var err error
	contractIdByte, err = contractIdHash.MarshalBinary()
	if err != nil {
		panic(err)
	}
	contractId, err = strkey.Encode(strkey.VersionByteContract, contractIdByte)
	if err != nil {
		panic(err)
	}

	return contractId, true
}
