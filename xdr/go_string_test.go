package xdr_test

import (
	"fmt"
	"testing"

	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
)

func Uint32Ptr(val uint32) *xdr.Uint32 {
	pval := xdr.Uint32(val)
	return &pval
}

func TestAssetGoStringerNative(t *testing.T) {
	asset, err := xdr.NewAsset(xdr.AssetTypeAssetTypeNative, nil)
	assert.NoError(t, err)
	assert.Equal(t, "xdr.MustNewNativeAsset()", fmt.Sprintf("%#v", asset))
}

func TestAssetGoStringerCredit(t *testing.T) {
	asset, err := xdr.BuildAsset("credit_alphanum4", "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H", "USD")
	assert.NoError(t, err)
	assert.Equal(
		t,
		`xdr.MustNewCreditAsset("USD", "GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")`,
		fmt.Sprintf("%#v", asset),
	)
}

func TestMemoGoStringerNone(t *testing.T) {
	memo := xdr.Memo{Type: xdr.MemoTypeMemoNone}
	assert.Equal(
		t,
		`xdr.Memo{Type: xdr.MemoTypeMemoNone}`,
		fmt.Sprintf("%#v", memo),
	)
}

func TestMemoGoStringerText(t *testing.T) {
	text := "abc"
	memo := xdr.Memo{Type: xdr.MemoTypeMemoText, Text: &text}
	assert.Equal(t, `xdr.MemoText("abc")`, fmt.Sprintf("%#v", memo))
}

func TestMemoGoStringerID(t *testing.T) {
	id := xdr.Uint64(123)
	memo := xdr.Memo{Type: xdr.MemoTypeMemoId, Id: &id}
	assert.Equal(t, `xdr.MemoID(123)`, fmt.Sprintf("%#v", memo))
}

func TestMemoGoStringerHash(t *testing.T) {
	hash := xdr.Hash{0x7b}
	memo := xdr.Memo{Type: xdr.MemoTypeMemoHash, Hash: &hash}
	assert.Equal(
		t,
		`xdr.MemoHash(xdr.Hash{0x7b, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0})`,
		fmt.Sprintf("%#v", memo),
	)
}

func TestMemoGoStringerRetHash(t *testing.T) {
	hash := xdr.Hash{0x7b}
	memo := xdr.Memo{Type: xdr.MemoTypeMemoReturn, RetHash: &hash}
	assert.Equal(
		t,
		`xdr.MemoRetHash(xdr.Hash{0x7b, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0})`,
		fmt.Sprintf("%#v", memo),
	)
}

func TestOperationGoStringerSource(t *testing.T) {
	operation := xdr.Operation{
		SourceAccount: xdr.MustMuxedAddressPtr("GC7ERFCD7QLDFRSEPLYB3GYSWX6GYMCHLDL45N4S5Q2N5EJDOMOJ63V4"),
		Body: xdr.OperationBody{
			Type: xdr.OperationTypeManageBuyOffer,
			ManageBuyOfferOp: &xdr.ManageBuyOfferOp{
				Selling:   xdr.MustNewNativeAsset(),
				Buying:    xdr.MustNewCreditAsset("USD", "GB2O5PBQJDAFCNM2U2DIMVAEI7ISOYL4UJDTLN42JYYXAENKBWY6OBKZ"),
				BuyAmount: 19995825,
				Price:     xdr.Price{N: 524087, D: 5000000},
				OfferId:   258020376,
			},
		},
	}
	assert.Equal(
		t,
		`xdr.Operation{SourceAccount: xdr.MustMuxedAddressPtr("GC7ERFCD7QLDFRSEPLYB3GYSWX6GYMCHLDL45N4S5Q2N5EJDOMOJ63V4"),Body: xdr.OperationBody{Type: xdr.OperationTypeManageBuyOffer,ManageBuyOfferOp: &xdr.ManageBuyOfferOp{Selling:xdr.MustNewNativeAsset(), Buying:xdr.MustNewCreditAsset("USD", "GB2O5PBQJDAFCNM2U2DIMVAEI7ISOYL4UJDTLN42JYYXAENKBWY6OBKZ"), BuyAmount:19995825, Price:xdr.Price{N:524087, D:5000000}, OfferId:258020376}}}`,
		fmt.Sprintf("%#v", operation),
	)
}

func TestOperationBodyGoStringerCreateAccount(t *testing.T) {
	operationBody := xdr.OperationBody{
		Type: xdr.OperationTypeCreateAccount,
		CreateAccountOp: &xdr.CreateAccountOp{
			Destination:     xdr.MustAddress("GC7ERFCD7QLDFRSEPLYB3GYSWX6GYMCHLDL45N4S5Q2N5EJDOMOJ63V4"),
			StartingBalance: 19995825,
		},
	}

	assert.Equal(
		t,
		`xdr.OperationBody{Type: xdr.OperationTypeCreateAccount,CreateAccountOp: &xdr.CreateAccountOp{Destination:xdr.MustAddress("GC7ERFCD7QLDFRSEPLYB3GYSWX6GYMCHLDL45N4S5Q2N5EJDOMOJ63V4"), StartingBalance:19995825}}`,
		fmt.Sprintf("%#v", operationBody),
	)
}

func TestOperationBodyGoStringerSetOptions(t *testing.T) {
	operationBody := xdr.OperationBody{
		Type: xdr.OperationTypeSetOptions,
		SetOptionsOp: &xdr.SetOptionsOp{
			InflationDest: xdr.MustAddressPtr("GC7ERFCD7QLDFRSEPLYB3GYSWX6GYMCHLDL45N4S5Q2N5EJDOMOJ63V4"),
			ClearFlags:    Uint32Ptr(0),
			SetFlags:      Uint32Ptr(1),
			MasterWeight:  Uint32Ptr(2),
			LowThreshold:  Uint32Ptr(3),
			MedThreshold:  Uint32Ptr(4),
			HighThreshold: Uint32Ptr(5),
			HomeDomain:    xdr.String32Ptr("stellar.org"),
			Signer: &xdr.Signer{
				Key:    xdr.MustSigner("GC7ERFCD7QLDFRSEPLYB3GYSWX6GYMCHLDL45N4S5Q2N5EJDOMOJ63V4"),
				Weight: 6,
			},
		},
	}

	assert.Equal(
		t,
		`xdr.OperationBody{Type: xdr.OperationTypeSetOptions,SetOptionsOp: &xdr.SetOptionsOp{InflationDest: xdr.MustAddressPtr("GC7ERFCD7QLDFRSEPLYB3GYSWX6GYMCHLDL45N4S5Q2N5EJDOMOJ63V4"),ClearFlags: xdr.Uint32Ptr(0),SetFlags: xdr.Uint32Ptr(1),MasterWeight: xdr.Uint32Ptr(2),LowThreshold: xdr.Uint32Ptr(3),MedThreshold: xdr.Uint32Ptr(4),HighThreshold: xdr.Uint32Ptr(5),HomeDomain: xdr.String32Ptr("stellar.org"),Signer: &xdr.Signer{Key: xdr.MustSigner("GC7ERFCD7QLDFRSEPLYB3GYSWX6GYMCHLDL45N4S5Q2N5EJDOMOJ63V4"),Weight: 6},}}`,
		fmt.Sprintf("%#v", operationBody),
	)
}

func TestOperationBodyGoStringerManageData(t *testing.T) {
	operationBody := xdr.OperationBody{
		Type: xdr.OperationTypeManageData,
		ManageDataOp: &xdr.ManageDataOp{
			DataName:  "abc",
			DataValue: &xdr.DataValue{0xa, 0xb},
		},
	}

	assert.Equal(
		t,
		`xdr.OperationBody{Type: xdr.OperationTypeManageData,ManageDataOp: &xdr.ManageDataOp{DataName: "abc",DataValue: &xdr.DataValue{0xa, 0xb}}}`,
		fmt.Sprintf("%#v", operationBody),
	)
}

func TestOperationBodyGoStringerAccountMerge(t *testing.T) {
	operationBody := xdr.OperationBody{
		Type:        xdr.OperationTypeAccountMerge,
		Destination: xdr.MustMuxedAddressPtr("GC7ERFCD7QLDFRSEPLYB3GYSWX6GYMCHLDL45N4S5Q2N5EJDOMOJ63V4"),
	}

	assert.Equal(
		t,
		`xdr.OperationBody{Type: xdr.OperationTypeAccountMerge,Destination: xdr.MustMuxedAddress("GC7ERFCD7QLDFRSEPLYB3GYSWX6GYMCHLDL45N4S5Q2N5EJDOMOJ63V4")}`,
		fmt.Sprintf("%#v", operationBody),
	)
}

func TestOperationBodyGoStringerAllowTrust(t *testing.T) {
	operationBody := xdr.OperationBody{
		Type: xdr.OperationTypeAllowTrust,
		AllowTrustOp: &xdr.AllowTrustOp{
			Trustor:   xdr.MustAddress("GC7ERFCD7QLDFRSEPLYB3GYSWX6GYMCHLDL45N4S5Q2N5EJDOMOJ63V4"),
			Asset:     xdr.MustNewAssetCodeFromString("USD"),
			Authorize: 1,
		},
	}

	assert.Equal(
		t,
		`xdr.OperationBody{Type: xdr.OperationTypeAllowTrust,AllowTrustOp: &xdr.AllowTrustOp{Trustor:xdr.MustAddress("GC7ERFCD7QLDFRSEPLYB3GYSWX6GYMCHLDL45N4S5Q2N5EJDOMOJ63V4"), Asset:xdr.MustNewAssetCodeFromString("USD"), Authorize:1}}`,
		fmt.Sprintf("%#v", operationBody),
	)
}

func TestTransactionEnvelopeGoStringerV1(t *testing.T) {
	envelope := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{
				SourceAccount: xdr.MustMuxedAddress("GC7ERFCD7QLDFRSEPLYB3GYSWX6GYMCHLDL45N4S5Q2N5EJDOMOJ63V4"),
				Fee:           100,
				SeqNum:        99284448289310326,
				Cond: xdr.Preconditions{
					Type: xdr.PreconditionTypePrecondTime,
					TimeBounds: &xdr.TimeBounds{
						MinTime: xdr.TimePoint(0),
						MaxTime: xdr.TimePoint(0),
					},
				},
				Memo: xdr.Memo{Type: xdr.MemoTypeMemoNone},
				Operations: []xdr.Operation{
					{
						Body: xdr.OperationBody{
							Type: xdr.OperationTypeManageSellOffer,
							ManageSellOfferOp: &xdr.ManageSellOfferOp{
								Selling: xdr.MustNewNativeAsset(),
								Buying:  xdr.MustNewCreditAsset("USD", "GB2O5PBQJDAFCNM2U2DIMVAEI7ISOYL4UJDTLN42JYYXAENKBWY6OBKZ"),
								Amount:  19995825,
								Price:   xdr.Price{N: 524087, D: 5000000},
								OfferId: 258020376,
							},
						},
					},
				},
				Ext: xdr.TransactionExt{V: 0}},
			Signatures: []xdr.DecoratedSignature{
				{
					Hint:      xdr.SignatureHint{0x23, 0x73, 0x1c, 0x9f},
					Signature: xdr.Signature{0x71, 0xd3, 0xfa, 0x9, 0xd9, 0x12, 0xd3, 0xcf, 0x2c, 0x6f, 0xd9, 0x29, 0x9a, 0xdd, 0xfd, 0x77, 0x84, 0xe1, 0x6, 0x4f, 0xe, 0xed, 0x9, 0x77, 0xe, 0x46, 0x9a, 0xa3, 0x59, 0xf3, 0x7, 0x16, 0xb3, 0x28, 0x4a, 0x40, 0x40, 0x98, 0x1e, 0xe1, 0xea, 0xc6, 0xa4, 0xc, 0x6e, 0x96, 0xc3, 0x1e, 0x46, 0x71, 0x4f, 0x54, 0x32, 0xc5, 0x93, 0x81, 0x7d, 0xb1, 0xa4, 0xf9, 0xa5, 0x3e, 0x33, 0x4},
				},
			},
		},
	}

	assert.Equal(
		t,
		`xdr.TransactionEnvelope{Type: xdr.EnvelopeTypeEnvelopeTypeTx,V1: &xdr.TransactionV1Envelope{Tx:xdr.Transaction{SourceAccount:xdr.MustMuxedAddress("GC7ERFCD7QLDFRSEPLYB3GYSWX6GYMCHLDL45N4S5Q2N5EJDOMOJ63V4"), Fee:100, SeqNum:99284448289310326, Cond:xdr.Preconditions{Type: xdr.PreconditionTypePrecondTime, TimeBounds: &xdr.TimeBounds{MinTime: xdr.TimePoint(0), MaxTime: xdr.TimePoint(0)}}, Memo:xdr.Memo{Type: xdr.MemoTypeMemoNone}, Operations:[]xdr.Operation{xdr.Operation{Body: xdr.OperationBody{Type: xdr.OperationTypeManageSellOffer,ManageSellOfferOp: &xdr.ManageSellOfferOp{Selling:xdr.MustNewNativeAsset(), Buying:xdr.MustNewCreditAsset("USD", "GB2O5PBQJDAFCNM2U2DIMVAEI7ISOYL4UJDTLN42JYYXAENKBWY6OBKZ"), Amount:19995825, Price:xdr.Price{N:524087, D:5000000}, OfferId:258020376}}}}, Ext:xdr.TransactionExt{V:0, SorobanData:(*xdr.SorobanTransactionData)(nil)}}, Signatures:[]xdr.DecoratedSignature{xdr.DecoratedSignature{Hint:xdr.SignatureHint{0x23, 0x73, 0x1c, 0x9f}, Signature:xdr.Signature{0x71, 0xd3, 0xfa, 0x9, 0xd9, 0x12, 0xd3, 0xcf, 0x2c, 0x6f, 0xd9, 0x29, 0x9a, 0xdd, 0xfd, 0x77, 0x84, 0xe1, 0x6, 0x4f, 0xe, 0xed, 0x9, 0x77, 0xe, 0x46, 0x9a, 0xa3, 0x59, 0xf3, 0x7, 0x16, 0xb3, 0x28, 0x4a, 0x40, 0x40, 0x98, 0x1e, 0xe1, 0xea, 0xc6, 0xa4, 0xc, 0x6e, 0x96, 0xc3, 0x1e, 0x46, 0x71, 0x4f, 0x54, 0x32, 0xc5, 0x93, 0x81, 0x7d, 0xb1, 0xa4, 0xf9, 0xa5, 0x3e, 0x33, 0x4}}}}}`,
		fmt.Sprintf("%#v", envelope),
	)
}

func TestTransactionEnvelopeGoStringerFeeBump(t *testing.T) {
	envelope := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTxFeeBump,
		FeeBump: &xdr.FeeBumpTransactionEnvelope{
			Tx: xdr.FeeBumpTransaction{
				FeeSource: xdr.MustMuxedAddress("GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"),
				Fee:       4000,
				InnerTx: xdr.FeeBumpTransactionInnerTx{
					Type: xdr.EnvelopeTypeEnvelopeTypeTx,
					V1: &xdr.TransactionV1Envelope{
						Tx: xdr.Transaction{
							SourceAccount: xdr.MustMuxedAddress("GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"),
							Fee:           0,
							SeqNum:        566862668627969,
							Cond: xdr.Preconditions{
								Type: xdr.PreconditionTypePrecondTime,
								TimeBounds: &xdr.TimeBounds{
									MinTime: xdr.TimePoint(0),
									MaxTime: xdr.TimePoint(0),
								},
							},
							Memo: xdr.MemoText("My 1st fee bump! Woohoo!"),
							Operations: []xdr.Operation{
								{
									Body: xdr.OperationBody{
										Type: xdr.OperationTypePayment,
										PaymentOp: &xdr.PaymentOp{
											Destination: xdr.MustMuxedAddress("GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"),
											Asset:       xdr.MustNewNativeAsset(),
											Amount:      1000000000,
										},
									},
								},
							},
							Ext: xdr.TransactionExt{V: 0},
						},
						Signatures: []xdr.DecoratedSignature{
							{
								Hint:      xdr.SignatureHint{0x96, 0xa1, 0x20, 0x62},
								Signature: xdr.Signature{0x5e, 0x36, 0x9, 0x6c, 0x7a, 0xa4, 0x73, 0xde, 0x20, 0xf9, 0x4f, 0x2, 0xf4, 0x9c, 0x66, 0x10, 0x42, 0x1f, 0xa1, 0x34, 0x68, 0x6b, 0xe4, 0xbf, 0xce, 0x67, 0x71, 0x3b, 0x61, 0x2c, 0x78, 0xae, 0x25, 0x66, 0xe, 0x28, 0xad, 0xe9, 0xe7, 0xb8, 0x8c, 0xf8, 0x46, 0xba, 0x98, 0x43, 0xde, 0x40, 0x27, 0xb8, 0xb4, 0x52, 0xf3, 0x70, 0xab, 0x80, 0x8b, 0xac, 0x45, 0xb, 0x1, 0xee, 0xbe, 0x6},
							},
						},
					},
				},
				Ext: xdr.FeeBumpTransactionExt{V: 0}},
			Signatures: []xdr.DecoratedSignature{
				{
					Hint:      xdr.SignatureHint{0x96, 0xa1, 0x20, 0x62},
					Signature: xdr.Signature{0xb2, 0xcc, 0x82, 0x6e, 0x9c, 0xa4, 0x3a, 0x11, 0x75, 0x33, 0xd1, 0xfd, 0xa2, 0x49, 0xc0, 0x50, 0xf1, 0xd8, 0x62, 0x7, 0xf6, 0xdf, 0x2, 0x9a, 0x46, 0xa5, 0xe8, 0x3a, 0xb7, 0xbf, 0x4b, 0xc7, 0xcb, 0xd4, 0x4f, 0xe0, 0xe5, 0x25, 0xb8, 0xe, 0xbe, 0xdc, 0x53, 0x68, 0x69, 0x19, 0xdc, 0x57, 0xf3, 0x39, 0x77, 0x71, 0xca, 0x73, 0x89, 0xa4, 0xdc, 0x2c, 0xca, 0xd4, 0x1d, 0x5f, 0x9d, 0x4},
				},
			},
		},
	}

	assert.Equal(
		t,
		`xdr.TransactionEnvelope{Type: xdr.EnvelopeTypeEnvelopeTypeTxFeeBump,FeeBump: &xdr.FeeBumpTransactionEnvelope{Tx:xdr.FeeBumpTransaction{FeeSource:xdr.MustMuxedAddress("GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"), Fee:4000, InnerTx:xdr.FeeBumpTransactionInnerTx{Type: xdr.EnvelopeTypeEnvelopeTypeTx,V1: &xdr.TransactionV1Envelope{Tx:xdr.Transaction{SourceAccount:xdr.MustMuxedAddress("GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"), Fee:0, SeqNum:566862668627969, Cond:xdr.Preconditions{Type: xdr.PreconditionTypePrecondTime, TimeBounds: &xdr.TimeBounds{MinTime: xdr.TimePoint(0), MaxTime: xdr.TimePoint(0)}}, Memo:xdr.MemoText("My 1st fee bump! Woohoo!"), Operations:[]xdr.Operation{xdr.Operation{Body: xdr.OperationBody{Type: xdr.OperationTypePayment,PaymentOp: &xdr.PaymentOp{Destination:xdr.MustMuxedAddress("GD6WNNTW664WH7FXC5RUMUTF7P5QSURC2IT36VOQEEGFZ4UWUEQGECAL"), Asset:xdr.MustNewNativeAsset(), Amount:1000000000}}}}, Ext:xdr.TransactionExt{V:0, SorobanData:(*xdr.SorobanTransactionData)(nil)}}, Signatures:[]xdr.DecoratedSignature{xdr.DecoratedSignature{Hint:xdr.SignatureHint{0x96, 0xa1, 0x20, 0x62}, Signature:xdr.Signature{0x5e, 0x36, 0x9, 0x6c, 0x7a, 0xa4, 0x73, 0xde, 0x20, 0xf9, 0x4f, 0x2, 0xf4, 0x9c, 0x66, 0x10, 0x42, 0x1f, 0xa1, 0x34, 0x68, 0x6b, 0xe4, 0xbf, 0xce, 0x67, 0x71, 0x3b, 0x61, 0x2c, 0x78, 0xae, 0x25, 0x66, 0xe, 0x28, 0xad, 0xe9, 0xe7, 0xb8, 0x8c, 0xf8, 0x46, 0xba, 0x98, 0x43, 0xde, 0x40, 0x27, 0xb8, 0xb4, 0x52, 0xf3, 0x70, 0xab, 0x80, 0x8b, 0xac, 0x45, 0xb, 0x1, 0xee, 0xbe, 0x6}}}}}, Ext:xdr.FeeBumpTransactionExt{V:0}}, Signatures:[]xdr.DecoratedSignature{xdr.DecoratedSignature{Hint:xdr.SignatureHint{0x96, 0xa1, 0x20, 0x62}, Signature:xdr.Signature{0xb2, 0xcc, 0x82, 0x6e, 0x9c, 0xa4, 0x3a, 0x11, 0x75, 0x33, 0xd1, 0xfd, 0xa2, 0x49, 0xc0, 0x50, 0xf1, 0xd8, 0x62, 0x7, 0xf6, 0xdf, 0x2, 0x9a, 0x46, 0xa5, 0xe8, 0x3a, 0xb7, 0xbf, 0x4b, 0xc7, 0xcb, 0xd4, 0x4f, 0xe0, 0xe5, 0x25, 0xb8, 0xe, 0xbe, 0xdc, 0x53, 0x68, 0x69, 0x19, 0xdc, 0x57, 0xf3, 0x39, 0x77, 0x71, 0xca, 0x73, 0x89, 0xa4, 0xdc, 0x2c, 0xca, 0xd4, 0x1d, 0x5f, 0x9d, 0x4}}}}}`,
		fmt.Sprintf("%#v", envelope),
	)
}
