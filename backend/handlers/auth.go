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

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/security"
	"pvmss/state"
)

// setLanguageCookieAndRedirect sets language cookie without modifying the URL
func setLanguageCookieAndRedirect(w http.ResponseWriter, r *http.Request, baseURL string) string {
	lang := i18n.GetLanguage(r)
	if lang == "" {
		return baseURL
	}

	// Set language cookie for persistence
	http.SetCookie(w, &http.Cookie{
		Name:     i18n.CookieNameLang,
		Value:    lang,
		Path:     "/",
		MaxAge:   int(i18n.CookieMaxAge / time.Second),
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})

	return baseURL
}

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

func (h *AuthHandler) renderAdminLoginForm(w http.ResponseWriter, r *http.Request, errorMsg string) {
	ctx := NewHandlerContext(w, r, "AuthHandler.renderAdminLoginForm")
	ctx.Log.Debug().
		Str("component", "auth").
		Str("operation", "render_admin_login_form").
		Str("reason", "form_render").
		Msg("Rendering admin login form")

	if !ctx.ValidateSessionManager() {
		return
	}

	csrfToken, err := ctx.GetCSRFToken()
	if err != nil {
		ctx.HandleError(err, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"TitleKey":    "AdminLogin.Title",
		"Error":       errorMsg,
		"CSRFToken":   csrfToken,
		"RedirectURL": r.URL.Query().Get("redirect"),
		"ReturnURL":   r.URL.Query().Get("return"),
		"Lang":        i18n.GetLanguage(r),
	}

	ctx.RenderTemplate("admin_login", data)
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
	ctx := NewHandlerContext(w, r, "AuthHandler.handleAdminLogin")

	if !ctx.ValidateSessionManager() {
		return
	}

	if err := validateCSRF(r); err != nil {
		loc := i18n.GetLocalizerFromRequest(r)
		errMsg := i18n.Localize(loc, "Login.Error.InvalidRequest")
		if err.Error() == "session expired" {
			errMsg = i18n.Localize(loc, "Login.Error.SessionExpired")
		}
		h.renderAdminLoginForm(w, r, errMsg)
		return
	}

	// Get admin password hash from environment
	adminHash := os.Getenv("ADMIN_PASSWORD_HASH")
	if adminHash == "" {
		ctx.Log.Error().Msg("ADMIN_PASSWORD_HASH is not set in environment variables")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.ServerConfigError"), http.StatusInternalServerError)
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
		h.renderAdminLoginForm(w, r, i18n.Localize(i18n.GetLocalizerFromRequest(r), "AdminLogin.Error.EmptyPassword"))
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
		h.renderAdminLoginForm(w, r, i18n.Localize(i18n.GetLocalizerFromRequest(r), "AdminLogin.Error.InvalidCredentials"))
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(adminHash), []byte(password)); err != nil {
		logger.AuthFailure("admin_login", "invalid_password").
			Str("auth_method", "password_hash").
			Str("client_ip", r.RemoteAddr).
			Msg("Admin login failed - incorrect password")
		h.renderAdminLoginForm(w, r, i18n.Localize(i18n.GetLocalizerFromRequest(r), "AdminLogin.Error.InvalidCredentials"))
		return
	}

	ctx.Log.Debug().
		Str("component", "auth").
		Str("operation", "admin_login").
		Str("reason", "auth_success").
		Msg("Admin authentication successful, creating session")

	if err := establishSession(w, r, true, ""); err != nil {
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.InternalServer"), http.StatusInternalServerError)
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
	redirectURL := getRedirectURL(r, "/admin/nodes")
	redirectURL = setLanguageCookieAndRedirect(w, r, redirectURL)
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
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Login.Error.ServiceUnavailable"), http.StatusInternalServerError)
		return
	}

	if err := validateCSRF(r); err != nil {
		loc := i18n.GetLocalizerFromRequest(r)
		errMsg := i18n.Localize(loc, "Login.Error.InvalidRequest")
		if err.Error() == "session expired" {
			errMsg = i18n.Localize(loc, "Login.Error.SessionExpired")
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
		h.renderLoginForm(w, r, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Login.Error.MissingCredentials"))
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
		h.renderLoginForm(w, r, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Login.Error.InvalidCredentials"))
		return
	}

	// Create a new Proxmox client for user authentication
	proxmoxURL := strings.TrimSpace(os.Getenv("PROXMOX_URL"))
	insecureSkip := strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL")) == "false"

	if proxmoxURL == "" {
		log.Error().Msg("PROXMOX_URL is not configured")
		h.renderLoginForm(w, r, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Login.Error.ServiceUnavailable"))
		return
	}

	// We create a new cookie-based client for user/pass login
	pxClient, err := proxmox.NewClientCookieAuth(proxmoxURL, insecureSkip)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Proxmox client for user authentication")
		h.renderLoginForm(w, r, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Login.Error.ServiceUnavailable"))
		return
	}

	// Attempt to authenticate user via Proxmox with "@pve" realm
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Use CreateTicket to get the authentication ticket with full response
	ticketResp, err := proxmox.CreateTicket(ctx, pxClient, username, password, &proxmox.CreateTicketOptions{
		Realm: "pve",
	})
	if err != nil {
		logger.AuthFailure("user_login", "proxmox_auth_failed").
			Err(err).
			Str("username", username).
			Str("realm", "pve").
			Str("client_ip", r.RemoteAddr).
			Msg("User login failed - Proxmox authentication failed")
		h.renderLoginForm(w, r, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Login.Error.InvalidCredentials"))
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
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.InternalServer"), http.StatusInternalServerError)
		return
	}

	// Persist language selection in cookie and append to redirect
	var defaultURL string
	if isAdmin {
		defaultURL = "/admin/nodes"
	} else {
		defaultURL = "/vm/create"
	}
	redirectURL := getRedirectURL(r, defaultURL)
	redirectURL = setLanguageCookieAndRedirect(w, r, redirectURL)
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
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Login.Error.ServiceUnavailable"), http.StatusInternalServerError)
		return
	}

	if err := validateCSRF(r); err != nil {
		loc := i18n.GetLocalizerFromRequest(r)
		errMsg := i18n.Localize(loc, "Login.Error.InvalidRequest")
		if err.Error() == "session expired" {
			errMsg = i18n.Localize(loc, "Login.Error.SessionExpired")
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
		h.renderAdminLoginForm(w, r, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Login.Error.MissingCredentials"))
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
		h.renderAdminLoginForm(w, r, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Login.Error.InvalidCredentials"))
		return
	}

	// Create a new Proxmox client for admin authentication
	proxmoxURL := strings.TrimSpace(os.Getenv("PROXMOX_URL"))
	insecureSkip := strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL")) == "false"

	if proxmoxURL == "" {
		log.Error().Msg("PROXMOX_URL is not configured")
		h.renderAdminLoginForm(w, r, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Login.Error.ServiceUnavailable"))
		return
	}

	// We create a new cookie-based client for admin/pass login
	pxClient, err := proxmox.NewClientCookieAuth(proxmoxURL, insecureSkip)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Proxmox client for admin authentication")
		h.renderAdminLoginForm(w, r, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Login.Error.ServiceUnavailable"))
		return
	}

	// Attempt to authenticate admin via Proxmox with "@pve" realm
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Use CreateTicket to get the authentication ticket with full response
	ticketResp, err := proxmox.CreateTicket(ctx, pxClient, username, password, &proxmox.CreateTicketOptions{
		Realm: "pve",
	})
	if err != nil {
		log.Info().Err(err).
			Str("ip", r.RemoteAddr).
			Str("username", username).
			Msg("Admin login failed - Proxmox authentication failed")
		h.renderAdminLoginForm(w, r, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Login.Error.InvalidCredentials"))
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
		h.renderAdminLoginForm(w, r, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Login.Error.InvalidCredentials"))
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
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.InternalServer"), http.StatusInternalServerError)
		return
	}

	// Persist language selection in cookie and append to redirect
	redirectURL := getRedirectURL(r, "/admin/nodes")
	redirectURL = setLanguageCookieAndRedirect(w, r, redirectURL)
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
	sessionManager := security.GetSession(r)
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

// ensureLocalPath ensures the URL is a local path starting with /
func ensureLocalPath(url string) string {
	if url == "" {
		return "/"
	}
	if url[0] != '/' {
		return "/" + url
	}
	return url
}

func (h *AuthHandler) renderLoginForm(w http.ResponseWriter, r *http.Request, errorMsg string) {
	ctx := NewHandlerContext(w, r, "AuthHandler.renderLoginForm")
	ctx.Log.Debug().
		Str("component", "auth").
		Str("operation", "render_login_form").
		Str("reason", "form_render").
		Msg("Rendering login form")

	if !ctx.ValidateSessionManager() {
		return
	}

	csrfToken, err := ctx.GetCSRFToken()
	if err != nil {
		ctx.HandleError(err, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Optional friendly warning (e.g., redirected from VM details to login)
	warning := ""
	if r.URL.Query().Get("warning") == "login_required" {
		warningKey := "Login.Warning.General"
		if r.URL.Query().Get("context") == "update_description" {
			warningKey = "Login.Warning.UpdateDescription"
		}
		warning = ctx.Translate(warningKey)
	}

	// Prepare template data with CSRF token
	data := map[string]interface{}{
		"TitleKey":    "Login.Title",
		"Error":       errorMsg,
		"Warning":     warning,
		"CSRFToken":   csrfToken,
		"RedirectURL": r.URL.Query().Get("redirect"),
		"ReturnURL":   r.URL.Query().Get("return"),
		"Lang":        i18n.GetLanguage(r),
	}

	ctx.RenderTemplate("login", data)
}
