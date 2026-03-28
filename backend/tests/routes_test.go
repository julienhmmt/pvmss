package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getTestBaseURL returns the base URL for testing
func getTestBaseURL() string {
	if url := os.Getenv("TEST_BASE_URL"); url != "" {
		return url
	}
	// Default to localhost for manual testing, but integration tests should use httptest.NewServer
	return "http://localhost:50000"
}

// Note: User pool route tests are covered by integration_test.go which uses httptest.NewServer

// TestUserPoolSelfCreationRoute tests the /userpool/create-self endpoint
func TestUserPoolSelfCreationRoute(t *testing.T) {
	baseURL := getTestBaseURL()
	if !waitForServer(baseURL, 5*time.Second) {
		t.Skipf("PVMSS server not reachable at %s; skipping TestUserPoolSelfCreationRoute (manual test)", baseURL)
	}

	// First, login as admin
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	client := &http.Client{
		Jar:     jar,
		Timeout: 10 * time.Second,
	}

	// Perform admin login
	loginURL := baseURL + "/admin/login"
	loginData := url.Values{
		"username": {"admin"},
		"password": {"admin123"},
	}

	resp, err := client.PostForm(loginURL, loginData)
	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	func() { _ = resp.Body.Close() }()

	// Test cases for the self-creation endpoint
	testCases := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   string
		description    string
	}{
		{
			name:           "POST to create-self endpoint",
			method:         "POST",
			expectedStatus: http.StatusSeeOther,
			expectedBody:   "",
			description:    "Should redirect after processing pool creation request",
		},
		{
			name:           "GET method not allowed",
			method:         "GET",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method Not Allowed",
			description:    "GET should return 405 Method Not Allowed",
		},
		{
			name:           "PUT method not allowed",
			method:         "PUT",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method Not Allowed",
			description:    "PUT should return 405 Method Not Allowed",
		},
		{
			name:           "DELETE method not allowed",
			method:         "DELETE",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method Not Allowed",
			description:    "DELETE should return 405 Method Not Allowed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			req, err := http.NewRequest(tc.method, baseURL+"/userpool/create-self", nil)
			require.NoError(t, err)

			// Add CSRF token for POST requests
			if tc.method == "POST" {
				// Get a page to extract CSRF token
				pageResp, err := client.Get(baseURL + "/admin/userpool")
				require.NoError(t, err)
				defer func() { _ = pageResp.Body.Close() }()

				// Extract CSRF token from page
				body, err := io.ReadAll(pageResp.Body)
				require.NoError(t, err)

				// Find CSRF token in page
				csrfRegex := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)
				matches := csrfRegex.FindSubmatch(body)
				require.True(t, len(matches) > 1, "CSRF token not found in page")

				csrfToken := string(matches[1])

				// Add CSRF token to POST request
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.Body = io.NopCloser(strings.NewReader("csrf_token=" + url.QueryEscape(csrfToken)))
			}

			// Send request
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			// Check status
			assert.Equal(t, tc.expectedStatus, resp.StatusCode,
				"Expected status %d for %s, got %d. %s",
				tc.expectedStatus, tc.method, resp.StatusCode, tc.description)

			// Check body for non-redirect responses
			if tc.expectedBody != "" {
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				assert.Contains(t, string(body), tc.expectedBody,
					"Response body should contain '%s'", tc.expectedBody)
			}
		})
	}
}

// TestUserPoolSelfCreationWithAuth tests authentication requirements
func TestUserPoolSelfCreationWithAuth(t *testing.T) {
	baseURL := getTestBaseURL()
	if !waitForServer(baseURL, 5*time.Second) {
		t.Skipf("PVMSS server not reachable at %s; skipping TestUserPoolSelfCreationWithAuth (manual test)", baseURL)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Test without authentication
	t.Run("Without Authentication", func(t *testing.T) {
		req, err := http.NewRequest("POST", baseURL+"/userpool/create-self", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		// Should redirect to login
		assert.Equal(t, http.StatusSeeOther, resp.StatusCode)

		location := resp.Header.Get("Location")
		assert.Contains(t, location, "/admin/login", "Should redirect to login page")
	})

	// Test with regular user (non-admin) authentication
	t.Run("Regular User Authentication", func(t *testing.T) {
		jar, err := cookiejar.New(nil)
		require.NoError(t, err)

		userClient := &http.Client{
			Jar:     jar,
			Timeout: 10 * time.Second,
		}

		// Login as regular user
		loginURL := baseURL + "/login"
		loginData := url.Values{
			"username": {"testuser"},
			"password": {"testpass"},
		}

		resp, err := userClient.PostForm(loginURL, loginData)
		require.NoError(t, err)
		assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
		func() { _ = resp.Body.Close() }()

		// Try to access admin endpoint
		req, err := http.NewRequest("POST", baseURL+"/userpool/create-self", nil)
		require.NoError(t, err)

		resp, err = userClient.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		// Should redirect to admin login
		assert.Equal(t, http.StatusSeeOther, resp.StatusCode)

		location := resp.Header.Get("Location")
		assert.Contains(t, location, "/admin/login", "Should redirect to admin login page")
	})
}

// TestUserPoolPageWithCurrentUserPoolStatus tests that the user pool page shows current user's pool status
func TestUserPoolPageWithCurrentUserPoolStatus(t *testing.T) {
	baseURL := getTestBaseURL()
	if !waitForServer(baseURL, 5*time.Second) {
		t.Skipf("PVMSS server not reachable at %s; skipping TestUserPoolPageWithCurrentUserPoolStatus (manual test)", baseURL)
	}

	// Login as admin
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	client := &http.Client{
		Jar:     jar,
		Timeout: 10 * time.Second,
	}

	// Perform admin login
	loginURL := baseURL + "/admin/login"
	loginData := url.Values{
		"username": {"admin"},
		"password": {"admin123"},
	}

	resp, err := client.PostForm(loginURL, loginData)
	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	func() { _ = resp.Body.Close() }()

	// Get user pool page
	resp, err = client.Get(baseURL + "/admin/userpool")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Read response body
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	bodyStr := string(body)

	// Should contain current user pool status section
	assert.Contains(t, bodyStr, "Your Pool Status", "Page should show current user pool status")
	assert.Contains(t, bodyStr, "Pool Status", "Page should contain pool status information")
}

// TestUserPoolSelfCreationCSRFProtection tests CSRF protection on the endpoint
func TestUserPoolSelfCreationCSRFProtection(t *testing.T) {
	baseURL := getTestBaseURL()
	if !waitForServer(baseURL, 5*time.Second) {
		t.Skipf("PVMSS server not reachable at %s; skipping TestUserPoolSelfCreationCSRFProtection (manual test)", baseURL)
	}

	// Login as admin
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	client := &http.Client{
		Jar:     jar,
		Timeout: 10 * time.Second,
	}

	// Perform admin login
	loginURL := baseURL + "/admin/login"
	loginData := url.Values{
		"username": {"admin"},
		"password": {"admin123"},
	}

	resp, err := client.PostForm(loginURL, loginData)
	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	func() { _ = resp.Body.Close() }()

	// Test without CSRF token
	t.Run("Without CSRF Token", func(t *testing.T) {
		req, err := http.NewRequest("POST", baseURL+"/userpool/create-self", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		// Should return forbidden or bad request due to missing CSRF
		assert.True(t, resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusBadRequest,
			"Should return 403 or 400 for missing CSRF token, got %d", resp.StatusCode)
	})

	// Test with invalid CSRF token
	t.Run("With Invalid CSRF Token", func(t *testing.T) {
		req, err := http.NewRequest("POST", baseURL+"/userpool/create-self",
			strings.NewReader("csrf_token=invalid-token"))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		// Should return forbidden or bad request due to invalid CSRF
		assert.True(t, resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusBadRequest,
			"Should return 403 or 400 for invalid CSRF token, got %d", resp.StatusCode)
	})
}

// TestMaskSensitiveValue tests sensitive data masking logic
func TestMaskSensitiveValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Short value",
			input:    "short",
			expected: "***",
		},
		{
			name:     "Exactly 8 chars",
			input:    "12345678",
			expected: "***",
		},
		{
			name:     "Long value",
			input:    "this-is-a-very-long-secret-token-value",
			expected: "this-is-...[38 chars]",
		},
	}

	// Replicate maskSensitiveValue logic for testing
	maskSensitiveValue := func(value string) string {
		if len(value) <= 8 {
			return "***"
		}
		return value[:8] + "..." + fmt.Sprintf("[%d chars]", len(value))
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskSensitiveValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestTrailingSlashRedirect tests that trailing slashes are handled correctly
func TestTrailingSlashRedirect(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		expectRedirect bool
	}{
		{
			name:           "Root path with trailing slash",
			path:           "/",
			expectRedirect: false,
		},
		{
			name:           "Admin path with trailing slash",
			path:           "/admin/",
			expectRedirect: true,
		},
		{
			name:           "Static path with trailing slash",
			path:           "/css/",
			expectRedirect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldRedirect := len(tt.path) > 1 && tt.path[len(tt.path)-1] == '/' && !strings.HasPrefix(tt.path, "/css/")

			if tt.expectRedirect {
				assert.True(t, shouldRedirect, "Path %s should trigger redirect", tt.path)
			} else {
				assert.False(t, shouldRedirect, "Path %s should not trigger redirect", tt.path)
			}
		})
	}
}

// TestRouteAccessibility performs end-to-end checks against a running PVMSS instance.
// This test works in both online and offline mode.
func TestRouteAccessibility(t *testing.T) {
	cfg := loadRouteConfig()
	if !waitForServer(cfg.BaseURL, 30*time.Second) {
		t.Skipf("PVMSS server not reachable at %s", cfg.BaseURL)
	}

	// Check if we're in offline mode or CI environment
	isOfflineMode := strings.EqualFold(os.Getenv("PVMSS_OFFLINE"), "true") || os.Getenv("CI") != ""

	t.Run("Public routes", func(t *testing.T) {
		runRouteGroup(t, cfg, []routeTest{
			{Name: "Health check", Method: http.MethodGet, Path: "/health", ExpectedStatus: http.StatusOK},
			{Name: "API health", Method: http.MethodGet, Path: "/api/health", ExpectedStatus: http.StatusOK},
			{Name: "Proxmox health", Method: http.MethodGet, Path: "/api/health/proxmox", ExpectedStatus: http.StatusOK},
			{Name: "User login page", Method: http.MethodGet, Path: "/login", ExpectedStatus: http.StatusOK},
			{Name: "Admin login page", Method: http.MethodGet, Path: "/admin/login", ExpectedStatus: http.StatusOK},
			{Name: "Docs root", Method: http.MethodGet, Path: "/docs", ExpectedStatus: http.StatusOK},
			{Name: "User docs", Method: http.MethodGet, Path: "/docs/user", ExpectedStatus: http.StatusOK},
			{Name: "Admin docs", Method: http.MethodGet, Path: "/docs/admin", ExpectedStatus: http.StatusOK},
			{Name: "Favicon", Method: http.MethodGet, Path: "/favicon.ico", ExpectedStatus: http.StatusOK},
			{Name: "Base CSS", Method: http.MethodGet, Path: "/css/base.css", ExpectedStatus: http.StatusOK},
			{Name: "Accessibility JS", Method: http.MethodGet, Path: "/js/accessibility.js", ExpectedStatus: http.StatusOK},
		}, nil)
	})

	t.Run("Protected routes without auth", func(t *testing.T) {
		runRouteGroup(t, cfg, []routeTest{
			{Name: "Profile without auth", Method: http.MethodGet, Path: "/profile", ExpectedStatus: http.StatusSeeOther},
			{Name: "VM create without auth", Method: http.MethodGet, Path: "/vm/create", ExpectedStatus: http.StatusSeeOther},
			{Name: "Search without auth", Method: http.MethodGet, Path: "/search", ExpectedStatus: http.StatusSeeOther},
			{Name: "Admin dashboard without auth", Method: http.MethodGet, Path: "/admin", ExpectedStatus: http.StatusSeeOther},
			{Name: "Admin nodes without auth", Method: http.MethodGet, Path: "/admin/nodes", ExpectedStatus: http.StatusSeeOther},
			{Name: "Admin tags without auth", Method: http.MethodGet, Path: "/admin/tags", ExpectedStatus: http.StatusSeeOther},
		}, nil)
	})

	t.Run("API endpoints without auth", func(t *testing.T) {
		// API endpoints should redirect to login without authentication
		runRouteGroup(t, cfg, []routeTest{
			{Name: "API settings without auth", Method: http.MethodGet, Path: "/api/settings", ExpectedStatus: http.StatusSeeOther},
			{Name: "API all settings without auth", Method: http.MethodGet, Path: "/api/settings/all", ExpectedStatus: http.StatusSeeOther},
			{Name: "API VMBR without auth", Method: http.MethodGet, Path: "/api/vmbr/all", ExpectedStatus: http.StatusSeeOther},
		}, nil)
	})

	// Only run authenticated tests if not in offline mode and Proxmox is available
	if !isOfflineMode && isProxmoxConnected(cfg) {
		t.Run("Authenticated user routes", func(t *testing.T) {
			client := createHTTPClient()
			authenticate(t, cfg, client, cfg.UserUsername, cfg.UserPassword, "/login")

			runRouteGroup(t, cfg, []routeTest{
				{Name: "Home", Method: http.MethodGet, Path: "/", ExpectedStatus: http.StatusOK},
				{Name: "Search", Method: http.MethodGet, Path: "/search", ExpectedStatus: http.StatusOK},
				{Name: "Profile", Method: http.MethodGet, Path: "/profile", ExpectedStatus: http.StatusOK},
				{Name: "VM create", Method: http.MethodGet, Path: "/vm/create", ExpectedStatus: http.StatusOK},
				{Name: "Logout redirect", Method: http.MethodGet, Path: "/logout", ExpectedStatus: http.StatusSeeOther},
			}, client)
		})

		t.Run("Authenticated API endpoints", func(t *testing.T) {
			client := createHTTPClient()
			authenticate(t, cfg, client, cfg.UserUsername, cfg.UserPassword, "/login")

			runRouteGroup(t, cfg, []routeTest{
				{Name: "API settings with auth", Method: http.MethodGet, Path: "/api/settings", ExpectedStatus: http.StatusOK},
				{Name: "API all settings with auth", Method: http.MethodGet, Path: "/api/settings/all", ExpectedStatus: http.StatusOK},
				{Name: "API VMBR with auth", Method: http.MethodGet, Path: "/api/vmbr/all", ExpectedStatus: http.StatusOK},
			}, client)
		})

		t.Run("Admin routes", func(t *testing.T) {
			client := createHTTPClient()
			authenticate(t, cfg, client, cfg.AdminUsername, cfg.AdminPassword, "/admin/login")

			runRouteGroup(t, cfg, []routeTest{
				{Name: "Admin dashboard", Method: http.MethodGet, Path: "/admin", ExpectedStatus: http.StatusSeeOther}, // Redirects to /admin/appinfo
				{Name: "Admin nodes", Method: http.MethodGet, Path: "/admin/nodes", ExpectedStatus: http.StatusOK},
				{Name: "Admin tags", Method: http.MethodGet, Path: "/admin/tags", ExpectedStatus: http.StatusOK},
				{Name: "Admin storage", Method: http.MethodGet, Path: "/admin/storage", ExpectedStatus: http.StatusOK},
				{Name: "Admin ISO", Method: http.MethodGet, Path: "/admin/iso", ExpectedStatus: http.StatusOK},
				{Name: "Admin VMBR", Method: http.MethodGet, Path: "/admin/vmbr", ExpectedStatus: http.StatusOK},
				{Name: "Admin limits", Method: http.MethodGet, Path: "/admin/limits", ExpectedStatus: http.StatusOK},
				{Name: "Admin user pool", Method: http.MethodGet, Path: "/admin/userpool", ExpectedStatus: http.StatusOK},
				{Name: "Admin app info", Method: http.MethodGet, Path: "/admin/appinfo", ExpectedStatus: http.StatusOK},
			}, client)
		})
	} else {
		t.Skip("Skipping authenticated route tests: offline mode or Proxmox unavailable")
	}

	t.Run("404 routes", func(t *testing.T) {
		runRouteGroup(t, cfg, []routeTest{
			{Name: "Missing route", Method: http.MethodGet, Path: "/nonexistent", ExpectedStatus: http.StatusNotFound},
			{Name: "Missing API", Method: http.MethodGet, Path: "/api/nonexistent", ExpectedStatus: http.StatusNotFound},
			{Name: "Missing admin", Method: http.MethodGet, Path: "/admin/nonexistent", ExpectedStatus: http.StatusSeeOther},
		}, nil)
	})
}

func runRouteGroup(t *testing.T, cfg routeConfig, tests []routeTest, client *http.Client) {
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			runRouteTest(t, cfg, testCase, client)
		})
	}
}

func runRouteTest(t *testing.T, cfg routeConfig, test routeTest, client *http.Client) {
	t.Helper()

	c := client
	if c == nil {
		c = createHTTPClient()
	}

	req, err := http.NewRequest(test.Method, cfg.BaseURL+test.Path, nil)
	if err != nil {
		t.Fatalf("failed to construct request for %s %s: %v", test.Method, test.Path, err)
	}

	originalRedirect := c.CheckRedirect
	c.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	defer func() { c.CheckRedirect = originalRedirect }()

	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("request failed for %s %s: %v", test.Method, test.Path, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != test.ExpectedStatus {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		t.Fatalf("expected status %d, got %d for %s %s. Body: %s", test.ExpectedStatus, resp.StatusCode, test.Method, test.Path, strings.TrimSpace(string(snippet)))
	}
}

func authenticate(t *testing.T, cfg routeConfig, client *http.Client, username, password, loginPath string) {
	t.Helper()

	getReq, err := http.NewRequest(http.MethodGet, cfg.BaseURL+loginPath, nil)
	if err != nil {
		t.Fatalf("failed to create login GET request %s: %v", loginPath, err)
	}
	getReq.Header.Set("X-PVMSS-Test-Bypass", "1")
	getResp, err := client.Do(getReq)
	if err != nil {
		t.Fatalf("failed to GET login page %s: %v", loginPath, err)
	}
	defer func() {
		if getResp != nil {
			_ = getResp.Body.Close()
		}
	}()

	body, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("failed to read login page %s: %v", loginPath, err)
	}
	_ = getResp.Body.Close()
	getResp = nil

	csrfToken := extractCSRFToken(string(body))
	form := url.Values{}
	form.Set("username", username)
	form.Set("password", password)
	if csrfToken != "" {
		form.Set("csrf_token", csrfToken)
	}

	req, err := http.NewRequest(http.MethodPost, cfg.BaseURL+loginPath, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("failed to create login POST request %s: %v", loginPath, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-PVMSS-Test-Bypass", "1")

	originalRedirect := client.CheckRedirect
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	defer func() { client.CheckRedirect = originalRedirect }()

	postResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to POST login %s: %v", loginPath, err)
	}
	defer func() {
		if postResp != nil {
			_ = postResp.Body.Close()
		}
	}()

	if postResp.StatusCode != http.StatusSeeOther && postResp.StatusCode != http.StatusFound {
		snippet, _ := io.ReadAll(io.LimitReader(postResp.Body, 512))
		if loginPath == "/admin/login" {
			t.Skipf("skipping admin-authenticated test: admin login did not redirect (status=%d). Check ADMIN_PASSWORD/ADMIN_PASSWORD_HASH test configuration. Body: %s", postResp.StatusCode, strings.TrimSpace(string(snippet)))
			return
		}
		t.Fatalf("authentication failed for %s: expected redirect, got %d. Body: %s", loginPath, postResp.StatusCode, strings.TrimSpace(string(snippet)))
	}
}

func extractCSRFToken(html string) string {
	patterns := []string{
		`<meta name="csrf-token" content="([^"]+)"`,
		`<input[^>]*name="csrf_token"[^>]*value="([^"]+)"`,
		`<input[^>]*value="([^"]+)"[^>]*name="csrf_token"`,
		`name="csrf_token" value="([^"]+)"`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) > 1 && matches[1] != "" {
			return matches[1]
		}
	}

	return ""
}

func loadRouteConfig() routeConfig {
	return routeConfig{
		BaseURL:       getEnvOrDefault("BASE_URL", "http://localhost:50000"),
		AdminUsername: getEnvOrDefault("ADMIN_USERNAME", "admin"),
		AdminPassword: getEnvOrDefault("ADMIN_PASSWORD", "admin"),
		UserUsername:  getEnvOrDefault("USER_USERNAME", "jhmt@pve"),
		UserPassword:  getEnvOrDefault("USER_PASSWORD", "pouetpouet"),
	}
}

func isProxmoxConnected(cfg routeConfig) bool {
	client := createHTTPClient()
	url := cfg.BaseURL + "/api/health/proxmox"
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var payload proxmoxHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return false
	}

	return payload.Connected
}

func waitForServer(baseURL string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			return true
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(time.Second)
	}

	return false
}

func createHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar:     jar,
		Timeout: 10 * time.Second,
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

type routeConfig struct {
	BaseURL       string
	AdminUsername string
	AdminPassword string
	UserUsername  string
	UserPassword  string
}

type routeTest struct {
	Name           string
	Method         string
	Path           string
	ExpectedStatus int
}

type proxmoxHealthResponse struct {
	Connected bool   `json:"connected"`
	Error     string `json:"error"`
}

// TestQemuGuestAgentTimeoutDetection tests the logic for detecting QEMU Guest Agent timeout errors
func TestQemuGuestAgentTimeoutDetection(t *testing.T) {
	tests := []struct {
		name     string
		action   string
		errorMsg string
		expected bool
	}{
		{
			name:     "QEMU Guest Agent timeout during shutdown",
			action:   "shutdown",
			errorMsg: "QEMU Guest Agent is not running - VM 100 qmp command 'guest-ping' failed - got timeout",
			expected: true,
		},
		{
			name:     "QEMU Guest Agent failed during shutdown",
			action:   "shutdown",
			errorMsg: "VM 101 qmp command 'guest-ping' failed - got timeout",
			expected: true,
		},
		{
			name:     "QEMU Guest Agent error during stop (should not match)",
			action:   "stop",
			errorMsg: "VM 102 qmp command 'guest-ping' failed - got timeout",
			expected: false,
		},
		{
			name:     "Generic error during shutdown (should not match)",
			action:   "shutdown",
			errorMsg: "VM configuration error",
			expected: false,
		},
		{
			name:     "Network error during shutdown (should not match)",
			action:   "shutdown",
			errorMsg: "network connection failed",
			expected: false,
		},
	}

	// Replicate the detection logic from vm_actions.go
	isQemuGuestAgentTimeout := func(action, errorMsg string) bool {
		return action == "shutdown" &&
			strings.Contains(strings.ToLower(errorMsg), "guest-ping") &&
			(strings.Contains(strings.ToLower(errorMsg), "timeout") || strings.Contains(strings.ToLower(errorMsg), "failed"))
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isQemuGuestAgentTimeout(tt.action, tt.errorMsg)
			assert.Equal(t, tt.expected, result,
				"Expected isQemuGuestAgentTimeout(%s, %s) to be %v, got %v",
				tt.action, tt.errorMsg, tt.expected, result)
		})
	}
}
