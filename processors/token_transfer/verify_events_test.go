package token_transfer

import (
	"math/big"
	"testing"

	assetProto "github.com/stellar/go-stellar-sdk/asset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindBalanceDeltasFromEvents_Int64Amounts(t *testing.T) {
	from := accountA.Address()
	to := accountB.Address()
	asset := xlmAsset
	protoAsset := assetProto.NewProtoAsset(asset)

	meta := &EventMeta{TxHash: "abc123"}
	transfer := NewTransferEvent(meta, from, to, "1000", protoAsset)

	deltas, err := findBalanceDeltasFromEvents([]*TokenTransferEvent{transfer})
	require.NoError(t, err)

	fromKey := balanceKey{holder: from, asset: asset.StringCanonical()}
	toKey := balanceKey{holder: to, asset: asset.StringCanonical()}

	assert.Equal(t, big.NewInt(-1000), deltas[fromKey])
	assert.Equal(t, big.NewInt(1000), deltas[toKey])
}

func TestFindBalanceDeltasFromEvents_AmountExceedingInt64(t *testing.T) {
	// This is the scenario from issue #5929: a SAC token balance that
	// exceeded int64 max through cumulative mints, then was fully burned.
	from := accountA.Address()
	asset := usdcAsset
	protoAsset := assetProto.NewProtoAsset(asset)

	// 18446947143889701584 exceeds int64 max (9223372036854775807)
	largeAmount := "18446947143889701584"

	meta := &EventMeta{TxHash: "abc123"}
	burn := NewBurnEvent(meta, from, largeAmount, protoAsset)

	deltas, err := findBalanceDeltasFromEvents([]*TokenTransferEvent{burn})
	require.NoError(t, err)

	fromKey := balanceKey{holder: from, asset: asset.StringCanonical()}
	expected, ok := new(big.Int).SetString("-18446947143889701584", 10)
	require.True(t, ok)
	assert.Equal(t, expected, deltas[fromKey])
}

func TestFindBalanceDeltasFromEvents_MintLargeAmount(t *testing.T) {
	to := accountB.Address()
	asset := usdcAsset
	protoAsset := assetProto.NewProtoAsset(asset)

	// Mint an amount exceeding int64 max
	largeAmount := "18446947143889701584"

	meta := &EventMeta{TxHash: "abc123"}
	mint := NewMintEvent(meta, to, largeAmount, protoAsset)

	deltas, err := findBalanceDeltasFromEvents([]*TokenTransferEvent{mint})
	require.NoError(t, err)

	toKey := balanceKey{holder: to, asset: asset.StringCanonical()}
	expected, ok := new(big.Int).SetString("18446947143889701584", 10)
	require.True(t, ok)
	assert.Equal(t, expected, deltas[toKey])
}

func TestFindBalanceDeltasFromEvents_InvalidAmount(t *testing.T) {
	from := accountA.Address()
	to := accountB.Address()
	asset := xlmAsset
	protoAsset := assetProto.NewProtoAsset(asset)

	meta := &EventMeta{TxHash: "abc123"}
	transfer := NewTransferEvent(meta, from, to, "not_a_number", protoAsset)

	_, err := findBalanceDeltasFromEvents([]*TokenTransferEvent{transfer})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid amount")
}

func TestFindBalanceDeltasFromEvents_SkipsEventsWithoutAsset(t *testing.T) {
	from := accountA.Address()
	to := accountB.Address()

	meta := &EventMeta{TxHash: "abc123"}
	// Event with nil asset (custom token, not SAC)
	transfer := NewTransferEvent(meta, from, to, "not_a_number", nil)

	deltas, err := findBalanceDeltasFromEvents([]*TokenTransferEvent{transfer})
	require.NoError(t, err)
	assert.Empty(t, deltas)
}

func TestFindBalanceDeltasFromEvents_TransferExceedingInt64(t *testing.T) {
	from := accountA.Address()
	to := accountB.Address()
	asset := usdcAsset
	protoAsset := assetProto.NewProtoAsset(asset)

	// Transfer an amount exceeding int64 max
	largeAmount := "18446947143889701584"

	meta := &EventMeta{TxHash: "abc123"}
	transfer := NewTransferEvent(meta, from, to, largeAmount, protoAsset)

	deltas, err := findBalanceDeltasFromEvents([]*TokenTransferEvent{transfer})
	require.NoError(t, err)

	fromKey := balanceKey{holder: from, asset: asset.StringCanonical()}
	toKey := balanceKey{holder: to, asset: asset.StringCanonical()}

	expectedNeg, ok := new(big.Int).SetString("-18446947143889701584", 10)
	require.True(t, ok)
	expectedPos, ok := new(big.Int).SetString("18446947143889701584", 10)
	require.True(t, ok)

	assert.Equal(t, expectedNeg, deltas[fromKey])
	assert.Equal(t, expectedPos, deltas[toKey])
}

func TestUpdateBalanceMap_BigInt(t *testing.T) {
	m := make(map[balanceKey]*big.Int)
	key := balanceKey{holder: accountA.Address(), asset: xlmAsset.StringCanonical()}

	// Add positive amount
	updateBalanceMap(m, key, big.NewInt(100))
	assert.Equal(t, big.NewInt(100), m[key])

	// Add more
	updateBalanceMap(m, key, big.NewInt(50))
	assert.Equal(t, big.NewInt(150), m[key])

	// Subtract to zero — entry should be removed
	updateBalanceMap(m, key, big.NewInt(-150))
	_, exists := m[key]
	assert.False(t, exists)
}

func TestUpdateBalanceMap_ZeroDeltaDoesNotInsert(t *testing.T) {
	m := make(map[balanceKey]*big.Int)
	key := balanceKey{holder: accountA.Address(), asset: xlmAsset.StringCanonical()}

	// A zero delta on a missing key should not create an entry
	updateBalanceMap(m, key, big.NewInt(0))
	_, exists := m[key]
	assert.False(t, exists, "zero delta should not insert a map entry")
}

func TestUpdateBalanceMap_SkipsContractAddresses(t *testing.T) {
	m := make(map[balanceKey]*big.Int)
	// Use a valid contract address format
	contractAddr := "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	key := balanceKey{holder: contractAddr, asset: xlmAsset.StringCanonical()}

	updateBalanceMap(m, key, big.NewInt(100))
	_, exists := m[key]
	assert.False(t, exists)
}

func TestFindBalanceDeltasFromEvents_FeeEvent(t *testing.T) {
	from := accountA.Address()

	meta := &EventMeta{TxHash: "abc123"}
	fee := NewFeeEvent(meta, from, "200", assetProto.NewProtoAsset(xlmAsset))

	deltas, err := findBalanceDeltasFromEvents([]*TokenTransferEvent{fee})
	require.NoError(t, err)

	fromKey := balanceKey{holder: from, asset: xlmAsset.StringCanonical()}
	assert.Equal(t, big.NewInt(-200), deltas[fromKey])
}

func TestFindBalanceDeltasFromEvents_ClawbackLargeAmount(t *testing.T) {
	from := accountA.Address()
	asset := usdcAsset
	protoAsset := assetProto.NewProtoAsset(asset)

	largeAmount := "18446947143889701584"

	meta := &EventMeta{TxHash: "abc123"}
	clawback := NewClawbackEvent(meta, from, largeAmount, protoAsset)

	deltas, err := findBalanceDeltasFromEvents([]*TokenTransferEvent{clawback})
	require.NoError(t, err)

	fromKey := balanceKey{holder: from, asset: asset.StringCanonical()}
	expected, ok := new(big.Int).SetString("-18446947143889701584", 10)
	require.True(t, ok)
	assert.Equal(t, expected, deltas[fromKey])
}
