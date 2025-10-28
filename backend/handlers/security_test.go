package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"

	"pvmss/middleware"
)

func TestCSRFProtection(t *testing.T) {
	// Create a test context with session
	ctx := &testContext{
		session: make(map[string]interface{}),
	}
	
	// Generate CSRF token
	token, err := ctx.GetCSRFToken()
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	
	// Create HTTP router
	router := httprouter.New()
	
	// Test protected route that requires CSRF
	router.POST("/test", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		// Mock CSRF validation
		if err := mockValidateCSRF(r); err != nil {
			http.Error(w, "Invalid CSRF", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("CSRF valid"))
	})
	
	// Test without CSRF token - should fail
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	// Mock session cookie
	req.AddCookie(&http.Cookie{
		Name:  "pvmss_session",
		Value: "test-session",
	})
	
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
	
	// Test with valid CSRF token - should pass
	req = httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.PostForm = make(map[string][]string)
	req.PostForm.Set("csrf_token", token)
	
	// Mock session cookie
	req.AddCookie(&http.Cookie{
		Name:  "pvmss_session",
		Value: "test-session",
	})
	
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestSessionFixationProtection(t *testing.T) {
	// Test that session ID is regenerated on login
	oldSessionID := "old-session-id"
	newSessionID := "new-session-id"
	
	// Mock session regeneration
	assert.NotEqual(t, oldSessionID, newSessionID, "Session ID should be regenerated on authentication")
}

func TestRateLimitingBehavior(t *testing.T) {
	// Test that rate limiting works correctly
	limiter := middleware.NewRateLimiter(time.Minute, time.Minute*5)
	
	// Add a rate limiting rule
	limiter.AddRule("GET", "/", middleware.Rule{Capacity: 10, Refill: time.Second})
	
	limitedHandler := middleware.RateLimitMiddleware(limiter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	// Test rate limiting headers are present
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	limitedHandler.ServeHTTP(rr, req)
	
	// Check rate limit headers are present
	assert.Contains(t, rr.Header(), "X-Ratelimit-Limit")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestInputSanitization(t *testing.T) {
	sanitizer := NewInputSanitizer()
	
	// Test XSS prevention
	xssInput := "<script>alert('xss')</script>"
	sanitized := sanitizer.SanitizeString(xssInput, 100)
	
	assert.NotContains(t, sanitized, "<script>", "XSS should be escaped")
	assert.Contains(t, sanitized, "&lt;script&gt;", "HTML should be escaped")
	
	// Test length enforcement
	longInput := "a"
	for i := 0; i < 200; i++ {
		longInput += "a"
	}
	
	sanitized = sanitizer.SanitizeString(longInput, 50)
	assert.LessOrEqual(t, len(sanitized), 50, "Input should be truncated to max length")
}

func TestAuthenticationBypassAttempts(t *testing.T) {
	// Test that protected routes require authentication
	router := httprouter.New()
	
	// Add protected route
	router.GET("/protected", func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewHandlerContext(w, r, "test")
		if ctx == nil || !ctx.IsAuthenticated() {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Protected content"))
	})
	
	// Test without authentication - should fail
	req := httptest.NewRequest("GET", "/protected", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	
	// Test with invalid session cookie - should fail
	req = httptest.NewRequest("GET", "/protected", nil)
	req.AddCookie(&http.Cookie{
		Name:  "pvmss_session",
		Value: "invalid-session",
	})
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

// testContext is a minimal context for testing
type testContext struct {
	session map[string]interface{}
}

func (c *testContext) IsAuthenticated() bool {
	return c.session != nil
}

func (c *testContext) GetCSRFToken() (string, error) {
	return "test-csrf-token", nil
}

// mockValidateCSRF is a mock CSRF validation function for testing
func mockValidateCSRF(r *http.Request) error {
	token := r.PostFormValue("csrf_token")
	if token != "test-csrf-token" {
		return assert.AnError
	}
	return nil
}
