package handlers

import (
	"bytes"
	"context"
	"html/template"
	"net/http"
	"net/url"

	"pvmss/i18n"
	"pvmss/logger"
	customMiddleware "pvmss/middleware"
	securityMw "pvmss/security/middleware"
	"pvmss/state"

	"github.com/julienschmidt/httprouter"
)

// ISOInfo représente les informations détaillées sur une image ISO.
type ISOInfo struct {
	VolID   string `json:"volid"`
	Format  string `json:"format"`
	Size    int64  `json:"size"`
	Node    string `json:"node,omitempty"`
	Storage string `json:"storage,omitempty"`
	Enabled bool   `json:"enabled"`
}

// contextKey is used for context keys to avoid collisions between packages using context
type contextKey string

// ParamsKey is the key used to store httprouter.Params in the request context
const ParamsKey contextKey = "params"

// InitState est une fonction de compatibilité qui ne fait plus rien
// car nous utilisons maintenant le package state
func InitState() {
	logger.Get().Info().Msg("Handlers using global state manager")
}

// RenderTemplate affiche un template avec les données fournies
// Cette fonction est exportée pour être utilisée par d'autres packages
func RenderTemplate(w http.ResponseWriter, r *http.Request, name string, data interface{}) {
	log := logger.Get().With().
		Str("handler", "RenderTemplate").
		Str("template", name).
		Str("path", r.URL.Path).
		Str("method", r.Method).
		Logger()

	log.Debug().Msg("Début du rendu du template")

	// Convertir data en map si nécessaire
	dataMap := make(map[string]interface{})
	if data != nil {
		if dm, ok := data.(map[string]interface{}); ok {
			dataMap = dm
			log.Debug().Int("data_map_size", len(dm)).Msg("Données fournies sous forme de map")
		} else {
			dataMap["Data"] = data
			log.Debug().Type("data_type", data).Msg("Données fournies sous forme de structure, conversion en map")
		}
	} else {
		log.Debug().Msg("Aucune donnée fournie pour le rendu du template")
	}

	// Utiliser la fonction interne avec la map
	renderTemplateInternal(w, r, name, dataMap)

	log.Info().
		Str("template", name).
		Msg("Rendu du template terminé avec succès")
}

// renderTemplate est la fonction interne pour le rendu des templates
func renderTemplateInternal(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	log := logger.Get().With().
		Str("handler", "renderTemplateInternal").
		Str("template", name).
		Str("path", r.URL.Path).
		Logger()

	log.Debug().Msg("Début du rendu interne du template")

	stateManager := state.GetGlobalState()
	tmpl := stateManager.GetTemplates()

	if tmpl == nil {
		errMsg := "Les templates ne sont pas initialisés"
		log.Error().Msg(errMsg)
		http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
		return
	}

	// Initialiser les données si nécessaire
	if data == nil {
		log.Debug().Msg("Initialisation d'une nouvelle map de données vide")
		data = make(map[string]interface{})
	} else {
		log.Debug().Int("data_size", len(data)).Msg("Données fournies pour le rendu")
	}

	// Récupérer les données de template du contexte si elles existent
	if ctxData, ok := r.Context().Value(customMiddleware.TemplateDataKey).(map[string]interface{}); ok {
		log.Debug().Int("context_data_size", len(ctxData)).Msg("Données de contexte récupérées")
		// Fusionner les données du contexte avec les données fournies (les données fournies ont la priorité)
		for k, v := range ctxData {
			if _, exists := data[k]; !exists {
				data[k] = v
			}
		}
	}

	// Add authentication data
	if IsAuthenticated(r) {
		log.Debug().Msg("Authenticated user detected, adding session data")
		data["IsAuthenticated"] = true
	} else {
		log.Debug().Msg("No authenticated user detected")
	}

	// Add CSRF token to template data
	csrfToken := securityMw.GetCSRFToken(r)
	if csrfToken != "" {
		data["CSRFToken"] = csrfToken
		log.Debug().Msg("CSRF token added to template data")
	} else {
		log.Warn().Msg("No CSRF token found in request context")
	}

	// Add i18n data and common variables
	log.Debug().Msg("Applying i18n data and common variables")
	i18n.LocalizePage(w, r, data)
	data["CurrentPath"] = r.URL.Path
	data["IsHTTPS"] = r.TLS != nil
	data["Host"] = r.Host

	log.Debug().
		Str("current_path", r.URL.Path).
		Bool("is_https", r.TLS != nil).
		Str("host", r.Host).
		Msg("Context variables added")

	// Execute the template
	buf := new(bytes.Buffer)
	log.Debug().Msg("Executing main template")

	if err := tmpl.ExecuteTemplate(buf, name, data); err != nil {
		log.Error().
			Err(err).
			Str("template", name).
			Msg("Failed to execute template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	log.Debug().
		Int("content_length", buf.Len()).
		Msg("Main template executed successfully")

	// Add content to layout
	content := buf.String()
	data["Content"] = template.HTML(content)

	log.Debug().
		Int("content_length", len(content)).
		Msg("Template content prepared for layout")

	// Execute the layout
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	log.Debug().Msg("Executing layout template")

	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		log.Error().
			Err(err).
			Str("template", "layout").
			Msg("Failed to execute layout template")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("template", name).
		Int("response_size", len(content)).
		Msg("Page rendering completed successfully")
}

// IsAuthenticated checks if the user is authenticated
// This function is exported for use by other packages
func IsAuthenticated(r *http.Request) bool {
	log := logger.Get().With().
		Str("handler", "IsAuthenticated").
		Str("path", r.URL.Path).
		Str("method", r.Method).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	stateManager := state.GetGlobalState()
	sessionManager := stateManager.GetSessionManager()

	// Check if the session contains the authentication flag
	authenticated, ok := sessionManager.Get(r.Context(), "authenticated").(bool)
	if !ok || !authenticated {
		log.Debug().
			Bool("authenticated", false).
			Msg("Access denied: user not authenticated")
		return false
	}

	// Additional security check for username
	username, ok := sessionManager.Get(r.Context(), "username").(string)
	if !ok || username == "" {
		log.Warn().
			Msg("Corrupted session: missing username")
		return false
	}

	// Verify CSRF token for state-changing requests
	if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete || r.Method == http.MethodPatch {
		csrfToken := r.Header.Get("X-CSRF-Token")
		if csrfToken == "" {
			// Try to get from form if not in header
			csrfToken = r.FormValue("csrf_token")
		}

		sessionToken, _ := sessionManager.Get(r.Context(), "csrf_token").(string)
		if csrfToken == "" || csrfToken != sessionToken {
			log.Warn().
				Str("session_token", sessionToken).
				Str("provided_token", csrfToken).
				Msg("CSRF token validation failed")
			return false
		}
	}

	log.Debug().
		Str("username", username).
		Str("session_id", sessionManager.Token(r.Context())).
		Msg("Access granted: user authenticated")

	return true
}

// RequireAuth is a middleware that enforces authentication for protected routes
// This function is exported for use by other packages
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logger.Get().With().
			Str("handler", "RequireAuth").
			Str("path", r.URL.Path).
			Str("method", r.Method).
			Str("remote_addr", r.RemoteAddr).
			Logger()

		if !IsAuthenticated(r) {
			log.Info().Msg("Authentication required, redirecting to login")

			// Store the original URL for redirection after login
			returnURL := r.URL.Path
			if r.URL.RawQuery != "" {
				returnURL = returnURL + "?" + r.URL.RawQuery
			}

			// Set cache control headers to prevent caching of protected pages
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")

			// Redirect to login page with return URL
			http.Redirect(w, r, "/login?return="+url.QueryEscape(returnURL), http.StatusSeeOther)
			return
		}

		// Add security headers for authenticated routes
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Add CSRF token to the response headers for AJAX requests
		if r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
			stateManager := state.GetGlobalState()
			sessionManager := stateManager.GetSessionManager()
			if csrfToken, ok := sessionManager.Get(r.Context(), "csrf_token").(string); ok && csrfToken != "" {
				w.Header().Set("X-CSRF-Token", csrfToken)
			}
		}
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		next.ServeHTTP(w, r)
	}
}

// IndexHandler est un handler pour la page d'accueil
// Cette fonction est exportée pour être utilisée par d'autres packages
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.Get().With().
		Str("handler", "IndexHandler").
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Msg("Traitement de la requête pour la page d'accueil")

	// Si ce n'est pas la racine, on renvoie une 404
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Préparer les données pour le template
	data := map[string]interface{}{
		"Title": "PVMSS",
		"Lang":  i18n.GetLanguage(r), // Ajouter la langue détectée
	}

	// Ajouter les données de traduction en fonction de la langue
	i18n.LocalizePage(w, r, data)

	log.Debug().Msg("Rendu du template index")
	renderTemplateInternal(w, r, "index", data) // Utiliser le nom du template au lieu du nom de fichier

	log.Info().Msg("Page d'accueil affichée avec succès")
}

// IndexRouterHandler est un handler pour la page d'accueil compatible avec httprouter
func IndexRouterHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	logger.Get().Debug().
		Str("handler", "IndexRouterHandler").
		Str("path", r.URL.Path).
		Msg("Appel du gestionnaire d'index via routeur HTTP")

	// Délègue le traitement au gestionnaire principal
	IndexHandler(w, r)
}

// HandlerFuncToHTTPrHandle adapte un http.HandlerFunc à une fonction httprouter.Handle.
// Cette fonction permet d'utiliser des handlers standards avec le routeur httprouter.
func HandlerFuncToHTTPrHandle(h http.HandlerFunc) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Créer un logger pour cette requête
		log := logger.Get().With().
			Str("adapter", "HandlerFuncToHTTPrHandle").
			Str("path", r.URL.Path).
			Str("method", r.Method).
			Int("params_count", len(ps)).
			Logger()

		log.Debug().Msg("Adaptation du gestionnaire HTTP standard pour httprouter")

		// Ajouter les paramètres de route au contexte de la requête
		ctx := context.WithValue(r.Context(), ParamsKey, ps)

		// Appeler le gestionnaire d'origine avec le nouveau contexte
		h(w, r.WithContext(ctx))

		log.Debug().Msg("Traitement du gestionnaire HTTP terminé")
	}
}
