package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func testContractID(t *testing.T) string {
	t.Helper()
	id, err := strkey.Encode(strkey.VersionByteContract, bytes.Repeat([]byte{1}, 32))
	require.NoError(t, err)
	return id
}

func shortContractID(t *testing.T) string {
	t.Helper()
	id, err := strkey.Encode(strkey.VersionByteContract, bytes.Repeat([]byte{1}, 31))
	require.NoError(t, err)
	return id
}

func testTopicB64(t *testing.T) json.RawMessage {
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
	testOpaqueCursor     = "opaque"
	testTopicXDR         = "AAAA"
	errMutuallyExclusive = "cursor is mutually exclusive with minLedger, maxLedger, order, filters, xdrInputFormat"
	errEmptyFilter       = "filters[0]: filter must specify type, contractId, or at least one topic position"
)

func limitPtr(v uint) *uint { return &v }

type requestValidCase struct {
	name    string
	request QueryEventsRequest
	wantErr string // empty means valid
}

func runRequestValidCases(t *testing.T, cases []requestValidCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.request.Valid(QueryEventsDefaultMaxFilters)
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

// TestQueryEventsConstants pins the wire values and spec caps as literals:
// tests that use the constants on both sides pass a value change in
// lockstep, these do not.
func TestQueryEventsConstants(t *testing.T) {
	assert.EqualValues(t, 1000, QueryEventsMaxLimit)
	assert.EqualValues(t, 256, QueryEventsDefaultMaxFilters)
	assert.EqualValues(t, 15, QueryEventsDefaultTermBudget)
	assert.Equal(t, "queryEvents", QueryEventsMethodName)
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

// TestQueryEventsRequestValidCustomMaxFilters proves Valid enforces the
// caller's filter cap, not the package default.
func TestQueryEventsRequestValidCustomMaxFilters(t *testing.T) {
	require.NoError(t,
		(&QueryEventsRequest{MinLedger: 1, Filters: typeOnlyFilters(2)}).Valid(2))
	assert.EqualError(t,
		(&QueryEventsRequest{MinLedger: 1, Filters: typeOnlyFilters(3)}).Valid(2),
		"filters must contain 1 to 2 filters")

	// A zero cap rejects any request that sets filters and passes one that
	// does not.
	assert.Error(t,
		(&QueryEventsRequest{MinLedger: 1, Filters: typeOnlyFilters(1)}).Valid(0))
	assert.NoError(t, (&QueryEventsRequest{MinLedger: 1, Limit: limitPtr(1)}).Valid(0))
}

func TestQueryEventsRequestValid(t *testing.T) {
	contractID := testContractID(t)
	topic := testTopicB64(t)

	runRequestValidCases(t, []requestValidCase{
		{
			name:    "ascending with minLedger only",
			request: QueryEventsRequest{MinLedger: 100},
		},
		{
			name:    "descending with no bounds",
			request: QueryEventsRequest{Order: OrderDescending},
		},
		{
			name: "full range query",
			request: QueryEventsRequest{
				MinLedger: 1, MaxLedger: 100, Order: OrderAscending,
				Filters: []QueryEventsFilter{
					{ContractID: contractID, EventType: EventTypeContract, Topic0: topic},
				},
				XDRInputFormat: FormatBase64, Limit: limitPtr(10), Format: FormatJSON,
			},
		},
		{
			name:    "ascending without minLedger",
			request: QueryEventsRequest{Order: OrderAscending},
			wantErr: "minLedger is required for ascending order",
		},
		{
			name:    "default order without minLedger",
			request: QueryEventsRequest{},
			wantErr: "minLedger is required for ascending order",
		},
		{
			name:    "minLedger greater than maxLedger",
			request: QueryEventsRequest{MinLedger: 5, MaxLedger: 2},
			wantErr: "minLedger must be <= maxLedger",
		},
		{
			name:    "inverted bounds rejected for descending too",
			request: QueryEventsRequest{MinLedger: 5, MaxLedger: 2, Order: OrderDescending},
			wantErr: "minLedger must be <= maxLedger",
		},
		{
			name:    "equal bounds accepted",
			request: QueryEventsRequest{MinLedger: 5, MaxLedger: 5},
		},
		{
			name:    "limit exactly at max accepted",
			request: QueryEventsRequest{MinLedger: 1, Limit: limitPtr(QueryEventsMaxLimit)},
		},
		{
			name:    "invalid order",
			request: QueryEventsRequest{MinLedger: 1, Order: "sideways"},
			wantErr: `order must be "asc" or "desc"`,
		},
		{
			name:    "limit over max",
			request: QueryEventsRequest{MinLedger: 1, Limit: limitPtr(QueryEventsMaxLimit + 1)},
			wantErr: fmt.Sprintf("limit must be between 1 and %d", QueryEventsMaxLimit),
		},
		{
			name:    "explicit zero limit rejected",
			request: QueryEventsRequest{MinLedger: 1, Limit: limitPtr(0)},
			wantErr: fmt.Sprintf("limit must be between 1 and %d", QueryEventsMaxLimit),
		},
		{
			name:    "invalid xdrFormat",
			request: QueryEventsRequest{MinLedger: 1, Format: "hex"},
			wantErr: `xdrFormat must be "base64" or "json"`,
		},
		{
			name:    "invalid xdrInputFormat",
			request: QueryEventsRequest{MinLedger: 1, XDRInputFormat: "hex"},
			wantErr: `xdrInputFormat must be "base64" or "json"`,
		},
	})
}

func TestQueryEventsRequestValidCursor(t *testing.T) {
	runRequestValidCases(t, []requestValidCase{
		{
			name:    "cursor query",
			request: QueryEventsRequest{Cursor: testOpaqueCursor, Limit: limitPtr(100), Format: FormatBase64},
		},
		{
			name:    "cursor with minLedger",
			request: QueryEventsRequest{Cursor: testOpaqueCursor, MinLedger: 1},
			wantErr: errMutuallyExclusive,
		},
		{
			name:    "cursor with maxLedger",
			request: QueryEventsRequest{Cursor: testOpaqueCursor, MaxLedger: 5},
			wantErr: errMutuallyExclusive,
		},
		{
			name:    "cursor with order",
			request: QueryEventsRequest{Cursor: testOpaqueCursor, Order: OrderDescending},
			wantErr: errMutuallyExclusive,
		},
		{
			name: "cursor with filters",
			request: QueryEventsRequest{
				Cursor: testOpaqueCursor, Filters: []QueryEventsFilter{{EventType: EventTypeContract}},
			},
			wantErr: errMutuallyExclusive,
		},
		{
			name:    "cursor with xdrInputFormat",
			request: QueryEventsRequest{Cursor: testOpaqueCursor, XDRInputFormat: FormatBase64},
			wantErr: errMutuallyExclusive,
		},
		{
			// mutual exclusion is the truer complaint: xdrInputFormat is not
			// a legal cursor-query field at all, whatever its value.
			name:    "cursor with invalid xdrInputFormat",
			request: QueryEventsRequest{Cursor: testOpaqueCursor, XDRInputFormat: "hex"},
			wantErr: errMutuallyExclusive,
		},
	})
}

// Explicit "filters": null on the wire decodes like an omitted member.
func TestQueryEventsFiltersNullJSON(t *testing.T) {
	var cursorReq QueryEventsRequest
	require.NoError(t, json.Unmarshal([]byte(`{"cursor":"x","filters":null}`), &cursorReq))
	assert.NoError(t, cursorReq.Valid(QueryEventsDefaultMaxFilters))

	var rangeReq QueryEventsRequest
	require.NoError(t, json.Unmarshal([]byte(`{"minLedger":1,"filters":null}`), &rangeReq))
	assert.NoError(t, rangeReq.Valid(QueryEventsDefaultMaxFilters))
}

// typeOnlyFilters builds n filters that each pass the at-least-one-field
// rule, for exercising the count bounds.
func typeOnlyFilters(n int) []QueryEventsFilter {
	filters := make([]QueryEventsFilter, n)
	for i := range filters {
		filters[i].EventType = EventTypeContract
	}
	return filters
}

func TestQueryEventsRequestValidFilters(t *testing.T) {
	runRequestValidCases(t, []requestValidCase{
		{
			name:    "empty filters array",
			request: QueryEventsRequest{MinLedger: 1, Filters: []QueryEventsFilter{}},
			wantErr: fmt.Sprintf("filters must contain 1 to %d filters", QueryEventsDefaultMaxFilters),
		},
		{
			name:    "too many filters",
			request: QueryEventsRequest{MinLedger: 1, Filters: typeOnlyFilters(int(QueryEventsDefaultMaxFilters) + 1)},
			wantErr: fmt.Sprintf("filters must contain 1 to %d filters", QueryEventsDefaultMaxFilters),
		},
		{
			name:    "exactly max filters accepted",
			request: QueryEventsRequest{MinLedger: 1, Filters: typeOnlyFilters(int(QueryEventsDefaultMaxFilters))},
		},
		{
			name:    "system event type accepted",
			request: QueryEventsRequest{MinLedger: 1, Filters: []QueryEventsFilter{{EventType: EventTypeSystem}}},
		},
		{
			name:    "filter with no fields",
			request: QueryEventsRequest{MinLedger: 1, Filters: []QueryEventsFilter{{}}},
			wantErr: errEmptyFilter,
		},
		{
			name: "invalid event type",
			request: QueryEventsRequest{
				MinLedger: 1, Filters: []QueryEventsFilter{{EventType: "diagnostic"}},
			},
			wantErr: `filters[0]: type must be "contract" or "system"`,
		},
		{
			name: "invalid contract ID",
			request: QueryEventsRequest{
				MinLedger: 1, Filters: []QueryEventsFilter{{ContractID: "not-a-contract"}},
			},
			wantErr: "filters[0]: contractId is invalid",
		},
		{
			// checksum-valid strkey with a payload that is not 32 bytes
			name: "contract ID with short payload",
			request: QueryEventsRequest{
				MinLedger: 1, Filters: []QueryEventsFilter{{ContractID: shortContractID(t)}},
			},
			wantErr: "filters[0]: contractId is invalid",
		},
		{
			name: "topic0 position validated",
			request: QueryEventsRequest{
				MinLedger: 1,
				Filters:   []QueryEventsFilter{{Topic0: json.RawMessage(`"!!!"`)}},
			},
			wantErr: "filters[0].topic0 is not valid base64-encoded XDR",
		},
		{
			name: "topic not base64 XDR under default input format",
			request: QueryEventsRequest{
				MinLedger: 1,
				Filters:   []QueryEventsFilter{{Topic1: json.RawMessage(`"!!!"`)}},
			},
			wantErr: "filters[0].topic1 is not valid base64-encoded XDR",
		},
		{
			name: "topic not a JSON string under base64 input format",
			request: QueryEventsRequest{
				MinLedger:      1,
				XDRInputFormat: FormatBase64,
				Filters:        []QueryEventsFilter{{Topic2: json.RawMessage(`{"symbol":"transfer"}`)}},
			},
			wantErr: "filters[0].topic2 is not valid base64-encoded XDR",
		},
		{
			name: "JSON topic under json input format",
			request: QueryEventsRequest{
				MinLedger:      1,
				XDRInputFormat: FormatJSON,
				Filters:        []QueryEventsFilter{{Topic0: json.RawMessage(`{"symbol":"transfer"}`)}},
			},
		},
		{
			name: "topic3 position validated",
			request: QueryEventsRequest{
				MinLedger: 1,
				Filters:   []QueryEventsFilter{{Topic3: json.RawMessage(`"!!!"`)}},
			},
			wantErr: "filters[0].topic3 is not valid base64-encoded XDR",
		},
		{
			name: "error names the failing filter index",
			request: QueryEventsRequest{
				MinLedger: 1,
				Filters:   []QueryEventsFilter{{EventType: EventTypeContract}, {}},
			},
			wantErr: "filters[1]: filter must specify type, contractId, or at least one topic position",
		},
		{
			name: "contractId still validated under json input format",
			request: QueryEventsRequest{
				MinLedger:      1,
				XDRInputFormat: FormatJSON,
				Filters:        []QueryEventsFilter{{ContractID: "not-a-contract"}},
			},
			wantErr: "filters[0]: contractId is invalid",
		},
	})
}

// Explicit JSON null topics mean the same as omitted (matches any value).
func TestQueryEventsRequestValidNullTopics(t *testing.T) {
	runRequestValidCases(t, []requestValidCase{
		{
			name: "null topic counts as omitted: empty filter rejected",
			request: QueryEventsRequest{
				MinLedger: 1,
				Filters:   []QueryEventsFilter{{Topic0: json.RawMessage(`null`)}},
			},
			wantErr: errEmptyFilter,
		},
		{
			name: "null topic counts as omitted under json input format too",
			request: QueryEventsRequest{
				MinLedger:      1,
				XDRInputFormat: FormatJSON,
				Filters:        []QueryEventsFilter{{Topic1: json.RawMessage(`null`)}},
			},
			wantErr: errEmptyFilter,
		},
		{
			name: "null topic alongside a real constraint is legal",
			request: QueryEventsRequest{
				MinLedger: 1,
				Filters: []QueryEventsFilter{{
					EventType: EventTypeContract, Topic0: json.RawMessage(`null`),
				}},
			},
		},
	})
}

func TestQueryEventsRequestJSONRoundTrip(t *testing.T) {
	contractID := testContractID(t)
	topic := testTopicB64(t)

	t.Run("range query", func(t *testing.T) {
		req := QueryEventsRequest{
			MinLedger: 1, MaxLedger: 2, Order: OrderDescending,
			Filters: []QueryEventsFilter{
				{ContractID: contractID, EventType: EventTypeSystem, Topic0: topic, Topic3: topic},
			},
			XDRInputFormat: FormatBase64, Limit: limitPtr(5), Format: FormatJSON,
		}
		raw, err := json.Marshal(req)
		require.NoError(t, err)
		var got QueryEventsRequest
		require.NoError(t, json.Unmarshal(raw, &got))
		assert.Equal(t, req, got)
	})

	// The tags are the public wire contract; round-trips through the same
	// struct pass a tag rename in lockstep, the literal does not.
	t.Run("golden request keys", func(t *testing.T) {
		raw, err := json.Marshal(QueryEventsRequest{
			MinLedger: 1, MaxLedger: 2, Order: OrderAscending,
			Filters: []QueryEventsFilter{
				{ContractID: contractID, EventType: EventTypeContract,
					Topic0: topic, Topic1: topic, Topic2: topic, Topic3: topic},
			},
			XDRInputFormat: FormatBase64, Limit: limitPtr(5), Format: FormatJSON,
		})
		require.NoError(t, err)
		golden := `{"minLedger":1,"maxLedger":2,"order":"asc","filters":[` +
			`{"contractId":` + string(mustJSON(t, contractID)) +
			`,"type":"contract","topic0":` + string(topic) + `,"topic1":` + string(topic) +
			`,"topic2":` + string(topic) + `,"topic3":` + string(topic) + `}],` +
			`"xdrInputFormat":"base64","limit":5,"xdrFormat":"json"}`
		assert.Equal(t, golden, string(raw))
	})

	t.Run("cursor query omits range keys", func(t *testing.T) {
		raw, err := json.Marshal(QueryEventsRequest{Cursor: "opaque", Limit: limitPtr(10)})
		require.NoError(t, err)
		assert.JSONEq(t, `{"cursor":"opaque","limit":10}`, string(raw))
	})

	// omitzero, not omitempty: a nil list is omitted, an empty list reaches
	// the server and gets its "1 to N filters" error instead of matching all.
	t.Run("empty filters are sent", func(t *testing.T) {
		raw, err := json.Marshal(QueryEventsRequest{MinLedger: 1, Filters: []QueryEventsFilter{}})
		require.NoError(t, err)
		assert.JSONEq(t, `{"minLedger":1,"filters":[]}`, string(raw))
	})

	t.Run("unset topics are omitted", func(t *testing.T) {
		raw, err := json.Marshal(QueryEventsFilter{Topic1: topic})
		require.NoError(t, err)
		var keys map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &keys))
		assert.Equal(t, []string{"topic1"}, slices.Collect(maps.Keys(keys)))
	})
}

func TestQueryEventsResponseJSON(t *testing.T) {
	t.Run("cursor omitted when empty", func(t *testing.T) {
		raw, err := json.Marshal(QueryEventsResponse{
			Events: []EventInfo{}, ScanStatus: ScanStatusComplete,
			ScannedLedger: 5, OldestLedger: 1, LatestLedger: 9,
		})
		require.NoError(t, err)
		assert.NotContains(t, string(raw), `"cursor"`)
		assert.Contains(t, string(raw), `"scanStatus":"COMPLETE"`)
	})

	t.Run("round trip with cursor", func(t *testing.T) {
		resp := QueryEventsResponse{
			Events: []EventInfo{{
				EventType: EventTypeContract, Ledger: 3, LedgerClosedAt: "2026-01-01T00:00:00Z",
				ContractID: "C...", ID: "0000000000000003-0000000001",
				OpIndex: 1, TxIndex: 2, TransactionHash: "ab",
				TopicXDR: []string{testTopicXDR}, ValueXDR: testTopicXDR,
			}},
			Cursor: "opaque", ScanStatus: ScanStatusHasMore,
			ScannedLedger: 3, OldestLedger: 1, LatestLedger: 9,
		}
		raw, err := json.Marshal(resp)
		require.NoError(t, err)
		var got QueryEventsResponse
		require.NoError(t, json.Unmarshal(raw, &got))
		assert.Equal(t, resp, got)
	})

	t.Run("golden response and event keys", func(t *testing.T) {
		raw, err := json.Marshal(QueryEventsResponse{
			Events: []EventInfo{{
				EventType: EventTypeContract, Ledger: 3, LedgerClosedAt: "2026-01-01T00:00:00Z",
				ContractID: "C1", ID: "id1", OpIndex: 1, TxIndex: 2, TransactionHash: "ab",
				TopicXDR: []string{testTopicXDR}, ValueXDR: testTopicXDR,
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

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}
