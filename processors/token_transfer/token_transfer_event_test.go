package token_transfer

import (
	"testing"
	"time"

	assetProto "github.com/stellar/go-stellar-sdk/asset"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func newTestAddress() string {
	return "someString"
}

func newTestAsset() *assetProto.Asset {
	return &assetProto.Asset{
		AssetType: &assetProto.Asset_IssuedAsset{
			IssuedAsset: &assetProto.IssuedAsset{
				AssetCode: "USDC",
				Issuer:    "GA5ZSEJYB37JRC5AVCIA5MOP4RHTM335X2KGX3IHOJAPP5RE34K4KZVN",
			},
		},
	}
}

func newTestEventMeta() *EventMeta {
	return &EventMeta{
		LedgerSequence:   12345,
		ClosedAt:         timestamppb.New(time.Now()),
		TxHash:           "abc123xyz",
		TransactionIndex: 0,
	}
}

func TestEventSerialization(t *testing.T) {
	tests := []struct {
		// test fixtureName
		fixtureName string
		// Setup the test fixture
		fixtureSetupFn func() (*TokenTransferEvent, *EventMeta, *assetProto.Asset, string, string)
		// Function to assert to see if data matches
		assertFn func(event *TokenTransferEvent) proto.Message
	}{
		{
			fixtureName: "Transfer",
			fixtureSetupFn: func() (*TokenTransferEvent, *EventMeta, *assetProto.Asset, string, string) {
				from := newTestAddress()
				to := newTestAddress()
				amount := "1000"
				meta := newTestEventMeta()
				token := newTestAsset()
				event := NewTransferEvent(meta, from, to, amount, token)
				return event, meta, token, from, amount
			},
			assertFn: func(event *TokenTransferEvent) proto.Message {
				return event.GetTransfer()
			},
		},
		{
			fixtureName: "Mint",
			fixtureSetupFn: func() (*TokenTransferEvent, *EventMeta, *assetProto.Asset, string, string) {
				to := newTestAddress()
				amount := "500"
				meta := newTestEventMeta()
				token := newTestAsset()
				event := NewMintEvent(meta, to, amount, token)
				return event, meta, token, to, amount
			},
			assertFn: func(event *TokenTransferEvent) proto.Message {
				return event.GetMint()
			},
		},
		{
			fixtureName: "Burn",
			fixtureSetupFn: func() (*TokenTransferEvent, *EventMeta, *assetProto.Asset, string, string) {
				from := newTestAddress()
				amount := "200"
				meta := newTestEventMeta()
				token := newTestAsset()
				event := NewBurnEvent(meta, from, amount, token)
				return event, meta, token, from, amount
			},
			assertFn: func(event *TokenTransferEvent) proto.Message {
				return event.GetBurn()
			},
		},
		{
			fixtureName: "Clawback",
			fixtureSetupFn: func() (*TokenTransferEvent, *EventMeta, *assetProto.Asset, string, string) {
				from := newTestAddress()
				amount := "300"
				meta := newTestEventMeta()
				token := newTestAsset()
				event := NewClawbackEvent(meta, from, amount, token)
				return event, meta, token, from, amount
			},
			assertFn: func(event *TokenTransferEvent) proto.Message {
				return event.GetClawback()
			},
		},
		{
			fixtureName: "Fee",
			fixtureSetupFn: func() (*TokenTransferEvent, *EventMeta, *assetProto.Asset, string, string) {
				from := newTestAddress()
				amount := "50"
				meta := newTestEventMeta()
				event := NewFeeEvent(meta, from, amount, xlmProtoAsset)
				return event, nil, xlmProtoAsset, from, amount // No meta for Fee event
			},
			assertFn: func(event *TokenTransferEvent) proto.Message {
				return event.GetFee()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.fixtureName, func(t *testing.T) {
			event, _, _, _, _ := tt.fixtureSetupFn()

			data, err := proto.Marshal(event)
			assert.NoError(t, err)

			var deserializedEvent TokenTransferEvent
			err = proto.Unmarshal(data, &deserializedEvent)
			assert.NoError(t, err)

			// Common assertions
			assert.True(t, proto.Equal(event.Meta, deserializedEvent.Meta))
			assert.True(t, proto.Equal(event.GetAsset(), deserializedEvent.GetAsset()))

			// Event-specific assertions via the provided getter function
			assert.True(t, proto.Equal(tt.assertFn(event), tt.assertFn(&deserializedEvent)))
		})
	}
}
