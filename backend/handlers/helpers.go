package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"pvmss/components"
	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/security"
	"pvmss/state"

	"github.com/alexedwards/scs/v2"
	"github.com/julienschmidt/httprouter"
)

// ValidateMethodAndParseForm validates HTTP method and parses form data
func ValidateMethodAndParseForm(w http.ResponseWriter, r *http.Request, requiredMethod string) bool {
	if r.Method != requiredMethod {
		RenderErrorPage(w, r, http.StatusMethodNotAllowed, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.MethodNotAllowed"))
		return false
	}

	if err := r.ParseForm(); err != nil {
		RenderErrorPage(w, r, http.StatusBadRequest, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.InvalidFormData"))
		return false
	}

	return true
}

// CreateHandlerLogger creates a standardized logger for handlers
func CreateHandlerLogger(handlerName string, r *http.Request) logger.Logger {
	logContext := logger.Get().With().Str("handler", handlerName)

	if r != nil {
		logContext = logContext.
			Str("method", r.Method).
			Str("path", r.URL.Path)
	}

	return logContext.Logger()
}

// PostOnlyHandler wraps a handler to only accept POST requests
func PostOnlyHandler(handler httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if r.Method != http.MethodPost {
			RenderErrorPage(w, r, http.StatusMethodNotAllowed, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.MethodNotAllowed"))
			return
		}
		handler(w, r, ps)
	}
}

// ParseFormMiddleware wraps a handler to parse form data first
func ParseFormMiddleware(handler httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if err := r.ParseForm(); err != nil {
			RenderErrorPage(w, r, http.StatusBadRequest, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.InvalidFormData"))
			return
		}
		handler(w, r, ps)
	}
}

// PostFormHandler combines POST validation and form parsing
func PostFormHandler(handler httprouter.Handle) httprouter.Handle {
	return PostOnlyHandler(ParseFormMiddleware(handler))
}

// RedirectWithSuccess redirects with success message in query params
func RedirectWithSuccess(w http.ResponseWriter, r *http.Request, targetURL, message string) {
	u, err := url.Parse(targetURL)
	if err != nil {
		http.Redirect(w, r, targetURL, http.StatusSeeOther)
		return
	}

	q := u.Query()
	q.Set("success", "1")
	if message != "" {
		q.Set("message", message)
	} else {
		q.Del("message")
	}
	u.RawQuery = q.Encode()

	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}

// RedirectWithError redirects with error message in query params
func RedirectWithError(w http.ResponseWriter, r *http.Request, targetURL, message string) {
	u, err := url.Parse(targetURL)
	if err != nil {
		http.Redirect(w, r, targetURL, http.StatusSeeOther)
		return
	}

	q := u.Query()
	q.Set("error", "1")
	if message != "" {
		q.Set("message", message)
	} else {
		q.Del("message")
	}
	u.RawQuery = q.Encode()

	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}

// RenderErrorPage renders a friendly error page with status code and message.
// It also provides navigation options (Back/Home) to help the user recover.
func RenderErrorPage(w http.ResponseWriter, r *http.Request, status int, message string) {
	// Prepare minimal data for the error template
	localizer := i18n.GetLocalizerFromRequest(r)
	// Best-effort return URL: prefer Referer, fallback to current path
	returnURL := ""
	if ref := r.Referer(); ref != "" {
		returnURL = ref
	} else if r.URL != nil {
		returnURL = r.URL.Path
	}

	errorData := components.ErrorData{
		Title:      i18n.Localize(localizer, "Error.Title"),
		StatusCode: status,
		Error:      message,
		ReturnURL:  returnURL,
		Lang:       i18n.GetLanguage(r),
	}

	// Translation function wrapper
	ctx := NewHandlerContext(w, r, "RespondWithTemplateError")
	translateFunc := func(key string) string {
		return ctx.Translate(key)
	}

	// Ensure dynamic error pages are not cached
	setNoCacheHeaders(w)
	// Set HTTP status before rendering the template body
	w.WriteHeader(status)

	// Render with Templ
	if err := components.ErrorPage(errorData, translateFunc).Render(r.Context(), w); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to render error page")
		// Fallback to plain text if Templ rendering fails
		http.Error(w, message, status)
	}
}

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

// Translate looks up a translation key using the request's locale, falling back to the key.
func (ctx *HandlerContext) Translate(key string) string {
	localizer := i18n.GetLocalizerFromRequest(ctx.Request)
	if localizer == nil {
		return key
	}
	return i18n.Localize(localizer, key)
}

// NewHandlerContext creates a new handler context with common setup
func NewHandlerContext(w http.ResponseWriter, r *http.Request, handlerName string) *HandlerContext {
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

// FormatMemoryGB converts memory from bytes or MB to GB with clean formatting
// Accepts bytes (from Proxmox API) or MB (from form input)
// Returns formatted string like "2 GB", "512 MB", "0.5 GB"
func FormatMemoryGB(value int64, isBytes bool) string {
	var memoryMB int64

	if isBytes {
		// Convert bytes to MB (Proxmox API returns bytes)
		memoryMB = value / (1024 * 1024)
	} else {
		// Already in MB (from form input)
		memoryMB = value
	}

	// Convert MB to GB
	memoryGB := float64(memoryMB) / 1024.0

	// Format based on size
	if memoryGB >= 1 {
		// For 1 GB or more, show as GB
		if memoryGB == float64(int64(memoryGB)) {
			// Whole number
			return fmt.Sprintf("%d GB", int64(memoryGB))
		}
		// Decimal
		return fmt.Sprintf("%.1f GB", memoryGB)
	}

	// Less than 1 GB, show as MB
	if memoryMB == int64(memoryMB) {
		return fmt.Sprintf("%d MB", memoryMB)
	}
	return fmt.Sprintf("%.0f MB", float64(memoryMB))
}

// BytesToGB converts bytes to GB as integer (for calculations)
func BytesToGB(bytes int64) int64 {
	return bytes / (1024 * 1024 * 1024)
}

// MBToGB converts MB to GB as integer (for calculations)
func MBToGB(mb int64) int64 {
	return mb / 1024
}

// FormatUptime formats uptime in seconds to human-readable format (days, hours, minutes, seconds)
// with i18n support
func FormatUptime(seconds int64, r *http.Request) string {
	localizer := i18n.GetLocalizerFromRequest(r)

	if seconds == 0 {
		return i18n.Localize(localizer, "Uptime.NotRunning")
	}

	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	var parts []string
	if days > 0 {
		if days == 1 {
			parts = append(parts, fmt.Sprintf("1 %s", i18n.Localize(localizer, "Uptime.Day")))
		} else {
			parts = append(parts, fmt.Sprintf("%d %s", days, i18n.Localize(localizer, "Uptime.Days")))
		}
	}
	if hours > 0 {
		if hours == 1 {
			parts = append(parts, fmt.Sprintf("1 %s", i18n.Localize(localizer, "Uptime.Hour")))
		} else {
			parts = append(parts, fmt.Sprintf("%d %s", hours, i18n.Localize(localizer, "Uptime.Hours")))
		}
	}
	if minutes > 0 {
		if minutes == 1 {
			parts = append(parts, fmt.Sprintf("1 %s", i18n.Localize(localizer, "Uptime.Minute")))
		} else {
			parts = append(parts, fmt.Sprintf("%d %s", minutes, i18n.Localize(localizer, "Uptime.Minutes")))
		}
	}
	if secs > 0 || len(parts) == 0 {
		if secs == 1 {
			parts = append(parts, fmt.Sprintf("1 %s", i18n.Localize(localizer, "Uptime.Second")))
		} else {
			parts = append(parts, fmt.Sprintf("%d %s", secs, i18n.Localize(localizer, "Uptime.Seconds")))
		}
	}

	// Join parts with commas - simple format for all languages
	if len(parts) == 1 {
		return parts[0]
	}
	result := ""
	for i, part := range parts {
		if i == 0 {
			result = part
		} else {
			result += ", " + part
		}
	}
	return result
}

// CreateNotificationScript generates JavaScript to show a notification
func CreateNotificationScript(notificationType, title, message string, options map[string]interface{}) string {
	script := fmt.Sprintf(`
		<script>
			document.addEventListener('DOMContentLoaded', function() {
				window.showNotification({
					type: '%s',
					title: '%s',
					message: '%s'`,
		notificationType, strings.ReplaceAll(title, "'", "\\'"), strings.ReplaceAll(message, "'", "\\'"))

	if options != nil {
		if icon, ok := options["icon"].(string); ok && icon != "" {
			script += fmt.Sprintf(",\n\t\t\t\t\ticon: '%s'", icon)
		}
		if duration, ok := options["duration"].(int); ok && duration > 0 {
			script += fmt.Sprintf(",\n\t\t\t\t\tduration: %d", duration)
		}
		if dismissible, ok := options["dismissible"].(bool); ok {
			script += fmt.Sprintf(",\n\t\t\t\t\tdismissible: %t", dismissible)
		}
		if urgent, ok := options["urgent"].(bool); ok {
			script += fmt.Sprintf(",\n\t\t\t\t\turgent: %t", urgent)
		}
	}

	script += `
				});
			});
		</script>`

	return script
}

// ShowSuccessNotification creates a success notification script
func ShowSuccessNotification(title, message string, options map[string]interface{}) string {
	if options == nil {
		options = make(map[string]interface{})
	}
	if _, ok := options["icon"]; !ok {
		options["icon"] = "fas fa-check-circle"
	}
	return CreateNotificationScript("success", title, message, options)
}

// ShowErrorNotification creates an error notification script
func ShowErrorNotification(title, message string, options map[string]interface{}) string {
	if options == nil {
		options = make(map[string]interface{})
	}
	if _, ok := options["icon"]; !ok {
		options["icon"] = "fas fa-exclamation-circle"
	}
	if _, ok := options["urgent"]; !ok {
		options["urgent"] = true
	}
	return CreateNotificationScript("danger", title, message, options)
}

// ShowWarningNotification creates a warning notification script
func ShowWarningNotification(title, message string, options map[string]interface{}) string {
	if options == nil {
		options = make(map[string]interface{})
	}
	if _, ok := options["icon"]; !ok {
		options["icon"] = "fas fa-exclamation-triangle"
	}
	return CreateNotificationScript("warning", title, message, options)
}

// ShowInfoNotification creates an info notification script
func ShowInfoNotification(title, message string, options map[string]interface{}) string {
	if options == nil {
		options = make(map[string]interface{})
	}
	if _, ok := options["icon"]; !ok {
		options["icon"] = "fas fa-info-circle"
	}
	return CreateNotificationScript("info", title, message, options)
}

// CreateProgressScript generates JavaScript to show a progress bar
func CreateProgressScript(title, message string, options map[string]interface{}) string {
	script := fmt.Sprintf(`
		<script>
			document.addEventListener('DOMContentLoaded', function() {
				window.showProgress({
					title: '%s',
					message: '%s'`,
		strings.ReplaceAll(title, "'", "\\'"), strings.ReplaceAll(message, "'",
			"\\'"))

	if options != nil {
		if progressType, ok := options["type"].(string); ok && progressType != "" {
			script += fmt.Sprintf(",\n\t\t\t\t\ttype: '%s'", progressType)
		}
		if icon, ok := options["icon"].(string); ok && icon != "" {
			script += fmt.Sprintf(",\n\t\t\t\t\ticon: '%s'", icon)
		}
		if showClose, ok := options["showClose"].(bool); ok {
			script += fmt.Sprintf(",\n\t\t\t\t\tshowClose: %t", showClose)
		}
		if showDetails, ok := options["showDetails"].(bool); ok {
			script += fmt.Sprintf(",\n\t\t\t\t\tshowDetails: %t", showDetails)
		}
		if onCancel, ok := options["onCancel"].(string); ok && onCancel != "" {
			script += fmt.Sprintf(",\n\t\t\t\t\tonCancel: function() { %s }", onCancel)
		}
	}

	script += `
				});
			});
		</script>`

	return script
}

// UpdateProgressScript generates JavaScript to update a progress bar
func UpdateProgressScript(progressId string, progress int, message string) string {
	return fmt.Sprintf(`
		<script>
			document.addEventListener('DOMContentLoaded', function() {
				window.updateProgress('%s', {
					progress: %d,
					message: '%s'
				});
			});
		</script>`,
		strings.ReplaceAll(progressId, "'", "\\'"),
		progress,
		strings.ReplaceAll(message, "'", "\\'"))
}

// HideProgressScript generates JavaScript to hide a progress bar
func HideProgressScript(progressId string) string {
	return fmt.Sprintf(`
		<script>
			document.addEventListener('DOMContentLoaded', function() {
				window.hideProgress('%s');
			});
		</script>`,
		strings.ReplaceAll(progressId, "'", "\\'"))
}
