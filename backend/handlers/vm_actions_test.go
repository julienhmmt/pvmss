package handlers

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"pvmss/proxmox"
	"pvmss/state"
	"pvmss/utils"

	"github.com/alexedwards/scs/v2"
	"github.com/julienschmidt/httprouter"
)

// TestVMActionsHandlerMACValidationTests tests MAC address validation in VM resource updates
func TestVMActionsHandlerMACValidationTests(t *testing.T) {
	// Test that all MAC-related functions work together correctly for VM actions
	t.Run("MAC Validation for VM Updates", func(t *testing.T) {
		testMACs := []struct {
			mac         string
			shouldPass  bool
			description string
		}{
			{"AA:BB:CC:DD:EE:FF", true, "Valid colon format"},
			{"aa:bb:cc:dd:ee:ff", true, "Valid lowercase"},
			{"Aa:Bb:Cc:Dd:Ee:Ff", true, "Valid mixed case"},
			{"AA-BB-CC-DD-EE-FF", false, "Invalid hyphen format"},
			{"AABBCCDDEEFF", false, "Invalid no separators"},
			{"AA:BB:CC:DD:EE", false, "Invalid too short"},
			{"AA:BB:CC:DD:EE:FF:GG", false, "Invalid too long"},
			{"AA:BB:CC:DD:EE:@@", false, "Invalid special characters"},
		}

		for _, tc := range testMACs {
			t.Run(tc.description, func(t *testing.T) {
				// Test validation
				isValid := utils.ValidateMACAddress(tc.mac)
				normalized := utils.NormalizeMACAddress(tc.mac)

				if tc.shouldPass && !isValid {
					t.Errorf("Expected MAC %q to be valid, but validation failed", tc.mac)
				}
				if !tc.shouldPass && isValid {
					t.Errorf("Expected MAC %q to be invalid, but validation passed", tc.mac)
				}

				// Test normalization behavior
				if tc.mac != "" {
					if isValid && !utils.ValidateMACAddress(normalized) {
						t.Errorf("Normalized MAC %q should be valid but validation failed", normalized)
					}
				}
			})
		}
	})

	t.Run("Generated MACs are valid for VM actions", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			generatedMAC := utils.GenerateRandomMACAddress()
			if !utils.ValidateMACAddress(generatedMAC) {
				t.Errorf("Generated MAC %q is not valid for VM actions", generatedMAC)
			}

			// Verify format is correct
			if !utils.ValidateMACAddress(generatedMAC) {
				t.Errorf("Generated MAC %q doesn't have correct format", generatedMAC)
			}
		}
	})
}

type fakeProxmoxClient struct{}

func (c *fakeProxmoxClient) DeleteWithContext(_ context.Context, _ string, _ url.Values) (map[string]interface{}, error) {
	return nil, nil
}

func (c *fakeProxmoxClient) Get(_ string) (map[string]interface{}, error) {
	return nil, nil
}

func (c *fakeProxmoxClient) GetApiUrl() string {
	return ""
}

func (c *fakeProxmoxClient) GetClusterName() string {
	return ""
}

func (c *fakeProxmoxClient) GetCSRFPreventionToken() string {
	return ""
}

func (c *fakeProxmoxClient) GetJSON(_ context.Context, _ string, _ interface{}) error {
	return nil
}

func (c *fakeProxmoxClient) GetPVEAuthCookie() string {
	return ""
}

func (c *fakeProxmoxClient) GetTimeout() time.Duration {
	return 0
}

func (c *fakeProxmoxClient) GetWithContext(_ context.Context, _ string) (map[string]interface{}, error) {
	return nil, nil
}

func (c *fakeProxmoxClient) InvalidateCache(_ string) {}

func (c *fakeProxmoxClient) PostFormAndGetJSON(_ context.Context, _ string, _ url.Values, _ interface{}) error {
	return nil
}

func (c *fakeProxmoxClient) PostFormWithContext(_ context.Context, _ string, _ url.Values) (map[string]interface{}, error) {
	return nil, nil
}

func (c *fakeProxmoxClient) PutFormWithContext(_ context.Context, _ string, _ url.Values) (map[string]interface{}, error) {
	return nil, nil
}

func (c *fakeProxmoxClient) SetTimeout(_ time.Duration) {}

type fakeStateManager struct {
	offline          bool
	proxmoxClient    proxmox.ClientInterface
	proxmoxConnected bool
}

func (f *fakeStateManager) GetTemplates() *template.Template {
	return nil
}

func (f *fakeStateManager) SetTemplates(_ *template.Template) error {
	return nil
}

func (f *fakeStateManager) GetSessionManager() *scs.SessionManager {
	return nil
}

func (f *fakeStateManager) SetSessionManager(_ *scs.SessionManager) error {
	return nil
}

func (f *fakeStateManager) GetProxmoxClient() proxmox.ClientInterface {
	return f.proxmoxClient
}

func (f *fakeStateManager) SetProxmoxClient(pc proxmox.ClientInterface) error {
	f.proxmoxClient = pc
	return nil
}

func (f *fakeStateManager) SetOfflineMode() {
	f.offline = true
}

func (f *fakeStateManager) IsOfflineMode() bool {
	return f.offline
}

func (f *fakeStateManager) GetProxmoxStatus() (bool, string) {
	return f.proxmoxConnected, ""
}

func (f *fakeStateManager) CheckProxmoxConnection() bool {
	return f.proxmoxConnected
}

func (f *fakeStateManager) GetNodeCache() ([]*proxmox.NodeDetails, time.Time) {
	return nil, time.Time{}
}

func (f *fakeStateManager) GetProxmoxSnapshot() *state.ProxmoxClusterSnapshot {
	return nil
}

func (f *fakeStateManager) RequestSnapshotRefresh() {}

func (f *fakeStateManager) GetSettings() *state.AppSettings {
	return nil
}

func (f *fakeStateManager) SetSettings(_ *state.AppSettings) error {
	return nil
}

func (f *fakeStateManager) SetSettingsWithoutSave(_ *state.AppSettings) {}

func (f *fakeStateManager) GetTags() []string {
	return nil
}

func (f *fakeStateManager) GetISOs() []string {
	return nil
}

func (f *fakeStateManager) GetVMBRs() []string {
	return nil
}

func (f *fakeStateManager) GetLimits() map[string]interface{} {
	return nil
}

func (f *fakeStateManager) GetStorages() []string {
	return nil
}

func (f *fakeStateManager) AddCSRFToken(_ string, _ time.Time) error {
	return nil
}

func (f *fakeStateManager) ValidateAndRemoveCSRFToken(_ string) bool {
	return true
}

func (f *fakeStateManager) CleanExpiredCSRFTokens() {}

func (f *fakeStateManager) GetFrontendPath() string {
	return ""
}

func (f *fakeStateManager) SetFrontendPath(_ string) {}

func (f *fakeStateManager) SetGuestAgentCleanupFunc(_ func()) {}

func TestGetGuestAgentStatus_OfflineMode(t *testing.T) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/vm/action", nil)
	fakeSM := &fakeStateManager{offline: true}
	ctx := context.WithValue(req.Context(), StateManagerKey, state.StateManager(fakeSM))
	req = req.WithContext(ctx)

	status := getGuestAgentStatus(req, "node1", 100)
	if status != agentStatusUnknown {
		t.Fatalf("expected agentStatusUnknown in offline mode, got %v", status)
	}
}

func TestGetGuestAgentStatus_UnavailableCached(t *testing.T) {
	t.Helper()

	node := "node1"
	vmid := 101
	cacheGuestAgentUnavailable(node, vmid)
	defer InvalidateGuestAgentCache(node, vmid)

	req := httptest.NewRequest(http.MethodPost, "/vm/action", nil)
	fakeSM := &fakeStateManager{offline: false, proxmoxConnected: true, proxmoxClient: &fakeProxmoxClient{}}
	ctx := context.WithValue(req.Context(), StateManagerKey, state.StateManager(fakeSM))
	req = req.WithContext(ctx)

	status := getGuestAgentStatus(req, node, vmid)
	if status != agentStatusUnavailable {
		t.Fatalf("expected agentStatusUnavailable when cached as unavailable, got %v", status)
	}
}

func TestVMActionHandler_ShutdownOfflineMode(t *testing.T) {
	t.Helper()

	form := url.Values{}
	form.Set("vmid", "100")
	form.Set("node", "node1")
	form.Set("action", "shutdown")
	req := httptest.NewRequest(http.MethodPost, "/vm/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	fakeSM := &fakeStateManager{offline: true}
	ctx := context.WithValue(req.Context(), StateManagerKey, state.StateManager(fakeSM))
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h := &VMHandler{}
	h.VMActionHandler(rec, req, httprouter.Params{})

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}

	location := rec.Header().Get("Location")
	if !strings.HasPrefix(location, "/vm/details/100") {
		t.Fatalf("expected redirect to /vm/details/100..., got %s", location)
	}
	u, err := url.Parse(location)
	if err != nil {
		t.Fatalf("failed to parse redirect URL: %v", err)
	}
	q := u.Query()
	if q.Get("error") != "1" {
		t.Fatalf("expected error=1 in redirect URL, got %s", location)
	}
	errorMsg := q.Get("error_msg")
	if errorMsg == "" {
		t.Fatalf("expected non-empty error_msg in redirect URL, got %s", location)
	}
	if !strings.Contains(errorMsg, "offline mode") || !strings.Contains(errorMsg, "QEMU Guest Agent") {
		t.Fatalf("unexpected offline guest agent message in redirect URL: %s", errorMsg)
	}
}

func TestVMActionHandler_ShutdownGuestAgentUnavailablePrecheck(t *testing.T) {
	t.Helper()

	node := "node1"
	vmid := 102
	cacheGuestAgentUnavailable(node, vmid)
	defer InvalidateGuestAgentCache(node, vmid)

	form := url.Values{}
	form.Set("vmid", "102")
	form.Set("node", node)
	form.Set("action", "shutdown")
	req := httptest.NewRequest(http.MethodPost, "/vm/action", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	fakeSM := &fakeStateManager{
		offline:          false,
		proxmoxClient:    &fakeProxmoxClient{},
		proxmoxConnected: true,
	}
	ctx := context.WithValue(req.Context(), StateManagerKey, state.StateManager(fakeSM))
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h := &VMHandler{}
	h.VMActionHandler(rec, req, httprouter.Params{})

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected status %d, got %d", http.StatusSeeOther, rec.Code)
	}
	location := rec.Header().Get("Location")
	if !strings.HasPrefix(location, "/vm/details/102") {
		t.Fatalf("expected redirect to /vm/details/102..., got %s", location)
	}
	u, err := url.Parse(location)
	if err != nil {
		t.Fatalf("failed to parse redirect URL: %v", err)
	}
	q := u.Query()
	if q.Get("error") != "1" {
		t.Fatalf("expected error=1 in redirect URL, got %s", location)
	}
	errorMsg := q.Get("error_msg")
	if errorMsg == "" {
		t.Fatalf("expected non-empty error_msg in redirect URL, got %s", location)
	}
	if !strings.Contains(errorMsg, "QEMU Guest Agent") || !strings.Contains(errorMsg, "not running or not responding") {
		t.Fatalf("unexpected guest agent timeout message in redirect URL: %s", errorMsg)
	}
}
