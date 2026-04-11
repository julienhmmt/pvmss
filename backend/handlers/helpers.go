package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"

	"pvmss/logger"

	"github.com/julienschmidt/httprouter"
)

// ValidateMethodAndParseForm validates HTTP method and parses form data
func ValidateMethodAndParseForm(w http.ResponseWriter, r *http.Request, requiredMethod string) bool {
	if r.Method != requiredMethod {
		RenderErrorPage(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
		return false
	}

	if err := r.ParseForm(); err != nil {
		RenderErrorPage(w, r, http.StatusBadRequest, "INVALID_FORM_DATA")
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
			RenderErrorPage(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
			return
		}
		handler(w, r, ps)
	}
}

// ParseFormMiddleware wraps a handler to parse form data first
func ParseFormMiddleware(handler httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if err := r.ParseForm(); err != nil {
			RenderErrorPage(w, r, http.StatusBadRequest, "INVALID_FORM_DATA")
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

// RenderErrorPage writes a JSON error response with the given status code.
func RenderErrorPage(w http.ResponseWriter, r *http.Request, status int, message string) {
	setNoCacheHeaders(w)
	_ = logger.Get() // keep logger import used
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(JSONErrorResponse{
		Code:    message,
		Message: message,
	})
}
