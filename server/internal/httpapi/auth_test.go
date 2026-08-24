//nolint:noctx // test scaffolding does not need real context
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

	"golang.org/x/crypto/bcrypt"
)

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuth_LoginPVE_StoresSession(t *testing.T) {
	handler := newAuthHandler(t)

	response := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	if len(response.Result().Cookies()) != 2 {
		t.Fatal("expected session and csrf cookies")
	}

	var got auth.Identity
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got != (auth.Identity{Username: cluster.FakeUserAlice, DisplayName: "alice", Pool: cluster.FakePoolAlice}) {
		t.Fatalf("identity = %+v", got)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuth_AdminLogin_StoresAdminSession(t *testing.T) {
	handler := newAuthHandler(t)

	response := serveJSON(handler.AdminLogin, "/api/v1/auth/admin-login", `{"password":"pvmss-local-admin"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var got auth.Identity
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got != (auth.Identity{Username: "admin", DisplayName: "admin", IsAdmin: true}) {
		t.Fatalf("identity = %+v", got)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuth_Me_RefreshesClusterDisplayName(t *testing.T) {
	handler, st := newAuthHandlerWithStore(t)

	login := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d: %s", login.Code, http.StatusOK, login.Body.String())
	}

	// The fake seed should already have set a display name, but simulate an
	// old session by clearing it and then updating the row after login.
	ctx := context.Background()
	if err := st.SetClusterDisplayName(ctx, "default", ""); err != nil {
		t.Fatalf("clear DisplayName: %v", err)
	}

	cookie := login.Result().Cookies()[0]
	meRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meRequest.AddCookie(cookie)

	me := httptest.NewRecorder()
	handler.Me(me, meRequest)

	if me.Code != http.StatusOK {
		t.Fatalf("me status = %d, want %d: %s", me.Code, http.StatusOK, me.Body.String())
	}

	var identity auth.Identity
	if err := json.Unmarshal(me.Body.Bytes(), &identity); err != nil {
		t.Fatalf("decode me: %v", err)
	}

	if identity.ClusterDisplayName == auditTestCluster {
		t.Fatalf("ClusterDisplayName still %q, should be refreshed from empty row fallback", identity.ClusterDisplayName)
	}

	// Now update the row with a custom display name and verify the same
	// session sees it on the next /me call, without re-login.
	if err := st.SetClusterDisplayName(ctx, "default", "Prod PVE"); err != nil {
		t.Fatalf("SetClusterDisplayName: %v", err)
	}

	meAgainRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meAgainRequest.AddCookie(cookie)

	meAgain := httptest.NewRecorder()
	handler.Me(meAgain, meAgainRequest)

	var refreshed auth.Identity
	if err := json.Unmarshal(meAgain.Body.Bytes(), &refreshed); err != nil {
		t.Fatalf("decode me again: %v", err)
	}

	if refreshed.ClusterDisplayName != "Prod PVE" {
		t.Fatalf("ClusterDisplayName = %q, want %q", refreshed.ClusterDisplayName, "Prod PVE")
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuth_MeAndLogout_RequireAndClearSession(t *testing.T) {
	handler := newAuthHandler(t)
	login := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)
	cookie := login.Result().Cookies()[0]
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	request.AddCookie(cookie)

	me := httptest.NewRecorder()
	handler.Me(me, request)

	if me.Code != http.StatusOK {
		t.Fatalf("me status = %d, want %d", me.Code, http.StatusOK)
	}

	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutRequest.AddCookie(cookie)

	logout := httptest.NewRecorder()
	handler.Logout(logout, logoutRequest)

	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want %d", logout.Code, http.StatusNoContent)
	}

	if got := logout.Result().Cookies()[0].MaxAge; got != -1 {
		t.Fatalf("logout cookie MaxAge = %d, want -1", got)
	}
}

// Regresses T02's original stateless signed-cookie session, which stayed
// valid after logout until its embedded expiry. The session must now be
// revoked server-side, so replaying the exact same cookie after logout fails.
//
//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuth_Logout_RevokesSessionServerSide(t *testing.T) {
	handler := newAuthHandler(t)
	login := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)
	cookie := login.Result().Cookies()[0]

	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutRequest.AddCookie(cookie)
	handler.Logout(httptest.NewRecorder(), logoutRequest)

	replay := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	replay.AddCookie(cookie)

	if _, err := handler.Principal(replay); err == nil {
		t.Fatal("expected revoked session cookie to be rejected on replay")
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuth_CreateToken_ResolvesBearerPrincipal(t *testing.T) {
	handler := newAuthHandler(t)
	login := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(`{"label":"automation","scope":"read"}`))
	request.AddCookie(login.Result().Cookies()[0])

	response := httptest.NewRecorder()
	handler.CreateToken(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusCreated)
	}

	var created struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode token response: %v", err)
	}

	bearer := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/nodes", nil)
	bearer.Header.Set("Authorization", "Bearer "+created.Value)

	identity, err := handler.Principal(bearer)
	if err != nil {
		t.Fatalf("Principal: %v", err)
	}

	if identity != (auth.Identity{Username: "alice@pve", DisplayName: "alice", Pool: "pool-alice"}) {
		t.Fatalf("identity = %+v", identity)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuth_ListTokens_OmitsValue(t *testing.T) {
	handler := newAuthHandler(t)
	login := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)
	cookie := login.Result().Cookies()[0]

	create := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(`{"label":"automation","scope":"read"}`))
	create.AddCookie(cookie)

	createResponse := httptest.NewRecorder()
	handler.CreateToken(createResponse, create)

	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createResponse.Code, http.StatusCreated)
	}

	list := httptest.NewRequest(http.MethodGet, "/api/v1/auth/tokens", nil)
	list.AddCookie(cookie)

	listResponse := httptest.NewRecorder()
	handler.ListTokens(listResponse, list)

	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listResponse.Code, http.StatusOK)
	}

	if strings.Contains(listResponse.Body.String(), "value") {
		t.Fatalf("token list leaks a value field: %s", listResponse.Body.String())
	}

	var got struct {
		Tokens []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode token list: %v", err)
	}

	if len(got.Tokens) != 1 || got.Tokens[0].Label != "automation" {
		t.Fatalf("tokens = %+v", got.Tokens)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuth_RevokeToken_StopsBearerAuthAndIsIdempotent(t *testing.T) {
	handler := newAuthHandler(t)
	login := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)
	cookie := login.Result().Cookies()[0]

	create := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(`{"label":"automation","scope":"read"}`))
	create.AddCookie(cookie)

	createResponse := httptest.NewRecorder()
	handler.CreateToken(createResponse, create)

	var created struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode token response: %v", err)
	}

	revoke := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/tokens/"+created.ID, nil)
	revoke.SetPathValue("id", created.ID)
	revoke.AddCookie(cookie)

	revokeResponse := httptest.NewRecorder()
	handler.RevokeToken(revokeResponse, revoke)

	if revokeResponse.Code != http.StatusNoContent {
		t.Fatalf("revoke status = %d, want %d", revokeResponse.Code, http.StatusNoContent)
	}

	bearer := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/nodes", nil)
	bearer.Header.Set("Authorization", "Bearer "+created.Value)

	if _, err := handler.Principal(bearer); err == nil {
		t.Fatal("expected revoked token to be rejected")
	}

	again := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/tokens/"+created.ID, nil)
	again.SetPathValue("id", created.ID)
	again.AddCookie(cookie)

	againResponse := httptest.NewRecorder()
	handler.RevokeToken(againResponse, again)

	if againResponse.Code != http.StatusNotFound {
		t.Fatalf("re-revoke status = %d, want %d", againResponse.Code, http.StatusNotFound)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuth_ChangePassword_AllowsLoginWithNewPasswordOnly(t *testing.T) {
	handler := newAuthHandler(t)
	login := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)
	cookie := login.Result().Cookies()[0]

	change := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password", strings.NewReader(`{"oldPassword":"pvmss-alice","newPassword":"new-alice-password"}`))
	change.AddCookie(cookie)

	changeResponse := httptest.NewRecorder()
	handler.ChangePassword(changeResponse, change)

	if changeResponse.Code != http.StatusNoContent {
		t.Fatalf("change status = %d, want %d", changeResponse.Code, http.StatusNoContent)
	}

	t.Cleanup(func() {
		restore := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password", strings.NewReader(`{"oldPassword":"new-alice-password","newPassword":"pvmss-alice"}`))
		restore.AddCookie(cookie)
		handler.ChangePassword(httptest.NewRecorder(), restore)
	})

	oldPassword := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)
	if oldPassword.Code != http.StatusUnauthorized {
		t.Fatalf("old password status = %d, want %d", oldPassword.Code, http.StatusUnauthorized)
	}

	newPassword := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"new-alice-password"}`)
	if newPassword.Code != http.StatusOK {
		t.Fatalf("new password status = %d, want %d", newPassword.Code, http.StatusOK)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuth_ChangePassword_RejectsShortPassword(t *testing.T) {
	handler := newAuthHandler(t)
	login := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice"}`)
	cookie := login.Result().Cookies()[0]

	change := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password", strings.NewReader(`{"oldPassword":"pvmss-alice","newPassword":"short"}`))
	change.AddCookie(cookie)

	changeResponse := httptest.NewRecorder()
	handler.ChangePassword(changeResponse, change)

	if changeResponse.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", changeResponse.Code, http.StatusBadRequest)
	}
}

//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestAuth_LoginRejectsInvalidCredentials(t *testing.T) {
	handler := newAuthHandler(t)

	response := serveJSON(handler.Login, "/api/v1/auth/login", `{"username":"alice","password":"wrong"}`)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}

	if strings.Contains(response.Body.String(), "wrong") {
		t.Fatalf("response leaks credential detail: %s", response.Body.String())
	}
}

func newAuthHandler(t *testing.T) *httpapi.Auth {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte("pvmss-local-admin"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	sessions, err := auth.NewSessionManager(newSessionRepository(), "a-session-secret-with-at-least-thirty-two-bytes", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	return httpapi.NewAuth(cluster.Fake{}, sessions, string(hash), auth.NewTokenService(newTokenRepository()), logger)
}

func newAuthHandlerWithStore(t *testing.T) (*httpapi.Auth, *store.Store) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "auth.db")
	cfg := config.Configuration{DBPath: dbPath, SessionSecret: "a-session-secret-with-at-least-thirty-two-bytes", ClusterSource: cluster.SourceFake}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	hash, err := bcrypt.GenerateFromPassword([]byte("pvmss-local-admin"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	sessions, err := auth.NewSessionManager(st, "a-session-secret-with-at-least-thirty-two-bytes", false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	registry, err := cluster.NewRegistry("fake", []store.ClusterRow{{Name: auditTestCluster}})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	return httpapi.NewAuthWithRegistry(registry, st, sessions, string(hash), auth.NewTokenService(newTokenRepository()), logger), st
}

func serveJSON(handler http.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	handler(response, request)

	return response
}

type testWriter struct {
	t *testing.T
}

func (w testWriter) Write(bytes []byte) (int, error) {
	w.t.Log(string(bytes))
	return len(bytes), nil
}
