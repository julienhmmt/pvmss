package handlers

import (
	"bytes"
	"context"
	// "encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/middleware"
	"pvmss/security"
	"pvmss/state"
	"pvmss/templates"

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

// contextKey is used for context keys to avoid collisions between packages using context
type contextKey string

// ParamsKey is the key used to store httprouter.Params in the request context
const ParamsKey contextKey = "params"

// StateManagerKey stores the state manager in request context
const StateManagerKey contextKey = "stateManager"

// getStateManager returns the state manager from request context when available.
// No global fallback: state is injected by handlers.InitHandlers.
func getStateManager(r *http.Request) state.StateManager {
	if sm, ok := r.Context().Value(StateManagerKey).(state.StateManager); ok && sm != nil {
		return sm
	}
	logger.Get().Error().Msg("State manager missing from request context")
	return nil
}

// RenderTemplate renders a template with the provided data
// This function is exported for use by other packages
func RenderTemplate(w http.ResponseWriter, r *http.Request, name string, data interface{}) {
	log := logger.Get().With().
		Str("handler", "RenderTemplate").
		Str("template", name).
		Str("path", r.URL.Path).
		Str("method", r.Method).
		Logger()

	log.Debug().Msg("Starting template rendering")

	// Convert data to map if necessary
	dataMap := make(map[string]interface{})
	if data != nil {
		if dm, ok := data.(map[string]interface{}); ok {
			dataMap = dm
			log.Debug().Int("data_map_size", len(dm)).Msg("Data provided as a map")
		} else {
			dataMap["Data"] = data
			log.Debug().Type("data_type", data).Msg("Data provided as a struct, converting to map")
		}
	} else {
		log.Debug().Msg("No data provided for template rendering")
	}

	// Use the internal function with the map
	renderTemplateInternal(w, r, name, dataMap)

	log.Info().
		Str("template", name).
		Msg("Template rendered successfully")
}

// populateTemplateData adds common data to the template data map.
func populateTemplateData(w http.ResponseWriter, r *http.Request, data map[string]interface{}) {
	log := logger.Get().With().Str("function", "populateTemplateData").Logger()

	// Get CSRF token from session and add to template data
	stateManager := getStateManager(r)
	var sessionManager *security.SessionManager
	if stateManager != nil {
		sessionManager = stateManager.GetSessionManager()
	}

	// Get template data from context if it exists (use the same key as the middleware)
	if ctxData, ok := r.Context().Value(middleware.TemplateDataKey).(map[string]interface{}); ok {
		log.Debug().Int("context_data_size", len(ctxData)).Msg("Context data retrieved")
		// Merge context data with provided data (provided data has priority)
		for k, v := range ctxData {
			if _, exists := data[k]; !exists {
				data[k] = v
			}
		}
	}

	// Add authentication data
	if sessionManager != nil && IsAuthenticated(r) {
		log.Debug().Msg("Authenticated user detected, adding session data")
		data["IsAuthenticated"] = true
		data["IsAdmin"] = IsAdmin(r)

		// Add username for regular users (admin users don't have username in session)
		if username, ok := sessionManager.Get(r.Context(), "username").(string); ok && username != "" {
			data["Username"] = username
		} else if IsAdmin(r) {
			data["Username"] = "Admin"
		}
	} else {
		log.Debug().Msg("No authenticated user detected")
		data["IsAuthenticated"] = false
		data["IsAdmin"] = false
	}

	// Add/override CSRF token from request context if available (prefer context value set by middleware)
	if token, ok := security.CSRFTokenFromContext(r.Context()); ok && token != "" {
		data["CSRFToken"] = token
		log.Debug().Msg("CSRF token added to template data from request context")
	}

	// Add i18n data and common variables
	i18n.LocalizePage(w, r, data)
	data["CurrentPath"] = r.URL.Path
	if r.URL.RawQuery != "" {
		data["CurrentURL"] = r.URL.Path + "?" + r.URL.RawQuery
	} else {
		data["CurrentURL"] = r.URL.Path
	}
	data["IsHTTPS"] = r.TLS != nil
	data["Host"] = r.Host

	// Theme from cookie (browser preference). Defaults to light.
	theme := "light"
	if c, err := r.Cookie("theme"); err == nil {
		if c.Value == "dark" || c.Value == "light" {
			theme = c.Value
		}
	}
	data["Theme"] = theme
	data["IsDark"] = (theme == "dark")
}

// renderTemplate is the internal function for rendering templates
func renderTemplateInternal(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	log := logger.Get().With().
		Str("handler", "renderTemplateInternal").
		Str("template", name).
		Str("path", r.URL.Path).
		Logger()

	log.Debug().Msg("Starting internal template rendering")

	// Initialize data map if nil
	if data == nil {
		data = make(map[string]interface{})
	}

	// Populate the template data with common values
	populateTemplateData(w, r, data)

	stateManager := getStateManager(r)
	tmpl := stateManager.GetTemplates()

	if tmpl == nil {
		errMsg := "Templates are not initialized"
		log.Error().Msg(errMsg)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Execute the template (attach request-aware helpers first)
	rt := tmpl.Funcs(templates.GetFuncMap(r))
	buf := new(bytes.Buffer)
	log.Debug().Msg("Executing main template")

	if err := rt.ExecuteTemplate(buf, name, data); err != nil {
		log.Error().
			Err(err).
			Str("template", name).
			Msg("Failed to execute template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	log.Debug().
		Int("content_length", buf.Len()).
		Msg("Main template executed successfully")

	// Add content to layout
	content := buf.String()
	data["Content"] = template.HTML(content)

	log.Debug().
		Int("content_length", len(content)).
		Msg("Template content prepared for layout")

	// Execute the layout
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	log.Debug().Msg("Executing layout template")

	if err := rt.ExecuteTemplate(w, "layout", data); err != nil {
		log.Error().
			Err(err).
			Str("template", "layout").
			Msg("Failed to execute layout template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("template", name).
		Int("response_size", len(content)).
		Msg("Page rendering completed successfully")
}

// IsAuthenticated checks if the user is authenticated
// This function is exported for use by other packages
func IsAuthenticated(r *http.Request) bool {
	log := logger.Get().With().
		Str("handler", "IsAuthenticated").
		Str("path", r.URL.Path).
		Str("method", r.Method).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	stateManager := getStateManager(r)
	sessionManager := stateManager.GetSessionManager()

	// Check if the session contains the authentication flag
	authenticated, ok := sessionManager.Get(r.Context(), "authenticated").(bool)
	if !ok || !authenticated {
		log.Debug().
			Bool("authenticated", false).
			Msg("Access denied: user not authenticated")
		return false
	}

	log.Debug().
		Bool("authenticated", true).
		Str("session_id", sessionManager.Token(r.Context())).
		Msg("Access granted: user authenticated")

	return true
}

// RequireAuth is a middleware that enforces authentication for protected routes
// This function is exported for use by other packages
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Get().With().
			Str("handler", "RequireAuth").
			Str("path", r.URL.Path).
			Str("method", r.Method).
			Str("remote_addr", r.RemoteAddr).
			Logger()

		if !IsAuthenticated(r) {
			log.Info().Msg("Authentication required, redirecting to login")

			// Store the original URL for redirection after login
			returnURL := r.URL.Path
			if r.URL.RawQuery != "" {
				returnURL = returnURL + "?" + r.URL.RawQuery
			}

			setNoCacheHeaders(w)

			// Redirect to login page with return URL
			http.Redirect(w, r, "/login?return="+url.QueryEscape(returnURL), http.StatusSeeOther)
			return
		}

		// Set security headers for authenticated routes
		setSecurityHeaders(w, r)

		next.ServeHTTP(w, r)
	}
}

// IsAdmin checks if the current user is an admin
func IsAdmin(r *http.Request) bool {
	log := logger.Get().With().
		Str("handler", "IsAdmin").
		Str("path", r.URL.Path).
		Logger()

	stateManager := getStateManager(r)
	if stateManager == nil {
		return false
	}
	sessionManager := stateManager.GetSessionManager()
	if sessionManager == nil {
		return false
	}

	isAdmin, ok := sessionManager.Get(r.Context(), "is_admin").(bool)
	if !ok || !isAdmin {
		log.Debug().
			Bool("is_admin", false).
			Msg("User is authenticated but not admin")
		return false
	}

	log.Debug().
		Bool("is_admin", true).
		Str("session_id", sessionManager.Token(r.Context())).
		Msg("Admin access verified")

	return true
}

// setNoCacheHeaders sets headers to prevent client-side caching.
func setNoCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}

// setSecurityHeaders sets security-related headers for authenticated routes.
func setSecurityHeaders(w http.ResponseWriter, r *http.Request) {
	// Prevent content sniffing
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Prevent clickjacking
	w.Header().Set("X-Frame-Options", "DENY")
	// Enable XSS filter
	w.Header().Set("X-XSS-Protection", "1; mode=block")

	// Set no-cache headers for dynamic content
	setNoCacheHeaders(w)

	// Add CSRF token to the response headers for AJAX requests
	if token, ok := security.CSRFTokenFromContext(r.Context()); ok {
		w.Header().Set("X-CSRF-Token", token)
	}
}

// RequireAdminAuth is a middleware that enforces admin authentication for admin routes
// This function is exported for use by other packages
func RequireAdminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Get().With().
			Str("handler", "RequireAdminAuth").
			Str("path", r.URL.Path).
			Str("method", r.Method).
			Str("remote_addr", r.RemoteAddr).
			Logger()

		if !IsAdmin(r) {
			log.Info().Msg("Admin authentication required")

			// If user is authenticated but not admin, show access denied
			if IsAuthenticated(r) {
				log.Warn().Msg("Authenticated user attempted to access admin area without privileges")
				http.Error(w, "Access Denied: Admin privileges required", http.StatusForbidden)
				return
			}

			// Store the original URL for redirection after admin login
			returnURL := r.URL.Path
			if r.URL.RawQuery != "" {
				returnURL = returnURL + "?" + r.URL.RawQuery
			}

			setNoCacheHeaders(w)

			// Redirect to admin login page with return URL
			http.Redirect(w, r, "/admin/login?return="+url.QueryEscape(returnURL), http.StatusSeeOther)
			return
		}

		// Set security headers for admin routes
		setSecurityHeaders(w, r)

		next.ServeHTTP(w, r)
	}
}

// RequireAuthHandleWS wraps a handler with session-based authentication check, suitable for WebSockets.
// It checks for an authenticated session and returns a 401 Unauthorized error if the check fails.
// This avoids redirects which are not suitable for WebSocket upgrade requests.
func RequireAuthHandleWS(h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		stateManager := getStateManager(r)
		if stateManager == nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		sessionManager := stateManager.GetSessionManager()
		if sessionManager == nil || !sessionManager.GetBool(r.Context(), "authenticated") {
			log := logger.Get().With().
				Str("handler", "RequireAuthHandleWS").
				Str("path", r.URL.Path).
				Logger()
			log.Warn().Msg("WebSocket connection rejected: not authenticated")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		h(w, r, ps)
	}
}

// RequireAuthHandle adapts a httprouter.Handle with the RequireAuth middleware.
// It allows protecting router handlers that use the params form.
func RequireAuthHandle(h func(http.ResponseWriter, *http.Request, httprouter.Params)) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Wrap the original handler into an http.HandlerFunc for RequireAuth
		wrapped := func(w http.ResponseWriter, r *http.Request) {
			// Also inject params in context for any downstream helper needing it
			ctx := context.WithValue(r.Context(), ParamsKey, ps)
			h(w, r.WithContext(ctx), ps)
		}
		RequireAuth(wrapped)(w, r)
	}
}

// IndexHandler is a handler for the home page
// This function is exported for use by other packages
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.Get().With().
		Str("handler", "IndexHandler").
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Msg("Processing request for home page")

	// If it's not the root, return a 404
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Prepare data for the template
	data := map[string]interface{}{
		"Title": "PVMSS",
		"Lang":  i18n.GetLanguage(r), // Add detected language
	}

	// Add translation data based on language
	i18n.LocalizePage(w, r, data)

	log.Debug().Msg("Rendering index template")
	renderTemplateInternal(w, r, "index", data) // Use the template name instead of the file name

	log.Info().Msg("Home page displayed successfully")
}

// IndexRouterHandler is a handler for the home page compatible with httprouter
func IndexRouterHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	logger.Get().Debug().
		Str("handler", "IndexRouterHandler").
		Str("path", r.URL.Path).
		Msg("Calling index handler via HTTP router")

	// Delegates processing to the main handler
	IndexHandler(w, r)
}

// HandlerFuncToHTTPrHandle adapts an http.HandlerFunc to an httprouter.Handle function.
// This function allows using standard handlers with the httprouter router.
func HandlerFuncToHTTPrHandle(h http.HandlerFunc) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Create a logger for this request
		log := logger.Get().With().
			Str("adapter", "HandlerFuncToHTTPrHandle").
			Str("path", r.URL.Path).
			Str("method", r.Method).
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
