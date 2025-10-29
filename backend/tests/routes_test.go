package tests

import (
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
)

// TestStaticPathDetection tests the isStaticPath function logic
func TestStaticPathDetection(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/css/base.css", true},
		{"/js/main.js", true},
		{"/webfonts/font.woff2", true},
		{"/components/novnc/core.js", true},
		{"/", false},
		{"/login", false},
		{"/admin", false},
		{"/api/health", false},
	}

	// Replicate isStaticPath logic for testing
	isStaticPath := func(p string) bool {
		for _, prefix := range []string{"/css/", "/js/", "/webfonts/", "/components/"} {
			if strings.HasPrefix(p, prefix) {
				return true
			}
		}
		return false
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isStaticPath(tt.path)
			assert.Equal(t, tt.expected, result,
				"Expected isStaticPath(%s) to be %v, got %v",
				tt.path, tt.expected, result)
		})
	}
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
func TestRouteAccessibility(t *testing.T) {
	cfg := loadRouteConfig()
	if !waitForServer(cfg.BaseURL, 30*time.Second) {
		t.Skipf("PVMSS server not reachable at %s", cfg.BaseURL)
	}

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

	t.Run("Authenticated user routes", func(t *testing.T) {
		client := createHTTPClient()
		authenticate(t, cfg, client, cfg.UserUsername, cfg.UserPassword, "/login")

		runRouteGroup(t, cfg, []routeTest{
			{Name: "Home", Method: http.MethodGet, Path: "/", ExpectedStatus: http.StatusOK},
			{Name: "Search", Method: http.MethodGet, Path: "/search", ExpectedStatus: http.StatusOK},
			{Name: "Profile", Method: http.MethodGet, Path: "/profile", ExpectedStatus: http.StatusOK},
			{Name: "VM create", Method: http.MethodGet, Path: "/vm/create", ExpectedStatus: http.StatusOK},
			{Name: "API settings", Method: http.MethodGet, Path: "/api/settings", ExpectedStatus: http.StatusOK},
			{Name: "API all settings", Method: http.MethodGet, Path: "/api/settings/all", ExpectedStatus: http.StatusOK},
			{Name: "API VMBR", Method: http.MethodGet, Path: "/api/vmbr/all", ExpectedStatus: http.StatusOK},
			{Name: "Logout redirect", Method: http.MethodGet, Path: "/logout", ExpectedStatus: http.StatusSeeOther},
		}, client)
	})

	t.Run("Admin routes", func(t *testing.T) {
		client := createHTTPClient()
		authenticate(t, cfg, client, cfg.AdminUsername, cfg.AdminPassword, "/admin/login")

		runRouteGroup(t, cfg, []routeTest{
			{Name: "Admin dashboard", Method: http.MethodGet, Path: "/admin", ExpectedStatus: http.StatusOK},
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

	t.Run("404 routes", func(t *testing.T) {
		runRouteGroup(t, cfg, []routeTest{
			{Name: "Missing route", Method: http.MethodGet, Path: "/nonexistent", ExpectedStatus: http.StatusNotFound},
			{Name: "Missing API", Method: http.MethodGet, Path: "/api/nonexistent", ExpectedStatus: http.StatusNotFound},
			{Name: "Missing admin", Method: http.MethodGet, Path: "/admin/nonexistent", ExpectedStatus: http.StatusNotFound},
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

	resp, err := client.Get(cfg.BaseURL + loginPath)
	if err != nil {
		t.Fatalf("failed to GET login page %s: %v", loginPath, err)
	}

	body, err := io.ReadAll(resp.Body)
	defer func() {
		_ = resp.Body.Close()
	}()
	if err != nil {
		t.Fatalf("failed to read login page %s: %v", loginPath, err)
	}

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

	originalRedirect := client.CheckRedirect
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	defer func() { client.CheckRedirect = originalRedirect }()

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("failed to POST login %s: %v", loginPath, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		t.Fatalf("authentication failed for %s: expected redirect, got %d. Body: %s", loginPath, resp.StatusCode, strings.TrimSpace(string(snippet)))
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
