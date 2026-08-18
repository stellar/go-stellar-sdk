package token_transfer

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// These tests cover the V4 "to_muxed_id" field of the token event data map,
// parsed by parseV4MapDataForTokenEvents in contract_events.go.
// They are expected to FAIL against the current implementation.

// v4MapTransferEvent builds a V4 transfer contract event whose data is an ScMap.
func v4MapTransferEvent(entries xdr.ScMap) xdr.ContractEvent {
	mapPtr := &entries
	return xdr.ContractEvent{
		Type:       xdr.ContractEventTypeContract,
		ContractId: &someContractId1,
		Body: xdr.ContractEventBody{
			V: 0,
			V0: &xdr.ContractEventV0{
				Topics: []xdr.ScVal{
					createSymbol(TransferEvent),
					createAddress(randomAccount),
					createAddress(someContract1),
				},
				Data: xdr.ScVal{Type: xdr.ScValTypeScvMap, Map: &mapPtr},
			},
		},
	}
}

// v4TxWithOperationEvents builds a V4 (protocol 23) transaction carrying the
// given contract events on its single operation.
func v4TxWithOperationEvents(events ...xdr.ContractEvent) ingest.LedgerTransaction {
	tx := someTxV3()
	tx.UnsafeMeta.V = 4
	tx.UnsafeMeta.V4 = &xdr.TransactionMetaV4{
		Operations: []xdr.OperationMetaV2{{Events: events}},
	}
	return tx
}

func scvVoid() xdr.ScVal {
	return xdr.ScVal{Type: xdr.ScValTypeScvVoid}
}

func scvBytes(b []byte) xdr.ScVal {
	scBytes := xdr.ScBytes(b)
	return xdr.ScVal{Type: xdr.ScValTypeScvBytes, Bytes: &scBytes}
}

// TestV4ToMuxedIdAbsent is the baseline: omitting to_muxed_id entirely is
// treated as "no muxed destination". This passes today, and is what makes the
// rejection of an explicit void value (below) inconsistent.
func TestV4ToMuxedIdAbsent(t *testing.T) {
	tx := v4TxWithOperationEvents()
	event, err := processor.parseEvent(tx, &someOperationIndex, v4MapTransferEvent(xdr.ScMap{
		{Key: createSymbol("amount"), Val: createInt128(thousand)},
	}))

	require.NoError(t, err)
	require.NotNil(t, event.GetTransfer())
	assert.Equal(t, thousandStr, event.GetTransfer().Amount)
	assert.Nil(t, event.Meta.ToMuxedInfo)
}

// TestV4ToMuxedIdVoid asserts that an explicitly void to_muxed_id means the same
// thing as an absent one: no muxed destination, event still emitted.
//
// Current behaviour: ScvVoid falls into the default arm of the type switch in
// parseV4MapDataForTokenEvents, which errors with
// "invalid to_muxed_id type for data: ScValTypeScvVoid".
func TestV4ToMuxedIdVoid(t *testing.T) {
	tx := v4TxWithOperationEvents()
	event, err := processor.parseEvent(tx, &someOperationIndex, v4MapTransferEvent(xdr.ScMap{
		{Key: createSymbol("amount"), Val: createInt128(thousand)},
		{Key: createSymbol("to_muxed_id"), Val: scvVoid()},
	}))

	require.NoError(t, err)
	require.NotNil(t, event)
	require.NotNil(t, event.GetTransfer())
	assert.Equal(t, thousandStr, event.GetTransfer().Amount)
	assert.Nil(t, event.Meta.ToMuxedInfo)
}

// TestV4ToMuxedIdVoidIsSilentlyDropped shows the consequence of the error above
// at the level callers actually see: contractEventsFromOperation discards any
// event that fails to parse (treating it as "not SEP-41"), so the whole transfer
// disappears from the output with no error and nothing logged.
func TestV4ToMuxedIdVoidIsSilentlyDropped(t *testing.T) {
	contractEvent := v4MapTransferEvent(xdr.ScMap{
		{Key: createSymbol("amount"), Val: createInt128(thousand)},
		{Key: createSymbol("to_muxed_id"), Val: scvVoid()},
	})
	tx := v4TxWithOperationEvents(contractEvent)

	events, err := processor.contractEventsFromOperation(tx, someOperationIndex)

	require.NoError(t, err)
	require.Len(t, events, 1, "transfer event was silently dropped instead of being emitted")
	assert.NotNil(t, events[0].GetTransfer())
}

// TestV4ToMuxedIdHashBytes covers the ScvBytes form of to_muxed_id, which is
// copied into a fixed 32 byte buffer:
//
//	hashBytes := make([]byte, 32)
//	copy(hashBytes, val)
//
// A 32 byte value round-trips (subtest "exactly 32 bytes", passes today). Any
// other length is reshaped instead of rejected: shorter values are zero-padded
// on the right, longer values are truncated. Both subtests assert that a
// malformed length is reported as an error rather than silently reinterpreted.
func TestV4ToMuxedIdHashBytes(t *testing.T) {
	tx := v4TxWithOperationEvents()

	parse := func(t *testing.T, val xdr.ScVal) (*TokenTransferEvent, error) {
		t.Helper()
		return processor.parseEvent(tx, &someOperationIndex, v4MapTransferEvent(xdr.ScMap{
			{Key: createSymbol("amount"), Val: createInt128(thousand)},
			{Key: createSymbol("to_muxed_id"), Val: val},
		}))
	}

	t.Run("exactly 32 bytes", func(t *testing.T) {
		hash := bytes.Repeat([]byte{0xab}, 32)

		event, err := parse(t, scvBytes(hash))

		require.NoError(t, err)
		require.NotNil(t, event.Meta.ToMuxedInfo)
		assert.Equal(t, hash, event.Meta.ToMuxedInfo.GetHash())
	})

	t.Run("fewer than 32 bytes is rejected", func(t *testing.T) {
		short := []byte{1, 2, 3, 4}

		event, err := parse(t, scvBytes(short))

		require.Error(t, err, "4 byte to_muxed_id was zero-padded into a 32 byte hash: %v",
			eventHash(event))
		assert.Contains(t, err.Error(), "to_muxed_id")
	})

	t.Run("more than 32 bytes is rejected", func(t *testing.T) {
		long := bytes.Repeat([]byte{0xcd}, 40)

		event, err := parse(t, scvBytes(long))

		require.Error(t, err, "40 byte to_muxed_id was truncated to 32 bytes: %v",
			eventHash(event))
		assert.Contains(t, err.Error(), "to_muxed_id")
	})
}

// eventHash is a test-only accessor used in failure messages.
func eventHash(event *TokenTransferEvent) []byte {
	if event == nil || event.Meta == nil || event.Meta.ToMuxedInfo == nil {
		return nil
	}
	return event.Meta.ToMuxedInfo.GetHash()
}
