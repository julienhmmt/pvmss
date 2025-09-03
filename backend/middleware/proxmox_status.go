package middleware

import (
	"context"
	"net/http"
	"pvmss/state"
)

// contextKey is a type for context keys
type contextKey string

// TemplateDataKey is the key used to store template data in the context
const TemplateDataKey contextKey = "templateData"

// ProxmoxStatusMiddlewareWithState creates a middleware that injects the Proxmox connection status
// into the request context. This allows UI components to display whether the backend is successfully
// connected to the Proxmox server.
//
// It requires a state.StateManager instance to be provided, from which it fetches the connection status.
// The status information is added to a map which is then placed in the request context
// under the key specified by TemplateDataKey.
func ProxmoxStatusMiddlewareWithState(sm state.StateManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := NewMiddlewareLogger("ProxmoxStatus")

			// This middleware relies on a state manager to function. If it's not provided,
			// log a warning and pass the request through without modification.
			if sm == nil {
				log.Warn().Msg("State manager is nil, skipping Proxmox status injection.")
				next.ServeHTTP(w, r)
				return
			}

			// Fetch the current connection status from the state manager.
			connected, message := sm.GetProxmoxStatus()
			log.Debug().Bool("connected", connected).Str("message", message).Msg("Proxmox connection status")

			// Retrieve existing template data from the context, or initialize a new map if none exists.
			var templateData map[string]interface{}
			if data, ok := r.Context().Value(TemplateDataKey).(map[string]interface{}); ok {
				templateData = data
			} else {
				templateData = make(map[string]interface{})
			}

			// Add the Proxmox status to the template data.
			templateData["ProxmoxConnected"] = connected
			if !connected {
				templateData["ProxmoxError"] = message
			}

			// Place the updated template data back into the request context.
			ctx := context.WithValue(r.Context(), TemplateDataKey, templateData)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
