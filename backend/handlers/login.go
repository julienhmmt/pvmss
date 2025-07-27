package handlers

import (
	"net/http"
	"os"

	"golang.org/x/crypto/bcrypt"

	"pvmss/logger"
	"pvmss/state"
)

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.Get().With().
		Str("handler", "LoginHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Msg("Traitement de la requête d'authentification")

	// Vérifier si l'utilisateur est déjà authentifié
	if IsAuthenticated(r) {
		log.Info().Msg("Utilisateur déjà authentifié, redirection vers /admin")
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		log.Debug().Msg("Affichage du formulaire de connexion")
		renderTemplateInternal(w, r, "login", nil)
		log.Info().Msg("Formulaire de connexion affiché avec succès")
		return
	}

	if r.Method == http.MethodPost {
		log.Debug().Msg("Traitement de la tentative de connexion")

		password := r.FormValue("password")
		storedHash := os.Getenv("ADMIN_PASSWORD_HASH")

		log.Debug().
			Bool("password_provided", password != "").
			Bool("stored_hash_exists", storedHash != "").
			Msg("Vérification des informations d'authentification")

		if storedHash == "" {
			errMsg := "SECURITY ALERT: ADMIN_PASSWORD_HASH is not set"
			log.Error().Msg(errMsg)
			data := map[string]interface{}{"Error": "Erreur de configuration du serveur."}
			renderTemplateInternal(w, r, "login", data)
			return
		}

		err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
		if err != nil {
			log.Warn().
				Err(err).
				Msg("Échec de la tentative de connexion: identifiants invalides")

			data := map[string]interface{}{"Error": "Identifiants invalides"}
			renderTemplateInternal(w, r, "login", data)
			return
		}

		// Authentification réussie
		stateManager := state.GetGlobalState()
		sessionManager := stateManager.GetSessionManager()
		sessionManager.Put(r.Context(), "authenticated", true)

		log.Info().
			Str("remote_addr", r.RemoteAddr).
			Msg("Utilisateur authentifié avec succès, redirection vers /admin")

		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	// Méthode HTTP non autorisée
	log.Warn().
		Str("method", r.Method).
		Msg("Méthode HTTP non autorisée pour la route de connexion")

	http.Error(w, "Méthode non autorisée", http.StatusMethodNotAllowed)
}
