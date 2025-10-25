package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
	"pvmss/proxmox"
)

// Helper function to build VM details URL with refresh
func buildVMDetailsURL(vmid string) string {
	return fmt.Sprintf("/vm/details/%s?refresh=1&ts=%d", vmid, time.Now().Unix())
}

// UpdateVMDescriptionHandler updates the VM description (Markdown supported on display)
func (h *VMHandler) UpdateVMDescriptionHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := NewHandlerContext(w, r, "UpdateVMDescriptionHandler")

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}
	vmid := strings.TrimSpace(r.FormValue("vmid"))
	node := strings.TrimSpace(r.FormValue("node"))
	desc := r.FormValue("description")
	// If user is not authenticated, redirect to login with return + context to show a friendly notice
	if !IsAuthenticated(r) {
		returnTo := "/"
		if vmid != "" {
			returnTo = "/vm/details/" + vmid + "?edit=description"
		}
		http.Redirect(w, r, "/login?warning=login_required&context=update_description&return="+url.QueryEscape(returnTo), http.StatusSeeOther)
		return
	}
	if vmid == "" || node == "" {
		ctx.HandleError(nil, "Bad request", http.StatusBadRequest)
		return
	}
	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		ctx.HandleError(err, "Invalid VM ID", http.StatusBadRequest)
		return
	}

	client := ctx.StateManager.GetProxmoxClient()
	if client == nil {
		ctx.HandleError(nil, "Proxmox client not available", http.StatusInternalServerError)
		return
	}

	if err := proxmox.UpdateVMConfigWithContext(r.Context(), client, node, vmidInt, map[string]string{"description": desc}); err != nil {
		ctx.Log.Error().Err(err).Msg("update description failed")
		ctx.RedirectWithError(buildVMDetailsURL(vmid), "Message.ActionFailed")
		return
	}
	ctx.Log.Info().Str("vmid", vmid).Str("node", node).Msg("VM description updated successfully")
	ctx.RedirectWithSuccess(buildVMDetailsURL(vmid), "Message.UpdatedSuccessfully")
}

// UpdateVMTagsHandler updates the VM tags from selected checkboxes
func (h *VMHandler) UpdateVMTagsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := NewHandlerContext(w, r, "UpdateVMTagsHandler")

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}
	vmid := strings.TrimSpace(r.FormValue("vmid"))
	node := strings.TrimSpace(r.FormValue("node"))
	if vmid == "" || node == "" {
		ctx.HandleError(nil, "Bad request", http.StatusBadRequest)
		return
	}
	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		ctx.HandleError(err, "Invalid VM ID", http.StatusBadRequest)
		return
	}

	// Get selected tags (comes as array of selected checkbox values)
	selectedTags := r.Form["tags"]
	tagsStr := strings.Join(selectedTags, ";")

	client := ctx.StateManager.GetProxmoxClient()
	if client == nil {
		ctx.HandleError(nil, "Proxmox client not available", http.StatusInternalServerError)
		return
	}

	// Update tags in Proxmox
	if err := proxmox.UpdateVMConfigWithContext(r.Context(), client, node, vmidInt, map[string]string{"tags": tagsStr}); err != nil {
		ctx.Log.Error().Err(err).Msg("update tags failed")
		ctx.RedirectWithError(buildVMDetailsURL(vmid), "Message.ActionFailed")
		return
	}
	ctx.RedirectWithSuccess(buildVMDetailsURL(vmid), "Message.UpdatedSuccessfully")
}

// VMActionHandler handles VM lifecycle actions via server-side POST forms
func (h *VMHandler) VMActionHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("VMActionHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	vmid := r.FormValue("vmid")
	node := r.FormValue("node")
	action := r.FormValue("action")
	if vmid == "" || node == "" || action == "" {
		log.Warn().Str("vmid", vmid).Str("node", node).Str("action", action).Msg("missing required fields")
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

	log.Info().Str("action", action).Int("vmid", vmidInt).Msg("executing VM action")

	// Execute the action using resty
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create resty client")
		ctx := NewHandlerContext(w, r, "VMActionHandler")
		ctx.RedirectWithError("/vm/details/"+vmid, "Error.InternalServer")
		return
	}

	_, err = proxmox.VMActionResty(r.Context(), restyClient, node, vmid, action)
	if err != nil {
		log.Error().Err(err).Str("action", action).Int("vmid", vmidInt).Msg("VM action failed")
		ctx := NewHandlerContext(w, r, "VMActionHandler")
		ctx.RedirectWithError(buildVMDetailsURL(vmid), "Message.ActionFailed")
		return
	}

	log.Info().Str("action", action).Int("vmid", vmidInt).Msg("VM action completed successfully")

	ctx := NewHandlerContext(w, r, "VMActionHandler")
	ctx.RedirectWithParams(buildVMDetailsURL(vmid), map[string]string{
		"success":     "1",
		"success_msg": ctx.Translate("VMDetails.Action.Success"),
		"action":      action,
	})
}

// UpdateVMResourcesHandler updates VM resources (CPU sockets/cores, memory, network bridge)
func (h *VMHandler) UpdateVMResourcesHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := NewHandlerContext(w, r, "UpdateVMResourcesHandler")

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	coresStr := strings.TrimSpace(r.FormValue("cores"))
	memoryStr := strings.TrimSpace(r.FormValue("memory"))
	networkModel := strings.TrimSpace(r.FormValue("network_model"))
	node := strings.TrimSpace(r.FormValue("node"))
	socketsStr := strings.TrimSpace(r.FormValue("sockets"))
	vmbr := strings.TrimSpace(r.FormValue("vmbr"))
	vmid := strings.TrimSpace(r.FormValue("vmid"))

	if vmid == "" || node == "" {
		ctx.HandleError(nil, "Bad request", http.StatusBadRequest)
		return
	}

	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		ctx.HandleError(err, "Invalid VM ID", http.StatusBadRequest)
		return
	}

	// Parse and validate numeric values
	sockets, err := strconv.Atoi(socketsStr)
	if err != nil || sockets < 1 {
		ctx.RedirectWithError(fmt.Sprintf("/vm/details/%d?edit=resources", vmidInt), "Error.InvalidInput")
		return
	}

	cores, err := strconv.Atoi(coresStr)
	if err != nil || cores < 1 {
		ctx.RedirectWithError(fmt.Sprintf("/vm/details/%d?edit=resources", vmidInt), "Error.InvalidInput")
		return
	}

	memory, err := strconv.ParseInt(memoryStr, 10, 64)
	if err != nil || memory < 1 {
		ctx.RedirectWithError(fmt.Sprintf("/vm/details/%d?edit=resources", vmidInt), "Error.InvalidInput")
		return
	}

	if vmbr == "" {
		ctx.RedirectWithError(fmt.Sprintf("/vm/details/%d?edit=resources", vmidInt), "Error.InvalidInput")
		return
	}

	// Validate network model
	validModels := map[string]bool{
		"virtio":  true,
		"e1000":   true,
		"e1000e":  true,
		"rtl8139": true,
		"vmxnet3": true,
	}
	if networkModel == "" {
		networkModel = "virtio" // default
	} else if !validModels[networkModel] {
		ctx.Log.Warn().Str("network_model", networkModel).Msg("Invalid network model, defaulting to virtio")
		networkModel = "virtio"
	}

	// Get Proxmox client
	stateManager := getStateManager(r)
	client := stateManager.GetProxmoxClient()
	if client == nil {
		ctx.HandleError(nil, "Proxmox client not available", http.StatusInternalServerError)
		return
	}

	// Get current VM config to preserve MAC address
	cfg, err := proxmox.GetVMConfigWithContext(r.Context(), client, node, vmidInt)
	if err != nil {
		ctx.Log.Warn().Err(err).Msg("Failed to get current VM config, MAC address may not be preserved")
		cfg = nil
	}

	// Extract current MAC address from net0 if it exists
	currentMAC := ""
	if cfg != nil {
		if net0Val, ok := cfg["net0"].(string); ok && net0Val != "" {
			// Parse net0 format to extract MAC: "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0"
			parts := strings.Split(net0Val, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				// Check for model=MAC format (Proxmox standard)
				validModels := []string{"virtio", "e1000", "e1000e", "rtl8139", "vmxnet3"}
				for _, model := range validModels {
					if strings.HasPrefix(part, model+"=") {
						currentMAC = strings.TrimPrefix(part, model+"=")
						break
					}
				}
				if currentMAC != "" {
					break
				}
			}
		}
	}

	// Build update parameters with selected network model and preserved MAC
	// Format: "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0"
	net0Value := networkModel
	if currentMAC != "" {
		net0Value = networkModel + "=" + currentMAC
	}
	net0Value = net0Value + ",bridge=" + vmbr
	ctx.Log.Debug().Str("net0_value", net0Value).Str("current_mac", currentMAC).Msg("Building net0 parameter")

	updateParams := map[string]string{
		"sockets": socketsStr,
		"cores":   coresStr,
		"memory":  memoryStr,
		"net0":    net0Value,
	}

	// Update VM config
	if err := proxmox.UpdateVMConfigWithContext(r.Context(), client, node, vmidInt, updateParams); err != nil {
		ctx.Log.Error().Err(err).Msg("update resources failed")
		ctx.RedirectWithError(buildVMDetailsURL(vmid), "Message.ActionFailed")
		return
	}

	ctx.Log.Info().Str("vmid", vmid).Str("node", node).
		Int("sockets", sockets).Int("cores", cores).Int64("memory", memory).
		Str("vmbr", vmbr).Str("network_model", networkModel).Msg("VM resources updated successfully")

	// Invalidate guest agent cache for this VM since network config changed
	InvalidateGuestAgentCache(node, vmidInt)

	ctx.RedirectWithSuccess(buildVMDetailsURL(vmid), "Message.UpdatedSuccessfully")
}
