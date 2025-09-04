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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return false
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handler(w, r, ps)
	}
}

// ParseFormMiddleware wraps a handler to parse form data first
func ParseFormMiddleware(handler httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
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
