package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/security"
	"pvmss/state"

	"github.com/alexedwards/scs/v2"
)

// HandlerContext provides common context for handlers
type HandlerContext struct {
	Log             logger.Logger
	StateManager    state.StateManager
	SessionManager  *scs.SessionManager
	Request         *http.Request
	ResponseWriter  http.ResponseWriter
	isAuthenticated bool // cached on creation
	isAdmin         bool // cached on creation
	authCached      bool // flag to indicate if auth state has been cached
}

// HandlerContextWith creates a handler context with common setup
func HandlerContextWith(w http.ResponseWriter, r *http.Request, handlerName string) *HandlerContext {
	log := CreateHandlerLogger(handlerName, r)

	stateManager := getStateManager(r)
	var sessionManager *scs.SessionManager
	if stateManager != nil {
		sessionManager = stateManager.GetSessionManager()
	}

	ctx := &HandlerContext{
		Log:            log,
		StateManager:   stateManager,
		SessionManager: sessionManager,
		Request:        r,
		ResponseWriter: w,
	}

	// Cache authentication state once on creation
	if ctx.SessionManager != nil {
		authenticated, ok := ctx.SessionManager.Get(ctx.Request.Context(), "authenticated").(bool)
		ctx.isAuthenticated = ok && authenticated

		isAdmin, ok := ctx.SessionManager.Get(ctx.Request.Context(), "is_admin").(bool)
		ctx.isAdmin = ok && isAdmin

		ctx.authCached = true
	}

	return ctx
}

// Translate looks up a translation key using the request's locale, falling back to the key.
func (ctx *HandlerContext) Translate(key string) string {
	localizer := i18n.GetLocalizerFromRequest(ctx.Request)
	if localizer == nil {
		return key
	}
	return i18n.Localize(localizer, key)
}

// IsAuthenticated checks if the current request is authenticated
// Returns cached value set during context creation (O(1) instead of map lookup)
func (ctx *HandlerContext) IsAuthenticated() bool {
	if !ctx.authCached {
		// Fallback if auth state wasn't cached (shouldn't happen in normal operation)
		if ctx.SessionManager == nil {
			ctx.Log.Error().Msg("Session manager not available")
			return false
		}
		authenticated, ok := ctx.SessionManager.Get(ctx.Request.Context(), "authenticated").(bool)
		return ok && authenticated
	}
	return ctx.isAuthenticated
}

// IsAdmin checks if the current user is an admin
// Returns cached value set during context creation (O(1) instead of map lookup)
func (ctx *HandlerContext) IsAdmin() bool {
	if !ctx.authCached {
		// Fallback if auth state wasn't cached (shouldn't happen in normal operation)
		if ctx.SessionManager == nil {
			return false
		}
		isAdmin, ok := ctx.SessionManager.Get(ctx.Request.Context(), "is_admin").(bool)
		return ok && isAdmin
	}
	return ctx.isAdmin
}

// GetUsername returns the current username if authenticated
func (ctx *HandlerContext) GetUsername() string {
	if ctx.SessionManager == nil {
		return ""
	}

	if username, ok := ctx.SessionManager.Get(ctx.Request.Context(), "username").(string); ok {
		return username
	}
	return ""
}

// HandleError logs and responds with an HTTP error
func (ctx *HandlerContext) HandleError(err error, message string, statusCode int) {
	ctx.Log.Error().Err(err).Msg(message)
	http.Error(ctx.ResponseWriter, message, statusCode)
}

// ValidateStateManager ensures state manager is available
func (ctx *HandlerContext) ValidateStateManager() bool {
	if ctx.StateManager == nil {
		ctx.HandleError(nil, "Internal Server Error", http.StatusInternalServerError)
		return false
	}
	return true
}

// ValidateSessionManager ensures session manager is available
func (ctx *HandlerContext) ValidateSessionManager() bool {
	if ctx.SessionManager == nil {
		ctx.HandleError(nil, "Internal Server Error", http.StatusInternalServerError)
		return false
	}
	return true
}

// RequireAuthentication checks authentication and handles errors
func (ctx *HandlerContext) RequireAuthentication() bool {
	if !ctx.ValidateSessionManager() {
		return false
	}

	if !ctx.IsAuthenticated() {
		ctx.Log.Info().Msg("Authentication required, redirecting to login")
		returnURL := ctx.Request.URL.Path
		if ctx.Request.URL.RawQuery != "" {
			returnURL = returnURL + "?" + ctx.Request.URL.RawQuery
		}
		http.Redirect(ctx.ResponseWriter, ctx.Request, "/login?return="+returnURL, http.StatusSeeOther)
		return false
	}
	return true
}

// RequireAdminAuth checks admin authentication and handles errors
func (ctx *HandlerContext) RequireAdminAuth() bool {
	if !ctx.ValidateSessionManager() {
		return false
	}

	if !ctx.IsAdmin() {
		ctx.Log.Info().Msg("Admin authentication required")
		if ctx.IsAuthenticated() {
			ctx.Log.Warn().Msg("Authenticated user attempted to access admin area without privileges")
			localizer := i18n.GetLocalizerFromRequest(ctx.Request)
			http.Error(ctx.ResponseWriter, i18n.Localize(localizer, "Error.AccessDenied"), http.StatusForbidden)
			return false
		}

		returnURL := ctx.Request.URL.Path
		if ctx.Request.URL.RawQuery != "" {
			returnURL = returnURL + "?" + ctx.Request.URL.RawQuery
		}
		http.Redirect(ctx.ResponseWriter, ctx.Request, "/admin/login?return="+returnURL, http.StatusSeeOther)
		return false
	}
	return true
}

// GetCSRFToken gets or generates a CSRF token for the session
func (ctx *HandlerContext) GetCSRFToken() (string, error) {
	// Try to get token from context first
	if token, ok := security.CSRFTokenFromContext(ctx.Request.Context()); ok && token != "" {
		return token, nil
	}

	// Fallback to session
	if ctx.SessionManager != nil {
		if token, ok := ctx.SessionManager.Get(ctx.Request.Context(), "csrf_token").(string); ok && token != "" {
			return token, nil
		}

		// Generate new token if none exists
		if token, err := security.GenerateCSRFToken(); err == nil {
			ctx.SessionManager.Put(ctx.Request.Context(), "csrf_token", token)
			return token, nil
		}
	}

	return "", fmt.Errorf("failed to generate CSRF token")
}

// GetReturnURL constructs a return URL for redirects
func (ctx *HandlerContext) GetReturnURL() string {
	returnURL := ctx.Request.URL.Path
	if ctx.Request.URL.RawQuery != "" {
		returnURL = returnURL + "?" + ctx.Request.URL.RawQuery
	}
	return url.QueryEscape(returnURL)
}

// Redirect performs a simple HTTP redirect
func (ctx *HandlerContext) Redirect(path string) {
	http.Redirect(ctx.ResponseWriter, ctx.Request, path, http.StatusSeeOther)
}

// RedirectWithSuccess redirects with a success message
func (ctx *HandlerContext) RedirectWithSuccess(path, messageKey string) {
	msg := ctx.Translate(messageKey)
	params := url.Values{}
	params.Set("success", "1")
	params.Set("success_msg", msg)
	params.Set("lang", i18n.GetLanguage(ctx.Request))

	fullURL := path
	if strings.Contains(path, "?") {
		fullURL += "&" + params.Encode()
	} else {
		fullURL += "?" + params.Encode()
	}
	ctx.Redirect(fullURL)
}

// RedirectWithError redirects with an error message
func (ctx *HandlerContext) RedirectWithError(path, messageKey string) {
	msg := ctx.Translate(messageKey)
	params := url.Values{}
	params.Set("error", "1")
	params.Set("error_msg", msg)
	params.Set("lang", i18n.GetLanguage(ctx.Request))

	fullURL := path
	if strings.Contains(path, "?") {
		fullURL += "&" + params.Encode()
	} else {
		fullURL += "?" + params.Encode()
	}
	ctx.Redirect(fullURL)
}

// RedirectWithWarning redirects with a warning message
func (ctx *HandlerContext) RedirectWithWarning(path, messageKey string) {
	msg := ctx.Translate(messageKey)
	params := url.Values{}
	params.Set("warning", "1")
	params.Set("warning_msg", msg)
	params.Set("lang", i18n.GetLanguage(ctx.Request))

	fullURL := path
	if strings.Contains(path, "?") {
		fullURL += "&" + params.Encode()
	} else {
		fullURL += "?" + params.Encode()
	}
	ctx.Redirect(fullURL)
}

// RedirectWithParams redirects with custom URL parameters
func (ctx *HandlerContext) RedirectWithParams(path string, params map[string]string) {
	urlParams := url.Values{}
	for k, v := range params {
		urlParams.Set(k, v)
	}
	// Always include language
	urlParams.Set("lang", i18n.GetLanguage(ctx.Request))

	fullURL := path
	if strings.Contains(path, "?") {
		fullURL += "&" + urlParams.Encode()
	} else {
		fullURL += "?" + urlParams.Encode()
	}
	ctx.Redirect(fullURL)
}
