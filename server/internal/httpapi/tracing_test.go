package httpapi_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/telemetry"
	"strings"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func tracedRouter(t *testing.T) (http.Handler, *tracetest.InMemoryExporter, *bytes.Buffer) {
	t.Helper()

	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exp)))

	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	var buf bytes.Buffer

	logger := slog.New(telemetry.LogHandler(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	health := httpapi.NewHealth(fakeHealthPinger{}, logger, nil, 60*time.Second)
	vms := httpapi.NewVMs(inventory.NewProjection(), newAuthHandler(t), 100, -1, logger)
	vmDetail := httpapi.NewVMDetail(inventory.NewProjection(), newAuthHandler(t), cluster.Fake{}, nil, nil, logger)

	router := httpapi.NewRouter(httpapi.RouterConfig{
		Health: health, VMs: vms, VMDetail: vmDetail, Auth: newAuthHandler(t), Log: logger, TracerProvider: tp,
	})

	return router, exp, &buf
}

func serve(router http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), method, path, nil))

	return rec
}

//nolint:paralleltest // serial: shared auth fixtures
func TestTracing_OneServerSpanPerAPIRequestNamedByRoute(t *testing.T) {
	router, exp, buf := tracedRouter(t)

	serve(router, http.MethodGet, "/api/v1/vms?token=SEKRET")

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("want 1 span, got %d", len(spans))
	}

	if spans[0].Name != "GET /api/v1/vms" {
		t.Errorf("span name = %q, want the route pattern", spans[0].Name)
	}

	// The access log line is written inside the span, so it carries the same ids.
	if !strings.Contains(buf.String(), `"traceId":"`+spans[0].SpanContext.TraceID().String()+`"`) {
		t.Errorf("access log missing matching traceId: %s", buf.String())
	}
}

//nolint:paralleltest // serial: shared auth fixtures
func TestTracing_HealthAndStaticAreNotTraced(t *testing.T) {
	router, exp, _ := tracedRouter(t)

	serve(router, http.MethodGet, "/health")
	serve(router, http.MethodGet, "/some/spa/route")

	if n := len(exp.GetSpans()); n != 0 {
		t.Fatalf("want 0 spans for /health and non-API paths, got %d", n)
	}
}

//nolint:paralleltest // serial: shared auth fixtures
func TestTracing_HonoursInboundTraceparent(t *testing.T) {
	router, exp, _ := tracedRouter(t)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/vms", nil)
	req.Header.Set("Traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
	router.ServeHTTP(httptest.NewRecorder(), req)

	spans := exp.GetSpans()
	if len(spans) != 1 || spans[0].SpanContext.TraceID().String() != "0af7651916cd43dd8448eb211c80319c" {
		t.Fatalf("server span must join the inbound trace, got %v", spans)
	}
}
