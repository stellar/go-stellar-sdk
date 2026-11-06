package xdr

// Transaction returns the transaction in the envelope in its V1 form, the
// transaction the decoded TransactionEnvelope helpers (SourceAccount, SeqNum,
// MinSeqNum, Operations, Memo) read: a V1 envelope's transaction, a fee
// bump's inner transaction, or a V0 transaction read as a Transaction. Like
// the other view accessors, it returns a view into v that may extend past the
// transaction; Raw trims it.
func (v TransactionEnvelopeView) Transaction() (TransactionView, error) {
	return Try(func() TransactionView {
		switch v.MustType() {
		case EnvelopeTypeEnvelopeTypeTxV0:
			// ENVELOPE_TYPE_TX_V0 and KEY_TYPE_ED25519 are both 0, so the
			// envelope type followed by the V0 transaction is the transaction
			// in V1 form, with an ed25519 source account. The rest encodes
			// the same for a well-formed V0 transaction: absent time bounds
			// as PRECOND_NONE, present time bounds as PRECOND_TIME, and the
			// void ext as ext 0.
			return TransactionView(v)
		case EnvelopeTypeEnvelopeTypeTxFeeBump:
			return v.MustFeeBump().MustTx().MustInnerTx().MustV1().MustTx()
		default:
			// MustV1 rejects every other envelope type.
			return v.MustV1().MustTx()
		}
	})
}

// MustTransaction is Transaction for use inside Try.
func (v TransactionEnvelopeView) MustTransaction() TransactionView { return must(v.Transaction()) }
