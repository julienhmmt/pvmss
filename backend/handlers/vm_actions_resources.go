package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"

	"pvmss/proxmox"
	"pvmss/utils"
)

// UpdateVMResourcesHandler updates VM resources (CPU sockets/cores, memory, network bridge).
func (h *VMHandler) UpdateVMResourcesHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := HandlerContextWith(w, r, "UpdateVMResourcesHandler")

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	coresStr := strings.TrimSpace(r.FormValue("cores"))
	memoryStr := strings.TrimSpace(r.FormValue("memory"))
	memoryUnit := strings.TrimSpace(r.FormValue("memory_unit"))
	node := strings.TrimSpace(r.FormValue("node"))
	socketsStr := strings.TrimSpace(r.FormValue("sockets"))
	vmid := strings.TrimSpace(r.FormValue("vmid"))

	// Disk resize parameters
	diskResizeDisk := strings.TrimSpace(r.FormValue("disk_resize_disk"))
	diskResizeGB := strings.TrimSpace(r.FormValue("disk_resize_gb"))

	if vmid == "" || node == "" {
		ctx.HandleError(nil, "Bad request", http.StatusBadRequest)
		return
	}

	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		ctx.HandleError(err, "Invalid VM ID", http.StatusBadRequest)
		return
	}

	// Strict validation: Both fields must be present if either is provided
	if (diskResizeDisk != "" && diskResizeGB == "") || (diskResizeDisk == "" && diskResizeGB != "") {
		ctx.Log.Warn().
			Str("component", "vm_actions").
			Str("operation", "validate_disk_resize").
			Str("reason", "incomplete_parameters").
			Str("disk", diskResizeDisk).
			Str("gb", diskResizeGB).
			Msg("Incomplete disk resize parameters")
		ctx.RedirectWithError(fmt.Sprintf("/vm/details/%d?edit=resources", vmidInt), "Error.InvalidInput")
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

	// Convert memory to MB based on unit
	var memoryMB int64
	if memoryUnit == "GB" {
		memoryMB = memory * 1024
	} else {
		memoryMB = memory
	}

	// Parse and validate disk resize parameters
	var diskResizeGBInt int64
	var performDiskResize bool
	if diskResizeDisk != "" && diskResizeGB != "" {
		performDiskResize = true
		diskResizeGBInt, err = strconv.ParseInt(diskResizeGB, 10, 64)
		if err != nil || diskResizeGBInt < 1 {
			ctx.RedirectWithError(fmt.Sprintf("/vm/details/%d?edit=resources", vmidInt), "Error.InvalidInput")
			return
		}

		ctx.Log.Info().Str("disk", diskResizeDisk).Int64("increment_gb", diskResizeGBInt).Msg("Disk resize parameters parsed")
	}

	stateManager := getStateManager(r)
	if stateManager == nil {
		ctx.HandleError(nil, "State manager not available", http.StatusInternalServerError)
		return
	}

	settings := stateManager.GetSettings()

	// Get memory limits from settings (optional). Stored in GB in settings; convert to MB.
	var vmRamMinMB, vmRamMaxMB int64 = 0, 0
	if settings != nil {
		vmRamMinMB = int64(settings.Limits.VM.RAM.Min * 1024)
		vmRamMaxMB = int64(settings.Limits.VM.RAM.Max * 1024)
	}

	// Validate memory limits only if configured in settings
	if (vmRamMinMB > 0 && memoryMB < vmRamMinMB) || (vmRamMaxMB > 0 && memoryMB > vmRamMaxMB) {
		ctx.RedirectWithError(fmt.Sprintf("/vm/details/%d?edit=resources", vmidInt), "Error.InvalidInput")
		return
	}

	ctx.Log.Info().
		Str("memory_input", memoryStr).
		Str("memory_unit", memoryUnit).
		Int64("memory_mb", memoryMB).
		Int64("min_mb", vmRamMinMB).
		Int64("max_mb", vmRamMaxMB).
		Msg("Memory parsed for VM update")

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
	values.Set("memory", fmt.Sprintf("%d", memoryMB))

	// Handle CD-ROM ISO update
	cdromISO := strings.TrimSpace(r.FormValue("cdrom_iso"))
	if cdromISO != "" {
		values.Set("ide2", cdromISO+",media=cdrom")
		ctx.Log.Info().Str("vmid", vmid).Str("node", node).Str("iso", cdromISO).Msg("Updating CD-ROM ISO")
	} else {
		values.Add("delete", "ide2")
		ctx.Log.Info().Str("vmid", vmid).Str("node", node).Msg("Ejecting CD-ROM ISO")
	}

	deleteTargets := []string{}

	for i := 0; i < maxNetworkCards; i++ {
		bridge := strings.TrimSpace(r.FormValue(fmt.Sprintf("bridge_%d", i)))
		model := strings.TrimSpace(r.FormValue(fmt.Sprintf("network_model_%d", i)))
		mac := strings.TrimSpace(r.FormValue(fmt.Sprintf("mac_address_%d", i)))
		vlan := strings.TrimSpace(r.FormValue(fmt.Sprintf("vlan_tag_%d", i)))
		rate := strings.TrimSpace(r.FormValue(fmt.Sprintf("rate_limit_%d", i)))
		mtu := strings.TrimSpace(r.FormValue(fmt.Sprintf("mtu_%d", i)))

		if mac != "" && !utils.ValidateMACAddress(mac) {
			ctx.RedirectWithError(fmt.Sprintf("/vm/details/%d?edit=resources", vmidInt), "VM.Create.Validation.InvalidMACAddress")
			return
		}
		if vlan != "" {
			if vlanID, err := strconv.Atoi(vlan); err != nil || vlanID < 1 || vlanID > 4096 {
				ctx.RedirectWithError(fmt.Sprintf("/vm/details/%d?edit=resources", vmidInt), "Validation.VLANRange")
				return
			}
		}
		if rate != "" {
			if rVal, err := strconv.ParseFloat(rate, 64); err != nil || rVal < 1 || rVal > 10240 {
				ctx.RedirectWithError(fmt.Sprintf("/vm/details/%d?edit=resources", vmidInt), "Validation.RateLimitRange")
				return
			}
		}
		if mtu != "" {
			if mtuVal, err := strconv.Atoi(mtu); err != nil || mtuVal < 576 || mtuVal > 9000 {
				ctx.RedirectWithError(fmt.Sprintf("/vm/details/%d?edit=resources", vmidInt), "Validation.MTURange")
				return
			}
		}

		mac = utils.NormalizeMACAddress(mac)
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
			ctx.Log.Warn().
				Str("component", "vm_actions").
				Str("operation", "validate_network_model").
				Str("reason", "invalid_model").
				Int("card_index", i).
				Str("network_model", model).
				Msg("Invalid network model; defaulting to virtio")
			model = "virtio"
		}

		netParts := []string{}
		if mac != "" {
			netParts = append(netParts, model+"="+mac)
		} else {
			netParts = append(netParts, model)
		}
		netParts = append(netParts, "bridge="+bridge)
		if vlan != "" {
			netParts = append(netParts, "tag="+vlan)
		}
		if rate != "" {
			netParts = append(netParts, "rate="+rate)
		}
		if mtu != "" {
			netParts = append(netParts, "mtu="+mtu)
		}
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

	ctx.Log.Info().Str("vmid", vmid).Str("node", node).Int("sockets", sockets).Int("cores", cores).Int64("memory", memory).Int("network_cards", maxNetworkCards).Msg("VM resources updated successfully")

	if performDiskResize {
		ctx.Log.Info().Str("disk", diskResizeDisk).Int64("increment_gb", diskResizeGBInt).Msg("Executing disk resize")
		sizeParam := fmt.Sprintf("+%dG", diskResizeGBInt)
		if err := proxmox.ResizeVMDiskResty(r.Context(), restyClient, node, vmidInt, diskResizeDisk, sizeParam); err != nil {
			ctx.Log.Error().Err(err).Str("disk", diskResizeDisk).Str("size", sizeParam).Msg("Disk resize failed")
			ctx.RedirectWithError(buildVMDetailsURL(vmid), "VMDetails.DiskResize.Failed")
			return
		}
		ctx.Log.Info().Str("disk", diskResizeDisk).Str("size", sizeParam).Msg("Disk resize completed successfully")
		vmStatus, err := proxmox.GetVMCurrentResty(r.Context(), restyClient, node, vmidInt)
		if err == nil && vmStatus != nil && vmStatus.Status == "running" {
			ctx.Log.Info().Str("vmid", vmid).Str("node", node).Msg("VM is running, checking QEMU agent availability")
			if getGuestAgentStatus(r, node, vmidInt) == agentStatusAvailable {
				ctx.Log.Info().Msg("QEMU agent is available, executing fstrim")
				fstrimCmd := []string{"fstrim", "-av"}
				if _, err := proxmox.ExecuteQemuAgentCommandResty(r.Context(), restyClient, node, vmidInt, fstrimCmd); err != nil {
					ctx.Log.Warn().Err(err).Str("component", "vm_actions").Str("operation", "disk_resize").Str("reason", "fstrim_failed").Msg("fstrim execution failed, but disk resize succeeded")
				} else {
					ctx.Log.Info().Msg("fstrim executed successfully via QEMU agent")
				}
			} else {
				ctx.Log.Info().Msg("QEMU agent is not available, skipping fstrim execution")
			}
		} else {
			ctx.Log.Info().Msg("VM is not running, skipping fstrim execution")
		}
		InvalidateGuestAgentCache(node, vmidInt)
		ctx.RedirectWithSuccess(buildVMDetailsURL(vmid), "VMDetails.DiskResize.Success")
		return
	}

	InvalidateGuestAgentCache(node, vmidInt)
	ctx.RedirectWithSuccess(buildVMDetailsURL(vmid), "Message.UpdatedSuccessfully")
}

// ToggleNetworkCardHandler toggles a single network card enable/disable state.
func (h *VMHandler) ToggleNetworkCardHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := HandlerContextWith(w, r, "ToggleNetworkCardHandler")
	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	// Check if this is an AJAX request
	isAjax := r.Header.Get("X-Requested-With") == "XMLHttpRequest" || r.Header.Get("Accept") == "application/json"

	vmidStr := strings.TrimSpace(r.FormValue("vmid"))
	node := strings.TrimSpace(r.FormValue("node"))
	cardIndexStr := strings.TrimSpace(r.FormValue("card_index"))
	action := strings.TrimSpace(r.FormValue("action"))
	enabledParam := strings.TrimSpace(r.FormValue("enabled"))
	if vmidStr == "" || node == "" || cardIndexStr == "" || (enabledParam == "" && (action != "enable" && action != "disable")) {
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Bad request"}); err != nil {
				ctx.Log.Error().Err(err).Msg("Failed to encode JSON response for bad request")
			}
		} else {
			ctx.HandleError(nil, "Bad request", http.StatusBadRequest)
		}
		return
	}
	vmidInt, err := strconv.Atoi(vmidStr)
	if err != nil {
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Invalid VM ID"}); err != nil {
				ctx.Log.Error().Err(err).Msg("Failed to encode JSON response for invalid VM ID")
			}
		} else {
			ctx.HandleError(err, "Invalid VM ID", http.StatusBadRequest)
		}
		return
	}
	cardIndex, err := strconv.Atoi(cardIndexStr)
	if err != nil || cardIndex < 0 {
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Invalid card index"}); err != nil {
				ctx.Log.Error().Err(err).Msg("Failed to encode JSON response for invalid card index")
			}
		} else {
			ctx.HandleError(err, "Invalid card index", http.StatusBadRequest)
		}
		return
	}
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Failed to create API client"}); err != nil {
				ctx.Log.Error().Err(err).Msg("Failed to encode JSON response for API client creation error")
			}
		} else {
			ctx.HandleError(err, "Failed to create API client", http.StatusInternalServerError)
		}
		return
	}
	vmConfig, err := proxmox.GetVMConfigResty(r.Context(), restyClient, node, vmidInt)
	if err != nil {
		ctx.Log.Error().Err(err).Msg("Failed to get VM config for network toggle")
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": ctx.Translate("Message.ActionFailed")}); err != nil {
				ctx.Log.Error().Err(err).Msg("Failed to encode JSON response for VM config error")
			}
		} else {
			ctx.RedirectWithError(buildVMDetailsURL(vmidStr), "Message.ActionFailed")
		}
		return
	}
	netKey := fmt.Sprintf("net%d", cardIndex)
	currentConfig := ""
	if vmConfig != nil {
		if netVal, ok := vmConfig[netKey].(string); ok {
			currentConfig = netVal
		}
	}
	if currentConfig == "" {
		ctx.Log.Warn().Str("component", "vm_actions").Str("operation", "network_interface_update").Str("reason", "interface_not_found").Int("card_index", cardIndex).Str("vmid", vmidStr).Msg("Network interface not found")
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": ctx.Translate("Message.ActionFailed")}); err != nil {
				ctx.Log.Error().Err(err).Msg("Failed to encode JSON response for interface not found")
			}
		} else {
			ctx.RedirectWithError(buildVMDetailsURL(vmidStr), "Message.ActionFailed")
		}
		return
	}

	model, mac, bridge, vlan, rate, mtu, options, currentLinkDown := parseNetworkConfig(currentConfig)
	ctx.Log.Info().Str("vmid", vmidStr).Str("node", node).Int("card_index", cardIndex).Str("current_config", currentConfig).Str("model", model).Str("mac", mac).Str("bridge", bridge).Bool("currently_link_down", currentLinkDown).Str("requested_action", action).Msg("Current network config")

	var newLinkDown bool
	if enabledParam != "" {
		newLinkDown = enabledParam != "1"
	} else {
		newLinkDown = (action == "disable")
	}

	if currentLinkDown == newLinkDown {
		ctx.Log.Info().Str("vmid", vmidStr).Int("card_index", cardIndex).Bool("link_down", newLinkDown).Msg("Network card already in requested state, no change needed")
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
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": successMsg}); err != nil {
				ctx.Log.Error().Err(err).Msg("Failed to encode JSON response for no change needed")
			}
		} else {
			redirectURL := fmt.Sprintf("/vm/details/%s?success=1&success_msg=%s", vmidStr, url.QueryEscape(successMsg))
			http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		}
		return
	}

	netParts := []string{}
	if mac != "" {
		netParts = append(netParts, model+"="+mac)
	} else {
		netParts = append(netParts, model)
	}
	netParts = append(netParts, "bridge="+bridge)
	if vlan != "" {
		netParts = append(netParts, "tag="+vlan)
	}
	if rate != "" {
		netParts = append(netParts, "rate="+rate)
	}
	if mtu != "" {
		netParts = append(netParts, "mtu="+mtu)
	}

	filteredOptions := []string{}
	for _, opt := range options {
		if !strings.HasPrefix(opt, "link_down") {
			filteredOptions = append(filteredOptions, opt)
		}
	}
	if newLinkDown {
		netParts = append(netParts, "link_down=1")
	} else {
		netParts = append(netParts, "link_down=0")
	}
	netParts = append(netParts, filteredOptions...)

	newConfig := strings.Join(netParts, ",")
	ctx.Log.Info().Str("vmid", vmidStr).Str("node", node).Int("card_index", cardIndex).Str("old_config", currentConfig).Str("new_config", newConfig).Bool("enabling", action == "enable").Msg("Applying network config change")
	params := map[string]string{netKey: newConfig}
	ctx.Log.Debug().Str("vmid", vmidStr).Str("node", node).Str("param_key", netKey).Str("param_value", newConfig).Msg("Sending update to Proxmox API")
	if err := proxmox.UpdateVMConfigResty(r.Context(), restyClient, node, vmidInt, params); err != nil {
		ctx.Log.Error().Err(err).Str("vmid", vmidStr).Int("card_index", cardIndex).Str("attempted_config", newConfig).Msg("Network toggle failed - Proxmox API error")
		if isAjax {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": ctx.Translate("Message.ActionFailed")}); err != nil {
				ctx.Log.Error().Err(err).Msg("Failed to encode JSON response for Proxmox API error")
			}
		} else {
			ctx.RedirectWithError(buildVMDetailsURL(vmidStr), "Message.ActionFailed")
		}
		return
	}
	ctx.Log.Info().Str("vmid", vmidStr).Str("node", node).Int("card_index", cardIndex).Str("action", action).Bool("link_down", newLinkDown).Msg("Network card state changed successfully in Proxmox")
	InvalidateGuestAgentCache(node, vmidInt)
	ctx.Log.Debug().Str("vmid", vmidStr).Int("vmid_int", vmidInt).Msg("Invalidated guest agent cache")
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
	ctx.Log.Info().Str("vmid", vmidStr).Int("card_index", cardIndex).Str("final_state", action).Msg("Network card toggle completed")
	if isAjax {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": successMsg}); err != nil {
			ctx.Log.Error().Err(err).Msg("Failed to encode JSON response for successful network toggle")
		}
	} else {
		redirectURL := fmt.Sprintf("/vm/details/%s?success=1&success_msg=%s&refresh=1", vmidStr, url.QueryEscape(successMsg))
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
	}
}
