package apiv1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"pvmss/proxmox"
	"pvmss/state"

	"github.com/alexedwards/scs/v2"
	"html/template"
)

// testStateManager is a minimal StateManager for api/v1 tests.
// It returns a fixed JWTSecret from AppSettings.
type testStateManager struct {
	settings *state.AppSettings
	offline  bool
}

func newTestSM(jwtSecret string) *testStateManager {
	return &testStateManager{settings: &state.AppSettings{JWTSecret: jwtSecret}}
}

func (m *testStateManager) GetSettings() *state.AppSettings               { return m.settings }
func (m *testStateManager) IsOfflineMode() bool                           { return m.offline }
func (m *testStateManager) GetTemplates() *template.Template              { return nil }
func (m *testStateManager) SetTemplates(_ *template.Template) error       { return nil }
func (m *testStateManager) GetSessionManager() *scs.SessionManager        { return nil }
func (m *testStateManager) SetSessionManager(_ *scs.SessionManager) error { return nil }
func (m *testStateManager) StartOnlineMode() error                        { return nil }
func (m *testStateManager) SetOfflineMode()                               { m.offline = true }
func (m *testStateManager) GetProxmoxStatus() (bool, string)              { return true, "" }
func (m *testStateManager) CheckProxmoxConnection() bool                  { return true }
func (m *testStateManager) RefreshNodeCache(_ context.Context)            {}
func (m *testStateManager) GetNodeCache() ([]*proxmox.NodeDetails, time.Time) {
	return nil, time.Time{}
}
func (m *testStateManager) GetProxmoxSnapshot() *state.ProxmoxClusterSnapshot { return nil }
func (m *testStateManager) RequestSnapshotRefresh()                           {}
func (m *testStateManager) SetSettings(_ *state.AppSettings) error            { return nil }
func (m *testStateManager) SetSettingsWithoutSave(_ *state.AppSettings)       {}
func (m *testStateManager) GetTags() []string                                 { return nil }
func (m *testStateManager) GetISOs() []string                                 { return nil }
func (m *testStateManager) GetVMBRs() []string                                { return nil }
func (m *testStateManager) GetLimits() map[string]interface{}                 { return nil }
func (m *testStateManager) GetStorages() []string                             { return nil }
func (m *testStateManager) AddCSRFToken(_ string, _ time.Time) error          { return nil }
func (m *testStateManager) ValidateAndRemoveCSRFToken(_ string) bool          { return true }
func (m *testStateManager) CleanExpiredCSRFTokens()                           {}
func (m *testStateManager) GetFrontendPath() string                           { return "" }
func (m *testStateManager) SetFrontendPath(_ string)                          {}
func (m *testStateManager) SetGuestAgentCleanupFunc(_ func())                 {}

// signToken issues a signed JWT for use in tests.
func signToken(t *testing.T, secret, username string, isAdmin bool, ttl time.Duration) string {
	t.Helper()
	claims := JWTClaims{
		Username: username,
		IsAdmin:  isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("signToken: %v", err)
	}
	return signed
}

func TestJWTMiddleware_MissingCookie(t *testing.T) {
	sm := newTestSM("testsecretthatis32byteslongexact!!")
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := JWTMiddleware(sm, next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vms", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestJWTMiddleware_ValidToken(t *testing.T) {
	secret := "testsecretthatis32byteslongexact!!"
	sm := newTestSM(secret)
	signed := signToken(t, secret, "testuser", false, 15*time.Minute)

	var capturedUsername string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUsername = usernameFromCtx(r)
		w.WriteHeader(http.StatusOK)
	})
	h := JWTMiddleware(sm, next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vms", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: signed})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if capturedUsername != "testuser" {
		t.Errorf("expected username 'testuser', got %q", capturedUsername)
	}
}

func TestJWTMiddleware_ExpiredToken(t *testing.T) {
	secret := "testsecretthatis32byteslongexact!!"
	sm := newTestSM(secret)
	signed := signToken(t, secret, "testuser", false, -1*time.Minute) // expired

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := JWTMiddleware(sm, next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vms", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: signed})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestJWTMiddleware_MissingSecret(t *testing.T) {
	sm := newTestSM("") // no secret configured
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := JWTMiddleware(sm, next)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vms", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rr.Code)
	}
}
