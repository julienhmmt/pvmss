package handlers

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// AdminMiddleware contains middleware functions for admin-level operations
type AdminMiddleware struct{}

// NewAdminMiddleware creates a new instance of AdminMiddleware
func NewAdminMiddleware() *AdminMiddleware {
	return &AdminMiddleware{}
}

// WithAdminAuth wraps a handler with admin authentication middleware
func (am *AdminMiddleware) WithAdminAuth(handler httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Check if user is authenticated and has admin privileges
		if !IsAdmin(r) {
			RenderErrorPage(w, r, http.StatusForbidden, "Admin access required")
			return
		}

		// Call the actual handler
		handler(w, r, ps)
	}
}

// WithAdminAuthHTTP wraps an HTTP handler function with admin authentication middleware
func (am *AdminMiddleware) WithAdminAuthHTTP(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if user is authenticated and has admin privileges
		if !IsAdmin(r) {
			RenderErrorPage(w, r, http.StatusForbidden, "Admin access required")
			return
		}

		// Call the actual handler
		handler(w, r)
	}
}

// WithLogging adds request logging to handlers
func (am *AdminMiddleware) WithLogging(handlerName string, handler httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		log := CreateHandlerLogger(handlerName, r)
		log.Info().Str("method", r.Method).Str("path", r.URL.Path).Msg("Admin handler called")

		// Call the actual handler
		handler(w, r, ps)
	}
}

// ChainMiddleware combines multiple middleware functions for httprouter handlers
func (am *AdminMiddleware) ChainMiddleware(handler httprouter.Handle, middlewares ...func(httprouter.Handle) httprouter.Handle) httprouter.Handle {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// AdminAuthChain creates a complete middleware chain for admin handlers
func (am *AdminMiddleware) AdminAuthChain(handlerName string, handler httprouter.Handle) httprouter.Handle {
	return am.ChainMiddleware(handler,
		func(h httprouter.Handle) httprouter.Handle { return am.WithLogging(handlerName, h) },
		am.WithAdminAuth,
	)
}
