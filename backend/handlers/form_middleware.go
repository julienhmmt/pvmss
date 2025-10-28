package handlers

import (
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"

	"pvmss/constants"
	"pvmss/logger"
	"pvmss/security"
)

// Form middleware functions for request validation and processing
// These are standalone functions that can be composed together

// WithFormValidation validates content type and parses form data
func WithFormValidation(handler httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Validate content type for POST/PUT/PATCH requests
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			contentType := r.Header.Get("Content-Type")
			// Allow form-encoded or multipart form data
			if contentType != "" &&
				!strings.Contains(contentType, "application/x-www-form-urlencoded") &&
				!strings.Contains(contentType, "multipart/form-data") {
				logger.Get().Warn().Str("content_type", contentType).Msg("Invalid content type for form")
				http.Error(w, "Invalid content type", http.StatusUnsupportedMediaType)
				return
			}
		}

		// Parse form data
		if err := r.ParseForm(); err != nil {
			logger.Get().Error().Err(err).Msg("Failed to parse form data")
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		// Security audit: detect suspicious input patterns
		if len(r.Form) > 0 {
			patterns := []string{"<script", "script>", "union ", "select ", "drop ", "insert ", "update ", "delete "}
			matches := make([]string, 0)
			for key, vals := range r.Form {
				for _, v := range vals {
					lv := strings.ToLower(v)
					for _, p := range patterns {
						if strings.Contains(lv, p) {
							matches = append(matches, key)
							break
						}
					}
				}
			}
			if len(matches) > 0 {
				u := ""
				if s := security.GetSession(r); s != nil {
					if name, ok := s.Get(r.Context(), "username").(string); ok {
						u = name
					}
				}
				logger.Get().Warn().
					Str("ip", r.RemoteAddr).
					Str("path", r.URL.Path).
					Str("username", u).
					Strs("form_keys", matches).
					Msg("Suspicious input patterns detected")
			}
		}

		handler(w, r, ps)
	}
}

// WithCSRFValidation validates CSRF tokens for state-changing requests
func WithCSRFValidation(handler httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Skip CSRF check for safe methods
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			handler(w, r, ps)
			return
		}

		// Get CSRF token from form or header
		token := r.FormValue("csrf_token")
		if token == "" {
			token = r.Header.Get("X-CSRF-Token")
		}

		// Get expected token from session
		if session := security.GetSession(r); session != nil {
			expectedToken, ok := session.Get(r.Context(), "csrf_token").(string)
			if !ok || expectedToken == "" || token != expectedToken {
				u := ""
				if name, ok := session.Get(r.Context(), "username").(string); ok {
					u = name
				}
				logger.Get().Warn().
					Str("ip", r.RemoteAddr).
					Str("path", r.URL.Path).
					Str("username", u).
					Bool("token_present", token != "").
					Msg("CSRF validation failed")
				http.Error(w, "Invalid or missing CSRF token", http.StatusForbidden)
				return
			}
		} else {
			logger.Get().Error().
				Str("ip", r.RemoteAddr).
				Str("path", r.URL.Path).
				Msg("Session not available for CSRF validation")
			http.Error(w, "Session error", http.StatusInternalServerError)
			return
		}

		handler(w, r, ps)
	}
}

// WithSizeLimits adds request body size limits
func WithSizeLimits(maxSize int64) func(httprouter.Handle) httprouter.Handle {
	return func(handler httprouter.Handle) httprouter.Handle {
		return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
			// Limit request body size (prevents DoS attacks)
			r.Body = http.MaxBytesReader(w, r.Body, maxSize)
			handler(w, r, ps)
		}
	}
}

// WithMethodCheck validates allowed HTTP methods
func WithMethodCheck(allowedMethods ...string) func(httprouter.Handle) httprouter.Handle {
	// Pre-compute method map for O(1) lookup
	methodMap := make(map[string]bool, len(allowedMethods))
	for _, method := range allowedMethods {
		methodMap[strings.ToUpper(method)] = true
	}

	return func(handler httprouter.Handle) httprouter.Handle {
		return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
			if !methodMap[r.Method] {
				w.Header().Set("Allow", strings.Join(allowedMethods, ", "))
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			handler(w, r, ps)
		}
	}
}

// WithFormLogging logs form submissions (excludes sensitive fields)
func WithFormLogging(handlerName string) func(httprouter.Handle) httprouter.Handle {
	// List of sensitive field substrings to exclude from logs
	sensitiveFields := []string{"password", "secret", "token", "key", "auth", "credential"}

	return func(handler httprouter.Handle) httprouter.Handle {
		return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
			// Only log state-changing methods
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodDelete {
				log := CreateHandlerLogger(handlerName, r)

				// Collect non-sensitive form field names
				formKeys := make([]string, 0, len(r.Form))
				for key := range r.Form {
					keyLower := strings.ToLower(key)
					isSensitive := false
					for _, sensitive := range sensitiveFields {
						if strings.Contains(keyLower, sensitive) {
							isSensitive = true
							break
						}
					}
					if !isSensitive {
						formKeys = append(formKeys, key)
					}
				}

				if len(formKeys) > 0 {
					log.Info().Strs("form_fields", formKeys).Msg("Form submission")
				} else {
					log.Info().Msg("Form submission (all fields sensitive)")
				}
			}

			handler(w, r, ps)
		}
	}
}

// ChainMiddleware combines multiple middleware functions (applied right-to-left)
func ChainMiddleware(handler httprouter.Handle, middlewares ...func(httprouter.Handle) httprouter.Handle) httprouter.Handle {
	// Apply middlewares in reverse order so they execute in the specified order
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// StandardFormHandler creates a standard form middleware chain
// Includes: method validation, size limits, form parsing, logging
func StandardFormHandler(handlerName string, handler httprouter.Handle) httprouter.Handle {
	return ChainMiddleware(handler,
		WithFormLogging(handlerName),
		WithFormValidation,
		WithSizeLimits(constants.MaxFormSize),
		WithMethodCheck(http.MethodGet, http.MethodPost),
	)
}

// SecureFormHandler creates a secure form chain with CSRF protection
func SecureFormHandler(handlerName string, handler httprouter.Handle) httprouter.Handle {
	return ChainMiddleware(handler,
		WithFormLogging(handlerName),
		WithCSRFValidation,
		WithFormValidation,
		WithSizeLimits(constants.MaxFormSize),
		WithMethodCheck(http.MethodGet, http.MethodPost),
	)
}
