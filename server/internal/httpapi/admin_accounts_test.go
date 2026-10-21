//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/store"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Local account fixture names, repeated across the suite.
const (
	accountsTestUsername  = "albert"
	testBootstrapUsername = "admin"
	accountsTestCreatedBy = testBootstrapUsername
)

func newAccountsFixture(t *testing.T, maxAccounts int) (*httpapi.Auth, *httpapi.AdminAccounts, *store.Store) {
	t.Helper()

	authHandler, st := newAuthHandlerWithStore(t)
	accounts := httpapi.NewAdminAccounts(authHandler, st, maxAccounts, slog.New(slog.NewTextHandler(testWriter{t}, nil)))

	return authHandler, accounts, st
}

func ownerCookie(t *testing.T, authHandler *httpapi.Auth) *http.Cookie {
	t.Helper()

	login := serveJSON(authHandler.AdminLogin, "/api/v1/auth/admin-login", `{"password":"pvmss-local-admin"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("owner login status = %d: %s", login.Code, login.Body.String())
	}

	return login.Result().Cookies()[0]
}

func createLocalAccount(t *testing.T, accounts *httpapi.AdminAccounts, cookie *http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)

	response := httptest.NewRecorder()
	accounts.ServeCreate(response, request)

	return response
}

func loginLocal(t *testing.T, authHandler *httpapi.Auth, body string) *httptest.ResponseRecorder {
	t.Helper()

	return serveJSON(authHandler.AdminLogin, "/api/v1/auth/admin-login", body)
}

//nolint:gocyclo,funlen // one end-to-end owner lifecycle keeps fixture setup linear
func TestAdminAccounts_OwnerLifecycle(t *testing.T) {
	t.Parallel()

	authHandler, accounts, st := newAccountsFixture(t, 5)
	owner := ownerCookie(t, authHandler)

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts", nil)
	listRequest.AddCookie(owner)

	list := httptest.NewRecorder()
	accounts.ServeList(list, listRequest)

	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", list.Code, list.Body.String())
	}

	var listing struct {
		Accounts []struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
		} `json:"accounts"`
		Max   int `json:"max"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listing); err != nil {
		t.Fatalf("decode list: %v", err)
	}

	if listing.Max != 5 || listing.Count != 0 || len(listing.Accounts) != 0 {
		t.Fatalf("empty listing = %+v", listing)
	}

	created := createLocalAccount(t, accounts, owner, `{"username":"Albert","password":"s3cret-passphrase"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", created.Code, created.Body.String())
	}

	var account struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		CreatedBy string `json:"createdBy"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &account); err != nil {
		t.Fatalf("decode created: %v", err)
	}

	if account.Username != accountsTestUsername || account.CreatedBy != accountsTestCreatedBy {
		t.Fatalf("created account = %+v, want albert created by admin", account)
	}

	if account.ID == 0 {
		t.Fatal("created account has no id")
	}

	duplicate := createLocalAccount(t, accounts, owner, `{"username":"albert","password":"another-passphrase"}`)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want 409: %s", duplicate.Code, duplicate.Body.String())
	}

	login := loginLocal(t, authHandler, `{"username":"albert","password":"s3cret-passphrase"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("local login status = %d: %s", login.Code, login.Body.String())
	}

	var identity auth.Identity
	if err := json.Unmarshal(login.Body.Bytes(), &identity); err != nil {
		t.Fatalf("decode identity: %v", err)
	}

	if identity.Username != accountsTestUsername || identity.Provider != auth.ProviderLocal || !identity.IsAdmin || identity.IsOwner {
		t.Fatalf("local identity = %+v", identity)
	}

	sessionCookie := login.Result().Cookies()[0]

	// Reset password: old secret dies, old sessions are revoked.
	resetRequest := httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/"+itoa(account.ID)+"/password", strings.NewReader(`{"password":"brand-new-passphrase"}`))
	resetRequest.Header.Set("Content-Type", "application/json")
	resetRequest.AddCookie(owner)
	resetRequest.SetPathValue("id", itoa(account.ID))

	reset := httptest.NewRecorder()
	accounts.ServeResetPassword(reset, resetRequest)

	if reset.Code != http.StatusNoContent {
		t.Fatalf("reset status = %d: %s", reset.Code, reset.Body.String())
	}

	if oldLogin := loginLocal(t, authHandler, `{"username":"albert","password":"s3cret-passphrase"}`); oldLogin.Code != http.StatusUnauthorized {
		t.Fatalf("old password login status = %d, want 401", oldLogin.Code)
	}

	if newLogin := loginLocal(t, authHandler, `{"username":"albert","password":"brand-new-passphrase"}`); newLogin.Code != http.StatusOK {
		t.Fatalf("new password login status = %d: %s", newLogin.Code, newLogin.Body.String())
	}

	revoked := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	revoked.AddCookie(sessionCookie)

	if _, err := authHandler.Principal(revoked); err == nil {
		t.Error("session created before the password reset still resolves")
	}

	// Disable: login refused and sessions revoked.
	toggleRequest := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/"+itoa(account.ID)+"/toggle", strings.NewReader(`{"disabled":true}`))
	toggleRequest.Header.Set("Content-Type", "application/json")
	toggleRequest.AddCookie(owner)
	toggleRequest.SetPathValue("id", itoa(account.ID))

	toggle := httptest.NewRecorder()
	accounts.ServeToggle(toggle, toggleRequest)

	if toggle.Code != http.StatusNoContent {
		t.Fatalf("toggle status = %d: %s", toggle.Code, toggle.Body.String())
	}

	if disabledLogin := loginLocal(t, authHandler, `{"username":"albert","password":"brand-new-passphrase"}`); disabledLogin.Code != http.StatusUnauthorized {
		t.Fatalf("disabled login status = %d, want 401", disabledLogin.Code)
	}

	// Enable: login works again.
	enableRequest := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/"+itoa(account.ID)+"/toggle", strings.NewReader(`{"disabled":false}`))
	enableRequest.Header.Set("Content-Type", "application/json")
	enableRequest.AddCookie(owner)
	enableRequest.SetPathValue("id", itoa(account.ID))

	enable := httptest.NewRecorder()
	accounts.ServeToggle(enable, enableRequest)

	if enable.Code != http.StatusNoContent {
		t.Fatalf("enable status = %d: %s", enable.Code, enable.Body.String())
	}

	if enabledLogin := loginLocal(t, authHandler, `{"username":"albert","password":"brand-new-passphrase"}`); enabledLogin.Code != http.StatusOK {
		t.Fatalf("enabled login status = %d: %s", enabledLogin.Code, enabledLogin.Body.String())
	}

	// Delete: gone from the list and unable to log in.
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/accounts/"+itoa(account.ID), nil)
	deleteRequest.AddCookie(owner)
	deleteRequest.SetPathValue("id", itoa(account.ID))

	deleted := httptest.NewRecorder()
	accounts.ServeDelete(deleted, deleteRequest)

	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d: %s", deleted.Code, deleted.Body.String())
	}

	if deletedLogin := loginLocal(t, authHandler, `{"username":"albert","password":"brand-new-passphrase"}`); deletedLogin.Code != http.StatusUnauthorized {
		t.Fatalf("deleted login status = %d, want 401", deletedLogin.Code)
	}

	entries, err := st.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	wantActions := map[string]bool{
		"admin.accounts.create":         false,
		"admin.accounts.password_reset": false,
		"admin.accounts.disable":        false,
		"admin.accounts.enable":         false,
		"admin.accounts.delete":         false,
	}

	for _, entry := range entries {
		if _, ok := wantActions[entry.Action]; ok {
			wantActions[entry.Action] = true
		}
	}

	for action, seen := range wantActions {
		if !seen {
			t.Errorf("audit action %q not recorded", action)
		}
	}
}

func TestAdminAccounts_NonOwnerForbidden(t *testing.T) {
	t.Parallel()

	authHandler, accounts, _ := newAccountsFixture(t, 5)
	owner := ownerCookie(t, authHandler)

	created := createLocalAccount(t, accounts, owner, `{"username":"albert","password":"s3cret-passphrase"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", created.Code, created.Body.String())
	}

	login := loginLocal(t, authHandler, `{"username":"albert","password":"s3cret-passphrase"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d: %s", login.Code, login.Body.String())
	}

	cookie := login.Result().Cookies()[0]

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts", nil)
	listRequest.AddCookie(cookie)

	list := httptest.NewRecorder()
	accounts.ServeList(list, listRequest)

	if list.Code != http.StatusForbidden {
		t.Fatalf("non-owner list status = %d, want 403: %s", list.Code, list.Body.String())
	}

	if !strings.Contains(list.Body.String(), "owner_only") {
		t.Fatalf("non-owner error = %s, want owner_only code", list.Body.String())
	}

	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts", strings.NewReader(`{"username":"other","password":"another-passphrase"}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)

	createResponse := httptest.NewRecorder()
	accounts.ServeCreate(createResponse, createRequest)

	if createResponse.Code != http.StatusForbidden {
		t.Fatalf("non-owner create status = %d, want 403", createResponse.Code)
	}
}

func TestAdminAccounts_ValidationAndCap(t *testing.T) {
	t.Parallel()

	authHandler, accounts, _ := newAccountsFixture(t, 1)
	owner := ownerCookie(t, authHandler)

	cases := []struct {
		name string
		body string
		code int
		icon string
	}{
		{name: "reserved admin", body: `{"username":"admin","password":"s3cret-passphrase"}`, code: http.StatusBadRequest, icon: "invalid_username"},
		{name: "realm suffix", body: `{"username":"alice@pve","password":"s3cret-passphrase"}`, code: http.StatusBadRequest, icon: "invalid_username"},
		{name: "too short", body: `{"username":"ab","password":"s3cret-passphrase"}`, code: http.StatusBadRequest, icon: "invalid_username"},
		{name: "short password", body: `{"username":"albert","password":"short"}`, code: http.StatusBadRequest, icon: "weak_password"},
		{name: "long password", body: `{"username":"albert","password":"` + strings.Repeat("a", 73) + `"}`, code: http.StatusBadRequest, icon: "weak_password"},
	}

	//nolint:paralleltest // subtests share one store fixture and only exercise validation
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := createLocalAccount(t, accounts, owner, tc.body)
			if response.Code != tc.code {
				t.Fatalf("status = %d, want %d: %s", response.Code, tc.code, response.Body.String())
			}

			if !strings.Contains(response.Body.String(), tc.icon) {
				t.Errorf("body = %s, want code %q", response.Body.String(), tc.icon)
			}
		})
	}

	first := createLocalAccount(t, accounts, owner, `{"username":"albert","password":"s3cret-passphrase"}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("first create status = %d: %s", first.Code, first.Body.String())
	}

	second := createLocalAccount(t, accounts, owner, `{"username":"beatrice","password":"s3cret-passphrase"}`)
	if second.Code != http.StatusConflict || !strings.Contains(second.Body.String(), "account_limit_reached") {
		t.Fatalf("cap response = %d %s, want 409 account_limit_reached", second.Code, second.Body.String())
	}
}

func TestAdminAccounts_BadPathAndMissingAccount(t *testing.T) {
	t.Parallel()

	authHandler, accounts, _ := newAccountsFixture(t, 5)
	owner := ownerCookie(t, authHandler)

	for _, id := range []string{"abc", "0", "-1"} {
		request := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/accounts/"+id, nil)
		request.AddCookie(owner)
		request.SetPathValue("id", id)

		response := httptest.NewRecorder()
		accounts.ServeDelete(response, request)

		if response.Code != http.StatusBadRequest {
			t.Errorf("id %q status = %d, want 400", id, response.Code)
		}
	}

	missing := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/accounts/999", nil)
	missing.AddCookie(owner)
	missing.SetPathValue("id", "999")

	response := httptest.NewRecorder()
	accounts.ServeDelete(response, missing)

	if response.Code != http.StatusNotFound {
		t.Fatalf("missing account status = %d, want 404: %s", response.Code, response.Body.String())
	}
}

func TestAdminAccounts_LocalLockout(t *testing.T) {
	t.Parallel()

	authHandler, accounts, st := newAccountsFixture(t, 5)
	owner := ownerCookie(t, authHandler)

	created := createLocalAccount(t, accounts, owner, `{"username":"albert","password":"s3cret-passphrase"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", created.Code, created.Body.String())
	}

	for range 5 {
		response := loginLocal(t, authHandler, `{"username":"albert","password":"wrong-password"}`)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("failure response = %d, want 401", response.Code)
		}
	}

	account, err := st.LocalAccountByUsername(context.Background(), accountsTestUsername)
	if err != nil {
		t.Fatalf("LocalAccountByUsername: %v", err)
	}

	if !account.Locked(time.Now()) {
		t.Fatal("account should be locked after five failures")
	}

	locked := loginLocal(t, authHandler, `{"username":"albert","password":"s3cret-passphrase"}`)
	if locked.Code != http.StatusUnauthorized {
		t.Fatalf("locked login status = %d, want 401", locked.Code)
	}

	// A password reset is the documented recovery path and clears the lock.
	resetRequest := httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/"+itoa(account.ID)+"/password", strings.NewReader(`{"password":"recovered-passphrase"}`))
	resetRequest.Header.Set("Content-Type", "application/json")
	resetRequest.AddCookie(owner)
	resetRequest.SetPathValue("id", itoa(account.ID))

	reset := httptest.NewRecorder()
	accounts.ServeResetPassword(reset, resetRequest)

	if reset.Code != http.StatusNoContent {
		t.Fatalf("reset status = %d: %s", reset.Code, reset.Body.String())
	}

	if recovered := loginLocal(t, authHandler, `{"username":"albert","password":"recovered-passphrase"}`); recovered.Code != http.StatusOK {
		t.Fatalf("recovered login status = %d: %s", recovered.Code, recovered.Body.String())
	}
}

func TestAuth_ChangePassword_LocalAccount(t *testing.T) {
	t.Parallel()

	authHandler, accounts, _ := newAccountsFixture(t, 5)
	owner := ownerCookie(t, authHandler)

	created := createLocalAccount(t, accounts, owner, `{"username":"albert","password":"s3cret-passphrase"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", created.Code, created.Body.String())
	}

	login := loginLocal(t, authHandler, `{"username":"albert","password":"s3cret-passphrase"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d: %s", login.Code, login.Body.String())
	}

	cookie := login.Result().Cookies()[0]

	change := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password", strings.NewReader(`{"oldPassword":"s3cret-passphrase","newPassword":"rotated-passphrase"}`))
	change.Header.Set("Content-Type", "application/json")
	change.AddCookie(cookie)

	response := httptest.NewRecorder()
	authHandler.ChangePassword(response, change)

	if response.Code != http.StatusNoContent {
		t.Fatalf("change status = %d: %s", response.Code, response.Body.String())
	}

	if old := loginLocal(t, authHandler, `{"username":"albert","password":"s3cret-passphrase"}`); old.Code != http.StatusUnauthorized {
		t.Errorf("old password still works: %d", old.Code)
	}

	if rotated := loginLocal(t, authHandler, `{"username":"albert","password":"rotated-passphrase"}`); rotated.Code != http.StatusOK {
		t.Errorf("new password rejected: %d %s", rotated.Code, rotated.Body.String())
	}

	wrong := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password", strings.NewReader(`{"oldPassword":"nope","newPassword":"another-passphrase"}`))
	wrong.Header.Set("Content-Type", "application/json")
	wrong.AddCookie(cookie)

	wrongResponse := httptest.NewRecorder()
	authHandler.ChangePassword(wrongResponse, wrong)

	if wrongResponse.Code != http.StatusUnauthorized {
		t.Errorf("wrong old password status = %d, want 401", wrongResponse.Code)
	}
}

func TestAuth_ChangePassword_BootstrapRefused(t *testing.T) {
	t.Parallel()

	authHandler, _ := newAuthHandlerWithStore(t)
	owner := ownerCookie(t, authHandler)

	change := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password", strings.NewReader(`{"oldPassword":"pvmss-local-admin","newPassword":"rotated-passphrase"}`))
	change.Header.Set("Content-Type", "application/json")
	change.AddCookie(owner)

	response := httptest.NewRecorder()
	authHandler.ChangePassword(response, change)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("bootstrap change status = %d, want 400: %s", response.Code, response.Body.String())
	}
}

func TestAuth_AdminLogin_UnknownUserIsGeneric(t *testing.T) {
	t.Parallel()

	authHandler, _ := newAuthHandlerWithStore(t)

	response := loginLocal(t, authHandler, `{"username":"ghost","password":"whatever-passphrase"}`)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unknown user status = %d, want 401", response.Code)
	}

	body := response.Body.String()
	if !strings.Contains(body, "invalid_credentials") {
		t.Fatalf("body = %s, want invalid_credentials", body)
	}

	if strings.Contains(strings.ToLower(body), "unknown") || strings.Contains(strings.ToLower(body), "not found") {
		t.Fatalf("response leaks account existence: %s", body)
	}
}

func TestAdminAccounts_PasswordHashesAreBcrypt(t *testing.T) {
	t.Parallel()

	authHandler, accounts, st := newAccountsFixture(t, 5)
	owner := ownerCookie(t, authHandler)

	created := createLocalAccount(t, accounts, owner, `{"username":"albert","password":"s3cret-passphrase"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", created.Code, created.Body.String())
	}

	account, err := st.LocalAccountByUsername(context.Background(), accountsTestUsername)
	if err != nil {
		t.Fatalf("LocalAccountByUsername: %v", err)
	}

	if !strings.HasPrefix(account.PasswordHash, "$2") {
		t.Fatalf("hash = %q, want bcrypt", account.PasswordHash)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte("s3cret-passphrase")); err != nil {
		t.Fatalf("stored hash does not match password: %v", err)
	}
}

func itoa(value int64) string {
	return strconv.FormatInt(value, 10)
}
