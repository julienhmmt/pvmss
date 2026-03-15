package apiv1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pvmss/proxmox"
	"pvmss/state"
)

// adminTestSM extends testStateManager with configurable node cache and settings.
type adminTestSM struct {
	testStateManager
	nodeCache []*proxmox.NodeDetails
	snapshot  *state.ProxmoxClusterSnapshot
}

func newAdminTestSM(jwtSecret string, offline bool) *adminTestSM {
	return &adminTestSM{
		testStateManager: testStateManager{
			settings: &state.AppSettings{
				JWTSecret:       jwtSecret,
				Tags:            []string{"pvmss"},
				EnabledStorages: []string{"local"},
				VMBRs:           []string{"vmbr0"},
				ISOs:            []string{},
				Limits: state.LimitsConfig{
					VM: state.VMResourceLimits{
						Sockets: state.ResourceRange{Min: 1, Max: 2},
						Cores:   state.ResourceRange{Min: 1, Max: 4},
						RAM:     state.ResourceRange{Min: 1, Max: 8},
						Disk:    state.ResourceRange{Min: 1, Max: 100},
					},
					Nodes:        make(map[string]state.NodeResourceLimits),
					MaxSnapshots: 8,
				},
				MaxNetworkCards:    1,
				MaxDiskPerVM:       1,
				MaxVMPerUser:       5,
				CloudInitTemplates: []state.CloudInitTemplate{},
			},
			offline: offline,
		},
	}
}

func (m *adminTestSM) GetNodeCache() ([]*proxmox.NodeDetails, time.Time) {
	return m.nodeCache, time.Now()
}

func (m *adminTestSM) GetProxmoxSnapshot() *state.ProxmoxClusterSnapshot {
	return m.snapshot
}

func (m *adminTestSM) SetSettings(s *state.AppSettings) error {
	m.settings = s
	return nil
}

func (m *adminTestSM) GetTags() []string {
	if m.settings != nil {
		return m.settings.Tags
	}
	return nil
}

func (m *adminTestSM) GetStorages() []string {
	if m.settings != nil {
		return m.settings.EnabledStorages
	}
	return nil
}

func (m *adminTestSM) GetVMBRs() []string {
	if m.settings != nil {
		return m.settings.VMBRs
	}
	return nil
}

func (m *adminTestSM) GetISOs() []string {
	if m.settings != nil {
		return m.settings.ISOs
	}
	return nil
}

// helper to make an admin request with JWT cookie
func adminRequest(t *testing.T, method, path, secret string, isAdmin bool) *httptest.ResponseRecorder {
	t.Helper()
	handler := makeAdminTestRouter(secret, isAdmin)
	req := httptest.NewRequest(method, path, nil)
	if secret != "" {
		token := signToken(t, secret, "admin", isAdmin, 15*time.Minute)
		req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func makeAdminTestRouter(secret string, _ bool) http.Handler {
	sm := newAdminTestSM(secret, true) // offline mode for tests
	sm.nodeCache = []*proxmox.NodeDetails{
		{Node: "node1", Status: "online", CPU: 0.5, MaxCPU: 4, Memory: 1024, MaxMemory: 2048, Uptime: 3600},
	}
	sm.snapshot = &state.ProxmoxClusterSnapshot{
		VMs: []state.SnapshotVM{
			{VMID: 100, Name: "test-vm", Node: "node1", Status: "running", Tags: "pvmss"},
		},
	}

	handler := MakeAdminHandler(sm)

	// Wire up via middleware so context is properly set
	return JWTAdminMiddleware(sm, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/admin/nodes":
			handler.Nodes(w, r)
		case "/api/v1/admin/appinfo":
			handler.AppInfo(w, r)
		case "/api/v1/admin/settings":
			handler.Settings(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestAdminNodes_Offline(t *testing.T) {
	secret := "testsecretthatis32byteslongexact!!"
	sm := newAdminTestSM(secret, true)
	handler := MakeAdminHandler(sm)

	// Wrap with admin middleware
	h := JWTAdminMiddleware(sm, http.HandlerFunc(handler.Nodes))
	token := signToken(t, secret, "admin", true, 15*time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/nodes", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var nodes []AdminNodeResponse
	if err := json.NewDecoder(rr.Body).Decode(&nodes); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected empty nodes in offline mode, got %d", len(nodes))
	}
}

func TestAdminNodes_WithCache(t *testing.T) {
	secret := "testsecretthatis32byteslongexact!!"
	sm := newAdminTestSM(secret, false) // online
	sm.nodeCache = []*proxmox.NodeDetails{
		{Node: "pve1", Status: "online", CPU: 0.25, MaxCPU: 8, Memory: 4096, MaxMemory: 8192, Uptime: 7200},
	}
	handler := MakeAdminHandler(sm)
	h := JWTAdminMiddleware(sm, http.HandlerFunc(handler.Nodes))
	token := signToken(t, secret, "admin", true, 15*time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/nodes", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var nodes []AdminNodeResponse
	if err := json.NewDecoder(rr.Body).Decode(&nodes); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Name != "pve1" {
		t.Errorf("expected name 'pve1', got %q", nodes[0].Name)
	}
	if nodes[0].MaxCPU != 8 {
		t.Errorf("expected maxcpu 8, got %d", nodes[0].MaxCPU)
	}
}

func TestAdminAppInfo(t *testing.T) {
	secret := "testsecretthatis32byteslongexact!!"
	sm := newAdminTestSM(secret, true)
	sm.snapshot = &state.ProxmoxClusterSnapshot{
		VMs: []state.SnapshotVM{{VMID: 100}},
	}
	handler := MakeAdminHandler(sm)
	h := JWTAdminMiddleware(sm, http.HandlerFunc(handler.AppInfo))
	token := signToken(t, secret, "admin", true, 15*time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/appinfo", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var info AdminAppInfoResponse
	if err := json.NewDecoder(rr.Body).Decode(&info); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if !info.OfflineMode {
		t.Error("expected offline_mode=true")
	}
	if info.TotalVMs != 1 {
		t.Errorf("expected 1 VM, got %d", info.TotalVMs)
	}
}

func TestAdminSettings(t *testing.T) {
	secret := "testsecretthatis32byteslongexact!!"
	sm := newAdminTestSM(secret, false)
	handler := MakeAdminHandler(sm)
	h := JWTAdminMiddleware(sm, http.HandlerFunc(handler.Settings))
	token := signToken(t, secret, "admin", true, 15*time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAdmin_NonAdminReturns403(t *testing.T) {
	secret := "testsecretthatis32byteslongexact!!"
	sm := newAdminTestSM(secret, false)
	handler := MakeAdminHandler(sm)
	h := JWTAdminMiddleware(sm, http.HandlerFunc(handler.Nodes))

	// Sign token with is_admin=false
	token := signToken(t, secret, "regularuser", false, 15*time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/nodes", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestAdmin_NoCookieReturns401(t *testing.T) {
	secret := "testsecretthatis32byteslongexact!!"
	sm := newAdminTestSM(secret, false)
	handler := MakeAdminHandler(sm)
	h := JWTAdminMiddleware(sm, http.HandlerFunc(handler.Nodes))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/nodes", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
