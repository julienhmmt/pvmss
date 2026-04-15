package handlers

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/url"
	pathpkg "path"
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

// LogoutGet handles GET requests to /logout by redirecting to POST /logout.
// For proper CSRF protection, clients should use POST, but we support GET for convenience.
func (h *AuthHandler) LogoutGet(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("AuthHandler.LogoutGet", r)

	// Get username before clearing session for audit
	username := ""
	if sessionManager := security.GetSession(r); sessionManager != nil {
		if user, ok := sessionManager.Get(r.Context(), "username").(string); ok {
			username = user
		}
	}

	logger.AuthEvent("logout").
		Str("username", username).
		Str("client_ip", r.RemoteAddr).
		Str("logout_method", "GET").
		Msg("User logout")

	log.Debug().
		Str("component", "auth").
		Str("operation", "logout").
		Str("reason", "get_request").
		Msg("Processing logout via GET")

	// Get session manager
	sessionManager := security.GetSession(r)
	if sessionManager == nil {
		sessionManager = h.stateManager.GetSessionManager()
	}

	// Clear all session data
	if err := sessionManager.Clear(r.Context()); err != nil {
		log.Error().Err(err).Msg("Failed to clear session during logout")
	}

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

// MakeAuthHandler creates a new instance of AuthHandler
func MakeAuthHandler(sm state.StateManager) *AuthHandler {
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
	log := CreateHandlerLogger("AuthHandler.ShowAdminLoginForm", r)
	log.Debug().
		Str("component", "auth").
		Str("operation", "admin_login_form").
		Str("reason", "form_display").
		Msg("Displaying admin login form")
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
	router.POST("/admin/proxmox-login", h.handleProxmoxAdminLogin)

	// Logout routes
	router.GET("/logout", h.LogoutGet)
	router.POST("/logout", h.LogoutHandler)
}

// ShowLoginForm renders the user login page.
func (h *AuthHandler) ShowLoginForm(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("AuthHandler.ShowLoginForm", r)
	log.Debug().
		Str("component", "auth").
		Str("operation", "login_form").
		Str("reason", "form_display").
		Msg("Displaying login form")
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

	// Get username before clearing session for audit
	username := ""
	if user, ok := sessionManager.Get(r.Context(), "username").(string); ok {
		username = user
	}

	logger.AuthEvent("logout").
		Str("username", username).
		Str("client_ip", r.RemoteAddr).
		Str("logout_method", "POST").
		Msg("User logout")

	log.Debug().Msg("Processing logout")

	// Clear all session data
	if err := sessionManager.Clear(r.Context()); err != nil {
		log.Error().Err(err).Msg("Failed to clear session during logout")
	}

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

func (h *AuthHandler) renderAdminLoginForm(w http.ResponseWriter, r *http.Request, _ string) {
	// Login is now handled by the SvelteKit SPA at /login.
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// validateCSRF checks the CSRF token from the form against the one in the session.
func validateCSRF(r *http.Request) error {
	// log := CreateHandlerLogger("validateCSRF", r)

	sessionManager := security.GetSession(r)
	if sessionManager == nil {
		return fmt.Errorf("session manager not available")
	}

	formToken := r.FormValue("csrf_token")
	if formToken == "" {
		logger.SecurityEvent("csrf_form_missing").
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("client_ip", r.RemoteAddr).
			Msg("CSRF token missing from form")
		return fmt.Errorf("invalid request")
	}

	sessionToken, ok := sessionManager.Get(r.Context(), "csrf_token").(string)
	if !ok || sessionToken == "" {
		logger.SecurityEvent("csrf_session_missing").
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("client_ip", r.RemoteAddr).
			Msg("No CSRF token found in session")
		return fmt.Errorf("session expired")
	}

	if subtle.ConstantTimeCompare([]byte(formToken), []byte(sessionToken)) != 1 {
		logger.SecurityEvent("csrf_validation_failed").
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("client_ip", r.RemoteAddr).
			Msg("CSRF token validation failed")
		return fmt.Errorf("invalid request")
	}

	return nil
}

// handleAdminLogin handles admin login form submission (password-only)
func (h *AuthHandler) handleAdminLogin(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := HandlerContextWith(w, r, "AuthHandler.handleAdminLogin")

	if !ctx.ValidateSessionManager() {
		return
	}

	if err := validateCSRF(r); err != nil {
		errMsg := "INVALID_REQUEST"
		if err.Error() == "session expired" {
			errMsg = "SESSION_EXPIRED"
		}
		h.renderAdminLoginForm(w, r, errMsg)
		return
	}

	// Get admin password hash from environment configuration
	envCfg := h.stateManager.GetEnvConfig()
	adminHash := envCfg.AdminPasswordHash
	if adminHash == "" {
		ctx.Log.Error().Msg("ADMIN_PASSWORD_HASH is not set in environment variables")
		http.Error(w, "SERVER_CONFIG_ERROR", http.StatusInternalServerError)
		return
	}

	// Get password from form
	password := r.FormValue("password")
	if password == "" {
		ctx.Log.Debug().
			Str("component", "auth").
			Str("operation", "admin_login").
			Str("reason", "empty_password").
			Msg("Admin login attempt with empty password")
		h.renderAdminLoginForm(w, r, "EMPTY_PASSWORD")
		return
	}

	// Basic input validation
	if len(password) > 200 {
		ctx.Log.Warn().
			Int("password_length", len(password)).
			Str("component", "auth").
			Str("operation", "admin_login").
			Str("reason", "password_too_long").
			Msg("Admin login attempt with too long password")
		h.renderAdminLoginForm(w, r, "INVALID_CREDENTIALS")
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(adminHash), []byte(password)); err != nil {
		logger.AuthFailure("admin_login", "invalid_password").
			Str("auth_method", "password_hash").
			Str("client_ip", r.RemoteAddr).
			Msg("Admin login failed - incorrect password")
		h.renderAdminLoginForm(w, r, "INVALID_CREDENTIALS")
		return
	}

	ctx.Log.Debug().
		Str("component", "auth").
		Str("operation", "admin_login").
		Str("reason", "auth_success").
		Msg("Admin authentication successful, creating session")

	if err := establishSession(w, r, true, "admin"); err != nil {
		http.Error(w, "INTERNAL_SERVER_ERROR", http.StatusInternalServerError)
		return
	}

	// Log admin authentication success for audit trail
	logger.AuthEvent("admin_login_success").
		Str("username", "admin").
		Str("realm", "builtin").
		Str("auth_method", "password_hash").
		Str("client_ip", r.RemoteAddr).
		Str("user_agent", r.Header.Get("User-Agent")).
		Bool("is_admin", true).
		Msg("Admin login successful via password hash")

	// Persist language selection in cookie and append to redirect
	redirectURL := getRedirectURL(r, "/admin")
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
		http.Error(w, "SERVICE_UNAVAILABLE", http.StatusInternalServerError)
		return
	}

	if err := validateCSRF(r); err != nil {
		errMsg := "INVALID_REQUEST"
		if err.Error() == "session expired" {
			errMsg = "SESSION_EXPIRED"
		}
		h.renderLoginForm(w, r, errMsg)
		return
	}

	// Get username and password from form
	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		logger.SecurityEvent("login_empty_credentials").
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("client_ip", r.RemoteAddr).
			Str("auth_type", "user").
			Msg("Login attempt with empty username or password")
		h.renderLoginForm(w, r, "MISSING_CREDENTIALS")
		return
	}

	// Basic input validation
	if len(username) > 100 || len(password) > 200 {
		log.Warn().
			Str("ip", r.RemoteAddr).
			Str("username_preview", username).
			Int("username_length", len(username)).
			Int("password_length", len(password)).
			Msg("User login attempt with too long credentials")
		h.renderLoginForm(w, r, "INVALID_CREDENTIALS")
		return
	}

	// Create a new Proxmox client for user authentication
	envCfg := h.stateManager.GetEnvConfig()
	proxmoxURL := envCfg.ProxmoxURL
	insecureSkip := !envCfg.ProxmoxSSLVerify

	if proxmoxURL == "" {
		log.Error().Msg("PROXMOX_URL is not configured")
		h.renderLoginForm(w, r, "SERVICE_UNAVAILABLE")
		return
	}

	// We create a new cookie-based client for user/pass login
	pxClient, err := proxmox.MakeRestyClientCookieAuth(proxmoxURL, insecureSkip, 10*time.Second)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Proxmox client for user authentication")
		h.renderLoginForm(w, r, "SERVICE_UNAVAILABLE")
		return
	}

	// Attempt to authenticate user via Proxmox with "@pve" realm
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Use CreateTicketResty to get the authentication ticket with full response
	ticketResp, err := proxmox.CreateTicketResty(ctx, pxClient, username, password, &proxmox.CreateTicketOptions{
		Realm: "pve",
	})
	if err != nil {
		logger.AuthFailure("user_login", "proxmox_auth_failed").
			Err(err).
			Str("username", username).
			Str("realm", "pve").
			Str("client_ip", r.RemoteAddr).
			Msg("User login failed - Proxmox authentication failed")
		h.renderLoginForm(w, r, "INVALID_CREDENTIALS")
		return
	}

	log.Debug().
		Str("ip", r.RemoteAddr).
		Str("username", username).
		Str("proxmox_username", ticketResp.Username).
		Bool("has_csrf_token", ticketResp.CSRFPreventionToken != "").
		Msg("User authentication successful via Proxmox, creating session")

	// Check if user has PVEAdmin role to determine admin access
	isAdmin := proxmox.HasRole(ticketResp.Cap, "PVEAdmin")
	if isAdmin {
		logger.AuthEvent("admin_login_success").
			Str("username", username).
			Str("proxmox_username", ticketResp.Username).
			Str("realm", "pve").
			Str("auth_method", "proxmox_ticket").
			Str("client_ip", r.RemoteAddr).
			Str("user_agent", r.Header.Get("User-Agent")).
			Bool("is_admin", true).
			Msg("Admin login successful via Proxmox")
	} else {
		logger.AuthEvent("user_login_success").
			Str("username", username).
			Str("proxmox_username", ticketResp.Username).
			Str("realm", "pve").
			Str("auth_method", "proxmox_ticket").
			Str("client_ip", r.RemoteAddr).
			Str("user_agent", r.Header.Get("User-Agent")).
			Bool("is_admin", false).
			Msg("User login successful via Proxmox")
	}

	// Establish session and store Proxmox ticket for later use (console access, API calls)
	if err := establishSessionWithTicket(w, r, isAdmin, username, ticketResp); err != nil {
		http.Error(w, "INTERNAL_SERVER_ERROR", http.StatusInternalServerError)
		return
	}

	// Persist language selection in cookie and append to redirect
	var defaultURL string
	if isAdmin {
		defaultURL = "/admin"
	} else {
		defaultURL = "/vm/create"
	}
	redirectURL := getRedirectURL(r, defaultURL)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// handleProxmoxAdminLogin handles admin login form submission via Proxmox (username + password with PVEAdmin role required)
func (h *AuthHandler) handleProxmoxAdminLogin(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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
		http.Error(w, "SERVICE_UNAVAILABLE", http.StatusInternalServerError)
		return
	}

	if err := validateCSRF(r); err != nil {
		errMsg := "INVALID_REQUEST"
		if err.Error() == "session expired" {
			errMsg = "SESSION_EXPIRED"
		}
		h.renderAdminLoginForm(w, r, errMsg)
		return
	}

	// Get username and password from form
	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		logger.SecurityEvent("login_empty_credentials").
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("client_ip", r.RemoteAddr).
			Str("auth_type", "admin").
			Msg("Admin login attempt with empty username or password")
		h.renderAdminLoginForm(w, r, "MISSING_CREDENTIALS")
		return
	}

	// Basic input validation
	if len(username) > 100 || len(password) > 200 {
		log.Warn().
			Str("ip", r.RemoteAddr).
			Str("username_preview", username).
			Int("username_length", len(username)).
			Int("password_length", len(password)).
			Msg("Admin login attempt with too long credentials")
		h.renderAdminLoginForm(w, r, "INVALID_CREDENTIALS")
		return
	}

	// Create a new Proxmox client for admin authentication
	envCfg := h.stateManager.GetEnvConfig()
	proxmoxURL := envCfg.ProxmoxURL
	insecureSkip := !envCfg.ProxmoxSSLVerify

	if proxmoxURL == "" {
		log.Error().Msg("PROXMOX_URL is not configured")
		h.renderAdminLoginForm(w, r, "SERVICE_UNAVAILABLE")
		return
	}

	// We create a new cookie-based client for admin/pass login
	pxClient, err := proxmox.MakeRestyClientCookieAuth(proxmoxURL, insecureSkip, 10*time.Second)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Proxmox client for admin authentication")
		h.renderAdminLoginForm(w, r, "SERVICE_UNAVAILABLE")
		return
	}

	// Attempt to authenticate admin via Proxmox with "@pve" realm
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Use CreateTicketResty to get the authentication ticket with full response
	ticketResp, err := proxmox.CreateTicketResty(ctx, pxClient, username, password, &proxmox.CreateTicketOptions{
		Realm: "pve",
	})
	if err != nil {
		log.Info().Err(err).
			Str("ip", r.RemoteAddr).
			Str("username", username).
			Msg("Admin login failed - Proxmox authentication failed")
		h.renderAdminLoginForm(w, r, "INVALID_CREDENTIALS")
		return
	}

	log.Debug().
		Str("ip", r.RemoteAddr).
		Str("username", username).
		Str("proxmox_username", ticketResp.Username).
		Bool("has_csrf_token", ticketResp.CSRFPreventionToken != "").
		Msg("Admin authentication successful via Proxmox, checking PVEAdmin role")

	// Check if user has PVEAdmin or PVMSS_Admin role - REQUIRED for admin login
	isAdmin := proxmox.HasRole(ticketResp.Cap, "PVEAdmin") || proxmox.HasRole(ticketResp.Cap, "PVMSS_Admin")
	if !isAdmin {
		log.Info().
			Str("ip", r.RemoteAddr).
			Str("username", username).
			Str("proxmox_username", ticketResp.Username).
			Msg("Admin login denied - User does not have PVEAdmin role")
		h.renderAdminLoginForm(w, r, "INVALID_CREDENTIALS")
		return
	}

	log.Info().
		Str("action", "admin_login").
		Str("username", username).
		Str("proxmox_username", ticketResp.Username).
		Str("realm", "pve").
		Str("client_ip", r.RemoteAddr).
		Str("user_agent", r.Header.Get("User-Agent")).
		Bool("has_pveadmin_role", true).
		Time("login_time", time.Now()).
		Msg("ADMIN LOGIN SUCCESS - Proxmox admin authenticated with PVEAdmin role")

	// Establish session and store Proxmox ticket for later use (console access, API calls)
	if err := establishSessionWithTicket(w, r, true, username, ticketResp); err != nil {
		http.Error(w, "INTERNAL_SERVER_ERROR", http.StatusInternalServerError)
		return
	}

	// Persist language selection in cookie and append to redirect
	redirectURL := getRedirectURL(r, "/admin")
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// establishSession renews the session token and sets authentication data.
func establishSession(_ http.ResponseWriter, r *http.Request, isAdmin bool, username string) error {
	return establishSessionWithTicket(nil, r, isAdmin, username, nil)
}

// establishSessionWithTicket renews the session token, sets authentication data, and stores Proxmox ticket.
func establishSessionWithTicket(_ http.ResponseWriter, r *http.Request, isAdmin bool, username string, ticket *proxmox.TicketResponse) error {
	log := CreateHandlerLogger("establishSessionWithTicket", r)

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

	// Store Proxmox ticket if provided (for console access and API operations)
	if ticket != nil {
		sessionManager.Put(r.Context(), "pve_auth_cookie", ticket.Ticket)
		sessionManager.Put(r.Context(), "pve_csrf_token", ticket.CSRFPreventionToken)
		sessionManager.Put(r.Context(), "pve_username", ticket.Username)
		// Store ticket creation time for renewal checks
		sessionManager.Put(r.Context(), "pve_ticket_created", time.Now().Unix())

		log.Debug().
			Str("pve_username", ticket.Username).
			Bool("has_csrf_token", ticket.CSRFPreventionToken != "").
			Msg("Proxmox ticket stored in session")
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
		Bool("has_pve_ticket", ticket != nil).
		Msg("User session established")

	return nil
}

// GetProxmoxTicketFromSession retrieves the stored Proxmox ticket from the user's session.
// Returns the ticket, CSRF token, and creation timestamp. Returns empty strings if not found.
func GetProxmoxTicketFromSession(r *http.Request) (ticket, csrfToken string, createdAt time.Time, ok bool) {
	// Try context-injected session manager first (app middleware stack).
	// Fall back to state manager for API routes that lack SessionMiddleware.
	sessionManager := security.GetSession(r)
	if sessionManager == nil {
		if sm := getStateManager(r); sm != nil {
			sessionManager = sm.GetSessionManager()
		}
	}
	if sessionManager == nil {
		return "", "", time.Time{}, false
	}

	ticket, ticketOk := sessionManager.Get(r.Context(), "pve_auth_cookie").(string)
	csrfToken, csrfOk := sessionManager.Get(r.Context(), "pve_csrf_token").(string)
	createdUnix, timeOk := sessionManager.Get(r.Context(), "pve_ticket_created").(int64)

	if !ticketOk || ticket == "" {
		return "", "", time.Time{}, false
	}

	if timeOk && createdUnix > 0 {
		createdAt = time.Unix(createdUnix, 0)
	}

	return ticket, csrfToken, createdAt, csrfOk && csrfToken != ""
}

// IsProxmoxTicketValid checks if the stored Proxmox ticket is still valid.
// Proxmox tickets are valid for 2 hours. This function returns false if the ticket
// is missing, older than 1 hour 55 minutes (with 5-minute buffer), or otherwise invalid.
func IsProxmoxTicketValid(r *http.Request) bool {
	_, _, createdAt, ok := GetProxmoxTicketFromSession(r)
	if !ok || createdAt.IsZero() {
		return false
	}

	// Check if ticket is less than 1h55m old (5min buffer before 2h expiration)
	age := time.Since(createdAt)
	return age < (1*time.Hour + 55*time.Minute)
}

// getRedirectURL determines the redirect URL from form values or query parameters.
func getRedirectURL(r *http.Request, defaultURL string) string {
	// Check form values first, then query parameters
	for _, key := range []string{"return", "redirect"} {
		if url := r.FormValue(key); url != "" {
			return ensureLocalPath(url)
		}
		if url := r.URL.Query().Get(key); url != "" {
			return ensureLocalPath(url)
		}
	}

	return ensureLocalPath(defaultURL)
}

// ensureLocalPath ensures the URL is a local path starting with /, on the same host.
func ensureLocalPath(urlStr string) string {
	if urlStr == "" {
		return "/"
	}

	// Normalize backslashes to forward slashes before parsing
	urlStr = strings.ReplaceAll(urlStr, "\\", "/")

	// Quickly reject obvious external or scheme-based URLs
	lower := strings.ToLower(urlStr)
	// Reject raw or encoded schemes or protocol-relative URLs.
	if strings.HasPrefix(lower, "//") || strings.HasPrefix(lower, "/\\") ||
		strings.Contains(lower, "://") ||
		strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") || // encoded slashes
		strings.Contains(lower, "%3a") { // encoded colon
		return "/"
	}

	parsed, err := url.Parse(urlStr)
	if err != nil {
		// Malformed, default to a safe path.
		return "/"
	}

	// Allow only *relative* URLs without scheme/host.
	if parsed.IsAbs() || parsed.Scheme != "" || parsed.Host != "" {
		return "/"
	}

	redirectPath := parsed.Path
	if redirectPath == "" {
		return "/"
	}

	// Ensure leading slash, then canonicalize and prevent traversal.
	if !strings.HasPrefix(redirectPath, "/") {
		redirectPath = "/" + redirectPath
	}

	cleanPath := pathpkg.Clean(redirectPath)

	// Clean path must still be absolute and must not contain traversal.
	if !strings.HasPrefix(cleanPath, "/") || strings.Contains(cleanPath, "..") {
		return "/"
	}

	// Prevent redirecting to a path starting with a second slash (//evil).
	if len(cleanPath) > 1 && (cleanPath[1] == '/' || cleanPath[1] == '\\') {
		return "/"
	}

	// Allow redirects only to approved internal route prefixes.
	allowedPrefixes := []string{"/", "/admin", "/login"}
	allowed := false
	for _, prefix := range allowedPrefixes {
		if cleanPath == prefix || strings.HasPrefix(cleanPath, prefix+"/") {
			allowed = true
			break
		}
	}
	if !allowed {
		return "/"
	}

	// Re-attach query and fragment only after the path is validated
	result := cleanPath
	if parsed.RawQuery != "" {
		result = result + "?" + parsed.RawQuery
	}
	if parsed.Fragment != "" {
		result = result + "#" + parsed.Fragment
	}

	return result
}

func (h *AuthHandler) renderLoginForm(w http.ResponseWriter, r *http.Request, _ string) {
	// Login is now handled by the SvelteKit SPA at /login.
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
