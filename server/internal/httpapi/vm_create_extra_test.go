//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/store"
	"strings"
	"testing"
)

// failingCreator embeds cluster.Fake (which already satisfies cluster.Creator)
// and overrides NextVMID so the ErrClusterCreate branch of writeCreateFailure
// is reachable: VMID allocation fails after validation passes, which is the
// path that maps to 502 cluster_error.
type failingCreator struct {
	cluster.Fake
	nextVMIDErr error
}

func (f failingCreator) NextVMID(ctx context.Context) (int, error) {
	if f.nextVMIDErr != nil {
		return 0, f.nextVMIDErr
	}

	return f.Fake.NextVMID(ctx)
}

// newVMCreateHandlerWithCreator builds the creation handler over the seeded
// store with a custom cluster.Creator, so the cluster-dispatch failure path
// can be exercised.
func newVMCreateHandlerWithCreator(t *testing.T, creator cluster.Creator) (*httpapi.VMCreate, *httpapi.Auth) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)

	authHandler := newAuthHandler(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	st, err := store.Open(config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "vm-create-extra.db"),
		LogLevel:  snapshotTestLogLevel,
		LogFormat: snapshotTestLogFormat,
		LogOutput: snapshotTestLogOutput,
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	return httpapi.NewVMCreate(authHandler, st, creator, cluster.Fake{}, logger), authHandler
}

// TestVMCreate_Unauthenticated — no cookie → 401, never reaches vm.Create.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreate_Unauthenticated(t *testing.T) {
	handler, _ := newVMCreateHandlerWithCreator(t, cluster.Fake{})

	rec := postVMCreate(t, handler,
		`{"cluster":"default","name":"web-04","profileId":"medium"}`, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	assertAPIError(t, rec.Body.Bytes(), apiCodeUnauthenticated)

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Errorf("fake calls = %d, want 0 (unauthenticated never reaches the cluster)", len(calls))
	}
}

// TestVMCreate_InvalidBody — malformed JSON → 400 invalid_request.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreate_InvalidBody(t *testing.T) {
	handler, authHandler := newVMCreateHandlerWithCreator(t, cluster.Fake{})
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	rec := postVMCreate(t, handler, `{not json`, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	assertAPIError(t, rec.Body.Bytes(), apiCodeInvalidRequest)
}

// TestVMCreate_ClusterCreateErrorMapped — a NextVMID failure wraps to
// ErrClusterCreate, which writeCreateFailure maps to 502 cluster_error.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreate_ClusterCreateErrorMapped(t *testing.T) {
	handler, authHandler := newVMCreateHandlerWithCreator(t, failingCreator{
		nextVMIDErr: errors.New("proxmox VMID endpoint down"),
	})
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	rec := postVMCreate(t, handler,
		`{"cluster":"default","name":"web-04","profileId":"medium"}`, cookie)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}

	assertAPIError(t, rec.Body.Bytes(), apiCodeClusterError)
}

// TestVMCreateCatalog_Unauthenticated — GET catalog without a cookie → 401.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreateCatalog_Unauthenticated(t *testing.T) {
	handler, _ := newVMCreateHandlerWithCreator(t, cluster.Fake{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vm-create/catalog", nil)
	rec := httptest.NewRecorder()
	handler.ServeCatalog(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	assertAPIError(t, rec.Body.Bytes(), apiCodeUnauthenticated)
}

// TestVMCreateCatalog_DefaultClusterWhenOmitted — a request without a cluster
// query parameter falls back to the default cluster name and still serves the
// seeded catalog (the ServeCatalog default-cluster branch).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreateCatalog_DefaultClusterWhenOmitted(t *testing.T) {
	handler, authHandler := newVMCreateHandlerWithCreator(t, cluster.Fake{})
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vm-create/catalog", nil)
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.ServeCatalog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), `"cluster":"default"`) {
		t.Errorf("body = %s, want cluster=default (the fallback)", rec.Body.String())
	}
}
