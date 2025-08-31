package handlers

import (
	"context"
	"crypto/subtle"
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

// AuthHandler gère les routes d'authentification
type AuthHandler struct {
	stateManager state.StateManager
}

// LogoutGet serves a minimal page that auto-submits a POST request to /logout including CSRF token.
// This preserves CSRF protection while allowing logout links to be simple GETs.
func (h *AuthHandler) LogoutGet(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().Str("handler", "AuthHandler.LogoutGet").Str("path", r.URL.Path).Logger()

	// Attempt to get CSRF token from context (populated by CSRFGeneratorMiddleware for GET)
	var csrfToken string
	if token, ok := r.Context().Value(security.CSRFTokenContextKey).(string); ok && token != "" {
		csrfToken = token
	} else {
		// Fallback to session
		if sm := security.GetSession(r); sm != nil {
			if t, ok := sm.Get(r.Context(), "csrf_token").(string); ok && t != "" {
				csrfToken = t
			}
		}
	}

	if csrfToken == "" {
		log.Warn().Msg("No CSRF token available for logout form; generating new one")
		if sm := security.GetSession(r); sm != nil {
			if t, err := security.GenerateCSRFToken(); err == nil {
				csrfToken = t
				sm.Put(r.Context(), "csrf_token", csrfToken)
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

// NewAuthHandler crée une nouvelle instance de AuthHandler
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

// AdminLoginHandler handles admin login page and form submission
func (h *AuthHandler) AdminLoginHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().Str("method", r.Method).Str("path", r.URL.Path).Str("remote_addr", r.RemoteAddr).Logger()

	switch r.Method {
	case http.MethodGet:
		log.Debug().Msg("Displaying admin login form")
		h.renderAdminLoginForm(w, r, "")
	case http.MethodPost:
		log.Debug().Msg("Processing admin login form submission")
		h.handleAdminLogin(w, r)
	default:
		log.Warn().Str("method", r.Method).Msg("Method not allowed")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// RegisterRoutes enregistre les routes d'authentification
func (h *AuthHandler) RegisterRoutes(router *httprouter.Router) {
	// User login routes (with redirect middleware)
	router.GET("/login", h.RedirectIfAuthenticated(h.LoginHandler))
	router.POST("/login", h.LoginHandler)

	// Admin login routes
	router.GET("/admin/login", h.AdminLoginHandler)
	router.POST("/admin/login", h.AdminLoginHandler)

	// Logout routes
	router.GET("/logout", h.LogoutGet)
	router.POST("/logout", h.LogoutHandler)
}

// LoginHandler gère la page de connexion
func (h *AuthHandler) LoginHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().Str("method", r.Method).Str("path", r.URL.Path).Str("remote_addr", r.RemoteAddr).Logger()

	switch r.Method {
	case http.MethodGet:
		log.Debug().Msg("Affichage du formulaire de connexion")
		// Afficher le formulaire de connexion
		h.renderLoginForm(w, r, "")
	case http.MethodPost:
		log.Debug().Msg("Traitement de la soumission du formulaire de connexion")
		// Traiter la soumission du formulaire
		h.handleLogin(w, r)
	default:
		log.Warn().Str("method", r.Method).Msg("Méthode HTTP non autorisée")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
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

// renderAdminLoginForm renders the admin login form with CSRF token and error message
func (h *AuthHandler) renderAdminLoginForm(w http.ResponseWriter, r *http.Request, errorMsg string) {
	log := logger.Get().With().
		Str("handler", "AuthHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Msg("Rendering admin login form")

	// Get session manager
	sessionManager := security.GetSession(r)
	if sessionManager == nil {
		log.Error().Msg("Session manager not available")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get CSRF token from context or session
	var csrfToken string
	if token, ok := r.Context().Value(security.CSRFTokenContextKey).(string); ok && token != "" {
		csrfToken = token
		log.Debug().Msg("Using CSRF token from request context")
	} else {
		if token, ok := sessionManager.Get(r.Context(), "csrf_token").(string); ok && token != "" {
			csrfToken = token
			log.Debug().Msg("Using CSRF token from session")
		} else {
			var err error
			csrfToken, err = security.GenerateCSRFToken()
			if err != nil {
				log.Error().Err(err).Msg("Failed to generate CSRF token")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			sessionManager.Put(r.Context(), "csrf_token", csrfToken)
			log.Debug().Msg("Generated new CSRF token")
		}
	}

	// Prepare template data
	data := map[string]interface{}{
		"Title":       "Admin Login",
		"Error":       errorMsg,
		"CSRFToken":   csrfToken,
		"RedirectURL": r.URL.Query().Get("redirect"),
		"ReturnURL":   r.URL.Query().Get("return"),
	}

	// Add translations
	i18n.LocalizePage(w, r, data)

	log.Debug().Msg("Rendering admin_login template")
	renderTemplateInternal(w, r, "admin_login", data)
}

// handleAdminLogin handles admin login form submission (password-only)
func (h *AuthHandler) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	log := logger.Get().With().
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

	// Validate CSRF token
	if r.Method == http.MethodPost {
		csrfToken := r.FormValue("csrf_token")
		if csrfToken == "" {
			log.Warn().Msg("CSRF token is missing from form")
			h.renderAdminLoginForm(w, r, "Invalid request. Please try again.")
			return
		}

		sessionToken, ok := sessionManager.Get(r.Context(), "csrf_token").(string)
		if !ok || sessionToken == "" {
			log.Warn().Msg("No CSRF token found in session")
			h.renderAdminLoginForm(w, r, "Session expired. Please try again.")
			return
		}

		if subtle.ConstantTimeCompare([]byte(csrfToken), []byte(sessionToken)) != 1 {
			log.Warn().Msg("CSRF token validation failed")
			h.renderAdminLoginForm(w, r, "Invalid request. Please try again.")
			return
		}
	}

	// Get admin password hash from environment
	adminHash := os.Getenv("ADMIN_PASSWORD_HASH")
	if adminHash == "" {
		log.Error().Msg("ADMIN_PASSWORD_HASH is not set in environment variables")
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	// Get password from form
	password := r.FormValue("password")
	if password == "" {
		log.Debug().Msg("Admin login attempt with empty password")
		h.renderAdminLoginForm(w, r, "Password cannot be empty.")
		return
	}

	// Basic input validation
	if len(password) > 200 {
		log.Warn().Int("password_length", len(password)).Msg("Admin login attempt with too long password")
		h.renderAdminLoginForm(w, r, "Invalid credentials.")
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(adminHash), []byte(password)); err != nil {
		log.Info().Err(err).Msg("Admin login failed - incorrect password")
		h.renderAdminLoginForm(w, r, "Invalid credentials.")
		return
	}

	log.Debug().Msg("Admin authentication successful, creating session")

	// Create new session with fresh token
	if err := sessionManager.RenewToken(r.Context()); err != nil {
		log.Error().Err(err).Msg("Failed to renew session token")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Store authentication data in session with admin flag
	sessionManager.Put(r.Context(), "authenticated", true)
	sessionManager.Put(r.Context(), "is_admin", true)

	// Generate new CSRF token for the session
	var newCSRFToken string
	newCSRFToken, err := security.GenerateCSRFToken()
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate CSRF token")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	sessionManager.Put(r.Context(), "csrf_token", newCSRFToken)

	log.Info().Str("session_id", sessionManager.Token(r.Context())).Msg("Admin logged in successfully")

	// Redirect to admin page or return URL
	redirectURL := r.FormValue("return")
	if redirectURL == "" {
		redirectURL = r.URL.Query().Get("return")
	}
	if redirectURL == "" {
		redirectURL = r.FormValue("redirect")
	}
	if redirectURL == "" {
		redirectURL = r.URL.Query().Get("redirect")
	}
	if redirectURL == "" {
		redirectURL = "/admin/nodes"
	}

	// Ensure the URL has a scheme
	if len(redirectURL) > 0 && redirectURL[0] != '/' {
		redirectURL = "/" + redirectURL
	}

	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// handleLogin handles the user login form submission (username + password via Proxmox)
func (h *AuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
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

	// For POST requests, validate CSRF token
	if r.Method == http.MethodPost {
		// Get CSRF token from form
		csrfToken := r.FormValue("csrf_token")
		if csrfToken == "" {
			log.Warn().Msg("CSRF token is missing from form")
			h.renderLoginForm(w, r, "Invalid request. Please try again.")
			return
		}

		// Get CSRF token from session
		sessionToken, ok := sessionManager.Get(r.Context(), "csrf_token").(string)
		if !ok || sessionToken == "" {
			log.Warn().Msg("No CSRF token found in session")
			h.renderLoginForm(w, r, "Session expired. Please try again.")
			return
		}

		// Validate CSRF token
		if subtle.ConstantTimeCompare([]byte(csrfToken), []byte(sessionToken)) != 1 {
			log.Warn().Msg("CSRF token validation failed")
			h.renderLoginForm(w, r, "Invalid request. Please try again.")
			return
		}
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

	// Get Proxmox client for user authentication
	proxmoxClient := h.stateManager.GetProxmoxClient()
	if proxmoxClient == nil {
		log.Error().Msg("Proxmox client not available for user authentication")
		h.renderLoginForm(w, r, "Authentication service unavailable. Please try again later.")
		return
	}

	// Cast to concrete client to access Login method
	pxClient, ok := proxmoxClient.(*proxmox.Client)
	if !ok {
		log.Error().Msg("Invalid Proxmox client type for user authentication")
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
	// Create a cookie-auth client for the same user
	proxmoxURL := strings.TrimSpace(os.Getenv("PROXMOX_URL"))
	insecureSkip := strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL")) == "false"

	if proxmoxURL != "" {
		cookieClient, err := proxmox.NewClientCookieAuth(proxmoxURL, insecureSkip)
		if err == nil {
			if err := cookieClient.Login(ctx, username, password, "pve"); err == nil {
				// Store the cookie auth credentials in session for console access
				sessionManager.Put(r.Context(), "pve_auth_cookie", cookieClient.PVEAuthCookie)
				sessionManager.Put(r.Context(), "csrf_prevention_token", cookieClient.CSRFPreventionToken)
				log.Debug().Str("username", username).Msg("Cookie authentication established for console access")
			} else {
				log.Warn().Err(err).Str("username", username).Msg("Failed to establish cookie auth for console - console access may be limited")
			}
		}
	}

	log.Debug().Str("username", username).Msg("User authentication successful via Proxmox, creating session")

	// Create new session with fresh token
	if err := sessionManager.RenewToken(r.Context()); err != nil {
		log.Error().Err(err).Msg("Failed to renew session token")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Store authentication data in session (user, not admin)
	sessionManager.Put(r.Context(), "authenticated", true)
	sessionManager.Put(r.Context(), "is_admin", false)
	sessionManager.Put(r.Context(), "username", username)

	// Generate new CSRF token for the session
	newCSRFToken, err := security.GenerateCSRFToken()
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate CSRF token")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	sessionManager.Put(r.Context(), "csrf_token", newCSRFToken)

	log.Info().
		Str("session_id", sessionManager.Token(r.Context())).
		Str("username", username).
		Msg("User logged in successfully via Proxmox")

	// Redirect to VM creation page or return URL for regular users
	redirectURL := r.FormValue("return")
	if redirectURL == "" {
		redirectURL = r.URL.Query().Get("return")
	}
	if redirectURL == "" {
		redirectURL = r.FormValue("redirect")
	}
	if redirectURL == "" {
		redirectURL = r.URL.Query().Get("redirect")
	}
	if redirectURL == "" {
		redirectURL = "/vm/create" // Default redirect for users
	}

	// Ensure the URL has a scheme
	if len(redirectURL) > 0 && redirectURL[0] != '/' {
		redirectURL = "/" + redirectURL
	}

	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (h *AuthHandler) renderLoginForm(w http.ResponseWriter, r *http.Request, errorMsg string) {
	log := logger.Get().
		With().
		Str("handler", "AuthHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Msg("Rendering login form")

	// Get session manager
	sessionManager := security.GetSession(r)
	if sessionManager == nil {
		log.Error().Msg("Session manager not available")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get CSRF token from context (should be set by CSRFGeneratorMiddleware)
	var csrfToken string
	if token, ok := r.Context().Value(security.CSRFTokenContextKey).(string); ok && token != "" {
		csrfToken = token
		log.Debug().Msg("Using CSRF token from request context")
	} else {
		// Fallback to session if not in context
		if token, ok := sessionManager.Get(r.Context(), "csrf_token").(string); ok && token != "" {
			csrfToken = token
			log.Debug().Msg("Using CSRF token from session")
		} else {
			// Generate a new CSRF token as last resort
			var err error
			csrfToken, err = security.GenerateCSRFToken()
			if err != nil {
				log.Error().Err(err).Msg("Failed to generate CSRF token")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			sessionManager.Put(r.Context(), "csrf_token", csrfToken)
			log.Debug().Msg("Generated new CSRF token")
		}
	}

	// Prepare template data with CSRF token
	data := map[string]interface{}{
		"Title":       "Login",
		"Error":       errorMsg,
		"CSRFToken":   csrfToken,
		"RedirectURL": r.URL.Query().Get("redirect"),
		"ReturnURL":   r.URL.Query().Get("return"),
	}

	// Add translations
	i18n.LocalizePage(w, r, data)

	log.Debug().Msg("Rendering login template")
	renderTemplateInternal(w, r, "login", data)
}
