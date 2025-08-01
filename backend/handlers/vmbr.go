package handlers

import (
	"net/http"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"

	"github.com/julienschmidt/httprouter"
)

// VMBRHandler handles VMBR-related operations.
type VMBRHandler struct{}

// NewVMBRHandler creates a new instance of VMBRHandler.
func NewVMBRHandler() *VMBRHandler {
	return &VMBRHandler{}
}

// VMBRPageHandler renders the VMBR management page.
func (h *VMBRHandler) VMBRPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().Str("handler", "VMBRPageHandler").Logger()

	// Get the global state
	gs := state.GetGlobalState()
	client := gs.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not available")
		http.Error(w, "Proxmox client not available", http.StatusInternalServerError)
		return
	}

	// Get all nodes
	nodes, err := proxmox.GetNodeNames(client)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get nodes")
		http.Error(w, "Failed to get nodes", http.StatusInternalServerError)
		return
	}

	// Get all VMBRs from all nodes
	allVMBRs := make([]map[string]string, 0)
	for _, node := range nodes {
		vmbrs, err := proxmox.GetVMBRs(client, node)
		if err != nil {
			log.Warn().Err(err).Str("node", node).Msg("Failed to get VMBRs for node")
			continue
		}

		if data, ok := vmbrs["data"].([]interface{}); ok {
			for _, iface := range data {
				if ifaceMap, ok := iface.(map[string]interface{}); ok {
					if ifaceMap["type"] == "bridge" {
						description := ""
						if desc, ok := ifaceMap["comments"].(string); ok {
							description = desc
						}

						allVMBRs = append(allVMBRs, map[string]string{
							"node":        node,
							"name":        ifaceMap["iface"].(string),
							"description": description,
						})
					}
				}
			}
		}
	}

	// Get current settings to check which VMBRs are enabled
	settings := gs.GetSettings()
	enabledVMBRs := make(map[string]bool)
	for _, vmbr := range settings.VMBRs {
		enabledVMBRs[vmbr] = true
	}

	// Prepare template data
	templateData := map[string]interface{}{
		"VMBRs":        allVMBRs,
		"EnabledVMBRs": enabledVMBRs,
	}

	// Render the template
	renderTemplateInternal(w, r, "vmbr.html", templateData)
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
	gs := state.GetGlobalState()
	settings := gs.GetSettings()
	settings.VMBRs = enabledVMBRs

	if err := state.WriteSettings(settings); err != nil {
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
