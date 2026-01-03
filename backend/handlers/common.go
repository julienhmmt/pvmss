package handlers

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"pvmss/components"
	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/middleware"
	"pvmss/security"
	"pvmss/state"
	"pvmss/templates"

	"github.com/alexedwards/scs/v2"
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
			Name:     i18n.CookieNameLang,
			Value:    lang,
			Path:     "/",
			MaxAge:   int(i18n.CookieMaxAge / time.Second),
			HttpOnly: false,
			Secure:   getSecureCookieFlag(r),
			SameSite: http.SameSiteLaxMode,
		})
	}

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
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.InternalServer"), http.StatusInternalServerError)
		return
	}

	// Clone the template set for this request to avoid concurrency issues and
	// allow adding request-specific functions.
	instance, err := tmpl.Clone()
	if err != nil {
		log.Error().Err(err).Msg("Failed to clone template set")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.InternalServer"), http.StatusInternalServerError)
		return
	}

	// Add the translation function and request-aware helpers to the template instance for this request.
	localizer := i18n.GetLocalizerFromRequest(r)
	instance.Funcs(template.FuncMap{
		"T": func(messageID string, args ...interface{}) string {
			localized := i18n.Localize(localizer, messageID)
			if localized == "" {
				return messageID
			}
			return localized
		},
	})

	// Merge in request-aware functions (currentPath, urlWithLang, withLang, etc.)
	instance.Funcs(templates.GetFuncMap(r))

	// Define a per-request content slot that renders the requested template.
	contentWrapper := fmt.Sprintf(`{{define "content_slot"}}{{template "%s" .}}{{end}}`, name)
	if _, err := instance.Parse(contentWrapper); err != nil {
		log.Error().Err(err).Msg("Failed to parse content slot template")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.InternalServer"), http.StatusInternalServerError)
		return
	}

	// Execute the layout template with the combined data.
	if err := instance.ExecuteTemplate(w, "layout", data); err != nil {
		log.Error().Err(err).Msg("Error executing layout template")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.InternalServer"), http.StatusInternalServerError)
	}

	log.Info().Msg("Page rendered successfully")
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

	// Prepare home data
	homeData := components.HomeData{
		Username:  username,
		Lang:      i18n.GetLanguage(r),
		CSRFToken: csrfToken,
		IsAdmin:   isAdmin,
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
