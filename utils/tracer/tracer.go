package tracer

import (
	"context"
	"time"

	"github.com/stellar/go-stellar-sdk/support/errors"
	"github.com/stellar/go-stellar-sdk/support/log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace/noop"
)

// InitializeTracer sets up traceProvider and returns a function to shutdown traceprovider
func InitializeTracer(enabled bool, OtelEndpoint, ServiceName, ServiceVersion string) (func(), error) {
	if !enabled {
		log.Info("Tracing disabled - using no-op tracer")
		// Set no-op tracer provider
		otel.SetTracerProvider(noop.NewTracerProvider())
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())

		// Return a no-op shutdown function
		return func() {}, nil
	}

	log.Infof("Initializing tracer")
	headers := map[string]string{
		"content-type": "application/json",
	}

	exporter, err := otlptrace.New(
		context.Background(),
		otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint(OtelEndpoint),
			otlptracehttp.WithHeaders(headers),
			otlptracehttp.WithInsecure(),
		),
	)
	if err != nil {
		return nil, errors.Wrap(err, "Error while creating exporter")
	}

	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(ServiceName),
			semconv.ServiceVersion(ServiceVersion),
		),
	)

	if err != nil {
		return nil, errors.Wrap(err, "Error while creating resource")
	}

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set traceprovider for the otel.
	otel.SetTracerProvider(traceProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := traceProvider.Shutdown(ctx); err != nil {
			log.Error("Error shutting down tracer provider", err)
		}
	}, nil
}
