package xdr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func createLegacyTx() TransactionEnvelope {
	return TransactionEnvelope{
		Type: EnvelopeTypeEnvelopeTypeTxV0,
		V0: &TransactionV0Envelope{
			Tx: TransactionV0{
				SourceAccountEd25519: Uint256{1, 2, 3},
				Fee:                  99,
				Memo: Memo{
					Type: MemoTypeMemoNone,
				},
				SeqNum: 33,
				TimeBounds: &TimeBounds{
					MinTime: 1,
					MaxTime: 2,
				},
				Operations: []Operation{
					{
						Body: OperationBody{
							BumpSequenceOp: &BumpSequenceOp{
								BumpTo: 34,
							},
						},
					},
				},
			},
			Signatures: []DecoratedSignature{
				{
					Hint:      SignatureHint{1, 1, 1, 1},
					Signature: Signature{10, 10, 10},
				},
			},
		},
	}
}

func createTx() TransactionEnvelope {
	sourceAccount, _ := NewMuxedAccount(CryptoKeyTypeKeyTypeEd25519, Uint256{3, 3, 3})
	memo := MemoHash(Hash{1, 1, 1})
	return TransactionEnvelope{
		Type: EnvelopeTypeEnvelopeTypeTx,
		V1: &TransactionV1Envelope{
			Tx: Transaction{
				SourceAccount: sourceAccount,
				Fee:           99,
				Memo:          memo,
				SeqNum:        97,
				Cond: Preconditions{
					Type: PreconditionTypePrecondTime,
					TimeBounds: &TimeBounds{
						MinTime: 2,
						MaxTime: 4,
					},
				},
				Operations: []Operation{
					{
						Body: OperationBody{
							BumpSequenceOp: &BumpSequenceOp{
								BumpTo: 98,
							},
						},
					},
				},
			},
			Signatures: []DecoratedSignature{
				{
					Hint:      SignatureHint{2, 2, 2, 2},
					Signature: Signature{20, 20, 20},
				},
			},
		},
	}
}

func createCondV2Tx() TransactionEnvelope {
	minSeqNum := SequenceNumber(7)
	sourceAccount, _ := NewMuxedAccount(CryptoKeyTypeKeyTypeEd25519, Uint256{3, 3, 3})
	memo := MemoHash(Hash{1, 1, 1})
	extraSigner, _ := NewSignerKey(SignerKeyTypeSignerKeyTypeEd25519, Uint256{3, 3, 3})
	return TransactionEnvelope{
		Type: EnvelopeTypeEnvelopeTypeTx,
		V1: &TransactionV1Envelope{
			Tx: Transaction{
				SourceAccount: sourceAccount,
				Fee:           99,
				Memo:          memo,
				SeqNum:        97,
				Cond: Preconditions{
					Type: PreconditionTypePrecondV2,
					V2: &PreconditionsV2{
						TimeBounds: &TimeBounds{
							MinTime: 2,
							MaxTime: 4,
						},
						LedgerBounds: &LedgerBounds{
							MinLedger: 5,
							MaxLedger: 6,
						},
						MinSeqNum:       &minSeqNum,
						MinSeqAge:       Duration(8),
						MinSeqLedgerGap: Uint32(9),
						ExtraSigners:    []SignerKey{extraSigner},
					},
				},
				Operations: []Operation{
					{
						Body: OperationBody{
							BumpSequenceOp: &BumpSequenceOp{
								BumpTo: 98,
							},
						},
					},
				},
			},
			Signatures: []DecoratedSignature{
				{
					Hint:      SignatureHint{2, 2, 2, 2},
					Signature: Signature{20, 20, 20},
				},
			},
		},
	}
}

func createFeeBumpTx() TransactionEnvelope {
	feeSource, _ := NewMuxedAccount(CryptoKeyTypeKeyTypeEd25519, Uint256{2, 2, 2})
	return TransactionEnvelope{
		Type: EnvelopeTypeEnvelopeTypeTxFeeBump,
		FeeBump: &FeeBumpTransactionEnvelope{
			Tx: FeeBumpTransaction{
				FeeSource: feeSource,
				Fee:       776,
				InnerTx: FeeBumpTransactionInnerTx{
					Type: EnvelopeTypeEnvelopeTypeTx,
					V1:   createTx().V1,
				},
			},
			Signatures: []DecoratedSignature{
				{
					Hint:      SignatureHint{3, 3, 3, 3},
					Signature: Signature{30, 30, 30},
				},
			},
		},
	}
}

func TestIsFeeBump(t *testing.T) {
	legacyTx := createLegacyTx()
	tx := createTx()
	feeBumpTx := createFeeBumpTx()

	assert.False(t, legacyTx.IsFeeBump())
	assert.False(t, tx.IsFeeBump())

	assert.True(t, feeBumpTx.IsFeeBump())
}

func TestFeeBumpAccount(t *testing.T) {
	legacyTx := createLegacyTx()
	tx := createTx()
	feeBumpTx := createFeeBumpTx()

	assert.Panics(t, func() {
		tx.FeeBumpAccount()
	})
	assert.Panics(t, func() {
		legacyTx.FeeBumpAccount()
	})

	account := feeBumpTx.FeeBumpAccount()
	assert.Equal(t, feeBumpTx.FeeBump.Tx.FeeSource, account)
}

func TestFeeBumpFee(t *testing.T) {
	legacyTx := createLegacyTx()
	tx := createTx()
	feeBumpTx := createFeeBumpTx()

	assert.Panics(t, func() {
		tx.FeeBumpFee()
	})
	assert.Panics(t, func() {
		legacyTx.FeeBumpFee()
	})

	fee := feeBumpTx.FeeBumpFee()
	assert.Equal(t, int64(feeBumpTx.FeeBump.Tx.Fee), fee)
}

func TestFeeBumpSignatures(t *testing.T) {
	legacyTx := createLegacyTx()
	tx := createTx()
	feeBumpTx := createFeeBumpTx()

	assert.Panics(t, func() {
		tx.FeeBumpSignatures()
	})
	assert.Panics(t, func() {
		legacyTx.FeeBumpSignatures()
	})

	sigs := feeBumpTx.FeeBumpSignatures()
	assert.Equal(t, feeBumpTx.FeeBump.Signatures, sigs)
}

func TestSourceAccount(t *testing.T) {
	legacyTx := createLegacyTx()
	tx := createTx()
	feeBumpTx := createFeeBumpTx()

	assert.Equal(
		t,
		CryptoKeyTypeKeyTypeEd25519,
		legacyTx.SourceAccount().Type,
	)
	sourceAccount := legacyTx.SourceAccount()
	assert.Equal(
		t,
		legacyTx.V0.Tx.SourceAccountEd25519,
		*sourceAccount.Ed25519,
	)

	assert.Equal(
		t,
		tx.V1.Tx.SourceAccount,
		tx.SourceAccount(),
	)

	assert.Equal(
		t,
		feeBumpTx.FeeBump.Tx.InnerTx.V1.Tx.SourceAccount,
		feeBumpTx.SourceAccount(),
	)
}

func TestFee(t *testing.T) {
	legacyTx := createLegacyTx()
	tx := createTx()
	feeBumpTx := createFeeBumpTx()

	assert.Equal(
		t,
		uint32(legacyTx.V0.Tx.Fee),
		legacyTx.Fee(),
	)

	assert.Equal(
		t,
		uint32(tx.V1.Tx.Fee),
		tx.Fee(),
	)

	assert.Equal(
		t,
		uint32(feeBumpTx.FeeBump.Tx.InnerTx.V1.Tx.Fee),
		feeBumpTx.Fee(),
	)
}

func TestSignatures(t *testing.T) {
	legacyTx := createLegacyTx()
	tx := createTx()
	feeBumpTx := createFeeBumpTx()

	assert.Equal(
		t,
		legacyTx.V0.Signatures,
		legacyTx.Signatures(),
	)

	assert.Equal(
		t,
		tx.V1.Signatures,
		tx.Signatures(),
	)

	assert.Equal(
		t,
		feeBumpTx.FeeBump.Tx.InnerTx.V1.Signatures,
		feeBumpTx.Signatures(),
	)
}

func TestSeqNum(t *testing.T) {
	legacyTx := createLegacyTx()
	tx := createTx()
	feeBumpTx := createFeeBumpTx()

	assert.Equal(
		t,
		int64(legacyTx.V0.Tx.SeqNum),
		legacyTx.SeqNum(),
	)

	assert.Equal(
		t,
		int64(tx.V1.Tx.SeqNum),
		tx.SeqNum(),
	)

	assert.Equal(
		t,
		int64(feeBumpTx.FeeBump.Tx.InnerTx.V1.Tx.SeqNum),
		feeBumpTx.SeqNum(),
	)
}

func TestPreconditions(t *testing.T) {
	t.Run("v0", func(t *testing.T) {
		legacyTx := createLegacyTx()

		assert.Equal(
			t,
			legacyTx.V0.Tx.TimeBounds,
			legacyTx.TimeBounds(),
		)
	})

	t.Run("v1", func(t *testing.T) {
		tx := createTx()
		assert.Equal(
			t,
			tx.V1.Tx.Cond.Type,
			PreconditionTypePrecondTime,
		)

		assert.Equal(
			t,
			tx.V1.Tx.Cond.TimeBounds,
			tx.TimeBounds(),
		)
	})

	t.Run("feebump", func(t *testing.T) {
		feeBumpTx := createFeeBumpTx()
		assert.Equal(
			t,
			feeBumpTx.FeeBump.Tx.InnerTx.V1.Tx.Cond.Type,
			PreconditionTypePrecondTime,
		)

		assert.Equal(
			t,
			feeBumpTx.FeeBump.Tx.InnerTx.V1.Tx.Cond.TimeBounds,
			feeBumpTx.TimeBounds(),
		)
	})

	t.Run("v2", func(t *testing.T) {
		tx := createCondV2Tx()
		assert.Equal(
			t,
			tx.V1.Tx.Cond.Type,
			PreconditionTypePrecondV2,
		)

		cond := tx.V1.Tx.Cond.V2
		minSeqNum := int64(*cond.MinSeqNum)
		if assert.NotNil(t, cond) {
			assert.Equal(t, cond.TimeBounds, tx.TimeBounds())
			assert.Equal(t, cond.LedgerBounds, tx.LedgerBounds())
			assert.Equal(t, &minSeqNum, tx.MinSeqNum())
			assert.Equal(t, &cond.MinSeqAge, tx.MinSeqAge())
			assert.Equal(t, &cond.MinSeqLedgerGap, tx.MinSeqLedgerGap())
			assert.Equal(t, cond.ExtraSigners, tx.ExtraSigners())
		}
	})
}

func TestOperations(t *testing.T) {
	legacyTx := createLegacyTx()
	tx := createTx()
	feeBumpTx := createFeeBumpTx()

	assert.Equal(
		t,
		legacyTx.V0.Tx.Operations,
		legacyTx.Operations(),
	)

	assert.Equal(
		t,
		tx.V1.Tx.Operations,
		tx.Operations(),
	)

	assert.Equal(
		t,
		feeBumpTx.FeeBump.Tx.InnerTx.V1.Tx.Operations,
		feeBumpTx.Operations(),
	)
}

func TestMemo(t *testing.T) {
	legacyTx := createLegacyTx()
	tx := createTx()
	feeBumpTx := createFeeBumpTx()

	assert.Equal(
		t,
		legacyTx.V0.Tx.Memo,
		legacyTx.Memo(),
	)

	assert.Equal(
		t,
		tx.V1.Tx.Memo,
		tx.Memo(),
	)

	assert.Equal(
		t,
		feeBumpTx.FeeBump.Tx.InnerTx.V1.Tx.Memo,
		feeBumpTx.Memo(),
	)
}
