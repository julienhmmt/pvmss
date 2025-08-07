package middleware

import (
	"context"
	"net/http"
	"pvmss/logger"
	"pvmss/state"
)

// contextKey is a type for context keys
type contextKey string

// TemplateDataKey is the key used to store template data in the context
const TemplateDataKey contextKey = "templateData"

// ProxmoxStatusMiddleware adds Proxmox connection status to the request context
// and template data
func ProxmoxStatusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Get().With().Str("middleware", "ProxmoxStatus").Logger()
		stateManager := state.GetGlobalState()

		// Get current Proxmox status
		connected, message := stateManager.GetProxmoxStatus()
		log.Debug().Bool("connected", connected).Str("message", message).Msg("Proxmox connection status")

		// Get existing template data from context or create new
		templateData, ok := r.Context().Value(TemplateDataKey).(map[string]interface{})
		if !ok {
			templateData = make(map[string]interface{})
		}

		// Add Proxmox status to template data
		templateData["ProxmoxConnected"] = connected
		if !connected {
			templateData["ProxmoxError"] = message
		}

		// Add template data back to context
		ctx := context.WithValue(r.Context(), TemplateDataKey, templateData)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}
