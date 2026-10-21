package httpapi_test

import (
	"encoding/json"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"strings"
	"testing"
)

// TestAuth_Login_PVEAdminIdentity locks the cluster-admin path: a Proxmox
// account whose ACL grants admin rights (PVEAdmin propagates
// Permissions.Modify on "/") signs in with no pool and isAdmin true. This is
// the credential every PVMSS operator uses from the pve or pam realm.
func TestAuth_Login_PVEAdminIdentity(t *testing.T) {
	t.Parallel()

	handler := newAuthHandler(t)

	response := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"admin","password":"pvmss-admin"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}

	var identity auth.Identity
	if err := json.Unmarshal(response.Body.Bytes(), &identity); err != nil {
		t.Fatalf("decode identity: %v", err)
	}

	if identity.Username != cluster.FakeUserAdmin || !identity.IsAdmin || identity.Pool != "" {
		t.Fatalf("identity = %+v, want cluster admin with no pool", identity)
	}

	if identity.Provider != auth.ProviderPVE || identity.IsOwner {
		t.Fatalf("provider = %q owner = %v, want pve and not owner", identity.Provider, identity.IsOwner)
	}
}

func TestAuth_Login_RealmSelection(t *testing.T) {
	t.Parallel()

	handler := newAuthHandler(t)

	// The selected realm is applied to a bare username: the fake has no
	// pam account, so the pve credentials must not authenticate as pam.
	pam := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice","realm":"pam"}`)
	if pam.Code != http.StatusUnauthorized {
		t.Fatalf("pam login status = %d, want 401", pam.Code)
	}

	pve := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice","realm":"pve"}`)
	if pve.Code != http.StatusOK {
		t.Fatalf("pve login status = %d, want 200: %s", pve.Code, pve.Body.String())
	}

	// An explicitly typed realm is respected and must agree with the picker.
	conflict := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice@pve","password":"pvmss-alice","realm":"pam"}`)
	if conflict.Code != http.StatusBadRequest {
		t.Fatalf("conflicting realm status = %d, want 400: %s", conflict.Code, conflict.Body.String())
	}

	// OIDC is not a selectable realm.
	oidc := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice","realm":"oidc"}`)
	if oidc.Code != http.StatusBadRequest || !strings.Contains(oidc.Body.String(), "realm_unsupported") {
		t.Fatalf("oidc realm = %d %s, want 400 realm_unsupported", oidc.Code, oidc.Body.String())
	}
}
