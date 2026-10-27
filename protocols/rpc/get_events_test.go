package protocol

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/gxdr"
	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/randxdr"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestEventTypeSetMatches(t *testing.T) {
	var defaultSet EventTypeSet
	all := EventTypeSet{}
	all[EventTypeContract] = nil
	all[EventTypeDiagnostic] = nil
	all[EventTypeSystem] = nil

	onlyContract := EventTypeSet{}
	onlyContract[EventTypeContract] = nil

	contractEvent := xdr.ContractEvent{Type: xdr.ContractEventTypeContract}
	diagnosticEvent := xdr.ContractEvent{Type: xdr.ContractEventTypeDiagnostic}
	systemEvent := xdr.ContractEvent{Type: xdr.ContractEventTypeSystem}

	for _, testCase := range []struct {
		name    string
		set     EventTypeSet
		event   xdr.ContractEvent
		matches bool
	}{
		{
			"all matches Contract events",
			all,
			contractEvent,
			true,
		},
		{
			"all matches System events",
			all,
			systemEvent,
			true,
		},
		{
			"all matches Diagnostic events",
			all,
			systemEvent,
			true,
		},
		{
			"defaultSet matches Contract events",
			defaultSet,
			contractEvent,
			true,
		},
		{
			"defaultSet matches System events",
			defaultSet,
			systemEvent,
			true,
		},
		{
			"defaultSet matches Diagnostic events",
			defaultSet,
			systemEvent,
			true,
		},
		{
			"onlyContract set matches Contract events",
			onlyContract,
			contractEvent,
			true,
		},
		{
			"onlyContract does not match System events",
			onlyContract,
			systemEvent,
			false,
		},
		{
			"onlyContract does not match Diagnostic events",
			defaultSet,
			diagnosticEvent,
			true,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.matches, testCase.set.matches(testCase.event))
		})
	}
}

func TestEventTypeSetValid(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		keys          []string
		expectedError bool
	}{
		{
			"empty set",
			[]string{},
			false,
		},
		{
			"set with one valid element",
			[]string{EventTypeSystem},
			false,
		},
		{
			"set with two valid elements",
			[]string{EventTypeSystem, EventTypeContract},
			false,
		},
		{
			"set with one invalid element",
			[]string{"abc"},
			true,
		},
		{
			"set with multiple invalid elements",
			[]string{"abc", "def"},
			true,
		},
		{
			"set with valid elements mixed with invalid elements",
			[]string{EventTypeSystem, "abc"},
			true,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			set := EventTypeSet{}
			for _, key := range testCase.keys {
				set[key] = nil
			}
			if testCase.expectedError {
				assert.Error(t, set.valid())
			} else {
				require.NoError(t, set.valid())
			}
		})
	}
}

func TestEventTypeSetMarshaling(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		input    string
		expected []string
	}{
		{
			"empty set",
			"",
			[]string{},
		},
		{
			"set with one element",
			"a",
			[]string{"a"},
		},
		{
			"set with more than one element",
			"a,b,c",
			[]string{"a", "b", "c"},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var set EventTypeSet
			input, err := json.Marshal(testCase.input)
			require.NoError(t, err)
			err = set.UnmarshalJSON(input)
			require.NoError(t, err)
			assert.Equal(t, len(testCase.expected), len(set))
			for _, val := range testCase.expected {
				_, ok := set[val]
				assert.True(t, ok)
			}
		})
	}
}

//nolint:funlen
func TestTopicFilterMatches(t *testing.T) {
	transferSym := xdr.ScSymbol("transfer")
	transfer := xdr.ScVal{
		Type: xdr.ScValTypeScvSymbol,
		Sym:  &transferSym,
	}
	sixtyfour := xdr.Uint64(64)
	number := xdr.ScVal{
		Type: xdr.ScValTypeScvU64,
		U64:  &sixtyfour,
	}
	wildCardExactOne := WildCardExactOne
	for _, tc := range []struct {
		name     string
		filter   TopicFilter
		includes []xdr.ScVec
		excludes []xdr.ScVec
	}{
		{
			name:   "<empty>",
			filter: nil,
			includes: []xdr.ScVec{
				{},
			},
			excludes: []xdr.ScVec{
				{transfer},
			},
		},

		// Exact matching
		{
			name: "ScSymbol(transfer)",
			filter: []SegmentFilter{
				{ScVal: scValXDR(t, transfer)},
			},
			includes: []xdr.ScVec{
				{transfer},
			},
			excludes: []xdr.ScVec{
				{number},
				{transfer, transfer},
			},
		},

		// Star
		{
			name: "*",
			filter: []SegmentFilter{
				{Wildcard: &wildCardExactOne},
			},
			includes: []xdr.ScVec{
				{transfer},
			},
			excludes: []xdr.ScVec{
				{transfer, transfer},
			},
		},
		{
			name: "*/transfer",
			filter: []SegmentFilter{
				{Wildcard: &wildCardExactOne},
				{ScVal: scValXDR(t, transfer)},
			},
			includes: []xdr.ScVec{
				{number, transfer},
				{transfer, transfer},
			},
			excludes: []xdr.ScVec{
				{number},
				{number, number},
				{number, transfer, number},
				{transfer},
				{transfer, number},
				{transfer, transfer, transfer},
			},
		},
		{
			name: "transfer/*",
			filter: []SegmentFilter{
				{ScVal: scValXDR(t, transfer)},
				{Wildcard: &wildCardExactOne},
			},
			includes: []xdr.ScVec{
				{transfer, number},
				{transfer, transfer},
			},
			excludes: []xdr.ScVec{
				{number},
				{number, number},
				{number, transfer, number},
				{transfer},
				{number, transfer},
				{transfer, transfer, transfer},
			},
		},
		{
			name: "transfer/*/*",
			filter: []SegmentFilter{
				{ScVal: scValXDR(t, transfer)},
				{Wildcard: &wildCardExactOne},
				{Wildcard: &wildCardExactOne},
			},
			includes: []xdr.ScVec{
				{transfer, number, number},
				{transfer, transfer, transfer},
			},
			excludes: []xdr.ScVec{
				{number},
				{number, number},
				{number, transfer},
				{number, transfer, number, number},
				{transfer},
				{transfer, transfer, transfer, transfer},
			},
		},
		{
			name: "transfer/*/number",
			filter: []SegmentFilter{
				{ScVal: scValXDR(t, transfer)},
				{Wildcard: &wildCardExactOne},
				{ScVal: scValXDR(t, number)},
			},
			includes: []xdr.ScVec{
				{transfer, number, number},
				{transfer, transfer, number},
			},
			excludes: []xdr.ScVec{
				{number},
				{number, number},
				{number, number, number},
				{number, transfer, number},
				{transfer},
				{number, transfer},
				{transfer, transfer, transfer},
				{transfer, number, transfer},
			},
		},
	} {
		name := tc.name
		if name == "" {
			name = topicFilterToString(tc.filter)
		}
		t.Run(name, func(t *testing.T) {
			for _, include := range tc.includes {
				assert.True(
					t,
					tc.filter.Matches(include),
					"Expected %v filter to include %v",
					name,
					include,
				)
			}
			for _, exclude := range tc.excludes {
				assert.False(
					t,
					tc.filter.Matches(exclude),
					"Expected %v filter to exclude %v",
					name,
					exclude,
				)
			}
		})
	}
}

//nolint:funlen
func TestTopicFilterMatchesFlexibleTopicLength(t *testing.T) {
	transferSym := xdr.ScSymbol("transfer")
	transfer := xdr.ScVal{
		Type: xdr.ScValTypeScvSymbol,
		Sym:  &transferSym,
	}
	sixtyfour := xdr.Uint64(64)
	number := xdr.ScVal{
		Type: xdr.ScValTypeScvU64,
		U64:  &sixtyfour,
	}
	wildCardExactOne := WildCardExactOne
	wildCardZeroOrMore := WildCardZeroOrMore
	for _, tc := range []struct {
		name     string
		filter   TopicFilter
		includes []xdr.ScVec
		excludes []xdr.ScVec
	}{
		{
			name: "ScSymbol(transfer)",
			filter: []SegmentFilter{
				{ScVal: scValXDR(t, transfer)},
				{Wildcard: &wildCardZeroOrMore},
			},
			includes: []xdr.ScVec{
				{transfer},
				{transfer, transfer},
				{transfer, number},
			},
			excludes: []xdr.ScVec{
				{number},
				{number, transfer},
			},
		},

		// Star
		{
			name: "*/**",
			filter: []SegmentFilter{
				{Wildcard: &wildCardExactOne},
				{Wildcard: &wildCardZeroOrMore},
			},
			includes: []xdr.ScVec{
				{transfer},
				{number},
				{transfer, transfer},
				{number, transfer},
			},
			excludes: []xdr.ScVec{},
		},
		{
			name: "*/transfer/**",
			filter: []SegmentFilter{
				{Wildcard: &wildCardExactOne},
				{ScVal: scValXDR(t, transfer)},
				{Wildcard: &wildCardZeroOrMore},
			},
			includes: []xdr.ScVec{
				{number, transfer},
				{number, transfer, number},
				{transfer, transfer},
				{transfer, transfer, transfer},
			},
			excludes: []xdr.ScVec{
				{number},
				{number, number},
				{transfer},
				{transfer, number},
			},
		},
		{
			name: "transfer/*/**",
			filter: []SegmentFilter{
				{ScVal: scValXDR(t, transfer)},
				{Wildcard: &wildCardExactOne},
				{Wildcard: &wildCardZeroOrMore},
			},
			includes: []xdr.ScVec{
				{transfer, number},
				{transfer, transfer},
				{transfer, transfer, transfer},
			},
			excludes: []xdr.ScVec{
				{number},
				{number, number},
				{number, transfer, number},
				{transfer},
				{number, transfer},
			},
		},
		{
			name: "transfer/*/*/**",
			filter: []SegmentFilter{
				{ScVal: scValXDR(t, transfer)},
				{Wildcard: &wildCardExactOne},
				{Wildcard: &wildCardExactOne},
				{Wildcard: &wildCardZeroOrMore},
			},
			includes: []xdr.ScVec{
				{transfer, number, number},
				{transfer, transfer, transfer},
				{transfer, transfer, transfer, transfer},
				{transfer, number, transfer, transfer},
			},
			excludes: []xdr.ScVec{
				{number},
				{number, number},
				{number, transfer},
				{number, transfer, number, number},
				{transfer},
			},
		},
		{
			name: "transfer/*/number/**",
			filter: []SegmentFilter{
				{ScVal: scValXDR(t, transfer)},
				{Wildcard: &wildCardExactOne},
				{ScVal: scValXDR(t, number)},
				{Wildcard: &wildCardZeroOrMore},
			},
			includes: []xdr.ScVec{
				{transfer, number, number},
				{transfer, transfer, number},
				{transfer, number, number, number},
				{transfer, number, number, transfer},
			},
			excludes: []xdr.ScVec{
				{number},
				{number, number},
				{number, number, number},
				{number, transfer, number},
				{transfer},
				{number, transfer},
				{transfer, transfer, transfer},
				{transfer, number, transfer},
			},
		},
	} {
		name := tc.name
		if name == "" {
			name = topicFilterToString(tc.filter)
		}
		t.Run(name, func(t *testing.T) {
			for _, include := range tc.includes {
				assert.True(
					t,
					tc.filter.Matches(include),
					"Expected %v filter to include %v",
					name,
					include,
				)
			}
			for _, exclude := range tc.excludes {
				assert.False(
					t,
					tc.filter.Matches(exclude),
					"Expected %v filter to exclude %v",
					name,
					exclude,
				)
			}
		})
	}
}

func TestTopicFilterJSON(t *testing.T) {
	var got TopicFilter

	// empty topic
	require.NoError(t, json.Unmarshal([]byte("[]"), &got))
	require.Equal(t, TopicFilter{}, got)

	// wildcard "*" as only segment
	wildCardExactlyOne := "*"
	require.NoError(t, json.Unmarshal([]byte("[\"*\"]"), &got))
	require.Equal(t, TopicFilter{{Wildcard: &wildCardExactlyOne}}, got)

	// wildcard "**" as only segment
	wildCardZeroOrMore := "**"
	require.NoError(t, json.Unmarshal([]byte("[\"**\"]"), &got))
	require.Equal(t, TopicFilter{{Wildcard: &wildCardZeroOrMore}}, got)

	sixtyfour := xdr.Uint64(64)
	scval := xdr.ScVal{Type: xdr.ScValTypeScvU64, U64: &sixtyfour}
	scvalstr, err := xdr.MarshalBase64(scval)
	require.NoError(t, err)

	//  valid ScVal segment
	require.NoError(t, json.Unmarshal(fmt.Appendf(nil, "[%q]", scvalstr), &got))
	require.Equal(t, TopicFilter{{ScVal: scValXDR(t, scval)}}, got)

	// valid topic with multiple ScVal segments (no wildcard)
	jsonStr := fmt.Sprintf(`[ %q, %q, %q ]`, scvalstr, scvalstr, scvalstr)
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &got))
	require.Equal(t, TopicFilter{
		{ScVal: scValXDR(t, scval)},
		{ScVal: scValXDR(t, scval)},
		{ScVal: scValXDR(t, scval)},
	}, got)
	err = got.Valid()
	require.NoError(t, err)

	// valid topic with wildcard at the end
	jsonStr = fmt.Sprintf(`[ %q, "**" ]`, scvalstr)
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &got))
	require.Equal(t, TopicFilter{
		{ScVal: scValXDR(t, scval)},
		{Wildcard: &wildCardZeroOrMore},
	}, got)

	//  valid topic with wildcard at end and other segments
	jsonStr = fmt.Sprintf(`[ %q, %q, "**" ]`, scvalstr, scvalstr)
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &got))
	require.Equal(t, TopicFilter{
		{ScVal: scValXDR(t, scval)},
		{ScVal: scValXDR(t, scval)},
		{Wildcard: &wildCardZeroOrMore},
	}, got)

	// invalid topic with wildcard not at end
	jsonStr = fmt.Sprintf(`[ "**", %q ]`, scvalstr)
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &got))
	require.Equal(t, TopicFilter{
		{Wildcard: &wildCardZeroOrMore},
		{ScVal: scValXDR(t, scval)},
	}, got)
	err = got.Valid()
	require.Error(t, err)
	require.Contains(t, err.Error(), "wildcard '**' is only allowed as the last segment")

	// invalid topic with wildcard in the middle
	jsonStr = fmt.Sprintf(`[ %q, "**", %q ]`, scvalstr, scvalstr)
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &got))
	require.Equal(t, TopicFilter{
		{ScVal: scValXDR(t, scval)},
		{Wildcard: &wildCardZeroOrMore},
		{ScVal: scValXDR(t, scval)},
	}, got)
	err = got.Valid()
	require.Error(t, err)
	require.Contains(t, err.Error(), "wildcard '**' is only allowed as the last segment")
}

func topicFilterToString(t TopicFilter) string {
	var s []string
	for _, segment := range t {
		switch {
		case segment.Wildcard != nil:
			s = append(s, *segment.Wildcard)
		case segment.ScVal != nil:
			s = append(s, base64.StdEncoding.EncodeToString(segment.ScVal))
		default:
			panic("Invalid topic filter")
		}
	}
	if len(s) == 0 {
		s = append(s, "<empty>")
	}
	return strings.Join(s, "/")
}

//nolint:funlen
func TestGetEventsRequestValid(t *testing.T) {
	// omit startLedger but include cursor
	var request GetEventsRequest
	require.NoError(t, json.Unmarshal(
		[]byte("{ \"filters\": [], \"pagination\": { \"cursor\": \"0000000021474840576-0000000000\"} }"),
		&request,
	))
	assert.Equal(t, uint32(0), request.StartLedger)
	require.NoError(t, request.Valid(1000))

	require.EqualError(t, (&GetEventsRequest{
		StartLedger: 1,
		Filters:     []EventFilter{},
		Pagination:  &PaginationOptions{Cursor: &Cursor{}},
	}).Valid(1000), "ledger ranges and cursor cannot both be set")

	require.NoError(t, (&GetEventsRequest{
		StartLedger: 1,
		Filters:     []EventFilter{},
		Pagination:  nil,
	}).Valid(1000))

	require.EqualError(t, (&GetEventsRequest{
		StartLedger: 1,
		Filters:     []EventFilter{},
		Pagination:  &PaginationOptions{Limit: 1001},
	}).Valid(1000), "limit must not exceed 1000")

	require.EqualError(t, (&GetEventsRequest{
		StartLedger: 0,
		Filters:     []EventFilter{},
		Pagination:  nil,
	}).Valid(1000), "startLedger must be positive")

	require.EqualError(t, (&GetEventsRequest{
		StartLedger: 1,
		Filters: []EventFilter{
			{}, {}, {}, {}, {}, {},
		},
		Pagination: nil,
	}).Valid(1000), "maximum 5 filters per request")

	err := (&GetEventsRequest{
		StartLedger: 1,
		Filters: []EventFilter{
			{EventType: map[string]interface{}{"foo": nil}},
		},
		Pagination: nil,
	}).Valid(1000)
	expectedErrStr := "filter 1 invalid: filter type invalid: if set, type must be either 'system' or 'contract'" //nolint:lll
	require.EqualError(t, err, expectedErrStr)

	require.EqualError(t, (&GetEventsRequest{
		StartLedger: 1,
		Filters: []EventFilter{
			{ContractIDs: []string{
				"CCVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKUD2U",
				"CC53XO53XO53XO53XO53XO53XO53XO53XO53XO53XO53XO53XO53WQD5",
				"CDGMZTGMZTGMZTGMZTGMZTGMZTGMZTGMZTGMZTGMZTGMZTGMZTGMZLND",
				"CDO53XO53XO53XO53XO53XO53XO53XO53XO53XO53XO53XO53XO53YUK",
				"CDXO53XO53XO53XO53XO53XO53XO53XO53XO53XO53XO53XO53XO4M7R",
				"CD7777777777777777777777777777777777777777777777777767GY",
			}},
		},
		Pagination: nil,
	}).Valid(1000), "filter 1 invalid: maximum 5 contract IDs per filter")

	require.EqualError(t, (&GetEventsRequest{
		StartLedger: 1,
		Filters: []EventFilter{
			{ContractIDs: []string{"a"}},
		},
		Pagination: nil,
	}).Valid(1000), "filter 1 invalid: contract ID 1 invalid")

	require.EqualError(t, (&GetEventsRequest{
		StartLedger: 1,
		Filters: []EventFilter{
			{ContractIDs: []string{"CCVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKVKVINVALID"}},
		},
		Pagination: nil,
	}).Valid(1000), "filter 1 invalid: contract ID 1 invalid")

	require.EqualError(t, (&GetEventsRequest{
		StartLedger: 1,
		Filters: []EventFilter{
			{
				Topics: []TopicFilter{
					{}, {}, {}, {}, {}, {},
				},
			},
		},
		Pagination: nil,
	}).Valid(1000), "filter 1 invalid: maximum 5 topics per filter")

	require.EqualError(t, (&GetEventsRequest{
		StartLedger: 1,
		Filters: []EventFilter{
			{Topics: []TopicFilter{
				{},
			}},
		},
		Pagination: nil,
	}).Valid(1000), "filter 1 invalid: topic 1 invalid: topic must have at least 1 segment")

	require.EqualError(t, (&GetEventsRequest{
		StartLedger: 1,
		Filters: []EventFilter{
			{Topics: []TopicFilter{
				{
					{},
					{},
					{},
					{},
					{},
				},
			}},
		},
		Pagination: nil,
	}).Valid(1000), "filter 1 invalid: topic 1 invalid: topic cannot have more than 4 segments")

	wildCardExactOne := WildCardExactOne
	wildCardZeroOrMore := WildCardZeroOrMore
	require.NoError(t, (&GetEventsRequest{
		StartLedger: 1,
		Filters: []EventFilter{
			{Topics: []TopicFilter{
				[]SegmentFilter{
					{Wildcard: &wildCardExactOne},
					{Wildcard: &wildCardExactOne},
					{Wildcard: &wildCardExactOne},
					{Wildcard: &wildCardExactOne},
					{Wildcard: &wildCardZeroOrMore},
				},
			}},
		},
		Pagination: nil,
	}).Valid(1000))

	require.NoError(t, (&GetEventsRequest{
		StartLedger: 1,
		Filters: []EventFilter{
			{Topics: []TopicFilter{
				[]SegmentFilter{
					{Wildcard: &wildCardExactOne},
					{Wildcard: &wildCardExactOne},
					{Wildcard: &wildCardExactOne},
					{Wildcard: &wildCardExactOne},
					{Wildcard: &wildCardZeroOrMore},
				},
			}},
		},
		Pagination: nil,
	}).Valid(1000))

	require.EqualError(t, (&GetEventsRequest{
		StartLedger: 1,
		Filters: []EventFilter{
			{Topics: []TopicFilter{
				[]SegmentFilter{
					{Wildcard: &wildCardExactOne},
					{Wildcard: &wildCardExactOne},
					{Wildcard: &wildCardExactOne},
					{Wildcard: &wildCardExactOne},
					{Wildcard: &wildCardExactOne},
					{Wildcard: &wildCardZeroOrMore},
				},
			}},
		},
		Pagination: nil,
	}).Valid(1000), "filter 1 invalid: topic 1 invalid: topic cannot have more than 4 segments")

	require.NoError(t, (&GetEventsRequest{
		StartLedger: 1,
		Filters: []EventFilter{
			{Topics: []TopicFilter{
				[]SegmentFilter{
					{Wildcard: &wildCardZeroOrMore},
				},
			}},
		},
		Pagination: nil,
	}).Valid(1000))

	require.EqualError(t, (&GetEventsRequest{
		StartLedger: 1,
		Filters: []EventFilter{
			{Topics: []TopicFilter{
				[]SegmentFilter{
					{Wildcard: &wildCardExactOne},
					{Wildcard: &wildCardZeroOrMore},
					{Wildcard: &wildCardZeroOrMore},
				},
			}},
		},
		Pagination: nil,
	}).Valid(1000), "filter 1 invalid: topic 1 invalid: "+
		"segment 2 invalid: wildcard '**' is only allowed as the last segment")

	require.EqualError(t, (&GetEventsRequest{
		StartLedger: 1,
		Filters: []EventFilter{
			{Topics: []TopicFilter{
				[]SegmentFilter{
					{Wildcard: &wildCardZeroOrMore},
					{Wildcard: &wildCardExactOne},
					{Wildcard: &wildCardZeroOrMore},
				},
			}},
		},
		Pagination: nil,
	}).Valid(1000), "filter 1 invalid: topic 1 invalid: "+
		"segment 1 invalid: wildcard '**' is only allowed as the last segment")

	// Segments built in code are held to the same rules as decoded ones.
	sym := xdr.ScSymbol("transfer")
	symbol := scValXDR(t, xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &sym})
	for _, tc := range []struct {
		topic   TopicFilter
		wantErr string
	}{
		{
			TopicFilter{{ScVal: append(append(xdr.ScValView{}, symbol...), 0, 0, 0, 0)}},
			"segment 1 invalid: scval: xdr view: trailing bytes",
		},
		{TopicFilter{{ScVal: xdr.ScValView{0, 0, 0, 0, 0, 0, 0, 2}}}, "segment 1 invalid: scval: xdr view: bool"},
		{
			TopicFilter{{ScVal: symbol}, {Wildcard: &wildCardZeroOrMore, ScVal: symbol}},
			"segment 2 invalid: cannot set both wildcard and scval",
		},
	} {
		request := GetEventsRequest{StartLedger: 1, Filters: []EventFilter{{Topics: []TopicFilter{tc.topic}}}}
		require.ErrorContains(t, request.Valid(1000), "filter 1 invalid: topic 1 invalid: "+tc.wantErr)
	}
}

func TestEventFilterSerialization(t *testing.T) {
	acct, err := xdr.AddressToAccountId(keypair.MustRandom().Address())
	require.NoError(t, err)

	scv := xdr.ScVal{
		Type: xdr.ScValTypeScvAddress,
		Address: &xdr.ScAddress{
			Type:      xdr.ScAddressTypeScAddressTypeAccount,
			AccountId: &acct,
		},
	}
	b64, err := xdr.MarshalBase64(scv)
	require.NoError(t, err)

	wc1 := WildCardExactOne
	wc0 := WildCardZeroOrMore

	for _, testCase := range []struct {
		Filter  SegmentFilter
		Encoded string
	}{
		{SegmentFilter{Wildcard: &wc1}, `"*"`},
		{SegmentFilter{Wildcard: &wc0}, `"**"`},
		{SegmentFilter{ScVal: scValXDR(t, scv)}, fmt.Sprintf("%q", b64)},
	} {
		filter := EventFilter{Topics: []TopicFilter{{testCase.Filter}}}

		var b []byte
		b, err = json.Marshal(testCase.Filter)
		require.NoError(t, err)
		require.JSONEq(t, testCase.Encoded, string(b))

		var f []byte
		f, err = json.Marshal(filter)
		require.NoError(t, err)
		require.JSONEq(t, fmt.Sprintf(`{"topics":[[%s]]}`, string(b)), string(f))
	}

	_, err = json.Marshal(SegmentFilter{})
	require.Error(t, err)
}

func scValXDR(t *testing.T, v xdr.ScVal) xdr.ScValView {
	t.Helper()
	raw, err := v.MarshalBinary()
	require.NoError(t, err)
	return raw
}

func TestSegmentFilterJSON(t *testing.T) {
	sixtyfour := xdr.Uint64(64)
	raw := scValXDR(t, xdr.ScVal{Type: xdr.ScValTypeScvU64, U64: &sixtyfour})
	encoded := fmt.Sprintf("%q", base64.StdEncoding.EncodeToString(raw))

	// The segment keeps the XDR it was given and encodes back to the same JSON.
	var segment SegmentFilter
	require.NoError(t, json.Unmarshal([]byte(encoded), &segment))
	require.Equal(t, raw, segment.ScVal)
	out, err := json.Marshal(segment)
	require.NoError(t, err)
	require.JSONEq(t, encoded, string(out))

	for name, invalid := range map[string][]byte{
		"unknown ScVal type":     {0, 0, 0, 99},
		"bool other than 0 or 1": {0, 0, 0, 0, 0, 0, 0, 2},
		"truncated u64":          raw[:len(raw)-4],
		"trailing bytes":         append(append([]byte{}, raw...), 0, 0, 0, 0),
	} {
		in := fmt.Sprintf("%q", base64.StdEncoding.EncodeToString(invalid))
		require.Error(t, json.Unmarshal([]byte(in), &segment), name)
	}
	require.Error(t, json.Unmarshal([]byte(`"not base64"`), &segment))
}

// TestSegmentFilterMatchesEquals checks that matching on topic XDR agrees with
// ScVal.Equals for every pair of random values.
func TestSegmentFilterMatchesEquals(t *testing.T) {
	gen := randxdr.NewGenerator()
	vals := make([]xdr.ScVal, 200)
	for i := range vals {
		shape := &gxdr.SCVal{}
		gen.Next(shape, randxdr.LedgerCloseMetaPresets)
		require.NoError(t, gxdr.Convert(shape, &vals[i]))
	}

	star := WildCardExactOne
	equal := 0
	for i, a := range vals {
		var segment SegmentFilter
		in := fmt.Sprintf("%q", base64.StdEncoding.EncodeToString(scValXDR(t, a)))
		require.NoError(t, json.Unmarshal([]byte(in), &segment))
		// The compared value is the second topic, after a "*" segment.
		request := GetEventsRequest{Filters: []EventFilter{{Topics: []TopicFilter{{{Wildcard: &star}, segment}}}}}
		for j, b := range vals {
			event := xdr.DiagnosticEvent{Event: xdr.ContractEvent{Body: xdr.ContractEventBody{
				V0: &xdr.ContractEventV0{Topics: []xdr.ScVal{a, b}},
			}}}
			want := a.Equals(b)
			require.Equal(t, want, request.Matches(event), "values %d and %d", i, j)
			if want && i != j {
				equal++
			}
		}
	}
	// Equal values from different draws exercise more than identity.
	require.Positive(t, equal)
}

// randomEventFilter returns a filter with random event types, contract IDs of
// random events, and topic filters that match random events with topics.
func randomEventFilter(t *testing.T, rng *rand.Rand, events, withTopics []xdr.DiagnosticEvent) EventFilter {
	filter := EventFilter{EventType: EventTypeSet{}}
	for _, typ := range []string{EventTypeContract, EventTypeSystem} {
		if rng.Intn(3) == 0 {
			filter.EventType[typ] = nil
		}
	}
	for range rng.Intn(3) {
		if id := events[rng.Intn(len(events))].Event.ContractId; id != nil {
			filter.ContractIDs = append(filter.ContractIDs, strkey.MustEncode(strkey.VersionByteContract, id[:]))
		}
	}
	for range rng.Intn(3) {
		filter.Topics = append(filter.Topics, randomTopicFilter(t, rng, withTopics[rng.Intn(len(withTopics))]))
	}
	return filter
}

// randomTopicFilter returns a filter that matches event: each of a random
// number of leading positions, possibly none, is the event's topic or "*",
// followed by "**" when the event has more topics.
func randomTopicFilter(t *testing.T, rng *rand.Rand, event xdr.DiagnosticEvent) TopicFilter {
	star, dstar := WildCardExactOne, WildCardZeroOrMore
	topics := event.Event.Body.MustV0().Topics
	filter := TopicFilter{}
	for _, topic := range topics[:rng.Intn(min(len(topics), MaxTopicCount)+1)] {
		if rng.Intn(2) == 0 {
			filter = append(filter, SegmentFilter{Wildcard: &star})
		} else {
			filter = append(filter, SegmentFilter{ScVal: scValXDR(t, topic)})
		}
	}
	if len(filter) < len(topics) {
		filter = append(filter, SegmentFilter{Wildcard: &dstar})
	}
	return filter
}

// TestEventMatcher checks that an EventMatcher gives the results of
// GetEventsRequest.Matches on random events in XDR form, for requests of up
// to four filters, without allocating.
func TestEventMatcher(t *testing.T) {
	gen := randxdr.NewGenerator()
	events := make([]xdr.DiagnosticEvent, 50)
	// The XDR form of each event: its contract ID (nil when absent) and topics.
	contractIDs := make([][]byte, len(events))
	topicsXDR := make([][][]byte, len(events))
	for i := range events {
		shape := &gxdr.ContractEvent{}
		gen.Next(shape, randxdr.LedgerCloseMetaPresets)
		require.NoError(t, gxdr.Convert(shape, &events[i].Event))
		if id := events[i].Event.ContractId; id != nil {
			contractIDs[i] = id[:]
		}
		for _, topic := range events[i].Event.Body.MustV0().Topics {
			topicsXDR[i] = append(topicsXDR[i], scValXDR(t, topic))
		}
	}

	// Filters are built from events with topics, so that each is one a client
	// can send.
	var withTopics []xdr.DiagnosticEvent
	for _, event := range events {
		if len(event.Event.Body.MustV0().Topics) > 0 {
			withTopics = append(withTopics, event)
		}
	}

	rng := rand.New(rand.NewSource(1)) //nolint:gosec // a fixed seed keeps the test data reproducible
	matched, unmatched := 0, 0
	for range 200 {
		var request GetEventsRequest
		for range rng.Intn(5) {
			request.Filters = append(request.Filters, randomEventFilter(t, rng, events, withTopics))
		}
		matcher, err := NewEventMatcher(request.Filters)
		require.NoError(t, err)
		for i, event := range events {
			want := request.Matches(event)
			typ := event.Event.Type
			require.Equal(t, want, matcher.Matches(typ, contractIDs[i], topicsXDR[i]), "event %d", i)
			if want {
				require.True(t, matcher.MatchesTypeAndContract(typ, contractIDs[i]), "event %d", i)
				matched++
			} else {
				unmatched++
			}
		}
		allocs := testing.AllocsPerRun(10, func() {
			for i, event := range events {
				matcher.MatchesTypeAndContract(event.Event.Type, contractIDs[i])
				matcher.Matches(event.Event.Type, contractIDs[i], topicsXDR[i])
			}
		})
		require.Zero(t, allocs)
	}
	require.Positive(t, matched)
	require.Positive(t, unmatched)

	_, err := NewEventMatcher([]EventFilter{{EventType: EventTypeSet{EventTypeDiagnostic: nil}}})
	require.Error(t, err)
}
