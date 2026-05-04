package ledgerbackend

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// TestMetricsGetLedgerRaw verifies that GetLedgerRaw on the metrics-decorated
// backend records the same ledgerFetchDurationSummary as GetLedger (both are
// "fetches" from the backend's perspective).
func TestMetricsGetLedgerRaw(t *testing.T) {
	ctx := context.Background()
	mock := &MockDatabaseBackend{}
	mock.On("GetLedgerRaw", ctx, uint32(5)).Return([]byte{0x01, 0x02, 0x03}, nil).Once()
	mock.On("GetLedger", ctx, uint32(6)).Return(xdr.LedgerCloseMeta{}, nil).Once()
	defer mock.AssertExpectations(t)

	registry := prometheus.NewRegistry()
	backend := WithMetrics(mock, registry, "test")

	_, err := backend.GetLedgerRaw(ctx, 5)
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
		"GetLedger and GetLedgerRaw should both record into the same summary")
}
