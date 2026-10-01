package telemetry_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"pvmss/server/internal/telemetry"
	"strings"
	"sync"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func env(kv map[string]string) func(string) string {
	return func(k string) string { return kv[k] }
}

func TestEnabled(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"nothing set", map[string]string{}, false},
		{"generic endpoint", map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://c:4318"}, true},
		{"traces endpoint", map[string]string{"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "http://c:4318/v1/traces"}, true},
		{"disabled wins", map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://c:4318", "OTEL_SDK_DISABLED": "true"}, false},
		{"disabled false is ignored", map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://c:4318", "OTEL_SDK_DISABLED": "false"}, true},
	}
	for _, c := range cases {
		if got := telemetry.Enabled(env(c.env)); got != c.want {
			t.Errorf("%s: Enabled = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestEndpointHost_NeverLeaksCredentials(t *testing.T) {
	t.Parallel()

	got := telemetry.EndpointHost(env(map[string]string{
		"OTEL_EXPORTER_OTLP_ENDPOINT": "https://user:pw-SECRET@otel.example:4318/v1/traces?token=SECRET",
		"OTEL_EXPORTER_OTLP_HEADERS":  "Authorization=Bearer SECRET",
	}))
	if got != "otel.example:4318" {
		t.Fatalf("EndpointHost = %q", got)
	}

	if telemetry.EndpointHost(env(nil)) != "" {
		t.Fatal("no endpoint must give empty host")
	}
}

func TestSetup_DisabledIsNoop(t *testing.T) {
	t.Parallel()

	res, err := telemetry.Setup(context.Background(), telemetry.Config{Version: "v", Commit: "c"}, env(nil))
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	if err := res.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestLogHandler_AddsTraceAndSpanIDInsideSpan(t *testing.T) {
	t.Parallel()

	tp := trace.NewTracerProvider(trace.WithSpanProcessor(trace.NewSimpleSpanProcessor(tracetest.NewInMemoryExporter())))
	ctx, span := tp.Tracer("t").Start(context.Background(), "op")

	defer span.End()

	var buf bytes.Buffer

	logger := slog.New(telemetry.LogHandler(slog.NewJSONHandler(&buf, nil))).With("k", "v").WithGroup("g")
	logger.InfoContext(ctx, "inside")
	logger.InfoContext(context.Background(), "outside")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")

	// The ids are added by Handle after WithGroup, so they nest under "g";
	// only their presence matters here.
	if !strings.Contains(lines[0], span.SpanContext().TraceID().String()) || !strings.Contains(lines[0], span.SpanContext().SpanID().String()) {
		t.Fatalf("inside span, want traceId/spanId: %s", lines[0])
	}

	if strings.Contains(lines[1], "traceId") {
		t.Fatalf("outside a span there must be no traceId: %s", lines[1])
	}
}

// With an endpoint configured, spans reach the OTLP/HTTP collector on flush,
// carrying the configured service name. Serial: it sets env and the global
// provider.
//
//nolint:paralleltest // serial: sets process env and the global tracer provider
func TestSetup_ExportsToOTLPEndpoint(t *testing.T) {
	var (
		mu      sync.Mutex
		hit     bool
		gotBody []byte
		metrics bool
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		mu.Lock()
		switch r.URL.Path {
		case "/v1/traces":
			hit, gotBody = true, body
		case "/v1/metrics":
			metrics = true
		}
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", srv.URL)
	t.Setenv("OTEL_SERVICE_NAME", "pvmss-test")

	prevTP, prevMP := otel.GetTracerProvider(), otel.GetMeterProvider()
	t.Cleanup(func() { otel.SetTracerProvider(prevTP); otel.SetMeterProvider(prevMP) })

	res, err := telemetry.Setup(context.Background(), telemetry.Config{Version: "9.9.9", Commit: "abc"}, os.Getenv)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}

	_, span := otel.Tracer("t").Start(context.Background(), "probe")
	span.End()

	if err := res.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if !hit || !bytes.Contains(gotBody, []byte("pvmss-test")) || !bytes.Contains(gotBody, []byte("probe")) {
		t.Fatalf("collector did not receive the span (hit=%v, %d bytes)", hit, len(gotBody))
	}

	if !metrics {
		t.Fatal("collector did not receive the OTLP metrics push on flush")
	}
}
