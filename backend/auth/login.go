package auth

import (
	"bytes"
	"html/template"
	"net/http"
	"time"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/state"
	"pvmss/templates"
)

// LoginHandler handles user authentication
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	// Handle GET request - show login form
	if r.Method == http.MethodGet {
		handleLoginGet(w, r)
		return
	}

	// Handle POST request - process login
	if r.Method == http.MethodPost {
		handleLoginPost(w, r)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func handleLoginGet(w http.ResponseWriter, r *http.Request) {
	if _, err := r.Cookie("session"); err == nil {
		returnURL := r.URL.Query().Get("return")
		if returnURL == "" {
			returnURL = "/"
		}
		http.Redirect(w, r, returnURL, http.StatusSeeOther)
		return
	}

	// Create data map for template
	data := make(map[string]interface{})

	// Use the main template rendering system with i18n
	renderTemplate(w, r, "login.html", data)
}

func handleLoginPost(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	// Validate input
	if username == "" || password == "" {
		handleLoginError(w, r, "Username and password are required")
		return
	}

	// Check credentials (TODO: Implement proper credential validation)
	// For now, just check if username and password are not empty
	if username == "" || password == "" {
		// Log failed login attempt
		logger.Get().Warn().
			Str("username", username).
			Str("ip", r.RemoteAddr).
			Msg("Failed login attempt")

		handleLoginError(w, r, "Invalid username or password")
		return
	}

	// Create session (simplified for now)
	sessionToken := "session_" + username + "_" + time.Now().Format(time.RFC3339)
	expiresAt := time.Now().Add(24 * time.Hour)

	// Set session cookie
	// In production, you should set Secure: true and configure proper SameSite policies
	secure := false // Set to true in production with HTTPS
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionToken,
		Expires:  expiresAt,
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode, // More flexible than StrictMode for local development
		Secure:   secure,
	})

	// Redirect to the originally requested URL or search page
	returnURL := r.URL.Query().Get("return")
	if returnURL == "" {
		returnURL = "/search"
	}
	http.Redirect(w, r, returnURL, http.StatusSeeOther)
}

func handleLoginError(w http.ResponseWriter, r *http.Request, message string) {
	// Create data map for template
	data := make(map[string]interface{})
	data["Error"] = message

	// Use the main template rendering system with i18n
	renderTemplate(w, r, "login.html", data)
}

// renderTemplate renders templates with i18n support and layout
// This mirrors the main renderTemplate function to ensure consistency
func renderTemplate(w http.ResponseWriter, r *http.Request, name string, data interface{}) {
	stateManager := state.GetGlobalState()
	tmpl := stateManager.GetTemplates()
	if tmpl == nil {
		logger.Get().Error().Msg("Templates not initialized")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Create a new template with the same name and parse tree
	t := template.New(name).Funcs(templates.GetFuncMap(r))

	// Clone the template's parse tree
	var err error
	tmpl, err = tmpl.Clone()
	if err != nil {
		logger.Get().Error().Err(err).Str("template", name).Msg("Failed to clone template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get the template definition
	t = template.Must(tmpl.Clone()).Funcs(templates.GetFuncMap(r))

	var dataMap map[string]interface{}
	if data == nil {
		dataMap = make(map[string]interface{})
	} else if dm, ok := data.(map[string]interface{}); ok {
		dataMap = dm
	} else {
		dataMap = map[string]interface{}{"Data": data}
	}

	// Add i18n data using the i18n package
	i18n.LocalizePage(w, r, dataMap)

	// Add request information for templates
	dataMap["CurrentPath"] = r.URL.Path
	dataMap["IsHTTPS"] = r.TLS != nil
	dataMap["Host"] = r.Host

	// Set content type to HTML
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Execute template with error handling
	buffer := new(bytes.Buffer)
	err = t.ExecuteTemplate(buffer, name, dataMap)
	if err != nil {
		logger.Get().Error().
			Err(err).
			Str("template", name).
			Str("path", r.URL.Path).
			Msg("Template execution failed")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Write the buffered output to the response
	if _, err := buffer.WriteTo(w); err != nil {
		logger.Get().Error().
			Err(err).
			Str("template", name).
			Msg("Failed to write template response")
	}
}
