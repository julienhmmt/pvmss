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
	health := httpapi.NewHealth(fakeHealthPinger{}, logger, nil, 60*time.Second)
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
	health := httpapi.NewHealth(fakeHealthPinger{}, logger, nil, 60*time.Second)
	clusterNodes := httpapi.NewClusterNodes(projection, logger)
	clusterRefresh := httpapi.NewClusterRefresh(inventory.NewRefresher(inventory.NewWorker(&stubClusterClient{}, projection, time.Hour, logger), 5*time.Second), logger)
	vms := httpapi.NewVMs(projection, authHandler, 100, -1, logger)
	vmDetail := httpapi.NewVMDetail(projection, authHandler, cluster.Fake{}, nil, nil, logger)
	cloudInit := httpapi.NewVMCloudInit(httpapi.VMCloudInitDeps{Projection: projection, Auth: authHandler, Reader: cluster.Fake{}, Writer: cluster.Fake{}, Store: nil, Refresher: nil, Log: logger})
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

// TestRouter_BootCDROMRouteRegistered verifies the boot-cdrom route is wired
// through NewRouter: an unauthenticated POST must reach the handler (401), not
// fall through to the mux's 404. Regression: the handler existed but the mux
// pattern was missing, so every real request 404'd.
func TestRouter_BootCDROMRouteRegistered(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	authHandler := newAuthHandler(t)
	projection := inventory.NewProjection()
	health := httpapi.NewHealth(fakeHealthPinger{}, logger, nil, 60*time.Second)
	clusterNodes := httpapi.NewClusterNodes(projection, logger)
	clusterRefresh := httpapi.NewClusterRefresh(inventory.NewRefresher(inventory.NewWorker(&stubClusterClient{}, projection, time.Hour, logger), 5*time.Second), logger)
	vms := httpapi.NewVMs(projection, authHandler, 100, -1, logger)
	vmDetail := httpapi.NewVMDetail(projection, authHandler, cluster.Fake{}, nil, nil, logger)
	mux := httpapi.NewRouter(httpapi.RouterConfig{
		Health: health, ClusterNodes: clusterNodes, ClusterRefresh: clusterRefresh,
		VMs: vms, VMDetail: vmDetail, Auth: authHandler, Log: logger,
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/vms/default/101/boot-cdrom", strings.NewReader("{}"))
	mux.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("POST boot-cdrom status = %d, want 401 (route reached handler, not 404)", recorder.Code)
	}
}

//nolint:paralleltest // serial: shared router and filesystem fixtures
func TestRouter_MissingBuildDir_HealthStillWorks(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	health := httpapi.NewHealth(fakeHealthPinger{}, logger, nil, 60*time.Second)
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

// TestRouter_DocsRoutesRegistered verifies the docs and admin-docs routes are
// wired through NewRouter when the handlers are provided (issue #53).
//
//nolint:paralleltest // serial: shared router and database fixtures
func TestRouter_DocsRoutesRegistered(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	authHandler := newAuthHandler(t)
	st := newAdminStore(t)
	projection := inventory.NewProjection()
	health := httpapi.NewHealth(fakeHealthPinger{}, logger, nil, 60*time.Second)
	clusterNodes := httpapi.NewClusterNodes(projection, logger)
	clusterRefresh := httpapi.NewClusterRefresh(inventory.NewRefresher(inventory.NewWorker(&stubClusterClient{}, projection, time.Hour, logger), 5*time.Second), logger)
	vms := httpapi.NewVMs(projection, authHandler, 100, -1, logger)
	vmDetail := httpapi.NewVMDetail(projection, authHandler, cluster.Fake{}, nil, nil, logger)
	docs := httpapi.NewDocsAPIHandler(authHandler, st, logger)
	adminDocs := httpapi.NewAdminDocs(authHandler, st, docs, logger)
	mux := httpapi.NewRouter(httpapi.RouterConfig{
		Health: health, ClusterNodes: clusterNodes, ClusterRefresh: clusterRefresh,
		VMs: vms, VMDetail: vmDetail, Auth: authHandler, Log: logger,
		Docs: docs, AdminDocs: adminDocs,
	})

	// Public docs list — reaches the handler (empty list, 200).
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/docs", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("public docs list status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	// Admin docs list without auth — reaches the RequireAdmin guard (401).
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/admin/docs", nil)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("admin docs list without auth status = %d, want 401", rec.Code)
	}

	// Admin docs list with non-admin auth — reaches the RequireAdmin guard (403).
	alice := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/admin/docs", nil)
	req.AddCookie(alice)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin docs list with non-admin status = %d, want 403", rec.Code)
	}

	// Admin docs list with admin auth — reaches the handler (200).
	cookie := adminCookie(t, authHandler)
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/admin/docs", nil)
	req.AddCookie(cookie)
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("admin docs list with auth status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	// Every admin docs CRUD route reaches the RequireAdmin guard (401 without
	// auth) — this covers the route registration lines for POST/PUT/DELETE/toggle.
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/admin/docs"},
		{http.MethodPut, "/api/v1/admin/docs/x/en"},
		{http.MethodDelete, "/api/v1/admin/docs/x/en"},
		{http.MethodPost, "/api/v1/admin/docs/x/en/toggle"},
	} {
		rec = httptest.NewRecorder()
		req = httptest.NewRequestWithContext(context.Background(), tc.method, tc.path, nil)
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: status = %d, want 401 (route reached guard)", tc.method, tc.path, rec.Code)
		}
	}
}
