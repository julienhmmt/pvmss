package handlers

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// RouteHelpers provides utility functions for common route registration patterns
type RouteHelpers struct{}

// NewRouteHelpers creates a new instance of RouteHelpers
func NewRouteHelpers() *RouteHelpers {
	return &RouteHelpers{}
}

// registerRoute is a helper to register a route with the appropriate HTTP method
func registerRoute(router *httprouter.Router, method, path string, handler httprouter.Handle) {
	switch method {
	case "GET":
		router.GET(path, handler)
	case "POST":
		router.POST(path, handler)
	case "PUT":
		router.PUT(path, handler)
	case "DELETE":
		router.DELETE(path, handler)
	}
}

// RegisterAdminRoute registers an admin-protected route
func (rh *RouteHelpers) RegisterAdminRoute(router *httprouter.Router, method, path string, handler func(w http.ResponseWriter, r *http.Request, ps httprouter.Params)) {
	wrappedHandler := HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r, httprouter.ParamsFromContext(r.Context()))
	}))
	registerRoute(router, method, path, wrappedHandler)
}

// RegisterAdminRouteWithRedirect registers an admin route and its trailing-slash redirect variant
func (rh *RouteHelpers) RegisterAdminRouteWithRedirect(router *httprouter.Router, path string, handler func(w http.ResponseWriter, r *http.Request, ps httprouter.Params)) {
	// Main route
	rh.RegisterAdminRoute(router, "GET", path, handler)

	// Trailing-slash redirect
	redirectPath := path + "/"
	router.GET(redirectPath, HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, path, http.StatusMovedPermanently)
	})))
}

// AdminPageRoutes is a helper struct for registering admin page routes with common patterns
type AdminPageRoutes struct {
	helpers *RouteHelpers
}

// NewAdminPageRoutes creates a new AdminPageRoutes helper
func NewAdminPageRoutes() *AdminPageRoutes {
	return &AdminPageRoutes{
		helpers: NewRouteHelpers(),
	}
}

// RegisterCRUDRoutes registers common CRUD routes for an admin resource
func (apr *AdminPageRoutes) RegisterCRUDRoutes(router *httprouter.Router, basePath string, handlers map[string]func(w http.ResponseWriter, r *http.Request, ps httprouter.Params)) {
	// Main page (with redirect)
	if pageHandler, exists := handlers["page"]; exists {
		apr.helpers.RegisterAdminRouteWithRedirect(router, basePath, pageHandler)
	}

	// Update handler
	if updateHandler, exists := handlers["update"]; exists {
		apr.helpers.RegisterAdminRoute(router, "POST", basePath+"/update", updateHandler)
	}

	// Toggle handler
	if toggleHandler, exists := handlers["toggle"]; exists {
		apr.helpers.RegisterAdminRoute(router, "POST", basePath+"/toggle", toggleHandler)
	}

	// Create handler
	if createHandler, exists := handlers["create"]; exists {
		apr.helpers.RegisterAdminRoute(router, "POST", basePath+"/create", createHandler)
	}

	// Delete handler
	if deleteHandler, exists := handlers["delete"]; exists {
		apr.helpers.RegisterAdminRoute(router, "POST", basePath+"/delete", deleteHandler)
	}
}
