package tests

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"pvmss/app"
)

// TestOfflineMode tests that the application works correctly in offline mode
func TestOfflineMode(t *testing.T) {
	// Set offline mode
	t.Setenv("PVMSS_OFFLINE", "true")

	// Create test app with minimal setup
	testApp := app.NewTestApp()

	// Create test server
	ts := httptest.NewServer(testApp.Router)
	defer ts.Close()

	// Test health endpoint
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to get health endpoint: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test Proxmox health endpoint (should return offline)
	resp, err = http.Get(ts.URL + "/api/health/proxmox")
	if err != nil {
		t.Fatalf("Failed to get Proxmox health endpoint: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

// TestOnlineMode tests that the application works correctly in online mode
func TestOnlineMode(t *testing.T) {
	// Ensure we're not in offline mode
	t.Setenv("PVMSS_OFFLINE", "false")

	// Create test app with minimal setup
	testApp := app.NewTestApp()

	// Create test server
	ts := httptest.NewServer(testApp.Router)
	defer ts.Close()

	// Test health endpoint
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to get health endpoint: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

// TestPublicRoutes tests that all public routes are accessible
func TestPublicRoutes(t *testing.T) {
	// Set offline mode for GitHub Actions compatibility
	if strings.EqualFold(os.Getenv("PVMSS_OFFLINE"), "true") || os.Getenv("CI") != "" {
		t.Setenv("PVMSS_OFFLINE", "true")
	}

	testApp := app.NewTestApp()

	ts := httptest.NewServer(testApp.Router)
	defer ts.Close()

	publicRoutes := []struct {
		name string
		path string
	}{
		{"Health check", "/health"},
		{"API health", "/api/health"},
		{"Proxmox health", "/api/health/proxmox"},
		{"Login page", "/login"},
		{"Admin login", "/admin/login"},
		{"Docs root", "/docs"},
		{"User docs", "/docs/user"},
		{"Admin docs", "/docs/admin"},
		{"Favicon", "/favicon.ico"},
		{"Base CSS", "/css/base.css"},
		{"Accessibility JS", "/js/accessibility.js"},
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

	testApp := app.NewTestApp()

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
		{"VM create", "/vm/create"},
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

// TestAPIEndpoints tests basic API functionality
func TestAPIEndpoints(t *testing.T) {
	// Set offline mode for GitHub Actions compatibility
	if strings.EqualFold(os.Getenv("PVMSS_OFFLINE"), "true") || os.Getenv("CI") != "" {
		t.Setenv("PVMSS_OFFLINE", "true")
	}

	testApp := app.NewTestApp()

	ts := httptest.NewServer(testApp.Router)
	defer ts.Close()

	apiEndpoints := []struct {
		name string
		path string
	}{
		{"Settings API", "/api/settings"},
		{"All settings API", "/api/settings/all"},
		{"VMBR API", "/api/vmbr/all"},
	}

	for _, endpoint := range apiEndpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			resp, err := http.Get(ts.URL + endpoint.path)
			if err != nil {
				t.Fatalf("Failed to get %s: %v", endpoint.path, err)
			}
			defer func() { _ = resp.Body.Close() }()

			// API endpoints should return 200 (even in offline mode with empty data)
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200 for %s, got %d", endpoint.path, resp.StatusCode)
			}
		})
	}
}
