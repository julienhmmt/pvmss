//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"strings"
	"testing"
)

// newAdminStore opens a fully-migrated store (V9) with the T06 seed and pvmss
// tag, ready for admin catalog handler tests.
func newAdminStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.Open(config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "admin-httpapi.db"),
		LogLevel:  snapshotTestLogLevel,
		LogFormat: snapshotTestLogFormat,
		LogOutput: snapshotTestLogOutput,
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	return st
}

// newAdminHandler builds the AdminCatalog handler with a real store, the
// fake cluster client, and a projection populated from the fake snapshot
// (needed for tag VM counts), and returns a mux that wires the admin routes
// through RequireAdmin — the real guard the tests must exercise.
func newAdminHandler(t *testing.T) (*httpapi.AdminCatalog, *httpapi.Auth, *store.Store) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)

	authHandler := newAuthHandler(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	st := newAdminStore(t)
	fake := cluster.Fake{}
	snap, _ := fake.Snapshot(context.Background())
	idx := inventory.BuildIndex(snap)
	projection := inventory.NewProjectionFromIndex(&idx)
	registry := fakeClusterListerProvider{names: []string{auditTestCluster}}
	adminCatalog := httpapi.NewAdminCatalogWithRegistry(authHandler, st, registry, projection, logger)

	return adminCatalog, authHandler, st
}

// adminMux builds a minimal ServeMux with the admin routes wired through
// RequireAdmin, so non-admin tests exercise the real 403 guard.
func adminMux(handler *httpapi.AdminCatalog, auth *httpapi.Auth) *http.ServeMux {
	mux := http.NewServeMux()
	guard := auth.RequireAdmin
	mux.Handle("GET /api/v1/admin/nodes", guard(http.HandlerFunc(handler.ServeNodes)))
	mux.Handle("POST /api/v1/admin/nodes/toggle", guard(http.HandlerFunc(handler.ServeNodeToggle)))
	mux.Handle("GET /api/v1/admin/storages", guard(http.HandlerFunc(handler.ServeStorages)))
	mux.Handle("POST /api/v1/admin/storages/toggle", guard(http.HandlerFunc(handler.ServeStorageToggle)))
	mux.Handle("GET /api/v1/admin/bridges", guard(http.HandlerFunc(handler.ServeBridges)))
	mux.Handle("POST /api/v1/admin/bridges/toggle", guard(http.HandlerFunc(handler.ServeBridgeToggle)))
	mux.Handle("GET /api/v1/admin/isos", guard(http.HandlerFunc(handler.ServeISOs)))
	mux.Handle("POST /api/v1/admin/isos/toggle", guard(http.HandlerFunc(handler.ServeISOToggle)))
	mux.Handle("GET /api/v1/admin/templates", guard(http.HandlerFunc(handler.ServeTemplates)))
	mux.Handle("POST /api/v1/admin/templates/toggle", guard(http.HandlerFunc(handler.ServeTemplateToggle)))
	mux.Handle("PUT /api/v1/admin/templates/{cluster}/{vmid}", guard(http.HandlerFunc(handler.ServeTemplateUpdate)))
	mux.Handle("DELETE /api/v1/admin/templates/{cluster}/{vmid}", guard(http.HandlerFunc(handler.ServeTemplateDelete)))
	mux.Handle("GET /api/v1/admin/profiles", guard(http.HandlerFunc(handler.ServeProfiles)))
	mux.Handle("POST /api/v1/admin/profiles", guard(http.HandlerFunc(handler.ServeProfileCreate)))
	mux.Handle("PUT /api/v1/admin/profiles/{id}", guard(http.HandlerFunc(handler.ServeProfileUpdate)))
	mux.Handle("DELETE /api/v1/admin/profiles/{id}", guard(http.HandlerFunc(handler.ServeProfileDelete)))
	mux.Handle("POST /api/v1/admin/profiles/{id}/toggle", guard(http.HandlerFunc(handler.ServeProfileToggle)))
	mux.Handle("GET /api/v1/admin/tags", guard(http.HandlerFunc(handler.ServeTags)))
	mux.Handle("POST /api/v1/admin/tags", guard(http.HandlerFunc(handler.ServeTagCreate)))
	mux.Handle("PUT /api/v1/admin/tags/{name}/color", guard(http.HandlerFunc(handler.ServeTagColor)))
	mux.Handle("DELETE /api/v1/admin/tags/{name}", guard(http.HandlerFunc(handler.ServeTagDelete)))

	return mux
}

// adminGet performs an authenticated GET as admin against the given path.
func adminGet(t *testing.T, handler *httpapi.AdminCatalog, auth *httpapi.Auth, cookie *http.Cookie, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	adminMux(handler, auth).ServeHTTP(rec, req)

	return rec
}

// adminPost performs an authenticated POST as admin with the given JSON body.
func adminPost(t *testing.T, handler *httpapi.AdminCatalog, auth *httpapi.Auth, cookie *http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	adminMux(handler, auth).ServeHTTP(rec, req)

	return rec
}

// adminPut performs an authenticated PUT as admin with the given JSON body.
func adminPut(t *testing.T, handler *httpapi.AdminCatalog, auth *httpapi.Auth, cookie *http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	adminMux(handler, auth).ServeHTTP(rec, req)

	return rec
}

// adminDelete performs an authenticated DELETE as admin.
func adminDelete(t *testing.T, handler *httpapi.AdminCatalog, auth *httpapi.Auth, cookie *http.Cookie, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodDelete, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	adminMux(handler, auth).ServeHTTP(rec, req)

	return rec
}
