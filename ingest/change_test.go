package ingest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/xdr"
)

func assertChangesAreEqual(t *testing.T, a, b Change) {
	assert.Equal(t, a.Type, b.Type)
	if a.Pre == nil {
		assert.Nil(t, b.Pre)
	} else {
		aBytes, err := a.Pre.MarshalBinary()
		assert.NoError(t, err)
		bBytes, err := b.Pre.MarshalBinary()
		assert.NoError(t, err)
		assert.Equal(t, aBytes, bBytes)
	}
	if a.Post == nil {
		assert.Nil(t, b.Post)
	} else {
		aBytes, err := a.Post.MarshalBinary()
		assert.NoError(t, err)
		bBytes, err := b.Post.MarshalBinary()
		assert.NoError(t, err)
		assert.Equal(t, aBytes, bBytes)
	}
}

func TestSortChanges(t *testing.T) {
	for _, testCase := range []struct {
		input    []Change
		expected []Change
	}{
		{[]Change{}, []Change{}},
		{
			[]Change{
				{
					Type: xdr.LedgerEntryTypeAccount,
					Pre:  nil,
					Post: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeAccount,
							Account: &xdr.AccountEntry{
								AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							},
						},
					},
				},
			},
			[]Change{
				{
					Type: xdr.LedgerEntryTypeAccount,
					Pre:  nil,
					Post: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeAccount,
							Account: &xdr.AccountEntry{
								AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
							},
						},
					},
				},
			},
		},
		{
			[]Change{
				{
					Type: xdr.LedgerEntryTypeAccount,
					Pre:  nil,
					Post: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeAccount,
							Account: &xdr.AccountEntry{
								AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
								Balance:   25,
							},
						},
					},
				},
				{
					Type: xdr.LedgerEntryTypeAccount,
					Pre: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeAccount,
							Account: &xdr.AccountEntry{
								AccountId: xdr.MustAddress("GCMNSW2UZMSH3ZFRLWP6TW2TG4UX4HLSYO5HNIKUSFMLN2KFSF26JKWF"),
								Balance:   20,
							},
						},
					},
					Post: nil,
				},
				{
					Type: xdr.LedgerEntryTypeTtl,
					Pre: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeTtl,
							Ttl: &xdr.TtlEntry{
								KeyHash:            xdr.Hash{1},
								LiveUntilLedgerSeq: 50,
							},
						},
					},
					Post: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeTtl,
							Ttl: &xdr.TtlEntry{
								KeyHash:            xdr.Hash{1},
								LiveUntilLedgerSeq: 100,
							},
						},
					},
				},
				{
					Type: xdr.LedgerEntryTypeAccount,
					Pre: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeAccount,
							Account: &xdr.AccountEntry{
								AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
								Balance:   25,
							},
						},
					},
					Post: nil,
				},
			},
			[]Change{
				{
					Type: xdr.LedgerEntryTypeAccount,
					Pre: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeAccount,
							Account: &xdr.AccountEntry{
								AccountId: xdr.MustAddress("GCMNSW2UZMSH3ZFRLWP6TW2TG4UX4HLSYO5HNIKUSFMLN2KFSF26JKWF"),
								Balance:   20,
							},
						},
					},
					Post: nil,
				},
				{
					Type: xdr.LedgerEntryTypeAccount,
					Pre:  nil,
					Post: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeAccount,
							Account: &xdr.AccountEntry{
								AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
								Balance:   25,
							},
						},
					},
				},
				{
					Type: xdr.LedgerEntryTypeAccount,
					Pre: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeAccount,
							Account: &xdr.AccountEntry{
								AccountId: xdr.MustAddress("GC3C4AKRBQLHOJ45U4XG35ESVWRDECWO5XLDGYADO6DPR3L7KIDVUMML"),
								Balance:   25,
							},
						},
					},
					Post: nil,
				},

				{
					Type: xdr.LedgerEntryTypeTtl,
					Pre: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeTtl,
							Ttl: &xdr.TtlEntry{
								KeyHash:            xdr.Hash{1},
								LiveUntilLedgerSeq: 50,
							},
						},
					},
					Post: &xdr.LedgerEntry{
						LastModifiedLedgerSeq: 11,
						Data: xdr.LedgerEntryData{
							Type: xdr.LedgerEntryTypeTtl,
							Ttl: &xdr.TtlEntry{
								KeyHash:            xdr.Hash{1},
								LiveUntilLedgerSeq: 100,
							},
						},
					},
				},
			},
		},
	} {
		sortChanges(testCase.input)
		assert.Equal(t, len(testCase.input), len(testCase.expected))
		for i := range testCase.input {
			assertChangesAreEqual(t, testCase.input[i], testCase.expected[i])
		}
	}
}

func createContractDataEntry() *xdr.ContractDataEntry {
	scVal := true
	return &xdr.ContractDataEntry{
		Contract: xdr.ScAddress{
			Type:       xdr.ScAddressTypeScAddressTypeContract,
			ContractId: &xdr.ContractId{0xca},
		},
		Key: xdr.ScVal{
			Type: xdr.ScValTypeScvBool,
			B:    &scVal,
		},
	}
}

func TestRestoreChange(t *testing.T) {
	contractDataEntry := createContractDataEntry()

	ledgerEntries := xdr.LedgerEntryChanges{
		xdr.LedgerEntryChange{
			Type: xdr.LedgerEntryChangeTypeLedgerEntryRestored,
			Restored: &xdr.LedgerEntry{
				Data: xdr.LedgerEntryData{
					Type:         xdr.LedgerEntryTypeContractData,
					ContractData: contractDataEntry,
				},
			},
		},
	}

	changes := GetChangesFromLedgerEntryChanges(ledgerEntries)

	change := changes[0]
	require.Equal(t, change.ChangeType, xdr.LedgerEntryChangeTypeLedgerEntryRestored)
	require.Equal(t, change.Type, xdr.LedgerEntryTypeContractData)
	require.Nil(t, change.Pre)
	require.Equal(t, contractDataEntry, change.Post.Data.ContractData)
}

func TestInvalidRestoreChange(t *testing.T) {
	contractDataEntry := createContractDataEntry()

	ledgerEntries := xdr.LedgerEntryChanges{
		xdr.LedgerEntryChange{
			Type: xdr.LedgerEntryChangeTypeLedgerEntryRestored,
			// Created instead of Restored
			Created: &xdr.LedgerEntry{
				Data: xdr.LedgerEntryData{
					Type:         xdr.LedgerEntryTypeContractData,
					ContractData: contractDataEntry,
				},
			},
		},
	}

	f := func() {
		GetChangesFromLedgerEntryChanges(ledgerEntries)
	}
	require.Panics(t, f)
}

func TestRemoveChangeWithRestore(t *testing.T) {
	contractDataEntry := createContractDataEntry()

	var ledgerEntry xdr.LedgerEntryData
	ledgerEntry.SetContractData(contractDataEntry)
	ledgerKey, err := ledgerEntry.LedgerKey()
	require.NoError(t, err)

	ledgerEntries := xdr.LedgerEntryChanges{
		xdr.LedgerEntryChange{
			Type: xdr.LedgerEntryChangeTypeLedgerEntryRestored,
			Restored: &xdr.LedgerEntry{
				Data: xdr.LedgerEntryData{
					Type:         xdr.LedgerEntryTypeContractData,
					ContractData: contractDataEntry,
				},
			},
		},
		xdr.LedgerEntryChange{
			Type:    xdr.LedgerEntryChangeTypeLedgerEntryRemoved,
			Removed: &ledgerKey,
		},
	}

	changes := GetChangesFromLedgerEntryChanges(ledgerEntries)

	change := changes[0]
	require.Equal(t, change.ChangeType, xdr.LedgerEntryChangeTypeLedgerEntryRestored)
	require.Equal(t, change.Type, xdr.LedgerEntryTypeContractData)
	require.Nil(t, change.Pre)
	require.Equal(t, contractDataEntry, change.Post.Data.ContractData)

	change = changes[1]
	require.Equal(t, change.ChangeType, xdr.LedgerEntryChangeTypeLedgerEntryRemoved)
	require.Equal(t, change.Type, xdr.LedgerEntryTypeContractData)
	require.Equal(t, contractDataEntry, change.Pre.Data.ContractData)
	require.Nil(t, change.Post)
}

func TestRemoveChangeWithState(t *testing.T) {
	contractDataEntry := createContractDataEntry()

	var ledgerEntry xdr.LedgerEntryData
	ledgerEntry.SetContractData(contractDataEntry)
	ledgerKey, err := ledgerEntry.LedgerKey()
	require.NoError(t, err)

	ledgerEntries := xdr.LedgerEntryChanges{
		xdr.LedgerEntryChange{
			Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
			State: &xdr.LedgerEntry{
				Data: xdr.LedgerEntryData{
					Type:         xdr.LedgerEntryTypeContractData,
					ContractData: contractDataEntry,
				},
			},
		},
		xdr.LedgerEntryChange{
			Type:    xdr.LedgerEntryChangeTypeLedgerEntryRemoved,
			Removed: &ledgerKey,
		},
	}

	changes := GetChangesFromLedgerEntryChanges(ledgerEntries)
	change := changes[0]
	require.Equal(t, change.ChangeType, xdr.LedgerEntryChangeTypeLedgerEntryRemoved)
	require.Equal(t, change.Type, xdr.LedgerEntryTypeContractData)
	require.Equal(t, contractDataEntry, change.Pre.Data.ContractData)
	require.Nil(t, change.Post)
}

func TestInvalidRemoveChange(t *testing.T) {
	contractDataEntry := createContractDataEntry()

	var ledgerEntry xdr.LedgerEntryData
	ledgerEntry.SetContractData(contractDataEntry)
	ledgerKey, err := ledgerEntry.LedgerKey()
	require.NoError(t, err)

	ledgerEntries := xdr.LedgerEntryChanges{
		xdr.LedgerEntryChange{
			Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
			State: &xdr.LedgerEntry{
				Data: xdr.LedgerEntryData{
					Type:         xdr.LedgerEntryTypeContractData,
					ContractData: contractDataEntry,
				},
			},
		},
		xdr.LedgerEntryChange{
			Type: xdr.LedgerEntryChangeTypeLedgerEntryCreated,
			Created: &xdr.LedgerEntry{
				Data: xdr.LedgerEntryData{
					Type:         xdr.LedgerEntryTypeContractData,
					ContractData: contractDataEntry,
				},
			},
		},
		// Remove change without an associated State or Restored change
		xdr.LedgerEntryChange{
			Type:    xdr.LedgerEntryChangeTypeLedgerEntryRemoved,
			Removed: &ledgerKey,
		},
	}
	f := func() {
		GetChangesFromLedgerEntryChanges(ledgerEntries)
	}
	require.Panics(t, f)
}

func TestUpdateChangeWithRestore(t *testing.T) {
	contractDataEntry := createContractDataEntry()

	ledgerEntries := xdr.LedgerEntryChanges{
		xdr.LedgerEntryChange{
			Type: xdr.LedgerEntryChangeTypeLedgerEntryRestored,
			Restored: &xdr.LedgerEntry{
				Data: xdr.LedgerEntryData{
					Type:         xdr.LedgerEntryTypeContractData,
					ContractData: contractDataEntry,
				},
			},
		},
		xdr.LedgerEntryChange{
			Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
			Updated: &xdr.LedgerEntry{
				Data: xdr.LedgerEntryData{
					Type:         xdr.LedgerEntryTypeContractData,
					ContractData: contractDataEntry,
				},
			},
		},
	}

	changes := GetChangesFromLedgerEntryChanges(ledgerEntries)

	change := changes[0]
	require.Equal(t, change.ChangeType, xdr.LedgerEntryChangeTypeLedgerEntryRestored)
	require.Equal(t, change.Type, xdr.LedgerEntryTypeContractData)
	require.Nil(t, change.Pre)
	require.Equal(t, contractDataEntry, change.Post.Data.ContractData)

	change = changes[1]
	require.Equal(t, change.ChangeType, xdr.LedgerEntryChangeTypeLedgerEntryUpdated)
	require.Equal(t, change.Type, xdr.LedgerEntryTypeContractData)
	require.Equal(t, contractDataEntry, change.Pre.Data.ContractData)
	require.Equal(t, contractDataEntry, change.Post.Data.ContractData)
}

func TestUpdateChangeWithState(t *testing.T) {
	contractDataEntry := createContractDataEntry()

	ledgerEntries := xdr.LedgerEntryChanges{
		xdr.LedgerEntryChange{
			Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
			State: &xdr.LedgerEntry{
				Data: xdr.LedgerEntryData{
					Type:         xdr.LedgerEntryTypeContractData,
					ContractData: contractDataEntry,
				},
			},
		},
		xdr.LedgerEntryChange{
			Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
			Updated: &xdr.LedgerEntry{
				Data: xdr.LedgerEntryData{
					Type:         xdr.LedgerEntryTypeContractData,
					ContractData: contractDataEntry,
				},
			},
		},
	}

	changes := GetChangesFromLedgerEntryChanges(ledgerEntries)

	change := changes[0]
	require.Equal(t, change.ChangeType, xdr.LedgerEntryChangeTypeLedgerEntryUpdated)
	require.Equal(t, change.Type, xdr.LedgerEntryTypeContractData)
	require.Equal(t, contractDataEntry, change.Pre.Data.ContractData)
	require.Equal(t, contractDataEntry, change.Post.Data.ContractData)
}

func TestInvalidUpdateChange(t *testing.T) {
	contractDataEntry := createContractDataEntry()

	ledgerEntries := xdr.LedgerEntryChanges{
		xdr.LedgerEntryChange{
			Type: xdr.LedgerEntryChangeTypeLedgerEntryState,
			State: &xdr.LedgerEntry{
				Data: xdr.LedgerEntryData{
					Type:         xdr.LedgerEntryTypeContractData,
					ContractData: contractDataEntry,
				},
			},
		},
		xdr.LedgerEntryChange{
			Type: xdr.LedgerEntryChangeTypeLedgerEntryCreated,
			Created: &xdr.LedgerEntry{
				Data: xdr.LedgerEntryData{
					Type:         xdr.LedgerEntryTypeContractData,
					ContractData: contractDataEntry,
				},
			},
		},
		// Update change without an associated State or Restored change
		xdr.LedgerEntryChange{
			Type: xdr.LedgerEntryChangeTypeLedgerEntryUpdated,
			Updated: &xdr.LedgerEntry{
				Data: xdr.LedgerEntryData{
					Type:         xdr.LedgerEntryTypeContractData,
					ContractData: contractDataEntry,
				},
			},
		},
	}

	f := func() {
		GetChangesFromLedgerEntryChanges(ledgerEntries)
	}
	require.Panics(t, f)
}
