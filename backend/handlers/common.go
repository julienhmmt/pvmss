package handlers

import (
	"context"
	"net/http"

	"pvmss/components"
	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/security"
	"pvmss/state"

	"github.com/julienschmidt/httprouter"
)

// ISOInfo represents detailed information about an ISO image.
type ISOInfo struct {
	VolID   string `json:"volid"`
	Format  string `json:"format"`
	Size    int64  `json:"size"`
	Node    string `json:"node,omitempty"`
	Storage string `json:"storage,omitempty"`
	Enabled bool   `json:"enabled"`
}

// handlerContextKey is used for context keys specific to handlers package
type handlerContextKey string

// ParamsKey is the key used to store httprouter.Params in the request context
const ParamsKey handlerContextKey = "params"

// StateManagerKey stores the state manager in request context
const StateManagerKey handlerContextKey = "stateManager"

// getStateManager returns the state manager from request context when available.
// No global fallback: state is injected by handlers.InitHandlers.
func getStateManager(r *http.Request) state.StateManager {
	if sm, ok := r.Context().Value(StateManagerKey).(state.StateManager); ok && sm != nil {
		return sm
	}
	logger.Get().Error().Msg("State manager missing from request context")
	return nil
}

// setNoCacheHeaders sets headers to prevent client-side caching.
func setNoCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}

// setSecurityHeaders sets security-related headers for authenticated routes.
func setSecurityHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	setNoCacheHeaders(w)
	if token, ok := security.CSRFTokenFromContext(r.Context()); ok {
		w.Header().Set("X-CSRF-Token", token)
	}
}

// IndexHandler is a handler for the home page
// This function is exported for use by other packages
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	ctx := NewHandlerContext(w, r, "IndexHandler")
	ctx.Log.Debug().Msg("Processing request for home page")

	// If it's not the root, return a 404
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Get username and admin status from session
	username := ""
	isAdmin := false
	if sessionManager := security.GetSession(r); sessionManager != nil {
		if user, ok := sessionManager.Get(r.Context(), "username").(string); ok {
			username = user
		}
		if admin, ok := sessionManager.Get(r.Context(), "is_admin").(bool); ok {
			isAdmin = admin
		}
	}

	// Get CSRF token
	csrfToken, _ := ctx.GetCSRFToken()

	// Check authentication status
	isAuthenticated := ctx.IsAuthenticated()
	proxmoxConnected := IsProxmoxTicketValid(r)

	// Prepare home data
	homeData := components.HomeData{
		Username:         username,
		Lang:             i18n.GetLanguage(r),
		CSRFToken:        csrfToken,
		IsAdmin:          isAdmin,
		IsAuthenticated:  isAuthenticated,
		ProxmoxConnected: proxmoxConnected,
	}

	// Translation function wrapper
	translateFunc := func(key string) string {
		return ctx.Translate(key)
	}

	// Render with Templ
	if err := components.HomePage(homeData, translateFunc).Render(r.Context(), w); err != nil {
		ctx.Log.Error().Err(err).Msg("Failed to render home page")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	ctx.Log.Info().Msg("Home page displayed successfully")
}

// IndexRouterHandler is a handler for the home page compatible with httprouter
func IndexRouterHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Delegates processing to the main handler
	IndexHandler(w, r)
}

// HandlerFuncToHTTPrHandle adapts an http.HandlerFunc to an httprouter.Handle function.
// This function allows using standard handlers with the httprouter router.
func HandlerFuncToHTTPrHandle(h http.HandlerFunc) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Create a logger for this request
		log := CreateHandlerLogger("HandlerFuncToHTTPrHandle", r).With().
			Int("params_count", len(ps)).
			Logger()

		log.Debug().Msg("Adapting standard HTTP handler for httprouter")

		// Add route parameters to the request context
		ctx := context.WithValue(r.Context(), ParamsKey, ps)

		// Call the original handler with the new context
		h(w, r.WithContext(ctx))

		log.Debug().Msg("HTTP handler processing finished")
	}
}

// AdminAuditMiddleware logs all admin actions for audit trail
func AdminAuditMiddleware(next httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Only audit if user is admin
		if !IsAdmin(r) {
			next(w, r, ps)
			return
		}

		// Get user context
		username := "unknown"
		proxmoxUsername := "unknown"
		isAdmin := false

		if sessionManager := security.GetSession(r); sessionManager != nil {
			if user, ok := sessionManager.Get(r.Context(), "username").(string); ok && user != "" {
				username = user
			}
			if admin, ok := sessionManager.Get(r.Context(), "is_admin").(bool); ok {
				isAdmin = admin
			}
			if pxUser, ok := sessionManager.Get(r.Context(), "pve_username").(string); ok && pxUser != "" {
				proxmoxUsername = pxUser
			}
		}

		// Determine auth method
		authMethod := "builtin"
		if proxmoxUsername != "unknown" {
			authMethod = "pve"
		}

		// Structured audit log for admin access (DEBUG level to avoid noise)
		logger.Get().Debug().
			Str("event_category", "admin").
			Str("event_type", "admin_access").
			Str("admin_username", username).
			Str("auth_method", authMethod).
			Bool("is_admin", isAdmin).
			Str("client_ip", r.RemoteAddr).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("Admin endpoint accessed")

		// Continue with the request
		next(w, r, ps)
	}
}
