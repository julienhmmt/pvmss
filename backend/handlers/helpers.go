package handlers

import (
	"net/http"
	"net/url"

	"pvmss/components"
	"pvmss/i18n"
	"pvmss/logger"

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
	ctx := HandlerContextWith(w, r, "RespondWithTemplateError")
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

// NewHandlerContext creates a new handler context with common setup
// Deprecated: Use HandlerContextWith instead
func NewHandlerContext(w http.ResponseWriter, r *http.Request, handlerName string) *HandlerContext {
	return HandlerContextWith(w, r, handlerName)
}
