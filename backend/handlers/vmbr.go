package handlers

import (
	"net/http"

	"pvmss/logger"
	"pvmss/state"

	"github.com/julienschmidt/httprouter"
)

// VMBRHandler handles VMBR-related operations.
type VMBRHandler struct {
	stateManager state.StateManager
}

// NewVMBRHandler creates a new instance of VMBRHandler.
func NewVMBRHandler(sm state.StateManager) *VMBRHandler {
	return &VMBRHandler{stateManager: sm}
}

// VMBRPageHandler renders the VMBR management page.
func (h *VMBRHandler) VMBRPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().Str("handler", "VMBRPageHandler").Logger()

	// Get the global state
	gs := h.stateManager
	client := gs.GetProxmoxClient()
	if client == nil {
		// Proceed gracefully in offline/read-only mode like AdminPageHandler
		log.Warn().Msg("Proxmox client not available; rendering page with empty VMBR list")
	}

	// Collect all VMBRs using common helper
	allVMBRs, err := collectAllVMBRs(h.stateManager)
	if err != nil {
		log.Warn().Err(err).Msg("collectAllVMBRs returned an error; continuing")
	}
	log.Info().Int("vmbr_total", len(allVMBRs)).Msg("Total VMBRs prepared for template")

	// Get current settings to check which VMBRs are enabled
	settings := gs.GetSettings()
	enabledVMBRs := make(map[string]bool)
	for _, vmbr := range settings.VMBRs {
		enabledVMBRs[vmbr] = true
	}

	// Prepare template data
	// Map to template shape used previously: name field instead of iface
	vmbrsForTemplate := make([]map[string]string, 0, len(allVMBRs))
	for _, v := range allVMBRs {
		vmbrsForTemplate = append(vmbrsForTemplate, map[string]string{
			"node":        v["node"],
			"name":        v["iface"],
			"description": v["description"],
		})
	}

	templateData := map[string]interface{}{
		"VMBRs":        vmbrsForTemplate,
		"EnabledVMBRs": enabledVMBRs,
	}
	if err != nil {
		templateData["Error"] = err.Error()
	}

	// Render the template
	renderTemplateInternal(w, r, "vmbr", templateData)
}

// UpdateVMBRHandler handles updating enabled VMBRs.
func (h *VMBRHandler) UpdateVMBRHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().Str("handler", "UpdateVMBRHandler").Logger()

	if err := r.ParseForm(); err != nil {
		log.Warn().Err(err).Msg("Error parsing form data")
		http.Error(w, "Invalid form data.", http.StatusBadRequest)
		return
	}

	// Get the list of enabled VMBRs from the form
	enabledVMBRs := r.Form["enabled_vmbrs"]

	// Update settings
	gs := h.stateManager
	settings := gs.GetSettings()
	settings.VMBRs = enabledVMBRs

	if err := gs.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to update settings")
		http.Error(w, "Failed to update settings", http.StatusInternalServerError)
		return
	}

	// Redirect back to the VMBR page
	http.Redirect(w, r, "/admin/vmbr", http.StatusSeeOther)
}

// RegisterRoutes registers the routes for VMBR management.
func (h *VMBRHandler) RegisterRoutes(router *httprouter.Router) {
	router.GET("/admin/vmbr", h.VMBRPageHandler)
	router.POST("/admin/vmbr/update", h.UpdateVMBRHandler)
}
