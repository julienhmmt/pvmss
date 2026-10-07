package httpapi

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// withTracing opens a server span per API request and joins an inbound W3C
// trace. otelhttp renames the span after routing from r.Pattern (the route
// template, never the concrete URL). Health checks and SPA assets are not
// traced: they are noise and would dominate span volume. Place it inside
// withRequestID and outside withAccessLog, so the access-log line is written
// inside the span and carries its traceId. With no provider configured the
// global no-op tracer makes this near-free.
func withTracing(tp trace.TracerProvider, next http.Handler) http.Handler {
	opts := []otelhttp.Option{
		// Explicit rather than the global propagator, so joining an inbound
		// trace never depends on telemetry.Setup having run.
		otelhttp.WithPropagators(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})),
		otelhttp.WithFilter(func(r *http.Request) bool { return isAPIPath(r.URL.Path) }),
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			if r.Pattern != "" {
				return r.Pattern
			}

			return operation
		}),
	}
	if tp != nil {
		opts = append(opts, otelhttp.WithTracerProvider(tp))
	}

	return otelhttp.NewHandler(next, "http.request", opts...)
}
