// Package server - HTTP routes configuration
package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"pvmss/handlers"
	"pvmss/security"
)

// setupStaticRoutes configures static file serving
func setupStaticRoutes(r *http.ServeMux) {
	// Serve CSS files
	r.Handle("/css/", http.StripPrefix("/css/",
		http.FileServer(http.Dir("frontend/css/"))))

	// Serve JavaScript files
	r.Handle("/js/", http.StripPrefix("/js/",
		http.FileServer(http.Dir("frontend/js/"))))

	// Serve root and handle SPA routing
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Try to serve the requested file directly
		if path != "/" {
			// Check if it's a CSS or JS file first
			if strings.HasPrefix(path, "/css/") || strings.HasPrefix(path, "/js/") {
				// Let the file server handle it
				http.NotFound(w, r)
				return
			}

			// Try to serve other files from the frontend directory
			filePath := filepath.Join("frontend", path)
			if _, err := os.Stat(filePath); err == nil {
				http.ServeFile(w, r, filePath)
				return
			}
		}

		// For any other case, use the index handler
		handlers.IndexHandler(w, r)
	})
}

// setupAPIRoutes configures API endpoints
func setupAPIRoutes(r *http.ServeMux) {
	// VM API
	r.HandleFunc("/api/vm/status", handlers.APIVmStatusHandler)

	// Tags API
	r.HandleFunc("/api/tags", handlers.TagsHandler)
	r.HandleFunc("/api/tags/", handlers.TagsHandler)

	// Storage API
	r.HandleFunc("/api/storage", handlers.StorageHandler)

	// ISO API
	r.HandleFunc("/api/iso/all", handlers.AllIsosHandler)
	r.HandleFunc("/api/iso/settings", handlers.UpdateIsoSettingsHandler)

	// VMBR API
	r.HandleFunc("/api/vmbr/all", handlers.AllVmbrsHandler)
	r.HandleFunc("/api/vmbr/settings", handlers.UpdateVmbrSettingsHandler)

	// Settings API
	r.HandleFunc("/api/settings", handlers.SettingsHandler)

	// Limits API
	r.HandleFunc("/api/limits", handlers.LimitsHandler)
}

// setupPageRoutes configures page routes
func setupPageRoutes(r *http.ServeMux) {
	r.HandleFunc("/search", handlers.SearchHandler)
	r.HandleFunc("/vm/details", handlers.VmDetailsHandler)
	r.HandleFunc("/vm/action", handlers.VmActionHandler)
	r.HandleFunc("/create-vm", handlers.CreateVmHandler)
	r.HandleFunc("/storage", handlers.StoragePageHandler)
	r.HandleFunc("/iso", handlers.IsoPageHandler)
	r.HandleFunc("/vmbr", handlers.VmbrPageHandler)
}

// setupAuthRoutes configures authentication routes
func setupAuthRoutes(r *http.ServeMux) {
	r.HandleFunc("/login", handlers.LoginHandler)
	r.HandleFunc("/logout", handlers.LogoutHandler)

	// Protected admin routes
	authedRoutes := http.NewServeMux()
	authedRoutes.HandleFunc("/admin", handlers.AdminHandler)
	r.Handle("/admin", security.AuthMiddleware(authedRoutes))
}

// setupDocRoutes configures documentation routes
func setupDocRoutes(r *http.ServeMux) {
	r.HandleFunc("/docs/admin", serveDocHandler("admin"))
	r.HandleFunc("/docs/user", serveDocHandler("user"))
}

// setupHealthRoute configures health check route
func setupHealthRoute(r *http.ServeMux) {
	r.HandleFunc("/health", handlers.HealthHandler)
}

// serveDocHandler returns a handler for serving documentation
func serveDocHandler(docType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get language from query parameter or default to English
		lang := r.URL.Query().Get("lang")
		if lang == "" {
			lang = "en"
		}

		// Validate doc type
		if docType != "admin" && docType != "user" {
			http.NotFound(w, r)
			return
		}

		// Call the docs handler with the appropriate parameters
		handlers.DocsHandler(w, r, docType, lang)
	}
}
