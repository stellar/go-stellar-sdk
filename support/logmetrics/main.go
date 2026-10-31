package logmetrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

// Metrics is a logrus hook-compliant struct that records metrics about logging
// when added to a logrus.Logger
type Metrics map[logrus.Level]prometheus.Counter

// New creates a new hook for recording metrics.
// New takes a namespace parameter which defines the namespace
// for the prometheus metrics.
func New(namespace string) Metrics {
	return Metrics{
		logrus.DebugLevel: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "log", Name: "debug_total",
		}),
		logrus.InfoLevel: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "log", Name: "info_total",
		}),
		logrus.WarnLevel: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "log", Name: "warn_total",
		}),
		logrus.ErrorLevel: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "log", Name: "error_total",
		}),
		logrus.PanicLevel: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: "log", Name: "panic_total",
		}),
	}
}

// Fire is triggered by logrus, in response to a logging event
func (m Metrics) Fire(e *logrus.Entry) error {
	counter, ok := m[e.Level]
	if ok {
		counter.Inc()
		return nil
	}
	return fmt.Errorf("level %v not supported", e.Level)
}

// Levels returns the logging levels that will trigger this hook to run.  In
// this case, all of them.
func (m Metrics) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.DebugLevel,
		logrus.InfoLevel,
		logrus.WarnLevel,
		logrus.ErrorLevel,
		logrus.PanicLevel,
	}
}
