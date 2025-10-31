package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

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
	// Sanitize description
	{
		s := NewInputSanitizer()
		desc = s.RemoveScriptTags(s.SanitizeString(desc, 2000))
	}
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
	// Sanitize tags
	if len(selectedTags) > 0 {
		s := NewInputSanitizer()
		cleaned := make([]string, 0, len(selectedTags))
		for _, t := range selectedTags {
			st := s.SanitizeString(strings.TrimSpace(t), 64)
			if st != "" {
				cleaned = append(cleaned, st)
			}
		}
		selectedTags = cleaned
	}
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
		RespondWithError(w, r, ErrBadRequest)
		return
	}

	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		log.Error().Err(err).Str("vmid", vmid).Msg("invalid VM ID")
		RespondWithError(w, r, ErrBadRequest)
		return
	}

	stateManager := getStateManager(r)
	if stateManager == nil {
		log.Error().Msg("state manager not available")
		RespondWithError(w, r, ErrInternalServer)
		return
	}

	client := stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not available")
		RespondWithError(w, r, ErrProxmoxConnection)
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
	node := strings.TrimSpace(r.FormValue("node"))
	socketsStr := strings.TrimSpace(r.FormValue("sockets"))
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

	stateManager := getStateManager(r)
	if stateManager == nil {
		ctx.HandleError(nil, "State manager not available", http.StatusInternalServerError)
		return
	}

	settings := stateManager.GetSettings()
	maxNetworkCards := 1
	if settings != nil && settings.MaxNetworkCards > 0 {
		maxNetworkCards = settings.MaxNetworkCards
	}

	restyClient, err := getDefaultRestyClient()
	if err != nil {
		ctx.HandleError(err, "Failed to create API client", http.StatusInternalServerError)
		return
	}

	validModels := map[string]bool{
		"virtio":  true,
		"e1000":   true,
		"e1000e":  true,
		"rtl8139": true,
		"vmxnet3": true,
	}

	values := url.Values{}
	values.Set("sockets", socketsStr)
	values.Set("cores", coresStr)
	values.Set("memory", memoryStr)

	// Handle CD-ROM ISO update
	cdromISO := strings.TrimSpace(r.FormValue("cdrom_iso"))
	if cdromISO != "" {
		// Set new ISO
		values.Set("ide2", cdromISO+",media=cdrom")
		ctx.Log.Info().Str("vmid", vmid).Str("node", node).Str("iso", cdromISO).Msg("Updating CD-ROM ISO")
	} else {
		// Eject ISO (remove ide2)
		values.Add("delete", "ide2")
		ctx.Log.Info().Str("vmid", vmid).Str("node", node).Msg("Ejecting CD-ROM ISO")
	}

	deleteTargets := []string{}

	for i := 0; i < maxNetworkCards; i++ {
		bridge := strings.TrimSpace(r.FormValue(fmt.Sprintf("bridge_%d", i)))
		model := strings.TrimSpace(r.FormValue(fmt.Sprintf("network_model_%d", i)))
		mac := strings.TrimSpace(strings.ToUpper(r.FormValue(fmt.Sprintf("mac_%d", i))))
		exists := strings.TrimSpace(r.FormValue(fmt.Sprintf("exists_%d", i))) == "1"
		optionsRaw := strings.TrimSpace(r.FormValue(fmt.Sprintf("options_%d", i)))
		linkDownStr := strings.TrimSpace(r.FormValue(fmt.Sprintf("link_down_%d", i)))
		linkDown := linkDownStr == "1" || linkDownStr == "true"

		var options []string
		if optionsRaw != "" {
			for _, opt := range strings.Split(optionsRaw, ",") {
				opt = strings.TrimSpace(opt)
				if opt != "" && opt != "link_down" {
					options = append(options, opt)
				}
			}
		}

		if i == 0 && bridge == "" {
			ctx.RedirectWithError(fmt.Sprintf("/vm/details/%d?edit=resources", vmidInt), "Error.InvalidInput")
			return
		}

		if bridge == "" {
			if exists {
				deleteTargets = append(deleteTargets, fmt.Sprintf("net%d", i))
			}
			continue
		}

		if model == "" {
			model = "virtio"
		}
		if !validModels[model] {
			ctx.Log.Warn().Int("card_index", i).Str("network_model", model).Msg("Invalid network model, defaulting to virtio")
			model = "virtio"
		}

		netParts := []string{}
		if mac != "" {
			netParts = append(netParts, model+"="+mac)
		} else {
			netParts = append(netParts, model)
		}
		netParts = append(netParts, "bridge="+bridge)

		// Add link_down option if interface is disabled
		if linkDown {
			netParts = append(netParts, "link_down=1")
		}

		netParts = append(netParts, options...)

		values.Set(fmt.Sprintf("net%d", i), strings.Join(netParts, ","))
	}

	for _, target := range deleteTargets {
		values.Add("delete", target)
	}

	path := fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmidInt)
	var response interface{}
	if err := restyClient.Post(r.Context(), path, values, &response); err != nil {
		ctx.Log.Error().Err(err).Msg("update resources failed")
		ctx.RedirectWithError(buildVMDetailsURL(vmid), "Message.ActionFailed")
		return
	}

	ctx.Log.Info().Str("vmid", vmid).Str("node", node).
		Int("sockets", sockets).Int("cores", cores).Int64("memory", memory).
		Int("network_cards", maxNetworkCards).Msg("VM resources updated successfully")

	// Invalidate guest agent cache for this VM since network config changed
	InvalidateGuestAgentCache(node, vmidInt)

	ctx.RedirectWithSuccess(buildVMDetailsURL(vmid), "Message.UpdatedSuccessfully")
}

// ToggleNetworkCardHandler toggles a single network card enable/disable state
func (h *VMHandler) ToggleNetworkCardHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := NewHandlerContext(w, r, "ToggleNetworkCardHandler")

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	vmidStr := strings.TrimSpace(r.FormValue("vmid"))
	node := strings.TrimSpace(r.FormValue("node"))
	cardIndexStr := strings.TrimSpace(r.FormValue("card_index"))
	action := strings.TrimSpace(r.FormValue("action"))        // enable|disable (legacy)
	enabledParam := strings.TrimSpace(r.FormValue("enabled")) // "1" when ON, empty when OFF

	if vmidStr == "" || node == "" || cardIndexStr == "" || (enabledParam == "" && (action != "enable" && action != "disable")) {
		ctx.HandleError(nil, "Bad request", http.StatusBadRequest)
		return
	}

	vmidInt, err := strconv.Atoi(vmidStr)
	if err != nil {
		ctx.HandleError(err, "Invalid VM ID", http.StatusBadRequest)
		return
	}

	cardIndex, err := strconv.Atoi(cardIndexStr)
	if err != nil || cardIndex < 0 {
		ctx.HandleError(err, "Invalid card index", http.StatusBadRequest)
		return
	}

	restyClient, err := getDefaultRestyClient()
	if err != nil {
		ctx.HandleError(err, "Failed to create API client", http.StatusInternalServerError)
		return
	}

	// Get current VM config to preserve existing network settings
	vmConfig, err := proxmox.GetVMConfigResty(r.Context(), restyClient, node, vmidInt)
	if err != nil {
		ctx.Log.Error().Err(err).Msg("Failed to get VM config for network toggle")
		ctx.RedirectWithError(buildVMDetailsURL(vmidStr), "Message.ActionFailed")
		return
	}

	// Find the network interface to modify
	netKey := fmt.Sprintf("net%d", cardIndex)
	currentConfig := ""
	if vmConfig != nil {
		if netVal, ok := vmConfig[netKey].(string); ok {
			currentConfig = netVal
		}
	}

	if currentConfig == "" {
		ctx.Log.Warn().Int("card_index", cardIndex).Str("vmid", vmidStr).Msg("Network interface not found")
		ctx.RedirectWithError(buildVMDetailsURL(vmidStr), "Message.ActionFailed")
		return
	}

	// Parse current config
	model, mac, bridge, options, currentLinkDown := parseNetworkConfig(currentConfig)

	ctx.Log.Info().Str("vmid", vmidStr).Str("node", node).Int("card_index", cardIndex).
		Str("current_config", currentConfig).Str("model", model).Str("mac", mac).
		Str("bridge", bridge).Bool("currently_link_down", currentLinkDown).
		Str("requested_action", action).Msg("Current network config")

	// Determine new link_down state
	var newLinkDown bool
	if enabledParam != "" {
		// enabled=1 means link should be UP (link_down=false)
		newLinkDown = enabledParam != "1"
	} else {
		newLinkDown = (action == "disable")
	}

	// Check if change is needed
	if currentLinkDown == newLinkDown {
		ctx.Log.Info().Str("vmid", vmidStr).Int("card_index", cardIndex).
			Bool("link_down", newLinkDown).Msg("Network card already in requested state, no change needed")
		successMsg := ""
		if enabledParam != "" {
			if enabledParam == "1" {
				successMsg = ctx.Translate("Message.NetworkCardEnabled")
			} else {
				successMsg = ctx.Translate("Message.NetworkCardDisabled")
			}
		} else if action == "enable" {
			successMsg = ctx.Translate("Message.NetworkCardEnabled")
		} else {
			successMsg = ctx.Translate("Message.NetworkCardDisabled")
		}
		redirectURL := fmt.Sprintf("/vm/details/%s?success=1&success_msg=%s", vmidStr, url.QueryEscape(successMsg))
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		return
	}

	// Build new network config
	netParts := []string{}
	if mac != "" {
		netParts = append(netParts, model+"="+mac)
	} else {
		netParts = append(netParts, model)
	}
	netParts = append(netParts, "bridge="+bridge)

	// Filter out any existing link_down options from the options list
	filteredOptions := []string{}
	for _, opt := range options {
		if !strings.HasPrefix(opt, "link_down") {
			filteredOptions = append(filteredOptions, opt)
		}
	}

	// Add link_down flag explicitly
	if newLinkDown {
		// Disable interface
		netParts = append(netParts, "link_down=1")
	} else {
		// Ensure interface is enabled; be explicit to clear any previous flag
		netParts = append(netParts, "link_down=0")
	}

	// Add back the filtered options
	netParts = append(netParts, filteredOptions...)

	newConfig := strings.Join(netParts, ",")
	ctx.Log.Info().Str("vmid", vmidStr).Str("node", node).Int("card_index", cardIndex).
		Str("old_config", currentConfig).Str("new_config", newConfig).
		Bool("enabling", action == "enable").Msg("Applying network config change")

	// Update VM config via Proxmox API
	params := map[string]string{
		netKey: newConfig,
	}

	ctx.Log.Debug().Str("vmid", vmidStr).Str("node", node).
		Str("param_key", netKey).Str("param_value", newConfig).
		Msg("Sending update to Proxmox API")

	if err := proxmox.UpdateVMConfigResty(r.Context(), restyClient, node, vmidInt, params); err != nil {
		ctx.Log.Error().Err(err).Str("vmid", vmidStr).Int("card_index", cardIndex).
			Str("attempted_config", newConfig).Msg("Network toggle failed - Proxmox API error")
		ctx.RedirectWithError(buildVMDetailsURL(vmidStr), "Message.ActionFailed")
		return
	}

	ctx.Log.Info().Str("vmid", vmidStr).Str("node", node).Int("card_index", cardIndex).
		Str("action", action).Bool("link_down", newLinkDown).
		Msg("Network card state changed successfully in Proxmox")

	// Invalidate guest agent cache for this VM since network config changed
	InvalidateGuestAgentCache(node, vmidInt)
	ctx.Log.Debug().Str("vmid", vmidStr).Int("vmid_int", vmidInt).Msg("Invalidated guest agent cache")

	// Prepare success message
	successMsg := ""
	if enabledParam != "" {
		if enabledParam == "1" {
			successMsg = ctx.Translate("Message.NetworkCardEnabled")
		} else {
			successMsg = ctx.Translate("Message.NetworkCardDisabled")
		}
	} else if action == "enable" {
		successMsg = ctx.Translate("Message.NetworkCardEnabled")
	} else {
		successMsg = ctx.Translate("Message.NetworkCardDisabled")
	}

	ctx.Log.Info().Str("vmid", vmidStr).Int("card_index", cardIndex).
		Str("final_state", action).Msg("Network card toggle completed, redirecting with success")

	redirectURL := fmt.Sprintf("/vm/details/%s?success=1&success_msg=%s&refresh=1", vmidStr, url.QueryEscape(successMsg))
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}
