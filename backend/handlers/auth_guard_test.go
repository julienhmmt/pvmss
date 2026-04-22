package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/stretchr/testify/assert"

	"pvmss/database"
	envpkg "pvmss/env"
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
func (m *mockStateManager) SetSessionManager(sm *scs.SessionManager) error { return nil }
func (m *mockStateManager) StartOnlineMode() error                         { return nil }
func (m *mockStateManager) SetOfflineMode()                                {}
func (m *mockStateManager) IsOfflineMode() bool                            { return false }
func (m *mockStateManager) GetProxmoxStatus() (bool, string)               { return true, "" }
func (m *mockStateManager) CheckProxmoxConnection() bool                   { return true }
func (m *mockStateManager) RefreshNodeCache(_ context.Context)             {}
func (m *mockStateManager) GetNodeCache() ([]*proxmox.NodeDetails, time.Time) {
	return nil, time.Time{}
}
func (m *mockStateManager) GetProxmoxSnapshot() *state.ProxmoxClusterSnapshot             { return nil }
func (m *mockStateManager) RequestSnapshotRefresh()                                       {}
func (m *mockStateManager) GetSettings() *state.AppSettings                               { return nil }
func (m *mockStateManager) SetSettings(settings *state.AppSettings) error                 { return nil }
func (m *mockStateManager) SetSettingsWithoutSave(settings *state.AppSettings)            {}
func (m *mockStateManager) GetTags() []string                                             { return nil }
func (m *mockStateManager) GetISOs() []string                                             { return nil }
func (m *mockStateManager) GetVMBRs() []string                                            { return nil }
func (m *mockStateManager) GetLimits() map[string]interface{}                             { return nil }
func (m *mockStateManager) GetStorages() []string                                         { return nil }
func (m *mockStateManager) AddCSRFToken(token string, expiry time.Time) error             { return nil }
func (m *mockStateManager) ValidateAndRemoveCSRFToken(token string) bool                  { return true }
func (m *mockStateManager) CleanExpiredCSRFTokens()                                       {}
func (m *mockStateManager) GetFrontendPath() string                                       { return "" }
func (m *mockStateManager) SetFrontendPath(path string)                                   {}
func (m *mockStateManager) SetGuestAgentCleanupFunc(cleanupFunc func())                   {}
func (m *mockStateManager) GetEnvConfig() *envpkg.EnvConfig                               { return &envpkg.EnvConfig{} }
func (m *mockStateManager) SetEnvConfig(cfg *envpkg.EnvConfig)                            {}
func (m *mockStateManager) LoadSettingsFromDB() error                                     { return nil }
func (m *mockStateManager) HasDB() bool                                                   { return false }
func (m *mockStateManager) SetVMLimits(limits *database.VMLimits, changedBy string) error { return nil }
func (m *mockStateManager) GetNodeLimitFromDB(node string) (database.NodeLimit, bool, error) {
	return database.NodeLimit{}, false, nil
}
func (m *mockStateManager) SetNodeLimit(limit database.NodeLimit, changedBy string) error { return nil }
func (m *mockStateManager) DeleteNodeLimit(node string, changedBy string) error           { return nil }
func (m *mockStateManager) SetEnabledNodes(nodes []string, changedBy string) error        { return nil }
func (m *mockStateManager) SetEnabledStorages(storages []string, changedBy string) error  { return nil }
func (m *mockStateManager) SetEnabledISOs(isos []string, changedBy string) error          { return nil }
func (m *mockStateManager) SetEnabledVMBRs(vmbrs []string, changedBy string) error        { return nil }
func (m *mockStateManager) SetTags(tags []string, changedBy string) error                 { return nil }
func (m *mockStateManager) CreateCloudInitTemplate(t *database.CloudInitTemplate, changedBy string) error {
	return nil
}
func (m *mockStateManager) UpdateCloudInitTemplate(t *database.CloudInitTemplate, changedBy string) error {
	return nil
}
func (m *mockStateManager) DeleteCloudInitTemplate(id string, changedBy string) error     { return nil }
func (m *mockStateManager) CreateVMProfile(p *database.VMProfile, changedBy string) error { return nil }
func (m *mockStateManager) UpdateVMProfile(p *database.VMProfile, changedBy string) error { return nil }
func (m *mockStateManager) DeleteVMProfile(id string, changedBy string) error             { return nil }
func (m *mockStateManager) SetSFTPConfig(cfg *database.SFTPConfig, changedBy string) error {
	return nil
}

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
