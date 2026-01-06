package xdr

// IsFeeBump returns true if the transaction envelope is a fee bump transaction
func (e TransactionEnvelope) IsFeeBump() bool {
	return e.Type == EnvelopeTypeEnvelopeTypeTxFeeBump
}

// FeeBumpAccount returns the account paying for the fee bump transaction
func (e TransactionEnvelope) FeeBumpAccount() MuxedAccount {
	return e.MustFeeBump().Tx.FeeSource
}

// FeeBumpFee returns the fee defined for the fee bump transaction
func (e TransactionEnvelope) FeeBumpFee() int64 {
	return int64(e.MustFeeBump().Tx.Fee)
}

// FeeBumpSignatures returns the list of signatures for the fee bump transaction
func (e TransactionEnvelope) FeeBumpSignatures() []DecoratedSignature {
	return e.MustFeeBump().Signatures
}

// SourceAccount returns the source account for the transaction
// If the transaction envelope is for a fee bump transaction, SourceAccount()
// returns the source account of the inner transaction
func (e TransactionEnvelope) SourceAccount() MuxedAccount {
	switch e.Type {
	case EnvelopeTypeEnvelopeTypeTxFeeBump:
		return e.FeeBump.Tx.InnerTx.V1.Tx.SourceAccount
	case EnvelopeTypeEnvelopeTypeTx:
		return e.V1.Tx.SourceAccount
	case EnvelopeTypeEnvelopeTypeTxV0:
		result, _ := NewMuxedAccount(CryptoKeyTypeKeyTypeEd25519, e.V0.Tx.SourceAccountEd25519)
		return result
	default:
		panic("unsupported transaction type: " + e.Type.String())
	}
}

func (e TransactionEnvelope) FeeAccount() MuxedAccount {
	switch e.Type {
	case EnvelopeTypeEnvelopeTypeTxFeeBump:
		return e.FeeBumpAccount()
	default:
		return e.SourceAccount()
	}
}

// Fee returns the fee defined for the transaction envelope
// If the transaction envelope is for a fee bump transaction, Fee()
// returns the fee defined in the inner transaction
func (e TransactionEnvelope) Fee() uint32 {
	switch e.Type {
	case EnvelopeTypeEnvelopeTypeTxFeeBump:
		return uint32(e.FeeBump.Tx.InnerTx.V1.Tx.Fee)
	case EnvelopeTypeEnvelopeTypeTx:
		return uint32(e.V1.Tx.Fee)
	case EnvelopeTypeEnvelopeTypeTxV0:
		return uint32(e.V0.Tx.Fee)
	default:
		panic("unsupported transaction type: " + e.Type.String())
	}
}

// Signatures returns the list of signatures included in the transaction envelope
// If the transaction envelope is for a fee bump transaction, Signatures()
// returns the signatures for the inner transaction
func (e TransactionEnvelope) Signatures() []DecoratedSignature {
	switch e.Type {
	case EnvelopeTypeEnvelopeTypeTxFeeBump:
		return e.FeeBump.Tx.InnerTx.V1.Signatures
	case EnvelopeTypeEnvelopeTypeTx:
		return e.V1.Signatures
	case EnvelopeTypeEnvelopeTypeTxV0:
		return e.V0.Signatures
	default:
		panic("unsupported transaction type: " + e.Type.String())
	}
}

// SeqNum returns the sequence number set in the transaction envelope
// Note for fee bump transactions, SeqNum() returns the sequence number
// of the inner transaction
func (e TransactionEnvelope) SeqNum() int64 {
	switch e.Type {
	case EnvelopeTypeEnvelopeTypeTxFeeBump:
		return int64(e.FeeBump.Tx.InnerTx.V1.Tx.SeqNum)
	case EnvelopeTypeEnvelopeTypeTx:
		return int64(e.V1.Tx.SeqNum)
	case EnvelopeTypeEnvelopeTypeTxV0:
		return int64(e.V0.Tx.SeqNum)
	default:
		panic("unsupported transaction type: " + e.Type.String())
	}
}

// MinSeqNum returns the minimum sequence number set in the transaction envelope
//
// Note for fee bump transactions, MinSeqNum() returns the sequence number
// of the inner transaction
func (e TransactionEnvelope) MinSeqNum() *int64 {
	var p Preconditions
	switch e.Type {
	case EnvelopeTypeEnvelopeTypeTxFeeBump:
		p = e.FeeBump.Tx.InnerTx.V1.Tx.Cond
	case EnvelopeTypeEnvelopeTypeTx:
		p = e.V1.Tx.Cond
	case EnvelopeTypeEnvelopeTypeTxV0:
		return nil
	default:
		panic("unsupported transaction type: " + e.Type.String())
	}
	if p.Type != PreconditionTypePrecondV2 {
		return nil
	}
	if p.V2.MinSeqNum == nil {
		return nil
	}
	ret := int64(*p.V2.MinSeqNum)
	return &ret
}

// TimeBounds returns the time bounds set in the transaction envelope
// Note for fee bump transactions, TimeBounds() returns the time bounds
// of the inner transaction
func (e TransactionEnvelope) TimeBounds() *TimeBounds {
	switch e.Type {
	case EnvelopeTypeEnvelopeTypeTxFeeBump:
		return e.FeeBump.Tx.InnerTx.V1.Tx.TimeBounds()
	case EnvelopeTypeEnvelopeTypeTx:
		return e.V1.Tx.TimeBounds()
	case EnvelopeTypeEnvelopeTypeTxV0:
		return e.V0.Tx.TimeBounds
	default:
		panic("unsupported transaction type: " + e.Type.String())
	}
}

// LedgerBounds returns the ledger bounds set in the transaction envelope. Note
// for fee bump transactions, LedgerBounds() returns the ledger bounds of the
// inner transaction
func (e TransactionEnvelope) LedgerBounds() *LedgerBounds {
	switch e.Type {
	case EnvelopeTypeEnvelopeTypeTxFeeBump:
		return e.FeeBump.Tx.InnerTx.V1.Tx.LedgerBounds()
	case EnvelopeTypeEnvelopeTypeTx:
		return e.V1.Tx.LedgerBounds()
	case EnvelopeTypeEnvelopeTypeTxV0:
		return nil
	default:
		panic("unsupported transaction type: " + e.Type.String())
	}
}

// MinSeqAge returns the min seq age set in the transaction envelope. Note for
// fee bump transactions, MinSeqAge() returns the field from the inner
// transaction
func (e TransactionEnvelope) MinSeqAge() *Duration {
	switch e.Type {
	case EnvelopeTypeEnvelopeTypeTxFeeBump:
		return e.FeeBump.Tx.InnerTx.V1.Tx.MinSeqAge()
	case EnvelopeTypeEnvelopeTypeTx:
		return e.V1.Tx.MinSeqAge()
	case EnvelopeTypeEnvelopeTypeTxV0:
		return nil
	default:
		panic("unsupported transaction type: " + e.Type.String())
	}
}

// MinSeqLedgerGap returns the min seq ledger gap set in the transaction.
// envelope. Note for fee bump transactions, MinSeqLedgerGap() returns the
// field from the inner transaction
func (e TransactionEnvelope) MinSeqLedgerGap() *Uint32 {
	switch e.Type {
	case EnvelopeTypeEnvelopeTypeTxFeeBump:
		return e.FeeBump.Tx.InnerTx.V1.Tx.MinSeqLedgerGap()
	case EnvelopeTypeEnvelopeTypeTx:
		return e.V1.Tx.MinSeqLedgerGap()
	case EnvelopeTypeEnvelopeTypeTxV0:
		return nil
	default:
		panic("unsupported transaction type: " + e.Type.String())
	}
}

// ExtraSigners returns the extra signers set in the transaction envelope. Note
// for fee bump transactions, ExtraSigners() returns the field from the inner
// transaction
func (e TransactionEnvelope) ExtraSigners() []SignerKey {
	switch e.Type {
	case EnvelopeTypeEnvelopeTypeTxFeeBump:
		return e.FeeBump.Tx.InnerTx.V1.Tx.ExtraSigners()
	case EnvelopeTypeEnvelopeTypeTx:
		return e.V1.Tx.ExtraSigners()
	case EnvelopeTypeEnvelopeTypeTxV0:
		return nil
	default:
		panic("unsupported transaction type: " + e.Type.String())
	}
}

// Preconditions returns the preconditions on the transaction. If the
// transaction is a V0 envelope (aka before preconditions existed), this returns
// a new precondition (timebound if present, empty otherwise). If the
// transaction is a fee bump, it returns the preconditions of the *inner*
// transaction.
func (e TransactionEnvelope) Preconditions() Preconditions {
	switch e.Type {
	case EnvelopeTypeEnvelopeTypeTxFeeBump:
		return e.FeeBump.Tx.InnerTx.V1.Tx.Cond
	case EnvelopeTypeEnvelopeTypeTx:
		return e.V1.Tx.Cond
	case EnvelopeTypeEnvelopeTypeTxV0:
		return NewPreconditionsWithTimeBounds(e.TimeBounds())
	default:
		panic("unsupported transaction type: " + e.Type.String())
	}
}

// Operations returns the operations set in the transaction envelope
// Note for fee bump transactions, Operations() returns the operations
// of the inner transaction
func (e TransactionEnvelope) Operations() []Operation {
	switch e.Type {
	case EnvelopeTypeEnvelopeTypeTxFeeBump:
		return e.FeeBump.Tx.InnerTx.V1.Tx.Operations
	case EnvelopeTypeEnvelopeTypeTx:
		return e.V1.Tx.Operations
	case EnvelopeTypeEnvelopeTypeTxV0:
		return e.V0.Tx.Operations
	default:
		panic("unsupported transaction type: " + e.Type.String())
	}
}

// Memo returns the memo set in the transaction envelope
// Note for fee bump transactions, Memo() returns the memo
// of the inner transaction
func (e TransactionEnvelope) Memo() Memo {
	switch e.Type {
	case EnvelopeTypeEnvelopeTypeTxFeeBump:
		return e.FeeBump.Tx.InnerTx.V1.Tx.Memo
	case EnvelopeTypeEnvelopeTypeTx:
		return e.V1.Tx.Memo
	case EnvelopeTypeEnvelopeTypeTxV0:
		return e.V0.Tx.Memo
	default:
		panic("unsupported transaction type: " + e.Type.String())
	}
}

func (e TransactionEnvelope) OperationsCount() uint32 {
	return uint32(len(e.Operations()))
}
