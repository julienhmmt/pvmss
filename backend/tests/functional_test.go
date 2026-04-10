package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestFunctionalUserWorkflow tests complete user workflows end-to-end
func TestFunctionalUserWorkflow(t *testing.T) {
	cfg := loadRouteConfig()

	if !waitForServer(cfg.BaseURL, 30*time.Second) {
		t.Skipf("PVMSS server not reachable at %s", cfg.BaseURL)
	}

	// Skip if in offline mode or Proxmox unavailable
	isOfflineMode := strings.EqualFold(os.Getenv("PVMSS_OFFLINE"), "true") || os.Getenv("CI") != ""
	if isOfflineMode || !isProxmoxConnected(cfg) {
		t.Skip("Skipping functional workflow test: offline mode or Proxmox unavailable")
	}

	t.Run("Complete VM lifecycle", func(t *testing.T) {
		client := createHTTPClient()

		// Step 1: Login as user
		authenticate(t, cfg, client, cfg.UserUsername, cfg.UserPassword, "/login")

		// Step 2: Validate VM name via API
		validateResp := callValidationAPI(t, cfg, client, "/api/vm/validate/name", "test-vm")
		assert.True(t, validateResp.Valid)

		// Step 4: Validate VM ID via API
		validateResp = callValidationAPI(t, cfg, client, "/api/vm/validate/vmid", "9000")
		assert.True(t, validateResp.Valid)

		// Step 5: Logout (capture redirect status instead of following it)
		client.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
		logoutResp, err := client.Get(cfg.BaseURL + "/logout")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusSeeOther, logoutResp.StatusCode)
		_ = logoutResp.Body.Close()
		client.CheckRedirect = nil
	})

	t.Run("Admin configuration workflow", func(t *testing.T) {
		client := createHTTPClient()

		// Step 1: Login as admin
		authenticate(t, cfg, client, cfg.AdminUsername, cfg.AdminPassword, "/admin/login")

		// Step 2: Access admin dashboard
		dashboardResp, err := client.Get(cfg.BaseURL + "/admin")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, dashboardResp.StatusCode)
		_ = dashboardResp.Body.Close()

		// Step 3: Access nodes management
		nodesResp, err := client.Get(cfg.BaseURL + "/admin/nodes")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, nodesResp.StatusCode)
		_ = nodesResp.Body.Close()

		// Step 4: Access storage management
		storageResp, err := client.Get(cfg.BaseURL + "/admin/storage")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, storageResp.StatusCode)
		_ = storageResp.Body.Close()

		// Step 5: Test API endpoints with admin auth
		settingsResp, err := client.Get(cfg.BaseURL + "/api/settings/all")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, settingsResp.StatusCode)
		_ = settingsResp.Body.Close()

		// Step 6: Logout (capture redirect status instead of following it)
		client.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
		logoutResp, err := client.Get(cfg.BaseURL + "/logout")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusSeeOther, logoutResp.StatusCode)
		_ = logoutResp.Body.Close()
		client.CheckRedirect = nil
	})
}

// TestFunctionalAPIWorkflow tests API-only workflows
func TestFunctionalAPIWorkflow(t *testing.T) {
	cfg := loadRouteConfig()

	if !waitForServer(cfg.BaseURL, 30*time.Second) {
		t.Skipf("PVMSS server not reachable at %s", cfg.BaseURL)
	}

	// Skip if in offline mode or Proxmox unavailable
	isOfflineMode := strings.EqualFold(os.Getenv("PVMSS_OFFLINE"), "true") || os.Getenv("CI") != ""
	if isOfflineMode || !isProxmoxConnected(cfg) {
		t.Skip("Skipping API workflow test: offline mode or Proxmox unavailable")
	}

	t.Run("VM validation API workflow", func(t *testing.T) {
		client := createHTTPClient()
		authenticate(t, cfg, client, cfg.UserUsername, cfg.UserPassword, "/login")

		// Test VM name validation scenarios
		testCases := []struct {
			name        string
			input       string
			expectValid bool
		}{
			{"Valid name", "test-vm", true},
			{"Empty name", "", false},
			{"Name with invalid chars", "test<vm", false},
			{"Too long name", strings.Repeat("a", 101), false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				resp := callValidationAPI(t, cfg, client, "/api/vm/validate/name", tc.input)
				assert.Equal(t, tc.expectValid, resp.Valid)
				assert.NotEmpty(t, resp.Message)
			})
		}

		// Test VM ID validation scenarios
		idTestCases := []struct {
			name        string
			input       string
			expectValid bool
		}{
			{"Valid ID", "9000", true},
			{"Empty ID", "", false},
			{"Invalid ID", "abc", false},
			{"Out of range ID", "9999999999", false},
		}

		for _, tc := range idTestCases {
			t.Run(tc.name, func(t *testing.T) {
				resp := callValidationAPI(t, cfg, client, "/api/vm/validate/vmid", tc.input)
				assert.Equal(t, tc.expectValid, resp.Valid)
				assert.NotEmpty(t, resp.Message)
			})
		}
	})

	t.Run("Settings API workflow", func(t *testing.T) {
		client := createHTTPClient()
		authenticate(t, cfg, client, cfg.UserUsername, cfg.UserPassword, "/login")

		// Test settings endpoints
		endpoints := []string{
			"/api/settings",
			"/api/settings/all",
			"/api/vmbr/all",
		}

		for _, endpoint := range endpoints {
			t.Run("Endpoint "+endpoint, func(t *testing.T) {
				resp, err := client.Get(cfg.BaseURL + endpoint)
				assert.NoError(t, err)
				assert.Equal(t, http.StatusOK, resp.StatusCode)

				// Verify response is valid JSON
				var data interface{}
				err = json.NewDecoder(resp.Body).Decode(&data)
				assert.NoError(t, err)
				_ = resp.Body.Close()
			})
		}
	})
}

// TestFunctionalErrorHandling tests error scenarios
func TestFunctionalErrorHandling(t *testing.T) {
	cfg := loadRouteConfig()

	if !waitForServer(cfg.BaseURL, 30*time.Second) {
		t.Skipf("PVMSS server not reachable at %s", cfg.BaseURL)
	}

	// Skip if in offline mode - error handling tests need a running server
	isOfflineMode := strings.EqualFold(os.Getenv("PVMSS_OFFLINE"), "true") || os.Getenv("CI") != ""
	if isOfflineMode {
		t.Skip("Skipping functional error handling test: offline mode")
	}

	t.Run("Authentication error scenarios", func(t *testing.T) {
		client := createHTTPClient()

		// Test accessing protected routes without authentication
		protectedRoutes := []string{
			"/profile",
			"/api/settings",
			"/admin",
		}

		for _, route := range protectedRoutes {
			t.Run("Protected route "+route, func(t *testing.T) {
				// Don't follow redirects to capture the 303 response
				client.CheckRedirect = func(*http.Request, []*http.Request) error {
					return http.ErrUseLastResponse
				}

				resp, err := client.Get(cfg.BaseURL + route)
				assert.NoError(t, err)
				assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
				_ = resp.Body.Close()

				// Reset redirect behavior
				client.CheckRedirect = nil
			})
		}
	})

	t.Run("Invalid API requests", func(t *testing.T) {
		client := createHTTPClient()
		authenticate(t, cfg, client, cfg.UserUsername, cfg.UserPassword, "/login")

		// Test invalid validation requests
		t.Run("Invalid validation request", func(t *testing.T) {
			resp, err := client.Post(cfg.BaseURL+"/api/vm/validate/name", "application/json", strings.NewReader(""))
			assert.NoError(t, err)
			// CSRF protection returns 403 for requests without CSRF token
			assert.Equal(t, http.StatusForbidden, resp.StatusCode)
			_ = resp.Body.Close()
		})

		// Test non-existent API endpoints
		t.Run("Non-existent endpoint", func(t *testing.T) {
			resp, err := client.Get(cfg.BaseURL + "/api/nonexistent")
			assert.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, resp.StatusCode)
			_ = resp.Body.Close()
		})
	})
}

// Helper function to call validation APIs
type validationResponse struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

func callValidationAPI(t *testing.T, cfg routeConfig, client *http.Client, endpoint, value string) validationResponse {
	t.Helper()

	// Get CSRF token from any authenticated page (use profile page)
	getResp, err := client.Get(cfg.BaseURL + "/profile")
	if err != nil {
		t.Fatalf("Failed to get CSRF token from profile page: %v", err)
	}
	defer func() {
		_ = getResp.Body.Close()
	}()

	body, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("Failed to read profile page for CSRF token: %v", err)
	}

	csrfToken := extractCSRFToken(string(body))
	if csrfToken == "" {
		t.Fatalf("Could not extract CSRF token from profile page")
	}

	// Make POST request with CSRF token
	payload := fmt.Sprintf(`{"value": "%s"}`, value)
	req, err := http.NewRequest("POST", cfg.BaseURL+endpoint, strings.NewReader(payload))
	if err != nil {
		t.Fatalf("Failed to create validation request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrfToken)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to call validation API %s: %v", endpoint, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Validation API %s returned status %d, expected 200", endpoint, resp.StatusCode)
	}

	var result validationResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		t.Fatalf("Failed to decode validation response from %s: %v", endpoint, err)
	}

	return result
}
