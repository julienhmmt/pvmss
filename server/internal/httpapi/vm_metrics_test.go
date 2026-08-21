//nolint:noctx // test scaffolding uses in-memory requests
package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"testing"
	"time"
)

type metricsHistoryResponse struct {
	Range   string `json:"range"`
	Samples []struct {
		Timestamp        string  `json:"timestamp"`
		CPUPercent       float64 `json:"cpuPercent"`
		MemoryUsedBytes  int64   `json:"memoryUsedBytes"`
		MemoryMaxBytes   int64   `json:"memoryMaxBytes"`
		DiskReadBytesPS  float64 `json:"diskReadBytesPerSec"`
		DiskWriteBytesPS float64 `json:"diskWriteBytesPerSec"`
		NetInBytesPS     float64 `json:"netInBytesPerSec"`
		NetOutBytesPS    float64 `json:"netOutBytesPerSec"`
	} `json:"samples"`
}

func newVMMetricsHandler(t *testing.T) (*httpapi.VMMetrics, *httpapi.Auth) {
	t.Helper()
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	snapshot, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	index := inventory.BuildIndex(snapshot)
	index.RefreshedAt = time.Now()
	projection := inventory.NewProjectionFromIndex(&index)
	authHandler := newAuthHandler(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	return httpapi.NewVMMetrics(projection, authHandler, cluster.Fake{}, logger), authHandler
}

func metricsRequest(path string, cookie *http.Cookie) *http.Request {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		request.AddCookie(cookie)
	}

	request.SetPathValue("cluster", "default")
	request.SetPathValue("vmid", pathVmid(path))

	return request
}

//nolint:paralleltest // serial: shared fake fixtures
func TestVMMetrics_History_OwnerContract(t *testing.T) {
	handler, authHandler := newVMMetricsHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, metricsRequest("/api/v1/vms/default/100/metrics/history?range=hour", cookie))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body metricsHistoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Range != "hour" {
		t.Errorf("range = %q, want hour", body.Range)
	}

	if len(body.Samples) == 0 {
		t.Fatal("expected at least one sample")
	}

	for _, s := range body.Samples {
		if s.MemoryUsedBytes > s.MemoryMaxBytes {
			t.Errorf("sample memoryUsedBytes %d > memoryMaxBytes %d", s.MemoryUsedBytes, s.MemoryMaxBytes)
		}
	}
}

//nolint:paralleltest // serial: shared fake fixtures
func TestVMMetrics_History_NonOwnerForbidden(t *testing.T) {
	handler, authHandler := newVMMetricsHandler(t)
	cookie := bobCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, metricsRequest("/api/v1/vms/default/100/metrics/history?range=hour", cookie))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake fixtures
func TestVMMetrics_History_InvalidRange(t *testing.T) {
	handler, authHandler := newVMMetricsHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, metricsRequest("/api/v1/vms/default/100/metrics/history?range=month", cookie))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake fixtures
func TestVMMetrics_History_VMNotFound(t *testing.T) {
	handler, authHandler := newVMMetricsHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, metricsRequest("/api/v1/vms/default/999999/metrics/history?range=hour", cookie))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
	}
}
