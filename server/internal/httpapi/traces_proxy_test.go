package httpapi_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/httpapi"
	"strings"
	"testing"
	"time"
)

func tracesRouter(t *testing.T, proxy *httpapi.TracesProxy) http.Handler {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	return httpapi.NewRouter(httpapi.RouterConfig{
		Health: httpapi.NewHealth(fakeHealthPinger{}, logger, nil, time.Minute),
		Auth:   newAuthHandler(t),
		Log:    logger,
		Traces: proxy,
	})
}

func postTraces(t *testing.T, router http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/traces", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec
}

//nolint:paralleltest // serial: shared auth fixtures
func TestTracesProxy_ForwardsBodyAndCollectorHeaders(t *testing.T) {
	var gotBody []byte

	var gotAuth, gotContentType string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	router := tracesRouter(t, httpapi.NewTracesProxy(upstream.URL, http.Header{"Authorization": {"Bearer SECRET"}}))
	rec := postTraces(t, router, `{"resourceSpans":[]}`)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}

	if string(gotBody) != `{"resourceSpans":[]}` {
		t.Fatalf("upstream body = %q, want the export verbatim", gotBody)
	}

	if gotAuth != "Bearer SECRET" {
		t.Fatalf("upstream Authorization = %q, want the configured header attached server-side", gotAuth)
	}

	if gotContentType != "application/json" {
		t.Fatalf("upstream Content-Type = %q, want application/json", gotContentType)
	}
}

//nolint:paralleltest // serial: shared auth fixtures
func TestTracesProxy_UpstreamRefusalIs502(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(upstream.Close)

	router := tracesRouter(t, httpapi.NewTracesProxy(upstream.URL, nil))
	rec := postTraces(t, router, `{"resourceSpans":[]}`)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
}

//nolint:paralleltest // serial: shared auth fixtures
func TestTracesProxy_BodyIsCapped(t *testing.T) {
	var upstreamHits int

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamHits++

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	router := tracesRouter(t, httpapi.NewTracesProxy(upstream.URL, nil))
	rec := postTraces(t, router, strings.Repeat("x", 128<<10))

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}

	if upstreamHits != 0 {
		t.Fatalf("oversized body must never reach the collector, got %d forwards", upstreamHits)
	}
}

//nolint:paralleltest // serial: shared auth fixtures
func TestTracesProxy_RouteIs404WithoutCollector(t *testing.T) {
	router := tracesRouter(t, nil)
	rec := postTraces(t, router, `{"resourceSpans":[]}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when no collector endpoint is configured", rec.Code)
	}
}
