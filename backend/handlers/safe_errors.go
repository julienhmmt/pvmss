package handlers

import (
	"net/http"

	"pvmss/i18n"
	"pvmss/logger"
)

// RespondWithSafeError returns a localized error response without exposing internal details
func RespondWithSafeError(w http.ResponseWriter, r *http.Request, statusCode int, i18nKey string, internalErr error) {
	// Log the internal error for debugging
	if internalErr != nil {
		logger.Get().Error().Err(internalErr).Str("i18n_key", i18nKey).Str("path", r.URL.Path).Msg("Internal error occurred")
	}
	
	// Return safe, localized error message to client
	localizer := i18n.GetLocalizerFromRequest(r)
	errorMessage := i18n.Localize(localizer, i18nKey)
	http.Error(w, errorMessage, statusCode)
}
