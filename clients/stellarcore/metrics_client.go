package stellarcore

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	proto "github.com/stellar/go-stellar-sdk/protocols/stellarcore"
	"github.com/stellar/go-stellar-sdk/xdr"
)

var envelopeTypeToLabel = map[xdr.EnvelopeType]string{
	xdr.EnvelopeTypeEnvelopeTypeTxV0:      "v0",
	xdr.EnvelopeTypeEnvelopeTypeTx:        "v1",
	xdr.EnvelopeTypeEnvelopeTypeTxFeeBump: "fee_bump",
}

type ClientWithMetrics struct {
	coreClient Client

	// submissionDuration exposes timing metrics about the rate and latency of
	// submissions to stellar-core
	submissionDuration *prometheus.SummaryVec
}

func (c ClientWithMetrics) SubmitTx(ctx context.Context, rawTx string) (*proto.TXResponse, error) {
	var envelope xdr.TransactionEnvelope
	err := xdr.SafeUnmarshalBase64(rawTx, &envelope)
	if err != nil {
		return &proto.TXResponse{}, err
	}

	startTime := time.Now()
	response, err := c.coreClient.SubmitTransaction(ctx, rawTx)
	duration := time.Since(startTime).Seconds()

	label := prometheus.Labels{}
	if err != nil {
		label["status"] = "request_error"
	} else if response.IsException() {
		label["status"] = "exception"
	} else {
		label["status"] = response.Status
	}

	label["envelope_type"] = envelopeTypeToLabel[envelope.Type]
	c.submissionDuration.With(label).Observe(duration)

	return response, err
}

func NewClientWithMetrics(client Client, registry *prometheus.Registry, prometheusSubsystem string) ClientWithMetrics {
	submissionDuration := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:  "horizon",
		Subsystem:  prometheusSubsystem,
		Name:       "submission_duration_seconds",
		Help:       "submission durations to Stellar-Core, sliding window = 10m",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"status", "envelope_type"})

	registry.MustRegister(
		submissionDuration,
	)

	return ClientWithMetrics{
		coreClient:         client,
		submissionDuration: submissionDuration,
	}
}
