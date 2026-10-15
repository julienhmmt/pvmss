//nolint:wsl_v5 // authentication scenarios keep request and session assertions together
package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/store"
	"strings"
	"testing"
)

// newClusterAuthFixture builds a registry-backed Auth handler against a
// freshly-migrated, 3-cluster-seeded store (default/secondary/offline-demo),
// shared by every test in this file that needs multi-cluster login
// behavior.
func newClusterAuthFixture(t *testing.T) (*httpapi.Auth, *store.Store) {
	t.Helper()
	secret := "auth-cluster-test-secret-with-32-bytes" //nolint:gosec // deterministic test secret
	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "auth-cluster.db"), ClusterSource: cluster.SourceFake, SessionSecret: secret})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	rows, err := st.ListClusters(context.Background())
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}
	registry, err := cluster.NewRegistry("fake", rows)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	sessions, err := auth.NewSessionManager(st, secret, false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}
	authHandler := httpapi.NewAuthWithRegistry(registry, st, sessions, "", slog.Default())
	return authHandler, st
}

//nolint:paralleltest // authentication fixture shares fake identities; wire value is asserted verbatim
func TestAuth_LoginRequiresAndStoresClusterChoice(t *testing.T) {
	authHandler, _ := newClusterAuthFixture(t)

	missing := loginRequest(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)
	if missing.Code != http.StatusBadRequest || !strings.Contains(missing.Body.String(), "cluster_required") {
		t.Fatalf("missing cluster response = %d %s", missing.Code, missing.Body.String())
	}

	selected := loginRequest(t, authHandler, `{"username":"alice","password":"pvmss-alice","cluster":"secondary"}`)
	if selected.Code != http.StatusOK {
		t.Fatalf("selected login status = %d: %s", selected.Code, selected.Body.String())
	}
	var identity auth.Identity
	if err := json.Unmarshal(selected.Body.Bytes(), &identity); err != nil {
		t.Fatalf("decode identity: %v", err)
	}
	if identity.Cluster != crossSecondaryCluster {
		t.Fatalf("identity cluster = %q, want secondary", identity.Cluster)
	}
	me := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/me", nil)
	me.AddCookie(selected.Result().Cookies()[0])
	resolved, err := authHandler.Principal(me)
	if err != nil {
		t.Fatalf("resolve persisted session: %v", err)
	}
	if resolved.Cluster != crossSecondaryCluster {
		t.Fatalf("persisted session cluster = %q, want secondary", resolved.Cluster)
	}
}

func loginRequest(t *testing.T, handler *httpapi.Auth, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.Login(response, request)
	return response
}
