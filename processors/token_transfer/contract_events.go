package token_transfer

import (
	"fmt"

	"github.com/stellar/go-stellar-sdk/amount"
	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

type ErrNotSep41TokenEvent struct {
	Message string
}

type InvalidFeeEvent struct {
	Message string
}

func errInvalidFeeEvent(msg string) InvalidFeeEvent {
	return InvalidFeeEvent{Message: msg}
}
func errInvalidFeeEventFromError(err error) InvalidFeeEvent {
	return InvalidFeeEvent{Message: err.Error()}
}

func (e InvalidFeeEvent) Error() string {
	return e.Message
}

func (e ErrNotSep41TokenEvent) Error() string {
	return e.Message
}

func errNotSep41TokenFromMsg(msg string) ErrNotSep41TokenEvent {
	return ErrNotSep41TokenEvent{msg}
}

func errNotSep41TokenFromError(err error) ErrNotSep41TokenEvent {
	return ErrNotSep41TokenEvent{err.Error()}
}

func (p *EventsProcessor) parseFeeEventsFromTransactionEvents(tx ingest.LedgerTransaction) ([]*TokenTransferEvent, error) {
	txHash := tx.Hash.HexString()
	txEvents, err := tx.GetTransactionEvents()
	if err != nil {
		return nil, fmt.Errorf("error parsing tx events for txHash: %v, error: %w", txHash, err)
	}

	var feeEvents []*TokenTransferEvent
	for _, ev := range txEvents.TransactionEvents {
		contractEvent := ev.Event
		// Validate basic contract contractEvent structure
		if contractEvent.Type != xdr.ContractEventTypeContract ||
			contractEvent.ContractId == nil ||
			contractEvent.Body.V != 0 {
			return nil, errInvalidFeeEvent(fmt.Sprintf("Invalid feeEvent format"))
		}

		topics := contractEvent.Body.V0.Topics
		value := contractEvent.Body.V0.Data

		// Extract the contractEvent function name
		fn, ok := topics[0].GetSym()
		if !ok {
			continue // this is to account for future proofing where xdr.TransactionEvents might be extended to include more than just fees.
		}
		if string(fn) != FeeEvent {
			continue
		}

		// Now that we have established it is a Fee event, it will need to undergo stricter checks
		if len(topics) != 2 {
			return nil, errInvalidFeeEvent(fmt.Sprintf("invalid topic length for fee event for txHash: %v, topicLength: %v", txHash, len(topics)))
		}

		// Parse token amount. If that fails, then no need to bother checking for eventType
		amt, ok := value.GetI128()
		if !ok {
			return nil, errInvalidFeeEvent(fmt.Sprintf("invalid fee amount event amount: %v", value.String()))
		}
		amtRaw128 := amount.String128Raw(amt)

		from, err := extractAddress(topics[1])
		if err != nil {
			return nil, errInvalidFeeEventFromError(fmt.Errorf("invalid fromAddress. error: %w", err))
		}

		// Verify contract ID matches expected native asset contract ID
		expectedId, idErr := xlmAsset.ContractID(p.networkPassphrase)
		if idErr != nil {
			return nil, errInvalidFeeEventFromError(fmt.Errorf("invalid contract id error: %w", idErr))
		} else if expectedId != *contractEvent.ContractId {
			return nil, errInvalidFeeEventFromError(fmt.Errorf("contractId in event does not match xlm SAC contract Id, eventContractId: %v", contractEvent.ContractId))
		}

		meta := p.generateEventMeta(tx, nil, xlmAsset)
		protoFeeEvent := NewFeeEvent(meta, from, amtRaw128, xlmProtoAsset)
		feeEvents = append(feeEvents, protoFeeEvent)
	}

	if len(feeEvents) == 0 {
		return nil, fmt.Errorf("no fee events found for txHash: %v", txHash)
	} else if len(feeEvents) > 2 {
		return nil, fmt.Errorf("too many fee events found for txHash: %v, feeEvents found: %v", txHash, len(feeEvents))
	}
	return feeEvents, nil
}

// parseEvent is the main entry point for parsing contract events
// It attempts to parse events with a flexible, hierarchical approach
func (p *EventsProcessor) parseEvent(tx ingest.LedgerTransaction, opIndex *uint32, contractEvent xdr.ContractEvent) (*TokenTransferEvent, error) {
	// Validate basic contract contractEvent structure
	if contractEvent.Type != xdr.ContractEventTypeContract ||
		contractEvent.ContractId == nil ||
		contractEvent.Body.V != 0 {
		return nil, errNotSep41TokenFromMsg("invalid contractEvent")
	}

	topics := contractEvent.Body.V0.Topics

	// Require at least 2 topics for meaningful contractEvent parsing
	if len(topics) < 2 {
		return nil, errNotSep41TokenFromMsg("insufficient topics in contract event")
	}

	// Extract the contractEvent function name
	fn, ok := topics[0].GetSym()
	if !ok {
		return nil, errNotSep41TokenFromMsg("invalid function name")
	}

	// Determine if this is V3 or V4 based on transaction meta version
	txMetaVersion := tx.UnsafeMeta.V

	// First, try parsing as a standard SEP41 token contractEvent
	var protoEvent *TokenTransferEvent
	var sepErr error

	switch txMetaVersion {
	case 3:
		protoEvent, sepErr = parseCustomTokenEventV3(string(fn), tx, opIndex, contractEvent)
	case 4:
		protoEvent, sepErr = parseCustomTokenEventV4(string(fn), tx, opIndex, contractEvent)
	default:
		return nil, errNotSep41TokenFromMsg(fmt.Sprintf("unsupported transaction meta version: %d", txMetaVersion))
	}

	if sepErr != nil {
		return nil, sepErr
	}

	// This has passed validation for SEP-41 complaint token.
	// At the very least, you will now emit a contractEvent.
	// Attempt SAC validation if possible, to get asset name

	// SAC validation requires a very strict check on len(topics)
	// For V3: transfer/mint/clawback have 4 topics, burn has 3
	// For V4: transfer has 4 topics, mint/clawback/burn have 3
	expectedTopics := getExpectedTopicsCount(string(fn), txMetaVersion)
	if len(topics) == expectedTopics {
		lastTopic := topics[len(topics)-1]
		if assetStr, ok := lastTopic.GetStr(); ok && assetStr != "" {
			// Try parsing the asset from its SEP-11 representation
			assets, err := xdr.BuildAssets(string(assetStr))
			if err == nil && len(assets) == 1 {
				asset := assets[0]
				// Verify contract ID matches expected asset contract ID
				expectedId, idErr := asset.ContractID(p.networkPassphrase)
				if idErr == nil && expectedId == *contractEvent.ContractId {
					// If contract ID matches, update with validated asset
					protoEvent.SetAsset(asset)

					// For TxMetaV4, this is all that needs to be validated. You can simply return the event as is
					if txMetaVersion == 4 {
						return protoEvent, nil
					}

					// This is tricky. Burn and mint events currently show up as transfer in SAC events in V3
					// This will be fixed once CAP-67 unified events is released:
					// https://github.com/stellar/stellar-protocol/blob/master/core/cap-0067.md#protocol-upgrade-transition
					// Meanwhile, we fix it here manually by checking if src/dst is issuer of asset, and if it is, we issue mint/burn instead
					maybeTransferEvent := protoEvent.GetTransfer()
					if maybeTransferEvent != nil {
						protoEvent, err = p.mintOrBurnOrTransferEvent(
							tx, opIndex,
							maybeTransferEvent.Asset.ToXdrAsset(),
							maybeTransferEvent.From,
							maybeTransferEvent.To,
							maybeTransferEvent.Amount,
							// For smart contract transactions prior to P23, there will not be any muxed information emitted.
							// So set the includeMuxedInfo flag to false here. Better than having  to add unnecessary checks in the setDestinationMuxedInfo functions
							false,
						)
						if err != nil {
							return nil, fmt.Errorf("contract transfer event error: %w", err)
						}
					}
				}
			}
		}
	}

	return protoEvent, nil
}

// getExpectedTopicsCount returns expected number of topics for each event type based on tx meta version
func getExpectedTopicsCount(eventType string, txMetaVersion int32) int {
	switch txMetaVersion {
	case 3:
		// V3 format includes admin addresses
		switch eventType {
		case BurnEvent:
			// ["burn", from, asset]
			return 3
		default:
			// ["transfer", from, to, asset]
			// ["mint", admin, to, asset]
			// ["clawback", admin, from, asset]
			return 4
		}
	case 4:
		// V4 format removes admin addresses
		switch eventType {
		case TransferEvent:
			// ["transfer", from, to, asset]
			return 4
		default:
			// ["mint", to, asset] - no admin
			// ["clawback", from, asset] - no admin
			// ["burn", from, asset]
			return 3
		}
	}
	return -1 // Invalid combination
}

// parseCustomTokenEventV3 attempts to parse a generic SEP41 token event for V3 format
func parseCustomTokenEventV3(
	eventType string, tx ingest.LedgerTransaction, opIndex *uint32, contractEvent xdr.ContractEvent,
) (*TokenTransferEvent, error) {
	topics := contractEvent.Body.V0.Topics
	value := contractEvent.Body.V0.Data

	// Parse token amount. If that fails, then no need to bother checking for eventType
	amt, ok := value.GetI128()
	if !ok {
		return nil, errNotSep41TokenFromMsg("invalid event amount")
	}
	amtRaw128 := amount.String128Raw(amt)

	contractAddress := strkey.MustEncode(strkey.VersionByteContract, contractEvent.ContractId[:])
	meta := NewEventMetaFromTx(tx, opIndex, contractAddress)
	var event *TokenTransferEvent

	// Determine event type based on function name
	lenTopics := len(topics)
	switch eventType {
	case TransferEvent:
		// Transfer requires MINIMUM 3 topics: event type, fromAddr, toAddr
		if lenTopics < 3 {
			return nil, errNotSep41TokenFromMsg(fmt.Sprintf("transfer event requires minimum 3 topics, found: %v", lenTopics))
		}
		from, err := extractAddress(topics[1])
		if err != nil {
			return nil, errNotSep41TokenFromError(fmt.Errorf("invalid fromAddress. error: %w", err))
		}
		to, err := extractAddress(topics[2])
		if err != nil {
			return nil, errNotSep41TokenFromError(fmt.Errorf("invalid toAddress. error: %w", err))
		}
		event = NewTransferEvent(meta, from, to, amtRaw128, nil)

	case MintEvent:
		// Mint requires MINIMUM 3 topics - event type, admin, toAddr
		if lenTopics < 3 {
			return nil, errNotSep41TokenFromMsg(fmt.Sprintf("mint event requires minimum 3 topics, found: %v", lenTopics))
		}
		// Validate admin but don't use it
		_, err := extractAddress(topics[1])
		if err != nil {
			return nil, errNotSep41TokenFromError(fmt.Errorf("invalid adminAddress. error: %w", err))
		}
		to, err := extractAddress(topics[2])
		if err != nil {
			return nil, errNotSep41TokenFromError(fmt.Errorf("invalid toAddress error: %w", err))
		}
		event = NewMintEvent(meta, to, amtRaw128, nil)

	case ClawbackEvent:
		// Clawback requires MINIMUM 3 topics - event type, admin, fromAddr
		if lenTopics < 3 {
			return nil, errNotSep41TokenFromMsg(fmt.Sprintf("clawback event requires minimum 3 topics, found: %v", lenTopics))
		}
		// Validate admin but don't use it
		_, err := extractAddress(topics[1])
		if err != nil {
			return nil, errNotSep41TokenFromError(fmt.Errorf("invalid adminAddress. error: %w", err))
		}
		from, err := extractAddress(topics[2])
		if err != nil {
			return nil, errNotSep41TokenFromError(fmt.Errorf("invalid fromAddress error: %w", err))
		}
		event = NewClawbackEvent(meta, from, amtRaw128, nil)

	case BurnEvent:
		// Burn requires MINIMUM 2 topics - event type, fromAddr
		if lenTopics < 2 {
			return nil, errNotSep41TokenFromMsg(fmt.Sprintf("burn event requires minimum 2 topics, found: %v", lenTopics))
		}
		from, err := extractAddress(topics[1])
		if err != nil {
			return nil, errNotSep41TokenFromError(fmt.Errorf("invalid fromAddress error: %w", err))
		}
		event = NewBurnEvent(meta, from, amtRaw128, nil)

	default:
		return nil, errNotSep41TokenFromMsg(fmt.Sprintf("unsupported custom token event type: %v", eventType))
	}

	return event, nil
}

// parseCustomTokenEventV4 attempts to parse a generic SEP41 token event for V4 format
func parseCustomTokenEventV4(
	eventType string, tx ingest.LedgerTransaction, opIndex *uint32, contractEvent xdr.ContractEvent,
) (*TokenTransferEvent, error) {
	topics := contractEvent.Body.V0.Topics
	value := contractEvent.Body.V0.Data

	// Parse amount and optional to_muxed_id from data
	var amt xdr.Int128Parts
	var destinationMemo *MuxedInfo

	// V4 data format can be:
	// 1. Direct i128 (when there's no memo)
	// 2. ScMap with exactly 2 fields: "amount" (i128) + "to_muxed_id" (u64/bytes/string)
	// If it's a map, at the very least "amount" should be present.
	if mapData, ok := value.GetMap(); ok {
		if mapData == nil {
			return nil, errNotSep41TokenFromMsg("map is empty")
		}
		var err error
		amt, destinationMemo, err = parseV4MapDataForTokenEvents(*mapData)
		if err != nil {
			return nil, errNotSep41TokenFromError(fmt.Errorf("failed to parse V4 map data: %w", err))
		}
	} else {
		// Fall back to direct i128 parsing (V4 without to_muxed_id)
		var ok bool
		amt, ok = value.GetI128()
		if !ok {
			return nil, errNotSep41TokenFromMsg("invalid event amount")
		}
	}

	amtRaw128 := amount.String128Raw(amt)
	contractAddress := strkey.MustEncode(strkey.VersionByteContract, contractEvent.ContractId[:])
	meta := NewEventMetaFromTx(tx, opIndex, contractAddress)

	// Set destination memo if present
	if destinationMemo != nil {
		meta.ToMuxedInfo = destinationMemo
	}

	var event *TokenTransferEvent
	lenTopics := len(topics)

	switch eventType {
	case TransferEvent:
		// Transfer requires MINIMUM 3 topics: event type, fromAddr, toAddr (same as V3)
		if lenTopics < 3 {
			return nil, errNotSep41TokenFromMsg(fmt.Sprintf("transfer event requires minimum 3 topics, found: %v", lenTopics))
		}
		from, err := extractAddress(topics[1])
		if err != nil {
			return nil, errNotSep41TokenFromError(fmt.Errorf("invalid fromAddress. error: %w", err))
		}
		to, err := extractAddress(topics[2])
		if err != nil {
			return nil, errNotSep41TokenFromError(fmt.Errorf("invalid toAddress. error: %w", err))
		}
		event = NewTransferEvent(meta, from, to, amtRaw128, nil)

	case MintEvent:
		// Mint requires MINIMUM 2 topics - event type, toAddr (NO admin in V4)
		if lenTopics < 2 {
			return nil, errNotSep41TokenFromMsg(fmt.Sprintf("mint event requires minimum 2 topics, found: %v", lenTopics))
		}
		to, err := extractAddress(topics[1])
		if err != nil {
			return nil, errNotSep41TokenFromError(fmt.Errorf("invalid toAddress error: %w", err))
		}
		event = NewMintEvent(meta, to, amtRaw128, nil)

	case ClawbackEvent:
		// Clawback requires MINIMUM 2 topics - event type, fromAddr (NO admin in V4)
		if lenTopics < 2 {
			return nil, errNotSep41TokenFromMsg(fmt.Sprintf("clawback event requires minimum 2 topics, found: %v", lenTopics))
		}
		from, err := extractAddress(topics[1])
		if err != nil {
			return nil, errNotSep41TokenFromError(fmt.Errorf("invalid fromAddress error: %w", err))
		}
		event = NewClawbackEvent(meta, from, amtRaw128, nil)

	case BurnEvent:
		// Burn requires MINIMUM 2 topics - event type, fromAddr (same as V3)
		if lenTopics < 2 {
			return nil, errNotSep41TokenFromMsg(fmt.Sprintf("burn event requires minimum 2 topics, found: %v", lenTopics))
		}
		from, err := extractAddress(topics[1])
		if err != nil {
			return nil, errNotSep41TokenFromError(fmt.Errorf("invalid fromAddress error: %w", err))
		}
		event = NewBurnEvent(meta, from, amtRaw128, nil)

	default:
		return nil, errNotSep41TokenFromMsg(fmt.Sprintf("unsupported custom token event type: %v", eventType))
	}

	return event, nil
}

// parseV4MapDataForTokenEvents parses the ScMap data format used in V4 token events
func parseV4MapDataForTokenEvents(mapData xdr.ScMap) (xdr.Int128Parts, *MuxedInfo, error) {
	var foundAmount bool
	var amt xdr.Int128Parts
	var muxedInfo *MuxedInfo

	for _, entry := range mapData {
		key, ok := entry.Key.GetSym()
		if !ok {
			return amt, nil, fmt.Errorf("invalid key type in data map: %s", entry.Key.Type)
		}

		switch string(key) {
		case "amount":
			amt, ok = entry.Val.GetI128()
			if !ok {
				return amt, nil, fmt.Errorf("amt field is not i128")
			}
			foundAmount = true

		case "to_muxed_id":
			// Convert to_muxed_id to MuxedInfo based on type
			switch entry.Val.Type {
			case xdr.ScValTypeScvU64:
				if val, ok := entry.Val.GetU64(); ok {
					muxedInfo = NewMuxedInfoFromId(uint64(val))
				}
			case xdr.ScValTypeScvBytes:
				if val, ok := entry.Val.GetBytes(); ok {
					hashBytes := make([]byte, 32)
					copy(hashBytes, val)
					muxedInfo = &MuxedInfo{
						Content: &MuxedInfo_Hash{
							Hash: hashBytes,
						},
					}
				}
			case xdr.ScValTypeScvString:
				if val, ok := entry.Val.GetStr(); ok {
					muxedInfo = &MuxedInfo{
						Content: &MuxedInfo_Text{
							Text: string(val),
						},
					}
				}
			default:
				return amt, nil, fmt.Errorf("invalid to_muxed_id type for data: %s", entry.Val.Type)
			}
		}
	}

	if !foundAmount {
		return amt, nil, fmt.Errorf("amount field not found in map")
	}

	return amt, muxedInfo, nil
}
