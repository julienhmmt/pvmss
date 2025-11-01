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
	ctx := NewHandlerContext(w, r, "UpdateDiskConfigHandler")

	if !ctx.RequireAdminAuth() {
		return
	}

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	maxDiskPerVMStr := r.FormValue("max_disk_per_vm")
	if maxDiskPerVMStr == "" {
		ctx.HandleError(nil, "Bad request", http.StatusBadRequest)
		return
	}

	maxDiskPerVM := 1
	if _, err := fmt.Sscanf(maxDiskPerVMStr, "%d", &maxDiskPerVM); err != nil {
		ctx.Log.Error().Err(err).Str("value", maxDiskPerVMStr).Msg("Failed to parse max_disk_per_vm")
		ctx.HandleError(err, "Invalid number", http.StatusBadRequest)
		return
	}

	// Validate range using constants
	if maxDiskPerVM < state.MinDiskPerVM || maxDiskPerVM > state.MaxDiskPerVM {
		ctx.Log.Warn().Int("value", maxDiskPerVM).Msg("Max disk per VM out of range, clamping")
		if maxDiskPerVM < state.MinDiskPerVM {
			maxDiskPerVM = state.MinDiskPerVM
		} else {
			maxDiskPerVM = state.MaxDiskPerVM
		}
	}

	settings := h.stateManager.GetSettings()
	settings.MaxDiskPerVM = maxDiskPerVM

	if err := h.stateManager.SetSettings(settings); err != nil {
		ctx.Log.Error().Err(err).Msg("Failed to update settings")
		ctx.HandleError(err, "Failed to update settings", http.StatusInternalServerError)
		return
	}

	ctx.Log.Info().Int("max_disk_per_vm", maxDiskPerVM).Msg("Updated max disk per VM setting")
	ctx.RedirectWithSuccess("/admin/storage", "Admin.DiskConfig.Updated")
}

// RegisterRoutes registers the routes for disk management
func (h *DiskHandler) RegisterRoutes(router *httprouter.Router) {
	routeHelpers := NewAdminPageRoutes()
	routeHelpers.helpers.RegisterAdminRoute(router, "POST", "/admin/storage/update-disk-config", h.UpdateDiskConfigHandler)
}
