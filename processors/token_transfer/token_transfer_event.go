package token_transfer

import (
	"fmt"

	assetProto "github.com/stellar/go-stellar-sdk/asset"
	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	TransferEvent = "transfer"
	MintEvent     = "mint"
	BurnEvent     = "burn"
	ClawbackEvent = "clawback"
	FeeEvent      = "fee"
)

func NewTransferEvent(meta *EventMeta, from, to string, amount string, token *assetProto.Asset) *TokenTransferEvent {
	return &TokenTransferEvent{
		Meta: meta,
		Event: &TokenTransferEvent_Transfer{
			Transfer: &Transfer{
				From:  from,
				To:    to,
				Asset: token,

				Amount: amount,
			},
		},
	}
}

func NewMintEvent(meta *EventMeta, to string, amount string, token *assetProto.Asset) *TokenTransferEvent {
	return &TokenTransferEvent{
		Meta: meta,
		Event: &TokenTransferEvent_Mint{
			Mint: &Mint{
				To:    to,
				Asset: token,

				Amount: amount,
			},
		},
	}
}

func NewBurnEvent(meta *EventMeta, from string, amount string, token *assetProto.Asset) *TokenTransferEvent {
	return &TokenTransferEvent{
		Meta: meta,
		Event: &TokenTransferEvent_Burn{
			Burn: &Burn{
				From:   from,
				Asset:  token,
				Amount: amount,
			},
		},
	}
}

func NewClawbackEvent(meta *EventMeta, from string, amount string, token *assetProto.Asset) *TokenTransferEvent {
	return &TokenTransferEvent{
		Meta: meta,
		Event: &TokenTransferEvent_Clawback{
			Clawback: &Clawback{
				From:   from,
				Asset:  token,
				Amount: amount,
			},
		},
	}
}

func NewFeeEvent(meta *EventMeta, from string, amount string, token *assetProto.Asset) *TokenTransferEvent {
	return &TokenTransferEvent{
		Meta: meta,
		Event: &TokenTransferEvent_Fee{
			Fee: &Fee{
				From:   from,
				Asset:  token,
				Amount: amount,
			},
		},
	}
}

func NewEventMetaFromTx(tx ingest.LedgerTransaction, operationIndex *uint32, contractAddress string) *EventMeta {
	// The input operationIndex is 0-indexed.
	// As per SEP-35, the OperationIndex in the output proto should be 1-indexed.
	// Make that conversion here
	var outputOpIndex *uint32
	if operationIndex != nil {
		temp := *operationIndex + 1
		outputOpIndex = &temp
	}
	return &EventMeta{
		LedgerSequence:   tx.Ledger.LedgerSequence(),
		ClosedAt:         timestamppb.New(tx.Ledger.ClosedAt()),
		TxHash:           tx.Hash.HexString(),
		TransactionIndex: tx.Index, // The index in ingest.LedgerTransaction is already 1-indexed
		OperationIndex:   outputOpIndex,
		ContractAddress:  contractAddress,
	}
}

func (event *TokenTransferEvent) GetEventType() string {
	switch event.GetEvent().(type) {
	case *TokenTransferEvent_Transfer:
		return TransferEvent
	case *TokenTransferEvent_Mint:
		return MintEvent
	case *TokenTransferEvent_Burn:
		return BurnEvent
	case *TokenTransferEvent_Clawback:
		return ClawbackEvent
	case *TokenTransferEvent_Fee:
		return FeeEvent
	default:
		return "Unknown"
	}
}

func (event *TokenTransferEvent) GetAsset() *assetProto.Asset {
	var asset *assetProto.Asset
	switch event.GetEvent().(type) {
	case *TokenTransferEvent_Mint:
		asset = event.GetMint().GetAsset()
	case *TokenTransferEvent_Burn:
		asset = event.GetBurn().GetAsset()
	case *TokenTransferEvent_Clawback:
		asset = event.GetClawback().GetAsset()
	case *TokenTransferEvent_Fee:
		asset = event.GetFee().GetAsset()
	case *TokenTransferEvent_Transfer:
		asset = event.GetTransfer().GetAsset()
	default:
		panic(fmt.Errorf("unkown event type:%v", event))
	}
	return asset
}

func (event *TokenTransferEvent) SetAsset(asset xdr.Asset) {
	protoAsset := assetProto.NewProtoAsset(asset)
	switch event.GetEvent().(type) {
	case *TokenTransferEvent_Mint:
		event.GetMint().Asset = protoAsset
	case *TokenTransferEvent_Burn:
		event.GetBurn().Asset = protoAsset
	case *TokenTransferEvent_Clawback:
		event.GetClawback().Asset = protoAsset
	case *TokenTransferEvent_Fee:
		event.GetFee().Asset = protoAsset
	case *TokenTransferEvent_Transfer:
		event.GetTransfer().Asset = protoAsset
	default:
		panic(fmt.Errorf("unknown event type:%v", event))
	}
}

func (event *TokenTransferEvent) GetAmount() string {
	var amount string
	switch event.GetEvent().(type) {
	case *TokenTransferEvent_Mint:
		amount = event.GetMint().GetAmount()
	case *TokenTransferEvent_Burn:
		amount = event.GetBurn().GetAmount()
	case *TokenTransferEvent_Clawback:
		amount = event.GetClawback().GetAmount()
	case *TokenTransferEvent_Fee:
		amount = event.GetFee().GetAmount()
	case *TokenTransferEvent_Transfer:
		amount = event.GetTransfer().GetAmount()
	default:
		panic(fmt.Errorf("unkown event type:%v", event))
	}
	return amount
}

func (e *TokenTransferEvent) setDestinationMuxedInfo(to string, tx ingest.LedgerTransaction) error {
	// Destination Mux info needs to be set only for accountAddresses, and not for LPs or CBs.
	// This is as per CAP-67
	if !isAccountAddress(to) {
		return nil
	}

	if strkey.IsValidMuxedAccountEd25519PublicKey(to) {
		muxedAcc := xdr.MustMuxedAddress(to)
		muxedId, err := muxedAcc.GetId()
		if err != nil {
			return fmt.Errorf("could not get muxed account id: %w", err)
		}
		e.Meta.ToMuxedInfo = NewMuxedInfoFromId(muxedId)
		return nil
	}

	txMemo := tx.Envelope.Memo()
	e.Meta.ToMuxedInfo = NewMuxedInfoFromMemo(txMemo)
	return nil
}
