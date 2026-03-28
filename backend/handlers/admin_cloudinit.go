package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"

	"pvmss/state"
)

// CloudInitHandler handles cloud-init template management.
type CloudInitHandler struct {
	stateManager state.StateManager
}

// MakeCloudInitHandler creates a new CloudInitHandler.
func MakeCloudInitHandler(stateManager state.StateManager) *CloudInitHandler {
	return &CloudInitHandler{stateManager: stateManager}
}

// RegisterRoutes registers cloud-init routes.
func (h *CloudInitHandler) RegisterRoutes(router *httprouter.Router) {
	// Public API for authenticated users to view template content (used on VM create page)
	router.GET("/api/cloudinit/template/:id", RequireAuthHandle(h.GetTemplateHandler))
}

// GetTemplateHandler returns a single template with its YAML content.
func (h *CloudInitHandler) GetTemplateHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := CreateHandlerLogger("GetTemplateHandler", r)

	id := ps.ByName("id")
	if id == "" {
		if ctxParams, ok := r.Context().Value(ParamsKey).(httprouter.Params); ok {
			id = ctxParams.ByName("id")
		}
	}
	if id == "" {
		http.Error(w, "Template ID is required", http.StatusBadRequest)
		return
	}

	settings := h.stateManager.GetSettings()

	var template *state.CloudInitTemplate
	for i := range settings.CloudInitTemplates {
		if settings.CloudInitTemplates[i].ID == id {
			template = &settings.CloudInitTemplates[i]
			break
		}
	}

	if template == nil {
		http.Error(w, "Template not found", http.StatusNotFound)
		return
	}

	content := template.YAMLContent
	source := "none"
	if content != "" {
		source = "local"
	}

	response := map[string]any{
		"id":          template.ID,
		"name":        template.Name,
		"description": template.Description,
		"storage":     template.Storage,
		"content":     content,
		"source":      source,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
