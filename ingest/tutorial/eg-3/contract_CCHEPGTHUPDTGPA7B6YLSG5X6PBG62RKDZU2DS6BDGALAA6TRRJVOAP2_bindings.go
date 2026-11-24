// SEP-48 Contract Bindings
// Auto-generated from Soroban contract specification
// Contract ID: CCHEPGTHUPDTGPA7B6YLSG5X6PBG62RKDZU2DS6BDGALAA6TRRJVOAP2
//
// This file contains:
// - Event structures and parsers
// - Function interfaces
// - Type definitions
// - Error constants

package contracts

import (
	"fmt"
	"math/big"
	"github.com/stellar/go/xdr"
)

// Contract metadata
const ContractID = "CCHEPGTHUPDTGPA7B6YLSG5X6PBG62RKDZU2DS6BDGALAA6TRRJVOAP2"

// ============================================================================
// CONTRACT EVENTS (Complete SEP-48 Implementation)
// ============================================================================

// DefaultEventEvent represents the 'DefaultEvent' contract event
//
// Event Structure:
// - Prefix Topics: [default_event]
// - Data Format: Map
// - Topic Parameters: 2
// - Data Parameters: 3
type DefaultEventEvent struct {
	// Event metadata (for validation)
	EventName string `json:"event_name"`
	Prefix0 string `json:"prefix_0"` // Expected: "default_event"

	// Topic parameters (indexed, searchable)
	Addr string `json:"addr"` // Topic: string
	Num uint32 `json:"num"` // Topic: uint32

	// Data parameters (event payload)
	Bignum *big.Int `json:"bignum"` // Data: *big.Int
	Nested []map[string]int64 `json:"nested"` // Data: []map[string]int64
	Any interface{} `json:"any"` // Data: interface{}
}

// ParseDefaultEventEvent parses a 'DefaultEvent' event from Stellar ContractEvent XDR
//
// This parser validates:
// - Topic count and structure
// - Prefix topic values
// - Data format (Map)
// - Parameter types and conversion
//
// Returns: (*DefaultEventEvent, error)
func ParseDefaultEventEvent(contractEvent xdr.ContractEvent) (*DefaultEventEvent, error) {
	// Extract event components from XDR
	topics := contractEvent.Body.V0.Topics
	data := contractEvent.Body.V0.Data

	// Validate topic structure
	if len(topics) < 3 {
		return nil, fmt.Errorf("invalid 'DefaultEvent' event: expected at least 3 topics, got %d", len(topics))
	}

	// Validate prefix topics (event signature)
	topic0, ok0 := topics[0].GetSym()
	if !ok0 {
		return nil, fmt.Errorf("invalid event format: topic0 does not exist")
	}
	if string(topic0) != "default_event" {
		return nil, fmt.Errorf("invalid event signature: expected 'default_event' at topic[0]")
	}

	// Validate data format (expected: Map)
	dataMap, ok := data.GetMap()
	if !ok {
		return nil, fmt.Errorf("invalid event format: data does not exist")
	}

	// Create event instance
	event := &DefaultEventEvent{
		EventName: "DefaultEvent",
		Prefix0: "default_event",
	}

	// Extract and convert topic parameters
	// Topic parameter: addr (string)
	topic1Value, ok := topics[1].GetSym()
	if !ok {
		return nil, fmt.Errorf("invalid event format: topic1")
	}
	event.Addr = string(topic1Value)

	// Topic parameter: num (uint32)
	topic2Value, ok := topics[2].GetU32()
	if !ok {
		return nil, fmt.Errorf("invalid event format: expected uint32 value for topic2")
	}
	event.Num = uint32(topic2Value)

	// Extract and convert data parameters from map
	// Data parameter: bignum (*big.Int)
	if bignumVal, exists := dataMap["bignum"]; exists {
		bignumValue, ok := bignumVal.GetI128()
		if !ok {
			return nil, fmt.Errorf("invalid event format: expected i128 value for bignum")
		}
		event.Bignum = new(big.Int).SetBytes(bignumValue[:])
	} else {
		return nil, fmt.Errorf("missing required data parameter: bignum")
	}

	// Data parameter: nested ([]map[string]int64)
	if nestedVal, exists := dataMap["nested"]; exists {
		// TODO: Convert nestedVal to []map[string]int64
		// This requires custom conversion logic for complex types
		event.Nested = nestedVal // Placeholder
	} else {
		return nil, fmt.Errorf("missing required data parameter: nested")
	}

	// Data parameter: any (interface{})
	if anyVal, exists := dataMap["any"]; exists {
		// Keep any as raw ScVal for interface{} type
		event.Any = anyVal
	} else {
		return nil, fmt.Errorf("missing required data parameter: any")
	}

	return event, nil
}

// ParseContractEvent attempts to parse any contract event
// Returns the parsed event as an interface{} or an error
func ParseContractEvent(contractEvent xdr.ContractEvent) (interface{}, error) {
	// Extract event components from XDR
	topics := contractEvent.Body.V0.Topics
	if len(topics) == 0 {
		return nil, fmt.Errorf("event has no topics")
	}

	// Try to identify event by first topic (event name/prefix)
	firstTopic, ok := topics[0].GetSym()
	if !ok {
		return nil, fmt.Errorf("invalid event format: first topic is not a symbol")
	}
	eventName := string(firstTopic)

	// Dispatch to appropriate parser based on event signature
	switch eventName {
	case "default_event":
		return ParseDefaultEventEvent(contractEvent)
	default:
		return nil, fmt.Errorf("unknown event type: %s", eventName)
	}
}

// ============================================================================
// CONTRACT FUNCTIONS
// ============================================================================

// ContractClient defines the interface for interacting with the contract
type ContractClient interface {
	emit() error

}

