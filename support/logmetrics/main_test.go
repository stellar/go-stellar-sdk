package logmetrics

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/stellar/go-stellar-sdk/support/log"
)

func TestLogPackageMetrics(t *testing.T) {
	output := new(bytes.Buffer)
	l := log.New()
	m := New("horizon")

	l.DisableColors()
	l.SetLevel(logrus.DebugLevel)
	l.SetOutput(output)
	l.AddHook(m)

	for _, meter := range m {
		assert.Equal(t, float64(0), getMetricValue(meter).GetCounter().GetValue())
	}

	l.Debug("foo")
	l.Info("foo")
	l.Warn("foo")
	l.Error("foo")
	assert.Panics(t, func() {
		l.Panic("foo")
	})

	for level, meter := range m {
		levelString := level.String()
		if levelString == "warning" {
			levelString = "warn"
		}
		expectedDesc := fmt.Sprintf(
			"Desc{fqName: \"horizon_log_%s_total\", help: \"\", constLabels: {}, variableLabels: {}}",
			levelString,
		)
		assert.Equal(t, expectedDesc, meter.Desc().String())
		assert.Equal(t, float64(1), getMetricValue(meter).GetCounter().GetValue())
	}
}

func getMetricValue(metric prometheus.Metric) *dto.Metric {
	value := &dto.Metric{}
	err := metric.Write(value)
	if err != nil {
		panic(err)
	}
	return value
}
