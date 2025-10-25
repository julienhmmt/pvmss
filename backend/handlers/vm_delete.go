package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
	"pvmss/proxmox"
	"pvmss/security"
)

// findVMByID finds a VM in a list by its ID
func findVMByID(vms []proxmox.VM, vmid int) *proxmox.VM {
	for i := range vms {
		if vms[i].VMID == vmid {
			return &vms[i]
		}
	}
	return nil
}

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

	// Get all VMs and find the one we want using resty
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create resty client")
		http.Error(w, "Failed to create API client", http.StatusInternalServerError)
		return
	}

	vms, err := proxmox.GetVMsResty(r.Context(), restyClient)
	if err != nil {
		log.Error().Err(err).Int("vmid", vmidInt).Msg("Failed to get VMs (resty)")
		http.Error(w, "Failed to get VMs", http.StatusInternalServerError)
		return
	}

	// Find the VM by ID
	vm := findVMByID(vms, vmidInt)
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

	// Create resty client for VM operations
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create resty client")
		http.Error(w, "Failed to create API client", http.StatusInternalServerError)
		return
	}

	// Step 1: Check current VM status
	log.Info().Int("vmid", vmidInt).Str("node", node).Msg("checking VM status before deletion")
	currentStatus, statusErr := proxmox.GetVMCurrentResty(r.Context(), restyClient, node, vmidInt)
	if statusErr != nil {
		log.Warn().Err(statusErr).Int("vmid", vmidInt).Msg("Could not get VM status, proceeding with deletion")
	} else if currentStatus != nil && currentStatus.Status == "running" {
		// VM is running, need to stop it first
		log.Info().Int("vmid", vmidInt).Str("node", node).Msg("VM is running, attempting shutdown")

		// Try graceful shutdown first
		log.Info().Int("vmid", vmidInt).Str("node", node).Msg("Attempting graceful shutdown")
		if taskID, err := proxmox.VMActionResty(r.Context(), restyClient, node, vmid, "shutdown"); err != nil {
			log.Warn().Err(err).Int("vmid", vmidInt).Str("node", node).Msg("Failed to send shutdown command")
		} else if taskID != "" {
			log.Info().Str("task_id", taskID).Int("vmid", vmidInt).Msg("Shutdown task started")
		}

		// Wait a bit to allow shutdown to proceed
		log.Info().Int("vmid", vmidInt).Msg("Waiting for VM to shutdown gracefully")
		time.Sleep(5 * time.Second)

		// Check status again
		checkStatus, checkErr := proxmox.GetVMCurrentResty(r.Context(), restyClient, node, vmidInt)
		if checkErr == nil && checkStatus != nil && checkStatus.Status == "running" {
			log.Warn().Int("vmid", vmidInt).Msg("Shutdown did not stop VM, sending stop command")
			// Send stop command
			if taskID, err := proxmox.VMActionResty(r.Context(), restyClient, node, vmid, "stop"); err != nil {
				log.Error().Err(err).Int("vmid", vmidInt).Str("node", node).Msg("Failed to send stop command")
			} else if taskID != "" {
				log.Info().Str("task_id", taskID).Int("vmid", vmidInt).Msg("Stop task started")
			}
		}

		log.Info().Int("vmid", vmidInt).Msg("Stop command sent, waiting for VM to stop")

		// Wait and check status in loop (up to 30 seconds)
		vmStopped := false
		for i := 0; i < 10; i++ {
			time.Sleep(3 * time.Second)
			checkStatus, checkErr := proxmox.GetVMCurrentResty(r.Context(), restyClient, node, vmidInt)
			if checkErr == nil && checkStatus != nil && checkStatus.Status != "running" {
				vmStopped = true
				log.Info().Int("vmid", vmidInt).Int("attempt", i+1).Msg("VM successfully stopped")
				break
			}
			log.Info().Int("vmid", vmidInt).Int("attempt", i+1).Msg("VM still running, waiting...")
		}

		if !vmStopped {
			log.Error().Int("vmid", vmidInt).Msg("VM did not stop after 30 seconds, cannot delete safely")
			ctx := NewHandlerContext(w, r, "VMDeleteHandler")
			ctx.RedirectWithError("/vm/details/"+vmid, "VMDelete.Error")
			return
		}
	} else {
		log.Info().Int("vmid", vmidInt).Msg("VM is already stopped, proceeding with deletion")
	}

	// Step 2: Delete the VM
	log.Info().Int("vmid", vmidInt).Str("node", node).Msg("deleting VM")
	if err := proxmox.DeleteVMResty(r.Context(), restyClient, node, vmidInt); err != nil {
		log.Error().Err(err).Int("vmid", vmidInt).Msg("VM deletion failed")
		ctx := NewHandlerContext(w, r, "VMDeleteHandler")
		ctx.RedirectWithError("/vm/details/"+vmid, "VMDelete.Error")
		return
	}

	log.Info().Int("vmid", vmidInt).Msg("VM deleted successfully")

	// Invalidate caches to ensure UI shows fresh data
	// 1) User pool cache (profile page)
	if sessionManager := security.GetSession(r); sessionManager != nil {
		if username, ok := sessionManager.Get(r.Context(), "username").(string); ok && username != "" {
			poolName := "pvmss_" + username
			client.InvalidateCache("/pools/" + poolName)
			log.Info().Str("pool", poolName).Msg("Invalidated pool cache after VM deletion")
		}
	}

	// 2) Nodes and per-node VM lists (details and listings)
	client.InvalidateCache("/nodes")
	client.InvalidateCache("/nodes/" + node + "/qemu")
	// 3) Specific VM paths just in case some views cached them
	client.InvalidateCache("/nodes/" + node + "/qemu/" + vmid)
	client.InvalidateCache("/nodes/" + node + "/qemu/" + vmid + "/status/current")
	log.Info().Str("node", node).Int("vmid", vmidInt).Msg("Invalidated node and VM caches after deletion")

	// Redirect to profile page with success message and refresh parameter
	ctx := NewHandlerContext(w, r, "VMDeleteHandler")
	ctx.RedirectWithParams("/profile", map[string]string{
		"success":     "1",
		"success_msg": ctx.Translate("VMDelete.Success"),
		"refresh":     "1",
	})
}
