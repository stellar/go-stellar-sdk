package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func testContractIDV2(t *testing.T) string {
	t.Helper()
	id, err := strkey.Encode(strkey.VersionByteContract, bytes.Repeat([]byte{1}, 32))
	require.NoError(t, err)
	return id
}

func shortContractIDV2(t *testing.T) string {
	t.Helper()
	id, err := strkey.Encode(strkey.VersionByteContract, bytes.Repeat([]byte{1}, 31))
	require.NoError(t, err)
	return id
}

func testTopicB64V2(t *testing.T) json.RawMessage {
	t.Helper()
	sym := xdr.ScSymbol("transfer")
	bin, err := xdr.ScVal{Type: xdr.ScValTypeScvSymbol, Sym: &sym}.MarshalBinary()
	require.NoError(t, err)
	raw, err := TopicScVal(xdr.ScValView(bin))
	require.NoError(t, err)
	return raw
}

func TestTopicScValRejectsMalformed(t *testing.T) {
	_, err := TopicScVal(xdr.ScValView([]byte{0xff}))
	assert.Error(t, err)
}

const (
	testOpaqueCursor       = "opaque"
	testTopicXDRV2         = "AAAA"
	errMutuallyExclusiveV2 = "cursor is mutually exclusive with minLedger, maxLedger, order, filters, xdrInputFormat"
	errEmptyFilterV2       = "filters[0]: filter must specify type, contractId, or at least one topic position"
)

func limitV2(v uint) *uint { return &v }

type requestValidCaseV2 struct {
	name    string
	request GetEventsV2Request
	wantErr string // empty means valid
}

func runRequestValidCasesV2(t *testing.T, cases []requestValidCaseV2) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.request.Valid(DefaultMaxFiltersV2)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.EqualError(t, err, tc.wantErr)
				var invalidParams *InvalidParamsError
				require.ErrorAs(t, err, &invalidParams)
				assert.Equal(t, ErrorReasonInvalidParams, invalidParams.Data.Reason)
			}
		})
	}
}

// TestGetEventsV2Constants pins the wire values and spec caps as literals:
// tests that use the constants on both sides pass a value change in
// lockstep, these do not.
func TestGetEventsV2Constants(t *testing.T) {
	assert.EqualValues(t, 1000, MaxLimitV2)
	assert.EqualValues(t, 256, DefaultMaxFiltersV2)
	assert.EqualValues(t, 15, DefaultTermBudgetV2)
	assert.Equal(t, "getEventsV2", GetEventsV2MethodName)
	assert.Equal(t, "asc", OrderAscending)
	assert.Equal(t, "desc", OrderDescending)
	assert.Equal(t, "HAS_MORE", ScanStatusHasMore)
	assert.Equal(t, "WAITING_FOR_LEDGERS", ScanStatusWaitingForLedgers)
	assert.Equal(t, "OLDEST_REACHED", ScanStatusOldestReached)
	assert.Equal(t, "COMPLETE", ScanStatusComplete)
	assert.Equal(t, "invalid_params", ErrorReasonInvalidParams)
	assert.Equal(t, "ledger_out_of_range", ErrorReasonLedgerOutOfRange)
	assert.Equal(t, "cursor_malformed", ErrorReasonCursorMalformed)
}

// TestGetEventsV2RequestValidCustomMaxFilters proves Valid enforces the
// caller's filter cap, not the package default.
func TestGetEventsV2RequestValidCustomMaxFilters(t *testing.T) {
	require.NoError(t,
		(&GetEventsV2Request{MinLedger: 1, Filters: typeOnlyFiltersV2(2)}).Valid(2))
	assert.EqualError(t,
		(&GetEventsV2Request{MinLedger: 1, Filters: typeOnlyFiltersV2(3)}).Valid(2),
		"filters must contain 1 to 2 filters")

	// A zero cap rejects any request that sets filters and passes one that
	// does not.
	assert.Error(t,
		(&GetEventsV2Request{MinLedger: 1, Filters: typeOnlyFiltersV2(1)}).Valid(0))
	assert.NoError(t, (&GetEventsV2Request{MinLedger: 1, Limit: limitV2(1)}).Valid(0))
}

func TestGetEventsV2RequestValid(t *testing.T) {
	contractID := testContractIDV2(t)
	topic := testTopicB64V2(t)

	runRequestValidCasesV2(t, []requestValidCaseV2{
		{
			name:    "ascending with minLedger only",
			request: GetEventsV2Request{MinLedger: 100},
		},
		{
			name:    "descending with no bounds",
			request: GetEventsV2Request{Order: OrderDescending},
		},
		{
			name: "full range query",
			request: GetEventsV2Request{
				MinLedger: 1, MaxLedger: 100, Order: OrderAscending,
				Filters: []EventFilterV2{
					{ContractID: contractID, EventType: EventTypeContract, Topic0: topic},
				},
				XDRInputFormat: FormatBase64, Limit: limitV2(10), Format: FormatJSON,
			},
		},
		{
			name:    "ascending without minLedger",
			request: GetEventsV2Request{Order: OrderAscending},
			wantErr: "minLedger is required for ascending order",
		},
		{
			name:    "default order without minLedger",
			request: GetEventsV2Request{},
			wantErr: "minLedger is required for ascending order",
		},
		{
			name:    "minLedger greater than maxLedger",
			request: GetEventsV2Request{MinLedger: 5, MaxLedger: 2},
			wantErr: "minLedger must be <= maxLedger",
		},
		{
			name:    "inverted bounds rejected for descending too",
			request: GetEventsV2Request{MinLedger: 5, MaxLedger: 2, Order: OrderDescending},
			wantErr: "minLedger must be <= maxLedger",
		},
		{
			name:    "equal bounds accepted",
			request: GetEventsV2Request{MinLedger: 5, MaxLedger: 5},
		},
		{
			name:    "limit exactly at max accepted",
			request: GetEventsV2Request{MinLedger: 1, Limit: limitV2(MaxLimitV2)},
		},
		{
			name:    "invalid order",
			request: GetEventsV2Request{MinLedger: 1, Order: "sideways"},
			wantErr: `order must be "asc" or "desc"`,
		},
		{
			name:    "limit over max",
			request: GetEventsV2Request{MinLedger: 1, Limit: limitV2(MaxLimitV2 + 1)},
			wantErr: fmt.Sprintf("limit must be between 1 and %d", MaxLimitV2),
		},
		{
			name:    "explicit zero limit rejected",
			request: GetEventsV2Request{MinLedger: 1, Limit: limitV2(0)},
			wantErr: fmt.Sprintf("limit must be between 1 and %d", MaxLimitV2),
		},
		{
			name:    "invalid xdrFormat",
			request: GetEventsV2Request{MinLedger: 1, Format: "hex"},
			wantErr: `xdrFormat must be "base64" or "json"`,
		},
		{
			name:    "invalid xdrInputFormat",
			request: GetEventsV2Request{MinLedger: 1, XDRInputFormat: "hex"},
			wantErr: `xdrInputFormat must be "base64" or "json"`,
		},
	})
}

func TestGetEventsV2RequestValidCursor(t *testing.T) {
	runRequestValidCasesV2(t, []requestValidCaseV2{
		{
			name:    "cursor query",
			request: GetEventsV2Request{Cursor: testOpaqueCursor, Limit: limitV2(100), Format: FormatBase64},
		},
		{
			name:    "cursor with minLedger",
			request: GetEventsV2Request{Cursor: testOpaqueCursor, MinLedger: 1},
			wantErr: errMutuallyExclusiveV2,
		},
		{
			name:    "cursor with maxLedger",
			request: GetEventsV2Request{Cursor: testOpaqueCursor, MaxLedger: 5},
			wantErr: errMutuallyExclusiveV2,
		},
		{
			name:    "cursor with order",
			request: GetEventsV2Request{Cursor: testOpaqueCursor, Order: OrderDescending},
			wantErr: errMutuallyExclusiveV2,
		},
		{
			name: "cursor with filters",
			request: GetEventsV2Request{
				Cursor: testOpaqueCursor, Filters: []EventFilterV2{{EventType: EventTypeContract}},
			},
			wantErr: errMutuallyExclusiveV2,
		},
		{
			name:    "cursor with xdrInputFormat",
			request: GetEventsV2Request{Cursor: testOpaqueCursor, XDRInputFormat: FormatBase64},
			wantErr: errMutuallyExclusiveV2,
		},
		{
			// mutual exclusion is the truer complaint: xdrInputFormat is not
			// a legal cursor-query field at all, whatever its value.
			name:    "cursor with invalid xdrInputFormat",
			request: GetEventsV2Request{Cursor: testOpaqueCursor, XDRInputFormat: "hex"},
			wantErr: errMutuallyExclusiveV2,
		},
	})
}

// Explicit "filters": null on the wire decodes like an omitted member.
func TestGetEventsV2FiltersNullJSON(t *testing.T) {
	var cursorReq GetEventsV2Request
	require.NoError(t, json.Unmarshal([]byte(`{"cursor":"x","filters":null}`), &cursorReq))
	assert.NoError(t, cursorReq.Valid(DefaultMaxFiltersV2))

	var rangeReq GetEventsV2Request
	require.NoError(t, json.Unmarshal([]byte(`{"minLedger":1,"filters":null}`), &rangeReq))
	assert.NoError(t, rangeReq.Valid(DefaultMaxFiltersV2))
}

// typeOnlyFiltersV2 builds n filters that each pass the at-least-one-field
// rule, for exercising the count bounds.
func typeOnlyFiltersV2(n int) []EventFilterV2 {
	filters := make([]EventFilterV2, n)
	for i := range filters {
		filters[i].EventType = EventTypeContract
	}
	return filters
}

func TestGetEventsV2RequestValidFilters(t *testing.T) {
	runRequestValidCasesV2(t, []requestValidCaseV2{
		{
			name:    "empty filters array",
			request: GetEventsV2Request{MinLedger: 1, Filters: []EventFilterV2{}},
			wantErr: fmt.Sprintf("filters must contain 1 to %d filters", DefaultMaxFiltersV2),
		},
		{
			name:    "too many filters",
			request: GetEventsV2Request{MinLedger: 1, Filters: typeOnlyFiltersV2(int(DefaultMaxFiltersV2) + 1)},
			wantErr: fmt.Sprintf("filters must contain 1 to %d filters", DefaultMaxFiltersV2),
		},
		{
			name:    "exactly max filters accepted",
			request: GetEventsV2Request{MinLedger: 1, Filters: typeOnlyFiltersV2(int(DefaultMaxFiltersV2))},
		},
		{
			name:    "system event type accepted",
			request: GetEventsV2Request{MinLedger: 1, Filters: []EventFilterV2{{EventType: EventTypeSystem}}},
		},
		{
			name:    "filter with no fields",
			request: GetEventsV2Request{MinLedger: 1, Filters: []EventFilterV2{{}}},
			wantErr: errEmptyFilterV2,
		},
		{
			name: "invalid event type",
			request: GetEventsV2Request{
				MinLedger: 1, Filters: []EventFilterV2{{EventType: "diagnostic"}},
			},
			wantErr: `filters[0]: type must be "contract" or "system"`,
		},
		{
			name: "invalid contract ID",
			request: GetEventsV2Request{
				MinLedger: 1, Filters: []EventFilterV2{{ContractID: "not-a-contract"}},
			},
			wantErr: "filters[0]: contractId is invalid",
		},
		{
			// checksum-valid strkey with a payload that is not 32 bytes
			name: "contract ID with short payload",
			request: GetEventsV2Request{
				MinLedger: 1, Filters: []EventFilterV2{{ContractID: shortContractIDV2(t)}},
			},
			wantErr: "filters[0]: contractId is invalid",
		},
		{
			name: "topic0 position validated",
			request: GetEventsV2Request{
				MinLedger: 1,
				Filters:   []EventFilterV2{{Topic0: json.RawMessage(`"!!!"`)}},
			},
			wantErr: "filters[0].topic0 is not valid base64-encoded XDR",
		},
		{
			name: "topic not base64 XDR under default input format",
			request: GetEventsV2Request{
				MinLedger: 1,
				Filters:   []EventFilterV2{{Topic1: json.RawMessage(`"!!!"`)}},
			},
			wantErr: "filters[0].topic1 is not valid base64-encoded XDR",
		},
		{
			name: "topic not a JSON string under base64 input format",
			request: GetEventsV2Request{
				MinLedger:      1,
				XDRInputFormat: FormatBase64,
				Filters:        []EventFilterV2{{Topic2: json.RawMessage(`{"symbol":"transfer"}`)}},
			},
			wantErr: "filters[0].topic2 is not valid base64-encoded XDR",
		},
		{
			name: "JSON topic under json input format",
			request: GetEventsV2Request{
				MinLedger:      1,
				XDRInputFormat: FormatJSON,
				Filters:        []EventFilterV2{{Topic0: json.RawMessage(`{"symbol":"transfer"}`)}},
			},
		},
		{
			name: "topic3 position validated",
			request: GetEventsV2Request{
				MinLedger: 1,
				Filters:   []EventFilterV2{{Topic3: json.RawMessage(`"!!!"`)}},
			},
			wantErr: "filters[0].topic3 is not valid base64-encoded XDR",
		},
		{
			name: "error names the failing filter index",
			request: GetEventsV2Request{
				MinLedger: 1,
				Filters:   []EventFilterV2{{EventType: EventTypeContract}, {}},
			},
			wantErr: "filters[1]: filter must specify type, contractId, or at least one topic position",
		},
		{
			name: "contractId still validated under json input format",
			request: GetEventsV2Request{
				MinLedger:      1,
				XDRInputFormat: FormatJSON,
				Filters:        []EventFilterV2{{ContractID: "not-a-contract"}},
			},
			wantErr: "filters[0]: contractId is invalid",
		},
	})
}

// Explicit JSON null topics mean the same as omitted (matches any value).
func TestGetEventsV2RequestValidNullTopics(t *testing.T) {
	runRequestValidCasesV2(t, []requestValidCaseV2{
		{
			name: "null topic counts as omitted: empty filter rejected",
			request: GetEventsV2Request{
				MinLedger: 1,
				Filters:   []EventFilterV2{{Topic0: json.RawMessage(`null`)}},
			},
			wantErr: errEmptyFilterV2,
		},
		{
			name: "null topic counts as omitted under json input format too",
			request: GetEventsV2Request{
				MinLedger:      1,
				XDRInputFormat: FormatJSON,
				Filters:        []EventFilterV2{{Topic1: json.RawMessage(`null`)}},
			},
			wantErr: errEmptyFilterV2,
		},
		{
			name: "null topic alongside a real constraint is legal",
			request: GetEventsV2Request{
				MinLedger: 1,
				Filters: []EventFilterV2{{
					EventType: EventTypeContract, Topic0: json.RawMessage(`null`),
				}},
			},
		},
	})
}

func TestGetEventsV2RequestJSONRoundTrip(t *testing.T) {
	contractID := testContractIDV2(t)
	topic := testTopicB64V2(t)

	t.Run("range query", func(t *testing.T) {
		req := GetEventsV2Request{
			MinLedger: 1, MaxLedger: 2, Order: OrderDescending,
			Filters: []EventFilterV2{
				{ContractID: contractID, EventType: EventTypeSystem, Topic0: topic, Topic3: topic},
			},
			XDRInputFormat: FormatBase64, Limit: limitV2(5), Format: FormatJSON,
		}
		raw, err := json.Marshal(req)
		require.NoError(t, err)
		var got GetEventsV2Request
		require.NoError(t, json.Unmarshal(raw, &got))
		assert.Equal(t, req, got)
	})

	// The tags are the public wire contract; round-trips through the same
	// struct pass a tag rename in lockstep, the literal does not.
	t.Run("golden request keys", func(t *testing.T) {
		raw, err := json.Marshal(GetEventsV2Request{
			MinLedger: 1, MaxLedger: 2, Order: OrderAscending,
			Filters: []EventFilterV2{
				{ContractID: contractID, EventType: EventTypeContract,
					Topic0: topic, Topic1: topic, Topic2: topic, Topic3: topic},
			},
			XDRInputFormat: FormatBase64, Limit: limitV2(5), Format: FormatJSON,
		})
		require.NoError(t, err)
		golden := `{"minLedger":1,"maxLedger":2,"order":"asc","filters":[` +
			`{"contractId":` + string(mustJSONV2(t, contractID)) +
			`,"type":"contract","topic0":` + string(topic) + `,"topic1":` + string(topic) +
			`,"topic2":` + string(topic) + `,"topic3":` + string(topic) + `}],` +
			`"xdrInputFormat":"base64","limit":5,"xdrFormat":"json"}`
		assert.Equal(t, golden, string(raw))
	})

	t.Run("cursor query omits range keys", func(t *testing.T) {
		raw, err := json.Marshal(GetEventsV2Request{Cursor: "opaque", Limit: limitV2(10)})
		require.NoError(t, err)
		assert.JSONEq(t, `{"cursor":"opaque","limit":10}`, string(raw))
	})

	t.Run("unset topics are omitted", func(t *testing.T) {
		raw, err := json.Marshal(EventFilterV2{Topic1: topic})
		require.NoError(t, err)
		var keys map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &keys))
		assert.Equal(t, []string{"topic1"}, slices.Collect(maps.Keys(keys)))
	})
}

func TestGetEventsV2ResponseJSON(t *testing.T) {
	t.Run("cursor omitted when empty", func(t *testing.T) {
		raw, err := json.Marshal(GetEventsV2Response{
			Events: []EventInfoV2{}, ScanStatus: ScanStatusComplete,
			ScannedLedger: 5, OldestLedger: 1, LatestLedger: 9,
		})
		require.NoError(t, err)
		assert.NotContains(t, string(raw), `"cursor"`)
		assert.Contains(t, string(raw), `"scanStatus":"COMPLETE"`)
	})

	t.Run("round trip with cursor", func(t *testing.T) {
		resp := GetEventsV2Response{
			Events: []EventInfoV2{{
				EventType: EventTypeContract, Ledger: 3, LedgerClosedAt: "2026-01-01T00:00:00Z",
				ContractID: "C...", ID: "0000000000000003-0000000001",
				OpIndex: 1, TxIndex: 2, TransactionHash: "ab",
				TopicXDR: []string{testTopicXDRV2}, ValueXDR: testTopicXDRV2,
			}},
			Cursor: "opaque", ScanStatus: ScanStatusHasMore,
			ScannedLedger: 3, OldestLedger: 1, LatestLedger: 9,
		}
		raw, err := json.Marshal(resp)
		require.NoError(t, err)
		var got GetEventsV2Response
		require.NoError(t, json.Unmarshal(raw, &got))
		assert.Equal(t, resp, got)
	})

	t.Run("golden response and event keys", func(t *testing.T) {
		raw, err := json.Marshal(GetEventsV2Response{
			Events: []EventInfoV2{{
				EventType: EventTypeContract, Ledger: 3, LedgerClosedAt: "2026-01-01T00:00:00Z",
				ContractID: "C1", ID: "id1", OpIndex: 1, TxIndex: 2, TransactionHash: "ab",
				TopicXDR: []string{testTopicXDRV2}, ValueXDR: testTopicXDRV2,
			}},
			Cursor: "c", ScanStatus: ScanStatusHasMore,
			ScannedLedger: 3, OldestLedger: 1, LatestLedger: 9,
		})
		require.NoError(t, err)
		golden := `{"events":[{"type":"contract","ledger":3,` +
			`"ledgerClosedAt":"2026-01-01T00:00:00Z","contractId":"C1","id":"id1",` +
			`"operationIndex":1,"transactionIndex":2,"txHash":"ab",` +
			`"topic":["AAAA"],"value":"AAAA"}],` +
			`"cursor":"c","scanStatus":"HAS_MORE","scannedLedger":3,` +
			`"oldestLedger":1,"latestLedger":9}`
		assert.Equal(t, golden, string(raw))
	})

	t.Run("golden error data keys", func(t *testing.T) {
		raw, err := json.Marshal(InvalidParamsErrorData{
			Reason: ErrorReasonInvalidParams, TermsUsed: 18, TermBudget: 15,
		})
		require.NoError(t, err)
		assert.Equal(t,
			`{"reason":"invalid_params","termsUsed":18,"termBudget":15}`,
			string(raw))

		raw, err = json.Marshal(InvalidParamsErrorData{Reason: ErrorReasonInvalidParams})
		require.NoError(t, err)
		assert.Equal(t, `{"reason":"invalid_params"}`, string(raw))

		// Reason is deliberately unset: MarshalJSON injects it.
		raw, err = json.Marshal(LedgerOutOfRangeErrorData{
			MissingLedger: 7, OldestLedger: 1, LatestLedger: 9,
		})
		require.NoError(t, err)
		assert.Equal(t,
			`{"reason":"ledger_out_of_range","missingLedger":7,"oldestLedger":1,"latestLedger":9}`,
			string(raw))

		raw, err = json.Marshal(CursorMalformedErrorData{OldestLedger: 1, LatestLedger: 9})
		require.NoError(t, err)
		assert.Equal(t,
			`{"reason":"cursor_malformed","oldestLedger":1,"latestLedger":9}`,
			string(raw))
	})
}

// TestEventInfoV2MirrorsV1 trips when EventInfo and EventInfoV2 drift apart:
// v2 is defined as v1 without the deprecated inSuccessfulContractCall field,
// so a field added to one must be added to the other (or the exception
// listed here). Compares full tags and Go types, not just key names.
func TestEventInfoV2MirrorsV1(t *testing.T) {
	v1 := fieldSpecSet(t, reflect.TypeOf(EventInfo{}))
	v2 := fieldSpecSet(t, reflect.TypeOf(EventInfoV2{}))
	delete(v1, "inSuccessfulContractCall")
	assert.Equal(t, v1, v2)
}

// fieldSpecSet maps each field's JSON key to its full tag and Go type.
func fieldSpecSet(t *testing.T, typ reflect.Type) map[string]string {
	t.Helper()
	specs := make(map[string]string, typ.NumField())
	for i := range typ.NumField() {
		field := typ.Field(i)
		tag := field.Tag.Get("json")
		require.NotEmpty(t, tag)
		specs[strings.Split(tag, ",")[0]] = tag + " " + field.Type.String()
	}
	return specs
}

func mustJSONV2(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}
