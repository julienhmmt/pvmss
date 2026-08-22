//nolint:noctx // test scaffolding uses in-memory requests
package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

//nolint:paralleltest // serial: shared fake fixtures
func TestVMMetricsCoverage_History_Unauthenticated(t *testing.T) {
	handler, _ := newVMMetricsHandler(t)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, metricsRequest("/api/v1/vms/default/100/metrics/history?range=hour", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertAPIError(t, rec.Body.Bytes(), "unauthenticated")
}

//nolint:paralleltest // serial: shared fake fixtures
func TestVMMetricsCoverage_History_InvalidRange(t *testing.T) {
	handler, authHandler := newVMMetricsHandler(t)
	cookie := aliceCookie(t, authHandler)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, metricsRequest("/api/v1/vms/default/100/metrics/history?range=bad", cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	assertAPIError(t, rec.Body.Bytes(), "invalid_range")
}

//nolint:paralleltest // serial: shared fake fixtures
func TestVMMetricsCoverage_History_DayRange(t *testing.T) {
	handler, authHandler := newVMMetricsHandler(t)
	cookie := aliceCookie(t, authHandler)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, metricsRequest("/api/v1/vms/default/100/metrics/history?range=day", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body metricsHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Range != "day" {
		t.Errorf("range = %q, want day", body.Range)
	}
}

//nolint:paralleltest // serial: shared fake fixtures
func TestVMMetricsCoverage_History_WeekRange(t *testing.T) {
	handler, authHandler := newVMMetricsHandler(t)
	cookie := aliceCookie(t, authHandler)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, metricsRequest("/api/v1/vms/default/100/metrics/history?range=week", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body metricsHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Range != "week" {
		t.Errorf("range = %q, want week", body.Range)
	}
}

//nolint:paralleltest // serial: shared fake fixtures
func TestVMMetricsCoverage_History_UntaggedNotFound(t *testing.T) {
	handler, authHandler := newVMMetricsHandler(t)
	cookie := aliceCookie(t, authHandler)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, metricsRequest("/api/v1/vms/default/109/metrics/history?range=hour", cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	assertAPIError(t, rec.Body.Bytes(), "not_found")
}

//nolint:paralleltest // serial: shared fake fixtures
func TestVMMetricsCoverage_History_AdminSeesAnyTaggedVM(t *testing.T) {
	handler, authHandler := newVMMetricsHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, metricsRequest("/api/v1/vms/default/103/metrics/history?range=hour", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake fixtures
func TestVMMetricsCoverage_Stream_Unauthenticated(t *testing.T) {
	handler, _ := newVMMetricsHandler(t)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, metricsRequest("/api/v1/vms/default/100/metrics/stream", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertAPIError(t, rec.Body.Bytes(), "unauthenticated")
}

//nolint:paralleltest // serial: shared fake fixtures
func TestVMMetricsCoverage_Stream_NonOwnerForbidden(t *testing.T) {
	handler, authHandler := newVMMetricsHandler(t)
	cookie := bobCookie(t, authHandler)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, metricsRequest("/api/v1/vms/default/100/metrics/stream", cookie))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake fixtures
func TestVMMetricsCoverage_Stream_UntaggedNotFound(t *testing.T) {
	handler, authHandler := newVMMetricsHandler(t)
	cookie := aliceCookie(t, authHandler)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, metricsRequest("/api/v1/vms/default/109/metrics/stream", cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake fixtures
func TestVMMetricsCoverage_Stream_AdminSeesAnyTaggedVM(t *testing.T) {
	handler, authHandler := newVMMetricsHandler(t)
	cookie := adminCookie(t, authHandler)

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	req := metricsRequest("/api/v1/vms/default/103/metrics/stream", cookie).Clone(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return after context expiry")
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake fixtures
func TestVMMetricsCoverage_History_InvalidVMPath(t *testing.T) {
	handler, authHandler := newVMMetricsHandler(t)
	cookie := aliceCookie(t, authHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vms/default/abc/metrics/history?range=hour", nil)
	req.SetPathValue("cluster", "default")
	req.SetPathValue("vmid", "abc")
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertAPIError(t, rec.Body.Bytes(), "invalid_request")
}

//nolint:paralleltest // serial: shared fake fixtures
func TestVMMetricsCoverage_History_NegativeVMID(t *testing.T) {
	handler, authHandler := newVMMetricsHandler(t)
	cookie := aliceCookie(t, authHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vms/default/-1/metrics/history?range=hour", nil)
	req.SetPathValue("cluster", "default")
	req.SetPathValue("vmid", "-1")
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertAPIError(t, rec.Body.Bytes(), "invalid_request")
}

//nolint:paralleltest // serial: shared fake fixtures
func TestVMMetricsCoverage_Stream_PausedVMRejected(t *testing.T) {
	handler, authHandler := newVMMetricsHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, metricsRequest("/api/v1/vms/default/113/metrics/stream", cookie))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake fixtures
func TestVMMetricsCoverage_Stream_ContentTypeAndRetryHeader(t *testing.T) {
	handler, authHandler := newVMMetricsHandler(t)
	cookie := aliceCookie(t, authHandler)

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	req := metricsRequest("/api/v1/vms/default/100/metrics/stream", cookie).Clone(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(rec, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return after context expiry")
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("content-type = %q, want text/event-stream prefix", ct)
	}

	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("cache-control = %q, want no-cache", cc)
	}
}
