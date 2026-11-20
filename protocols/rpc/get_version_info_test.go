package protocol

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A response without methods, limits or retention encodes to the five
// original fields only, so servers that do not report them keep their JSON.
func TestGetVersionInfoResponseOmitsUnreportedFields(t *testing.T) {
	out, err := json.Marshal(GetVersionInfoResponse{Version: "v1", ProtocolVersion: 23})
	require.NoError(t, err)
	assert.JSONEq(t, `{"version":"v1","commitHash":"","buildTimestamp":"",`+
		`"captiveCoreVersion":"","protocolVersion":23}`, string(out))

	var decoded GetVersionInfoResponse
	require.NoError(t, json.Unmarshal(out, &decoded))
	assert.Nil(t, decoded.Methods)
	assert.Nil(t, decoded.Limits)
	assert.Nil(t, decoded.Retention)
}

// A response that reports methods and limits decodes into caps that the
// request validators enforce.
func TestGetVersionInfoResponseDecodesMethodsAndLimits(t *testing.T) {
	var resp GetVersionInfoResponse
	require.NoError(t, json.Unmarshal([]byte(`{
		"version": "v2", "commitHash": "", "buildTimestamp": "",
		"captiveCoreVersion": "", "protocolVersion": 23,
		"methods": ["getLedgers", "getVersionInfo", "queryEvents"],
		"limits": {
			"maxBatchSize": 10,
			"getEvents": {"maxLimit": 10000},
			"getLedgers": {"maxLimit": 20},
			"getTransactions": {"maxLimit": 200},
			"queryEvents": {"termBudget": 7, "maxFilters": 2, "maxLimit": 500}
		}
	}`), &resp))

	assert.Equal(t, []string{GetLedgersMethodName, GetVersionInfoMethodName, QueryEventsMethodName}, resp.Methods)
	assert.Equal(t, &VersionInfoLimits{
		MaxBatchSize:    10,
		GetEvents:       &PageLimits{MaxLimit: 10000},
		GetLedgers:      &PageLimits{MaxLimit: 20},
		GetTransactions: &PageLimits{MaxLimit: 200},
		QueryEvents:     &QueryEventsLimits{TermBudget: 7, MaxFilters: 2, MaxLimit: 500},
	}, resp.Limits)

	ledgers := GetLedgersRequest{StartLedger: 10, Pagination: &LedgerPaginationOptions{Limit: 21}}
	assert.EqualError(t,
		ledgers.Validate(resp.Limits.GetLedgers.MaxLimit, LedgerSeqRange{FirstLedger: 1, LastLedger: 100}),
		"limit must not exceed 20")
	queryEvents := resp.Limits.QueryEvents
	assert.EqualError(t,
		(&QueryEventsRequest{MinLedger: 1, Filters: typeOnlyFilters(3)}).Valid(queryEvents.MaxFilters, queryEvents.MaxLimit),
		"filters must contain 1 to 2 filters")
	assert.EqualError(t,
		(&QueryEventsRequest{MinLedger: 1, Limit: limitPtr(501)}).Valid(queryEvents.MaxFilters, queryEvents.MaxLimit),
		"limit must be between 1 and 500")
}

// Limits with only some methods leave the others out of the JSON.
func TestVersionInfoLimitsOmitsUnreportedMethods(t *testing.T) {
	out, err := json.Marshal(VersionInfoLimits{GetLedgers: &PageLimits{MaxLimit: 20}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"getLedgers":{"maxLimit":20}}`, string(out))
}

// Each retention kind encodes only the field that belongs to it.
func TestRetentionEncodesOnlyItsKindField(t *testing.T) {
	for _, tc := range []struct {
		retention Retention
		want      string
	}{
		{Retention{Keeps: RetentionAll, FromLedger: 2}, `{"keeps":"all","fromLedger":2}`},
		{Retention{Keeps: RetentionLatest, Ledgers: 130000}, `{"keeps":"latest","ledgers":130000}`},
	} {
		out, err := json.Marshal(GetVersionInfoResponse{Retention: &tc.retention})
		require.NoError(t, err)
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(out, &fields))
		assert.JSONEq(t, tc.want, string(fields["retention"]))

		var decoded GetVersionInfoResponse
		require.NoError(t, json.Unmarshal(out, &decoded))
		assert.Equal(t, &tc.retention, decoded.Retention)
	}
}
