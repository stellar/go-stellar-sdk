package token_transfer

import (
	"fmt"
	"testing"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/strkey"
	"google.golang.org/protobuf/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/amount"
	"github.com/stellar/go-stellar-sdk/xdr"
)

var (
	randomAccount   = keypair.MustRandom().Address()
	someContractId1 = xdr.ContractId([32]byte{1, 2, 3, 4}) // Needed for V3 tests, since they use xdr.ContractId
	someContract1   = strkey.MustEncode(strkey.VersionByteContract, someContractId1[:])
	someContractId2 = xdr.ContractId([32]byte{4, 3, 2, 1}) // Needed for V3 tests, since they use xdr.ContractId

	someContract2 = strkey.MustEncode(strkey.VersionByteContract, someContractId2[:])

	processor = &EventsProcessor{
		networkPassphrase: someNetworkPassphrase,
	}
	contractIdFromAsset = func(asset xdr.Asset) *xdr.ContractId {
		contractId, _ := asset.ContractID(someNetworkPassphrase)
		hash := xdr.ContractId(contractId)
		return &hash
	}

	thousand    = int64(1000)
	thousandStr = "1000"
)

// Helper function to create a ScVal symbol
func createSymbol(sym string) xdr.ScVal {
	symStr := xdr.ScSymbol(sym)
	return xdr.ScVal{
		Type: xdr.ScValTypeScvSymbol,
		Sym:  &symStr,
	}
}

func createAddress(address string) xdr.ScVal {
	scAddress := xdr.ScAddress{}

	switch {
	case strkey.IsValidContractAddress(address) == true:
		scAddress.Type = xdr.ScAddressTypeScAddressTypeContract
		contractHash := strkey.MustDecode(strkey.VersionByteContract, address)
		contractId := xdr.ContractId(contractHash)
		scAddress.ContractId = &contractId

	case strkey.IsValidEd25519PublicKey(address) == true:
		scAddress.Type = xdr.ScAddressTypeScAddressTypeAccount
		scAddress.AccountId = xdr.MustAddressPtr(address)

	default:
		panic(fmt.Errorf("unsupported address: %s", address))
	}

	return xdr.ScVal{
		Type:    xdr.ScValTypeScvAddress,
		Address: &scAddress,
	}
}

// Helper function to create a ScVal string
func createString(str string) xdr.ScVal {
	symStr := xdr.ScString(str)
	return xdr.ScVal{
		Type: xdr.ScValTypeScvString,
		Str:  &symStr,
	}
}

// Helper function to create an Int128 ScVal
func createInt128(val int64) xdr.ScVal {
	var parts xdr.Int128Parts

	if val >= 0 {
		parts = xdr.Int128Parts{
			Lo: xdr.Uint64(val),
			Hi: 0,
		}
	} else {
		// For negative numbers, you need to do two's complement
		parts = xdr.Int128Parts{
			Lo: xdr.Uint64(val),
			Hi: xdr.Int64(-1),
		}
	}

	return xdr.ScVal{
		Type: xdr.ScValTypeScvI128,
		I128: &parts,
	}
}

// createScMap creates an xdr.ScVal of type ScvMap from key-value pairs
// Usage: createScMap("key1", value1, "key2", value2, ...)
func createScMap(keyValuePairs ...interface{}) xdr.ScVal {
	mapEntries := xdr.ScMap{}

	for i := 0; i < len(keyValuePairs); i += 2 {
		key := keyValuePairs[i]
		value := keyValuePairs[i+1]

		// Convert key to ScVal (assuming string keys, but you can extend this)
		var keyScVal xdr.ScVal
		switch k := key.(type) {
		case string:
			keyScVal = createSymbol(k)
		case xdr.ScVal:
			keyScVal = k
		default:
			panic(fmt.Sprintf("unsupported key type: %T", key))
		}

		// Convert value to ScVal
		var valueScVal xdr.ScVal
		switch v := value.(type) {
		case xdr.ScVal:
			valueScVal = v
		case string:
			valueScVal = createString(v)
		case int:
			valueScVal = createInt128(int64(v))
		case int64:
			valueScVal = createInt128(v)
		case uint64:
			val := xdr.Uint64(v)
			valueScVal = xdr.ScVal{
				Type: xdr.ScValTypeScvU64,
				U64:  &val,
			}
		default:
			panic(fmt.Sprintf("unsupported value type: %T", value))
		}

		entry := xdr.ScMapEntry{
			Key: keyScVal,
			Val: valueScVal,
		}
		mapEntries = append(mapEntries, entry)
	}

	mapPtr := &mapEntries
	return xdr.ScVal{
		Type: xdr.ScValTypeScvMap,
		Map:  &mapPtr,
	}
}

func createContractEventFromTopicsAndData(contractId *xdr.ContractId, topics []xdr.ScVal, data xdr.ScVal) xdr.ContractEvent {
	return xdr.ContractEvent{
		Type:       xdr.ContractEventTypeContract,
		ContractId: contractId,
		Body: xdr.ContractEventBody{
			V: 0,
			V0: &xdr.ContractEventV0{
				Topics: topics,
				Data:   data,
			},
		},
	}
}

// Helper function to create a mock ContractEvent
func createContractEvent(
	eventType string,
	from, to string,
	amount int64,
	amount128 *xdr.Int128Parts,
	assetStr string,
	contractId *xdr.ContractId,
) xdr.ContractEvent {
	topics := []xdr.ScVal{
		createSymbol(eventType),
	}

	if from != "" {
		topics = append(topics, createAddress(from))
	}

	if to != "" {
		topics = append(topics, createAddress(to))
	}

	if assetStr != "" {
		topics = append(topics, createString(assetStr))
	}

	var data xdr.ScVal
	if amount128 != nil {
		data = xdr.ScVal{
			Type: xdr.ScValTypeScvI128,
			I128: amount128,
		}

	} else {
		data = createInt128(amount)
	}

	return xdr.ContractEvent{
		Type:       xdr.ContractEventTypeContract,
		ContractId: contractId,
		Body: xdr.ContractEventBody{
			V: 0,
			V0: &xdr.ContractEventV0{
				Topics: topics,
				Data:   data,
			},
		},
	}
}

func TestValidContractEventsV3(t *testing.T) {
	testCases := []struct {
		name          string
		eventType     string
		addr1         string // meaning depends on event type (from/admin)
		addr2         string // meaning depends on event type (to/from/empty)
		amount        int64
		amount128     *xdr.Int128Parts
		isSacEvent    bool
		validateEvent func(t *testing.T, event *TokenTransferEvent, addr1, addr2 string, amount string, assetItem interface{})
	}{
		{
			name:       "Transfer SEP-41 Token Event",
			eventType:  TransferEvent,
			addr1:      someContract1, // from
			addr2:      someContract2, // to
			amount:     1000,
			isSacEvent: false,
			validateEvent: func(t *testing.T, event *TokenTransferEvent, from, to string, amount string, _ interface{}) {
				assert.NotNil(t, event.GetTransfer())
				assert.Equal(t, from, event.GetTransfer().From)
				assert.Equal(t, to, event.GetTransfer().To)
				assert.Equal(t, amount, event.GetTransfer().Amount)
				assert.Nil(t, event.GetAsset()) // asset is nil for non-SAC events
			},
		},
		{
			name:       "Transfer SAC Event",
			eventType:  TransferEvent,
			addr1:      randomAccount, // from
			addr2:      someContract1, // to
			amount:     1000,
			isSacEvent: true,
			validateEvent: func(t *testing.T, event *TokenTransferEvent, from, to string, amount string, assetItem interface{}) {
				assert.NotNil(t, event.GetTransfer())
				assert.Equal(t, from, event.GetTransfer().From)
				assert.Equal(t, to, event.GetTransfer().To)
				assert.Equal(t, amount, event.GetTransfer().Amount)
				assert.NotNil(t, event.GetAsset())

				asset := assetItem.(xdr.Asset)
				assert.True(t, event.GetAsset().ToXdrAsset().Equals(asset))
			},
		},
		{
			name:       "Mint SEP-41 Token Event",
			eventType:  MintEvent,
			addr1:      someContract2, // admin
			addr2:      someContract1, // to
			amount:     500,
			isSacEvent: false,
			validateEvent: func(t *testing.T, event *TokenTransferEvent, admin, to string, amount string, _ interface{}) {
				assert.NotNil(t, event.GetMint())
				assert.Equal(t, to, event.GetMint().To)
				assert.Equal(t, amount, event.GetMint().Amount)
				assert.Nil(t, event.GetAsset()) // asset is nil for non-SAC events
			},
		},
		{
			name:       "Mint SAC Event",
			eventType:  MintEvent,
			addr1:      randomAccount, // admin
			addr2:      someContract1, // to
			amount:     500,
			isSacEvent: true,
			validateEvent: func(t *testing.T, event *TokenTransferEvent, admin, to string, amount string, assetItem interface{}) {
				assert.NotNil(t, event.GetMint())
				assert.Equal(t, to, event.GetMint().To)
				assert.Equal(t, amount, event.GetMint().Amount)
				assert.NotNil(t, event.GetAsset())

				asset := assetItem.(xdr.Asset)
				assert.True(t, event.GetAsset().ToXdrAsset().Equals(asset))
			},
		},
		{
			name:       "Burn SEP-41 Token Event",
			eventType:  BurnEvent,
			addr1:      randomAccount, // from
			addr2:      "",            // no second address for burn
			amount:     300,
			isSacEvent: false,
			validateEvent: func(t *testing.T, event *TokenTransferEvent, from, _ string, amount string, _ interface{}) {
				assert.NotNil(t, event.GetBurn())
				assert.Equal(t, from, event.GetBurn().From)
				assert.Equal(t, amount, event.GetBurn().Amount)
				assert.Nil(t, event.GetAsset()) // asset is nil for non-SAC events
			},
		},
		{
			name:       "Burn SAC Event",
			eventType:  BurnEvent,
			addr1:      randomAccount, // from
			addr2:      "",            // no second address for burn
			amount:     300,
			isSacEvent: true,
			validateEvent: func(t *testing.T, event *TokenTransferEvent, from, _ string, amount string, assetItem interface{}) {
				assert.NotNil(t, event.GetBurn())
				assert.Equal(t, from, event.GetBurn().From)
				assert.Equal(t, amount, event.GetBurn().Amount)
				assert.NotNil(t, event.GetAsset())

				asset := assetItem.(xdr.Asset)
				assert.True(t, event.GetAsset().ToXdrAsset().Equals(asset))
			},
		},
		{
			name:       "Clawback SEP-41 Token Event",
			eventType:  ClawbackEvent,
			addr1:      someContract1, // admin
			addr2:      someContract2, // from (user whose tokens are clawed back)
			amount:     200,
			isSacEvent: false,
			validateEvent: func(t *testing.T, event *TokenTransferEvent, admin, from string, amount string, _ interface{}) {
				assert.NotNil(t, event.GetClawback())
				assert.Equal(t, from, event.GetClawback().From)
				assert.Equal(t, amount, event.GetClawback().Amount)
				assert.Nil(t, event.GetAsset()) // asset is nil for non-SAC events
			},
		},
		{
			name:       "Clawback SAC Event",
			eventType:  ClawbackEvent,
			addr1:      someContract1, // admin
			addr2:      randomAccount, // from (user whose tokens are clawed back)
			amount:     200,
			isSacEvent: true,
			validateEvent: func(t *testing.T, event *TokenTransferEvent, admin, from string, amount string, assetItem interface{}) {
				assert.NotNil(t, event.GetClawback())
				assert.Equal(t, from, event.GetClawback().From)
				assert.Equal(t, amount, event.GetClawback().Amount)
				assert.NotNil(t, event.GetAsset())

				asset := assetItem.(xdr.Asset)
				assert.True(t, event.GetAsset().ToXdrAsset().Equals(asset))
			},
		},
		{
			name:       "Transfer SEP-41 Token Event - Amount is 128bits",
			eventType:  TransferEvent,
			addr1:      someContract1, // from
			addr2:      someContract2, // to
			amount128:  &xdr.Int128Parts{Hi: 5000, Lo: 1000},
			isSacEvent: false,
			validateEvent: func(t *testing.T, event *TokenTransferEvent, from, to string, amount string, _ interface{}) {
				assert.NotNil(t, event.GetTransfer())
				assert.Equal(t, from, event.GetTransfer().From)
				assert.Equal(t, to, event.GetTransfer().To)
				assert.Equal(t, amount, event.GetTransfer().Amount)
				assert.Nil(t, event.GetAsset()) // asset is nil for non-SAC events
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test with multiple assets
			var assets []interface{}
			var contractId *xdr.ContractId

			if tc.isSacEvent {
				// Test with SAC assets
				for _, asset := range []xdr.Asset{xlmAsset, usdcAsset, ethAsset} {
					assets = append(assets, asset)
				}
			} else {
				// Test with non-SAC assets
				for _, asset := range []string{"someNonSep11AssetString", xlmAsset.StringCanonical(), usdcAsset.StringCanonical()} {
					assets = append(assets, asset)
				}
				contractId = &someContractId1
			}

			for _, assetItem := range assets {
				var assetStr string

				if tc.isSacEvent {
					asset := assetItem.(xdr.Asset)
					assetStr = asset.StringCanonical()
					contractId = contractIdFromAsset(asset)
				} else {
					assetStr = assetItem.(string)
				}

				// Create the contract event based on event type
				var contractEvent xdr.ContractEvent

				switch tc.eventType {
				case TransferEvent:
					// from, to
					contractEvent = createContractEvent(
						tc.eventType,
						tc.addr1, // from
						tc.addr2, // to
						tc.amount,
						tc.amount128,
						assetStr,
						contractId,
					)
				case MintEvent:
					// admin, to
					contractEvent = createContractEvent(
						tc.eventType,
						tc.addr1, // admin
						tc.addr2, // to
						tc.amount,
						tc.amount128,
						assetStr,
						contractId,
					)
				case BurnEvent:
					// from, ""
					contractEvent = createContractEvent(
						tc.eventType,
						tc.addr1, // from
						"",       // toAddress is empty
						tc.amount,
						tc.amount128,
						assetStr,
						contractId,
					)
				case ClawbackEvent:
					// admin, from
					contractEvent = createContractEvent(
						tc.eventType,
						tc.addr1, // admin
						tc.addr2, // address from which asset needs to be clawed back
						tc.amount,
						tc.amount128,
						assetStr,
						contractId,
					)
				}

				event, err := processor.parseEvent(someTxV3(), &someOperationIndex, contractEvent)

				require.NoError(t, err)
				require.NotNil(t, event)

				var amountStr string
				if tc.amount128 != nil {
					amountStr = amount.String128Raw(*tc.amount128)
				} else {
					amountStr = fmt.Sprintf("%d", tc.amount)
				}
				tc.validateEvent(t, event, tc.addr1, tc.addr2, amountStr, assetItem)
			}
		})
	}
}

func TestInvalidEventsV3(t *testing.T) {
	testCases := []struct {
		name           string
		setupEvent     func() xdr.ContractEvent
		expectedErrMsg string
	}{
		{
			name: "Invalid contract event type",
			setupEvent: func() xdr.ContractEvent {
				// Use a non-contract event type
				event := createContractEvent(TransferEvent, randomAccount, someContract1, 1000, nil, "asset", &someContractId1)
				event.Type = xdr.ContractEventTypeSystem // Invalid type
				return event
			},
			expectedErrMsg: "invalid contractEvent",
		},
		{
			name: "Missing contract ID",
			setupEvent: func() xdr.ContractEvent {
				event := createContractEvent(TransferEvent, randomAccount, someContract1, 1000, nil, "asset", nil)
				return event
			},
			expectedErrMsg: "invalid contractEvent",
		},
		{
			name: "Invalid body version",
			setupEvent: func() xdr.ContractEvent {
				event := createContractEvent(TransferEvent, randomAccount, someContract1, 1000, nil, "asset", &someContractId1)
				event.Body.V = 1 // Invalid version
				return event
			},
			expectedErrMsg: "invalid contractEvent",
		},
		{
			name: "Insufficient topics",
			setupEvent: func() xdr.ContractEvent {
				// Create event with only one topic (the function name)
				topics := []xdr.ScVal{
					createSymbol(TransferEvent),
				}

				return xdr.ContractEvent{
					Type:       xdr.ContractEventTypeContract,
					ContractId: &someContractId1,
					Body: xdr.ContractEventBody{
						V: 0,
						V0: &xdr.ContractEventV0{
							Topics: topics,
							Data:   createInt128(1000),
						},
					},
				}
			},
			expectedErrMsg: "insufficient topics in contract event",
		},
		{
			name: "Invalid function name type",
			setupEvent: func() xdr.ContractEvent {
				// Use a string instead of a symbol for function name
				topics := []xdr.ScVal{
					createString(TransferEvent), // Should be a symbol
					createAddress(randomAccount),
					createAddress(someContract1),
				}

				return xdr.ContractEvent{
					Type:       xdr.ContractEventTypeContract,
					ContractId: &someContractId1,
					Body: xdr.ContractEventBody{
						V: 0,
						V0: &xdr.ContractEventV0{
							Topics: topics,
							Data:   createInt128(1000),
						},
					},
				}
			},
			expectedErrMsg: "invalid function name",
		},
		{
			name: "Invalid amount format",
			setupEvent: func() xdr.ContractEvent {
				event := createContractEvent(TransferEvent, randomAccount, someContract1, 1000, nil, "asset", &someContractId1)
				// Replace the amount with a string value instead of Int128
				event.Body.V0.Data = createString("1000")
				return event
			},
			expectedErrMsg: "invalid event amount",
		},
		{
			name: "Transfer: Too few topics",
			setupEvent: func() xdr.ContractEvent {
				// Only include function name and from address, missing to address
				topics := []xdr.ScVal{
					createSymbol(TransferEvent),
					createAddress(randomAccount),
					// Missing to address
				}

				return xdr.ContractEvent{
					Type:       xdr.ContractEventTypeContract,
					ContractId: &someContractId1,
					Body: xdr.ContractEventBody{
						V: 0,
						V0: &xdr.ContractEventV0{
							Topics: topics,
							Data:   createInt128(1000),
						},
					},
				}
			},
			expectedErrMsg: "transfer event requires minimum 3 topics, found: 2",
		},
		{
			name: "Transfer: Invalid from address",
			setupEvent: func() xdr.ContractEvent {
				// Use an invalid value for from address (not an address)
				topics := []xdr.ScVal{
					createSymbol(TransferEvent),
					createString("not an address"), // Invalid from address
					createAddress(someContract1),
				}

				return xdr.ContractEvent{
					Type:       xdr.ContractEventTypeContract,
					ContractId: &someContractId1,
					Body: xdr.ContractEventBody{
						V: 0,
						V0: &xdr.ContractEventV0{
							Topics: topics,
							Data:   createInt128(1000),
						},
					},
				}
			},
			expectedErrMsg: "invalid fromAddress",
		},
		{
			name: "Transfer: Invalid to address",
			setupEvent: func() xdr.ContractEvent {
				// Use an invalid value for to address (not an address)
				topics := []xdr.ScVal{
					createSymbol(TransferEvent),
					createAddress(randomAccount),
					createString("not an address"), // Invalid to address
				}

				return xdr.ContractEvent{
					Type:       xdr.ContractEventTypeContract,
					ContractId: &someContractId1,
					Body: xdr.ContractEventBody{
						V: 0,
						V0: &xdr.ContractEventV0{
							Topics: topics,
							Data:   createInt128(1000),
						},
					},
				}
			},
			expectedErrMsg: "invalid toAddress",
		},
		{
			name: "Mint: Too few topics",
			setupEvent: func() xdr.ContractEvent {
				// Only include function name and admin, missing to address
				topics := []xdr.ScVal{
					createSymbol(MintEvent),
					createAddress(randomAccount),
					// Missing to address
				}

				return xdr.ContractEvent{
					Type:       xdr.ContractEventTypeContract,
					ContractId: &someContractId1,
					Body: xdr.ContractEventBody{
						V: 0,
						V0: &xdr.ContractEventV0{
							Topics: topics,
							Data:   createInt128(1000),
						},
					},
				}
			},
			expectedErrMsg: "mint event requires minimum 3 topics",
		},
		{
			name: "Mint: Invalid admin address",
			setupEvent: func() xdr.ContractEvent {
				// Use an invalid value for admin address (not an address)
				topics := []xdr.ScVal{
					createSymbol(MintEvent),
					createString("not an address"), // Invalid admin address
					createAddress(someContract1),
				}

				return xdr.ContractEvent{
					Type:       xdr.ContractEventTypeContract,
					ContractId: &someContractId1,
					Body: xdr.ContractEventBody{
						V: 0,
						V0: &xdr.ContractEventV0{
							Topics: topics,
							Data:   createInt128(1000),
						},
					},
				}
			},
			expectedErrMsg: "invalid adminAddress",
		},
		{
			name: "Mint: Invalid to address",
			setupEvent: func() xdr.ContractEvent {
				// Use an invalid value for to address (not an address)
				topics := []xdr.ScVal{
					createSymbol(MintEvent),
					createAddress(randomAccount),
					createString("not an address"), // Invalid to address
				}

				return xdr.ContractEvent{
					Type:       xdr.ContractEventTypeContract,
					ContractId: &someContractId1,
					Body: xdr.ContractEventBody{
						V: 0,
						V0: &xdr.ContractEventV0{
							Topics: topics,
							Data:   createInt128(1000),
						},
					},
				}
			},
			expectedErrMsg: "invalid toAddress",
		},
		{
			name: "Clawback: Too few topics",
			setupEvent: func() xdr.ContractEvent {
				// Only include function name and admin, missing from address
				topics := []xdr.ScVal{
					createSymbol(ClawbackEvent),
					createAddress(randomAccount),
					// Missing from address
				}

				return xdr.ContractEvent{
					Type:       xdr.ContractEventTypeContract,
					ContractId: &someContractId1,
					Body: xdr.ContractEventBody{
						V: 0,
						V0: &xdr.ContractEventV0{
							Topics: topics,
							Data:   createInt128(1000),
						},
					},
				}
			},
			expectedErrMsg: "clawback event requires minimum 3 topics",
		},
		{
			name: "Clawback: Invalid admin address",
			setupEvent: func() xdr.ContractEvent {
				// Use an invalid value for admin address (not an address)
				topics := []xdr.ScVal{
					createSymbol(ClawbackEvent),
					createString("not an address"), // Invalid admin address
					createAddress(randomAccount),
				}

				return xdr.ContractEvent{
					Type:       xdr.ContractEventTypeContract,
					ContractId: &someContractId1,
					Body: xdr.ContractEventBody{
						V: 0,
						V0: &xdr.ContractEventV0{
							Topics: topics,
							Data:   createInt128(1000),
						},
					},
				}
			},
			expectedErrMsg: "invalid adminAddress",
		},
		{
			name: "Clawback: Invalid from address",
			setupEvent: func() xdr.ContractEvent {
				// Use an invalid value for from address (not an address)
				topics := []xdr.ScVal{
					createSymbol(ClawbackEvent),
					createAddress(someContract1),
					createString("not an address"), // Invalid from address
				}

				return xdr.ContractEvent{
					Type:       xdr.ContractEventTypeContract,
					ContractId: &someContractId1,
					Body: xdr.ContractEventBody{
						V: 0,
						V0: &xdr.ContractEventV0{
							Topics: topics,
							Data:   createInt128(1000),
						},
					},
				}
			},
			expectedErrMsg: "invalid fromAddress",
		},
		{
			name: "Burn: Too few topics",
			// this is true for any event really. Minimum number of topics is 2.
			setupEvent: func() xdr.ContractEvent {
				// Only include function name, missing from address
				topics := []xdr.ScVal{
					createSymbol(BurnEvent),
					// Missing from address
				}

				return xdr.ContractEvent{
					Type:       xdr.ContractEventTypeContract,
					ContractId: &someContractId1,
					Body: xdr.ContractEventBody{
						V: 0,
						V0: &xdr.ContractEventV0{
							Topics: topics,
							Data:   createInt128(1000),
						},
					},
				}
			},
			expectedErrMsg: "insufficient topics in contract event",
		},
		{
			name: "Burn: Invalid from address",
			setupEvent: func() xdr.ContractEvent {
				// Use an invalid value for from address (not an address)
				topics := []xdr.ScVal{
					createSymbol(BurnEvent),
					createString("not an address"), // Invalid from address
				}

				return xdr.ContractEvent{
					Type:       xdr.ContractEventTypeContract,
					ContractId: &someContractId1,
					Body: xdr.ContractEventBody{
						V: 0,
						V0: &xdr.ContractEventV0{
							Topics: topics,
							Data:   createInt128(1000),
						},
					},
				}
			},
			expectedErrMsg: "invalid fromAddress",
		},
		{
			name: "Unsupported event type",
			setupEvent: func() xdr.ContractEvent {
				return createContractEvent(
					"unknown_event", // Unsupported event type
					randomAccount,
					someContract1,
					1000,
					nil,
					"asset",
					&someContractId1,
				)
			},
			expectedErrMsg: "unsupported custom token event type",
		},
		{
			name: "Invalid SAC asset string",
			setupEvent: func() xdr.ContractEvent {
				// Create transfer event with invalid asset string that can't be parsed
				topics := []xdr.ScVal{
					createSymbol(TransferEvent),
					createAddress(randomAccount),
					createAddress(someContract1),
					createString("not:a:valid:asset"), // Invalid asset string
				}

				return xdr.ContractEvent{
					Type:       xdr.ContractEventTypeContract,
					ContractId: &someContractId1,
					Body: xdr.ContractEventBody{
						V: 0,
						V0: &xdr.ContractEventV0{
							Topics: topics,
							Data:   createInt128(1000),
						},
					},
				}
			},
			expectedErrMsg: "", // This should not error as SAC validation is only for enhancement
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			contractEvent := tc.setupEvent()
			event, err := processor.parseEvent(someTxV3(), &someOperationIndex, contractEvent)

			if tc.expectedErrMsg == "" {
				// If no error is expected, the test should pass
				require.NoError(t, err)
				require.NotNil(t, event)
			} else {
				// If an error is expected
				require.Error(t, err)
				assert.Nil(t, event)
				assert.Contains(t, err.Error(), tc.expectedErrMsg, "Error message should contain expected text")

				// Verify it's the right error type
				_, ok := err.(ErrNotSep41TokenEvent)
				assert.True(t, ok, "Error should be of type ErrNotSep41TokenEvent")
			}
		})
	}
}

func TestSacAssetValidation(t *testing.T) {
	// None of the test fixtures here will result in errors
	// They simply might not pass the additional validation required to qualify as a SAC event, is all
	xlmContractId := contractIdFromAsset(xlmAsset)

	testCases := []struct {
		name              string
		setupEvent        func() xdr.ContractEvent
		isAssetSetInEvent bool // Whether to check if the asset is set in the event
	}{
		{
			name: "Valid SAC asset with matching contract ID",
			setupEvent: func() xdr.ContractEvent {
				return createContractEvent(
					TransferEvent,
					randomAccount,
					someContract1,
					1000,
					nil,
					xlmAsset.StringCanonical(),
					xlmContractId,
				)
			},
			isAssetSetInEvent: true,
		},
		{
			name: "Valid SAC asset string but mismatched contract ID",
			setupEvent: func() xdr.ContractEvent {
				// Use valid asset string but wrong contract ID
				return createContractEvent(
					TransferEvent,
					randomAccount,
					someContract1,
					1000,
					nil,
					xlmAsset.StringCanonical(),
					&someContractId2, // Different from xlmContractId
				)
			},
			isAssetSetInEvent: false, // Asset should not be set due to mismatch
		},
		{
			name: "Valid SAC asset with correct format but not exactly 4 topics",
			setupEvent: func() xdr.ContractEvent {
				// Add an extra topic to make it 5 instead of 4
				topics := []xdr.ScVal{
					createSymbol(TransferEvent),
					createAddress(randomAccount),
					createAddress(someContract1),
					createString(xlmAsset.StringCanonical()),
					createString("extra topic"),
				}

				return xdr.ContractEvent{
					Type:       xdr.ContractEventTypeContract,
					ContractId: xlmContractId,
					Body: xdr.ContractEventBody{
						V: 0,
						V0: &xdr.ContractEventV0{
							Topics: topics,
							Data:   createInt128(1000),
						},
					},
				}
			},
			isAssetSetInEvent: false, // Should not set asset due to wrong topic count
		},
		{
			name: "Valid burn event with exactly 3 topics and valid asset",
			setupEvent: func() xdr.ContractEvent {
				topics := []xdr.ScVal{
					createSymbol(BurnEvent),
					createAddress(randomAccount),
					createString(xlmAsset.StringCanonical()),
				}

				return xdr.ContractEvent{
					Type:       xdr.ContractEventTypeContract,
					ContractId: xlmContractId,
					Body: xdr.ContractEventBody{
						V: 0,
						V0: &xdr.ContractEventV0{
							Topics: topics,
							Data:   createInt128(1000),
						},
					},
				}
			},
			isAssetSetInEvent: true, // Should set asset for burn with 3 topics
		},
		{
			name: "Valid token event but last topic is not a string",
			setupEvent: func() xdr.ContractEvent {
				topics := []xdr.ScVal{
					createSymbol(TransferEvent),
					createAddress(randomAccount),
					createAddress(someContract1),
					createInt128(12345), // Not a string
				}

				return xdr.ContractEvent{
					Type:       xdr.ContractEventTypeContract,
					ContractId: xlmContractId,
					Body: xdr.ContractEventBody{
						V: 0,
						V0: &xdr.ContractEventV0{
							Topics: topics,
							Data:   createInt128(1000),
						},
					},
				}
			},
			isAssetSetInEvent: false, // Should not set asset due to non-string
		},
		{
			name: "Valid token event but asset string is empty",
			setupEvent: func() xdr.ContractEvent {
				topics := []xdr.ScVal{
					createSymbol(TransferEvent),
					createAddress(randomAccount),
					createAddress(someContract1),
					createString(""), // Empty string
				}

				return xdr.ContractEvent{
					Type:       xdr.ContractEventTypeContract,
					ContractId: xlmContractId,
					Body: xdr.ContractEventBody{
						V: 0,
						V0: &xdr.ContractEventV0{
							Topics: topics,
							Data:   createInt128(1000),
						},
					},
				}
			},
			isAssetSetInEvent: false, // Should not set asset due to empty string
		},
		{
			name: "Valid token event but BuildAssets returns multiple assets",
			setupEvent: func() xdr.ContractEvent {
				// Make it so that some custom token emits a string that is a SEP-11 representation of multiple assets
				topics := []xdr.ScVal{
					createSymbol(TransferEvent),
					createAddress(randomAccount),
					createAddress(someContract1),
					createString(
						fmt.Sprintf("%s,%s",
							xlmAsset.StringCanonical(), xlmAsset.StringCanonical()),
					),
				}

				return xdr.ContractEvent{
					Type:       xdr.ContractEventTypeContract,
					ContractId: xlmContractId,
					Body: xdr.ContractEventBody{
						V: 0,
						V0: &xdr.ContractEventV0{
							Topics: topics,
							Data:   createInt128(1000),
						},
					},
				}
			},
			isAssetSetInEvent: false, // Asset shouldn't be set due to buildAssets returning multiple assets
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			contractEvent := tc.setupEvent()
			event, err := processor.parseEvent(someTxV3(), &someOperationIndex, contractEvent)

			require.NoError(t, err, "Should not error for this test case")
			require.NotNil(t, event, "Event should be returned")

			// Check if we need to verify asset is set or not set
			if tc.isAssetSetInEvent {
				eventAsset := event.GetAsset()

				assert.NotNil(t, eventAsset, "Asset should be set for this event")
				assert.True(t, eventAsset.ToXdrAsset().Equals(xlmAsset))
			} else {
				eventAsset := event.GetAsset()
				assert.Nil(t, eventAsset, "Asset should not be set for this event")
			}

		})
	}
}

// ------ V4 Testing -----

func TestValidSep41EventsWithExtraTopicsAndDataV4(t *testing.T) {
	// Create V4 transaction
	v4Tx := someTxV3()
	v4Tx.UnsafeMeta.V = 4
	v4Tx.UnsafeMeta.V4 = &xdr.TransactionMetaV4{
		Operations: []xdr.OperationMetaV2{{}},
	}

	mapWithAmountMuxedInfoAndExtraFields := createScMap(
		"amount", createInt128(thousand),
		"to_muxed_id", uint64(999),
		"some random key", "some random value",
	)

	mapWithJustAmount := createScMap("amount", createInt128(thousand))

	createContract := func(contractId *xdr.ContractId, topics []xdr.ScVal, data xdr.ScVal) xdr.ContractEvent {
		return xdr.ContractEvent{
			Type:       xdr.ContractEventTypeContract,
			ContractId: contractId,
			Body: xdr.ContractEventBody{
				V: 0,
				V0: &xdr.ContractEventV0{
					Topics: topics,
					Data:   data,
				},
			},
		}
	}

	testCases := []struct {
		name          string
		setupEvent    func() xdr.ContractEvent
		validateEvent func(t *testing.T, event *TokenTransferEvent)
	}{
		{
			name: "Transfer Event with extra topics and i128 amount - Valid SEP-41 token",
			setupEvent: func() xdr.ContractEvent {
				topics := []xdr.ScVal{
					createSymbol(TransferEvent),
					createAddress(randomAccount),              // from
					createAddress(someContract1),              // to
					createString("some random extra topic 1"), // extra
					createString("some random extra topic 2"), // extra
					createString(
						fmt.Sprintf("%s,%s", xlmAsset.StringCanonical(), xlmAsset.StringCanonical()), // spoofing a SAC event
					),
				}
				data := createInt128(thousand)
				return createContract(&someContractId1, topics, data)
			},
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetTransfer())
				assert.Equal(t, randomAccount, event.GetTransfer().From)
				assert.Equal(t, someContract1, event.GetTransfer().To)
				assert.Nil(t, event.GetAsset())
				assert.Equal(t, thousandStr, event.GetTransfer().Amount)
			},
		},
		{
			name: "Transfer Event with extra topics and map data with extra fields - Valid Sep-41 token",
			setupEvent: func() xdr.ContractEvent {
				topics := []xdr.ScVal{
					createSymbol(TransferEvent),
					createAddress(randomAccount),              // from
					createAddress(someContract1),              // to
					createString("some random extra topic 1"), // extra
					createString("some random extra topic 2"), // extra
				}
				data := mapWithAmountMuxedInfoAndExtraFields
				return createContract(&someContractId1, topics, data)
			},
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetTransfer())
				assert.Equal(t, randomAccount, event.GetTransfer().From)
				assert.Equal(t, someContract1, event.GetTransfer().To)
				assert.Nil(t, event.GetAsset())
				assert.Equal(t, thousandStr, event.GetTransfer().Amount)
				assert.NotNil(t, event.Meta.GetToMuxedInfo())
				assert.Equal(t, uint64(999), event.Meta.GetToMuxedInfo().GetId())
			},
		}, {
			name: "Transfer Event with extra topics and just amount as map data - Valid Sep-41 token",
			setupEvent: func() xdr.ContractEvent {
				topics := []xdr.ScVal{
					createSymbol(TransferEvent),
					createAddress(randomAccount),              // from
					createAddress(someContract1),              // to
					createString("some random extra topic 1"), // extra
					createString("some random extra topic 2"), // extra
				}
				data := mapWithJustAmount
				return createContract(&someContractId1, topics, data)
			},
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetTransfer())
				assert.Equal(t, randomAccount, event.GetTransfer().From)
				assert.Equal(t, someContract1, event.GetTransfer().To)
				assert.Nil(t, event.GetAsset())
				assert.Equal(t, thousandStr, event.GetTransfer().Amount)
				assert.Nil(t, event.Meta.GetToMuxedInfo())
			},
		},

		{
			name: "Mint Event with extra topics and i128 amount - Valid SEP-41 token",
			setupEvent: func() xdr.ContractEvent {
				topics := []xdr.ScVal{
					createSymbol(MintEvent),
					createAddress(someContract1),              // to
					createString("some random extra topic 1"), // extra
					createString("some random extra topic 2"), // extra
					createString(
						fmt.Sprintf("%s,%s", xlmAsset.StringCanonical(), xlmAsset.StringCanonical()), // spoofing a SAC event
					),
				}
				data := createInt128(thousand)
				return createContract(&someContractId1, topics, data)
			},
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetMint())
				assert.Equal(t, someContract1, event.GetMint().To)
				assert.Nil(t, event.GetAsset())
				assert.Equal(t, thousandStr, event.GetMint().Amount)
			},
		},
		{
			name: "Mint Event with extra topics and map data with extra fields - Valid Sep-41 token",
			setupEvent: func() xdr.ContractEvent {
				topics := []xdr.ScVal{
					createSymbol(MintEvent),
					createAddress(someContract1),              // to
					createString("some random extra topic 1"), // extra
					createString("some random extra topic 2"), // extra
				}
				data := mapWithAmountMuxedInfoAndExtraFields
				return createContract(&someContractId1, topics, data)
			},
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetMint())
				assert.Equal(t, someContract1, event.GetMint().To)
				assert.Nil(t, event.GetAsset())
				assert.Equal(t, thousandStr, event.GetMint().Amount)
				assert.NotNil(t, event.Meta.GetToMuxedInfo())
				assert.Equal(t, uint64(999), event.Meta.GetToMuxedInfo().GetId())
			},
		}, {
			name: "Mint Event with extra topics and just amount as map data - Valid Sep-41 token",
			setupEvent: func() xdr.ContractEvent {
				topics := []xdr.ScVal{
					createSymbol(MintEvent),
					createAddress(someContract1),              // to
					createString("some random extra topic 1"), // extra
					createString("some random extra topic 2"), // extra
				}
				data := mapWithJustAmount
				return createContract(&someContractId1, topics, data)
			},
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetMint())
				assert.Equal(t, someContract1, event.GetMint().To)
				assert.Nil(t, event.GetAsset())
				assert.Equal(t, thousandStr, event.GetMint().Amount)
				assert.Nil(t, event.Meta.GetToMuxedInfo())
			},
		},

		{
			name: "Burn Event with extra topics and i128 amount - Valid SEP-41 token",
			setupEvent: func() xdr.ContractEvent {
				topics := []xdr.ScVal{
					createSymbol(BurnEvent),
					createAddress(someContract1),              // from
					createString("some random extra topic 1"), // extra
					createString("some random extra topic 2"), // extra
					createString(
						fmt.Sprintf("%s,%s", xlmAsset.StringCanonical(), xlmAsset.StringCanonical()), // spoofing a SAC event
					),
				}
				data := createInt128(thousand)
				return createContract(&someContractId1, topics, data)
			},
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetBurn())
				assert.Equal(t, someContract1, event.GetBurn().From)
				assert.Nil(t, event.GetAsset())
				assert.Equal(t, thousandStr, event.GetBurn().Amount)
			},
		},
		{
			name: "Burn Event with extra topics and map data with extra fields - Valid Sep-41 token",
			setupEvent: func() xdr.ContractEvent {
				topics := []xdr.ScVal{
					createSymbol(BurnEvent),
					createAddress(someContract1),              // from
					createString("some random extra topic 1"), // extra
					createString("some random extra topic 2"), // extra
				}
				data := mapWithAmountMuxedInfoAndExtraFields
				return createContract(&someContractId1, topics, data)
			},
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetBurn())
				assert.Equal(t, someContract1, event.GetBurn().From)
				assert.Nil(t, event.GetAsset())
				assert.Equal(t, thousandStr, event.GetBurn().Amount)
				assert.NotNil(t, event.Meta.GetToMuxedInfo())
				assert.Equal(t, uint64(999), event.Meta.GetToMuxedInfo().GetId())
			},
		}, {
			name: "Burn Event with extra topics and just amount as map data - Valid Sep-41 token",
			setupEvent: func() xdr.ContractEvent {
				topics := []xdr.ScVal{
					createSymbol(BurnEvent),
					createAddress(someContract1),              // from
					createString("some random extra topic 1"), // extra
					createString("some random extra topic 2"), // extra
				}
				data := mapWithJustAmount
				return createContract(&someContractId1, topics, data)
			},
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetBurn())
				assert.Equal(t, someContract1, event.GetBurn().From)
				assert.Nil(t, event.GetAsset())
				assert.Equal(t, thousandStr, event.GetBurn().Amount)
				assert.Nil(t, event.Meta.GetToMuxedInfo())
			},
		},

		{
			name: "Clawback Event with extra topics and i128 amount - Valid SEP-41 token",
			setupEvent: func() xdr.ContractEvent {
				topics := []xdr.ScVal{
					createSymbol(ClawbackEvent),
					createAddress(someContract1),              // from
					createString("some random extra topic 1"), // extra
					createString("some random extra topic 2"), // extra
					createString(
						fmt.Sprintf("%s,%s", xlmAsset.StringCanonical(), xlmAsset.StringCanonical()), // spoofing a SAC event
					),
				}
				data := createInt128(thousand)
				return createContract(&someContractId1, topics, data)
			},
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetClawback())
				assert.Equal(t, someContract1, event.GetClawback().From)
				assert.Nil(t, event.GetAsset())
				assert.Equal(t, thousandStr, event.GetClawback().Amount)
			},
		},
		{
			name: "Clawback Event with extra topics and map data with extra fields - Valid Sep-41 token",
			setupEvent: func() xdr.ContractEvent {
				topics := []xdr.ScVal{
					createSymbol(ClawbackEvent),
					createAddress(someContract1),              // from
					createString("some random extra topic 1"), // extra
					createString("some random extra topic 2"), // extra
				}
				data := mapWithAmountMuxedInfoAndExtraFields
				return createContract(&someContractId1, topics, data)
			},
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetClawback())
				assert.Equal(t, someContract1, event.GetClawback().From)
				assert.Nil(t, event.GetAsset())
				assert.Equal(t, thousandStr, event.GetClawback().Amount)
				assert.NotNil(t, event.Meta.GetToMuxedInfo())
				assert.Equal(t, uint64(999), event.Meta.GetToMuxedInfo().GetId())
			},
		}, {
			name: "Clawback Event with extra topics and just amount as map data - Valid Sep-41 token",
			setupEvent: func() xdr.ContractEvent {
				topics := []xdr.ScVal{
					createSymbol(ClawbackEvent),
					createAddress(someContract1),              // from
					createString("some random extra topic 1"), // extra
					createString("some random extra topic 2"), // extra
				}
				data := mapWithJustAmount
				return createContract(&someContractId1, topics, data)
			},
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetClawback())
				assert.Equal(t, someContract1, event.GetClawback().From)
				assert.Nil(t, event.GetAsset())
				assert.Equal(t, thousandStr, event.GetClawback().Amount)
				assert.Nil(t, event.Meta.GetToMuxedInfo())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			contractEvent := tc.setupEvent()
			event, err := processor.parseEvent(v4Tx, &someOperationIndex, contractEvent)
			require.NoError(t, err, "Should not error for this test case")
			require.NotNil(t, event, "Event should be returned")
			tc.validateEvent(t, event)
		})
	}

}

func TestValidContractEventsV4(t *testing.T) {
	// Create V4 transaction
	v4Tx := someTxV3()
	v4Tx.UnsafeMeta.V = 4
	v4Tx.UnsafeMeta.V4 = &xdr.TransactionMetaV4{
		Operations: []xdr.OperationMetaV2{{}},
	}

	testCases := []struct {
		name          string
		eventType     string
		addr1         string // meaning depends on event type (from/admin)
		addr2         string // meaning depends on event type (to/from/empty)
		amount        int64
		isSacEvent    bool
		hasV4Memo     bool
		memoType      string // "id", "text", "hash"
		memoValue     interface{}
		validateEvent func(t *testing.T, event *TokenTransferEvent)
	}{
		{
			name:       "V4 Transfer SEP-41 Token Event - No Map (Direct i128)",
			eventType:  TransferEvent,
			addr1:      someContract1, // from
			addr2:      someContract2, // to
			amount:     thousand,
			isSacEvent: false,
			hasV4Memo:  false,
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetTransfer())
				assert.Equal(t, someContract1, event.GetTransfer().From)
				assert.Equal(t, someContract2, event.GetTransfer().To)
				assert.Equal(t, thousandStr, event.GetTransfer().Amount)
				assert.Nil(t, event.GetAsset())       // asset is nil for non-SAC events
				assert.Nil(t, event.Meta.ToMuxedInfo) // no memo
			},
		},
		{
			name:       "V4 Transfer SEP-41 Token Event - With Amount and ID Memo",
			eventType:  TransferEvent,
			addr1:      someContract1, // from
			addr2:      someContract2, // to
			amount:     thousand,
			isSacEvent: false,
			hasV4Memo:  true,
			memoType:   "id",
			memoValue:  uint64(12345),
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetTransfer())
				assert.Equal(t, someContract1, event.GetTransfer().From)
				assert.Equal(t, someContract2, event.GetTransfer().To)
				assert.Equal(t, thousandStr, event.GetTransfer().Amount)
				assert.Nil(t, event.GetAsset()) // asset is nil for non-SAC events
				assert.NotNil(t, event.Meta.ToMuxedInfo)
				assert.Equal(t, uint64(12345), event.Meta.ToMuxedInfo.GetId())
			},
		},
		{
			name:       "V4 Transfer SEP-41 Token Event - With Amount and Text Memo",
			eventType:  TransferEvent,
			addr1:      someContract1, // from
			addr2:      someContract2, // to
			amount:     thousand,
			isSacEvent: false,
			hasV4Memo:  true,
			memoType:   "text",
			memoValue:  "hello world",
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetTransfer())
				assert.Equal(t, someContract1, event.GetTransfer().From)
				assert.Equal(t, someContract2, event.GetTransfer().To)
				assert.Equal(t, thousandStr, event.GetTransfer().Amount)
				assert.Nil(t, event.GetAsset()) // asset is nil for non-SAC events
				assert.NotNil(t, event.Meta.ToMuxedInfo)
				assert.Equal(t, "hello world", event.Meta.ToMuxedInfo.GetText())
			},
		},
		{
			name:       "V4 Transfer SEP-41 Token Event - With Amount and Hash Memo",
			eventType:  TransferEvent,
			addr1:      someContract1, // from
			addr2:      someContract2, // to
			amount:     thousand,
			isSacEvent: false,
			hasV4Memo:  true,
			memoType:   "hash",
			memoValue:  []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32},
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetTransfer())
				assert.Equal(t, someContract1, event.GetTransfer().From)
				assert.Equal(t, someContract2, event.GetTransfer().To)
				assert.Equal(t, thousandStr, event.GetTransfer().Amount)
				assert.Nil(t, event.GetAsset()) // asset is nil for non-SAC events
				assert.NotNil(t, event.Meta.ToMuxedInfo)
				expectedHash := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
				assert.Equal(t, expectedHash, event.Meta.ToMuxedInfo.GetHash())
			},
		},
		{
			name:       "V4 Mint SEP-41 Token Event - No Admin Address",
			eventType:  MintEvent,
			addr1:      someContract1, // to
			amount:     thousand,
			isSacEvent: false,
			hasV4Memo:  false,
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetMint())
				assert.Equal(t, someContract1, event.GetMint().To)
				assert.Equal(t, thousandStr, event.GetMint().Amount)
				assert.Nil(t, event.GetAsset()) // asset is nil for non-SAC events
			},
		},
		{
			name:       "V4 Clawback SEP-41 Token Event - No Admin Address",
			eventType:  ClawbackEvent,
			addr1:      someContract1, // from
			amount:     thousand,
			isSacEvent: false,
			hasV4Memo:  false,
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetClawback())
				assert.Equal(t, someContract1, event.GetClawback().From)
				assert.Equal(t, thousandStr, event.GetClawback().Amount)
				assert.Nil(t, event.GetAsset()) // asset is nil for non-SAC events
			},
		},
		{
			name:       "V4 Burn SEP-41 Token Event - Same as V3",
			eventType:  BurnEvent,
			addr1:      randomAccount, // from
			amount:     thousand,
			isSacEvent: false,
			hasV4Memo:  false,
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetBurn())
				assert.Equal(t, randomAccount, event.GetBurn().From)
				assert.Equal(t, thousandStr, event.GetBurn().Amount)
				assert.Nil(t, event.GetAsset()) // asset is nil for non-SAC events
			},
		},
		{
			name:       "V4 Transfer SAC Event - With direct amount",
			eventType:  TransferEvent,
			addr1:      randomAccount, // from
			addr2:      someContract1, // to
			amount:     thousand,
			isSacEvent: true,
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetTransfer())
				assert.Equal(t, randomAccount, event.GetTransfer().From)
				assert.Equal(t, someContract1, event.GetTransfer().To)
				assert.Equal(t, thousandStr, event.GetTransfer().Amount)
				assert.NotNil(t, event.GetAsset())
				assert.Nil(t, event.Meta.ToMuxedInfo)
			},
		},
		{
			name:       "V4 Transfer SAC Event - With Memo",
			eventType:  TransferEvent,
			addr1:      randomAccount, // from
			addr2:      someContract1, // to
			amount:     thousand,
			isSacEvent: true,
			hasV4Memo:  true,
			memoType:   "id",
			memoValue:  uint64(99999),
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetTransfer())
				assert.Equal(t, randomAccount, event.GetTransfer().From)
				assert.Equal(t, someContract1, event.GetTransfer().To)
				assert.Equal(t, thousandStr, event.GetTransfer().Amount)
				assert.NotNil(t, event.GetAsset())
				assert.NotNil(t, event.Meta.ToMuxedInfo)
				assert.Equal(t, uint64(99999), event.Meta.ToMuxedInfo.GetId())
			},
		},
		{
			name:       "V4 Mint SAC Event - With direct amount",
			eventType:  MintEvent,
			addr1:      randomAccount, // to
			amount:     thousand,
			isSacEvent: true,
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetMint())
				assert.Equal(t, randomAccount, event.GetMint().To)
				assert.Equal(t, thousandStr, event.GetMint().Amount)
				assert.NotNil(t, event.GetAsset())
				assert.Nil(t, event.Meta.ToMuxedInfo)
			},
		},
		{
			name:       "V4 Mint SAC Event - With Memo",
			eventType:  MintEvent,
			addr1:      randomAccount, // to
			amount:     thousand,
			isSacEvent: true,
			hasV4Memo:  true,
			memoType:   "id",
			memoValue:  uint64(99999),
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetMint())
				assert.Equal(t, randomAccount, event.GetMint().To)
				assert.Equal(t, thousandStr, event.GetMint().Amount)
				assert.NotNil(t, event.GetAsset())
				assert.NotNil(t, event.Meta.ToMuxedInfo)
				assert.Equal(t, uint64(99999), event.Meta.ToMuxedInfo.GetId())
			},
		},
		{
			name:       "V4 Burn SAC Event - With direct amount",
			eventType:  BurnEvent,
			addr1:      randomAccount, // from
			amount:     thousand,
			isSacEvent: true,
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetBurn())
				assert.Equal(t, randomAccount, event.GetBurn().From)
				assert.Equal(t, thousandStr, event.GetBurn().Amount)
				assert.NotNil(t, event.GetAsset())
				assert.Nil(t, event.Meta.ToMuxedInfo)
			},
		},
		{
			name:       "V4 Burn SAC Event - With Memo",
			eventType:  BurnEvent,
			addr1:      randomAccount, // from
			amount:     thousand,
			isSacEvent: true,
			hasV4Memo:  true,
			memoType:   "id",
			memoValue:  uint64(99999),
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetBurn())
				assert.Equal(t, randomAccount, event.GetBurn().From)
				assert.Equal(t, thousandStr, event.GetBurn().Amount)
				assert.NotNil(t, event.GetAsset())
				assert.NotNil(t, event.Meta.ToMuxedInfo)
				assert.Equal(t, uint64(99999), event.Meta.ToMuxedInfo.GetId())
			},
		},
		{
			name:       "V4 Clawback SAC Event - With direct amount",
			eventType:  ClawbackEvent,
			addr1:      randomAccount, // from
			amount:     thousand,
			isSacEvent: true,
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetClawback())
				assert.Equal(t, randomAccount, event.GetClawback().From)
				assert.Equal(t, thousandStr, event.GetClawback().Amount)
				assert.NotNil(t, event.GetAsset())
				assert.Nil(t, event.Meta.ToMuxedInfo)
			},
		},
		{
			name:       "V4 Clawback SAC Event - With Memo",
			eventType:  ClawbackEvent,
			addr1:      randomAccount, // from
			amount:     thousand,
			isSacEvent: true,
			hasV4Memo:  true,
			memoType:   "id",
			memoValue:  uint64(99999),
			validateEvent: func(t *testing.T, event *TokenTransferEvent) {
				assert.NotNil(t, event.GetClawback())
				assert.Equal(t, randomAccount, event.GetClawback().From)
				assert.Equal(t, thousandStr, event.GetClawback().Amount)
				assert.NotNil(t, event.GetAsset())
				assert.NotNil(t, event.Meta.ToMuxedInfo)
				assert.Equal(t, uint64(99999), event.Meta.ToMuxedInfo.GetId())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create contract event for V4
			var contractEvent xdr.ContractEvent
			var contractId *xdr.ContractId
			var assetStr string

			if tc.isSacEvent {
				// For SAC events, use asset-derived contract ID
				asset := xlmAsset // Use XLM for simplicity
				assetStr = asset.StringCanonical()
				contractId = contractIdFromAsset(asset)
			} else {
				// For non-SAC events, use arbitrary contract ID and asset string
				assetStr = "someNonSep11AssetString"
				contractId = &someContractId1
			}

			// Build topics based on event type and V4 format (no admin for mint/clawback)
			topics := []xdr.ScVal{createSymbol(tc.eventType)}

			if tc.addr1 != "" {
				topics = append(topics, createAddress(tc.addr1)) // from
			}
			if tc.addr2 != "" {
				topics = append(topics, createAddress(tc.addr2)) // to
			}

			if assetStr != "" {
				topics = append(topics, createString(assetStr))
			}

			// Create data - either direct i128 or ScMap with memo
			var data xdr.ScVal
			if tc.hasV4Memo {
				// Create ScMap with amount + to_muxed_id
				mapEntries := xdr.ScMap{}

				// Add amount
				amountVal := createInt128(tc.amount)

				amountEntry := xdr.ScMapEntry{
					Key: createSymbol("amount"),
					Val: amountVal,
				}
				mapEntries = append(mapEntries, amountEntry)

				// Add to_muxed_id based on memo type
				var muxedIdVal xdr.ScVal
				switch tc.memoType {
				case "id":
					id := tc.memoValue.(uint64)
					val := xdr.Uint64(id)
					muxedIdVal = xdr.ScVal{
						Type: xdr.ScValTypeScvU64,
						U64:  &val,
					}
				case "text":
					text := tc.memoValue.(string)
					str := xdr.ScString(text)
					muxedIdVal = xdr.ScVal{
						Type: xdr.ScValTypeScvString,
						Str:  &str,
					}
				case "hash":
					hashBytes := tc.memoValue.([]byte)
					bytes := xdr.ScBytes(hashBytes)
					muxedIdVal = xdr.ScVal{
						Type:  xdr.ScValTypeScvBytes,
						Bytes: &bytes,
					}
				}

				muxedIdEntry := xdr.ScMapEntry{
					Key: createSymbol("to_muxed_id"),
					Val: muxedIdVal,
				}
				mapEntries = append(mapEntries, muxedIdEntry)

				mapPtr := &mapEntries
				data = xdr.ScVal{
					Type: xdr.ScValTypeScvMap,
					Map:  &mapPtr,
				}
			} else {
				data = createInt128(tc.amount)
			}

			contractEvent = xdr.ContractEvent{
				Type:       xdr.ContractEventTypeContract,
				ContractId: contractId,
				Body: xdr.ContractEventBody{
					V: 0,
					V0: &xdr.ContractEventV0{
						Topics: topics,
						Data:   data,
					},
				},
			}

			event, err := processor.parseEvent(v4Tx, &someOperationIndex, contractEvent)
			require.NoError(t, err)
			require.NotNil(t, event)

			tc.validateEvent(t, event)
		})
	}
}

func TestV4FeeEventParsing(t *testing.T) {
	// Create V4 transaction
	v4Tx := someTxV3()
	v4Tx.UnsafeMeta.V = 4
	v4Tx.UnsafeMeta.V4 = &xdr.TransactionMetaV4{
		Operations: []xdr.OperationMetaV2{{}},
	}

	xlmContractId := contractIdFromAsset(xlmAsset)
	temp := *xlmContractId
	xlmContractIdStr := strkey.MustEncode(strkey.VersionByteContract, temp[:])

	tests := []struct {
		name           string
		txLevelEvents  []xdr.TransactionEvent
		hasError       bool
		errorStr       string
		expectedEvents []*TokenTransferEvent
	}{
		{
			name: "Valid Fee Event - One Fee Event",
			txLevelEvents: []xdr.TransactionEvent{
				{xdr.TransactionEventStageTransactionEventStageBeforeAllTxs,
					createContractEventFromTopicsAndData(
						xlmContractId,
						[]xdr.ScVal{
							createSymbol("fee"),
							createAddress(randomAccount),
						},
						createInt128(100),
					),
				},
			},
			hasError: false,
			expectedEvents: []*TokenTransferEvent{
				NewFeeEvent(NewEventMetaFromTx(v4Tx, nil, xlmContractIdStr), randomAccount, "100", xlmProtoAsset),
			},
		},
		{
			name: "Valid Fee Event - 2 Fee Events",
			txLevelEvents: []xdr.TransactionEvent{
				{xdr.TransactionEventStageTransactionEventStageBeforeAllTxs,
					createContractEventFromTopicsAndData(
						xlmContractId,
						[]xdr.ScVal{
							createSymbol("fee"),
							createAddress(randomAccount),
						},
						createInt128(100),
					),
				},
				{xdr.TransactionEventStageTransactionEventStageAfterAllTxs,
					createContractEventFromTopicsAndData(
						xlmContractId,
						[]xdr.ScVal{
							createSymbol("fee"),
							createAddress(randomAccount),
						},
						createInt128(-20), // simulating a fee refund
					),
				},
			},
			hasError: false,
			expectedEvents: []*TokenTransferEvent{
				NewFeeEvent(NewEventMetaFromTx(v4Tx, nil, xlmContractIdStr), randomAccount, "100", xlmProtoAsset),
				NewFeeEvent(NewEventMetaFromTx(v4Tx, nil, xlmContractIdStr), randomAccount, "-20", xlmProtoAsset),
			},
		},
		{
			name: "Valid Fee Event - One Fee Event and other nonsencial events",
			txLevelEvents: []xdr.TransactionEvent{
				{xdr.TransactionEventStageTransactionEventStageBeforeAllTxs,
					createContractEventFromTopicsAndData(
						xlmContractId,
						[]xdr.ScVal{
							createSymbol("fee"),
							createAddress(randomAccount),
						},
						createInt128(100),
					),
				},
				{xdr.TransactionEventStageTransactionEventStageBeforeAllTxs,
					createContractEventFromTopicsAndData(
						xlmContractId,
						[]xdr.ScVal{
							createSymbol("foo"),
						},
						createInt128(100),
					),
				},
			},
			hasError: false,
			expectedEvents: []*TokenTransferEvent{
				NewFeeEvent(NewEventMetaFromTx(v4Tx, nil, xlmContractIdStr), randomAccount, "100", xlmProtoAsset),
			},
		},
		{
			name:          "Error - No fee events",
			txLevelEvents: []xdr.TransactionEvent{},
			hasError:      true,
			errorStr:      "no fee events found",
		},
		{
			name: "Error - More than 2 Fee Events",
			txLevelEvents: []xdr.TransactionEvent{
				{xdr.TransactionEventStageTransactionEventStageBeforeAllTxs,
					createContractEventFromTopicsAndData(
						xlmContractId,
						[]xdr.ScVal{
							createSymbol("fee"),
							createAddress(randomAccount),
						},
						createInt128(100),
					),
				},
				{xdr.TransactionEventStageTransactionEventStageAfterTx,
					createContractEventFromTopicsAndData(
						xlmContractId,
						[]xdr.ScVal{
							createSymbol("fee"),
							createAddress(randomAccount),
						},
						createInt128(-5),
					),
				},
				{xdr.TransactionEventStageTransactionEventStageAfterAllTxs,
					createContractEventFromTopicsAndData(
						xlmContractId,
						[]xdr.ScVal{
							createSymbol("fee"),
							createAddress(randomAccount),
						},
						createInt128(-10),
					),
				},
			},
			hasError: true,
			errorStr: "too many fee events",
		},
		{
			name: "Error - invalid fee event - extra topics",
			txLevelEvents: []xdr.TransactionEvent{
				{xdr.TransactionEventStageTransactionEventStageBeforeAllTxs,
					createContractEventFromTopicsAndData(
						xlmContractId,
						[]xdr.ScVal{
							createSymbol("fee"),
							createAddress(randomAccount),
							createString("extra field"),
						},
						createInt128(100),
					),
				},
			},
			hasError: true,
			errorStr: "invalid topic length for fee event",
		},
		{
			name: "Error - invalid fee event - invalid amount",
			txLevelEvents: []xdr.TransactionEvent{
				{xdr.TransactionEventStageTransactionEventStageBeforeAllTxs,
					createContractEventFromTopicsAndData(
						xlmContractId,
						[]xdr.ScVal{
							createSymbol("fee"),
							createAddress(randomAccount),
						},
						createString("not a valid amount"),
					),
				},
			},
			hasError: true,
			errorStr: "invalid fee amount",
		},
		{
			name: "Error - invalid fee event - invalid contractId",
			txLevelEvents: []xdr.TransactionEvent{
				{xdr.TransactionEventStageTransactionEventStageBeforeAllTxs,
					createContractEventFromTopicsAndData(
						&someContractId1, // not xlm SAC contractId
						[]xdr.ScVal{
							createSymbol("fee"),
							createAddress(randomAccount),
						},
						createInt128(100),
					),
				},
			},
			hasError: true,
			errorStr: "contractId in event does not match xlm SAC contract Id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ttp := NewEventsProcessorForUnifiedEvents(someNetworkPassphrase)
			v4Tx.UnsafeMeta.V4.Events = tc.txLevelEvents
			protoFeeEvents, err := ttp.parseFeeEventsFromTransactionEvents(v4Tx)
			if !tc.hasError {
				require.NoError(t, err)
				assert.Equal(t, len(protoFeeEvents), len(tc.expectedEvents),
					"length mismatch: got %d events, expected %d",
					len(protoFeeEvents), len(tc.expectedEvents))

				// For each expected event, try to find a matching actual event
				for i := range tc.expectedEvents {
					if !proto.Equal(protoFeeEvents[i], tc.expectedEvents[i]) {
						assert.Fail(t, "result does not match expected event",
							"index: %d\nExpected: %+v\nFound: %+v", i, tc.expectedEvents[i], protoFeeEvents[i])
					}
				}
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorStr)
			}

		})
	}
}

func TestV4InvalidEvents(t *testing.T) {
	// Create V4 transaction
	v4Tx := someTxV3()
	v4Tx.UnsafeMeta.V = 4
	v4Tx.UnsafeMeta.V4 = &xdr.TransactionMetaV4{
		Operations: []xdr.OperationMetaV2{{}},
	}

	testCases := []struct {
		name           string
		setupEvent     func() xdr.ContractEvent
		expectedErrMsg string
	}{
		{
			name: "V4 Map: Missing amount field",
			setupEvent: func() xdr.ContractEvent {
				// Create map with only to_muxed_id, missing amount
				mapEntries := xdr.ScMap{
					{
						Key: createSymbol("to_muxed_id"),
						Val: xdr.ScVal{Type: xdr.ScValTypeScvU64, U64: &[]xdr.Uint64{12345}[0]},
					},
				}

				mapPtr := &mapEntries
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
							Data: xdr.ScVal{
								Type: xdr.ScValTypeScvMap,
								Map:  &mapPtr,
							},
						},
					},
				}
			},
			expectedErrMsg: "amount field not found in map",
		},
		{
			name: "V4 Map: Invalid to_muxed_id type",
			setupEvent: func() xdr.ContractEvent {
				// Create map with invalid to_muxed_id type
				mapEntries := xdr.ScMap{
					{
						Key: createSymbol("amount"),
						Val: createInt128(1000),
					},
					{
						Key: createSymbol("to_muxed_id"),
						Val: createSymbol("invalid_type"), // Should be u64, bytes, or string
					},
				}

				mapPtr := &mapEntries
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
							Data: xdr.ScVal{
								Type: xdr.ScValTypeScvMap,
								Map:  &mapPtr,
							},
						},
					},
				}
			},
			expectedErrMsg: "invalid to_muxed_id type for data",
		},
		{
			name: "Unsupported Transaction Meta Version",
			setupEvent: func() xdr.ContractEvent {
				// This will be tested with a V5 transaction
				return createContractEvent(
					TransferEvent,
					randomAccount,
					someContract1,
					1000,
					nil,
					"asset",
					&someContractId1,
				)
			},
			expectedErrMsg: "unsupported transaction meta version: 5",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			contractEvent := tc.setupEvent()

			// Use V5 transaction for the unsupported version test
			testTx := v4Tx
			if tc.name == "Unsupported Transaction Meta Version" {
				testTx.UnsafeMeta.V = 5
			}

			event, err := processor.parseEvent(testTx, &someOperationIndex, contractEvent)

			require.Error(t, err)
			assert.Nil(t, event)
			assert.Contains(t, err.Error(), tc.expectedErrMsg, "Error message should contain expected text")

			// Verify it's the right error type
			_, ok := err.(ErrNotSep41TokenEvent)
			assert.True(t, ok, "Error should be of type ErrNotSep41TokenEvent")
		})
	}
}

func TestVersionSpecificSACValidation(t *testing.T) {
	testCases := []struct {
		name              string
		txMetaVersion     int32
		eventType         string
		topicCount        int
		isAssetSetInEvent bool
	}{
		{
			name:              "V3 transfer with correct topic count should set asset",
			txMetaVersion:     3,
			eventType:         TransferEvent,
			topicCount:        4,
			isAssetSetInEvent: true,
		},
		{
			name:              "V3 mint with admin address should set asset",
			txMetaVersion:     3,
			eventType:         MintEvent,
			topicCount:        4,
			isAssetSetInEvent: true,
		},
		{
			name:              "V4 mint without admin address should set asset",
			txMetaVersion:     4,
			eventType:         MintEvent,
			topicCount:        3,
			isAssetSetInEvent: true,
		},
		{
			name:              "V4 mint with V3 format should not set asset",
			txMetaVersion:     4,
			eventType:         MintEvent,
			topicCount:        4,
			isAssetSetInEvent: false,
		},
		{
			name:              "V3 clawback with admin address should set asset",
			txMetaVersion:     3,
			eventType:         ClawbackEvent,
			topicCount:        4,
			isAssetSetInEvent: true,
		},
		{
			name:              "V4 clawback without admin address should set asset",
			txMetaVersion:     4,
			eventType:         ClawbackEvent,
			topicCount:        3,
			isAssetSetInEvent: true,
		},
		{
			name:              "V3 burn should set asset (same as V4)",
			txMetaVersion:     3,
			eventType:         BurnEvent,
			topicCount:        3,
			isAssetSetInEvent: true,
		},
		{
			name:              "V4 burn should set asset (same as V3)",
			txMetaVersion:     4,
			eventType:         BurnEvent,
			topicCount:        3,
			isAssetSetInEvent: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create transaction with specified version
			testTx := someTxV3()
			testTx.UnsafeMeta.V = tc.txMetaVersion
			if tc.txMetaVersion == 4 {
				testTx.UnsafeMeta.V4 = &xdr.TransactionMetaV4{
					Operations: []xdr.OperationMetaV2{{}},
				}
			}

			// Create contract event with specified topic count
			xlmContractId := contractIdFromAsset(xlmAsset)
			topics := []xdr.ScVal{createSymbol(tc.eventType)}

			// Add topics to reach the desired count
			for i := 1; i < tc.topicCount-1; i++ {
				topics = append(topics, createAddress(randomAccount))
			}
			// Last topic is always the asset
			topics = append(topics, createString(xlmAsset.StringCanonical()))

			contractEvent := xdr.ContractEvent{
				Type:       xdr.ContractEventTypeContract,
				ContractId: xlmContractId,
				Body: xdr.ContractEventBody{
					V: 0,
					V0: &xdr.ContractEventV0{
						Topics: topics,
						Data:   createInt128(1000),
					},
				},
			}

			event, err := processor.parseEvent(testTx, &someOperationIndex, contractEvent)

			require.NoError(t, err, "Should not error for: %s", tc.name)
			require.NotNil(t, event, "Event should be returned")

			if tc.isAssetSetInEvent {
				eventAsset := event.GetAsset()
				assert.NotNil(t, eventAsset, "Asset should be set for: %s", tc.name)
				assert.True(t, eventAsset.ToXdrAsset().Equals(xlmAsset))
			} else {
				eventAsset := event.GetAsset()
				assert.Nil(t, eventAsset, "Asset should not be set for: %s", tc.name)
			}
		})
	}
}

func TestNoMuxedInfoInPreProtocol23SorobanTxEvents(t *testing.T) {
	// This test is specifically to test this edge case - https://github.com/stellar/go-stellar-sdk/issues/5770
	someEvent := createContractEvent(
		TransferEvent,
		someContract1,
		randomAccount,
		1000,
		nil,
		xlmAsset.StringCanonical(),
		contractIdFromAsset(xlmAsset),
	)

	// Just reusing the someTxV3WithOperationChangesAndMemo to get a V3 SorobanTx
	memo := xdr.MemoID(999)
	// The fixture Tx needs to have a TxMemo set
	somePreProtocol23SorobanTxAndMemo := someSorobanTxV3(nil, nil, &memo)

	event, err := processor.parseEvent(somePreProtocol23SorobanTxAndMemo, &someOperationIndex, someEvent)
	assert.NoError(t, err, "Should not error parsing event")
	assert.NotNil(t, event.GetTransfer())
	assert.Equal(t, someContract1, event.GetTransfer().From)
	assert.Equal(t, randomAccount, event.GetTransfer().To)
	assert.Equal(t, thousandStr, event.GetTransfer().Amount)
	// there shudnt be muxedInfo here.
	assert.Nil(t, event.GetMeta().ToMuxedInfo)
}
