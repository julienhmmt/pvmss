package handlers_test

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/stretchr/testify/assert"

	"pvmss/handlers"
	"pvmss/proxmox"
	"pvmss/state"
)

// mockStateManager implements state.StateManager for testing
type mockStateManager struct {
	sessionManager *scs.SessionManager
}

func (m *mockStateManager) GetSessionManager() *scs.SessionManager {
	return m.sessionManager
}

// Implement other StateManager methods as no-ops for testing
func (m *mockStateManager) GetTemplates() *template.Template                  { return nil }
func (m *mockStateManager) SetTemplates(t *template.Template) error           { return nil }
func (m *mockStateManager) SetSessionManager(sm *scs.SessionManager) error    { return nil }
func (m *mockStateManager) GetProxmoxClient() proxmox.ClientInterface         { return nil }
func (m *mockStateManager) SetProxmoxClient(pc proxmox.ClientInterface) error { return nil }
func (m *mockStateManager) SetOfflineMode()                                   {}
func (m *mockStateManager) IsOfflineMode() bool                               { return false }
func (m *mockStateManager) GetProxmoxStatus() (bool, string)                  { return true, "" }
func (m *mockStateManager) CheckProxmoxConnection() bool                      { return true }
func (m *mockStateManager) GetNodeCache() ([]*proxmox.NodeDetails, time.Time) {
	return nil, time.Time{}
}
func (m *mockStateManager) GetProxmoxSnapshot() *state.ProxmoxClusterSnapshot  { return nil }
func (m *mockStateManager) RequestSnapshotRefresh()                            {}
func (m *mockStateManager) GetSettings() *state.AppSettings                    { return nil }
func (m *mockStateManager) SetSettings(settings *state.AppSettings) error      { return nil }
func (m *mockStateManager) SetSettingsWithoutSave(settings *state.AppSettings) {}
func (m *mockStateManager) GetTags() []string                                  { return nil }
func (m *mockStateManager) GetISOs() []string                                  { return nil }
func (m *mockStateManager) GetVMBRs() []string                                 { return nil }
func (m *mockStateManager) GetLimits() map[string]interface{}                  { return nil }
func (m *mockStateManager) GetStorages() []string                              { return nil }
func (m *mockStateManager) AddCSRFToken(token string, expiry time.Time) error  { return nil }
func (m *mockStateManager) ValidateAndRemoveCSRFToken(token string) bool       { return true }
func (m *mockStateManager) CleanExpiredCSRFTokens()                            {}
func (m *mockStateManager) GetFrontendPath() string                            { return "" }
func (m *mockStateManager) SetFrontendPath(path string)                        {}
func (m *mockStateManager) SetGuestAgentCleanupFunc(cleanupFunc func())        {}

func newTestRequest(method, path string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	return req
}

func withStateManager(r *http.Request, sm state.StateManager) *http.Request {
	ctx := context.WithValue(r.Context(), handlers.StateManagerKey, sm)
	return r.WithContext(ctx)
}

func withSession(r *http.Request, sm *scs.SessionManager, authenticated bool, isAdmin bool) *http.Request {
	ctx, _ := sm.Load(r.Context(), "")
	sm.Put(ctx, "authenticated", authenticated)
	sm.Put(ctx, "is_admin", isAdmin)
	sm.Put(ctx, "username", "testuser")
	return r.WithContext(ctx)
}

func TestIsAuthenticated_NoStateManager(t *testing.T) {
	req := newTestRequest(http.MethodGet, "/test")
	// No state manager in context
	result := handlers.IsAuthenticated(req)
	assert.False(t, result, "Should return false when state manager is missing")
}

func TestIsAuthenticated_NoSessionManager(t *testing.T) {
	req := newTestRequest(http.MethodGet, "/test")
	sm := &mockStateManager{sessionManager: nil}
	req = withStateManager(req, sm)

	result := handlers.IsAuthenticated(req)
	assert.False(t, result, "Should return false when session manager is nil")
}

func TestIsAuthenticated_NotAuthenticated(t *testing.T) {
	sessionManager := scs.New()
	req := newTestRequest(http.MethodGet, "/test")
	sm := &mockStateManager{sessionManager: sessionManager}
	req = withStateManager(req, sm)
	req = withSession(req, sessionManager, false, false)

	result := handlers.IsAuthenticated(req)
	assert.False(t, result, "Should return false when not authenticated")
}

func TestIsAuthenticated_Authenticated(t *testing.T) {
	sessionManager := scs.New()
	req := newTestRequest(http.MethodGet, "/test")
	sm := &mockStateManager{sessionManager: sessionManager}
	req = withStateManager(req, sm)
	req = withSession(req, sessionManager, true, false)

	result := handlers.IsAuthenticated(req)
	assert.True(t, result, "Should return true when authenticated")
}

func TestIsAdmin_NotAdmin(t *testing.T) {
	sessionManager := scs.New()
	req := newTestRequest(http.MethodGet, "/admin")
	sm := &mockStateManager{sessionManager: sessionManager}
	req = withStateManager(req, sm)
	req = withSession(req, sessionManager, true, false)

	result := handlers.IsAdmin(req)
	assert.False(t, result, "Should return false when user is not admin")
}

func TestIsAdmin_IsAdmin(t *testing.T) {
	sessionManager := scs.New()
	req := newTestRequest(http.MethodGet, "/admin")
	sm := &mockStateManager{sessionManager: sessionManager}
	req = withStateManager(req, sm)
	req = withSession(req, sessionManager, true, true)

	result := handlers.IsAdmin(req)
	assert.True(t, result, "Should return true when user is admin")
}

func TestRequireAuth_RedirectsWhenNotAuthenticated(t *testing.T) {
	sessionManager := scs.New()
	req := newTestRequest(http.MethodGet, "/protected")
	sm := &mockStateManager{sessionManager: sessionManager}
	req = withStateManager(req, sm)
	req = withSession(req, sessionManager, false, false)

	rr := httptest.NewRecorder()

	handler := handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code, "Should redirect when not authenticated")
	assert.Contains(t, rr.Header().Get("Location"), "/login", "Should redirect to login page")
}

func TestRequireAuth_AllowsWhenAuthenticated(t *testing.T) {
	sessionManager := scs.New()
	req := newTestRequest(http.MethodGet, "/protected")
	sm := &mockStateManager{sessionManager: sessionManager}
	req = withStateManager(req, sm)
	req = withSession(req, sessionManager, true, false)

	rr := httptest.NewRecorder()
	handlerCalled := false

	handler := handlers.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler.ServeHTTP(rr, req)

	assert.True(t, handlerCalled, "Handler should be called when authenticated")
	assert.Equal(t, http.StatusOK, rr.Code, "Should return OK when authenticated")
}

func TestRequireAdminAuth_RedirectsWhenNotAdmin(t *testing.T) {
	sessionManager := scs.New()
	req := newTestRequest(http.MethodGet, "/admin/settings")
	sm := &mockStateManager{sessionManager: sessionManager}
	req = withStateManager(req, sm)
	req = withSession(req, sessionManager, false, false)

	rr := httptest.NewRecorder()

	handler := handlers.RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusSeeOther, rr.Code, "Should redirect when not admin")
	assert.Contains(t, rr.Header().Get("Location"), "/admin/login", "Should redirect to admin login page")
}

func TestRequireAdminAuth_AllowsWhenAdmin(t *testing.T) {
	sessionManager := scs.New()
	req := newTestRequest(http.MethodGet, "/admin/settings")
	sm := &mockStateManager{sessionManager: sessionManager}
	req = withStateManager(req, sm)
	req = withSession(req, sessionManager, true, true)

	rr := httptest.NewRecorder()
	handlerCalled := false

	handler := handlers.RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler.ServeHTTP(rr, req)

	assert.True(t, handlerCalled, "Handler should be called when admin")
	assert.Equal(t, http.StatusOK, rr.Code, "Should return OK when admin")
}
