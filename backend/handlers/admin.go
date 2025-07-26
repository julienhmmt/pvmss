package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
	"pvmss/state"
)

// AdminHandler gère les routes d'administration
type AdminHandler struct{}

// NewAdminHandler crée une nouvelle instance de AdminHandler
func NewAdminHandler() *AdminHandler {
	return &AdminHandler{}
}

// AdminPageHandler gère la page d'administration
func (h *AdminHandler) AdminPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Vérifier les autorisations (utilise la session)
	if !IsAuthenticated(r) {
		http.Error(w, "Accès refusé", http.StatusForbidden)
		return
	}

	// Récupérer les données nécessaires (mock temporaire)
	settings := map[string]interface{}{
		"VMCount":   42,
		"UserCount": 10,
	}

	// Préparer les données pour le template
	data := map[string]interface{}{
		"Settings": settings,
	}

	// Ajouter les traductions
	i18n.LocalizePage(w, r, data)
	data["Title"] = data["Admin.Title"]

	renderTemplateInternal(w, r, "admin", data)
}

// TagsAPIHandler gère l'API des tags
func (h *AdminHandler) TagsAPIHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	settings := state.GetGlobalState().GetSettings()
	tags := []string{}
	if settings != nil {
		tags = settings.Tags
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"tags": tags})
}

// RegisterRoutes enregistre les routes d'administration
func (h *AdminHandler) RegisterRoutes(router *httprouter.Router) {
	// Protection des routes admin par authentification
	router.GET("/admin", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.AdminPageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	// API routes protégées par authentification
	router.GET("/api/tags", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.TagsAPIHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
}
