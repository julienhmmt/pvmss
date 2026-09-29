// Package telemetry wires OpenTelemetry tracing and log correlation.
//
// Tracing is opt-in through the standard OTEL_* environment: without an OTLP
// endpoint (or with OTEL_SDK_DISABLED=true) nothing is exported, no goroutine
// starts and the global tracer stays the no-op default. The SDK reads the
// OTEL_* variables itself; this package never parses or logs them, because
// OTEL_EXPORTER_OTLP_HEADERS may hold credentials.
package telemetry

import (
	"context"
	"errors"
	"net/url"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const defaultServiceName = "pvmss"

// Config carries the build identity stamped on every span's resource.
type Config struct {
	Version string
	Commit  string
}

// Enabled reports whether tracing export is configured: an OTLP endpoint is set
// and the SDK is not disabled. getenv is os.Getenv in production.
func Enabled(getenv func(string) string) bool {
	if getenv("OTEL_SDK_DISABLED") == "true" {
		return false
	}

	return getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != "" || getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != ""
}

// EndpointHost returns only host[:port] of the configured endpoint, for the
// startup banner. Userinfo, path and query are dropped so credentials embedded
// in the URL can never reach a log line. It is empty when tracing is disabled.
func EndpointHost(getenv func(string) string) string {
	if !Enabled(getenv) {
		return ""
	}

	raw := getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
	if raw == "" {
		raw = getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "invalid"
	}

	return u.Host
}

// Setup installs the W3C propagator and, when Enabled, an OTLP/HTTP trace
// exporter behind a batching provider. The returned shutdown flushes pending
// spans; it is always safe to call, and a no-op when tracing is disabled.
func Setup(ctx context.Context, cfg Config, getenv func(string) string) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	noop := func(context.Context) error { return nil }

	if !Enabled(getenv) {
		return noop, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", defaultServiceName),
			attribute.String("service.version", cfg.Version),
			attribute.String("vcs.revision", cfg.Commit),
		),
		resource.WithFromEnv(), // OTEL_SERVICE_NAME / OTEL_RESOURCE_ATTRIBUTES override the defaults above
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return noop, errors.New("build otel resource failed")
	}

	exp, err := otlptracehttp.New(ctx) // reads OTEL_EXPORTER_OTLP_* itself
	if err != nil {
		// The underlying error can quote the endpoint URL, which may embed
		// credentials, so it is deliberately not propagated.
		return noop, errors.New("create otlp trace exporter failed: check the OTEL_EXPORTER_OTLP_* settings")
	}

	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}
