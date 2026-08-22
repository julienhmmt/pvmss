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
// shared by every test in this file that needs multi-cluster login/OIDC
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
	authHandler := httpapi.NewAuthWithRegistry(registry, st, sessions, "", auth.NewTokenService(st), slog.Default())
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

func oidcRequestFor(t *testing.T, handler *httpapi.Auth, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/oidc", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.OIDC(response, request)
	return response
}

// TestAuth_OIDC — T041/FR-012: OIDC sign-in on a cluster with oidcEnabled
// false or missing 404s (nothing to attempt), a missing cluster is a plain
// 400, and — the case this tranche's whole OIDC surface exists to prove —
// attempting sign-in on a cluster the admin HAS enabled OIDC for returns a
// clean 501, never a redirect or any other partial success.
//
//nolint:paralleltest // authentication fixture shares fake identities
func TestAuth_OIDC(t *testing.T) {
	authHandler, st := newClusterAuthFixture(t)

	t.Run("missing cluster is 400", func(t *testing.T) {
		response := oidcRequestFor(t, authHandler, `{}`)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400: %s", response.Code, response.Body.String())
		}
	})

	t.Run("oidc disabled is 404, never a redirect", func(t *testing.T) {
		response := oidcRequestFor(t, authHandler, `{"cluster":"default"}`)
		if response.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404: %s", response.Code, response.Body.String())
		}
		if location := response.Header().Get("Location"); location != "" {
			t.Fatalf("unexpected Location header on a disabled-OIDC response: %q", location)
		}
	})

	t.Run("oidc enabled returns 501, not a broken redirect", func(t *testing.T) {
		runOIDCEnabledCase(t, authHandler, st)
	})
}

// runOIDCEnabledCase enables OIDC on the secondary cluster, requests it
// (expecting 501 with no redirect), and verifies the default cluster is
// unaffected (still 404). Extracted from TestAuth_OIDC to keep its Cognitive
// Complexity under the SonarQube go:S3776 threshold.
func runOIDCEnabledCase(t *testing.T, authHandler *httpapi.Auth, st *store.Store) {
	t.Helper()

	if err := st.SetClusterOIDC(context.Background(), crossSecondaryCluster, true); err != nil {
		t.Fatalf("SetClusterOIDC: %v", err)
	}

	response := oidcRequestFor(t, authHandler, `{"cluster":"secondary"}`)
	if response.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501: %s", response.Code, response.Body.String())
	}

	if location := response.Header().Get("Location"); location != "" {
		t.Fatalf("unexpected Location header on the 501 response: %q", location)
	}

	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Code != "not_implemented" {
		t.Fatalf("error code = %q, want not_implemented", body.Code)
	}

	// The 501 must be scoped to the enabled cluster only — default,
	// never toggled, must still 404 (FR-011's isolation, checked from
	// the login-affordance side rather than the admin-toggle side).
	untouched := oidcRequestFor(t, authHandler, `{"cluster":"default"}`)
	if untouched.Code != http.StatusNotFound {
		t.Fatalf("default status after secondary's toggle = %d, want still 404: %s", untouched.Code, untouched.Body.String())
	}
}
