package contractevents

import (
	"fmt"
	"strings"

	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/xdr"
)

type Event = xdr.ContractEvent
type EventType int

// Note that there is no distinction between transfer() and transfer_from() in events,
// nor the other *_from variants. This is intentional from the host environment.

const (
	// Implemented
	EventTypeTransfer EventType = iota
	EventTypeMint
	EventTypeClawback
	EventTypeBurn
	// TODO: Not implemented
	EventTypeIncrAllow
	EventTypeDecrAllow
	EventTypeSetAuthorized
	EventTypeSetAdmin
)

var (
	STELLAR_ASSET_CONTRACT_TOPICS = map[xdr.ScSymbol]EventType{
		xdr.ScSymbol("transfer"): EventTypeTransfer,
		xdr.ScSymbol("mint"):     EventTypeMint,
		xdr.ScSymbol("clawback"): EventTypeClawback,
		xdr.ScSymbol("burn"):     EventTypeBurn,
	}

	ErrNotStellarAssetContract = errors.New("event was not from a Stellar Asset Contract")
	ErrEventUnsupported        = errors.New("this type of Stellar Asset Contract event is unsupported")
	ErrEventIntegrity          = errors.New("contract ID doesn't match asset + passphrase")
)

type StellarAssetContractEvent interface {
	GetType() EventType
	GetAsset() xdr.Asset
}

type sacEvent struct {
	Type  EventType
	Asset xdr.Asset
}

func (e sacEvent) GetAsset() xdr.Asset {
	return e.Asset
}

func (e sacEvent) GetType() EventType {
	return e.Type
}

func NewStellarAssetContractEvent(event *Event, networkPassphrase string) (StellarAssetContractEvent, error) {
	evt := &sacEvent{}

	if event.Type != xdr.ContractEventTypeContract || event.ContractId == nil || event.Body.V != 0 {
		return evt, ErrNotStellarAssetContract
	}

	// SAC event topics take the form <fn name>/<params...>/<token name>.
	//
	// For specific event forms, see here:
	// https://github.com/stellar/rs-soroban-env/blob/main/soroban-env-host/src/native_contract/token/event.rs#L44-L49
	topics := event.Body.V0.Topics
	value := event.Body.V0.Data

	// No relevant SAC events have <= 2 topics
	if len(topics) <= 2 {
		return evt, ErrNotStellarAssetContract
	}

	// Filter out events for function calls we don't care about
	fn, ok := topics[0].GetSym()
	if !ok {
		return evt, ErrNotStellarAssetContract
	}

	if eventType, found := STELLAR_ASSET_CONTRACT_TOPICS[fn]; !found {
		return evt, ErrNotStellarAssetContract
	} else {
		evt.Type = eventType
	}

	// This looks like a SAC event, but does it act like a SAC event?
	//
	// To check that, ensure that the contract ID of the event matches the
	// contract ID that *would* represent the asset the event is claiming to
	// be as included as the last topic in canonical asset encoding.
	//
	// For all parsing errors, we just continue, since it's not a real error,
	// just an event non-complaint with SAC events.
	rawAsset := topics[len(topics)-1]
	assetSc, ok := rawAsset.GetStr()
	if !ok || assetSc == "" {
		return evt, ErrNotStellarAssetContract
	}

	asset, err := parseCanonicalAsset(string(assetSc))
	if err != nil {
		return evt, errors.Wrap(ErrNotStellarAssetContract, err.Error())
	}

	evt.Asset = *asset
	expectedId, err := evt.Asset.ContractID(networkPassphrase)
	if err != nil {
		return evt, errors.Wrap(ErrNotStellarAssetContract, err.Error())
	}

	// This is the DEFINITIVE integrity check for whether or not this is a
	// SAC event. At this point, we can parse the event and treat it as
	// truth, mapping it to effects where appropriate.
	if expectedId != *event.ContractId { // nil check was earlier
		return evt, ErrEventIntegrity
	}

	switch evt.GetType() {
	case EventTypeTransfer:
		transferEvent := TransferEvent{sacEvent: *evt}
		return &transferEvent, transferEvent.parse(topics, value)

	case EventTypeMint:
		mintEvent := MintEvent{sacEvent: *evt}
		return &mintEvent, mintEvent.parse(topics, value)

	case EventTypeClawback:
		cbEvent := ClawbackEvent{sacEvent: *evt}
		return &cbEvent, cbEvent.parse(topics, value)

	case EventTypeBurn:
		burnEvent := BurnEvent{sacEvent: *evt}
		return &burnEvent, burnEvent.parse(topics, value)

	default:
		return evt, errors.Wrapf(ErrEventUnsupported,
			"event type %d ('%s') unsupported", evt.Type, fn)
	}
}

func parseCanonicalAsset(assetStr string) (*xdr.Asset, error) {
	// The asset is in canonical SEP-11 form:
	//  https://stellar.org/protocol/sep-11#alphanum4-alphanum12
	// namely, its split by colon, first part is asset code padded to
	// exactly 4 or 12 bytes. and second part is issuer encoded
	// as strkey
	asset := xdr.Asset{
		Type: xdr.AssetTypeAssetTypeNative,
	}

	if assetStr == "native" {
		return &asset, nil
	}

	parts := strings.Split(assetStr, ":")
	if len(parts) != 2 {
		return nil, errors.New("invalid asset byte format (expected canonical <code>:<issuer>)")
	}
	rawCode, rawIssuerKey := parts[0], parts[1]

	issuerKey, err := xdr.AddressToAccountId(rawIssuerKey)
	if err != nil {
		return nil, errors.New("invalid asset byte format (expected canonical <code>:<issuer>)")
	}
	accountId := xdr.AccountId(xdr.PublicKey{
		Type:    xdr.PublicKeyTypePublicKeyTypeEd25519,
		Ed25519: issuerKey.Ed25519,
	})

	if len(rawCode) <= 4 {
		code := [4]byte{}
		copy(code[:], rawCode[:])

		asset.Type = xdr.AssetTypeAssetTypeCreditAlphanum4
		asset.AlphaNum4 = &xdr.AlphaNum4{
			AssetCode: xdr.AssetCode4(code),
			Issuer:    accountId,
		}
	} else if len(rawCode) <= 12 {
		code := [12]byte{}
		copy(code[:], rawCode[:])

		asset.Type = xdr.AssetTypeAssetTypeCreditAlphanum12
		asset.AlphaNum12 = &xdr.AlphaNum12{
			AssetCode: xdr.AssetCode12(code),
			Issuer:    accountId,
		}
	} else {
		return nil, fmt.Errorf(
			"asset code invalid (expected 4 or 12 bytes, got %d: '%v' or '%s')",
			len(rawCode), rawCode, string(rawCode))
	}

	return &asset, nil
}
