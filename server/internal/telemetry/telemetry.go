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
	"net/http"
	"net/url"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const defaultServiceName = "pvmss"

// Config carries the build identity stamped on every span's resource, and
// whether the Prometheus pull endpoint is wanted.
type Config struct {
	Version string
	Commit  string
	// Metrics turns on the Prometheus reader; the caller serves its handler on
	// PVMSS_METRICS_PORT. OTLP metric push follows the endpoint rule instead.
	Metrics bool
}

// Result is what Setup hands back. Shutdown is always non-nil and safe to call;
// MetricsHandler is non-nil only when Config.Metrics was set.
type Result struct {
	Shutdown       func(context.Context) error
	MetricsHandler http.Handler
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

// Setup installs the W3C propagator, then builds only what is configured:
// OTLP/HTTP trace and metric export when Enabled, a Prometheus reader when
// cfg.Metrics. With neither, it installs nothing and the global tracer and
// meter stay the no-op defaults. Shutdown flushes whatever was started.
func Setup(ctx context.Context, cfg Config, getenv func(string) string) (Result, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	res := Result{Shutdown: func(context.Context) error { return nil }}
	export := Enabled(getenv)

	if !export && !cfg.Metrics {
		return res, nil
	}

	resource, err := newResource(ctx, cfg)
	if err != nil {
		return res, err
	}

	var shutdowns []func(context.Context) error

	if export {
		tp, err := newTracerProvider(ctx, resource)
		if err != nil {
			return res, err
		}

		otel.SetTracerProvider(tp)

		shutdowns = append(shutdowns, tp.Shutdown)
	}

	mp, handler, err := newMeterProvider(ctx, resource, export, cfg.Metrics)
	if err != nil {
		return res, errors.Join(err, shutdownAll(ctx, shutdowns))
	}

	otel.SetMeterProvider(mp)

	shutdowns = append(shutdowns, mp.Shutdown)

	// Runtime metrics register on the global provider just installed.
	if err := runtime.Start(runtime.WithMeterProvider(mp)); err != nil {
		return res, errors.Join(errors.New("start go runtime metrics failed"), shutdownAll(ctx, shutdowns))
	}

	res.MetricsHandler = handler
	res.Shutdown = func(ctx context.Context) error { return shutdownAll(ctx, shutdowns) }

	return res, nil
}

func shutdownAll(ctx context.Context, fns []func(context.Context) error) error {
	var errs []error

	for _, fn := range fns {
		errs = append(errs, fn(ctx))
	}

	return errors.Join(errs...)
}

func newResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
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
		return nil, errors.New("build otel resource failed")
	}

	return res, nil
}

func newTracerProvider(ctx context.Context, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	exp, err := otlptracehttp.New(ctx) // reads OTEL_EXPORTER_OTLP_* itself
	if err != nil {
		// The underlying error can quote the endpoint URL, which may embed
		// credentials, so it is deliberately not propagated.
		return nil, errors.New("create otlp trace exporter failed: check the OTEL_EXPORTER_OTLP_* settings")
	}

	return sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp), sdktrace.WithResource(res)), nil
}

// newMeterProvider builds one provider with a Prometheus pull reader (own
// registry, so nothing leaks into the process-global default) and/or an OTLP
// periodic reader, plus views that enforce the label allowlist.
func newMeterProvider(ctx context.Context, res *resource.Resource, push, pull bool) (*sdkmetric.MeterProvider, http.Handler, error) {
	opts := []sdkmetric.Option{
		sdkmetric.WithResource(res),
		allowlistView("pvmss.*"),
		allowlistView("http.server.*"),
	}

	var handler http.Handler

	if pull {
		reg := prom.NewRegistry()

		reader, err := promexporter.New(promexporter.WithRegisterer(reg))
		if err != nil {
			return nil, nil, errors.New("create prometheus exporter failed")
		}

		opts = append(opts, sdkmetric.WithReader(reader))
		handler = promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	}

	if push {
		exp, err := otlpmetrichttp.New(ctx) // reads OTEL_EXPORTER_OTLP_* itself
		if err != nil {
			return nil, nil, errors.New("create otlp metric exporter failed: check the OTEL_EXPORTER_OTLP_* settings")
		}

		opts = append(opts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)))
	}

	return sdkmetric.NewMeterProvider(opts...), handler, nil
}

// allowlistView drops every attribute outside allowedMetricKeys from the
// instruments matching name (a "*" wildcard pattern).
func allowlistView(name string) sdkmetric.Option {
	return sdkmetric.WithView(sdkmetric.NewView(
		sdkmetric.Instrument{Name: name},
		sdkmetric.Stream{AttributeFilter: attribute.NewAllowKeysFilter(allowedMetricKeys...)},
	))
}
