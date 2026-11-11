package httpapi_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"testing"
	"time"
)

const (
	cacheImmutable = "public, max-age=31536000, immutable"
	cacheNoCache   = "no-cache"
)

// TestRouter_SPACacheHeaders: hashed assets under _app/immutable/ are cached
// for a year; the shell (index.html, including the SPA fallback) must
// revalidate every time so a new deploy is picked up immediately.
//
//nolint:paralleltest // serial: shared router and filesystem fixtures
func TestRouter_SPACacheHeaders(t *testing.T) {
	dir := t.TempDir()

	writeFixture(t, filepath.Join(dir, "index.html"), "<!doctype html><html><body>shell</body></html>")

	if err := os.MkdirAll(filepath.Join(dir, "_app", "immutable"), 0o750); err != nil {
		t.Fatalf("mkdir immutable: %v", err)
	}

	writeFixture(t, filepath.Join(dir, "_app", "immutable", "entry.abc123.js"), "console.log(1)")
	writeFixture(t, filepath.Join(dir, "favicon.txt"), "icon")

	mux := newCacheTestRouter(t, dir)

	cases := []struct {
		name string
		path string
		want string
	}{
		{name: "hashed asset", path: "/_app/immutable/entry.abc123.js", want: cacheImmutable},
		{name: "root shell", path: "/", want: cacheNoCache},
		{name: "spa fallback", path: "/some/client/route", want: cacheNoCache},
		{name: "other static file unchanged", path: "/favicon.txt", want: ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, c.path, nil))

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}

			if got := w.Header().Get("Cache-Control"); got != c.want {
				t.Fatalf("Cache-Control = %q, want %q", got, c.want)
			}
		})
	}
}

// TestRouter_APIKeepsNoStore: API responses must never be cached, even when
// the SPA cache rules apply to other paths.
//
//nolint:paralleltest // serial: shared router and filesystem fixtures
func TestRouter_APIKeepsNoStore(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "index.html"), "shell")

	mux := newCacheTestRouter(t, dir)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/unknown", nil))

	if got := w.Header().Get("Cache-Control"); got != "no-store, no-cache, must-revalidate, private" {
		t.Fatalf("Cache-Control = %q, want API no-store policy", got)
	}
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func newCacheTestRouter(t *testing.T, dir string) http.Handler {
	t.Helper()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	health := httpapi.NewHealth(fakeHealthPinger{}, logger, nil, 60*time.Second)
	clusterNodes := httpapi.NewClusterNodesWithRegistry(emptyRegistry(), newAuthHandler(t), logger)
	projection := inventory.NewProjection()
	clusterRefresh := httpapi.NewClusterRefreshWithRegistry(refreshRegistryFor(projection,
		inventory.NewRefresher(inventory.NewWorker(&stubClusterClient{}, projection, time.Hour, logger), 5*time.Second)), nil, newAuthHandler(t), logger)
	vms := httpapi.NewVMsWithRegistry(emptyRegistry(), newAuthHandler(t), 100, -1, logger, nil, nil)
	vmDetail := httpapi.NewVMDetailWithRegistry(httpapi.VMDetailDeps{Source: emptyRegistry(), Projection: inventory.NewProjection(), Auth: newAuthHandler(t), Writer: cluster.Fake{}, Store: nil, Refresher: nil, Log: logger}, nil)

	return httpapi.NewRouter(httpapi.RouterConfig{
		Health: health, ClusterNodes: clusterNodes, ClusterRefresh: clusterRefresh,
		VMs: vms, VMDetail: vmDetail, Auth: newAuthHandler(t), WebBuildDir: dir, Log: logger,
	})
}
