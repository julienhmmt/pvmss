package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"pvmss/components"
	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/security"

	"github.com/julienschmidt/httprouter"
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
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.MissingRequiredFields"), http.StatusBadRequest)
		return
	}

	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		log.Error().Err(err).Str("vmid", vmid).Msg("Invalid VM ID")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.Generic"), http.StatusBadRequest)
		return
	}

	// Get all VMs and find the one we want using resty
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create resty client")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.ServerConfigError"), http.StatusInternalServerError)
		return
	}

	vms, err := proxmox.GetVMsResty(r.Context(), restyClient)
	if err != nil {
		log.Error().Err(err).Int("vmid", vmidInt).Msg("Failed to get VMs (resty)")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.FailedToGetResources"), http.StatusInternalServerError)
		return
	}

	// Find the VM by ID
	vm := findVMByID(vms, vmidInt)
	if vm == nil {
		log.Error().Int("vmid", vmidInt).Msg("VM not found")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.NotFound"), http.StatusNotFound)
		return
	}

	handlerCtx := HandlerContextWith(w, r, "VMDeleteConfirmHandler")
	csrfToken, _ := handlerCtx.GetCSRFToken()

	// Get username from session
	username := ""
	if sessionManager := security.GetSession(r); sessionManager != nil {
		if user, ok := sessionManager.Get(r.Context(), "username").(string); ok {
			username = user
		}
	}

	// Prepare delete confirmation data
	deleteData := components.VMDeleteData{
		VMID:      vm.VMID,
		Name:      vm.Name,
		Node:      vm.Node,
		Status:    vm.Status,
		CSRFToken: csrfToken,
		Username:  username,
		Lang:      i18n.GetLanguage(r),
	}

	// Translation function wrapper
	translateFunc := func(key string) string {
		return handlerCtx.Translate(key)
	}

	// Render with Templ
	if err := components.VMDeleteConfirmPage(deleteData, translateFunc).Render(r.Context(), w); err != nil {
		log.Error().Err(err).Msg("Failed to render VM delete confirmation page")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
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
		log.Warn().
			Str("component", "vm_delete").
			Str("operation", "validate_delete_request").
			Str("reason", "missing_fields").
			Str("vmid", vmid).
			Str("node", node).
			Msg("Missing required fields for VM deletion")
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

	log.Info().Int("vmid", vmidInt).Str("node", node).Msg("starting VM deletion process")

	// Create resty client for VM operations
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create resty client")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.ServerConfigError"), http.StatusInternalServerError)
		return
	}

	// Step 1: Check current VM status
	log.Info().Int("vmid", vmidInt).Str("node", node).Msg("checking VM status before deletion")
	currentStatus, statusErr := proxmox.GetVMCurrentResty(r.Context(), restyClient, node, vmidInt)
	if statusErr != nil {
		log.Warn().
			Err(statusErr).
			Str("component", "vm_delete").
			Str("operation", "check_vm_status").
			Str("reason", "status_check_failed").
			Int("vmid", vmidInt).
			Msg("Could not get VM status; proceeding with deletion")
	} else if currentStatus != nil && currentStatus.Status == "running" {
		// VM is running, need to stop it first
		log.Info().Int("vmid", vmidInt).Str("node", node).Msg("VM is running, attempting shutdown")

		// Try graceful shutdown first
		log.Info().Int("vmid", vmidInt).Str("node", node).Msg("Attempting graceful shutdown")
		if taskID, err := proxmox.VMActionResty(r.Context(), restyClient, node, vmid, "shutdown"); err != nil {
			log.Warn().
				Err(err).
				Str("component", "vm_delete").
				Str("operation", "shutdown_vm").
				Str("reason", "shutdown_command_failed").
				Int("vmid", vmidInt).
				Str("node", node).
				Msg("Failed to send shutdown command")
		} else if taskID != "" {
			log.Info().Str("task_id", taskID).Int("vmid", vmidInt).Msg("Shutdown task started")
		}

		// Wait a bit to allow shutdown to proceed
		log.Info().Int("vmid", vmidInt).Msg("Waiting for VM to shutdown gracefully")
		time.Sleep(5 * time.Second)

		// Check status again
		checkStatus, checkErr := proxmox.GetVMCurrentResty(r.Context(), restyClient, node, vmidInt)
		if checkErr == nil && checkStatus != nil && checkStatus.Status == "running" {
			log.Warn().
				Str("component", "vm_delete").
				Str("operation", "shutdown_vm").
				Str("reason", "graceful_shutdown_failed").
				Int("vmid", vmidInt).
				Msg("Shutdown did not stop VM; sending stop command")
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
			ctx := HandlerContextWith(w, r, "VMDeleteHandler")
			ctx.RedirectWithError("/vm/details/"+vmid, "VMDelete.Error")
			return
		}
	} else {
		log.Info().Int("vmid", vmidInt).Msg("VM is already stopped, proceeding with deletion")
	}

	// Get username for audit before deletion
	username := "unknown"
	isAdmin := false
	if sessionManager := security.GetSession(r); sessionManager != nil {
		if user, ok := sessionManager.Get(r.Context(), "username").(string); ok && user != "" {
			username = user
		}
		if admin, ok := sessionManager.Get(r.Context(), "is_admin").(bool); ok {
			isAdmin = admin
		}
	}

	// Step 2: Delete the VM
	log.Debug().Int("vmid", vmidInt).Str("node", node).Msg("Sending delete request to Proxmox")
	if err := proxmox.DeleteVMResty(r.Context(), restyClient, node, vmidInt); err != nil {
		logger.VMFailure("vm_delete", vmidInt, node, "proxmox_api_error").
			Err(err).
			Str("username", username).
			Str("client_ip", r.RemoteAddr).
			Msg("VM deletion failed")
		ctx := HandlerContextWith(w, r, "VMDeleteHandler")
		ctx.RedirectWithError("/vm/details/"+vmid, "VMDelete.Error")
		return
	}

	// Log successful deletion with structured event
	logger.VMEvent("vm_delete", vmidInt, node).
		Str("username", username).
		Bool("is_admin", isAdmin).
		Str("client_ip", r.RemoteAddr).
		Msg("VM deleted successfully")

	// Delete associated cloud-init snippet if SFTP is enabled
	settings := stateManager.GetSettings()
	if settings != nil && settings.CloudInitSFTP.Enabled {
		snippetFilename := fmt.Sprintf("pvmss-%d.yml", vmidInt)
		if err := proxmox.DeleteSnippetFileSFTP(settings.CloudInitSFTP, snippetFilename); err != nil {
			log.Warn().
				Err(err).
				Str("snippet", snippetFilename).
				Int("vmid", vmidInt).
				Msg("Failed to delete cloud-init snippet (VM already deleted)")
		} else {
			log.Info().
				Str("snippet", snippetFilename).
				Int("vmid", vmidInt).
				Msg("Cloud-init snippet deleted successfully")
		}
	}

	// Redirect to profile page with success message and refresh parameter
	ctx := HandlerContextWith(w, r, "VMDeleteHandler")
	ctx.RedirectWithParams("/profile", map[string]string{
		"success":     "1",
		"success_msg": ctx.Translate("VMDelete.Success"),
		"refresh":     "1",
	})
}
