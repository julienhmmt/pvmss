package handlers

import (
	"fmt"
	"net/http"
	"pvmss/logger"

	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
)

// Common validation and setup helpers to reduce code duplication

// ValidateMethodAndParseForm validates HTTP method and parses form data
func ValidateMethodAndParseForm(w http.ResponseWriter, r *http.Request, requiredMethod string) bool {
	if r.Method != requiredMethod {
		RenderErrorPage(w, r, http.StatusMethodNotAllowed, "Method not allowed")
		return false
	}

	if err := r.ParseForm(); err != nil {
		RenderErrorPage(w, r, http.StatusBadRequest, "Invalid form data")
		return false
	}

	return true
}

// CreateHandlerLogger creates a standardized logger for handlers
func CreateHandlerLogger(handlerName string, r *http.Request) zerolog.Logger {
	logContext := logger.Get().With().Str("handler", handlerName)

	if r != nil {
		logContext = logContext.
			Str("method", r.Method).
			Str("path", r.URL.Path)
	}

	return logContext.Logger()
}

// AdminPageData creates common data structure for admin pages
func AdminPageData(title, activeSection string) map[string]interface{} {
	return map[string]interface{}{
		"Title":       title,
		"AdminActive": activeSection,
	}
}

// AdminPageDataWithMessage creates admin page data with success/error messages
func AdminPageDataWithMessage(title, activeSection, successMsg, errorMsg string) map[string]interface{} {
	data := AdminPageData(title, activeSection)

	if successMsg != "" {
		data["Success"] = true
		data["SuccessMessage"] = successMsg
	}

	if errorMsg != "" {
		data["Error"] = errorMsg
	}

	return data
}

// PostOnlyHandler wraps a handler to only accept POST requests
func PostOnlyHandler(handler httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if r.Method != http.MethodPost {
			RenderErrorPage(w, r, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		handler(w, r, ps)
	}
}

// ParseFormMiddleware wraps a handler to parse form data first
func ParseFormMiddleware(handler httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if err := r.ParseForm(); err != nil {
			RenderErrorPage(w, r, http.StatusBadRequest, "Invalid form data")
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
func RedirectWithSuccess(w http.ResponseWriter, r *http.Request, url, message string) {
	redirectURL := fmt.Sprintf("%s?success=1&message=%s", url, message)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// RedirectWithError redirects with error message in query params
func RedirectWithError(w http.ResponseWriter, r *http.Request, url, message string) {
	redirectURL := fmt.Sprintf("%s?error=1&message=%s", url, message)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// RenderErrorPage renders a friendly error page with status code and message.
// It also provides navigation options (Back/Home) to help the user recover.
func RenderErrorPage(w http.ResponseWriter, r *http.Request, status int, message string) {
	// Prepare minimal data for the error template
	data := map[string]interface{}{
		"Title":      "Error",
		"StatusCode": status,
		"Error":      message,
	}

	// Best-effort return URL: prefer Referer, fallback to current path
	if ref := r.Referer(); ref != "" {
		data["ReturnURL"] = ref
	} else if r.URL != nil {
		data["ReturnURL"] = r.URL.Path
	}

	// Ensure dynamic error pages are not cached
	setNoCacheHeaders(w)
	// Set HTTP status before rendering the template body
	w.WriteHeader(status)

	// Render the dedicated error content inside the standard layout
	renderTemplateInternal(w, r, "error", data)
}
