package protocol

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// EventMatcher matches events in their XDR form against getEvents filters, as
// GetEventsRequest.Matches does for decoded events, so a caller that reads
// events as views never decodes them. Matching does not allocate. The zero
// value matches every event.
type EventMatcher struct {
	filters []eventFilterXDR
}

// eventFilterXDR is an EventFilter with its event types and contract IDs in
// XDR form. An empty field matches every event.
type eventFilterXDR struct {
	types       []xdr.ContractEventType
	contractIDs [][]byte
	topics      []TopicFilter
}

// NewEventMatcher compiles filters into an EventMatcher, returning an error
// for a filter that EventFilter.Valid rejects.
func NewEventMatcher(filters []EventFilter) (EventMatcher, error) {
	eventTypes := GetEventTypeXDRFromEventType()
	m := EventMatcher{filters: make([]eventFilterXDR, len(filters))}
	for i := range filters {
		filter := &filters[i]
		if err := filter.Valid(); err != nil {
			return EventMatcher{}, fmt.Errorf("filter %d invalid: %w", i+1, err)
		}
		compiled := &m.filters[i]
		for name := range filter.EventType {
			compiled.types = append(compiled.types, eventTypes[name])
		}
		for _, id := range filter.ContractIDs {
			compiled.contractIDs = append(compiled.contractIDs, strkey.MustDecode(strkey.VersionByteContract, id))
		}
		compiled.topics = filter.Topics
	}
	return m, nil
}

// MatchesTypeAndContract reports whether some filter accepts an event's type
// and contract ID: the ID's 32 bytes, or nil when the event has none. When it
// returns false, Matches does too, so the event's topics need not be read.
func (m EventMatcher) MatchesTypeAndContract(typ xdr.ContractEventType, contractID []byte) bool {
	return len(m.filters) == 0 || slices.ContainsFunc(m.filters, func(f eventFilterXDR) bool {
		return f.matchesTypeAndContract(typ, contractID)
	})
}

// Matches reports whether some filter accepts the event. Each element of
// topics is exactly one topic's XDR.
func (m EventMatcher) Matches(typ xdr.ContractEventType, contractID []byte, topics [][]byte) bool {
	return len(m.filters) == 0 || slices.ContainsFunc(m.filters, func(f eventFilterXDR) bool {
		return f.matchesTypeAndContract(typ, contractID) && (len(f.topics) == 0 ||
			slices.ContainsFunc(f.topics, func(t TopicFilter) bool { return t.matchesXDR(topics) }))
	})
}

func (f eventFilterXDR) matchesTypeAndContract(typ xdr.ContractEventType, contractID []byte) bool {
	return (len(f.types) == 0 || slices.Contains(f.types, typ)) &&
		(len(f.contractIDs) == 0 || slices.ContainsFunc(f.contractIDs, func(id []byte) bool {
			return bytes.Equal(id, contractID)
		}))
}
