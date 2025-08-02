package handlers

import (
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/crypto/bcrypt"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/security"
	"pvmss/state"
)

// AuthHandler gère les routes d'authentification
type AuthHandler struct{}

// NewAuthHandler crée une nouvelle instance de AuthHandler
func NewAuthHandler() *AuthHandler {
	return &AuthHandler{}
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
	log := logger.Get().With().
		Str("handler", "AuthHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	// Get username from session before destroying it
	stateManager := state.GetGlobalState()
	sessionManager := stateManager.GetSessionManager()
	username, _ := sessionManager.Get(r.Context(), "username").(string)

	if username != "" {
		log = log.With().Str("username", username).Logger()
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

// handleLogin handles the login form submission
func (h *AuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	log := logger.Get().With().
		Str("handler", "AuthHandler").
		Str("method", r.Method).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	// Validate CSRF token
	if !security.ValidateCSRFToken(r) {
		log.Warn().Msg("CSRF token validation failed")
		h.renderLoginForm(w, r, "Invalid session. Please try again.")
		return
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
		log.Debug().Msg("Login attempt with empty password")
		h.renderLoginForm(w, r, "Password cannot be empty.")
		return
	}

	// Basic input validation
	if len(password) > 200 {
		log.Warn().Int("password_length", len(password)).Msg("Login attempt with too long password")
		h.renderLoginForm(w, r, "Invalid credentials.")
		return
	}

	// Verify password
	err := bcrypt.CompareHashAndPassword([]byte(adminHash), []byte(password))
	if err != nil {
		log.Info().Err(err).Msg("Login failed - incorrect password")
		h.renderLoginForm(w, r, "Invalid credentials.")
		return
	}

	log.Debug().Msg("Authentication successful, creating session")

	// Get session manager
	stateManager := state.GetGlobalState()
	sessionManager := stateManager.GetSessionManager()

	// Create new session with fresh token
	err = sessionManager.RenewToken(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to renew session token")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Store authentication data in session
	sessionManager.Put(r.Context(), "authenticated", true)
	sessionManager.Put(r.Context(), "username", "admin")

	// Generate new CSRF token for the session
	csrfToken, err := security.GenerateCSRFToken()
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate CSRF token")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	sessionManager.Put(r.Context(), "csrf_token", csrfToken)

	log.Info().
		Str("session_id", sessionManager.Token(r.Context())).
		Msg("User logged in successfully")

	// Redirect to admin page or return URL
	redirectURL := r.URL.Query().Get("redirect")
	if redirectURL == "" {
		redirectURL = "/admin"
	}

	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// renderLoginForm affiche le formulaire de connexion
func (h *AuthHandler) renderLoginForm(w http.ResponseWriter, r *http.Request, errorMsg string) {
	log := logger.Get().With().Str("remote_addr", r.RemoteAddr).Logger()

	data := map[string]interface{}{
		"Error": errorMsg,
	}

	// Ajouter les traductions
	log.Debug().Msg("Localisation de la page de connexion")
	i18n.LocalizePage(w, r, data)

	data["Title"] = data["Auth.LoginTitle"]

	log.Debug().Msg("Rendu du template de connexion")
	renderTemplateInternal(w, r, "login", data)
}

// RegisterRoutes enregistre les routes d'authentification
func (h *AuthHandler) RegisterRoutes(router *httprouter.Router) {
	router.GET("/login", h.LoginHandler)
	router.POST("/login", h.LoginHandler)
	router.GET("/logout", h.LogoutHandler)
}
