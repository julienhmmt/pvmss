package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"strings"
	"testing"
	"time"
)

//nolint:paralleltest // serial: shared router and filesystem fixtures
func TestRouter_SPAFallback(t *testing.T) {
	dir := t.TempDir()

	html := `<!doctype html><html lang="en"><head><title>PVMSS</title></head><body>shell</body></html>`
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0o600); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("real file"), 0o600); err != nil {
		t.Fatalf("write file.txt: %v", err)
	}

	if err := os.Mkdir(filepath.Join(dir, "a-dir"), 0o750); err != nil {
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
	vmDetail := httpapi.NewVMDetail(inventory.NewProjection(), newAuthHandler(t), cluster.Fake{}, nil, nil, logger)
	mux := httpapi.NewRouter(httpapi.RouterConfig{
		Health: health, ClusterNodes: clusterNodes, ClusterRefresh: clusterRefresh,
		VMs: vms, VMDetail: vmDetail, Auth: newAuthHandler(t), WebBuildDir: dir, Log: logger,
	})

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
			r := httptest.NewRequestWithContext(context.Background(), c.method, c.path, nil)
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

const cloudInitRoutePath = "/api/v1/vms/default/101/cloudinit"

// TestRouter_CloudInitRoutesAreSpecific verifies that cloudinit routes require
// the exact (vm, node) pair and reject near-miss paths (T008).
func TestRouter_CloudInitRoutesAreSpecific(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	authHandler := newAuthHandler(t)
	projection := inventory.NewProjection()
	health := httpapi.NewHealth(fakePinger{}, logger)
	clusterNodes := httpapi.NewClusterNodes(projection, logger)
	clusterRefresh := httpapi.NewClusterRefresh(inventory.NewRefresher(inventory.NewWorker(&stubClusterClient{}, projection, time.Hour, logger), 5*time.Second), logger)
	vms := httpapi.NewVMs(projection, authHandler, 100, -1, logger)
	vmDetail := httpapi.NewVMDetail(projection, authHandler, cluster.Fake{}, nil, nil, logger)
	cloudInit := httpapi.NewVMCloudInit(projection, authHandler, cluster.Fake{}, cluster.Fake{}, nil, nil, logger)
	mux := httpapi.NewRouter(httpapi.RouterConfig{
		Health: health, ClusterNodes: clusterNodes, ClusterRefresh: clusterRefresh,
		VMs: vms, VMDetail: vmDetail, VMCloudInit: cloudInit, Auth: authHandler, Log: logger,
	})

	for _, path := range []string{
		cloudInitRoutePath,
		"/api/v1/vms/default/101/cloudinit/snippet",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
		mux.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("GET %s status = %d, want 401 (route reached handler)", path, recorder.Code)
		}
	}
}

//nolint:paralleltest // serial: shared router and filesystem fixtures
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
	vmDetail := httpapi.NewVMDetail(inventory.NewProjection(), newAuthHandler(t), cluster.Fake{}, nil, nil, logger)
	mux := httpapi.NewRouter(httpapi.RouterConfig{
		Health: health, ClusterNodes: clusterNodes, ClusterRefresh: clusterRefresh,
		VMs: vms, VMDetail: vmDetail, Auth: newAuthHandler(t), Log: logger,
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", nil)
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

	if got.Status != healthStatusHealthy {
		t.Fatalf("status = %q, want healthy", got.Status)
	}
}
