package handlers

import (
	"encoding/json"
	"net/http"
)

// TestHandlerCollection provides minimal handlers for testing
type TestHandlerCollection struct{}

// HealthHandler returns a simple health check response
func (hc *TestHandlerCollection) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// APIHealthHandler returns API health status
func (hc *TestHandlerCollection) APIHealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"api": "healthy"})
}

// ProxmoxHealthHandler returns Proxmox connection status
func (hc *TestHandlerCollection) ProxmoxHealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check if offline mode is enabled
	if r.Header.Get("X-Offline-Mode") == "true" || r.URL.Query().Get("offline") == "true" {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"connected": false,
			"error":     "Offline mode enabled",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"connected": true,
		"error":     "",
	})
}

// LoginHandler handles login requests
func (hc *TestHandlerCollection) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>Login Page</body></html>"))
		return
	}
	// POST login logic would go here
	w.Header().Set("Location", "/")
	w.WriteHeader(http.StatusSeeOther)
}

// AdminLoginHandler handles admin login requests
func (hc *TestHandlerCollection) AdminLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>Admin Login Page</body></html>"))
		return
	}
	// POST login logic would go here
	w.Header().Set("Location", "/admin")
	w.WriteHeader(http.StatusSeeOther)
}

// LogoutHandler handles logout requests
func (hc *TestHandlerCollection) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Location", "/login")
	w.WriteHeader(http.StatusSeeOther)
}

// ProfileHandler handles profile requests
func (hc *TestHandlerCollection) ProfileHandler(w http.ResponseWriter, r *http.Request) {
	// Redirect to login if not authenticated
	w.Header().Set("Location", "/login")
	w.WriteHeader(http.StatusSeeOther)
}

// VMCreateHandler handles VM creation requests
func (hc *TestHandlerCollection) VMCreateHandler(w http.ResponseWriter, r *http.Request) {
	// Redirect to login if not authenticated
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// HomeHandler handles the home page
func (hc *TestHandlerCollection) HomeHandler(w http.ResponseWriter, r *http.Request) {
	// Redirect to login if not authenticated
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// SearchHandler handles search requests
func (hc *TestHandlerCollection) SearchHandler(w http.ResponseWriter, r *http.Request) {
	// Redirect to login if not authenticated
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// AdminPageHandler handles admin dashboard
func (hc *TestHandlerCollection) AdminPageHandler(w http.ResponseWriter, r *http.Request) {
	// Redirect to admin login if not authenticated
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// NodesPageHandler handles admin nodes page
func (hc *TestHandlerCollection) NodesPageHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// TagsPageHandler handles admin tags page
func (hc *TestHandlerCollection) TagsPageHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// StoragePageHandler handles admin storage page
func (hc *TestHandlerCollection) StoragePageHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// ISOPageHandler handles admin ISO page
func (hc *TestHandlerCollection) ISOPageHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// VMBRPageHandler handles admin VMBR page
func (hc *TestHandlerCollection) VMBRPageHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// LimitsPageHandler handles admin limits page
func (hc *TestHandlerCollection) LimitsPageHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// UserPoolPageHandler handles admin user pool page
func (hc *TestHandlerCollection) UserPoolPageHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// AppInfoPageHandler handles admin app info page
func (hc *TestHandlerCollection) AppInfoPageHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// SettingsHandler handles settings API
func (hc *TestHandlerCollection) SettingsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"tags":   []string{"test"},
		"isos":   []string{"test.iso"},
		"vmbrs":  []string{"vmbr0"},
		"limits": map[string]interface{}{},
	})
}

// AllSettingsHandler handles all settings API
func (hc *TestHandlerCollection) AllSettingsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"tags":   []string{"test"},
		"isos":   []string{"test.iso"},
		"vmbrs":  []string{"vmbr0"},
		"limits": map[string]interface{}{},
	})
}

// AllVMBRHandler handles VMBR API
func (hc *TestHandlerCollection) AllVMBRHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode([]string{"vmbr0"})
}

// DocsHandler handles documentation
func (hc *TestHandlerCollection) DocsHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("<html><body>Documentation</body></html>"))
}

// UserDocsHandler handles user documentation
func (hc *TestHandlerCollection) UserDocsHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("<html><body>User Documentation</body></html>"))
}

// AdminDocsHandler handles admin documentation
func (hc *TestHandlerCollection) AdminDocsHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("<html><body>Admin Documentation</body></html>"))
}

// FaviconHandler handles favicon requests
func (hc *TestHandlerCollection) FaviconHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
}

// StaticHandler handles static file requests
func (hc *TestHandlerCollection) StaticHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
}
