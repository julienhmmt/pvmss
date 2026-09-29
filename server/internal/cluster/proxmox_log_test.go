package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/logctx"
	"strings"
	"sync/atomic"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestTemplateProxmoxPath(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"/nodes":                                      "/nodes",
		"/nodes/pve1/qemu/101/status/current":         "/nodes/{node}/qemu/{vmid}/status/current",
		"/nodes/pve1/lxc/7/config":                    "/nodes/{node}/lxc/{vmid}/config",
		"/nodes/pve1/tasks/UPID:pve1:0001:x:y/status": "/nodes/{node}/tasks/{upid}/status",
		"/nodes/pve1/storage/local/content":           "/nodes/{node}/storage/{storage}/content",
		"/pools/alice-pool":                           "/pools/{pool}",
		"/access/users/alice@pve":                     "/access/users/{user}",
		"/cluster/resources":                          "/cluster/resources",
	}
	for in, want := range cases {
		if got := templateProxmoxPath(in); got != want {
			t.Errorf("templateProxmoxPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func logRESTFixture(t *testing.T, status func(n int32) int) (proxmoxRESTClient, *bytes.Buffer) {
	t.Helper()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status(calls.Add(1)))
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	t.Cleanup(srv.Close)

	var buf bytes.Buffer

	c := newProxmoxREST(srv.URL, testTokenName, testTokenVal, newProxmoxHTTPClient(false))
	c.log = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	c.cluster = "lab"

	return c, &buf
}

func parseLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	var out []map[string]any

	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if l == "" {
			continue
		}

		m := map[string]any{}
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatal(err)
		}

		out = append(out, m)
	}

	return out
}

func TestProxmoxREST_RetryWarnsAndDebugLogsTemplatedPath(t *testing.T) {
	t.Parallel()

	c, buf := logRESTFixture(t, func(n int32) int {
		if n == 1 {
			return http.StatusServiceUnavailable
		}

		return http.StatusOK
	})

	if _, err := c.do(context.Background(), http.MethodGet, "/nodes/pve1/qemu/101/status/current", nil); err != nil {
		t.Fatalf("do: %v", err)
	}

	lines := parseLines(t, buf)

	var warns, debugs int

	for _, l := range lines {
		switch l["level"] {
		case "WARN":
			warns++

			if l["msg"] != "proxmox request retrying" || l["attempt"] != float64(1) || l["error"] != "HTTP 503" {
				t.Errorf("warn line = %v", l)
			}
		case "DEBUG":
			debugs++

			if l["path"] != "/nodes/{node}/qemu/{vmid}/status/current" || l["cluster"] != "lab" || l["method"] != "GET" || l["durationMs"] == nil {
				t.Errorf("debug line = %v", l)
			}
		default:
			t.Errorf("unexpected level: %v", l)
		}
	}

	if warns != 1 || debugs != 2 {
		t.Fatalf("want 1 Warn + 2 Debug, got %d/%d: %s", warns, debugs, buf.String())
	}

	if strings.Contains(buf.String(), testTokenVal) || strings.Contains(buf.String(), "101/") {
		t.Errorf("token or raw path leaked: %s", buf.String())
	}
}

func TestProxmoxREST_FinalFailureLogsNoWarnOrError(t *testing.T) {
	t.Parallel()

	c, buf := logRESTFixture(t, func(int32) int { return http.StatusBadGateway })

	if _, err := c.do(context.Background(), http.MethodGet, "/cluster/resources", nil); err == nil {
		t.Fatal("expected error")
	}

	warns := 0

	for _, l := range parseLines(t, buf) {
		if l["level"] == "ERROR" {
			t.Errorf("client must not log the final failure: %v", l)
		}

		if l["level"] == "WARN" {
			warns++
		}
	}

	if warns != retryMaxAttempts-1 {
		t.Fatalf("warns = %d, want %d (one per retry)", warns, retryMaxAttempts-1)
	}
}

func TestProxmoxREST_PrefersRequestLogger(t *testing.T) {
	t.Parallel()

	c, fallback := logRESTFixture(t, func(int32) int { return http.StatusOK })

	var reqBuf bytes.Buffer

	ctx := logctx.With(context.Background(), slog.New(slog.NewJSONHandler(&reqBuf, &slog.HandlerOptions{Level: slog.LevelDebug})).With("requestId", "r1"))

	if _, err := c.do(ctx, http.MethodGet, "/nodes", nil); err != nil {
		t.Fatal(err)
	}

	if fallback.Len() != 0 || !strings.Contains(reqBuf.String(), `"requestId":"r1"`) {
		t.Fatalf("fallback=%q request=%q", fallback.String(), reqBuf.String())
	}
}

// One client span per Proxmox attempt, child of the caller's span, carrying
// cluster, templated path and status, but never the raw path or the token.
//
//nolint:paralleltest // serial: swaps the global tracer provider
func TestProxmoxREST_SpanPerCall(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exp)))
	prev := otel.GetTracerProvider()

	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev); _ = tp.Shutdown(context.Background()) })

	c, _ := logRESTFixture(t, func(int32) int { return http.StatusOK })

	ctx, parent := tp.Tracer("test").Start(context.Background(), "request")
	if _, err := c.do(ctx, http.MethodGet, "/nodes/pve1/qemu/101/status/current", nil); err != nil {
		t.Fatal(err)
	}

	parent.End()

	var client tracetest.SpanStub

	for _, s := range exp.GetSpans() {
		if s.SpanKind == trace.SpanKindClient {
			client = s
		}
	}

	if client.Name != "GET /nodes/{node}/qemu/{vmid}/status/current" {
		t.Fatalf("span name = %q", client.Name)
	}

	if client.Parent.SpanID() != parent.SpanContext().SpanID() {
		t.Errorf("client span is not a child of the request span")
	}

	attrs := map[string]string{}
	for _, a := range client.Attributes {
		attrs[string(a.Key)] = a.Value.Emit()
	}

	if attrs["cluster"] != "lab" || attrs["http.response.status_code"] != "200" {
		t.Errorf("attrs = %v", attrs)
	}

	for k, v := range attrs {
		if strings.Contains(v, "pve1") || strings.Contains(v, testTokenVal) {
			t.Errorf("attr %s leaks %q", k, v)
		}
	}
}
