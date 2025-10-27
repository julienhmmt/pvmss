package handlers

import (
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"pvmss/state"
)

// DiskHandler handles disk-related operations
type DiskHandler struct {
	stateManager state.StateManager
}

// NewDiskHandler creates a new instance of DiskHandler
func NewDiskHandler(sm state.StateManager) *DiskHandler {
	return &DiskHandler{stateManager: sm}
}

// UpdateDiskConfigHandler updates the maximum number of disks per VM
func (h *DiskHandler) UpdateDiskConfigHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("UpdateDiskConfigHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	maxDiskPerVMStr := r.FormValue("max_disk_per_vm")
	if maxDiskPerVMStr == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	maxDiskPerVM := 1
	if _, err := fmt.Sscanf(maxDiskPerVMStr, "%d", &maxDiskPerVM); err != nil {
		log.Error().Err(err).Str("value", maxDiskPerVMStr).Msg("Failed to parse max_disk_per_vm")
		http.Error(w, "Invalid number", http.StatusBadRequest)
		return
	}

	// Validate range (1-16 for VirtIO Block max)
	if maxDiskPerVM < 1 || maxDiskPerVM > 16 {
		log.Warn().Int("value", maxDiskPerVM).Msg("Max disk per VM out of range, clamping")
		if maxDiskPerVM < 1 {
			maxDiskPerVM = 1
		} else {
			maxDiskPerVM = 16
		}
	}

	settings := h.stateManager.GetSettings()
	settings.MaxDiskPerVM = maxDiskPerVM

	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to update settings")
		http.Error(w, "Failed to update settings", http.StatusInternalServerError)
		return
	}

	log.Info().Int("max_disk_per_vm", maxDiskPerVM).Msg("Updated max disk per VM setting")
	redirectURL := "/admin/storage?success=1&action=update_disk_config"
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// RegisterRoutes registers the routes for disk management
func (h *DiskHandler) RegisterRoutes(router *httprouter.Router) {
	routeHelpers := NewAdminPageRoutes()
	routeHelpers.helpers.RegisterAdminRoute(router, "POST", "/admin/storage/update-disk-config", h.UpdateDiskConfigHandler)
}
