package app

import (
	"net/http"

	"pvmss/handlers"
)

// TestApp represents a minimal application for testing
type TestApp struct {
	Router   *http.ServeMux
	Handlers *handlers.TestHandlerCollection
}

// MakeTestApp creates a new test application instance with minimal setup
func MakeTestApp() *TestApp {
	// Create handlers
	hc := &handlers.TestHandlerCollection{}

	// Create router
	router := http.NewServeMux()

	// Register minimal routes for testing
	registerTestRoutes(router, hc)

	return &TestApp{
		Router:   router,
		Handlers: hc,
	}
}

// registerTestRoutes registers minimal HTTP routes for testing
func registerTestRoutes(router *http.ServeMux, hc *handlers.TestHandlerCollection) {
	// Health endpoints
	router.HandleFunc("/health", hc.HealthHandler)
	router.HandleFunc("/api/health", hc.APIHealthHandler)
	router.HandleFunc("/api/health/proxmox", hc.ProxmoxHealthHandler)

	// Authentication endpoints
	router.HandleFunc("/login", hc.LoginHandler)
	router.HandleFunc("/admin/login", hc.AdminLoginHandler)
	router.HandleFunc("/logout", hc.LogoutHandler)

	// User endpoints
	router.HandleFunc("/", hc.HomeHandler)
	router.HandleFunc("/search", hc.SearchHandler)
	router.HandleFunc("/vm/create", hc.VMCreateHandler)
	router.HandleFunc("/userpool/create-self", hc.UserPoolSelfCreateHandler)

	// Admin endpoints
	router.HandleFunc("/admin", hc.AdminPageHandler)
	router.HandleFunc("/admin/nodes", hc.NodesPageHandler)
	router.HandleFunc("/admin/tags", hc.TagsPageHandler)
	router.HandleFunc("/admin/storage", hc.StoragePageHandler)
	router.HandleFunc("/admin/iso", hc.ISOPageHandler)
	router.HandleFunc("/admin/vmbr", hc.VMBRPageHandler)
	router.HandleFunc("/admin/limits", hc.LimitsPageHandler)
	router.HandleFunc("/admin/userpool", hc.UserPoolPageHandler)
	router.HandleFunc("/admin/appinfo", hc.AppInfoPageHandler)

	// Static files (simplified)
	router.HandleFunc("/favicon.ico", hc.FaviconHandler)
	router.HandleFunc("/css/", hc.StaticHandler)
	router.HandleFunc("/js/", hc.StaticHandler)
	router.HandleFunc("/webfonts/", hc.StaticHandler)
	router.HandleFunc("/components/", hc.StaticHandler)
}
