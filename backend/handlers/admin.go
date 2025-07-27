package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
	"pvmss/logger"
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
	log := logger.Get().With().
		Str("handler", "AdminHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Msg("Tentative d'accès à la page d'administration")

	// Vérifier les autorisations (utilise la session)
	if !IsAuthenticated(r) {
		errMsg := "Accès refusé: utilisateur non authentifié"
		log.Warn().
			Str("status", "forbidden").
			Msg(errMsg)
		http.Error(w, errMsg, http.StatusForbidden)
		return
	}

	log.Debug().Msg("Préparation des données pour la page d'administration")

	// Récupérer les données nécessaires (mock temporaire)
	settings := map[string]interface{}{
		"VMCount":   42,
		"UserCount": 10,
	}

	log.Debug().
		Interface("settings", settings).
		Msg("Données de la page d'administration chargées")

	// Préparer les données pour le template
	data := map[string]interface{}{
		"Settings": settings,
	}

	// Ajouter les traductions
	i18n.LocalizePage(w, r, data)
	data["Title"] = data["Admin.Title"]

	log.Debug().Msg("Rendu du template d'administration")
	renderTemplateInternal(w, r, "admin", data)

	log.Info().Msg("Page d'administration affichée avec succès")
}

// TagsAPIHandler gère l'API des tags
func (h *AdminHandler) TagsAPIHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "AdminHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Msg("Traitement de la requête API des tags")

	settings := state.GetGlobalState().GetSettings()
	tags := []string{}
	if settings != nil {
		tags = settings.Tags
	}

	log.Debug().
		Int("tags_count", len(tags)).
		Msg("Tags récupérés avec succès")

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{"tags": tags}); err != nil {
		log.Error().
			Err(err).
			Msg("Échec de l'encodage de la réponse JSON des tags")
		return
	}

	log.Debug().Msg("Réponse API des tags envoyée avec succès")
}

// RegisterRoutes enregistre les routes d'administration
func (h *AdminHandler) RegisterRoutes(router *httprouter.Router) {
	log := logger.Get().With().
		Str("component", "AdminHandler").
		Str("method", "RegisterRoutes").
		Logger()

	// Définition des routes
	routes := []struct {
		method  string
		path    string
		handler httprouter.Handle
		desc    string
	}{
		{"GET", "/admin", h.AdminPageHandler, "Page d'administration"},
		{"GET", "/api/tags", h.TagsAPIHandler, "API des tags"},
	}

	// Enregistrement des routes
	for _, route := range routes {
		router.Handle(route.method, route.path, route.handler)
		log.Debug().
			Str("method", route.method).
			Str("path", route.path).
			Str("description", route.desc).
			Msg("Route d'administration enregistrée")
	}

	log.Info().
		Int("routes_count", len(routes)).
		Msg("Routes d'administration enregistrées avec succès")
}
