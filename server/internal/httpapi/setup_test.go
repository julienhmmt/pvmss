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
	"time"

	"golang.org/x/crypto/bcrypt"
)

const setupTestSecret = "setup-test-secret-with-32-bytes!!"

// newSetupStore opens a database on the proxmox source with no PROXMOX_*
// credentials: zero clusters, zero local accounts - the fresh-install state.
func newSetupStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "setup.db"), ClusterSource: "proxmox", SessionSecret: setupTestSecret})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func setupStatus(t *testing.T, handler *httpapi.Setup, method string) (int, map[string]bool) {
	t.Helper()
	request := httptest.NewRequestWithContext(context.Background(), method, "/api/v1/setup", nil)
	response := httptest.NewRecorder()
	handler.ServeStatus(response, request)
	if method != http.MethodGet {
		return response.Code, nil
	}
	var body map[string]bool
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return response.Code, body
}

func TestSetupStatus_FreshDatabaseNeedsEverything(t *testing.T) {
	handler := httpapi.NewSetup(newSetupStore(t), false, slog.Default())
	code, body := setupStatus(t, handler, http.MethodGet)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if !body["required"] || !body["needsAdmin"] || !body["needsCluster"] {
		t.Fatalf("fresh database = %+v, want all three true", body)
	}
}

func TestSetupStatus_BootstrapHashClearsNeedsAdmin(t *testing.T) {
	handler := httpapi.NewSetup(newSetupStore(t), true, slog.Default())
	_, body := setupStatus(t, handler, http.MethodGet)
	if body["needsAdmin"] {
		t.Fatalf("hasBootstrap=true still reports needsAdmin: %+v", body)
	}
	if !body["required"] || !body["needsCluster"] {
		t.Fatalf("zero clusters must still report required+needsCluster: %+v", body)
	}
}

func TestSetupStatus_ClusterAndAccountClearRequirements(t *testing.T) {
	st := newSetupStore(t)
	ctx := context.Background()
	if err := st.CreateCluster(ctx, store.ClusterRow{Name: "prod", URL: "https://pve.example.com:8006", TokenID: "pvmss@pve!svc", TokenSecret: "s"}); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}
	if _, err := st.CreateLocalAccount(ctx, store.LocalAccount{Username: "root", PasswordHash: "$2a$10$hash", CreatedAt: time.Now(), UpdatedAt: time.Now(), CreatedBy: "setup"}); err != nil {
		t.Fatalf("CreateLocalAccount: %v", err)
	}
	handler := httpapi.NewSetup(st, false, slog.Default())
	_, body := setupStatus(t, handler, http.MethodGet)
	if body["required"] || body["needsAdmin"] || body["needsCluster"] {
		t.Fatalf("configured instance = %+v, want all three false", body)
	}
}

func TestSetupStatus_RejectsNonGet(t *testing.T) {
	handler := httpapi.NewSetup(newSetupStore(t), false, slog.Default())
	code, _ := setupStatus(t, handler, http.MethodPost)
	if code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", code)
	}
}

func createSetupAdmin(t *testing.T, handler *httpapi.Setup, body string) (int, map[string]string) {
	t.Helper()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/setup/admin", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeCreateAdmin(response, request)
	var decoded map[string]string
	_ = json.Unmarshal(response.Body.Bytes(), &decoded)
	return response.Code, decoded
}

func TestSetupCreateAdmin_FirstCallCreatesSetupAccount(t *testing.T) {
	st := newSetupStore(t)
	handler := httpapi.NewSetup(st, false, slog.Default())
	code, _ := createSetupAdmin(t, handler, `{"username":"owner","password":"correct horse battery"}`)
	if code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", code)
	}
	account, err := st.LocalAccountByUsername(context.Background(), "owner")
	if err != nil {
		t.Fatalf("created account lookup: %v", err)
	}
	if account.CreatedBy != "setup" {
		t.Fatalf("created_by = %q, want setup (setup accounts carry ownership)", account.CreatedBy)
	}
}

func TestSetupCreateAdmin_ClosesOnceAnAccountExists(t *testing.T) {
	st := newSetupStore(t)
	handler := httpapi.NewSetup(st, false, slog.Default())
	if code, _ := createSetupAdmin(t, handler, `{"username":"owner","password":"correct horse battery"}`); code != http.StatusCreated {
		t.Fatalf("first call status = %d, want 201", code)
	}
	code, body := createSetupAdmin(t, handler, `{"username":"second","password":"correct horse battery"}`)
	if code != http.StatusForbidden {
		t.Fatalf("second call status = %d, want 403", code)
	}
	if body["code"] != "setup_complete" {
		t.Fatalf("error code = %q, want setup_complete", body["code"])
	}
}

func TestSetupCreateAdmin_RefusedWhenBootstrapConfigured(t *testing.T) {
	handler := httpapi.NewSetup(newSetupStore(t), true, slog.Default())
	code, body := createSetupAdmin(t, handler, `{"username":"owner","password":"correct horse battery"}`)
	if code != http.StatusForbidden || body["code"] != "setup_complete" {
		t.Fatalf("bootstrap configured: status = %d code = %q, want 403 setup_complete", code, body["code"])
	}
}

// newSetupAuth builds an Auth + AdminAccounts pair with no bootstrap hash,
// the state a fresh install runs in until the wizard creates its account.
func newSetupAuth(t *testing.T, st *store.Store, adminHash string) (*httpapi.Auth, *httpapi.AdminAccounts) {
	t.Helper()
	sessions, err := auth.NewSessionManager(st, setupTestSecret, false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}
	registry, err := cluster.NewRegistry("proxmox", nil)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	authHandler := httpapi.NewAuthWithRegistry(registry, st, sessions, adminHash, slog.Default())
	return authHandler, httpapi.NewAdminAccounts(authHandler, st, 5, slog.Default())
}

func setupAccountCookie(t *testing.T, authHandler *httpapi.Auth, st *store.Store, username, createdBy string) *http.Cookie {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse battery"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}
	now := time.Now().UTC()
	if _, err := st.CreateLocalAccount(context.Background(), store.LocalAccount{Username: username, PasswordHash: string(hash), CreatedAt: now, UpdatedAt: now, CreatedBy: createdBy}); err != nil {
		t.Fatalf("CreateLocalAccount: %v", err)
	}
	login := serveJSON(authHandler.AdminLogin, "/api/v1/auth/admin-login", `{"username":"`+username+`","password":"correct horse battery"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d: %s", login.Code, login.Body.String())
	}
	return login.Result().Cookies()[0]
}

func serveAccountList(t *testing.T, accounts *httpapi.AdminAccounts, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/admin/accounts", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	accounts.ServeList(response, request)
	return response
}

func TestAdminAccounts_SetupAccountIsOwnerWithoutBootstrap(t *testing.T) {
	st := newSetupStore(t)
	authHandler, accounts := newSetupAuth(t, st, "")
	cookie := setupAccountCookie(t, authHandler, st, "owner", "setup")
	if response := serveAccountList(t, accounts, cookie); response.Code != http.StatusOK {
		t.Fatalf("setup account list status = %d, want 200: %s", response.Code, response.Body.String())
	}
}

func TestAdminAccounts_NonSetupLocalStaysNonOwner(t *testing.T) {
	st := newSetupStore(t)
	authHandler, accounts := newSetupAuth(t, st, "")
	cookie := setupAccountCookie(t, authHandler, st, "albert", "owner")
	response := serveAccountList(t, accounts, cookie)
	if response.Code != http.StatusForbidden {
		t.Fatalf("non-setup local account status = %d, want 403", response.Code)
	}
}

func TestAdminAccounts_BootstrapWinsOverSetupAccount(t *testing.T) {
	st := newSetupStore(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("env-admin"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}
	authHandler, accounts := newSetupAuth(t, st, string(hash))
	cookie := setupAccountCookie(t, authHandler, st, "owner", "setup")
	response := serveAccountList(t, accounts, cookie)
	if response.Code != http.StatusForbidden {
		t.Fatalf("setup account with bootstrap configured status = %d, want 403", response.Code)
	}
}

// TestSetupOwner_IsOwnerInPrincipal - the session identity must carry
// isOwner for the setup account: the SPA hides the Accounts tab on it.
//
//nolint:paralleltest // serial: shared store fixture
func TestSetupOwner_IsOwnerInPrincipal(t *testing.T) {
	for name, tc := range map[string]struct {
		adminHash string
		createdBy string
		want      bool
	}{
		"setup account, no bootstrap":   {"", "setup", true},
		"non-setup local, no bootstrap": {"", "owner", false},
		"setup account, bootstrap wins": {"$2a$10$bootstrap", "setup", false},
	} {
		st := newSetupStore(t)
		authHandler, _ := newSetupAuth(t, st, tc.adminHash)
		cookie := setupAccountCookie(t, authHandler, st, "owner", tc.createdBy)
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/me", nil)
		request.AddCookie(cookie)
		identity, err := authHandler.Principal(request)
		if err != nil {
			t.Fatalf("%s: Principal: %v", name, err)
		}
		if identity.IsOwner != tc.want {
			t.Fatalf("%s: IsOwner = %v, want %v", name, identity.IsOwner, tc.want)
		}
	}
}

func TestSetupCreateAdmin_RejectsBadInput(t *testing.T) {
	handler := httpapi.NewSetup(newSetupStore(t), false, slog.Default())
	for name, body := range map[string]string{
		"reserved username": `{"username":"admin","password":"correct horse battery"}`,
		"short password":    `{"username":"owner","password":"short"}`,
		"bad username":      `{"username":"Bad User!","password":"correct horse battery"}`,
		"missing fields":    `{}`,
	} {
		if code, _ := createSetupAdmin(t, handler, body); code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", name, code)
		}
	}
}
