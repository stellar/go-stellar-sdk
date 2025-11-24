package ingest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/xdr"
)

var (
	mockContractEvent1 = xdr.ContractEvent{
		Type: xdr.ContractEventTypeContract,
		Body: xdr.ContractEventBody{
			V0: &xdr.ContractEventV0{},
		},
	}

	mockContractEvent2 = xdr.ContractEvent{
		Type: xdr.ContractEventTypeContract,
		Body: xdr.ContractEventBody{
			V:  0,
			V0: &xdr.ContractEventV0{},
		},
	}

	mockDiagnosticEvent1 = xdr.DiagnosticEvent{
		InSuccessfulContractCall: true,
		Event:                    mockContractEvent1,
	}

	mockDiagnosticEvent2 = xdr.DiagnosticEvent{
		InSuccessfulContractCall: false,
		Event:                    mockContractEvent2,
	}

	mockTransactionEvent1 = xdr.TransactionEvent{
		Stage: xdr.TransactionEventStageTransactionEventStageBeforeAllTxs,
		Event: mockContractEvent1,
	}

	mockTransactionEvent2 = xdr.TransactionEvent{
		Stage: xdr.TransactionEventStageTransactionEventStageAfterTx,
		Event: mockContractEvent2,
	}

	someSorobanTxEnvelope = xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{
				Ext: xdr.TransactionExt{
					V:           1,
					SorobanData: &xdr.SorobanTransactionData{},
				},
			},
		},
	}

	someClassicTxEnvelope = xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{
				Ext: xdr.TransactionExt{},
			},
		},
	}
)

func TestChangeAccountChangedExceptSignersInvalidType(t *testing.T) {
	change := Change{
		Type: xdr.LedgerEntryTypeOffer,
	}

	var err error
	assert.Panics(t, func() {
		_, err = change.AccountChangedExceptSigners()
	})
	// the following is here only to avoid false-positive warning by the linter.
	require.NoError(t, err)
}

func TestFeeMetaAndOperationsChangesSeparate(t *testing.T) {
	tx := LedgerTransaction{
		LedgerVersion: 12,
		FeeChanges: xdr.LedgerEntryChanges{
			xdr.LedgerEntryChange{
				Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
				State: &xdr.LedgerEntry{
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A"),
							Balance:   100,
						},
					},
				},
			},
			xdr.LedgerEntryChange{
				Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
				Updated: &xdr.LedgerEntry{
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.MustAddress("GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A"),
							Balance:   200,
						},
					},
				},
			},
		},
		UnsafeMeta: xdr.TransactionMeta{
			V: 1,
			V1: &xdr.TransactionMetaV1{
				Operations: []xdr.OperationMeta{
					{
						Changes: xdr.LedgerEntryChanges{
							xdr.LedgerEntryChange{
								Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
								State: &xdr.LedgerEntry{
									Data: xdr.LedgerEntryData{
										Type: xdr.LedgerEntryTypeAccount,
										Account: &xdr.AccountEntry{
											AccountId: xdr.MustAddress("GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A"),
											Balance:   300,
										},
									},
								},
							},
							xdr.LedgerEntryChange{
								Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
								Updated: &xdr.LedgerEntry{
									Data: xdr.LedgerEntryData{
										Type: xdr.LedgerEntryTypeAccount,
										Account: &xdr.AccountEntry{
											AccountId: xdr.MustAddress("GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A"),
											Balance:   400,
										},
									},
								},
							},
						},
					},
				},
			},
		}}

	feeChanges := tx.GetFeeChanges()
	assert.Len(t, feeChanges, 1)
	assert.Equal(t, feeChanges[0].Pre.Data.MustAccount().Balance, xdr.Int64(100))
	assert.Equal(t, feeChanges[0].Post.Data.MustAccount().Balance, xdr.Int64(200))

	metaChanges, err := tx.GetChanges()
	assert.NoError(t, err)
	assert.Len(t, metaChanges, 1)
	assert.Equal(t, metaChanges[0].Pre.Data.MustAccount().Balance, xdr.Int64(300))
	assert.Equal(t, metaChanges[0].Post.Data.MustAccount().Balance, xdr.Int64(400))

	operationChanges, err := tx.GetOperationChanges(0)
	assert.NoError(t, err)
	assert.Len(t, operationChanges, 1)
	assert.Equal(t, operationChanges[0].Pre.Data.MustAccount().Balance, xdr.Int64(300))
	assert.Equal(t, operationChanges[0].Post.Data.MustAccount().Balance, xdr.Int64(400))

	// Ignore operation meta if tx result is txInternalError
	// https://github.com/stellar/go-stellar-sdk/issues/2111
	tx.Result.Result.Result.Code = xdr.TransactionResultCodeTxInternalError
	metaChanges, err = tx.GetChanges()
	assert.NoError(t, err)
	assert.Len(t, metaChanges, 0)

	operationChanges, err = tx.GetOperationChanges(0)
	assert.NoError(t, err)
	assert.Len(t, operationChanges, 0)

	// Starting from protocol 13, we no longer need to ignore txInternalError
	tx.LedgerVersion = 13

	metaChanges, err = tx.GetChanges()
	assert.NoError(t, err)
	assert.Len(t, metaChanges, 1)
	assert.Equal(t, metaChanges[0].Pre.Data.MustAccount().Balance, xdr.Int64(300))
	assert.Equal(t, metaChanges[0].Post.Data.MustAccount().Balance, xdr.Int64(400))

	operationChanges, err = tx.GetOperationChanges(0)
	assert.NoError(t, err)
	assert.Len(t, operationChanges, 1)
	assert.Equal(t, operationChanges[0].Pre.Data.MustAccount().Balance, xdr.Int64(300))
	assert.Equal(t, operationChanges[0].Post.Data.MustAccount().Balance, xdr.Int64(400))
}

func TestFailedTransactionOperationChangesMeta(t *testing.T) {
	testCases := []struct {
		desc string
		meta xdr.TransactionMeta
	}{
		{
			desc: "V0",
			meta: xdr.TransactionMeta{
				Operations: &[]xdr.OperationMeta{},
			},
		},
		{
			desc: "V1",
			meta: xdr.TransactionMeta{
				V:  1,
				V1: &xdr.TransactionMetaV1{},
			},
		},
		{
			desc: "V2",
			meta: xdr.TransactionMeta{
				V:  2,
				V2: &xdr.TransactionMetaV2{},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			tx := LedgerTransaction{
				Result: xdr.TransactionResultPair{
					Result: xdr.TransactionResult{
						Result: xdr.TransactionResultResult{
							Code: xdr.TransactionResultCodeTxFailed,
						},
					},
				},
				UnsafeMeta: tc.meta,
			}

			operationChanges, err := tx.GetOperationChanges(0)
			if tx.UnsafeMeta.V == 0 {
				assert.Error(t, err)
				assert.EqualError(t, err, "TransactionMeta.V=0 not supported")
			} else {
				assert.NoError(t, err)
				assert.Len(t, operationChanges, 0)
			}
		})
	}
}
func TestMetaV2Order(t *testing.T) {
	tx := LedgerTransaction{
		UnsafeMeta: xdr.TransactionMeta{
			V: 2,
			V2: &xdr.TransactionMetaV2{
				TxChangesBefore: xdr.LedgerEntryChanges{
					xdr.LedgerEntryChange{
						Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
						State: &xdr.LedgerEntry{
							Data: xdr.LedgerEntryData{
								Type: xdr.LedgerEntryTypeAccount,
								Account: &xdr.AccountEntry{
									AccountId: xdr.MustAddress("GACMZD5VJXTRLKVET72CETCYKELPNCOTTBDC6DHFEUPLG5DHEK534JQX"),
									Balance:   100,
								},
							},
						},
					},
					xdr.LedgerEntryChange{
						Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
						Updated: &xdr.LedgerEntry{
							Data: xdr.LedgerEntryData{
								Type: xdr.LedgerEntryTypeAccount,
								Account: &xdr.AccountEntry{
									AccountId: xdr.MustAddress("GACMZD5VJXTRLKVET72CETCYKELPNCOTTBDC6DHFEUPLG5DHEK534JQX"),
									Balance:   200,
								},
							},
						},
					},
					xdr.LedgerEntryChange{
						Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
						State: &xdr.LedgerEntry{
							Data: xdr.LedgerEntryData{
								Type: xdr.LedgerEntryTypeAccount,
								Account: &xdr.AccountEntry{
									AccountId: xdr.MustAddress("GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A"),
									Balance:   100,
								},
							},
						},
					},
					xdr.LedgerEntryChange{
						Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
						Updated: &xdr.LedgerEntry{
							Data: xdr.LedgerEntryData{
								Type: xdr.LedgerEntryTypeAccount,
								Account: &xdr.AccountEntry{
									AccountId: xdr.MustAddress("GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A"),
									Balance:   200,
								},
							},
						},
					},
				},
				Operations: []xdr.OperationMeta{
					{
						Changes: xdr.LedgerEntryChanges{
							xdr.LedgerEntryChange{
								Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
								State: &xdr.LedgerEntry{
									Data: xdr.LedgerEntryData{
										Type: xdr.LedgerEntryTypeAccount,
										Account: &xdr.AccountEntry{
											AccountId: xdr.MustAddress("GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A"),
											Balance:   300,
										},
									},
								},
							},
							xdr.LedgerEntryChange{
								Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
								Updated: &xdr.LedgerEntry{
									Data: xdr.LedgerEntryData{
										Type: xdr.LedgerEntryTypeAccount,
										Account: &xdr.AccountEntry{
											AccountId: xdr.MustAddress("GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A"),
											Balance:   400,
										},
									},
								},
							},
						},
					},
				},
				TxChangesAfter: xdr.LedgerEntryChanges{
					xdr.LedgerEntryChange{
						Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
						State: &xdr.LedgerEntry{
							Data: xdr.LedgerEntryData{
								Type: xdr.LedgerEntryTypeAccount,
								Account: &xdr.AccountEntry{
									AccountId: xdr.MustAddress("GACMZD5VJXTRLKVET72CETCYKELPNCOTTBDC6DHFEUPLG5DHEK534JQX"),
									Balance:   300,
								},
							},
						},
					},
					xdr.LedgerEntryChange{
						Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
						Updated: &xdr.LedgerEntry{
							Data: xdr.LedgerEntryData{
								Type: xdr.LedgerEntryTypeAccount,
								Account: &xdr.AccountEntry{
									AccountId: xdr.MustAddress("GACMZD5VJXTRLKVET72CETCYKELPNCOTTBDC6DHFEUPLG5DHEK534JQX"),
									Balance:   400,
								},
							},
						},
					},
				},
			},
		}}

	metaChanges, err := tx.GetChanges()
	assert.NoError(t, err)
	assert.Len(t, metaChanges, 4)

	change := metaChanges[0]
	id := change.Pre.Data.MustAccount().AccountId
	assert.Equal(t, id.Address(), "GACMZD5VJXTRLKVET72CETCYKELPNCOTTBDC6DHFEUPLG5DHEK534JQX")
	assert.Equal(t, change.Pre.Data.MustAccount().Balance, xdr.Int64(100))
	assert.Equal(t, change.Post.Data.MustAccount().Balance, xdr.Int64(200))

	change = metaChanges[1]
	id = change.Pre.Data.MustAccount().AccountId
	assert.Equal(t, id.Address(), "GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A")
	assert.Equal(t, change.Pre.Data.MustAccount().Balance, xdr.Int64(100))
	assert.Equal(t, change.Post.Data.MustAccount().Balance, xdr.Int64(200))

	change = metaChanges[2]
	id = change.Pre.Data.MustAccount().AccountId
	assert.Equal(t, id.Address(), "GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A")
	assert.Equal(t, change.Pre.Data.MustAccount().Balance, xdr.Int64(300))
	assert.Equal(t, change.Post.Data.MustAccount().Balance, xdr.Int64(400))

	change = metaChanges[3]
	id = change.Pre.Data.MustAccount().AccountId
	assert.Equal(t, id.Address(), "GACMZD5VJXTRLKVET72CETCYKELPNCOTTBDC6DHFEUPLG5DHEK534JQX")
	assert.Equal(t, change.Pre.Data.MustAccount().Balance, xdr.Int64(300))
	assert.Equal(t, change.Post.Data.MustAccount().Balance, xdr.Int64(400))

	operationChanges, err := tx.GetOperationChanges(0)
	assert.NoError(t, err)
	assert.Len(t, operationChanges, 1)

	// Ignore operations meta and txChangesAfter if txInternalError
	// https://github.com/stellar/go-stellar-sdk/issues/2111
	tx.Result.Result.Result.Code = xdr.TransactionResultCodeTxInternalError
	metaChanges, err = tx.GetChanges()
	assert.NoError(t, err)
	assert.Len(t, metaChanges, 2)

	change = metaChanges[0]
	id = change.Pre.Data.MustAccount().AccountId
	assert.Equal(t, id.Address(), "GACMZD5VJXTRLKVET72CETCYKELPNCOTTBDC6DHFEUPLG5DHEK534JQX")
	assert.Equal(t, change.Pre.Data.MustAccount().Balance, xdr.Int64(100))
	assert.Equal(t, change.Post.Data.MustAccount().Balance, xdr.Int64(200))

	change = metaChanges[1]
	id = change.Pre.Data.MustAccount().AccountId
	assert.Equal(t, id.Address(), "GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A")
	assert.Equal(t, change.Pre.Data.MustAccount().Balance, xdr.Int64(100))
	assert.Equal(t, change.Post.Data.MustAccount().Balance, xdr.Int64(200))

	operationChanges, err = tx.GetOperationChanges(0)
	assert.NoError(t, err)
	assert.Len(t, operationChanges, 0)

}

func TestMetaV0(t *testing.T) {
	tx := LedgerTransaction{
		UnsafeMeta: xdr.TransactionMeta{
			V: 0,
		}}

	_, err := tx.GetChanges()
	assert.Error(t, err)
	assert.EqualError(t, err, "TransactionMeta.V=0 not supported")
}

func TestChangeAccountChangedExceptSignersLastModifiedLedgerSeq(t *testing.T) {
	change := Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: 10,
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
				},
			},
		},
		Post: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: 11,
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
				},
			},
		},
	}
	changed, err := change.AccountChangedExceptSigners()
	assert.NoError(t, err)
	assert.True(t, changed)
}

func TestChangeAccountChangedExceptSignersNoPre(t *testing.T) {
	change := Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre:  nil,
		Post: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: 10,
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
				},
			},
		},
	}
	changed, err := change.AccountChangedExceptSigners()
	assert.NoError(t, err)
	assert.True(t, changed)
}

func TestChangeAccountChangedExceptSignersNoPost(t *testing.T) {
	change := Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: 10,
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
				},
			},
		},
		Post: nil,
	}
	changed, err := change.AccountChangedExceptSigners()
	assert.NoError(t, err)
	assert.True(t, changed)
}

func TestChangeAccountChangedExceptSignersMasterKeyRemoved(t *testing.T) {
	change := Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: 10,
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					// Master weight = 1
					Thresholds: [4]byte{1, 1, 1, 1},
				},
			},
		},
		Post: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: 10,
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					// Master weight = 0
					Thresholds: [4]byte{0, 1, 1, 1},
				},
			},
		},
	}

	changed, err := change.AccountChangedExceptSigners()
	assert.NoError(t, err)
	assert.True(t, changed)
}

func TestChangeAccountChangedExceptSignersSignerChange(t *testing.T) {
	change := Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: 10,
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Signers: []xdr.Signer{
						{
							Key:    xdr.MustSigner("GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"),
							Weight: 1,
						},
					},
				},
			},
		},
		Post: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: 10,
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Signers: []xdr.Signer{
						{
							Key:    xdr.MustSigner("GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"),
							Weight: 2,
						},
					},
				},
			},
		},
	}

	changed, err := change.AccountChangedExceptSigners()
	assert.NoError(t, err)
	assert.False(t, changed)
}

func TestChangeAccountChangedExceptSignersNoChanges(t *testing.T) {
	inflationDest := xdr.MustAddress("GBAH2GBLJB54JAROJ3FVO4ZTTJJI3XKOBTMJOZFUJ3UHYIVNJTLJUYFY")
	change := Change{
		Type: xdr.LedgerEntryTypeAccount,
		Pre: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: 10,
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId:     xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Balance:       1000,
					SeqNum:        432894732,
					NumSubEntries: 2,
					InflationDest: &inflationDest,
					Flags:         4,
					HomeDomain:    "stellar.org",
					Thresholds:    [4]byte{1, 1, 1, 1},
					Signers: []xdr.Signer{
						{
							Key:    xdr.MustSigner("GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"),
							Weight: 1,
						},
					},
					Ext: xdr.AccountEntryExt{
						V: 1,
						V1: &xdr.AccountEntryExtensionV1{
							Liabilities: xdr.Liabilities{
								Buying:  10,
								Selling: 20,
							},
						},
					},
				},
			},
		},
		Post: &xdr.LedgerEntry{
			LastModifiedLedgerSeq: 10,
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId:     xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
					Balance:       1000,
					SeqNum:        432894732,
					NumSubEntries: 2,
					InflationDest: &inflationDest,
					Flags:         4,
					HomeDomain:    "stellar.org",
					Thresholds:    [4]byte{1, 1, 1, 1},
					Signers: []xdr.Signer{
						{
							Key:    xdr.MustSigner("GCCCU34WDY2RATQTOOQKY6SZWU6J5DONY42SWGW2CIXGW4LICAGNRZKX"),
							Weight: 1,
						},
					},
					Ext: xdr.AccountEntryExt{
						V: 1,
						V1: &xdr.AccountEntryExtensionV1{
							Liabilities: xdr.Liabilities{
								Buying:  10,
								Selling: 20,
							},
						},
					},
				},
			},
		},
	}

	changed, err := change.AccountChangedExceptSigners()
	assert.NoError(t, err)
	assert.False(t, changed)

	// Make sure pre and post not modified
	assert.NotNil(t, change.Pre.Data.Account.Signers)
	assert.Len(t, change.Pre.Data.Account.Signers, 1)

	assert.NotNil(t, change.Post.Data.Account.Signers)
	assert.Len(t, change.Post.Data.Account.Signers, 1)
}

func TestTransactionHelperFunctions(t *testing.T) {
	transaction := transactionHelperFunctionsTestInput()

	assert.Equal(t, int64(131335723340009472), transaction.ID())

	var err error
	var ok bool
	var account string
	account, err = transaction.Account()
	assert.Equal(t, nil, err)
	assert.Equal(t, "GAISEMYAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAABCAK", account)

	assert.Equal(t, int64(30578981), transaction.AccountSequence())
	assert.Equal(t, uint32(4560), transaction.MaxFee())

	var feeCharged int64
	feeCharged, ok = transaction.FeeCharged()
	assert.Equal(t, true, ok)
	assert.Equal(t, int64(789), feeCharged)

	assert.Equal(t, uint32(3), transaction.OperationCount())
	assert.Equal(t, "test memo", transaction.Memo())
	assert.Equal(t, "MemoTypeMemoText", transaction.MemoType())

	var timeBounds string
	timeBounds, ok = transaction.TimeBounds()
	assert.Equal(t, true, ok)
	assert.Equal(t, "[1,10)", timeBounds)

	var ledgerBounds string
	ledgerBounds, ok = transaction.LedgerBounds()
	assert.Equal(t, true, ok)
	assert.Equal(t, "[2,20)", ledgerBounds)

	var minSequence int64
	minSequence, ok = transaction.MinSequence()
	assert.Equal(t, true, ok)
	assert.Equal(t, int64(123), minSequence)

	var minSequenceAge int64
	minSequenceAge, ok = transaction.MinSequenceAge()
	assert.Equal(t, true, ok)
	assert.Equal(t, int64(456), minSequenceAge)

	var minSequenceLedgerGap int64
	minSequenceLedgerGap, ok = transaction.MinSequenceLedgerGap()
	assert.Equal(t, true, ok)
	assert.Equal(t, int64(789), minSequenceLedgerGap)

	var sorobanResourceFee int64
	sorobanResourceFee, ok = transaction.SorobanResourceFee()
	assert.Equal(t, true, ok)
	assert.Equal(t, int64(1234), sorobanResourceFee)

	var sorobanResourcesInstructions uint32
	sorobanResourcesInstructions, ok = transaction.SorobanResourcesInstructions()
	assert.Equal(t, true, ok)
	assert.Equal(t, uint32(123), sorobanResourcesInstructions)

	var sorobanResourcesDiskReadBytes uint32
	sorobanResourcesDiskReadBytes, ok = transaction.SorobanResourcesDiskReadBytes()
	assert.Equal(t, true, ok)
	assert.Equal(t, uint32(456), sorobanResourcesDiskReadBytes)

	var sorobanResourcesWriteBytes uint32
	sorobanResourcesWriteBytes, ok = transaction.SorobanResourcesWriteBytes()
	assert.Equal(t, true, ok)
	assert.Equal(t, uint32(789), sorobanResourcesWriteBytes)

	var inclusionFeeBid int64
	inclusionFeeBid, ok = transaction.SorobanInclusionFeeBid()
	assert.Equal(t, true, ok)
	assert.Equal(t, int64(3326), inclusionFeeBid)

	var sorobanInclusionFeeCharged int64
	sorobanInclusionFeeCharged, ok = transaction.SorobanInclusionFeeCharged()
	assert.Equal(t, true, ok)
	assert.Equal(t, int64(-1234), sorobanInclusionFeeCharged)

	var inclusionFee int64
	inclusionFee, ok = transaction.InclusionFeeCharged()
	assert.Equal(t, true, ok)
	assert.Equal(t, int64(-1234), inclusionFee)

	var sorobanResourceFeeRefund int64
	sorobanResourceFeeRefund = transaction.SorobanResourceFeeRefund()
	assert.Equal(t, int64(0), sorobanResourceFeeRefund)

	var sorobanTotalNonRefundableResourceFeeCharged int64
	sorobanTotalNonRefundableResourceFeeCharged, ok = transaction.SorobanTotalNonRefundableResourceFeeCharged()
	assert.Equal(t, true, ok)
	assert.Equal(t, int64(321), sorobanTotalNonRefundableResourceFeeCharged)

	var sorobanTotalRefundableResourceFeeCharged int64
	sorobanTotalRefundableResourceFeeCharged, ok = transaction.SorobanTotalRefundableResourceFeeCharged()
	assert.Equal(t, true, ok)
	assert.Equal(t, int64(123), sorobanTotalRefundableResourceFeeCharged)

	var sorobanRentFeeCharged int64
	sorobanRentFeeCharged, ok = transaction.SorobanRentFeeCharged()
	assert.Equal(t, true, ok)
	assert.Equal(t, int64(456), sorobanRentFeeCharged)

	assert.Equal(t, "TransactionResultCodeTxSuccess", transaction.ResultCode())

	var signers []string
	signers, err = transaction.Signers()
	assert.Equal(t, nil, err)
	assert.Equal(t, []string{"GAISFR7R"}, signers)

	var accountMuxed string
	accountMuxed, ok = transaction.AccountMuxed()
	assert.Equal(t, true, ok)
	assert.Equal(t, "MAISEMYAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAPMJ2I", accountMuxed)

	var innerTransactionHash string
	innerTransactionHash, ok = transaction.InnerTransactionHash()
	assert.Equal(t, false, ok)
	assert.Equal(t, "", innerTransactionHash)

	var newMaxFee uint32
	newMaxFee, ok = transaction.NewMaxFee()
	assert.Equal(t, false, ok)
	assert.Equal(t, uint32(0), newMaxFee)

	assert.Equal(t, true, transaction.Successful())
}

func transactionHelperFunctionsTestInput() LedgerTransaction {
	ed25519 := xdr.Uint256([32]byte{0x11, 0x22, 0x33})
	muxedAccount := xdr.MuxedAccount{
		Type:    256,
		Ed25519: &ed25519,
		Med25519: &xdr.MuxedAccountMed25519{
			Id:      xdr.Uint64(123),
			Ed25519: ed25519,
		},
	}

	memoText := "test memo"
	minSeqNum := xdr.SequenceNumber(123)

	transaction := LedgerTransaction{
		Index: 1,
		Envelope: xdr.TransactionEnvelope{
			Type: xdr.EnvelopeTypeEnvelopeTypeTx,
			V1: &xdr.TransactionV1Envelope{
				Signatures: []xdr.DecoratedSignature{
					{
						Signature: []byte{0x11, 0x22},
					},
				},
				Tx: xdr.Transaction{
					SourceAccount: muxedAccount,
					SeqNum:        xdr.SequenceNumber(30578981),
					Fee:           xdr.Uint32(4560),
					Operations: []xdr.Operation{
						{
							SourceAccount: &muxedAccount,
							Body:          xdr.OperationBody{},
						},
						{
							SourceAccount: &muxedAccount,
							Body:          xdr.OperationBody{},
						},
						{
							SourceAccount: &muxedAccount,
							Body:          xdr.OperationBody{},
						},
					},
					Memo: xdr.Memo{
						Type: xdr.MemoTypeMemoText,
						Text: &memoText,
					},
					Cond: xdr.Preconditions{
						Type: 2,
						V2: &xdr.PreconditionsV2{
							TimeBounds: &xdr.TimeBounds{
								MinTime: xdr.TimePoint(1),
								MaxTime: xdr.TimePoint(10),
							},
							LedgerBounds: &xdr.LedgerBounds{
								MinLedger: 2,
								MaxLedger: 20,
							},
							MinSeqNum:       &minSeqNum,
							MinSeqAge:       456,
							MinSeqLedgerGap: 789,
						},
					},
					Ext: xdr.TransactionExt{
						V: 1,
						SorobanData: &xdr.SorobanTransactionData{
							Resources: xdr.SorobanResources{
								Instructions:  123,
								DiskReadBytes: 456,
								WriteBytes:    789,
							},
							ResourceFee: 1234,
						},
					},
				},
			},
		},
		Result: xdr.TransactionResultPair{
			TransactionHash: xdr.Hash{0x11, 0x22, 0x33},
			Result: xdr.TransactionResult{
				FeeCharged: xdr.Int64(789),
				Result: xdr.TransactionResultResult{
					Code: 0,
				},
			},
		},
		FeeChanges: xdr.LedgerEntryChanges{
			{
				Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
				State: &xdr.LedgerEntry{
					Data: xdr.LedgerEntryData{
						Type: xdr.LedgerEntryTypeAccount,
						Account: &xdr.AccountEntry{
							AccountId: xdr.AccountId{
								Type:    0,
								Ed25519: &ed25519,
							},
							Balance: 1000,
						},
					},
				},
			},
			{},
		},
		UnsafeMeta: xdr.TransactionMeta{
			V: 3,
			V3: &xdr.TransactionMetaV3{
				TxChangesAfter: xdr.LedgerEntryChanges{},
				SorobanMeta: &xdr.SorobanTransactionMeta{
					Ext: xdr.SorobanTransactionMetaExt{
						V: 1,
						V1: &xdr.SorobanTransactionMetaExtV1{
							TotalNonRefundableResourceFeeCharged: 321,
							TotalRefundableResourceFeeCharged:    123,
							RentFeeCharged:                       456,
						},
					},
				},
			},
		},
		LedgerVersion: 22,
		Ledger: xdr.LedgerCloseMeta{
			V: 1,
			V1: &xdr.LedgerCloseMetaV1{
				LedgerHeader: xdr.LedgerHeaderHistoryEntry{
					Header: xdr.LedgerHeader{
						LedgerSeq:     30578981,
						LedgerVersion: 22,
					},
				},
			},
		},
		Hash: xdr.Hash{},
	}

	return transaction
}

// Events tests

func TestGetContractEventsForOperation(t *testing.T) {
	testCases := []struct {
		name           string
		txMeta         xdr.TransactionMeta
		opIndex        uint32
		expectedEvents []xdr.ContractEvent
		expectedError  string
	}{
		{
			name:           "V1 transaction meta should return nil events",
			txMeta:         xdr.TransactionMeta{V: 1},
			opIndex:        0,
			expectedEvents: nil,
		},
		{
			name:           "V2 transaction meta should return nil events",
			txMeta:         xdr.TransactionMeta{V: 2},
			opIndex:        0,
			expectedEvents: nil,
		},
		{
			name: "V3 soroban transaction should return events from SorobanMeta",
			txMeta: xdr.TransactionMeta{
				V: 3,
				V3: &xdr.TransactionMetaV3{
					SorobanMeta: &xdr.SorobanTransactionMeta{
						Events: []xdr.ContractEvent{mockContractEvent1, mockContractEvent2},
					},
				},
			},
			opIndex:        0, // opIndex ignored in V3
			expectedEvents: []xdr.ContractEvent{mockContractEvent1, mockContractEvent2},
		},
		{
			name: "V3 soroban transaction with no events should return empty slice",
			txMeta: xdr.TransactionMeta{
				V: 3,
				V3: &xdr.TransactionMetaV3{
					SorobanMeta: &xdr.SorobanTransactionMeta{
						Events: []xdr.ContractEvent{},
					},
				},
			},
			opIndex:        0,
			expectedEvents: []xdr.ContractEvent{},
		},
		{
			name: "V4 transaction should return events from specific operation",
			txMeta: xdr.TransactionMeta{
				V: 4,
				V4: &xdr.TransactionMetaV4{
					Operations: []xdr.OperationMetaV2{
						{
							Events: []xdr.ContractEvent{mockContractEvent1},
						},
					},
				},
			},
			opIndex:        0,
			expectedEvents: []xdr.ContractEvent{mockContractEvent1},
		},
		{
			name: "V4 transaction should return events from specified operation index",
			txMeta: xdr.TransactionMeta{
				V: 4,
				V4: &xdr.TransactionMetaV4{
					Operations: []xdr.OperationMetaV2{
						{
							Events: []xdr.ContractEvent{},
						},
						{
							Events: []xdr.ContractEvent{mockContractEvent1, mockContractEvent2},
						},
					},
				},
			},
			opIndex:        1,
			expectedEvents: []xdr.ContractEvent{mockContractEvent1, mockContractEvent2},
		},
		{
			name: "V4 transaction with no events should return empty slice",
			txMeta: xdr.TransactionMeta{
				V: 4,
				V4: &xdr.TransactionMetaV4{
					Operations: []xdr.OperationMetaV2{
						{
							Events: []xdr.ContractEvent{},
						},
					},
				},
			},
			opIndex:        0,
			expectedEvents: []xdr.ContractEvent{},
		},
		{
			name:          "Unsupported version should return error",
			txMeta:        xdr.TransactionMeta{V: 5},
			opIndex:       0,
			expectedError: "unsupported TransactionMeta version: 5",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tx := &LedgerTransaction{
				UnsafeMeta: tc.txMeta,
			}

			events, err := tx.GetContractEventsForOperation(tc.opIndex)

			if tc.expectedError != "" {
				require.Error(t, err, "Expected error for: %s", tc.name)
				assert.Contains(t, err.Error(), tc.expectedError, "Error message mismatch for: %s", tc.name)
				assert.Nil(t, events, "Events should be nil when error expected for: %s", tc.name)
			} else {
				require.NoError(t, err, "Unexpected error for: %s", tc.name)
				assert.Equal(t, tc.expectedEvents, events, "Events mismatch for: %s", tc.name)
			}
		})
	}
}

func TestGetSorobanContractEvents(t *testing.T) {
	testCases := []struct {
		name           string
		envelope       xdr.TransactionEnvelope
		txMeta         xdr.TransactionMeta
		expectedEvents []xdr.ContractEvent
		expectedError  string
	}{
		{
			name:     "Soroban V3 transaction should return contract events",
			envelope: someSorobanTxEnvelope,
			txMeta: xdr.TransactionMeta{
				V: 3,
				V3: &xdr.TransactionMetaV3{
					SorobanMeta: &xdr.SorobanTransactionMeta{
						Events: []xdr.ContractEvent{mockContractEvent1, mockContractEvent2},
					},
				},
			},
			expectedEvents: []xdr.ContractEvent{mockContractEvent1, mockContractEvent2},
		},
		{
			name:     "V4 Soroban transaction should return events from operation 0",
			envelope: someSorobanTxEnvelope,
			txMeta: xdr.TransactionMeta{
				V: 4,
				V4: &xdr.TransactionMetaV4{
					Operations: []xdr.OperationMetaV2{
						{
							Events: []xdr.ContractEvent{mockContractEvent1}, // you'll only ever have 1 operation for soroban txs in TxMetaV4
						},
					},
				},
			},
			expectedEvents: []xdr.ContractEvent{mockContractEvent1},
		},
		{
			name:     "Non-Soroban transaction should return error",
			envelope: someClassicTxEnvelope,
			txMeta: xdr.TransactionMeta{
				V: 3,
				V3: &xdr.TransactionMetaV3{
					SorobanMeta: nil,
				},
			},
			expectedError: "not a soroban transaction",
		},
		{
			name:     "V3 Soroban transaction with no sorobabMeta should return nil",
			envelope: someSorobanTxEnvelope,
			txMeta: xdr.TransactionMeta{
				V: 3,
				V3: &xdr.TransactionMetaV3{
					SorobanMeta: nil,
				},
			},
			expectedEvents: nil,
		},
		{
			name:     "V3 Soroban transaction with no events should return empty slice",
			envelope: someSorobanTxEnvelope,
			txMeta: xdr.TransactionMeta{
				V: 3,
				V3: &xdr.TransactionMetaV3{
					SorobanMeta: &xdr.SorobanTransactionMeta{
						Events: []xdr.ContractEvent{},
					},
				},
			},
			expectedEvents: []xdr.ContractEvent{},
		},
		{
			name:     "V4 Soroban transaction should with no events should return empty slice",
			envelope: someSorobanTxEnvelope,
			txMeta: xdr.TransactionMeta{
				V: 4,
				V4: &xdr.TransactionMetaV4{
					Operations: []xdr.OperationMetaV2{
						{
							Events: []xdr.ContractEvent{},
						},
					},
				},
			},
			expectedEvents: []xdr.ContractEvent{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tx := &LedgerTransaction{
				Envelope:   tc.envelope,
				UnsafeMeta: tc.txMeta,
			}

			events, err := tx.GetContractEvents()

			if tc.expectedError != "" {
				require.Error(t, err, "Expected error for: %s", tc.name)
				assert.Contains(t, err.Error(), tc.expectedError, "Error message mismatch for: %s", tc.name)
				assert.Nil(t, events, "Events should be nil when error expected for: %s", tc.name)
			} else {
				require.NoError(t, err, "Unexpected error for: %s", tc.name)
				assert.Equal(t, tc.expectedEvents, events, "Events mismatch for: %s", tc.name)
			}
		})
	}
}

func TestGetDiagnosticEvents(t *testing.T) {
	testCases := []struct {
		name           string
		txMeta         xdr.TransactionMeta
		expectedEvents []xdr.DiagnosticEvent
		expectedError  string
	}{
		{
			name:           "V1 transaction meta should return nil diagnostic events",
			txMeta:         xdr.TransactionMeta{V: 1},
			expectedEvents: nil,
		},
		{
			name:           "V2 transaction meta should return nil diagnostic events",
			txMeta:         xdr.TransactionMeta{V: 2},
			expectedEvents: nil,
		},
		{
			name: "V3 should return diagnostic events from SorobanMeta",
			txMeta: xdr.TransactionMeta{
				V: 3,
				V3: &xdr.TransactionMetaV3{
					SorobanMeta: &xdr.SorobanTransactionMeta{
						DiagnosticEvents: []xdr.DiagnosticEvent{mockDiagnosticEvent1, mockDiagnosticEvent2},
					},
				},
			},
			expectedEvents: []xdr.DiagnosticEvent{mockDiagnosticEvent1, mockDiagnosticEvent2},
		},
		{
			name: "V3 with no SorobanMeta should return nil",
			txMeta: xdr.TransactionMeta{
				V: 3,
				V3: &xdr.TransactionMetaV3{
					SorobanMeta: nil,
				},
			},
			expectedEvents: nil,
		},
		{
			name: "V3 with empty diagnostic events should return empty slice",
			txMeta: xdr.TransactionMeta{
				V: 3,
				V3: &xdr.TransactionMetaV3{
					SorobanMeta: &xdr.SorobanTransactionMeta{
						DiagnosticEvents: []xdr.DiagnosticEvent{},
					},
				},
			},
			expectedEvents: []xdr.DiagnosticEvent{},
		},
		{
			name: "V4 should return diagnostic events from top level",
			txMeta: xdr.TransactionMeta{
				V: 4,
				V4: &xdr.TransactionMetaV4{
					DiagnosticEvents: []xdr.DiagnosticEvent{mockDiagnosticEvent1},
				},
			},
			expectedEvents: []xdr.DiagnosticEvent{mockDiagnosticEvent1},
		},
		{
			name: "V4 with no diagnostic events should return empty slice",
			txMeta: xdr.TransactionMeta{
				V: 4,
				V4: &xdr.TransactionMetaV4{
					DiagnosticEvents: []xdr.DiagnosticEvent{},
				},
			},
			expectedEvents: []xdr.DiagnosticEvent{},
		},
		{
			name:          "Unsupported version should return error",
			txMeta:        xdr.TransactionMeta{V: 5},
			expectedError: "unsupported TransactionMeta version: 5",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tx := &LedgerTransaction{
				UnsafeMeta: tc.txMeta,
			}

			events, err := tx.GetDiagnosticEvents()

			if tc.expectedError != "" {
				require.Error(t, err, "Expected error for: %s", tc.name)
				assert.Contains(t, err.Error(), tc.expectedError, "Error message mismatch for: %s", tc.name)
				assert.Nil(t, events, "Events should be nil when error expected for: %s", tc.name)
			} else {
				require.NoError(t, err, "Unexpected error for: %s", tc.name)
				assert.Equal(t, tc.expectedEvents, events, "Events mismatch for: %s", tc.name)
			}
		})
	}
}

func TestGetTransactionEvents(t *testing.T) {
	testCases := []struct {
		envelope         xdr.TransactionEnvelope
		txMeta           xdr.TransactionMeta
		expectedTxEvents TransactionEvents
		expectedError    string
		name             string
	}{
		{
			name:             "V1 should return empty TransactionEvents",
			envelope:         someClassicTxEnvelope,
			txMeta:           xdr.TransactionMeta{V: 1},
			expectedTxEvents: TransactionEvents{},
		},
		{
			name:             "V2 should return empty TransactionEvents",
			envelope:         someClassicTxEnvelope,
			txMeta:           xdr.TransactionMeta{V: 2},
			expectedTxEvents: TransactionEvents{},
		},
		{
			name:             "V3 non-Soroban transaction should return empty events",
			envelope:         someClassicTxEnvelope,
			txMeta:           xdr.TransactionMeta{V: 3},
			expectedTxEvents: TransactionEvents{},
		},
		{
			name:     "V3 Soroban transaction should return events from SorobanMeta",
			envelope: someSorobanTxEnvelope,
			txMeta: xdr.TransactionMeta{
				V: 3,
				V3: &xdr.TransactionMetaV3{
					SorobanMeta: &xdr.SorobanTransactionMeta{
						Events:           []xdr.ContractEvent{mockContractEvent1, mockContractEvent2},
						DiagnosticEvents: []xdr.DiagnosticEvent{mockDiagnosticEvent1},
					},
				},
			},
			expectedTxEvents: TransactionEvents{
				OperationEvents:  [][]xdr.ContractEvent{{mockContractEvent1, mockContractEvent2}},
				DiagnosticEvents: []xdr.DiagnosticEvent{mockDiagnosticEvent1},
			},
		},
		{
			name:     "V4 should return all event types from their respective locations",
			envelope: someClassicTxEnvelope, // doesnt matter here if its soroban tx or not for txMetaV4
			txMeta: xdr.TransactionMeta{
				V: 4,
				V4: &xdr.TransactionMetaV4{
					Events:           []xdr.TransactionEvent{mockTransactionEvent1, mockTransactionEvent2},
					DiagnosticEvents: []xdr.DiagnosticEvent{mockDiagnosticEvent1, mockDiagnosticEvent2},
					Operations: []xdr.OperationMetaV2{
						{
							Events: []xdr.ContractEvent{mockContractEvent1},
						},
						{
							Events: []xdr.ContractEvent{mockContractEvent2},
						},
					},
				},
			},
			expectedTxEvents: TransactionEvents{
				TransactionEvents: []xdr.TransactionEvent{mockTransactionEvent1, mockTransactionEvent2},
				OperationEvents:   [][]xdr.ContractEvent{{mockContractEvent1}, {mockContractEvent2}},
				DiagnosticEvents:  []xdr.DiagnosticEvent{mockDiagnosticEvent1, mockDiagnosticEvent2},
			},
		},
		{
			name:     "V4 with no events should return empty slices",
			envelope: someSorobanTxEnvelope, // doesnt matter here if its soroban tx or not for txMetaV4
			txMeta: xdr.TransactionMeta{
				V: 4,
				V4: &xdr.TransactionMetaV4{
					Events:           []xdr.TransactionEvent{},
					DiagnosticEvents: []xdr.DiagnosticEvent{},
					Operations: []xdr.OperationMetaV2{
						{
							Events: []xdr.ContractEvent{},
						},
					},
				},
			},
			expectedTxEvents: TransactionEvents{
				TransactionEvents: []xdr.TransactionEvent{},
				OperationEvents:   [][]xdr.ContractEvent{{}},
				DiagnosticEvents:  []xdr.DiagnosticEvent{},
			},
		},
		{
			name:     "V4 Soroban transaction should return all event types",
			envelope: someSorobanTxEnvelope, // doesnt matter here if its soroban tx or not for txMetaV4
			txMeta: xdr.TransactionMeta{
				V: 4,
				V4: &xdr.TransactionMetaV4{
					Events:           []xdr.TransactionEvent{mockTransactionEvent1},
					DiagnosticEvents: []xdr.DiagnosticEvent{mockDiagnosticEvent1},
					Operations: []xdr.OperationMetaV2{
						{
							Events: []xdr.ContractEvent{mockContractEvent1, mockContractEvent2},
						},
					},
				},
			},
			expectedTxEvents: TransactionEvents{
				TransactionEvents: []xdr.TransactionEvent{mockTransactionEvent1},
				OperationEvents:   [][]xdr.ContractEvent{{mockContractEvent1, mockContractEvent2}},
				DiagnosticEvents:  []xdr.DiagnosticEvent{mockDiagnosticEvent1},
			},
		},
		{
			name:          "Unsupported version should return error",
			envelope:      someClassicTxEnvelope,
			txMeta:        xdr.TransactionMeta{V: 5},
			expectedError: "unsupported TransactionMeta version: 5",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tx := &LedgerTransaction{
				Envelope:   tc.envelope,
				UnsafeMeta: tc.txMeta,
			}

			events, err := tx.GetTransactionEvents()

			if tc.expectedError != "" {
				require.Error(t, err, "Expected error for: %s", tc.name)
				assert.Contains(t, err.Error(), tc.expectedError, "Error message mismatch for: %s", tc.name)
			} else {
				require.NoError(t, err, "Unexpected error for: %s", tc.name)
				assert.Equal(t, tc.expectedTxEvents.TransactionEvents, events.TransactionEvents, "TransactionEvents mismatch for: %s", tc.name)
				assert.Equal(t, tc.expectedTxEvents.DiagnosticEvents, events.DiagnosticEvents, "DiagnosticEvents mismatch for: %s", tc.name)
				assert.Equal(t, tc.expectedTxEvents.OperationEvents, events.OperationEvents, "OperationEvents mismatch for: %s", tc.name)
			}
		})
	}
}
