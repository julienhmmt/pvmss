package handlers

import (
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"

	"pvmss/logger"
)

// FormMiddleware contains middleware functions for form handling operations
type FormMiddleware struct{}

// NewFormMiddleware creates a new instance of FormMiddleware
func NewFormMiddleware() *FormMiddleware {
	return &FormMiddleware{}
}

// WithFormValidation wraps a handler with form validation middleware
func (fm *FormMiddleware) WithFormValidation(handler httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Validate content type for POST/PUT requests
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			contentType := r.Header.Get("Content-Type")
			if !strings.Contains(contentType, "application/x-www-form-urlencoded") &&
				!strings.Contains(contentType, "multipart/form-data") {
				http.Error(w, "Invalid content type for form submission", http.StatusBadRequest)
				return
			}
		}

		// Parse form data
		if err := r.ParseForm(); err != nil {
			logger.Get().Error().Err(err).Msg("Failed to parse form data")
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		// Call the actual handler
		handler(w, r, ps)
	}
}

// WithCSRFProtection adds CSRF protection to form handlers
func (fm *FormMiddleware) WithCSRFProtection(handler httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Skip CSRF check for GET requests
		if r.Method == http.MethodGet {
			handler(w, r, ps)
			return
		}

		// Check for CSRF token in form data or headers
		token := r.FormValue("csrf_token")
		if token == "" {
			token = r.Header.Get("X-CSRF-Token")
		}

		// Validate CSRF token (implement your CSRF validation logic here)
		if token == "" {
			logger.Get().Warn().Str("path", r.URL.Path).Msg("Missing CSRF token")
			http.Error(w, "CSRF token required", http.StatusBadRequest)
			return
		}

		// Call the actual handler
		handler(w, r, ps)
	}
}

// WithSizeLimits adds request size limits for form submissions
func (fm *FormMiddleware) WithSizeLimits(maxSize int64) func(httprouter.Handle) httprouter.Handle {
	return func(handler httprouter.Handle) httprouter.Handle {
		return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
			// Limit request body size
			r.Body = http.MaxBytesReader(w, r.Body, maxSize)

			// Call the actual handler
			handler(w, r, ps)
		}
	}
}

// WithMethodCheck ensures only specific HTTP methods are allowed
func (fm *FormMiddleware) WithMethodCheck(allowedMethods ...string) func(httprouter.Handle) httprouter.Handle {
	methodMap := make(map[string]bool)
	for _, method := range allowedMethods {
		methodMap[method] = true
	}

	return func(handler httprouter.Handle) httprouter.Handle {
		return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
			if !methodMap[r.Method] {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			// Call the actual handler
			handler(w, r, ps)
		}
	}
}

// WithFormLogging adds form-specific logging
func (fm *FormMiddleware) WithFormLogging(handlerName string) func(httprouter.Handle) httprouter.Handle {
	return func(handler httprouter.Handle) httprouter.Handle {
		return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
			log := CreateHandlerLogger(handlerName, r)

			// Log form submission details (without sensitive data)
			if r.Method == http.MethodPost || r.Method == http.MethodPut {
				formKeys := make([]string, 0, len(r.Form))
				for key := range r.Form {
					// Skip logging sensitive fields
					if !strings.Contains(strings.ToLower(key), "password") &&
						!strings.Contains(strings.ToLower(key), "secret") &&
						!strings.Contains(strings.ToLower(key), "token") {
						formKeys = append(formKeys, key)
					}
				}
				log.Info().Strs("form_fields", formKeys).Msg("Form submission received")
			}

			// Call the actual handler
			handler(w, r, ps)
		}
	}
}

// ChainFormMiddleware combines multiple form middleware functions
func (fm *FormMiddleware) ChainFormMiddleware(handler httprouter.Handle, middlewares ...func(httprouter.Handle) httprouter.Handle) httprouter.Handle {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// StandardFormChain creates a standard middleware chain for form handlers
func (fm *FormMiddleware) StandardFormChain(handlerName string, handler httprouter.Handle) httprouter.Handle {
	return fm.ChainFormMiddleware(handler,
		fm.WithFormLogging(handlerName),
		fm.WithFormValidation,
		fm.WithSizeLimits(10*1024*1024), // 10MB limit
		fm.WithMethodCheck(http.MethodGet, http.MethodPost),
	)
}

// AdminFormChain creates a form middleware chain specifically for admin forms
func (fm *FormMiddleware) AdminFormChain(handlerName string, handler httprouter.Handle) httprouter.Handle {
	adminMiddleware := NewAdminMiddleware()
	return adminMiddleware.ChainMiddleware(
		fm.StandardFormChain(handlerName, handler),
		adminMiddleware.WithAdminAuth,
	)
}
