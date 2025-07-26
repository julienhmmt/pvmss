package handlers

import (
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/crypto/bcrypt"

	"pvmss/i18n"
	"pvmss/logger"
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
	switch r.Method {
	case http.MethodGet:
		// Afficher le formulaire de connexion
		h.renderLoginForm(w, r, "")
	case http.MethodPost:
		// Traiter la soumission du formulaire
		h.handleLogin(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// LogoutHandler gère la déconnexion
func (h *AuthHandler) LogoutHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	stateManager := state.GetGlobalState()
	sessionManager := stateManager.GetSessionManager()

	// Détruire la session
	sessionManager.Destroy(r.Context())

	logger.Get().Info().Str("path", r.URL.Path).Msg("User logged out")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// handleLogin traite la soumission du formulaire de connexion
func (h *AuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	adminHash := os.Getenv("ADMIN_PASSWORD_HASH")
	password := r.FormValue("password")

	// Validation de base
	if len(password) > 200 {
		logger.Get().Warn().Str("ip", r.RemoteAddr).Msg("Password input too long")
		h.renderLoginForm(w, r, "Identifiants invalides.")
		return
	}

	// Vérification du mot de passe
	err := bcrypt.CompareHashAndPassword([]byte(adminHash), []byte(password))
	if err != nil {
		logger.Get().Info().Str("ip", r.RemoteAddr).Msg("Failed login attempt")
		h.renderLoginForm(w, r, "Identifiants invalides.")
		return
	}

	// Authentification réussie
	stateManager := state.GetGlobalState()
	sessionManager := stateManager.GetSessionManager()
	sessionManager.Put(r.Context(), "authenticated", true)
	sessionManager.Put(r.Context(), "username", "admin")

	logger.Get().Info().Str("ip", r.RemoteAddr).Msg("Successful login")
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// renderLoginForm affiche le formulaire de connexion
func (h *AuthHandler) renderLoginForm(w http.ResponseWriter, r *http.Request, errorMsg string) {
	data := map[string]interface{}{
		"Error": errorMsg,
	}

	// Ajouter les traductions
	i18n.LocalizePage(w, r, data)
	data["Title"] = data["Auth.LoginTitle"]

	renderTemplateInternal(w, r, "login", data)
}

// RegisterRoutes enregistre les routes d'authentification
func (h *AuthHandler) RegisterRoutes(router *httprouter.Router) {
	router.GET("/login", h.LoginHandler)
	router.POST("/login", h.LoginHandler)
	router.GET("/logout", h.LogoutHandler)
}
