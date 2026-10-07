package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// With no OTEL configuration the global tracer is the no-op default; these two
// benchmarks show what withTracing costs a handler in that mode.
func benchHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
}

func BenchmarkHandler_Bare(b *testing.B) {
	h := benchHandler()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/x", nil)

	b.ReportAllocs()

	for b.Loop() {
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
}

func BenchmarkHandler_WithTracingNoop(b *testing.B) {
	h := withTracing(nil, benchHandler())
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/x", nil)

	b.ReportAllocs()

	for b.Loop() {
		h.ServeHTTP(httptest.NewRecorder(), req)
	}
}
