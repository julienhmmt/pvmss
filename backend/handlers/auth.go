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

// LogoutHandler gère la déconnexion
func (h *AuthHandler) LogoutHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("remote_addr", r.RemoteAddr).
		Logger()

	// Ajouter le nom d'utilisateur s'il est disponible
	if username, ok := r.Context().Value("username").(string); ok {
		log = log.With().Str("username", username).Logger()
	}

	log.Info().Msg("Déconnexion de l'utilisateur")

	stateManager := state.GetGlobalState()
	sessionManager := stateManager.GetSessionManager()

	// Détruire la session
	err := sessionManager.Destroy(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("Erreur lors de la destruction de la session")
	} else {
		log.Info().Msg("Session détruite avec succès")
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// handleLogin traite la soumission du formulaire de connexion
func (h *AuthHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	log := logger.Get().With().Str("remote_addr", r.RemoteAddr).Logger()

	adminHash := os.Getenv("ADMIN_PASSWORD_HASH")
	if adminHash == "" {
		log.Error().Msg("ADMIN_PASSWORD_HASH non configuré dans les variables d'environnement")
		h.renderLoginForm(w, r, "Erreur de configuration du serveur. Veuillez contacter l'administrateur.")
		return
	}

	// Récupération du mot de passe depuis le formulaire
	password := r.FormValue("password")
	log.Debug().Int("password_length", len(password)).Msg("Tentative de connexion")

	// Validation de base
	if len(password) > 200 {
		log.Warn().Int("password_length", len(password)).Msg("Tentative de connexion avec un mot de passe trop long")
		h.renderLoginForm(w, r, "Identifiants invalides.")
		return
	}

	if password == "" {
		log.Debug().Msg("Tentative de connexion avec un mot de passe vide")
		h.renderLoginForm(w, r, "Le mot de passe ne peut pas être vide.")
		return
	}

	// Vérification du mot de passe
	err := bcrypt.CompareHashAndPassword([]byte(adminHash), []byte(password))
	if err != nil {
		log.Info().Err(err).Msg("Échec de la tentative de connexion - mot de passe incorrect")
		h.renderLoginForm(w, r, "Identifiants invalides.")
		return
	}

	log.Debug().Msg("Authentification réussie, création de la session")

	// Authentification réussie
	stateManager := state.GetGlobalState()
	sessionManager := stateManager.GetSessionManager()

	// Création de la session
	sessionManager.Put(r.Context(), "authenticated", true)
	sessionManager.Put(r.Context(), "username", "admin")

	// Récupération de l'URL de redirection
	redirectURL := r.URL.Query().Get("redirect")
	if redirectURL == "" {
		redirectURL = "/admin"
	}

	log.Info().Str("redirect_to", redirectURL).Msg("Connexion réussie, redirection")
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
