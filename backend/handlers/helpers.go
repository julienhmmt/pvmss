package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"pvmss/security"
	"pvmss/state"

	"github.com/alexedwards/scs/v2"
	"github.com/rs/zerolog"
)

// HandlerContext provides common context for handlers
type HandlerContext struct {
	Log            zerolog.Logger
	StateManager   state.StateManager
	SessionManager *scs.SessionManager
	Request        *http.Request
	ResponseWriter http.ResponseWriter
}

// NewHandlerContext creates a new handler context with common setup
func NewHandlerContext(w http.ResponseWriter, r *http.Request, handlerName string) *HandlerContext {
	log := CreateHandlerLogger(handlerName, r)

	stateManager := getStateManager(r)
	var sessionManager *scs.SessionManager
	if stateManager != nil {
		sessionManager = stateManager.GetSessionManager()
	}

	return &HandlerContext{
		Log:            log,
		StateManager:   stateManager,
		SessionManager: sessionManager,
		Request:        r,
		ResponseWriter: w,
	}
}

// IsAuthenticated checks if the current request is authenticated
func (ctx *HandlerContext) IsAuthenticated() bool {
	if ctx.SessionManager == nil {
		ctx.Log.Error().Msg("Session manager not available")
		return false
	}

	authenticated, ok := ctx.SessionManager.Get(ctx.Request.Context(), "authenticated").(bool)
	return ok && authenticated
}

// IsAdmin checks if the current user is an admin
func (ctx *HandlerContext) IsAdmin() bool {
	if ctx.SessionManager == nil {
		return false
	}

	isAdmin, ok := ctx.SessionManager.Get(ctx.Request.Context(), "is_admin").(bool)
	return ok && isAdmin
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

// RenderTemplate renders a template with common data population
func (ctx *HandlerContext) RenderTemplate(templateName string, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}

	ctx.Log.Debug().Str("template", templateName).Msg("Rendering template")
	renderTemplateInternal(ctx.ResponseWriter, ctx.Request, templateName, data)
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
			http.Error(ctx.ResponseWriter, "Access Denied: Admin privileges required", http.StatusForbidden)
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

// FormatBytes formats byte values to human-readable format (MB/GB)
func FormatBytes(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}

	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
