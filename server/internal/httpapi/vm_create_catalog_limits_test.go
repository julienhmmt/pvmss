package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"testing"
)

func newVMCreateHandlerWithPolicy(t *testing.T) (*httpapi.VMCreate, *httpapi.Auth) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)

	authHandler := newAuthHandler(t)

	st, err := store.Open(config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "vm-create-policy.db"),
		LogLevel:  snapshotTestLogLevel,
		LogFormat: snapshotTestLogFormat,
		LogOutput: snapshotTestLogOutput,
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })
	seedBridgeApprovals(t, st)
	seedISOApprovals(t, st)
	seedTagApprovals(t, st)

	snapshot, err := cluster.Fake{}.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	index := inventory.BuildIndex(snapshot)
	service := policy.New(st, inventory.NewProjectionFromIndex(&index), cluster.Fake{})
	provider := vmCreateClientProvider{clients: map[string]cluster.Client{auditTestCluster: cluster.Fake{}}}

	return httpapi.NewVMCreateWithRegistry(authHandler, st, provider, cluster.Fake{}, cluster.Fake{}, slog.New(slog.DiscardHandler), service), authHandler
}

// With a policy service wired in, the catalog carries the gabarit and the
// caller's quota so the wizard can validate hardware fields client-side.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreateCatalog_CarriesGabaritAndQuota(t *testing.T) {
	handler, authHandler := newVMCreateHandlerWithPolicy(t)
	cookie := aliceCookie(t, authHandler)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/vm-create/catalog", nil)
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.ServeCatalog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Gabarit *struct {
			MaxCores int `json:"maxCores"`
		} `json:"gabarit"`
		Quota *struct {
			Allowed int `json:"allowed"`
		} `json:"quota"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}

	if body.Gabarit == nil || body.Quota == nil {
		t.Fatalf("catalog = %s, want gabarit and quota present", rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreateCatalog_RejectsAnonymousAndUnknownCluster(t *testing.T) {
	handler, authHandler := newVMCreateHandlerWithPolicy(t)

	anonymous := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/vm-create/catalog", nil)
	rec := httptest.NewRecorder()
	handler.ServeCatalog(rec, anonymous)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want 401", rec.Code)
	}

	unknown := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/vm-create/catalog?cluster=nonexistent", nil)
	unknown.AddCookie(aliceCookie(t, authHandler))

	rec = httptest.NewRecorder()
	handler.ServeCatalog(rec, unknown)

	if rec.Code != http.StatusNotFound && rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown cluster status = %d, want 404 or 400: %s", rec.Code, rec.Body.String())
	}
}
