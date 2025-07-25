// Package handlers provides HTTP request handlers for the PVMSS application
package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"pvmss/logger"
	"pvmss/security"
	"pvmss/state"
)

// IndexHandler handles the root path
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	lang := getLanguage(r)
	data := getI18nData(lang)

	// Create a buffer to capture the template output
	var buf bytes.Buffer

	tmpl := state.GetTemplates()
	if tmpl == nil {
		logger.Get().Error().Msg("Templates not initialized")
		http.Error(w, "Templates not initialized", http.StatusInternalServerError)
		return
	}

	// Execute the index template
	if err := tmpl.ExecuteTemplate(&buf, "index.html", data); err != nil {
		logger.Get().Error().Err(err).Msg("Error rendering index template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Add the rendered template to the data map as SafeContent (expected by layout)
	data["SafeContent"] = template.HTML(buf.String())

	// Execute the layout template with the content
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		logger.Get().Error().Err(err).Msg("Error executing layout template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// SearchHandler handles VM search requests
func SearchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		lang := getLanguage(r)
		data := getI18nData(lang)
		
		// Add CSRF token for the form
		data["CSRFToken"] = security.GenerateCSRFToken(r)

		// Create a buffer to capture the template output
		var buf bytes.Buffer

		tmpl := state.GetTemplates()
		if tmpl == nil {
			logger.Get().Error().Msg("Templates not initialized")
			http.Error(w, "Templates not initialized", http.StatusInternalServerError)
			return
		}

		// Execute the search template
		if err := tmpl.ExecuteTemplate(&buf, "search.html", data); err != nil {
			logger.Get().Error().Err(err).Msg("Error rendering search template")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Add the rendered template to the data map as SafeContent (expected by layout)
		data["SafeContent"] = template.HTML(buf.String())

		// Execute the layout template with the content
		if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
			logger.Get().Error().Err(err).Msg("Error executing layout template")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	if r.Method == "POST" {
		// Validate CSRF token
		if !security.ValidateCSRFToken(r) {
			logger.Get().Warn().
				Str("ip", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Msg("CSRF token validation failed for search")
			http.Error(w, "CSRF token validation failed", http.StatusBadRequest)
			return
		}

		// Get search parameters with validation
		vmid := security.ValidateInput(r.FormValue("vmid"), 10)
		name := security.ValidateInput(r.FormValue("name"), 100)
		node := security.ValidateInput(r.FormValue("node"), 50)

		lang := getLanguage(r)
		data := getI18nData(lang)
		data["CSRFToken"] = security.GenerateCSRFToken(r)

		// Perform search
		results, err := performVMSearch(vmid, name, node)
		if err != nil {
			logger.Get().Error().Err(err).
				Str("vmid", vmid).
				Str("name", name).
				Str("node", node).
				Msg("Error performing VM search")
			
			data["Error"] = fmt.Sprintf("Search error: %v", err)
		} else {
			data["Results"] = results
			data["SearchQuery"] = map[string]string{
				"vmid": vmid,
				"name": name,
				"node": node,
			}
		}

		// Create a buffer to capture the template output
		var buf bytes.Buffer

		tmpl := state.GetTemplates()
		if tmpl == nil {
			logger.Get().Error().Msg("Templates not initialized")
			http.Error(w, "Templates not initialized", http.StatusInternalServerError)
			return
		}

		// Execute the search template
		if err := tmpl.ExecuteTemplate(&buf, "search.html", data); err != nil {
			logger.Get().Error().Err(err).Msg("Error rendering search template with results")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Add the rendered template to the data map as SafeContent (expected by layout)
		data["SafeContent"] = template.HTML(buf.String())

		// Execute the layout template with the content
		if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
			logger.Get().Error().Err(err).Msg("Error executing layout template with results")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// LoginHandler handles user authentication
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		lang := getLanguage(r)
		data := getI18nData(lang)
		data["CSRFToken"] = security.GenerateCSRFToken(r)

		// Create a buffer to capture the template output
		var buf bytes.Buffer

		tmpl := state.GetTemplates()
		if tmpl == nil {
			logger.Get().Error().Msg("Templates not initialized")
			http.Error(w, "Templates not initialized", http.StatusInternalServerError)
			return
		}

		// Execute the login template
		if err := tmpl.ExecuteTemplate(&buf, "login.html", data); err != nil {
			logger.Get().Error().Err(err).Msg("Error rendering login template")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Add the rendered template to the data map as SafeContent (expected by layout)
		data["SafeContent"] = template.HTML(buf.String())

		// Execute the layout template with the content
		if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
			logger.Get().Error().Err(err).Msg("Error executing layout template")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	if r.Method == "POST" {
		// Validate CSRF token
		if !security.ValidateCSRFToken(r) {
			logger.Get().Warn().
				Str("ip", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Msg("CSRF token validation failed for login")
			http.Error(w, "CSRF token validation failed", http.StatusBadRequest)
			return
		}

		// Get form values with validation
		username := security.ValidateInput(r.FormValue("username"), 100)
		password := security.ValidateInput(r.FormValue("password"), 200)

		// Authenticate user
		if authenticateUser(username, password) {
			// Set session
			sm := state.GetSessionManager()
			sm.Put(r.Context(), "authenticated", true)
			sm.Put(r.Context(), "username", username)
			sm.Put(r.Context(), "login_time", time.Now().Unix())

			logger.Get().Info().
				Str("username", username).
				Str("ip", r.RemoteAddr).
				Msg("User logged in successfully")

			http.Redirect(w, r, "/admin", http.StatusSeeOther)
		} else {
			logger.Get().Warn().
				Str("username", username).
				Str("ip", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Msg("Failed login attempt")

			lang := getLanguage(r)
			data := getI18nData(lang)
			data["CSRFToken"] = security.GenerateCSRFToken(r)
			data["Error"] = "Invalid username or password"

			// Create a buffer to capture the template output
			var buf bytes.Buffer

			tmpl := state.GetTemplates()
			if tmpl == nil {
				logger.Get().Error().Msg("Templates not initialized")
				http.Error(w, "Templates not initialized", http.StatusInternalServerError)
				return
			}

			// Execute the login template
			if err := tmpl.ExecuteTemplate(&buf, "login.html", data); err != nil {
				logger.Get().Error().Err(err).Msg("Error rendering login template with error")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			// Add the rendered template to the data map as SafeContent (expected by layout)
			data["SafeContent"] = template.HTML(buf.String())

			// Execute the layout template with the content
			if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
				logger.Get().Error().Err(err).Msg("Error executing layout template with error")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}
	}
}

// LogoutHandler handles user logout
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	sm := state.GetSessionManager()
	
	username := sm.GetString(r.Context(), "username")
	
	// Destroy session
	sm.Destroy(r.Context())

	logger.Get().Info().
		Str("username", username).
		Str("ip", r.RemoteAddr).
		Msg("User logged out")

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// AdminHandler handles the admin dashboard
func AdminHandler(w http.ResponseWriter, r *http.Request) {
	lang := getLanguage(r)
	data := getI18nData(lang)

	// Get settings from cache or load from file
	settings := state.GetAppSettings()
	if settings == nil {
		logger.Get().Error().Msg("Settings not available")
		http.Error(w, "Settings not available", http.StatusInternalServerError)
		return
	}

	data["Settings"] = settings

	// Create a buffer to capture the template output
	var buf bytes.Buffer

	tmpl := state.GetTemplates()
	if tmpl == nil {
		logger.Get().Error().Msg("Templates not initialized")
		http.Error(w, "Templates not initialized", http.StatusInternalServerError)
		return
	}

	// Execute the admin template
	if err := tmpl.ExecuteTemplate(&buf, "admin.html", data); err != nil {
		logger.Get().Error().Err(err).Msg("Error rendering admin template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Add the rendered template to the data map as SafeContent (expected by layout)
	data["SafeContent"] = template.HTML(buf.String())

	// Execute the layout template with the content
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		logger.Get().Error().Err(err).Msg("Error executing layout template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HealthHandler provides health check endpoint
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// APIVmStatusHandler provides VM status API endpoint
func APIVmStatusHandler(w http.ResponseWriter, r *http.Request) {
	vmidStr := r.URL.Query().Get("vmid")
	if vmidStr == "" {
		http.Error(w, "VMID is required", http.StatusBadRequest)
		return
	}

	vmid, err := strconv.Atoi(vmidStr)
	if err != nil {
		http.Error(w, "Invalid VMID", http.StatusBadRequest)
		return
	}

	client := state.GetProxmoxClient()
	if client == nil {
		http.Error(w, "Proxmox client not available", http.StatusInternalServerError)
		return
	}

	// Get VM status - this is a simplified implementation
	// In a real implementation, you'd need to find which node the VM is on
	status := map[string]interface{}{
		"vmid":   vmid,
		"status": "unknown",
		"name":   "VM-" + vmidStr,
		"node":   "unknown",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}


