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

// TestAuthenticatedUserFlow tests the complete user authentication flow
func TestAuthenticatedUserFlow(t *testing.T) {
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

	// Step 1: User login
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

	// Step 2: Access protected route with authentication
	req, err := http.NewRequest("GET", ts.URL+"/profile", nil)
	require.NoError(t, err)

	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	profileResp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = profileResp.Body.Close() }()
	assert.Equal(t, http.StatusOK, profileResp.StatusCode, "Should access profile with valid session")

	// Step 3: Logout
	logoutReq, err := http.NewRequest("GET", ts.URL+"/logout", nil)
	require.NoError(t, err)

	for _, cookie := range cookies {
		logoutReq.AddCookie(cookie)
	}

	logoutResp, err := client.Do(logoutReq)
	require.NoError(t, err)
	defer func() { _ = logoutResp.Body.Close() }()
	assert.Equal(t, http.StatusSeeOther, logoutResp.StatusCode, "Logout should redirect")
}

// TestAuthenticatedAdminFlow tests the complete admin authentication flow
func TestAuthenticatedAdminFlow(t *testing.T) {
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

	// Step 2: Access admin pages
	adminPages := []string{
		"/admin/nodes",
		"/admin/tags",
		"/admin/storage",
		"/admin/iso",
		"/admin/vmbr",
		"/admin/limits",
		"/admin/userpool",
		"/admin/appinfo",
	}

	for _, page := range adminPages {
		t.Run("Admin page: "+page, func(t *testing.T) {
			req, err := http.NewRequest("GET", ts.URL+page, nil)
			require.NoError(t, err)

			for _, cookie := range cookies {
				req.AddCookie(cookie)
			}

			pageResp, err := client.Do(req)
			require.NoError(t, err)
			defer func() { _ = pageResp.Body.Close() }()
			assert.Equal(t, http.StatusOK, pageResp.StatusCode, "Should access admin page with valid session")
		})
	}
}

// TestCSRFProtection tests CSRF token validation
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

	// Step 1: Admin login to get session
	loginData := url.Values{
		"username": {"admin"},
		"password": {"admin123"},
	}
	loginResp, err := client.PostForm(ts.URL+"/admin/login", loginData)
	require.NoError(t, err)
	defer func() { _ = loginResp.Body.Close() }()

	cookies := loginResp.Cookies()
	require.NotEmpty(t, cookies)

	// Step 2: Get a page to extract CSRF token
	pageResp, err := client.Get(ts.URL + "/admin/nodes")
	require.NoError(t, err)
	defer func() { _ = pageResp.Body.Close() }()

	body, err := io.ReadAll(pageResp.Body)
	require.NoError(t, err)

	csrfRegex := regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)
	matches := csrfRegex.FindSubmatch(body)
	require.True(t, len(matches) > 1, "CSRF token should be found in page")

	// Step 3: Test POST without CSRF token
	t.Run("Missing CSRF token", func(t *testing.T) {
		req, err := http.NewRequest("POST", ts.URL+"/admin/nodes",
			strings.NewReader(""))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.True(t, resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusBadRequest,
			"Should return 403 or 400 for missing CSRF token, got %d", resp.StatusCode)
	})

	// Step 4: Test POST with invalid CSRF token
	t.Run("Invalid CSRF token", func(t *testing.T) {
		req, err := http.NewRequest("POST", ts.URL+"/admin/nodes",
			strings.NewReader("csrf_token=invalid-token"))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.True(t, resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusBadRequest,
			"Should return 403 or 400 for invalid CSRF token, got %d", resp.StatusCode)
	})
}
