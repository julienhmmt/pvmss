package tests

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"pvmss/app"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPublicRoutes tests that all public routes are accessible
func TestPublicRoutes(t *testing.T) {
	// Set offline mode for GitHub Actions compatibility
	if strings.EqualFold(os.Getenv("PVMSS_OFFLINE"), "true") || os.Getenv("CI") != "" {
		t.Setenv("PVMSS_OFFLINE", "true")
	}

	testApp := app.MakeTestApp()

	ts := httptest.NewServer(testApp.Router)
	defer ts.Close()

	publicRoutes := []struct {
		name string
		path string
	}{
		{"Health check", "/health"},
		{"API v1 health", "/api/v1/health"},
		{"API v1 proxmox health", "/api/v1/health/proxmox"},
		{"Favicon", "/favicon.ico"},
	}

	for _, route := range publicRoutes {
		t.Run(route.name, func(t *testing.T) {
			resp, err := http.Get(ts.URL + route.path)
			if err != nil {
				t.Fatalf("Failed to get %s: %v", route.path, err)
			}
			defer func() { _ = resp.Body.Close() }()

			// All public routes should return 200 or 404 (for missing static files)
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
				t.Errorf("Expected status 200 or 404 for %s, got %d", route.path, resp.StatusCode)
			}
		})
	}
}

// TestProtectedRoutes tests that protected routes redirect to login
func TestProtectedRoutes(t *testing.T) {
	// Set offline mode for GitHub Actions compatibility
	if strings.EqualFold(os.Getenv("PVMSS_OFFLINE"), "true") || os.Getenv("CI") != "" {
		t.Setenv("PVMSS_OFFLINE", "true")
	}

	testApp := app.MakeTestApp()

	ts := httptest.NewServer(testApp.Router)
	defer ts.Close()

	// Create client that doesn't follow redirects
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	protectedRoutes := []struct {
		name string
		path string
	}{
		{"Profile", "/profile"},
		{"Search", "/search"},
		{"Admin dashboard", "/admin"},
		{"Admin nodes", "/admin/nodes"},
		{"Admin tags", "/admin/tags"},
		{"Admin storage", "/admin/storage"},
		{"Admin ISO", "/admin/iso"},
		{"Admin VMBR", "/admin/vmbr"},
		{"Admin limits", "/admin/limits"},
		{"Admin user pool", "/admin/userpool"},
		{"Admin app info", "/admin/appinfo"},
	}

	for _, route := range protectedRoutes {
		t.Run(route.name, func(t *testing.T) {
			resp, err := client.Get(ts.URL + route.path)
			if err != nil {
				t.Fatalf("Failed to get %s: %v", route.path, err)
			}
			defer func() { _ = resp.Body.Close() }()

			// Protected routes should redirect to login
			if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
				t.Errorf("Expected redirect status for %s, got %d", route.path, resp.StatusCode)
			}
			if resp.StatusCode == http.StatusSeeOther || resp.StatusCode == http.StatusFound {
				location := resp.Header.Get("Location")
				expectedLocation := "/login"
				if strings.HasPrefix(route.path, "/admin") {
					expectedLocation = "/admin/login"
				}
				if location != expectedLocation {
					t.Errorf("Expected redirect to %s for %s, got %s", expectedLocation, route.path, location)
				}
			}
		})
	}
}

// TestCSRFProtection tests that POST without authentication fails
func TestCSRFProtection(t *testing.T) {
	// Set offline mode for GitHub Actions compatibility
	if strings.EqualFold(os.Getenv("PVMSS_OFFLINE"), "true") || os.Getenv("CI") != "" {
		t.Setenv("PVMSS_OFFLINE", "true")
	}

	testApp := app.MakeTestApp()

	ts := httptest.NewServer(testApp.Router)
	defer ts.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Test POST without session should redirect to login
	req, err := http.NewRequest("POST", ts.URL+"/admin/nodes",
		strings.NewReader(""))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	// Should redirect to login (302/303)
	assert.True(t, resp.StatusCode == http.StatusSeeOther || resp.StatusCode == http.StatusFound,
		"Should redirect for POST without session, got %d", resp.StatusCode)

	if resp.StatusCode == http.StatusSeeOther || resp.StatusCode == http.StatusFound {
		location := resp.Header.Get("Location")
		assert.Contains(t, location, "/admin/login", "Should redirect to admin login")
	}
}
