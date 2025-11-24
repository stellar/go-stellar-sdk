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

		// First txn post apply fee changes
		isBalance(feeAddress, 400),

		// Upgrades last
		isBalance(upgradeAddress, 2),
		isBalance(upgradeAddress, 3),
	})
	mock.AssertExpectations(t)

	mock.AssertExpectations(t)
}
