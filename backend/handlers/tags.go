package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"pvmss/logger"
	"pvmss/state"
)

// TagsHandler gère les opérations liées aux tags
type TagsHandler struct{}

// NewTagsHandler crée une nouvelle instance de TagsHandler
func NewTagsHandler() *TagsHandler {
	return &TagsHandler{}
}

// GetTagsHandler renvoie la liste des tags disponibles
func (h *TagsHandler) GetTagsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "TagsHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Logger()

	settings := state.GetGlobalState().GetSettings()
	if settings == nil {
		log.Error().Msg("Settings not available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Settings not available",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"tags":   settings.Tags,
	})
}

// CreateTagHandler crée un nouveau tag
func (h *TagsHandler) CreateTagHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "TagsHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Logger()

	// Décoder le corps de la requête
	var requestData struct {
		Tag string `json:"tag"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		log.Warn().Err(err).Msg("Invalid request body")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Invalid request body",
		})
		return
	}

	tagName := strings.TrimSpace(requestData.Tag)
	if tagName == "" {
		log.Warn().Msg("Empty tag name provided")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Tag name cannot be empty",
		})
		return
	}

	// Vérifier si le tag existe déjà
	state := state.GetGlobalState()
	settings := state.GetSettings()
	if settings == nil {
		log.Error().Msg("Settings not available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Settings not available",
		})
		return
	}

	for _, existingTag := range settings.Tags {
		if strings.EqualFold(existingTag, tagName) {
			log.Warn().Str("tag", tagName).Msg("Tag already exists")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Tag already exists",
			})
			return
		}
	}

	// Ajouter le nouveau tag
	settings.Tags = append(settings.Tags, tagName)

	// Sauvegarder les paramètres
	if err := state.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save settings")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Failed to save settings",
		})
		return
	}

	log.Info().Str("tag", tagName).Msg("Tag created successfully")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Tag created successfully",
		"tag":     tagName,
	})
}

// DeleteTagHandler supprime un tag existant
func (h *TagsHandler) DeleteTagHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tagName := ps.ByName("tag")
	log := logger.Get().With().
		Str("handler", "TagsHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("tag", tagName).
		Logger()

	// Vérifier si le tag est le tag par défaut
	if strings.EqualFold(tagName, "pvmss") {
		log.Warn().Msg("Attempt to delete default tag 'pvmss'")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Cannot delete default tag 'pvmss'",
		})
		return
	}

	state := state.GetGlobalState()
	settings := state.GetSettings()
	if settings == nil {
		log.Error().Msg("Settings not available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Settings not available",
		})
		return
	}

	// Rechercher et supprimer le tag
	found := false
	for i, tag := range settings.Tags {
		if strings.EqualFold(tag, tagName) {
			// Supprimer le tag du slice
			settings.Tags = append(settings.Tags[:i], settings.Tags[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		log.Warn().Msg("Tag not found")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Tag not found",
		})
		return
	}

	// Sauvegarder les paramètres
	if err := state.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save settings")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Failed to save settings",
		})
		return
	}

	log.Info().Str("tag", tagName).Msg("Tag deleted successfully")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Tag deleted successfully",
	})
}

// RegisterRoutes enregistre les routes pour la gestion des tags
func (h *TagsHandler) RegisterRoutes(router *httprouter.Router) {
	log := logger.Get().With().
		Str("component", "TagsHandler").
		Str("method", "RegisterRoutes").
		Logger()

	// Définition des routes
	routes := []struct {
		method  string
		path    string
		handler httprouter.Handle
		desc    string
	}{
		{"GET", "/api/tags", h.GetTagsHandler, "Get all tags"},
		{"POST", "/api/tags", h.CreateTagHandler, "Create a new tag"},
		{"DELETE", "/api/tags/:tag", h.DeleteTagHandler, "Delete a tag"},
	}

	// Enregistrement des routes
	for _, route := range routes {
		router.Handle(route.method, route.path, route.handler)
		log.Debug().
			Str("method", route.method).
			Str("path", route.path).
			Str("description", route.desc).
			Msg("Route enregistrée pour la gestion des tags")
	}

	log.Info().
		Int("routes_count", len(routes)).
		Msg("Routes de gestion des tags enregistrées avec succès")
}

// EnsureDefaultTag s'assure que le tag par défaut existe dans les paramètres
func EnsureDefaultTag() error {
	state := state.GetGlobalState()
	settings := state.GetSettings()
	if settings == nil {
		return nil
	}

	// Vérifier si le tag par défaut existe déjà
	defaultTag := "pvmss"
	for _, tag := range settings.Tags {
		if tag == defaultTag {
			return nil
		}
	}

	// Ajouter le tag par défaut s'il n'existe pas
	settings.Tags = append(settings.Tags, defaultTag)
	return state.SetSettings(settings)
}
