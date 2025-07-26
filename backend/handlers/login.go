package handlers

import (
	"net/http"
	"os"

	"golang.org/x/crypto/bcrypt"

	"pvmss/logger"
	"pvmss/state"
)

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if IsAuthenticated(r) {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodGet {
		renderTemplateInternal(w, r, "login", nil)
		return
	}

	if r.Method == http.MethodPost {
		password := r.FormValue("password")
		storedHash := os.Getenv("ADMIN_PASSWORD_HASH")

		if storedHash == "" {
			logger.Get().Error().Msg("SECURITY ALERT: ADMIN_PASSWORD_HASH is not set.")
			data := map[string]interface{}{"Error": "Server configuration error."}
			renderTemplateInternal(w, r, "login", data)
			return
		}

		err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
		if err != nil {
			logger.Get().Warn().Msg("Failed login attempt")
			data := map[string]interface{}{"Error": "Invalid credentials"}
			renderTemplateInternal(w, r, "login", data)
			return
		}

		stateManager := state.GetGlobalState()
		sessionManager := stateManager.GetSessionManager()
		sessionManager.Put(r.Context(), "authenticated", true)

		logger.Get().Info().Msg("Admin user logged in successfully")
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}
