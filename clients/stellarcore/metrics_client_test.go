package stellarcore

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	proto "github.com/stellar/go-stellar-sdk/protocols/stellarcore"
	"github.com/stellar/go-stellar-sdk/support/http/httptest"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestClientWithMetricsSubmitTxLabels(t *testing.T) {
	source := xdr.MustMuxedAddress("GBXGQJWVLWOYHFLVTKWV5FGHA3LNYY2JQKM7OAJAUEQFU6LPCSEFVXON")
	v1 := &xdr.TransactionV1Envelope{Tx: xdr.Transaction{SourceAccount: source}}
	envelopes := []xdr.TransactionEnvelope{
		{Type: xdr.EnvelopeTypeEnvelopeTypeTxV0, V0: &xdr.TransactionV0Envelope{}},
		{Type: xdr.EnvelopeTypeEnvelopeTypeTx, V1: v1},
		{Type: xdr.EnvelopeTypeEnvelopeTypeTxFeeBump, FeeBump: &xdr.FeeBumpTransactionEnvelope{Tx: xdr.FeeBumpTransaction{
			FeeSource: source,
			InnerTx:   xdr.FeeBumpTransactionInnerTx{Type: xdr.EnvelopeTypeEnvelopeTypeTx, V1: v1},
		}}},
	}

	hmock := httptest.NewClient()
	registry := prometheus.NewRegistry()
	client := NewClientWithMetrics(Client{HTTP: hmock, URL: "http://localhost:11626"}, registry, "txsub")
	submit := func(rawTx string, response proto.TXResponse) {
		hmock.On("GET", "http://localhost:11626/tx?"+url.Values{"blob": {rawTx}}.Encode()).
			ReturnJSON(http.StatusOK, response)
		_, err := client.SubmitTx(context.Background(), rawTx)
		require.NoError(t, err, rawTx)
	}
	for _, envelope := range envelopes {
		rawTx, err := xdr.MarshalBase64(envelope)
		require.NoError(t, err)
		submit(rawTx, proto.TXResponse{Status: proto.TXStatusPending})
	}
	// Only the envelope type is read, so a V1 envelope with a malformed body
	// is submitted, and stellar-core answers with an exception.
	submit(base64.StdEncoding.EncodeToString([]byte{0, 0, 0, 2, 1}),
		proto.TXResponse{Exception: "insufficient buffer space in xdr_generic_get"})

	// Input that is not a transaction envelope is rejected before submission,
	// so it records no metric.
	for _, rawTx := range []string{
		"not base64",
		base64.StdEncoding.EncodeToString([]byte{0, 0}),       // shorter than the envelope type
		base64.StdEncoding.EncodeToString([]byte{0, 0, 0, 1}), // ENVELOPE_TYPE_SCP
	} {
		_, err := client.SubmitTx(context.Background(), rawTx)
		require.Error(t, err, rawTx)
	}

	families, err := registry.Gather()
	require.NoError(t, err)
	require.Len(t, families, 1)
	require.Equal(t, "horizon_txsub_submission_duration_seconds", families[0].GetName())
	series := map[string]uint64{}
	for _, metric := range families[0].GetMetric() {
		labels := map[string]string{}
		for _, pair := range metric.GetLabel() {
			labels[pair.GetName()] = pair.GetValue()
		}
		require.Len(t, labels, 2)
		series[labels["envelope_type"]+" "+labels["status"]] += metric.GetSummary().GetSampleCount()
	}
	require.Equal(t, map[string]uint64{
		"v0 PENDING":       1,
		"v1 PENDING":       1,
		"fee_bump PENDING": 1,
		"v1 exception":     1,
	}, series)
}
