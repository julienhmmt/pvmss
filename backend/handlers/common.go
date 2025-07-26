package handlers

import (
	"bytes"
	"context"
	"html/template"
	"net/http"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/state"

	"github.com/julienschmidt/httprouter"
)

// InitState est une fonction de compatibilité qui ne fait plus rien
// car nous utilisons maintenant le package state
func InitState() {
	logger.Get().Info().Msg("Handlers using global state manager")
}

// RenderTemplate affiche un template avec les données fournies
// Cette fonction est exportée pour être utilisée par d'autres packages
func RenderTemplate(w http.ResponseWriter, r *http.Request, name string, data interface{}) {
	// Convertir data en map si nécessaire
	dataMap := make(map[string]interface{})
	if data != nil {
		if dm, ok := data.(map[string]interface{}); ok {
			dataMap = dm
		} else {
			dataMap["Data"] = data
		}
	}

	// Utiliser la fonction interne avec la map
	renderTemplateInternal(w, r, name, dataMap)
}

// renderTemplate est la fonction interne pour le rendu des templates
func renderTemplateInternal(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	stateManager := state.GetGlobalState()
	tmpl := stateManager.GetTemplates()

	if tmpl == nil {
		logger.Get().Error().Msg("Templates not initialized")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Initialiser les données si nécessaire
	if data == nil {
		data = make(map[string]interface{})
	}

	// Ajouter les données d'authentification
	if IsAuthenticated(r) {
		data["IsAuthenticated"] = true
		sessionManager := stateManager.GetSessionManager()
		username := sessionManager.GetString(r.Context(), "username")
		if username != "" {
			data["Username"] = username
		}
	}

	// Ajouter les données i18n et variables communes
	i18n.LocalizePage(w, r, data)
	data["CurrentPath"] = r.URL.Path
	data["IsHTTPS"] = r.TLS != nil
	data["Host"] = r.Host

	// Exécuter le template
	buf := new(bytes.Buffer)
	if err := tmpl.ExecuteTemplate(buf, name, data); err != nil {
		logger.Get().Error().
			Err(err).
			Str("template", name).
			Str("path", r.URL.Path).
			Msg("Template execution failed")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Ajouter le contenu au layout
	data["Content"] = template.HTML(buf.String())

	// Exécuter le layout
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		logger.Get().Error().
			Err(err).
			Str("template", "layout").
			Str("path", r.URL.Path).
			Msg("Layout template execution failed")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// IsAuthenticated vérifie si l'utilisateur est authentifié
// Cette fonction est exportée pour être utilisée par d'autres packages
func IsAuthenticated(r *http.Request) bool {
	stateManager := state.GetGlobalState()
	sessionManager := stateManager.GetSessionManager()
	return sessionManager.GetBool(r.Context(), "authenticated")
}

// RequireAuth est un middleware pour protéger les routes nécessitant une authentification
// Cette fonction est exportée pour être utilisée par d'autres packages
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			stateManager := state.GetGlobalState()
			sessionManager := stateManager.GetSessionManager()
			sessionManager.Put(r.Context(), "redirect_after_login", r.URL.Path)

			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Ajouter des en-têtes de sécurité
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		next.ServeHTTP(w, r)
	}
}

// IndexHandler est un handler pour la page d'accueil
// Cette fonction est exportée pour être utilisée par d'autres packages
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Initialize data with required i18n keys
	data := map[string]interface{}{
		"Title":           "PVMSS",
		"Description":     "Proxmox Virtual Machine Self-Service",
		"IsAuthenticated": IsAuthenticated(r),
	}

	RenderTemplate(w, r, "index", data)
}

// IndexRouterHandler est un handler pour la page d'accueil compatible avec httprouter
func IndexRouterHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	IndexHandler(w, r)
}

// HandlerFuncToHTTPrHandle adapts an http.HandlerFunc to a httprouter.Handle.
func HandlerFuncToHTTPrHandle(h http.HandlerFunc) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Utilisation de la clé de contexte centralisée pour les paramètres
		ctx := context.WithValue(r.Context(), ParamsKey, ps)
		h(w, r.WithContext(ctx))
	}
}
