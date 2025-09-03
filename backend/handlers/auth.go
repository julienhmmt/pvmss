package handlers

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/crypto/bcrypt"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/security"
	"pvmss/state"
)

// AuthHandler handles authentication routes
type AuthHandler struct {
	stateManager state.StateManager
}

// LogoutGet serves a minimal page that auto-submits a POST request to /logout including CSRF token.
// This preserves CSRF protection while allowing logout links to be simple GETs.
func (h *AuthHandler) LogoutGet(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := NewHandlerContext(w, r, "AuthHandler.LogoutGet")

	// Attempt to get CSRF token from context (populated by CSRFGeneratorMiddleware for GET)
	var csrfToken string
	if token, ok := security.CSRFTokenFromContext(r.Context()); ok && token != "" {
		csrfToken = token
	} else {
		// Fallback to session
		if ctx.SessionManager != nil {
			if t, ok := ctx.SessionManager.Get(r.Context(), "csrf_token").(string); ok && t != "" {
				csrfToken = t
			}
		}
	}

	if csrfToken == "" {
		ctx.Log.Warn().Msg("No CSRF token available for logout form; generating new one")
		if ctx.SessionManager != nil {
			if t, err := security.GenerateCSRFToken(); err == nil {
				csrfToken = t
				ctx.SessionManager.Put(r.Context(), "csrf_token", csrfToken)
			}
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Minimal HTML page with auto-submitting POST form containing CSRF token
	_, _ = w.Write([]byte(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <title>Logging out…</title>
  </head>
  <body>
    <form id="logoutForm" method="POST" action="/logout">
      <input type="hidden" name="csrf_token" value="` + csrfToken + `" />
    </form>
    <noscript>
      <p>JavaScript is required to log out automatically.</p>
      <button type="submit" form="logoutForm">Log out</button>
    </noscript>
    <script>document.getElementById('logoutForm').submit();</script>
  </body>
</html>`))
}

// NewAuthHandler creates a new instance of AuthHandler
func NewAuthHandler(sm state.StateManager) *AuthHandler {
	return &AuthHandler{stateManager: sm}
}

// RedirectIfAuthenticated is middleware that redirects authenticated users away from login page
func (h *AuthHandler) RedirectIfAuthenticated(next httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if IsAuthenticated(r) {
			// Redirect authenticated users to VM creation page
			http.Redirect(w, r, "/vm/create", http.StatusSeeOther)
			return
		}
		next(w, r, ps)
	}
}

// ShowAdminLoginForm renders the admin login page.
func (h *AuthHandler) ShowAdminLoginForm(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().Str("handler", "AuthHandler.ShowAdminLoginForm").Logger()
	log.Debug().Msg("Displaying admin login form")
	h.renderAdminLoginForm(w, r, "")
}

// RegisterRoutes registers authentication routes
func (h *AuthHandler) RegisterRoutes(router *httprouter.Router) {
	// User login routes
	router.GET("/login", h.RedirectIfAuthenticated(h.ShowLoginForm))
	router.POST("/login", h.handleLogin)

	// Admin login routes
	router.GET("/admin/login", h.ShowAdminLoginForm)
	router.POST("/admin/login", h.handleAdminLogin)

	// Logout routes
	router.GET("/logout", h.LogoutGet)
	router.POST("/logout", h.LogoutHandler)
}

// ShowLoginForm renders the user login page.
func (h *AuthHandler) ShowLoginForm(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().Str("handler", "AuthHandler.ShowLoginForm").Logger()
	log.Debug().Msg("Displaying login form")
	h.renderLoginForm(w, r, "")
}

// LogoutHandler handles user logout
func (h *AuthHandler) LogoutHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().
		With().
		Str("handler", "AuthHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	// Prefer session from middleware context
	sessionManager := security.GetSession(r)
	if sessionManager == nil {
		// Fallback to state manager if needed
		sessionManager = h.stateManager.GetSessionManager()
	}

	log.Info().Msg("User logging out")

	// Clear all session data
	sessionManager.Clear(r.Context())

	// Regenerate session token to prevent session fixation
	err := sessionManager.RenewToken(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to renew session token during logout")
	}

	// Add cache control headers to prevent caching
	headers := w.Header()
	headers.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	headers.Set("Pragma", "no-cache")
	headers.Set("Expires", "0")

	// Redirect to login page with a fresh session
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// getOrSetCSRFToken retrieves a CSRF token from the request context or session, generating a new one if necessary.
func getOrSetCSRFToken(r *http.Request) (string, error) {
	log := logger.Get().With().Str("function", "getOrSetCSRFToken").Logger()

	// Get session manager
	sessionManager := security.GetSession(r)
	if sessionManager == nil {
		return "", fmt.Errorf("session manager not available")
	}

	// Try to get CSRF token from context first (set by middleware)
	if token, ok := security.CSRFTokenFromContext(r.Context()); ok && token != "" {
		log.Debug().Msg("Using CSRF token from request context")
		return token, nil
	}

	// Fallback to session
	if token, ok := sessionManager.Get(r.Context(), "csrf_token").(string); ok && token != "" {
		log.Debug().Msg("Using CSRF token from session")
		return token, nil
	}

	// Generate a new token if none exists
	token, err := security.GenerateCSRFToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate CSRF token: %w", err)
	}
	sessionManager.Put(r.Context(), "csrf_token", token)
	log.Debug().Msg("Generated and stored new CSRF token")

	return token, nil
}

func (h *AuthHandler) renderAdminLoginForm(w http.ResponseWriter, r *http.Request, errorMsg string) {
	ctx := NewHandlerContext(w, r, "AuthHandler.renderAdminLoginForm")
	ctx.Log.Debug().Msg("Rendering admin login form")

	if !ctx.ValidateSessionManager() {
		return
	}

	csrfToken, err := ctx.GetCSRFToken()
	if err != nil {
		ctx.HandleError(err, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data := map[string]interface{}{
		"Title":       "Admin Login",
		"Error":       errorMsg,
		"CSRFToken":   csrfToken,
		"RedirectURL": r.URL.Query().Get("redirect"),
		"ReturnURL":   r.URL.Query().Get("return"),
	}

	ctx.RenderTemplate("admin_login", data)
}

// validateCSRF checks the CSRF token from the form against the one in the session.
func validateCSRF(r *http.Request) error {
	log := logger.Get().With().Str("function", "validateCSRF").Logger()

	sessionManager := security.GetSession(r)
	if sessionManager == nil {
		return fmt.Errorf("session manager not available")
	}

	formToken := r.FormValue("csrf_token")
	if formToken == "" {
		log.Warn().Msg("CSRF token is missing from form")
		return fmt.Errorf("invalid request")
	}

	sessionToken, ok := sessionManager.Get(r.Context(), "csrf_token").(string)
	if !ok || sessionToken == "" {
		log.Warn().Msg("No CSRF token found in session")
		return fmt.Errorf("session expired")
	}

	if subtle.ConstantTimeCompare([]byte(formToken), []byte(sessionToken)) != 1 {
		log.Warn().Msg("CSRF token validation failed")
		return fmt.Errorf("invalid request")
	}

	return nil
}

// handleAdminLogin handles admin login form submission (password-only)
func (h *AuthHandler) handleAdminLogin(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := NewHandlerContext(w, r, "AuthHandler.handleAdminLogin")

	if !ctx.ValidateSessionManager() {
		return
	}

	if err := validateCSRF(r); err != nil {
		errMsg := "Invalid request. Please try again."
		if err.Error() == "session expired" {
			errMsg = "Session expired. Please try again."
		}
		h.renderAdminLoginForm(w, r, errMsg)
		return
	}

	// Get admin password hash from environment
	adminHash := os.Getenv("ADMIN_PASSWORD_HASH")
	if adminHash == "" {
		ctx.Log.Error().Msg("ADMIN_PASSWORD_HASH is not set in environment variables")
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	// Get password from form
	password := r.FormValue("password")
	if password == "" {
		ctx.Log.Debug().Msg("Admin login attempt with empty password")
		h.renderAdminLoginForm(w, r, "Password cannot be empty.")
		return
	}

	// Basic input validation
	if len(password) > 200 {
		ctx.Log.Warn().Int("password_length", len(password)).Msg("Admin login attempt with too long password")
		h.renderAdminLoginForm(w, r, "Invalid credentials.")
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(adminHash), []byte(password)); err != nil {
		ctx.Log.Info().Err(err).Msg("Admin login failed - incorrect password")
		h.renderAdminLoginForm(w, r, "Invalid credentials.")
		return
	}

	ctx.Log.Debug().Msg("Admin authentication successful, creating session")

	if err := establishSession(w, r, true, ""); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	redirectURL := getRedirectURL(r, "/admin/nodes")
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// handleLogin handles the user login form submission (username + password via Proxmox)
func (h *AuthHandler) handleLogin(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().
		With().
		Str("handler", "AuthHandler").
		Str("method", r.Method).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	// Get session manager
	sessionManager := security.GetSession(r)
	if sessionManager == nil {
		log.Error().Msg("Session manager not available")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := validateCSRF(r); err != nil {
		errMsg := "Invalid request. Please try again."
		if err.Error() == "session expired" {
			errMsg = "Session expired. Please try again."
		}
		h.renderLoginForm(w, r, errMsg)
		return
	}

	// Get username and password from form
	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		log.Debug().Msg("User login attempt with empty username or password")
		h.renderLoginForm(w, r, "Username and password are required.")
		return
	}

	// Basic input validation
	if len(username) > 100 || len(password) > 200 {
		log.Warn().Int("username_length", len(username)).Int("password_length", len(password)).Msg("User login attempt with too long credentials")
		h.renderLoginForm(w, r, "Invalid credentials.")
		return
	}

	// Create a new Proxmox client for user authentication
	proxmoxURL := strings.TrimSpace(os.Getenv("PROXMOX_URL"))
	insecureSkip := strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL")) == "false"

	if proxmoxURL == "" {
		log.Error().Msg("PROXMOX_URL is not configured")
		h.renderLoginForm(w, r, "Authentication service unavailable. Please try again later.")
		return
	}

	// We create a new cookie-based client for user/pass login
	pxClient, err := proxmox.NewClientCookieAuth(proxmoxURL, insecureSkip)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Proxmox client for user authentication")
		h.renderLoginForm(w, r, "Authentication service unavailable. Please try again later.")
		return
	}

	// Attempt to authenticate user via Proxmox with "@pve" realm
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := pxClient.Login(ctx, username, password, "pve"); err != nil {
		log.Info().Err(err).Str("username", username).Msg("User login failed - Proxmox authentication failed")
		h.renderLoginForm(w, r, "Invalid credentials.")
		return
	}

	// Also establish cookie-based authentication for console access
	// Store the cookie auth credentials in session for console access
	sessionManager.Put(r.Context(), "pve_auth_cookie", pxClient.PVEAuthCookie)
	sessionManager.Put(r.Context(), "csrf_prevention_token", pxClient.CSRFPreventionToken)
	log.Debug().Str("username", username).Msg("Cookie authentication established for console access")

	log.Debug().Str("username", username).Msg("User authentication successful via Proxmox, creating session")

	if err := establishSession(w, r, false, username); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	redirectURL := getRedirectURL(r, "/vm/create")
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// establishSession renews the session token and sets authentication data.
func establishSession(_ http.ResponseWriter, r *http.Request, isAdmin bool, username string) error {
	log := logger.Get().With().Str("function", "establishSession").Logger()

	sessionManager := security.GetSession(r)
	if sessionManager == nil {
		return fmt.Errorf("session manager not available")
	}

	// Renew session token to prevent session fixation
	if err := sessionManager.RenewToken(r.Context()); err != nil {
		log.Error().Err(err).Msg("Failed to renew session token")
		return fmt.Errorf("internal server error")
	}

	// Store authentication data
	sessionManager.Put(r.Context(), "authenticated", true)
	sessionManager.Put(r.Context(), "is_admin", isAdmin)
	if username != "" {
		sessionManager.Put(r.Context(), "username", username)
	}

	// Generate a new CSRF token for the new session
	newCSRFToken, err := security.GenerateCSRFToken()
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate new CSRF token")
		return fmt.Errorf("internal server error")
	}
	sessionManager.Put(r.Context(), "csrf_token", newCSRFToken)

	log.Info().
		Str("session_id", sessionManager.Token(r.Context())).
		Str("username", username).
		Bool("is_admin", isAdmin).
		Msg("User session established")

	return nil
}

// getRedirectURL determines the redirect URL from form values or query parameters.
func getRedirectURL(r *http.Request, defaultURL string) string {
	// Check form values first, then query parameters
	url := r.FormValue("return")
	if url == "" {
		url = r.URL.Query().Get("return")
	}
	if url == "" {
		url = r.FormValue("redirect")
	}
	if url == "" {
		url = r.URL.Query().Get("redirect")
	}

	// Use default if no URL is found
	if url == "" {
		url = defaultURL
	}

	// Ensure the URL is a local path
	if len(url) > 0 && url[0] != '/' {
		url = "/" + url
	}

	return url
}

func (h *AuthHandler) renderLoginForm(w http.ResponseWriter, r *http.Request, errorMsg string) {
	ctx := NewHandlerContext(w, r, "AuthHandler.renderLoginForm")
	ctx.Log.Debug().Msg("Rendering login form")

	if !ctx.ValidateSessionManager() {
		return
	}

	csrfToken, err := ctx.GetCSRFToken()
	if err != nil {
		ctx.HandleError(err, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Prepare template data with CSRF token
	data := map[string]interface{}{
		"Title":       "Login",
		"Error":       errorMsg,
		"CSRFToken":   csrfToken,
		"RedirectURL": r.URL.Query().Get("redirect"),
		"ReturnURL":   r.URL.Query().Get("return"),
	}

	ctx.RenderTemplate("login", data)
}
