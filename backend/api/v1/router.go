package apiv1

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"pvmss/state"
)

// RegisterRoutes mounts all /api/v1/ routes onto the provided router.
// JWT-protected routes are wrapped with JWTMiddleware; the auth exchange
// endpoint only needs a session (session is loaded by the API middleware chain).
func RegisterRoutes(router *httprouter.Router, s state.StateManager) {
	authHandler := NewAuthHandler(s)
	vmHandler := NewVMHandler(s)
	vmActionHandler := NewVMActionHandler(s)
	searchHandler := NewSearchHandler(s)

	// Auth routes — no JWT required (login/exchange issue tokens)
	router.POST("/api/v1/auth/login", wrap(authHandler.Login))
	router.POST("/api/v1/auth/exchange", wrap(authHandler.Exchange))
	router.POST("/api/v1/auth/refresh", wrap(authHandler.Refresh))
	router.POST("/api/v1/auth/logout", wrap(authHandler.Logout))

	// Authenticated auth routes
	router.GET("/api/v1/auth/me", jwtWrap(s, authHandler.Me))

	// VM routes — JWT required
	router.GET("/api/v1/vms", jwtWrap(s, vmHandler.ListVMs))
	router.GET("/api/v1/vms/:id", jwtWrap(s, vmHandler.GetVM))
	router.POST("/api/v1/vms/:id/action", jwtWrap(s, vmActionHandler.VMAction))

	// Search routes — JWT required
	router.GET("/api/v1/search/vms", jwtWrap(s, searchHandler.SearchVMs))
}

// wrap converts a plain http.HandlerFunc into the httprouter.Handle signature.
func wrap(h http.HandlerFunc) httprouter.Handle {
	return httprouterWrap(h)
}

// jwtWrap wraps a handler with JWT authentication and converts it to httprouter.Handle.
func jwtWrap(s state.StateManager, h http.HandlerFunc) httprouter.Handle {
	return httprouterWrap(JWTMiddleware(s, h))
}
