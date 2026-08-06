package httpapi_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
)

func TestRouter_SPAFallback(t *testing.T) {
	dir := t.TempDir()
	html := `<!doctype html><html lang="en"><head><title>PVMSS</title></head><body>shell</body></html>`
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("real file"), 0644); err != nil {
		t.Fatalf("write file.txt: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "a-dir"), 0755); err != nil {
		t.Fatalf("create a-dir: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	health := httpapi.NewHealth(fakePinger{}, logger)
	clusterNodes := httpapi.NewClusterNodes(inventory.NewProjection(), logger)
	clusterRefresh := httpapi.NewClusterRefresh(
		inventory.NewRefresher(
			inventory.NewWorker(&stubClusterClient{}, inventory.NewProjection(), time.Hour, logger),
			5*time.Second,
		),
		logger,
	)
	vms := httpapi.NewVMs(inventory.NewProjection(), newAuthHandler(t), 100, -1, logger)
	vmDetail := httpapi.NewVmDetail(inventory.NewProjection(), newAuthHandler(t), cluster.Fake{}, nil, nil, logger)
	mux := httpapi.NewRouter(health, clusterNodes, clusterRefresh, vms, vmDetail, newAuthHandler(t), dir, logger)

	cases := []struct {
		method     string
		path       string
		wantStatus int
		wantBody   string
	}{
		{method: http.MethodGet, path: "/", wantStatus: http.StatusOK, wantBody: "shell"},
		{method: http.MethodGet, path: "/anything-that-does-not-exist", wantStatus: http.StatusOK, wantBody: "shell"},
		{method: http.MethodGet, path: "/file.txt", wantStatus: http.StatusOK, wantBody: "real file"},
		{method: http.MethodGet, path: "/missing.txt", wantStatus: http.StatusNotFound, wantBody: "asset not found"},
		{method: http.MethodGet, path: "/a-dir", wantStatus: http.StatusOK, wantBody: "shell"},
		{method: http.MethodGet, path: "/api/unknown", wantStatus: http.StatusNotFound, wantBody: "unknown API path"},
		{method: http.MethodPost, path: "/api/unknown", wantStatus: http.StatusNotFound, wantBody: "unknown API path"},
	}

	for _, c := range cases {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(c.method, c.path, nil)
			mux.ServeHTTP(w, r)

			if w.Code != c.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, c.wantStatus)
			}
			body, _ := io.ReadAll(w.Result().Body)
			if !strings.Contains(string(body), c.wantBody) {
				t.Fatalf("body = %q, want to contain %q", string(body), c.wantBody)
			}
		})
	}
}

func TestRouter_MissingBuildDir_HealthStillWorks(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	health := httpapi.NewHealth(fakePinger{}, logger)
	clusterNodes := httpapi.NewClusterNodes(inventory.NewProjection(), logger)
	clusterRefresh := httpapi.NewClusterRefresh(
		inventory.NewRefresher(
			inventory.NewWorker(&stubClusterClient{}, inventory.NewProjection(), time.Hour, logger),
			5*time.Second,
		),
		logger,
	)
	vms := httpapi.NewVMs(inventory.NewProjection(), newAuthHandler(t), 100, -1, logger)
	vmDetail := httpapi.NewVmDetail(inventory.NewProjection(), newAuthHandler(t), cluster.Fake{}, nil, nil, logger)
	mux := httpapi.NewRouter(health, clusterNodes, clusterRefresh, vms, vmDetail, newAuthHandler(t), "", logger)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", w.Code, http.StatusOK)
	}
	var got struct {
		Status    string                         `json:"status"`
		Checks    map[string]httpapi.CheckResult `json:"checks"`
		Timestamp string                         `json:"timestamp"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if got.Status != "healthy" {
		t.Fatalf("status = %q, want healthy", got.Status)
	}
}
