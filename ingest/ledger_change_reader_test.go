package ingest

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/ingest/ledgerbackend"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const (
	feeAddress     = "GAHK7EEG2WWHVKDNT4CEQFZGKF2LGDSW2IVM4S5DP42RBW3K6BTODB4A"
	metaAddress    = "GACMZD5VJXTRLKVET72CETCYKELPNCOTTBDC6DHFEUPLG5DHEK534JQX"
	upgradeAddress = "GA5WBPYA5Y4WAEHXWR2UKO2UO4BUGHUQ74EUPKON2QHV4WRHOIRNKKH2"
)

func TestNewLedgerChangeReaderFails(t *testing.T) {
	ctx := context.Background()
	mock := &ledgerbackend.MockDatabaseBackend{}
	seq := uint32(123)
	mock.On("GetLedger", ctx, seq).Return(
		xdr.LedgerCloseMeta{},
		fmt.Errorf("ledger error"),
	).Once()
	_, err := NewLedgerChangeReader(ctx, mock, network.TestNetworkPassphrase, seq)
	assert.EqualError(
		t,
		err,
		"error getting ledger from the backend: ledger error",
	)
}

func TestNewLedgerChangeReaderSucceeds(t *testing.T) {
	ctx := context.Background()
	mock := &ledgerbackend.MockDatabaseBackend{}
	seq := uint32(123)

	header := xdr.LedgerHeaderHistoryEntry{
		Hash: xdr.Hash{1, 2, 3},
		Header: xdr.LedgerHeader{
			LedgerVersion: 7,
		},
	}

	mock.On("GetLedger", ctx, seq).Return(
		xdr.LedgerCloseMeta{
			V0: &xdr.LedgerCloseMetaV0{
				LedgerHeader: header,
			},
		},
		nil,
	).Once()

	reader, err := NewLedgerChangeReader(ctx, mock, network.TestNetworkPassphrase, seq)
	assert.NoError(t, err)

	assert.Equal(t, reader.GetHeader(), header)
}

func buildChange(account string, balance int64) xdr.LedgerEntryChange {
	return xdr.LedgerEntryChange{
		Type: xdr.LedgerEntryChangeTypeLedgerEntryCreated,
		Created: &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress(account),
					Balance:   xdr.Int64(balance),
				},
			},
		},
	}
}

type changePredicate func(*testing.T, int, Change)

func isBalance(address string, balance int64) changePredicate {
	return func(t *testing.T, idx int, change Change) {
		msg := fmt.Sprintf("change %d", idx)
		require.NotNil(t, change.Post, msg)
		assert.Equal(t, xdr.LedgerEntryTypeAccount.String(), change.Post.Data.Type.String(), msg)
		assert.Equal(t, address, change.Post.Data.Account.AccountId.Address(), msg)
		assert.EqualValues(t, balance, change.Post.Data.Account.Balance, msg)
	}
}

func isContractDataExtension(contract xdr.ScAddress, key xdr.ScVal, extension uint32) changePredicate {
	return func(t *testing.T, idx int, change Change) {
		msg := fmt.Sprintf("change %d", idx)
		require.NotNil(t, change.Post, msg)
		require.NotNil(t, change.Pre, msg)
		assert.Equal(t, xdr.LedgerEntryTypeContractData.String(), change.Post.Data.Type.String(), msg)
		assert.Equal(t, contract, change.Post.Data.ContractData.Contract, msg)
		assert.Equal(t, key, change.Post.Data.ContractData.Key, msg)
	}
}

func isContractDataEviction(contract xdr.ScAddress, key xdr.ScVal) changePredicate {
	return func(t *testing.T, idx int, change Change) {
		msg := fmt.Sprintf("change %d", idx)
		require.NotNil(t, change.Pre, msg)
		assert.Nil(t, change.Post, msg)
		assert.Equal(t, xdr.LedgerEntryTypeContractData.String(), change.Pre.Data.Type.String(), msg)
		assert.Equal(t, contract, change.Pre.Data.ContractData.Contract, msg)
		assert.Equal(t, key, change.Pre.Data.ContractData.Key, msg)
	}
}

func assertChangesEqual(
	t *testing.T,
	ctx context.Context,
	sequence uint32,
	backend ledgerbackend.LedgerBackend,
	expectations []changePredicate,
) {
	reader, err := NewLedgerChangeReader(ctx, backend, network.TestNetworkPassphrase, sequence)
	assert.NoError(t, err)

	// Read all the changes
	var changes []Change
	for {
		change, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		changes = append(changes, change)
	}

	assert.Len(t, changes, len(expectations), "unexpected number of changes")

	// Check each change is what we expect
	for i, change := range changes {
		expectations[i](t, i+1, change)
	}
}

func TestLedgerChangeReaderOrder(t *testing.T) {
	ctx := context.Background()
	mock := &ledgerbackend.MockDatabaseBackend{}
	seq := uint32(123)

	src := xdr.MustAddress("GBXGQJWVLWOYHFLVTKWV5FGHA3LNYY2JQKM7OAJAUEQFU6LPCSEFVXON")
	firstTx := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{
				Fee:           1,
				SourceAccount: src.ToMuxedAccount(),
			},
		},
	}
	firstTxHash, err := network.HashTransactionInEnvelope(firstTx, network.TestNetworkPassphrase)
	assert.NoError(t, err)

	src = xdr.MustAddress("GCXKG6RN4ONIEPCMNFB732A436Z5PNDSRLGWK7GBLCMQLIFO4S7EYWVU")
	secondTx := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{
				Fee:           2,
				SourceAccount: src.ToMuxedAccount(),
			},
		},
	}
	secondTxHash, err := network.HashTransactionInEnvelope(secondTx, network.TestNetworkPassphrase)
	assert.NoError(t, err)

	ledger := xdr.LedgerCloseMeta{
		V0: &xdr.LedgerCloseMetaV0{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{Header: xdr.LedgerHeader{LedgerVersion: 10}},
			TxSet: xdr.TransactionSet{
				Txs: []xdr.TransactionEnvelope{
					secondTx,
					firstTx,
				},
			},
			TxProcessing: []xdr.TransactionResultMeta{
				{
					Result: xdr.TransactionResultPair{TransactionHash: firstTxHash},
					FeeProcessing: xdr.LedgerEntryChanges{
						buildChange(feeAddress, 100),
						buildChange(feeAddress, 200),
					},
					TxApplyProcessing: xdr.TransactionMeta{
						V: 1,
						V1: &xdr.TransactionMetaV1{
							Operations: []xdr.OperationMeta{
								{
									Changes: xdr.LedgerEntryChanges{
										buildChange(
											metaAddress,
											300,
										),
										buildChange(
											metaAddress,
											400,
										),
									},
								},
							},
						},
					},
				},
				{
					Result: xdr.TransactionResultPair{TransactionHash: secondTxHash},
					FeeProcessing: xdr.LedgerEntryChanges{
						buildChange(feeAddress, 300),
					},
					TxApplyProcessing: xdr.TransactionMeta{
						V: 2,
						V2: &xdr.TransactionMetaV2{
							TxChangesBefore: xdr.LedgerEntryChanges{
								buildChange(metaAddress, 600),
							},
							Operations: []xdr.OperationMeta{
								{
									Changes: xdr.LedgerEntryChanges{
										buildChange(metaAddress, 700),
									},
								},
							},
							TxChangesAfter: xdr.LedgerEntryChanges{
								buildChange(metaAddress, 800),
								buildChange(metaAddress, 900),
							},
						},
					},
				},
			},
			UpgradesProcessing: []xdr.UpgradeEntryMeta{
				{
					Changes: xdr.LedgerEntryChanges{
						buildChange(upgradeAddress, 2),
					},
				},
				{
					Changes: xdr.LedgerEntryChanges{
						buildChange(upgradeAddress, 3),
					},
				},
			},
		},
	}
	mock.On("GetLedger", ctx, seq).Return(ledger, nil).Once()

	assertChangesEqual(t, ctx, seq, mock, []changePredicate{
		isBalance(feeAddress, 100),
		isBalance(feeAddress, 200),
		isBalance(feeAddress, 300),
		isBalance(metaAddress, 300),
		isBalance(metaAddress, 400),
		isBalance(metaAddress, 600),
		isBalance(metaAddress, 700),
		isBalance(metaAddress, 800),
		isBalance(metaAddress, 900),
		isBalance(upgradeAddress, 2),
		isBalance(upgradeAddress, 3),
	})
	mock.AssertExpectations(t)

	ledger.V0.LedgerHeader.Header.LedgerVersion = 8
	mock.On("GetLedger", ctx, seq).Return(ledger, nil).Once()
	_, err = NewLedgerChangeReader(ctx, mock, network.TestNetworkPassphrase, seq)
	assert.EqualError(
		t,
		err,
		"error extracting transactions from ledger close meta: TransactionMeta.V=2 is required in protocol"+
			" version older than version 10. Please process ledgers again using the latest stellar-core version.",
	)
	mock.AssertExpectations(t)

	ledger.V0.LedgerHeader.Header.LedgerVersion = 9
	ledger.V0.TxProcessing[0].FeeProcessing = xdr.LedgerEntryChanges{}
	ledger.V0.TxProcessing[1].FeeProcessing = xdr.LedgerEntryChanges{}
	mock.On("GetLedger", ctx, seq).Return(ledger, nil).Once()

	assertChangesEqual(t, ctx, seq, mock, []changePredicate{
		isBalance(metaAddress, 300),
		isBalance(metaAddress, 400),
		isBalance(metaAddress, 600),
		isBalance(metaAddress, 700),
		isBalance(metaAddress, 800),
		isBalance(metaAddress, 900),
		isBalance(upgradeAddress, 2),
		isBalance(upgradeAddress, 3),
	})
	mock.AssertExpectations(t)

	ledger.V0.LedgerHeader.Header.LedgerVersion = 10
	ledger.V0.TxProcessing[0].FeeProcessing = xdr.LedgerEntryChanges{}
	ledger.V0.TxProcessing[1].FeeProcessing = xdr.LedgerEntryChanges{}
	mock.On("GetLedger", ctx, seq).Return(ledger, nil).Once()

	assertChangesEqual(t, ctx, seq, mock, []changePredicate{
		isBalance(metaAddress, 300),
		isBalance(metaAddress, 400),
		isBalance(metaAddress, 600),
		isBalance(metaAddress, 700),
		isBalance(metaAddress, 800),
		isBalance(metaAddress, 900),
		isBalance(upgradeAddress, 2),
		isBalance(upgradeAddress, 3),
	})
	mock.AssertExpectations(t)

	ledger.V0.UpgradesProcessing = []xdr.UpgradeEntryMeta{
		{
			Changes: xdr.LedgerEntryChanges{},
		},
		{
			Changes: xdr.LedgerEntryChanges{},
		},
	}
	mock.On("GetLedger", ctx, seq).Return(ledger, nil).Once()

	assertChangesEqual(t, ctx, seq, mock, []changePredicate{
		isBalance(metaAddress, 300),
		isBalance(metaAddress, 400),
		isBalance(metaAddress, 600),
		isBalance(metaAddress, 700),
		isBalance(metaAddress, 800),
		isBalance(metaAddress, 900),
	})
	mock.AssertExpectations(t)

	ledger.V0.TxProcessing[0].TxApplyProcessing = xdr.TransactionMeta{
		V: 1,
		V1: &xdr.TransactionMetaV1{
			Operations: []xdr.OperationMeta{},
		},
	}
	ledger.V0.TxProcessing[1].TxApplyProcessing = xdr.TransactionMeta{
		V: 1,
		V1: &xdr.TransactionMetaV1{
			Operations: []xdr.OperationMeta{},
		},
	}
	mock.On("GetLedger", ctx, seq).Return(ledger, nil).Once()

	assertChangesEqual(t, ctx, seq, mock, []changePredicate{})
	mock.AssertExpectations(t)
}

func TestLedgerChangeLedgerCloseMetaV1(t *testing.T) {
	ctx := context.Background()
	mock := &ledgerbackend.MockDatabaseBackend{}
	seq := uint32(123)

	src := xdr.MustAddress("GBXGQJWVLWOYHFLVTKWV5FGHA3LNYY2JQKM7OAJAUEQFU6LPCSEFVXON")
	firstTx := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{
				Fee:           1,
				SourceAccount: src.ToMuxedAccount(),
			},
		},
	}
	firstTxHash, err := network.HashTransactionInEnvelope(firstTx, network.TestNetworkPassphrase)
	assert.NoError(t, err)

	src = xdr.MustAddress("GCXKG6RN4ONIEPCMNFB732A436Z5PNDSRLGWK7GBLCMQLIFO4S7EYWVU")
	secondTx := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{
				Fee:           2,
				SourceAccount: src.ToMuxedAccount(),
			},
		},
	}
	secondTxHash, err := network.HashTransactionInEnvelope(secondTx, network.TestNetworkPassphrase)
	assert.NoError(t, err)

	baseFee := xdr.Int64(100)
	tempKey := xdr.ScSymbol("TEMPKEY")
	persistentKey := xdr.ScSymbol("TEMPVAL")
	contractIDBytes, err := hex.DecodeString("df06d62447fd25da07c0135eed7557e5a5497ee7d15b7fe345bd47e191d8f577")
	assert.NoError(t, err)
	var contractID xdr.ContractId
	copy(contractID[:], contractIDBytes)
	contractAddress := xdr.ScAddress{
		Type:       xdr.ScAddressTypeScAddressTypeContract,
		ContractId: &contractID,
	}
	ledger := xdr.LedgerCloseMeta{
		V: 1,
		V1: &xdr.LedgerCloseMetaV1{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{Header: xdr.LedgerHeader{LedgerVersion: 10}},
			TxSet: xdr.GeneralizedTransactionSet{
				V: 1,
				V1TxSet: &xdr.TransactionSetV1{
					PreviousLedgerHash: xdr.Hash{1, 2, 3},
					Phases: []xdr.TransactionPhase{
						{
							V0Components: &[]xdr.TxSetComponent{
								{
									Type: xdr.TxSetComponentTypeTxsetCompTxsMaybeDiscountedFee,
									TxsMaybeDiscountedFee: &xdr.TxSetComponentTxsMaybeDiscountedFee{
										BaseFee: &baseFee,
										Txs: []xdr.TransactionEnvelope{
											secondTx,
											firstTx,
										},
									},
								},
							},
						},
					},
				},
			},
			TxProcessing: []xdr.TransactionResultMeta{
				{
					Result: xdr.TransactionResultPair{TransactionHash: firstTxHash},
					FeeProcessing: xdr.LedgerEntryChanges{
						buildChange(feeAddress, 100),
						buildChange(feeAddress, 200),
					},
					TxApplyProcessing: xdr.TransactionMeta{
						V: 3,
						V3: &xdr.TransactionMetaV3{
							Operations: []xdr.OperationMeta{
								{
									Changes: xdr.LedgerEntryChanges{
										buildChange(
											metaAddress,
											300,
										),
										buildChange(
											metaAddress,
											400,
										),

										// Add a couple changes simulating a ledger entry extension
										{
											Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
											State: &xdr.LedgerEntry{
												LastModifiedLedgerSeq: 1,
												Data: xdr.LedgerEntryData{
													Type: xdr.LedgerEntryTypeContractData,
													ContractData: &xdr.ContractDataEntry{
														Contract: contractAddress,
														Key: xdr.ScVal{
															Type: xdr.ScValTypeScvSymbol,
															Sym:  &persistentKey,
														},
														Durability: xdr.ContractDataDurabilityPersistent,
													},
												},
											},
										},
										{
											Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
											Updated: &xdr.LedgerEntry{
												LastModifiedLedgerSeq: 1,
												Data: xdr.LedgerEntryData{
													Type: xdr.LedgerEntryTypeContractData,
													ContractData: &xdr.ContractDataEntry{
														Contract: xdr.ScAddress{
															Type:       xdr.ScAddressTypeScAddressTypeContract,
															ContractId: &contractID,
														},
														Key: xdr.ScVal{
															Type: xdr.ScValTypeScvSymbol,
															Sym:  &persistentKey,
														},
														Durability: xdr.ContractDataDurabilityPersistent,
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				{
					Result: xdr.TransactionResultPair{TransactionHash: secondTxHash},
					FeeProcessing: xdr.LedgerEntryChanges{
						buildChange(feeAddress, 300),
					},
					TxApplyProcessing: xdr.TransactionMeta{
						V: 3,
						V3: &xdr.TransactionMetaV3{
							TxChangesBefore: xdr.LedgerEntryChanges{
								buildChange(metaAddress, 600),
							},
							Operations: []xdr.OperationMeta{
								{
									Changes: xdr.LedgerEntryChanges{
										buildChange(metaAddress, 700),
									},
								},
							},
							TxChangesAfter: xdr.LedgerEntryChanges{
								buildChange(metaAddress, 800),
								buildChange(metaAddress, 900),
							},
						},
					},
				},
			},
			UpgradesProcessing: []xdr.UpgradeEntryMeta{
				{
					Changes: xdr.LedgerEntryChanges{
						buildChange(upgradeAddress, 2),
					},
				},
				{
					Changes: xdr.LedgerEntryChanges{
						buildChange(upgradeAddress, 3),
					},
				},
			},
			EvictedKeys: []xdr.LedgerKey{
				{
					Type: xdr.LedgerEntryTypeContractData,
					ContractData: &xdr.LedgerKeyContractData{
						Contract: contractAddress,
						Key: xdr.ScVal{
							Type: xdr.ScValTypeScvSymbol,
							Sym:  &tempKey,
						},
						Durability: xdr.ContractDataDurabilityTemporary,
					},
				},
			},
		},
	}
	mock.On("GetLedger", ctx, seq).Return(ledger, nil).Once()

	// Check the changes are as expected
	assertChangesEqual(t, ctx, seq, mock, []changePredicate{
		// First the first txn balance xfers
		isBalance(feeAddress, 100),
		isBalance(feeAddress, 200),
		isBalance(feeAddress, 300),
		isBalance(metaAddress, 300),
		isBalance(metaAddress, 400),
		// Then the first txn data entry extension
		isContractDataExtension(
			contractAddress,
			xdr.ScVal{
				Type: xdr.ScValTypeScvSymbol,
				Sym:  &persistentKey,
			},
			5904,
		),

		// Second txn transfers
		isBalance(metaAddress, 600),
		isBalance(metaAddress, 700),
		isBalance(metaAddress, 800),
		isBalance(metaAddress, 900),

		// Upgrades last
		isBalance(upgradeAddress, 2),
		isBalance(upgradeAddress, 3),
	})
	mock.AssertExpectations(t)

	mock.AssertExpectations(t)
}

func TestLedgerChangeLedgerCloseMetaV1ParallelPhases(t *testing.T) {
	ctx := context.Background()
	mock := &ledgerbackend.MockDatabaseBackend{}
	seq := uint32(123)

	src := xdr.MustAddress("GBXGQJWVLWOYHFLVTKWV5FGHA3LNYY2JQKM7OAJAUEQFU6LPCSEFVXON")
	firstTx := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{
				Fee:           1,
				SourceAccount: src.ToMuxedAccount(),
			},
		},
	}
	firstTxHash, err := network.HashTransactionInEnvelope(firstTx, network.TestNetworkPassphrase)
	assert.NoError(t, err)

	src = xdr.MustAddress("GCXKG6RN4ONIEPCMNFB732A436Z5PNDSRLGWK7GBLCMQLIFO4S7EYWVU")
	secondTx := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{
				Fee:           2,
				SourceAccount: src.ToMuxedAccount(),
			},
		},
	}
	secondTxHash, err := network.HashTransactionInEnvelope(secondTx, network.TestNetworkPassphrase)
	assert.NoError(t, err)

	tempKey := xdr.ScSymbol("TEMPKEY")
	persistentKey := xdr.ScSymbol("TEMPVAL")
	contractIDBytes, err := hex.DecodeString("df06d62447fd25da07c0135eed7557e5a5497ee7d15b7fe345bd47e191d8f577")
	assert.NoError(t, err)
	var contractID xdr.ContractId
	copy(contractID[:], contractIDBytes)
	contractAddress := xdr.ScAddress{
		Type:       xdr.ScAddressTypeScAddressTypeContract,
		ContractId: &contractID,
	}
	ledger := xdr.LedgerCloseMeta{
		V: 1,
		V1: &xdr.LedgerCloseMetaV1{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{Header: xdr.LedgerHeader{LedgerVersion: 10}},
			TxSet: xdr.GeneralizedTransactionSet{
				V: 1,
				V1TxSet: &xdr.TransactionSetV1{
					PreviousLedgerHash: xdr.Hash{1, 2, 3},
					Phases: []xdr.TransactionPhase{
						{
							V: 1,
							ParallelTxsComponent: &xdr.ParallelTxsComponent{
								ExecutionStages: []xdr.ParallelTxExecutionStage{
									{
										xdr.DependentTxCluster{secondTx},
									},
									{
										xdr.DependentTxCluster{firstTx},
									},
								},
							},
						},
					},
				},
			},
			TxProcessing: []xdr.TransactionResultMeta{
				{
					Result: xdr.TransactionResultPair{TransactionHash: firstTxHash},
					FeeProcessing: xdr.LedgerEntryChanges{
						buildChange(feeAddress, 100),
						buildChange(feeAddress, 200),
					},
					TxApplyProcessing: xdr.TransactionMeta{
						V: 3,
						V3: &xdr.TransactionMetaV3{
							Operations: []xdr.OperationMeta{
								{
									Changes: xdr.LedgerEntryChanges{
										buildChange(
											metaAddress,
											300,
										),
										buildChange(
											metaAddress,
											400,
										),

										// Add a couple changes simulating a ledger entry extension
										{
											Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
											State: &xdr.LedgerEntry{
												LastModifiedLedgerSeq: 1,
												Data: xdr.LedgerEntryData{
													Type: xdr.LedgerEntryTypeContractData,
													ContractData: &xdr.ContractDataEntry{
														Contract: contractAddress,
														Key: xdr.ScVal{
															Type: xdr.ScValTypeScvSymbol,
															Sym:  &persistentKey,
														},
														Durability: xdr.ContractDataDurabilityPersistent,
													},
												},
											},
										},
										{
											Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
											Updated: &xdr.LedgerEntry{
												LastModifiedLedgerSeq: 1,
												Data: xdr.LedgerEntryData{
													Type: xdr.LedgerEntryTypeContractData,
													ContractData: &xdr.ContractDataEntry{
														Contract: xdr.ScAddress{
															Type:       xdr.ScAddressTypeScAddressTypeContract,
															ContractId: &contractID,
														},
														Key: xdr.ScVal{
															Type: xdr.ScValTypeScvSymbol,
															Sym:  &persistentKey,
														},
														Durability: xdr.ContractDataDurabilityPersistent,
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				{
					Result: xdr.TransactionResultPair{TransactionHash: secondTxHash},
					FeeProcessing: xdr.LedgerEntryChanges{
						buildChange(feeAddress, 300),
					},
					TxApplyProcessing: xdr.TransactionMeta{
						V: 3,
						V3: &xdr.TransactionMetaV3{
							TxChangesBefore: xdr.LedgerEntryChanges{
								buildChange(metaAddress, 600),
							},
							Operations: []xdr.OperationMeta{
								{
									Changes: xdr.LedgerEntryChanges{
										buildChange(metaAddress, 700),
									},
								},
							},
							TxChangesAfter: xdr.LedgerEntryChanges{
								buildChange(metaAddress, 800),
								buildChange(metaAddress, 900),
							},
						},
					},
				},
			},
			UpgradesProcessing: []xdr.UpgradeEntryMeta{
				{
					Changes: xdr.LedgerEntryChanges{
						buildChange(upgradeAddress, 2),
					},
				},
				{
					Changes: xdr.LedgerEntryChanges{
						buildChange(upgradeAddress, 3),
					},
				},
			},
			EvictedKeys: []xdr.LedgerKey{
				{
					Type: xdr.LedgerEntryTypeContractData,
					ContractData: &xdr.LedgerKeyContractData{
						Contract: contractAddress,
						Key: xdr.ScVal{
							Type: xdr.ScValTypeScvSymbol,
							Sym:  &tempKey,
						},
						Durability: xdr.ContractDataDurabilityTemporary,
					},
				},
			},
		},
	}
	mock.On("GetLedger", ctx, seq).Return(ledger, nil).Once()

	// Check the changes are as expected. Both transactions are in a parallel
	// phase (CAP-63), so the txChangesBefore of the second transaction is
	// streamed before any operation change.
	assertChangesEqual(t, ctx, seq, mock, []changePredicate{
		// Fee changes
		isBalance(feeAddress, 100),
		isBalance(feeAddress, 200),
		isBalance(feeAddress, 300),
		// txChangesBefore of the parallel phase (only the second txn has one)
		isBalance(metaAddress, 600),
		// First txn balance xfers
		isBalance(metaAddress, 300),
		isBalance(metaAddress, 400),
		// Then the first txn data entry extension
		isContractDataExtension(
			contractAddress,
			xdr.ScVal{
				Type: xdr.ScValTypeScvSymbol,
				Sym:  &persistentKey,
			},
			5904,
		),

		// Second txn operations and txChangesAfter
		isBalance(metaAddress, 700),
		isBalance(metaAddress, 800),
		isBalance(metaAddress, 900),

		// Upgrades last
		isBalance(upgradeAddress, 2),
		isBalance(upgradeAddress, 3),
	})
	mock.AssertExpectations(t)

	mock.AssertExpectations(t)
}

func TestLedgerChangeLedgerCloseMetaV1Empty(t *testing.T) {
	ctx := context.Background()
	mock := &ledgerbackend.MockDatabaseBackend{}
	seq := uint32(123)

	baseFee := xdr.Int64(100)
	ledger := xdr.LedgerCloseMeta{
		V: 1,
		V1: &xdr.LedgerCloseMetaV1{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{Header: xdr.LedgerHeader{LedgerVersion: 10}},
			TxSet: xdr.GeneralizedTransactionSet{
				V: 1,
				V1TxSet: &xdr.TransactionSetV1{
					PreviousLedgerHash: xdr.Hash{1, 2, 3},
					Phases: []xdr.TransactionPhase{
						{
							V0Components: &[]xdr.TxSetComponent{
								{
									Type: xdr.TxSetComponentTypeTxsetCompTxsMaybeDiscountedFee,
									TxsMaybeDiscountedFee: &xdr.TxSetComponentTxsMaybeDiscountedFee{
										BaseFee: &baseFee,
										Txs:     []xdr.TransactionEnvelope{},
									},
								},
							},
						},
					},
				},
			},
			TxProcessing:       []xdr.TransactionResultMeta{},
			UpgradesProcessing: []xdr.UpgradeEntryMeta{},
			EvictedKeys:        []xdr.LedgerKey{},
		},
	}
	mock.On("GetLedger", ctx, seq).Return(ledger, nil).Once()

	// Check there are no changes
	assertChangesEqual(t, ctx, seq, mock, []changePredicate{})
	mock.AssertExpectations(t)
}

func TestLedgerChangeLedgerCloseMetaV2Empty(t *testing.T) {
	ctx := context.Background()
	mock := &ledgerbackend.MockDatabaseBackend{}
	seq := uint32(123)

	baseFee := xdr.Int64(100)
	ledger := xdr.LedgerCloseMeta{
		V: 2,
		V2: &xdr.LedgerCloseMetaV2{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{Header: xdr.LedgerHeader{LedgerVersion: 10}},
			TxSet: xdr.GeneralizedTransactionSet{
				V: 1,
				V1TxSet: &xdr.TransactionSetV1{
					PreviousLedgerHash: xdr.Hash{1, 2, 3},
					Phases: []xdr.TransactionPhase{
						{
							V0Components: &[]xdr.TxSetComponent{
								{
									Type: xdr.TxSetComponentTypeTxsetCompTxsMaybeDiscountedFee,
									TxsMaybeDiscountedFee: &xdr.TxSetComponentTxsMaybeDiscountedFee{
										BaseFee: &baseFee,
										Txs:     []xdr.TransactionEnvelope{},
									},
								},
							},
						},
					},
				},
			},
			TxProcessing:       []xdr.TransactionResultMetaV1{},
			UpgradesProcessing: []xdr.UpgradeEntryMeta{},
			EvictedKeys:        []xdr.LedgerKey{},
		},
	}
	mock.On("GetLedger", ctx, seq).Return(ledger, nil).Once()

	// Check there are no changes
	assertChangesEqual(t, ctx, seq, mock, []changePredicate{})
	mock.AssertExpectations(t)
}

func TestLedgerChangeLedgerCloseMetaV2(t *testing.T) {
	ctx := context.Background()
	mock := &ledgerbackend.MockDatabaseBackend{}
	seq := uint32(123)

	src := xdr.MustAddress("GBXGQJWVLWOYHFLVTKWV5FGHA3LNYY2JQKM7OAJAUEQFU6LPCSEFVXON")
	firstTx := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{
				Fee:           1,
				SourceAccount: src.ToMuxedAccount(),
			},
		},
	}
	firstTxHash, err := network.HashTransactionInEnvelope(firstTx, network.TestNetworkPassphrase)
	assert.NoError(t, err)

	src = xdr.MustAddress("GCXKG6RN4ONIEPCMNFB732A436Z5PNDSRLGWK7GBLCMQLIFO4S7EYWVU")
	secondTx := xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{
				Fee:           2,
				SourceAccount: src.ToMuxedAccount(),
			},
		},
	}
	secondTxHash, err := network.HashTransactionInEnvelope(secondTx, network.TestNetworkPassphrase)
	assert.NoError(t, err)

	tempKey := xdr.ScSymbol("TEMPKEY")
	persistentKey := xdr.ScSymbol("TEMPVAL")
	contractIDBytes, err := hex.DecodeString("df06d62447fd25da07c0135eed7557e5a5497ee7d15b7fe345bd47e191d8f577")
	assert.NoError(t, err)
	var contractID xdr.ContractId
	copy(contractID[:], contractIDBytes)
	contractAddress := xdr.ScAddress{
		Type:       xdr.ScAddressTypeScAddressTypeContract,
		ContractId: &contractID,
	}
	ledger := xdr.LedgerCloseMeta{
		V: 2,
		V2: &xdr.LedgerCloseMetaV2{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{Header: xdr.LedgerHeader{LedgerVersion: 10}},
			TxSet: xdr.GeneralizedTransactionSet{
				V: 1,
				V1TxSet: &xdr.TransactionSetV1{
					PreviousLedgerHash: xdr.Hash{1, 2, 3},
					Phases: []xdr.TransactionPhase{
						{
							V: 1,
							ParallelTxsComponent: &xdr.ParallelTxsComponent{
								ExecutionStages: []xdr.ParallelTxExecutionStage{
									{
										xdr.DependentTxCluster{secondTx},
									},
									{
										xdr.DependentTxCluster{firstTx},
									},
								},
							},
						},
					},
				},
			},
			TxProcessing: []xdr.TransactionResultMetaV1{
				{
					Result: xdr.TransactionResultPair{TransactionHash: firstTxHash},
					FeeProcessing: xdr.LedgerEntryChanges{
						buildChange(feeAddress, 100),
						buildChange(feeAddress, 200),
					},
					PostTxApplyFeeProcessing: xdr.LedgerEntryChanges{
						buildChange(feeAddress, 400),
					},
					TxApplyProcessing: xdr.TransactionMeta{
						V: 4,
						V4: &xdr.TransactionMetaV4{
							Operations: []xdr.OperationMetaV2{
								{
									Changes: xdr.LedgerEntryChanges{
										buildChange(
											metaAddress,
											300,
										),
										buildChange(
											metaAddress,
											400,
										),

										// Add a couple changes simulating a ledger entry extension
										{
											Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
											State: &xdr.LedgerEntry{
												LastModifiedLedgerSeq: 1,
												Data: xdr.LedgerEntryData{
													Type: xdr.LedgerEntryTypeContractData,
													ContractData: &xdr.ContractDataEntry{
														Contract: contractAddress,
														Key: xdr.ScVal{
															Type: xdr.ScValTypeScvSymbol,
															Sym:  &persistentKey,
														},
														Durability: xdr.ContractDataDurabilityPersistent,
													},
												},
											},
										},
										{
											Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
											Updated: &xdr.LedgerEntry{
												LastModifiedLedgerSeq: 1,
												Data: xdr.LedgerEntryData{
													Type: xdr.LedgerEntryTypeContractData,
													ContractData: &xdr.ContractDataEntry{
														Contract: xdr.ScAddress{
															Type:       xdr.ScAddressTypeScAddressTypeContract,
															ContractId: &contractID,
														},
														Key: xdr.ScVal{
															Type: xdr.ScValTypeScvSymbol,
															Sym:  &persistentKey,
														},
														Durability: xdr.ContractDataDurabilityPersistent,
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				{
					Result: xdr.TransactionResultPair{TransactionHash: secondTxHash},
					FeeProcessing: xdr.LedgerEntryChanges{
						buildChange(feeAddress, 300),
					},
					TxApplyProcessing: xdr.TransactionMeta{
						V: 4,
						V4: &xdr.TransactionMetaV4{
							TxChangesBefore: xdr.LedgerEntryChanges{
								buildChange(metaAddress, 600),
							},
							Operations: []xdr.OperationMetaV2{
								{
									Changes: xdr.LedgerEntryChanges{
										buildChange(metaAddress, 700),
									},
								},
							},
							TxChangesAfter: xdr.LedgerEntryChanges{
								buildChange(metaAddress, 800),
								buildChange(metaAddress, 900),
							},
						},
					},
				},
			},
			UpgradesProcessing: []xdr.UpgradeEntryMeta{
				{
					Changes: xdr.LedgerEntryChanges{
						buildChange(upgradeAddress, 2),
					},
				},
				{
					Changes: xdr.LedgerEntryChanges{
						buildChange(upgradeAddress, 3),
					},
				},
			},
			EvictedKeys: []xdr.LedgerKey{
				{
					Type: xdr.LedgerEntryTypeContractData,
					ContractData: &xdr.LedgerKeyContractData{
						Contract: contractAddress,
						Key: xdr.ScVal{
							Type: xdr.ScValTypeScvSymbol,
							Sym:  &tempKey,
						},
						Durability: xdr.ContractDataDurabilityTemporary,
					},
				},
			},
		},
	}
	mock.On("GetLedger", ctx, seq).Return(ledger, nil).Once()

	// Check the changes are as expected. Both transactions are in a parallel
	// phase (CAP-63), so the txChangesBefore of the second transaction is
	// streamed before any operation change.
	assertChangesEqual(t, ctx, seq, mock, []changePredicate{
		// Fee changes
		isBalance(feeAddress, 100),
		isBalance(feeAddress, 200),
		isBalance(feeAddress, 300),
		// txChangesBefore of the parallel phase (only the second txn has one)
		isBalance(metaAddress, 600),
		// First txn balance xfers
		isBalance(metaAddress, 300),
		isBalance(metaAddress, 400),
		// Then the first txn data entry extension
		isContractDataExtension(
			contractAddress,
			xdr.ScVal{
				Type: xdr.ScValTypeScvSymbol,
				Sym:  &persistentKey,
			},
			5904,
		),

		// Second txn operations and txChangesAfter
		isBalance(metaAddress, 700),
		isBalance(metaAddress, 800),
		isBalance(metaAddress, 900),

		// First txn post apply fee changes
		isBalance(feeAddress, 400),

		// Upgrades last
		isBalance(upgradeAddress, 2),
		isBalance(upgradeAddress, 3),
	})
	mock.AssertExpectations(t)

	mock.AssertExpectations(t)
}

// buildUpdate returns a State/Updated pair for the account, which is how
// stellar-core reports a modification to an existing account.
func buildUpdate(account string, preBalance, preSeq, postBalance, postSeq int64) []xdr.LedgerEntryChange {
	entry := func(balance, seq int64) *xdr.LedgerEntry {
		return &xdr.LedgerEntry{
			Data: xdr.LedgerEntryData{
				Type: xdr.LedgerEntryTypeAccount,
				Account: &xdr.AccountEntry{
					AccountId: xdr.MustAddress(account),
					Balance:   xdr.Int64(balance),
					SeqNum:    xdr.SequenceNumber(seq),
				},
			},
		}
	}
	return []xdr.LedgerEntryChange{
		{Type: xdr.LedgerEntryChangeTypeLedgerEntryState, State: entry(preBalance, preSeq)},
		{Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated, Updated: entry(postBalance, postSeq)},
	}
}

func parallelPhaseTx(source string, fee uint32) xdr.TransactionEnvelope {
	return xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{
				Fee:           xdr.Uint32(fee),
				SourceAccount: xdr.MustMuxedAddress(source),
				Ext:           xdr.TransactionExt{V: 1, SorobanData: &xdr.SorobanTransactionData{}},
			},
		},
	}
}

// parallelPhaseFeeBumpTx wraps a Soroban transaction in a fee bump envelope.
func parallelPhaseFeeBumpTx(feeSource string, fee int64, inner xdr.TransactionEnvelope) xdr.TransactionEnvelope {
	return xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTxFeeBump,
		FeeBump: &xdr.FeeBumpTransactionEnvelope{
			Tx: xdr.FeeBumpTransaction{
				FeeSource: xdr.MustMuxedAddress(feeSource),
				Fee:       xdr.Int64(fee),
				InnerTx: xdr.FeeBumpTransactionInnerTx{
					Type: xdr.EnvelopeTypeEnvelopeTypeTx,
					V1:   inner.V1,
				},
			},
		},
	}
}

func classicTx(source string, fee uint32) xdr.TransactionEnvelope {
	return xdr.TransactionEnvelope{
		Type: xdr.EnvelopeTypeEnvelopeTypeTx,
		V1: &xdr.TransactionV1Envelope{
			Tx: xdr.Transaction{
				Fee:           xdr.Uint32(fee),
				SourceAccount: xdr.MustMuxedAddress(source),
			},
		},
	}
}

// parallelPhaseOrderLedger builds a protocol 23 ledger with a classic phase
// holding classic and a parallel phase with ty in stage 1 and tx in stage 2. ty credits accountX
// in its operation, tx has accountX as its source, so the sequence bump of tx
// (txChangesBefore) is older in ledger time than the operation of ty.
func parallelPhaseOrderLedger(t *testing.T, classic, ty, tx xdr.TransactionEnvelope, accountX string) xdr.LedgerCloseMeta {
	hashOf := func(env xdr.TransactionEnvelope) xdr.Hash {
		h, err := network.HashTransactionInEnvelope(env, network.TestNetworkPassphrase)
		require.NoError(t, err)
		return h
	}
	return xdr.LedgerCloseMeta{
		V: 2,
		V2: &xdr.LedgerCloseMetaV2{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{Header: xdr.LedgerHeader{LedgerVersion: 23}},
			TxSet: xdr.GeneralizedTransactionSet{
				V: 1,
				V1TxSet: &xdr.TransactionSetV1{
					Phases: []xdr.TransactionPhase{
						{V: 0, V0Components: &[]xdr.TxSetComponent{{
							Type:                  xdr.TxSetComponentTypeTxsetCompTxsMaybeDiscountedFee,
							TxsMaybeDiscountedFee: &xdr.TxSetComponentTxsMaybeDiscountedFee{Txs: []xdr.TransactionEnvelope{classic}},
						}}},
						{V: 1, ParallelTxsComponent: &xdr.ParallelTxsComponent{
							ExecutionStages: []xdr.ParallelTxExecutionStage{
								{xdr.DependentTxCluster{ty}},
								{xdr.DependentTxCluster{tx}},
							},
						}},
					},
				},
			},
			TxProcessing: []xdr.TransactionResultMetaV1{
				{
					Result:        xdr.TransactionResultPair{TransactionHash: hashOf(classic)},
					FeeProcessing: xdr.LedgerEntryChanges{buildChange(feeAddress, 1)},
					TxApplyProcessing: xdr.TransactionMeta{V: 3, V3: &xdr.TransactionMetaV3{
						TxChangesBefore: xdr.LedgerEntryChanges{buildChange(metaAddress, 10)},
						Operations:      []xdr.OperationMeta{{Changes: xdr.LedgerEntryChanges{buildChange(metaAddress, 11)}}},
						TxChangesAfter:  xdr.LedgerEntryChanges{buildChange(metaAddress, 12)},
					}},
				},
				{
					Result:        xdr.TransactionResultPair{TransactionHash: hashOf(ty)},
					FeeProcessing: xdr.LedgerEntryChanges{buildChange(feeAddress, 2)},
					TxApplyProcessing: xdr.TransactionMeta{V: 3, V3: &xdr.TransactionMetaV3{
						TxChangesBefore: xdr.LedgerEntryChanges{buildChange(metaAddress, 20)},
						// Core loaded X after all sequence bumps, so X already has seq 2.
						Operations:     []xdr.OperationMeta{{Changes: buildUpdate(accountX, 100, 2, 101, 2)}},
						TxChangesAfter: xdr.LedgerEntryChanges{buildChange(metaAddress, 22)},
					}},
				},
				{
					Result:        xdr.TransactionResultPair{TransactionHash: hashOf(tx)},
					FeeProcessing: xdr.LedgerEntryChanges{buildChange(feeAddress, 3)},
					TxApplyProcessing: xdr.TransactionMeta{V: 3, V3: &xdr.TransactionMetaV3{
						// Sequence bump of X: older in ledger time than ty's operation.
						TxChangesBefore: buildUpdate(accountX, 100, 1, 100, 2),
						Operations:      []xdr.OperationMeta{{Changes: xdr.LedgerEntryChanges{buildChange(metaAddress, 31)}}},
					}},
				},
			},
		},
	}
}

// TestLedgerChangeReaderParallelPhaseTxChangesBeforeOrder verifies that, from
// protocol 23 on (CAP-63), txChangesBefore of every Soroban transaction is
// streamed before any operation change of the parallel phase. stellar-core
// bumps all sequence numbers of the phase before it applies any operation, so
// a later transaction's txChangesBefore is older in ledger time than an
// earlier transaction's operation changes.
func TestLedgerChangeReaderParallelPhaseTxChangesBeforeOrder(t *testing.T) {
	ctx := context.Background()
	mock := &ledgerbackend.MockDatabaseBackend{}
	seq := uint32(123)

	const (
		accountX = "GBXGQJWVLWOYHFLVTKWV5FGHA3LNYY2JQKM7OAJAUEQFU6LPCSEFVXON"
		accountY = "GCXKG6RN4ONIEPCMNFB732A436Z5PNDSRLGWK7GBLCMQLIFO4S7EYWVU"
	)

	classic := classicTx(feeAddress, 1)
	ty := parallelPhaseTx(accountY, 2) // credits X in its operation
	// Source X, applied in the stage after ty. Wrapped in a fee bump so the
	// parallel phase classification is also exercised for fee bump envelopes.
	tx := parallelPhaseFeeBumpTx(accountY, 3, parallelPhaseTx(accountX, 3))

	ledger := parallelPhaseOrderLedger(t, classic, ty, tx, accountX)
	mock.On("GetLedger", ctx, seq).Return(ledger, nil).Twice()

	isAccountUpdate := func(address string, postBalance, postSeq int64) changePredicate {
		return func(t *testing.T, idx int, change Change) {
			msg := fmt.Sprintf("change %d", idx)
			require.NotNil(t, change.Post, msg)
			require.Equal(t, xdr.LedgerEntryTypeAccount, change.Post.Data.Type, msg)
			assert.Equal(t, address, change.Post.Data.Account.AccountId.Address(), msg)
			assert.EqualValues(t, postBalance, change.Post.Data.Account.Balance, msg)
			assert.EqualValues(t, postSeq, change.Post.Data.Account.SeqNum, msg)
		}
	}

	assertChangesEqual(t, ctx, seq, mock, []changePredicate{
		// fee changes of all transactions
		isBalance(feeAddress, 1),
		isBalance(feeAddress, 2),
		isBalance(feeAddress, 3),
		// classic phase: per transaction order
		isBalance(metaAddress, 10),
		isBalance(metaAddress, 11),
		isBalance(metaAddress, 12),
		// parallel phase: txChangesBefore of every transaction first, across stages
		isBalance(metaAddress, 20),
		isAccountUpdate(accountX, 100, 2),
		// then operations and txChangesAfter per transaction
		isAccountUpdate(accountX, 101, 2),
		isBalance(metaAddress, 22),
		isBalance(metaAddress, 31),
	})

	// After compaction the reader must report the final ledger state of X.
	// The marker changes on the other accounts are not valid compactor input
	// (repeated "created" changes), so only the changes of X are compacted.
	reader, err := NewLedgerChangeReader(ctx, mock, network.TestNetworkPassphrase, seq)
	require.NoError(t, err)
	compactor := NewChangeCompactor(ChangeCompactorConfig{})
	for {
		change, err := reader.Read()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if change.Post != nil && change.Post.Data.Type == xdr.LedgerEntryTypeAccount &&
			change.Post.Data.Account.AccountId.Address() == accountX {
			require.NoError(t, compactor.AddChange(change))
		}
	}
	compacted := compactor.GetChanges()
	require.Len(t, compacted, 1)
	assert.EqualValues(t, 100, compacted[0].Pre.Data.Account.Balance)
	assert.EqualValues(t, 1, compacted[0].Pre.Data.Account.SeqNum)
	assert.EqualValues(t, 101, compacted[0].Post.Data.Account.Balance)
	assert.EqualValues(t, 2, compacted[0].Post.Data.Account.SeqNum)
	mock.AssertExpectations(t)
}

// TestLedgerChangeReaderUpgradeLedgerKeepsSequentialOrder verifies that the
// gate is the transaction set phase encoding, not the ledger protocol version.
// In the ledger that upgrades to protocol 23 the header already reports 23,
// but stellar-core built and applied the transaction set under protocol 22,
// so the Soroban phase is sequential and every transaction keeps the
// per-transaction order.
func TestLedgerChangeReaderUpgradeLedgerKeepsSequentialOrder(t *testing.T) {
	ctx := context.Background()
	mock := &ledgerbackend.MockDatabaseBackend{}
	seq := uint32(123)

	const (
		accountX = "GBXGQJWVLWOYHFLVTKWV5FGHA3LNYY2JQKM7OAJAUEQFU6LPCSEFVXON"
		accountY = "GCXKG6RN4ONIEPCMNFB732A436Z5PNDSRLGWK7GBLCMQLIFO4S7EYWVU"
	)

	classic := classicTx(feeAddress, 1)
	ty := parallelPhaseTx(accountY, 2)
	tx := parallelPhaseTx(accountX, 3)

	hashOf := func(env xdr.TransactionEnvelope) xdr.Hash {
		h, err := network.HashTransactionInEnvelope(env, network.TestNetworkPassphrase)
		require.NoError(t, err)
		return h
	}

	txMeta := func(before, op, after int64) xdr.TransactionMeta {
		return xdr.TransactionMeta{V: 3, V3: &xdr.TransactionMetaV3{
			TxChangesBefore: xdr.LedgerEntryChanges{buildChange(metaAddress, before)},
			Operations:      []xdr.OperationMeta{{Changes: xdr.LedgerEntryChanges{buildChange(metaAddress, op)}}},
			TxChangesAfter:  xdr.LedgerEntryChanges{buildChange(metaAddress, after)},
		}}
	}

	ledger := xdr.LedgerCloseMeta{
		V: 2,
		V2: &xdr.LedgerCloseMetaV2{
			// Header is already at 23, but both phases are sequential (V: 0).
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{Header: xdr.LedgerHeader{LedgerVersion: 23}},
			TxSet: xdr.GeneralizedTransactionSet{
				V: 1,
				V1TxSet: &xdr.TransactionSetV1{
					Phases: []xdr.TransactionPhase{
						{V: 0, V0Components: &[]xdr.TxSetComponent{{
							Type:                  xdr.TxSetComponentTypeTxsetCompTxsMaybeDiscountedFee,
							TxsMaybeDiscountedFee: &xdr.TxSetComponentTxsMaybeDiscountedFee{Txs: []xdr.TransactionEnvelope{classic}},
						}}},
						{V: 0, V0Components: &[]xdr.TxSetComponent{{
							Type:                  xdr.TxSetComponentTypeTxsetCompTxsMaybeDiscountedFee,
							TxsMaybeDiscountedFee: &xdr.TxSetComponentTxsMaybeDiscountedFee{Txs: []xdr.TransactionEnvelope{ty, tx}},
						}}},
					},
				},
			},
			TxProcessing: []xdr.TransactionResultMetaV1{
				{
					Result:            xdr.TransactionResultPair{TransactionHash: hashOf(classic)},
					FeeProcessing:     xdr.LedgerEntryChanges{buildChange(feeAddress, 1)},
					TxApplyProcessing: txMeta(10, 11, 12),
				},
				{
					Result:            xdr.TransactionResultPair{TransactionHash: hashOf(ty)},
					FeeProcessing:     xdr.LedgerEntryChanges{buildChange(feeAddress, 2)},
					TxApplyProcessing: txMeta(20, 21, 22),
				},
				{
					Result:            xdr.TransactionResultPair{TransactionHash: hashOf(tx)},
					FeeProcessing:     xdr.LedgerEntryChanges{buildChange(feeAddress, 3)},
					TxApplyProcessing: txMeta(30, 31, 32),
				},
			},
		},
	}
	mock.On("GetLedger", ctx, seq).Return(ledger, nil).Once()

	assertChangesEqual(t, ctx, seq, mock, []changePredicate{
		isBalance(feeAddress, 1),
		isBalance(feeAddress, 2),
		isBalance(feeAddress, 3),
		isBalance(metaAddress, 10),
		isBalance(metaAddress, 11),
		isBalance(metaAddress, 12),
		isBalance(metaAddress, 20),
		isBalance(metaAddress, 21),
		isBalance(metaAddress, 22),
		isBalance(metaAddress, 30),
		isBalance(metaAddress, 31),
		isBalance(metaAddress, 32),
	})
	mock.AssertExpectations(t)
}

// TestLedgerChangeReaderFlushesDeferredAtPhaseBoundary verifies that the
// deferred parallel phase changes are streamed when the phase ends, not when
// the meta pass ends. stellar-core produces exactly two phases today, but the
// XDR allows more, so the reader must not assume the parallel phase is last.
func TestLedgerChangeReaderFlushesDeferredAtPhaseBoundary(t *testing.T) {
	ctx := context.Background()
	mock := &ledgerbackend.MockDatabaseBackend{}
	seq := uint32(123)

	classic := classicTx(feeAddress, 1)
	ty := parallelPhaseTx("GCXKG6RN4ONIEPCMNFB732A436Z5PNDSRLGWK7GBLCMQLIFO4S7EYWVU", 2)
	tx := parallelPhaseTx("GBXGQJWVLWOYHFLVTKWV5FGHA3LNYY2JQKM7OAJAUEQFU6LPCSEFVXON", 3)
	after := classicTx("GDBFRRLMYGI3YEKIM5XYKYGJ5OWSKNUMGDMTCNBS2LEAE3J2ZAFR4A6O", 4)

	hashOf := func(env xdr.TransactionEnvelope) xdr.Hash {
		h, err := network.HashTransactionInEnvelope(env, network.TestNetworkPassphrase)
		require.NoError(t, err)
		return h
	}
	txMeta := func(before, op, after int64) xdr.TransactionMeta {
		return xdr.TransactionMeta{V: 3, V3: &xdr.TransactionMetaV3{
			TxChangesBefore: xdr.LedgerEntryChanges{buildChange(metaAddress, before)},
			Operations:      []xdr.OperationMeta{{Changes: xdr.LedgerEntryChanges{buildChange(metaAddress, op)}}},
			TxChangesAfter:  xdr.LedgerEntryChanges{buildChange(metaAddress, after)},
		}}
	}
	sequential := func(txs ...xdr.TransactionEnvelope) xdr.TransactionPhase {
		return xdr.TransactionPhase{V: 0, V0Components: &[]xdr.TxSetComponent{{
			Type:                  xdr.TxSetComponentTypeTxsetCompTxsMaybeDiscountedFee,
			TxsMaybeDiscountedFee: &xdr.TxSetComponentTxsMaybeDiscountedFee{Txs: txs},
		}}}
	}
	result := func(env xdr.TransactionEnvelope, fee, before, op, after int64) xdr.TransactionResultMetaV1 {
		return xdr.TransactionResultMetaV1{
			Result:            xdr.TransactionResultPair{TransactionHash: hashOf(env)},
			FeeProcessing:     xdr.LedgerEntryChanges{buildChange(feeAddress, fee)},
			TxApplyProcessing: txMeta(before, op, after),
		}
	}

	ledger := xdr.LedgerCloseMeta{
		V: 2,
		V2: &xdr.LedgerCloseMetaV2{
			LedgerHeader: xdr.LedgerHeaderHistoryEntry{Header: xdr.LedgerHeader{LedgerVersion: 23}},
			TxSet: xdr.GeneralizedTransactionSet{
				V: 1,
				V1TxSet: &xdr.TransactionSetV1{
					Phases: []xdr.TransactionPhase{
						sequential(classic),
						{V: 1, ParallelTxsComponent: &xdr.ParallelTxsComponent{
							ExecutionStages: []xdr.ParallelTxExecutionStage{{xdr.DependentTxCluster{ty, tx}}},
						}},
						// A sequential phase after the parallel one. Not produced
						// by core today, but allowed by the XDR.
						sequential(after),
					},
				},
			},
			TxProcessing: []xdr.TransactionResultMetaV1{
				result(classic, 1, 10, 11, 12),
				result(ty, 2, 20, 21, 22),
				result(tx, 3, 30, 31, 32),
				result(after, 4, 40, 41, 42),
			},
		},
	}
	mock.On("GetLedger", ctx, seq).Return(ledger, nil).Once()

	assertChangesEqual(t, ctx, seq, mock, []changePredicate{
		isBalance(feeAddress, 1),
		isBalance(feeAddress, 2),
		isBalance(feeAddress, 3),
		isBalance(feeAddress, 4),
		// sequential phase, per transaction order
		isBalance(metaAddress, 10),
		isBalance(metaAddress, 11),
		isBalance(metaAddress, 12),
		// parallel phase: all txChangesBefore, then the deferred changes
		isBalance(metaAddress, 20),
		isBalance(metaAddress, 30),
		isBalance(metaAddress, 21),
		isBalance(metaAddress, 22),
		isBalance(metaAddress, 31),
		isBalance(metaAddress, 32),
		// the phase after the parallel one is streamed after the flush
		isBalance(metaAddress, 40),
		isBalance(metaAddress, 41),
		isBalance(metaAddress, 42),
	})
	mock.AssertExpectations(t)
}
