package security

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"pvmss/logger"
	"pvmss/state"
)

// GenerateCSRFToken generates a new CSRF token using the state manager
func GenerateCSRFToken(r *http.Request) string {
	// Generate random token
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to generate CSRF token")
		return ""
	}

	token := hex.EncodeToString(bytes)

	// Get state manager
	stateManager := state.GetGlobalState()
	if stateManager == nil {
		logger.Get().Error().Msg("State manager not initialized")
		return ""
	}

	log := logger.Get().With().
		Str("function", "GenerateCSRFToken").
		Str("token", token).
		Logger()

	log.Debug().
		Str("token", token).
		Msg("Vérification du jeton CSRF avec le gestionnaire d'état")

	// Store token with expiry
	expiry := time.Now().Add(CSRFTokenTTL)
	if err := stateManager.AddCSRFToken(token, expiry); err != nil {
		log.Error().
			Err(err).
			Str("token", token).
			Msg("Erreur lors de l'ajout du jeton CSRF")
		return ""
	}

	log.Debug().
		Str("token", token).
		Msg("Jeton CSRF ajouté avec succès")

	return token
}

// ValidateCSRFToken validates a CSRF token using the state manager
func ValidateCSRFToken(r *http.Request) bool {
	log := logger.Get().With().
		Str("function", "ValidateCSRFToken").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	// Log des en-têtes pour le débogage
	headers := make(map[string]string)
	for name, values := range r.Header {
		headers[name] = values[0] // On ne prend que la première valeur pour simplifier
	}
	log.Debug().
		Interface("headers", headers).
		Msg("En-têtes de la requête")

	// Récupération du jeton depuis le formulaire
	token := r.FormValue("csrf_token")
	tokenSource := "form"

	// Si pas trouvé dans le formulaire, on cherche dans les en-têtes
	if token == "" {
		token = r.Header.Get("X-CSRF-Token")
		tokenSource = "header"
	}

	log.Debug().
		Str("token_source", tokenSource).
		Str("token", token).
		Msg("Jeton CSRF extrait")

	if token == "" {
		log.Warn().Msg("Aucun jeton CSRF trouvé dans la requête")
		return false
	}

	// Get state manager
	stateManager := state.GetGlobalState()
	if stateManager == nil {
		log.Warn().Msg("State manager not available for CSRF validation")
		return false
	}

	log.Debug().
		Str("token", token).
		Msg("Vérification du jeton CSRF avec le gestionnaire d'état")

	// Validate token
	valid := stateManager.ValidateAndRemoveCSRFToken(token)

	// Ajout de logs supplémentaires pour le débogage
	logContext := log.With().
		Bool("is_valid", valid).
		Str("token", token).
		Logger()

	if valid {
		logContext.Info().Msg("Jeton CSRF validé avec succès")
	} else {
		logContext.Warn().Msg("Échec de la validation du jeton CSRF")
	}

	return valid
}
