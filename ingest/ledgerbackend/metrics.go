package ledgerbackend

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// WithMetrics decorates the given LedgerBackend with metrics
func WithMetrics(base LedgerBackend, registry *prometheus.Registry, namespace string) LedgerBackend {
	if captiveCoreBackend, ok := base.(*CaptiveStellarCore); ok {
		captiveCoreBackend.registerMetrics(registry, namespace)
	}
	summary := prometheus.NewSummary(
		prometheus.SummaryOpts{
			Namespace: namespace, Subsystem: "ingest", Name: "ledger_fetch_duration_seconds",
			Help:       "duration of fetching ledgers from ledger backend, sliding window = 10m",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
	)
	registry.MustRegister(summary)
	return metricsLedgerBackend{
		LedgerBackend:              base,
		ledgerFetchDurationSummary: summary,
	}
}

type metricsLedgerBackend struct {
	LedgerBackend
	ledgerFetchDurationSummary prometheus.Summary
}

func (m metricsLedgerBackend) GetLedger(ctx context.Context, sequence uint32) (xdr.LedgerCloseMeta, error) {
	startTime := time.Now()
	lcm, err := m.LedgerBackend.GetLedger(ctx, sequence)
	if err != nil {
		return xdr.LedgerCloseMeta{}, err
	}
	m.ledgerFetchDurationSummary.Observe(time.Since(startTime).Seconds())
	return lcm, nil
}
