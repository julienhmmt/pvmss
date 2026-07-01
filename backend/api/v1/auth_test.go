package apiv1_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	apiv1 "pvmss/api/v1"
	"pvmss/database"
	envpkg "pvmss/env"
	"pvmss/state"
)

const testJWTSecret = "test-jwt-secret-must-be-at-least-32-bytes!!"

// newAuthTestHandler builds an AuthHandler backed by a real in-memory DB in
// offline mode. adminHash (may be "") is stored as the ADMIN_PASSWORD_HASH so
// the admin bcrypt login path can be exercised. Handlers are called directly,
// bypassing JWTMiddleware; usernameFromCtx/isAdminFromCtx return zero values
// when the context key is absent.
func newAuthTestHandler(t *testing.T, adminHash string) (*apiv1.AuthHandler, state.StateManager) {
	t.Helper()
	db, err := database.OpenMemory()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.CompleteBootstrap("test"))

	sm := state.MakeAppStateWithDB(db)
	require.NoError(t, sm.LoadSettingsFromDB())
	sm.SetEnvConfig(&envpkg.EnvConfig{
		AdminPasswordHash: adminHash,
		Environment:       "development",
	})
	sm.SetOfflineMode()

	return apiv1.MakeAuthHandler(sm, testJWTSecret), sm
}

// bcryptHash returns a fast bcrypt hash of password for test fixtures.
func bcryptHash(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	return string(h)
}

// mintHS256 signs a JWTClaims token with the given HMAC secret.
func mintHS256(t *testing.T, secret, username string, isAdmin bool, ttl time.Duration) string {
	t.Helper()
	claims := apiv1.JWTClaims{
		Username: username,
		IsAdmin:  isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(secret))
	require.NoError(t, err)
	return signed
}

// mintRS256 signs a JWTClaims token with an RSA key to exercise the
// alg-confusion guard (handler only accepts *jwt.SigningMethodHMAC).
func mintRS256(t *testing.T, username string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	claims := apiv1.JWTClaims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := tok.SignedString(key)
	require.NoError(t, err)
	return signed
}

// loginBody marshals a LoginRequest to JSON.
func loginBody(t *testing.T, username, password string, admin bool) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(apiv1.LoginRequest{Username: username, Password: password, Admin: admin})
	require.NoError(t, err)
	return bytes.NewBuffer(b)
}

// cookieValue returns the value of the named Set-Cookie from a recorder, or "".
func cookieValue(rec *httptest.ResponseRecorder, name string) (string, bool) {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c.Value, true
		}
	}
	return "", false
}

// ── Login: admin (bcrypt) ─────────────────────────────────────────────────────

func TestLogin_Admin(t *testing.T) {
	const password = "correct-horse"
	hash := bcryptHash(t, password)

	tests := []struct {
		name       string
		adminHash  string
		username   string
		password   string
		wantStatus int
	}{
		{"valid credentials", hash, "admin", password, http.StatusOK},
		{"wrong password", hash, "admin", "wrong", http.StatusUnauthorized},
		{"empty username", hash, "", password, http.StatusBadRequest},
		{"empty password", hash, "admin", "", http.StatusBadRequest},
		{"hash not configured", "", "admin", password, http.StatusUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newAuthTestHandler(t, tc.adminHash)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", loginBody(t, tc.username, tc.password, true))
			w := httptest.NewRecorder()
			h.Login(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
			if tc.wantStatus == http.StatusOK {
				var resp apiv1.AuthResponse
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
				assert.Equal(t, tc.username, resp.Username)
				assert.True(t, resp.IsAdmin)
				_, ok := cookieValue(w, "access_token")
				assert.True(t, ok, "access_token cookie should be set")
				_, ok = cookieValue(w, "refresh_token")
				assert.True(t, ok, "refresh_token cookie should be set")
			}
		})
	}
}

// ── Login: Proxmox user path (offline) ────────────────────────────────────────

func TestLogin_ProxmoxUser_OfflineReturnsError(t *testing.T) {
	h, _ := newAuthTestHandler(t, "")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", loginBody(t, "alice", "pw", false))
	w := httptest.NewRecorder()
	h.Login(w, req)

	// Offline mode refuses non-admin (Proxmox) login before any network call.
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// ── Exchange ──────────────────────────────────────────────────────────────────

func TestExchange(t *testing.T) {
	h, _ := newAuthTestHandler(t, "")

	tests := []struct {
		name       string
		cookie     *http.Cookie
		wantStatus int
	}{
		{
			name:       "valid token",
			cookie:     &http.Cookie{Name: "access_token", Value: mintHS256(t, testJWTSecret, "alice", false, time.Hour)},
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing cookie",
			cookie:     nil,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "tampered token",
			cookie:     &http.Cookie{Name: "access_token", Value: "not.a.jwt"},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "wrong secret",
			cookie:     &http.Cookie{Name: "access_token", Value: mintHS256(t, "some-other-secret-that-is-long-enough!!", "alice", false, time.Hour)},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "wrong algorithm (RS256)",
			cookie:     &http.Cookie{Name: "access_token", Value: mintRS256(t, "alice")},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "expired token",
			cookie:     &http.Cookie{Name: "access_token", Value: mintHS256(t, testJWTSecret, "alice", false, -time.Hour)},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/exchange", nil)
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}
			w := httptest.NewRecorder()
			h.Exchange(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
			if tc.wantStatus == http.StatusOK {
				var resp apiv1.AuthResponse
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
				assert.Equal(t, "alice", resp.Username)
				assert.False(t, resp.IsAdmin)
			}
		})
	}
}

func TestExchange_AdminClaimPreserved(t *testing.T) {
	h, _ := newAuthTestHandler(t, "")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/exchange", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: mintHS256(t, testJWTSecret, "root", true, time.Hour)})
	w := httptest.NewRecorder()
	h.Exchange(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp apiv1.AuthResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.IsAdmin)
}

// ── Refresh ───────────────────────────────────────────────────────────────────

func TestRefresh(t *testing.T) {
	h, _ := newAuthTestHandler(t, "")

	tests := []struct {
		name       string
		cookie     *http.Cookie
		wantStatus int
	}{
		{
			name:       "valid refresh token",
			cookie:     &http.Cookie{Name: "refresh_token", Value: mintHS256(t, testJWTSecret, "alice", false, 24*time.Hour)},
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing cookie",
			cookie:     nil,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid token",
			cookie:     &http.Cookie{Name: "refresh_token", Value: "garbage"},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "expired token",
			cookie:     &http.Cookie{Name: "refresh_token", Value: mintHS256(t, testJWTSecret, "alice", false, -time.Hour)},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}
			w := httptest.NewRecorder()
			h.Refresh(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
			if tc.wantStatus == http.StatusOK {
				newAccess, ok := cookieValue(w, "access_token")
				assert.True(t, ok, "refresh should set a new access_token cookie")
				assert.NotEqual(t, tc.cookie.Value, newAccess, "new access token differs from the refresh token")
			}
		})
	}
}

// ── ProxmoxAdminLogin (offline / input validation) ────────────────────────────

func TestProxmoxAdminLogin(t *testing.T) {
	tests := []struct {
		name       string
		username   string
		password   string
		wantStatus int
	}{
		// Offline: PROXMOX_URL unset → client construction fails → 500.
		{"offline client error", "root@pve", "pw", http.StatusInternalServerError},
		{"empty username", "", "pw", http.StatusBadRequest},
		{"empty password", "root@pve", "", http.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newAuthTestHandler(t, "")
			body, err := json.Marshal(map[string]string{"username": tc.username, "password": tc.password})
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/proxmox-admin-login", bytes.NewBuffer(body))
			w := httptest.NewRecorder()
			h.ProxmoxAdminLogin(w, req)

			assert.Equal(t, tc.wantStatus, w.Code)
		})
	}
}

// ── Logout ────────────────────────────────────────────────────────────────────

func TestLogout_ClearsCookies(t *testing.T) {
	h, _ := newAuthTestHandler(t, "")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	w := httptest.NewRecorder()
	h.Logout(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	for _, name := range []string{"access_token", "refresh_token"} {
		var found *http.Cookie
		for _, c := range w.Result().Cookies() {
			if c.Name == name {
				found = c
			}
		}
		require.NotNil(t, found, "%s cookie should be present", name)
		assert.Equal(t, -1, found.MaxAge, "%s should be expired", name)
	}
}

// ── Me ────────────────────────────────────────────────────────────────────────

func TestMe_RequiresContext(t *testing.T) {
	h, _ := newAuthTestHandler(t, "")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	w := httptest.NewRecorder()
	h.Me(w, req)

	// Without JWTMiddleware injecting context, Me reports an empty, non-admin user.
	require.Equal(t, http.StatusOK, w.Code)
	var resp apiv1.MeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "", resp.Username)
	assert.False(t, resp.IsAdmin)
}
