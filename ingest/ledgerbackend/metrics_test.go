package ledgerbackend

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// TestMetricsGetLedger verifies that GetLedger on the metrics-decorated backend
// records into the ledgerFetchDurationSummary — once per fetch.
func TestMetricsGetLedger(t *testing.T) {
	ctx := context.Background()
	mock := &MockDatabaseBackend{}
	mock.On("GetLedger", ctx, uint32(6)).Return(xdr.LedgerCloseMeta{}, nil).Twice()
	defer mock.AssertExpectations(t)

	registry := prometheus.NewRegistry()
	backend := WithMetrics(mock, registry, "test")

	_, err := backend.GetLedger(ctx, 6)
	require.NoError(t, err)
	_, err = backend.GetLedger(ctx, 6)
	require.NoError(t, err)

	metrics, err := registry.Gather()
	require.NoError(t, err)

	var summary *io_prometheus_client.Summary
	for _, mf := range metrics {
		if mf.GetName() == "test_ingest_ledger_fetch_duration_seconds" {
			require.Len(t, mf.Metric, 1)
			summary = mf.Metric[0].Summary
			break
		}
	}
	require.NotNil(t, summary, "ledger_fetch_duration_seconds summary not found")
	require.Equal(t, uint64(2), summary.GetSampleCount(),
		"each GetLedger should record into the summary")
}
