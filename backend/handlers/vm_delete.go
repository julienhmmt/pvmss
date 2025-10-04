package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
	"pvmss/proxmox"
	"pvmss/security"
)

// VMDeleteConfirmHandler shows a confirmation page before deleting a VM
func (h *VMHandler) VMDeleteConfirmHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := CreateHandlerLogger("VMDeleteConfirmHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodGet) {
		return
	}

	vmid := ps.ByName("vmid")
	if vmid == "" {
		log.Error().Msg("VM ID is required")
		http.Error(w, "VM ID is required", http.StatusBadRequest)
		return
	}

	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		log.Error().Err(err).Str("vmid", vmid).Msg("Invalid VM ID")
		http.Error(w, "Invalid VM ID", http.StatusBadRequest)
		return
	}

	stateManager := getStateManager(r)
	client := stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not available")
		http.Error(w, "Proxmox client not available", http.StatusInternalServerError)
		return
	}

	// Get all VMs and find the one we want
	vms, err := proxmox.GetVMsWithContext(r.Context(), client)
	if err != nil {
		log.Error().Err(err).Int("vmid", vmidInt).Msg("Failed to get VMs")
		http.Error(w, "Failed to get VMs", http.StatusInternalServerError)
		return
	}

	// Find the VM by ID
	var vm *proxmox.VM
	for i := range vms {
		if vms[i].VMID == vmidInt {
			vm = &vms[i]
			break
		}
	}

	if vm == nil {
		log.Error().Int("vmid", vmidInt).Msg("VM not found")
		http.Error(w, "VM not found", http.StatusNotFound)
		return
	}

	// Get CSRF token
	handlerCtx := NewHandlerContext(w, r, "VMDeleteConfirmHandler")
	csrfToken, _ := handlerCtx.GetCSRFToken()

	// Build custom data for template
	custom := map[string]interface{}{
		"VM":        vm,
		"CSRFToken": csrfToken,
	}

	// Render confirmation page
	th := NewTemplateHelpers()
	th.RenderUserPage(w, r, "vm_delete_confirm", "Confirm Delete VM", stateManager, custom)
}

// VMDeleteHandler handles the actual VM deletion (force stop + delete)
func (h *VMHandler) VMDeleteHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("VMDeleteHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	vmid := r.FormValue("vmid")
	node := r.FormValue("node")
	if vmid == "" || node == "" {
		log.Warn().Str("vmid", vmid).Str("node", node).Msg("missing required fields")
		localizer := i18n.GetLocalizerFromRequest(r)
		http.Error(w, i18n.Localize(localizer, "Error.Generic"), http.StatusBadRequest)
		return
	}

	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		log.Error().Err(err).Str("vmid", vmid).Msg("invalid VM ID")
		localizer := i18n.GetLocalizerFromRequest(r)
		http.Error(w, i18n.Localize(localizer, "Error.Generic"), http.StatusBadRequest)
		return
	}

	stateManager := getStateManager(r)
	if stateManager == nil {
		log.Error().Msg("state manager not available")
		localizer := i18n.GetLocalizerFromRequest(r)
		http.Error(w, i18n.Localize(localizer, "Error.Generic"), http.StatusInternalServerError)
		return
	}

	client := stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not available")
		localizer := i18n.GetLocalizerFromRequest(r)
		http.Error(w, i18n.Localize(localizer, "Error.Generic"), http.StatusInternalServerError)
		return
	}

	log.Info().Int("vmid", vmidInt).Str("node", node).Msg("starting VM deletion process")

	// Step 1: Force stop the VM (ignore errors if already stopped)
	log.Info().Int("vmid", vmidInt).Str("node", node).Msg("forcing VM stop")
	_, stopErr := proxmox.VMActionWithContext(r.Context(), client, node, vmid, "stop")
	if stopErr != nil {
		log.Warn().Err(stopErr).Int("vmid", vmidInt).Msg("VM stop failed (may already be stopped)")
	} else {
		log.Info().Int("vmid", vmidInt).Msg("VM stopped successfully")
		// Wait a moment for the stop to complete
		time.Sleep(2 * time.Second)
	}

	// Step 2: Delete the VM
	log.Info().Int("vmid", vmidInt).Str("node", node).Msg("deleting VM")
	if err := proxmox.DeleteVMWithContext(r.Context(), client, node, vmidInt); err != nil {
		log.Error().Err(err).Int("vmid", vmidInt).Msg("VM deletion failed")
		mh := NewMessageHandlers()
		errMsg := fmt.Sprintf("Failed to delete VM: %v", err)
		errURL := mh.helper.BuildErrorURL("/vm/details/"+vmid, errMsg)
		http.Redirect(w, r, errURL, http.StatusSeeOther)
		return
	}

	log.Info().Int("vmid", vmidInt).Msg("VM deleted successfully")

	// Invalidate pool cache to ensure profile page shows updated VM list
	// Get username from session to derive pool name
	username := ""
	if sessionManager := security.GetSession(r); sessionManager != nil {
		if user, ok := sessionManager.Get(r.Context(), "username").(string); ok {
			username = user
		}
	}
	if username != "" {
		poolName := "pvmss_" + username
		client.InvalidateCache("/pools/" + poolName)
		log.Info().Str("pool", poolName).Msg("Invalidated pool cache after VM deletion")
	}

	// Redirect to profile page with success message and refresh parameter
	mh := NewMessageHandlers()
	params := map[string]string{
		"lang":    i18n.GetLanguage(r),
		"refresh": "1",
	}
	mh.RedirectWithSuccess(w, r, "/profile", i18n.Localize(i18n.GetLocalizerFromRequest(r), "VMDelete.Success"), params)
}
