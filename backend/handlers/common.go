package handlers

import (
	"bytes"
	"context"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/middleware"
	"pvmss/security"
	"pvmss/state"
	"pvmss/templates"

	"github.com/alexedwards/scs/v2"
	"github.com/julienschmidt/httprouter"
	i18n_bundle "github.com/nicksnyder/go-i18n/v2/i18n"
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

// RenderTemplate renders a template with the provided data
// This function is exported for use by other packages
func RenderTemplate(w http.ResponseWriter, r *http.Request, name string, data interface{}) {
	log := CreateHandlerLogger("RenderTemplate", r).With().
		Str("template", name).
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
	log := CreateHandlerLogger("populateTemplateData", r)

	// Get CSRF token from session and add to template data
	stateManager := getStateManager(r)
	var sessionManager *scs.SessionManager
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
	} else if sessionManager != nil {
		// Fallback: ensure a CSRF token exists in session for templates even if middleware/context didn't set it.
		// This covers cases where a GET page is rendered without the CSRF middleware injecting the token in context.
		sessToken := sessionManager.GetString(r.Context(), "csrf_token")
		if sessToken == "" {
			if newToken, err := security.GenerateCSRFToken(); err == nil {
				sessionManager.Put(r.Context(), "csrf_token", newToken)
				sessToken = newToken
				log.Debug().Msg("Generated new CSRF token and stored in session for template rendering")
			} else {
				log.Error().Err(err).Msg("Failed to generate CSRF token for template rendering")
			}
		}
		if sessToken != "" {
			data["CSRFToken"] = sessToken
			log.Debug().Msg("CSRF token added to template data from session fallback")
		}
	}
	// Add language to data for template rendering
	lang := i18n.GetLanguage(r)
	data["Lang"] = lang

	// Persist selected language in cookie when explicitly provided via query param
	if qLang := strings.TrimSpace(r.URL.Query().Get(i18n.QueryParamLang)); qLang != "" {
		// Normalize to the effective language code and set cookie on this response
		http.SetCookie(w, &http.Cookie{
			Name:   i18n.CookieNameLang,
			Value:  lang,
			Path:   "/",
			MaxAge: int(i18n.CookieMaxAge / time.Second),
		})
	}

	// Add language switcher URLs
	i18n.SetLanguageSwitcher(r, data)
	data["CurrentPath"] = r.URL.Path
	if r.URL.RawQuery != "" {
		data["CurrentURL"] = r.URL.Path + "?" + r.URL.RawQuery
	} else {
		data["CurrentURL"] = r.URL.Path
	}
	data["IsHTTPS"] = r.TLS != nil
	data["Host"] = r.Host
}

// renderTemplateInternal renders a template with a layout, injecting translation functions.
func renderTemplateInternal(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	log := CreateHandlerLogger("renderTemplateInternal", r).With().
		Str("template", name).
		Logger()

	if data == nil {
		data = make(map[string]interface{})
	}
	// Ensure dynamic pages are not cached by browsers or proxies
	setNoCacheHeaders(w)
	populateTemplateData(w, r, data)

	data["IsAdminPage"] = strings.HasPrefix(r.URL.Path, "/admin")
	data["NeedsRegularIcons"] = detectNeedsRegularIcons(name, data)
	data["NeedsBrandIcons"] = detectNeedsBrandIcons(name, data)

	stateManager := getStateManager(r)
	tmpl := stateManager.GetTemplates()
	if tmpl == nil {
		log.Error().Msg("Templates not initialized")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Clone the template set for this request to avoid concurrency issues and
	// allow adding request-specific functions.
	instance, err := tmpl.Clone()
	if err != nil {
		log.Error().Err(err).Msg("Failed to clone template set")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Add the translation function and request-aware helpers to the template instance for this request.
	localizer := i18n.GetLocalizerFromRequest(r)
	instance.Funcs(template.FuncMap{
		"T": func(messageID string, args ...interface{}) template.HTML {
			config := &i18n_bundle.LocalizeConfig{MessageID: messageID}
			if len(args) > 0 {
				if count, ok := args[0].(int); ok {
					config.PluralCount = count
				}
			}
			localized, err := localizer.Localize(config)
			if err != nil || localized == "" {
				return template.HTML(messageID)
			}
			return template.HTML(localized)
		},
	})

	// Merge in request-aware functions (currentPath, urlWithLang, withLang, etc.)
	instance.Funcs(templates.GetFuncMap(r))

	// Render the main content template to a buffer.
	var buf bytes.Buffer
	if err := instance.ExecuteTemplate(&buf, name, data); err != nil {
		log.Error().Err(err).Str("template", name).Msg("Error executing content template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Inject the rendered content into the main data map for the layout.
	data["Content"] = template.HTML(buf.String())

	// Execute the layout template with the combined data.
	if err := instance.ExecuteTemplate(w, "layout", data); err != nil {
		log.Error().Err(err).Msg("Error executing layout template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}

	log.Info().Msg("Page rendered successfully")
}

// IsAuthenticated checks if the user is authenticated
// This function is exported for use by other packages
func IsAuthenticated(r *http.Request) bool {
	log := CreateHandlerLogger("IsAuthenticated", r).With().
		Str("remote_addr", r.RemoteAddr).
		Logger()

	stateManager := getStateManager(r)
	sessionManager := stateManager.GetSessionManager()

	// Check if the session manager is available
	if sessionManager == nil {
		log.Error().Msg("Session manager not available in IsAuthenticated")
		return false
	}

	// Log the session token to help diagnose issues
	sessionToken := sessionManager.Token(r.Context())
	log.Debug().
		Str("session_token", sessionToken).
		Msg("Session token in IsAuthenticated")

	// Check if the session contains the authentication flag
	authenticated, ok := sessionManager.Get(r.Context(), "authenticated").(bool)

	// Log detailed session data for debugging
	sessionData := map[string]interface{}{
		"authenticated_found": ok,
		"authenticated_value": authenticated,
	}

	// Try to get other session values for diagnostic purposes
	if username, ok := sessionManager.Get(r.Context(), "username").(string); ok {
		sessionData["username"] = username
	}

	if isAdmin, ok := sessionManager.Get(r.Context(), "is_admin").(bool); ok {
		sessionData["is_admin"] = isAdmin
	}

	if !ok || !authenticated {
		log.Debug().
			Bool("authenticated", false).
			Interface("session_data", sessionData).
			Str("session_id", sessionToken).
			Msg("Access denied: user not authenticated")
		return false
	}

	log.Debug().
		Bool("authenticated", true).
		Interface("session_data", sessionData).
		Str("session_id", sessionToken).
		Msg("Access granted: user authenticated")

	return true
}

// RequireAuth is a middleware that enforces authentication for protected routes
// This function is exported for use by other packages
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := CreateHandlerLogger("RequireAuth", r).With().
			Str("remote_addr", r.RemoteAddr).
			Logger()

		if !IsAuthenticated(r) {
			log.Info().Msg("Authentication required, redirecting to login")

			// Store the original URL for redirection after login
			returnURL := r.URL.Path
			if r.URL.RawQuery != "" {
				returnURL = returnURL + "?" + r.URL.RawQuery
			}

			// Special-case POST actions that should return to a GET page after login
			if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/vm/update/description") {
				// Try to parse form to extract vmid for a better redirect target
				if err := r.ParseForm(); err == nil {
					if vmid := r.FormValue("vmid"); vmid != "" {
						returnURL = "/vm/details/" + vmid + "?edit=description"
					} else {
						// Fallback to VM list/create page if vmid isn't available
						returnURL = "/vm/create"
					}
				} else {
					// If form can't be parsed, still avoid redirecting back to a POST-only endpoint
					returnURL = "/vm/create"
				}
			}

			setNoCacheHeaders(w)

			// Build login redirect with return URL
			loginURL := "/login?return=" + url.QueryEscape(returnURL)
			// If user was trying to update a VM (e.g., description), add a friendly warning/context
			if strings.HasPrefix(r.URL.Path, "/vm/update/description") {
				loginURL += "&warning=login_required&context=update_description"
			}
			// Redirect to login page with enriched context
			http.Redirect(w, r, loginURL, http.StatusSeeOther)
			return
		}

		// Set security headers for authenticated routes
		setSecurityHeaders(w, r)

		next.ServeHTTP(w, r)
	}
}

// IsAdmin checks if the current user is an admin
func IsAdmin(r *http.Request) bool {
	log := CreateHandlerLogger("IsAdmin", r)

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
		log := CreateHandlerLogger("RequireAdminAuth", r).With().
			Str("remote_addr", r.RemoteAddr).
			Logger()

		if !IsAdmin(r) {
			log.Info().Msg("Admin authentication required")

			// If user is authenticated but not admin, show access denied
			if IsAuthenticated(r) {
				log.Warn().Msg("Authenticated user attempted to access admin area without privileges")
				RenderErrorPage(w, r, http.StatusForbidden, "Access Denied: Admin privileges required")
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
			log := CreateHandlerLogger("RequireAuthHandleWS", r)
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
	ctx := NewHandlerContext(w, r, "IndexHandler")
	ctx.Log.Debug().Msg("Processing request for home page")

	// If it's not the root, return a 404
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Prepare data for the template
	data := map[string]interface{}{
		"Title": "PVMSS",
		"Lang":  i18n.GetLanguage(r),
	}

	ctx.RenderTemplate("index", data)
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

// detectNeedsRegularIcons determines if regular Font Awesome icons are needed
func detectNeedsRegularIcons(_ string, _ map[string]interface{}) bool {
	// Add logic to detect if regular icons are needed based on template or data
	// For now, return false to optimize CSS loading
	return false
}

// detectNeedsBrandIcons determines if brand Font Awesome icons are needed
func detectNeedsBrandIcons(_ string, _ map[string]interface{}) bool {
	// Add logic to detect if brand icons are needed based on template or data
	// For now, return false to optimize CSS loading
	return false
}
