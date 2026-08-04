package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// The getEvents v2 wire types, transcribed from the accepted proposal
// (https://github.com/orgs/stellar/discussions/1872).

const (
	// GetEventsV2MethodName is the working method name.
	GetEventsV2MethodName = "getEventsV2"

	// MaxLimitV2 caps the limit parameter (max events per response). The
	// proposal fixes it, unlike the provider-configurable defaults below.
	// (The server's default when limit is unset, 100, is a separate number
	// this package never applies.)
	MaxLimitV2 = 1000

	// DefaultMaxFiltersV2 is the default cap on the filters list per
	// request; providers may configure their own, passed to Valid as
	// maxFilters.
	DefaultMaxFiltersV2 = 256

	// DefaultTermBudgetV2 is the default cap on distinct index terms per
	// query. Counting terms needs the canonical form of json-format topics,
	// which only the server's XDR-JSON converter can produce, hence the
	// server enforces it (reporting via InvalidParamsErrorData's TermsUsed
	// and TermBudget).
	DefaultTermBudgetV2 = 15
)

const (
	OrderAscending  = "asc"
	OrderDescending = "desc"
)

// Scan statuses reported by GetEventsV2Response.ScanStatus.
const (
	// ScanStatusHasMore: more to scan on this node right now.
	ScanStatusHasMore = "HAS_MORE"
	// ScanStatusWaitingForLedgers: the query needs ledgers this node does not
	// have yet.
	ScanStatusWaitingForLedgers = "WAITING_FOR_LEDGERS"
	// ScanStatusOldestReached: the query wants history older than this node
	// holds.
	ScanStatusOldestReached = "OLDEST_REACHED"
	// ScanStatusComplete: the query is finished and will never return more;
	// the response carries no cursor.
	ScanStatusComplete = "COMPLETE"
)

// Machine-readable error reasons carried in error.data.reason.
const (
	ErrorReasonInvalidParams    = "invalid_params"
	ErrorReasonLedgerOutOfRange = "ledger_out_of_range"
	ErrorReasonCursorMalformed  = "cursor_malformed"
)

// InvalidParamsErrorData is error.data for invalid_params. TermsUsed and
// TermBudget are present only when the query exceeds the term budget.
type InvalidParamsErrorData struct {
	Reason     string `json:"reason"`
	TermsUsed  uint32 `json:"termsUsed,omitempty"`
	TermBudget uint32 `json:"termBudget,omitempty"`
}

// LedgerOutOfRangeErrorData is error.data for ledger_out_of_range.
type LedgerOutOfRangeErrorData struct {
	Reason        string `json:"reason"`
	MissingLedger uint32 `json:"missingLedger"`
	OldestLedger  uint32 `json:"oldestLedger"`
	LatestLedger  uint32 `json:"latestLedger"`
}

// CursorMalformedErrorData is error.data for cursor_malformed.
type CursorMalformedErrorData struct {
	Reason       string `json:"reason"`
	OldestLedger uint32 `json:"oldestLedger"`
	LatestLedger uint32 `json:"latestLedger"`
}

// EventFilterV2 is one filter in a range query. Fields within a filter are
// AND-ed; filters in the list are OR-ed. At least one field must be set.
// Topic values are ScVals encoded per the request's xdrInputFormat (base64
// XDR strings, or JSON objects); an omitted topic position matches any
// value.
type EventFilterV2 struct {
	ContractID string          `json:"contractId,omitempty"`
	EventType  string          `json:"type,omitempty"`
	Topic0     json.RawMessage `json:"topic0,omitempty"`
	Topic1     json.RawMessage `json:"topic1,omitempty"`
	Topic2     json.RawMessage `json:"topic2,omitempty"`
	Topic3     json.RawMessage `json:"topic3,omitempty"`
}

// Topics returns the positional topic values, index i holding topicI. An
// explicit JSON null is returned as nil: null means the same as omitted
// (matches any value), and RawMessage stores it as the non-nil bytes "null".
func (f *EventFilterV2) Topics() [MaxTopicCount]json.RawMessage {
	topics := [MaxTopicCount]json.RawMessage{f.Topic0, f.Topic1, f.Topic2, f.Topic3}
	for i, topic := range topics {
		if bytes.Equal(topic, jsonNull) {
			topics[i] = nil
		}
	}
	return topics
}

var jsonNull = []byte("null")

// GetEventsV2Request is the union of two request shapes: a
// range query (minLedger, maxLedger, order, filters, xdrInputFormat) or a
// cursor query (cursor). The two are mutually exclusive; limit and xdrFormat
// apply to both.
type GetEventsV2Request struct {
	MinLedger uint32 `json:"minLedger,omitempty"`
	MaxLedger uint32 `json:"maxLedger,omitempty"`
	Order     string `json:"order,omitempty"`
	// Filters: a JSON null decodes like an omitted member (match all
	// events), consistent with null topics.
	Filters        []EventFilterV2 `json:"filters,omitempty"`
	XDRInputFormat string          `json:"xdrInputFormat,omitempty"`
	Cursor         string          `json:"cursor,omitempty"`
	// Limit is nil when omitted; the server applies its default. An
	// explicit limit outside [1, MaxLimitV2] is rejected.
	Limit  *uint  `json:"limit,omitempty"`
	Format string `json:"xdrFormat,omitempty"`
}

// InvalidParamsError is a request validation failure: the proposal's
// invalid_params message plus the error.data payload to return with it. The
// server serializes Message and Data into the JSON-RPC error.
type InvalidParamsError struct {
	Message string
	Data    InvalidParamsErrorData
}

func (e *InvalidParamsError) Error() string { return e.Message }

func invalidParamsf(format string, args ...any) error {
	return &InvalidParamsError{
		Message: fmt.Sprintf(format, args...),
		Data:    InvalidParamsErrorData{Reason: ErrorReasonInvalidParams},
	}
}

// Valid checks the request against the proposal's parameter rules and
// returns the first violation as an *InvalidParamsError. maxFilters is the
// provider-configured cap on the filters list (DefaultMaxFiltersV2 unless
// the provider sets its own).
func (r *GetEventsV2Request) Valid(maxFilters uint) error {
	if err := r.validFormats(); err != nil {
		return err
	}
	if err := r.validShape(); err != nil {
		return err
	}
	if err := r.validLimit(); err != nil {
		return err
	}
	return r.validFilters(maxFilters)
}

func (r *GetEventsV2Request) validFormats() error {
	switch r.Format {
	case "", FormatBase64, FormatJSON:
	default:
		return invalidParamsf("xdrFormat must be %q or %q", FormatBase64, FormatJSON)
	}
	switch r.XDRInputFormat {
	case "", FormatBase64, FormatJSON:
	default:
		return invalidParamsf("xdrInputFormat must be %q or %q", FormatBase64, FormatJSON)
	}
	return nil
}

func (r *GetEventsV2Request) validShape() error {
	if r.Cursor != "" {
		if r.MinLedger != 0 || r.MaxLedger != 0 || r.Order != "" ||
			r.Filters != nil || r.XDRInputFormat != "" {
			return invalidParamsf(
				"cursor is mutually exclusive with minLedger, maxLedger, order, filters, xdrInputFormat")
		}
		return nil
	}
	switch r.Order {
	case "", OrderAscending, OrderDescending:
	default:
		return invalidParamsf("order must be %q or %q", OrderAscending, OrderDescending)
	}
	if r.Order != OrderDescending && r.MinLedger == 0 {
		return invalidParamsf("minLedger is required for ascending order")
	}
	if r.MinLedger != 0 && r.MaxLedger != 0 && r.MinLedger > r.MaxLedger {
		return invalidParamsf("minLedger must be <= maxLedger")
	}
	return nil
}

func (r *GetEventsV2Request) validLimit() error {
	if r.Limit != nil && (*r.Limit < 1 || *r.Limit > MaxLimitV2) {
		return invalidParamsf("limit must be between 1 and %d", MaxLimitV2)
	}
	return nil
}

func (r *GetEventsV2Request) validFilters(maxFilters uint) error {
	if r.Cursor != "" {
		return nil // mutual exclusion is validShape's finding
	}
	if r.Filters != nil && (len(r.Filters) == 0 || uint(len(r.Filters)) > maxFilters) {
		return invalidParamsf("filters must contain 1 to %d filters", maxFilters)
	}
	for i := range r.Filters {
		if err := r.Filters[i].valid(i, r.XDRInputFormat); err != nil {
			return err
		}
	}
	return nil
}

func (f *EventFilterV2) valid(index int, xdrInputFormat string) error {
	topics := f.Topics()
	hasTopic := false
	for _, topic := range topics {
		if topic != nil {
			hasTopic = true
			break
		}
	}
	if f.ContractID == "" && f.EventType == "" && !hasTopic {
		return invalidParamsf(
			"filters[%d]: filter must specify type, contractId, or at least one topic position", index)
	}
	switch f.EventType {
	case "", EventTypeContract, EventTypeSystem:
	default:
		return invalidParamsf("filters[%d]: type must be %q or %q",
			index, EventTypeContract, EventTypeSystem)
	}
	if f.ContractID != "" {
		if _, err := strkey.Decode(strkey.VersionByteContract, f.ContractID); err != nil {
			return invalidParamsf("filters[%d]: contractId is invalid", index)
		}
	}
	// The base64 form is decodable here; the JSON form needs the server's
	// XDR-JSON converter and is validated there. An invalid format is
	// validFormats's finding, so no topic error is derived from it.
	if xdrInputFormat != "" && xdrInputFormat != FormatBase64 {
		return nil
	}
	for pos, topic := range topics {
		if topic == nil {
			continue
		}
		var b64 string
		if err := json.Unmarshal(topic, &b64); err != nil {
			return invalidParamsf("filters[%d].topic%d is not valid base64-encoded XDR", index, pos)
		}
		var scVal xdr.ScVal
		if err := xdr.SafeUnmarshalBase64(b64, &scVal); err != nil {
			return invalidParamsf("filters[%d].topic%d is not valid base64-encoded XDR", index, pos)
		}
	}
	return nil
}

// EventInfoV2 is one event in a v2 response. Exactly one of TopicXDR and
// TopicJSON, and one of ValueXDR and ValueJSON, is present, per the
// request's xdrFormat.
type EventInfoV2 struct {
	EventType       string `json:"type"`
	Ledger          int32  `json:"ledger"`
	LedgerClosedAt  string `json:"ledgerClosedAt"`
	ContractID      string `json:"contractId"`
	ID              string `json:"id"`
	OpIndex         uint32 `json:"operationIndex"`
	TxIndex         uint32 `json:"transactionIndex"`
	TransactionHash string `json:"txHash"`

	// TopicXDR is a base64-encoded list of ScVals
	TopicXDR  []string          `json:"topic,omitempty"`
	TopicJSON []json.RawMessage `json:"topicJson,omitempty"`

	// ValueXDR is a base64-encoded ScVal
	ValueXDR  string          `json:"value,omitempty"`
	ValueJSON json.RawMessage `json:"valueJson,omitempty"`
}

// GetEventsV2Response is the v2 response. Cursor is present on every
// response except when ScanStatus is COMPLETE: an absent cursor means the
// query is finished.
type GetEventsV2Response struct {
	Events        []EventInfoV2 `json:"events"`
	Cursor        string        `json:"cursor,omitempty"`
	ScanStatus    string        `json:"scanStatus"`
	ScannedLedger uint32        `json:"scannedLedger"`
	OldestLedger  uint32        `json:"oldestLedger"`
	LatestLedger  uint32        `json:"latestLedger"`
}
