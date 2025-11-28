package tests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"pvmss/app"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserPoolSelfCreationIntegration tests the complete user pool self-creation flow
func TestUserPoolSelfCreationIntegration(t *testing.T) {
	// Set offline mode to avoid actual Proxmox dependency
	t.Setenv("PVMSS_OFFLINE", "true")

	// Create test app
	testApp := app.NewTestApp()

	// Create test server
	ts := httptest.NewServer(testApp.Router)
	defer ts.Close()

	// Test the complete flow: login -> check pool status -> create pool
	t.Run("Complete Pool Self-Creation Flow", func(t *testing.T) {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		// Step 1: Admin login
		loginData := url.Values{
			"username": {"admin"},
			"password": {"admin123"},
		}
		loginResp, err := client.PostForm(ts.URL+"/admin/login", loginData)
		require.NoError(t, err)
		defer func() { _ = loginResp.Body.Close() }()
		assert.Equal(t, http.StatusSeeOther, loginResp.StatusCode)

		// Extract session cookies
		cookies := loginResp.Cookies()
		require.NotEmpty(t, cookies, "Login should set session cookies")

		// Step 2: Get user pool page to check current status
		req, err := http.NewRequest("GET", ts.URL+"/admin/userpool", nil)
		require.NoError(t, err)

		// Add session cookies
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}

		poolPageResp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = poolPageResp.Body.Close() }()
		assert.Equal(t, http.StatusOK, poolPageResp.StatusCode)

		// Read page content
		body, err := io.ReadAll(poolPageResp.Body)
		require.NoError(t, err)
		bodyStr := string(body)

		// Should contain pool status section
		assert.Contains(t, bodyStr, "Your Pool Status", "Page should show current user pool status")
		assert.Contains(t, bodyStr, "Pool Status", "Page should contain pool status information")

		// Step 3: Extract CSRF token and attempt pool creation
		csrfRegex := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)
		matches := csrfRegex.FindSubmatch(body)
		require.True(t, len(matches) > 1, "CSRF token should be found in page")

		csrfToken := string(matches[1])

		// Create pool self-creation request
		createReq, err := http.NewRequest("POST", ts.URL+"/userpool/create-self",
			strings.NewReader("csrf_token="+csrfToken))
		require.NoError(t, err)
		createReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		// Add session cookies
		for _, cookie := range cookies {
			createReq.AddCookie(cookie)
		}

		createResp, err := client.Do(createReq)
		require.NoError(t, err)
		defer func() { _ = createResp.Body.Close() }()

		// Should redirect after processing
		assert.Equal(t, http.StatusSeeOther, createResp.StatusCode)

		// Check redirect location contains success
		location := createResp.Header.Get("Location")
		assert.Contains(t, location, "/admin/userpool", "Should redirect back to user pool page")
		// Note: In offline mode, this might show an error due to no Proxmox connection
	})

	// Test authentication requirements
	t.Run("Authentication Requirements", func(t *testing.T) {
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		// Test without authentication
		resp, err := client.Post(ts.URL+"/userpool/create-self", "application/x-www-form-urlencoded",
			strings.NewReader("csrf_token=fake"))
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		// Should redirect to login
		assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
		location := resp.Header.Get("Location")
		assert.Contains(t, location, "/admin/login", "Should redirect to admin login")
	})

	// Test method restrictions
	t.Run("Method Restrictions", func(t *testing.T) {
		// Test GET method
		resp, err := http.Get(ts.URL + "/userpool/create-self")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		// Should return method not allowed
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})
}

// TestUserPoolStatusDetectionIntegration tests pool status detection in offline mode
func TestUserPoolStatusDetectionIntegration(t *testing.T) {
	// Set offline mode
	t.Setenv("PVMSS_OFFLINE", "true")

	// Create test app
	testApp := app.NewTestApp()

	// Create test server
	ts := httptest.NewServer(testApp.Router)
	defer ts.Close()

	// Create client that doesn't follow redirects
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	t.Run("Pool Status Detection for Authenticated Admin", func(t *testing.T) {
		// Admin login
		loginData := url.Values{
			"username": {"admin"},
			"password": {"admin123"},
		}
		loginResp, err := client.PostForm(ts.URL+"/admin/login", loginData)
		require.NoError(t, err)
		defer func() { _ = loginResp.Body.Close() }()
		assert.Equal(t, http.StatusSeeOther, loginResp.StatusCode)

		// Extract session cookies
		cookies := loginResp.Cookies()
		require.NotEmpty(t, cookies)

		// Get user pool page
		req, err := http.NewRequest("GET", ts.URL+"/admin/userpool", nil)
		require.NoError(t, err)

		// Add session cookies
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Read page content
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		bodyStr := string(body)

		// Should contain current user pool status section
		assert.Contains(t, bodyStr, "Your Pool Status", "Page should show current user pool status")

		// In offline mode, should indicate pool doesn't exist (since no Proxmox connection)
		assert.Contains(t, bodyStr, "does not exist", "Should indicate pool missing in offline mode")
		assert.Contains(t, bodyStr, "Create Your Pool", "Should show pool creation button")
	})
}

// TestUserPoolSelfCreationCSRFIntegration tests CSRF protection in integration
func TestUserPoolSelfCreationCSRFIntegration(t *testing.T) {
	// Set offline mode
	t.Setenv("PVMSS_OFFLINE", "true")

	// Create test app
	testApp := app.NewTestApp()

	// Create test server
	ts := httptest.NewServer(testApp.Router)
	defer ts.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	t.Run("CSRF Protection", func(t *testing.T) {
		loginData := url.Values{
			"username": {"admin"},
			"password": {"admin123"},
		}
		loginResp, err := client.PostForm(ts.URL+"/admin/login", loginData)
		require.NoError(t, err)
		defer func() { _ = loginResp.Body.Close() }()

		// Extract session cookies
		cookies := loginResp.Cookies()
		require.NotEmpty(t, cookies)

		req, err := http.NewRequest("POST", ts.URL+"/userpool/create-self",
			strings.NewReader(""))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		// Add session cookies
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		// Should return forbidden or bad request
		assert.True(t, resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusBadRequest,
			"Should return 403 or 400 for missing CSRF token, got %d", resp.StatusCode)

		req2, err := http.NewRequest("POST", ts.URL+"/userpool/create-self",
			strings.NewReader("csrf_token=invalid-token"))
		require.NoError(t, err)
		req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		// Add session cookies
		for _, cookie := range cookies {
			req2.AddCookie(cookie)
		}

		resp2, err := client.Do(req2)
		require.NoError(t, err)
		defer func() { _ = resp2.Body.Close() }()

		// Should return forbidden or bad request
		assert.True(t, resp2.StatusCode == http.StatusForbidden || resp2.StatusCode == http.StatusBadRequest,
			"Should return 403 or 400 for invalid CSRF token, got %d", resp2.StatusCode)
	})
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
