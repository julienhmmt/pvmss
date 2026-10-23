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

func assertLocalLoginStatus(t *testing.T, authHandler *httpapi.Auth, body string, wantCode int) {
	t.Helper()

	response := loginLocal(t, authHandler, body)
	if response.Code != wantCode {
		t.Fatalf("local login status = %d, want %d: %s", response.Code, wantCode, response.Body.String())
	}
}

func serveOwnerAccountAction(t *testing.T, handler http.HandlerFunc, method, path string, id int64, owner *http.Cookie, body string) {
	t.Helper()

	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}

	request.AddCookie(owner)
	request.SetPathValue("id", itoa(id))

	response := httptest.NewRecorder()
	handler(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("%s %s status = %d, want 204: %s", method, path, response.Code, response.Body.String())
	}
}

func assertEmptyAccountList(t *testing.T, accounts *httpapi.AdminAccounts, owner *http.Cookie, wantMax int) {
	t.Helper()

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

	if listing.Max != wantMax || listing.Count != 0 || len(listing.Accounts) != 0 {
		t.Fatalf("empty listing = %+v", listing)
	}
}

func mustCreateLocalAccount(t *testing.T, accounts *httpapi.AdminAccounts, owner *http.Cookie, body string) int64 {
	t.Helper()

	created := createLocalAccount(t, accounts, owner, body)
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

	if account.Username != accountsTestUsername || account.CreatedBy != accountsTestCreatedBy || account.ID == 0 {
		t.Fatalf("created account = %+v, want non-zero id and albert created by admin", account)
	}

	return account.ID
}

func mustLoginLocal(t *testing.T, authHandler *httpapi.Auth, body string) *http.Cookie {
	t.Helper()

	login := loginLocal(t, authHandler, body)
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

	return login.Result().Cookies()[0]
}

func assertAuditActions(t *testing.T, st *store.Store, actions ...string) {
	t.Helper()

	entries, err := st.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		seen[entry.Action] = true
	}

	for _, action := range actions {
		if !seen[action] {
			t.Errorf("audit action %q not recorded", action)
		}
	}
}

func TestAdminAccounts_OwnerLifecycle(t *testing.T) {
	t.Parallel()

	authHandler, accounts, st := newAccountsFixture(t, 5)
	owner := ownerCookie(t, authHandler)

	assertEmptyAccountList(t, accounts, owner, 5)
	accountID := mustCreateLocalAccount(t, accounts, owner, `{"username":"Albert","password":"s3cret-passphrase"}`)

	duplicate := createLocalAccount(t, accounts, owner, `{"username":"albert","password":"another-passphrase"}`)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want 409: %s", duplicate.Code, duplicate.Body.String())
	}

	sessionCookie := mustLoginLocal(t, authHandler, `{"username":"albert","password":"s3cret-passphrase"}`)

	// Reset password: old secret dies, old sessions are revoked.
	serveOwnerAccountAction(t, accounts.ServeResetPassword, http.MethodPut, "/api/v1/admin/accounts/"+itoa(accountID)+"/password", accountID, owner, `{"password":"brand-new-passphrase"}`)
	assertLocalLoginStatus(t, authHandler, `{"username":"albert","password":"s3cret-passphrase"}`, http.StatusUnauthorized)
	assertLocalLoginStatus(t, authHandler, `{"username":"albert","password":"brand-new-passphrase"}`, http.StatusOK)

	revoked := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	revoked.AddCookie(sessionCookie)

	if _, err := authHandler.Principal(revoked); err == nil {
		t.Error("session created before the password reset still resolves")
	}

	// Disable: login refused and sessions revoked.
	serveOwnerAccountAction(t, accounts.ServeToggle, http.MethodPost, "/api/v1/admin/accounts/"+itoa(accountID)+"/toggle", accountID, owner, `{"disabled":true}`)
	assertLocalLoginStatus(t, authHandler, `{"username":"albert","password":"brand-new-passphrase"}`, http.StatusUnauthorized)

	// Enable: login works again.
	serveOwnerAccountAction(t, accounts.ServeToggle, http.MethodPost, "/api/v1/admin/accounts/"+itoa(accountID)+"/toggle", accountID, owner, `{"disabled":false}`)
	assertLocalLoginStatus(t, authHandler, `{"username":"albert","password":"brand-new-passphrase"}`, http.StatusOK)

	// Delete: gone from the list and unable to log in.
	serveOwnerAccountAction(t, accounts.ServeDelete, http.MethodDelete, "/api/v1/admin/accounts/"+itoa(accountID), accountID, owner, "")
	assertLocalLoginStatus(t, authHandler, `{"username":"albert","password":"brand-new-passphrase"}`, http.StatusUnauthorized)

	assertAuditActions(t, st,
		"admin.accounts.create",
		"admin.accounts.password_reset",
		"admin.accounts.disable",
		"admin.accounts.enable",
		"admin.accounts.delete",
	)
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

func TestAdminAccounts_ListPopulatedAndEdgeErrors(t *testing.T) {
	t.Parallel()

	authHandler, accounts, _ := newAccountsFixture(t, 5)
	accounts.SetTrustedProxyHops(1)

	owner := ownerCookie(t, authHandler)
	accountID := mustCreateLocalAccount(t, accounts, owner, `{"username":"albert","password":"s3cret-passphrase"}`)

	assertLocalLoginStatus(t, authHandler, `{"username":"albert","password":"s3cret-passphrase"}`, http.StatusOK)

	for range 5 {
		assertLocalLoginStatus(t, authHandler, `{"username":"albert","password":"wrong-password"}`, http.StatusUnauthorized)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts", nil)
	listReq.AddCookie(owner)

	listResp := httptest.NewRecorder()
	accounts.ServeList(listResp, listReq)

	if listResp.Code != http.StatusOK || !strings.Contains(listResp.Body.String(), "lockedUntil") || !strings.Contains(listResp.Body.String(), "lastLoginAt") {
		t.Fatalf("populated list = %d %s", listResp.Code, listResp.Body.String())
	}

	assertAccountErrorCases(t, accounts, owner, accountID)
}

func assertAccountErrorCases(t *testing.T, accounts *httpapi.AdminAccounts, owner *http.Cookie, accountID int64) {
	t.Helper()

	unauth := httptest.NewRecorder()
	accounts.ServeList(unauth, httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts", nil))

	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list status = %d, want 401", unauth.Code)
	}

	badCreate := createLocalAccount(t, accounts, owner, badJSONBody)
	if badCreate.Code != http.StatusBadRequest {
		t.Fatalf("bad create JSON status = %d, want 400", badCreate.Code)
	}

	cases := []struct {
		name    string
		handler http.HandlerFunc
		id      string
		body    string
		want    int
	}{
		{name: "reset bad id", handler: accounts.ServeResetPassword, id: "0", body: `{"password":"valid-passphrase"}`, want: http.StatusBadRequest},
		{name: "reset bad json", handler: accounts.ServeResetPassword, id: itoa(accountID), body: badJSONBody, want: http.StatusBadRequest},
		{name: "reset weak password", handler: accounts.ServeResetPassword, id: itoa(accountID), body: `{"password":"short"}`, want: http.StatusBadRequest},
		{name: "reset missing id", handler: accounts.ServeResetPassword, id: "999", body: `{"password":"valid-passphrase"}`, want: http.StatusNotFound},
		{name: "toggle bad id", handler: accounts.ServeToggle, id: "0", body: `{"disabled":true}`, want: http.StatusBadRequest},
		{name: "toggle bad json", handler: accounts.ServeToggle, id: itoa(accountID), body: badJSONBody, want: http.StatusBadRequest},
		{name: "toggle missing id", handler: accounts.ServeToggle, id: "999", body: `{"disabled":true}`, want: http.StatusNotFound},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/"+tc.id, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(owner)
		req.SetPathValue("id", tc.id)

		rec := httptest.NewRecorder()
		tc.handler(rec, req)

		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d: %s", tc.name, rec.Code, tc.want, rec.Body.String())
		}
	}
}

// TestAuth_AdminLogin_LockedAccountSpendsBcryptWork: a locked or disabled
// account answered without any bcrypt work, so its fast 401 told an attacker
// the account exists. The rejection must cost about a password comparison.
func TestAuth_AdminLogin_LockedAccountSpendsBcryptWork(t *testing.T) {
	t.Parallel()

	authHandler, accounts, _ := newAccountsFixture(t, 5)
	owner := ownerCookie(t, authHandler)
	mustCreateLocalAccount(t, accounts, owner, `{"username":"albert","password":"s3cret-passphrase"}`)

	for range 5 {
		assertLocalLoginStatus(t, authHandler, `{"username":"albert","password":"wrong-password"}`, http.StatusUnauthorized)
	}

	reference, err := bcrypt.GenerateFromPassword([]byte("reference"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}

	start := time.Now()

	_ = bcrypt.CompareHashAndPassword(reference, []byte("other"))
	oneCompare := time.Since(start)

	start = time.Now()

	assertLocalLoginStatus(t, authHandler, `{"username":"albert","password":"s3cret-passphrase"}`, http.StatusUnauthorized)

	if locked := time.Since(start); locked < oneCompare/2 {
		t.Fatalf("locked rejection took %v, want about one bcrypt comparison (%v)", locked, oneCompare)
	}
}
