package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/security"
	"pvmss/state"
	"pvmss/utils"
)

// handleVMCreation processes the VM creation form submission.
func (h *VMCreateOptimizedHandler) handleVMCreation(w http.ResponseWriter, r *http.Request, data map[string]interface{}) {
	log := CreateHandlerLogger("handleVMCreation", r)
	ctx := r.Context()

	bridgeName := strings.TrimSpace(r.FormValue("bridge_0"))
	description := strings.TrimSpace(r.FormValue("description"))
	isoImage := strings.TrimSpace(r.FormValue("iso"))
	macAddress := strings.TrimSpace(r.FormValue("mac_address_0"))
	mtu := strings.TrimSpace(r.FormValue("mtu_0"))
	name := strings.TrimSpace(r.FormValue("name"))
	networkModel := strings.TrimSpace(r.FormValue("network_model_0"))
	node := strings.TrimSpace(r.FormValue("node"))
	pool := strings.TrimSpace(r.FormValue("pool"))
	rateLimit := strings.TrimSpace(r.FormValue("rate_limit_0"))
	storage := strings.TrimSpace(r.FormValue("storage"))
	vlanTag := strings.TrimSpace(r.FormValue("vlan_tag_0"))
	vmidStr := strings.TrimSpace(r.FormValue("vmid"))

	if macAddress != "" && !utils.ValidateMACAddress(macAddress) {
		h.preserveNetworkCardFormData(r, data)
		data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "VM.Create.Validation.InvalidMACAddress")
		renderVMCreateTempl(w, r, data)
		return
	}
	if vlanTag != "" {
		if vlanID, err := strconv.Atoi(vlanTag); err != nil || vlanID < 1 || vlanID > 4096 {
			h.preserveNetworkCardFormData(r, data)
			data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "Validation.VLANRange")
			renderVMCreateTempl(w, r, data)
			return
		}
	}
	if rateLimit != "" {
		if rate, err := strconv.ParseFloat(rateLimit, 64); err != nil || rate < 1 || rate > 10240 {
			h.preserveNetworkCardFormData(r, data)
			data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "Validation.RateLimitRange")
			renderVMCreateTempl(w, r, data)
			return
		}
	}
	macAddress = utils.NormalizeMACAddress(macAddress)
	diskBus := strings.TrimSpace(r.FormValue("disk_bus_type"))

	selectedTags := r.Form["tags"]
	var tags string
	if len(selectedTags) > 0 {
		cleanedTags := make([]string, 0, len(selectedTags))
		for _, tag := range selectedTags {
			if cleaned := strings.TrimSpace(tag); cleaned != "" {
				cleanedTags = append(cleanedTags, cleaned)
			}
		}
		tags = strings.Join(cleanedTags, ";")
		log.Debug().Strs("selected_tags", selectedTags).Str("tags_joined", tags).Msg("Tags extracted from form")
	}

	enableEFI := r.FormValue("enable_efi")
	enableTPM := r.FormValue("enable_tpm")

	memoryStr := strings.TrimSpace(r.FormValue("memory"))
	memoryUnit := strings.TrimSpace(r.FormValue("memory_unit"))
	cpuSocketsStr := strings.TrimSpace(r.FormValue("sockets"))
	cpuCoresStr := strings.TrimSpace(r.FormValue("cores"))
	diskSizeStr := strings.TrimSpace(r.FormValue("disk_size_0"))
	diskSizeUnit := strings.TrimSpace(r.FormValue("disk_size_unit_0"))

	if name == "" {
		data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "VM.Create.Validation.VMNameRequired")
		renderVMCreateTempl(w, r, data)
		return
	}

	if macAddress != "" && !ValidateMACAddress(macAddress) {
		data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "VM.Create.Validation.InvalidMACAddress")
		renderVMCreateTempl(w, r, data)
		return
	}

	if storage == "" {
		data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "VM.Create.Validation.StorageRequired")
		renderVMCreateTempl(w, r, data)
		return
	}

	log.Debug().Str("selected_storage", storage).Msg("Validating selected storage")

	settings := h.stateManager.GetSettings()
	if settings == nil {
		log.Error().Msg("Settings not available for VM creation")
		data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.SettingsUnavailable")
		renderVMCreateTempl(w, r, data)
		return
	}

	extractInt := func(str string, defaultValue int, minVal, maxVal int, fieldName string) int {
		if str == "" {
			return defaultValue
		}
		cleanStr := strings.TrimSpace(str)
		var value int
		if val, err := fmt.Sscanf(cleanStr, "%d", &value); err != nil || val != 1 {
			log.Warn().Str("field", fieldName).Str("input", str).Int("default", defaultValue).Msg("Invalid numeric value, using default")
			return defaultValue
		}
		if value < minVal || value > maxVal {
			log.Warn().Str("field", fieldName).Int("value", value).Int("min", minVal).Int("max", maxVal).Int("default", defaultValue).Msg("Value out of range, using default")
			return defaultValue
		}
		return value
	}

	extractSize := func(str string, unit string, defaultValue int, minVal, maxVal int, fieldName string) int {
		if str == "" {
			return defaultValue
		}
		cleanStr := strings.TrimSpace(str)
		var value float64
		if val, err := fmt.Sscanf(cleanStr, "%f", &value); err != nil || val != 1 {
			log.Warn().Str("component", "vm_create").Str("operation", "validate_numeric_field").Str("field", fieldName).Str("input", str).Str("unit", unit).Int("default", defaultValue).Msg("Invalid numeric value, using default")
			return defaultValue
		}
		var mb int
		if unit == "GB" {
			mb = int(value * 1024)
		} else {
			mb = int(value)
		}
		if mb < minVal || mb > maxVal {
			log.Warn().Str("component", "vm_create").Str("operation", "validate_numeric_field").Str("field", fieldName).Str("unit", unit).Float64("value", value).Int("mb", mb).Int("min", minVal).Int("max", maxVal).Int("default", defaultValue).Msg("Value out of range, using default")
			return defaultValue
		}
		return mb
	}

	var memoryMin, memoryMax int
	var socketsMin, socketsMax int
	var coresMin, coresMax int
	var diskMin, diskMax int

	if settings.Limits.VM.Sockets.Min > 0 {
		memoryMin = settings.Limits.VM.RAM.Min * 1024
		memoryMax = settings.Limits.VM.RAM.Max * 1024
		socketsMin = settings.Limits.VM.Sockets.Min
		socketsMax = settings.Limits.VM.Sockets.Max
		coresMin = settings.Limits.VM.Cores.Min
		coresMax = settings.Limits.VM.Cores.Max
		diskMin = settings.Limits.VM.Disk.Min
		diskMax = settings.Limits.VM.Disk.Max
	} else {
		log.Error().Msg("Settings or limits not available for VM creation POST")
		data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.SystemUnavailable")
		renderVMCreateTempl(w, r, data)
		return
	}

	if memoryMin == 0 || memoryMax == 0 || coresMin == 0 || coresMax == 0 || socketsMin == 0 || socketsMax == 0 || diskMin == 0 || diskMax == 0 {
		log.Error().Msg("Incomplete VM limits in settings.json for POST")
		data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.SystemUnavailable")
		renderVMCreateTempl(w, r, data)
		return
	}

	maxVMPerUser := settings.MaxVMPerUser
	if maxVMPerUser > 0 && pool != "" {
		currentVMCount, err := countVMsInPool(ctx, pool)
		if err != nil {
			log.Warn().Err(err).Str("component", "vm_create").Str("operation", "check_pool_limit").Str("pool", pool).Str("reason", "pool_count_failed").Msg("Failed to count VMs in pool; skipping limit check")
		} else {
			log.Info().Str("pool", pool).Int("current_vms", currentVMCount).Int("max_allowed", maxVMPerUser).Msg("Checking user VM limit")
			if currentVMCount >= maxVMPerUser {
				log.Warn().Str("component", "vm_create").Str("operation", "check_pool_limit").Str("pool", pool).Str("reason", "max_vm_limit_reached").Int("current_vms", currentVMCount).Int("max_allowed", maxVMPerUser).Msg("User has reached maximum VM limit")
				localizer := i18n.GetLocalizerFromRequest(r)
				errorMsg := i18n.Localize(localizer, "VM.Create.Error.MaxVMPerUserReached", currentVMCount, maxVMPerUser)
				data["ValidationError"] = errorMsg
				renderVMCreateTempl(w, r, data)
				return
			}
		}
	}

	memoryMB := extractSize(memoryStr, memoryUnit, memoryMin, memoryMin, memoryMax, "memory")
	cpuSockets := extractInt(cpuSocketsStr, socketsMin, socketsMin, socketsMax, "sockets")
	cpuCores := extractInt(cpuCoresStr, coresMin, coresMin, coresMax, "cores")
	diskSizeMB := extractSize(diskSizeStr, diskSizeUnit, diskMin*1024, diskMin*1024, diskMax*1024, "disk_size")

	if diskBus == "" {
		diskBus = "virtio"
	}
	if networkModel == "" {
		networkModel = "virtio"
	}

	vmid := 0
	if vmidStr != "" {
		if parsed, err := strconv.Atoi(vmidStr); err == nil && parsed > 0 {
			vmid = parsed
		}
	}
	if vmid == 0 {
		if snapshot := h.stateManager.GetProxmoxSnapshot(); snapshot != nil && len(snapshot.VMs) > 0 {
			highest := 0
			for _, svm := range snapshot.VMs {
				if svm.VMID > highest {
					highest = svm.VMID
				}
			}
			if highest > 0 {
				vmid = highest + 1
			}
		}
	}
	if vmid == 0 {
		restyClient, err := getDefaultRestyClient()
		if err != nil {
			data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "VM.Create.Error.FailedToGetNextVMID")
			renderVMCreateTempl(w, r, data)
			return
		}
		nextID, err := proxmox.GetNextVMIDResty(ctx, restyClient)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get next VMID")
			data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "VM.Create.Error.FailedToGetNextVMID")
			renderVMCreateTempl(w, r, data)
			return
		}
		vmid = nextID
	}

	params := url.Values{}
	params.Set("vmid", fmt.Sprintf("%d", vmid))
	params.Set("name", name)
	params.Set("memory", fmt.Sprintf("%d", memoryMB))
	params.Set("sockets", fmt.Sprintf("%d", cpuSockets))
	params.Set("cores", fmt.Sprintf("%d", cpuCores))
	params.Set("cpu", "host")

	if description != "" {
		params.Set("description", description)
	}
	if pool != "" {
		params.Set("pool", pool)
	}
	if tags != "" {
		params.Set("tags", tags)
	}

	if isoImage != "" {
		params.Set("ide2", isoImage+",media=cdrom")
		params.Set("boot", "order=ide2;"+diskBus+"0")
	} else {
		params.Set("boot", "order="+diskBus+"0")
	}

	if enableEFI == "1" {
		params.Set("bios", "ovmf")
		params.Set("efidisk0", storage+":1,format=raw,efitype=4m")
	}

	diskParam := fmt.Sprintf("%s0", diskBus)
	params.Set(diskParam, fmt.Sprintf("%s:%d", storage, diskSizeMB/1024))
	log.Info().Str("disk_param", diskParam).Int("size_mb", diskSizeMB).Int("size_gb", diskSizeMB/1024).Msg("Configured primary disk")

	if enableTPM == "1" {
		log.Debug().Str("storage", storage).Bool("enable_tpm", true).Msg("TPM requested for VM creation")
		log.Info().Msg("TPM requested, checking storage compatibility")

		var selectedStorageType string
		if snapshot := h.stateManager.GetProxmoxSnapshot(); snapshot != nil {
			for _, st := range snapshot.GlobalStorages {
				if st.Storage == storage {
					selectedStorageType = st.Type
					break
				}
			}
		}

		configureTPM := func(storageType string) {
			log.Debug().Str("storage", storage).Str("storage_type", storageType).Msg("Checking TPM storage compatibility")
			format, compatible := getTPMDiskFormat(storageType)
			if compatible {
				tpmParam := fmt.Sprintf("%s:4,version=v2.0", storage)
				params.Set("tpmstate0", tpmParam)
				log.Info().Str("storage", storage).Str("storage_type", storageType).Str("format", format).Msg("TPM disk configured successfully")
				log.Debug().Str("storage", storage).Str("tpm_param", tpmParam).Str("tpmstate0", fmt.Sprintf("%s:4,version=v2.0", storage)).Msg("TPM disk parameter details")
			} else {
				log.Warn().Str("storage", storage).Str("storage_type", storageType).Msg("Storage type not compatible with TPM (requires raw format support), skipping TPM")
			}
		}

		if selectedStorageType != "" {
			configureTPM(selectedStorageType)
		} else {
			restyClient, err := getDefaultRestyClient()
			if err != nil {
				log.Warn().Err(err).Str("component", "vm_create").Str("operation", "validate_tpm_storage").Str("reason", "resty_client_failed").Msg("Failed to create resty client for TPM storage check; skipping TPM")
			} else {
				storageInfo, err := proxmox.GetStoragesResty(r.Context(), restyClient)
				if err != nil {
					log.Warn().Err(err).Str("component", "vm_create").Str("operation", "validate_tpm_storage").Str("reason", "storage_info_failed").Msg("Failed to retrieve storage info for TPM; skipping TPM")
				} else {
					var liveType string
					for i := range storageInfo {
						if storageInfo[i].Storage == storage {
							liveType = storageInfo[i].Type
							break
						}
					}
					if liveType != "" {
						configureTPM(liveType)
					} else {
						log.Warn().Str("component", "vm_create").Str("operation", "validate_tpm_storage").Str("reason", "storage_not_found").Str("storage", storage).Msg("Selected storage not found in storage list; skipping TPM")
					}
				}
			}
		}
	}

	if settings.MaxDiskPerVM > 1 {
		for diskIdx := 1; diskIdx < settings.MaxDiskPerVM; diskIdx++ {
			diskSizeStr := strings.TrimSpace(r.FormValue(fmt.Sprintf("disk_size_%d", diskIdx)))
			diskSizeUnit := strings.TrimSpace(r.FormValue(fmt.Sprintf("disk_size_unit_%d", diskIdx)))
			if diskSizeStr == "" || diskSizeStr == "0" {
				continue
			}
			additionalDiskSizeMB := extractSize(diskSizeStr, diskSizeUnit, 0, 1024, 1048576, fmt.Sprintf("disk_size_%d", diskIdx))
			if additionalDiskSizeMB <= 0 {
				log.Warn().Int("disk_idx", diskIdx).Str("component", "vm_create").Str("operation", "configure_additional_disks").Str("reason", "invalid_disk_size").Str("size", diskSizeStr).Str("unit", diskSizeUnit).Msg("Invalid additional disk size; skipping")
				continue
			}
			additionalDiskParam := fmt.Sprintf("%s%d", diskBus, diskIdx)
			params.Set(additionalDiskParam, fmt.Sprintf("%s:%d", storage, additionalDiskSizeMB/1024))
			log.Info().Int("disk_idx", diskIdx).Int("size_mb", additionalDiskSizeMB).Int("size_gb", additionalDiskSizeMB/1024).Str("param", additionalDiskParam).Msg("Added additional disk")
		}
	}

	log.Info().Str("bridgeName", bridgeName).Msg("Starting network card configuration")
	if bridgeName != "" {
		var netConfig string
		if macAddress != "" {
			netConfig = networkModel + "=" + macAddress + ",bridge=" + bridgeName
		} else {
			netConfig = networkModel + ",bridge=" + bridgeName
		}
		if vlanTag != "" {
			netConfig += ",tag=" + vlanTag
			log.Info().Str("vlanTag", vlanTag).Msg("VLAN tag added to network configuration")
		}
		if rateLimit != "" {
			netConfig += ",rate=" + rateLimit
			log.Info().Str("rateLimit", rateLimit).Msg("Rate limit added to network configuration")
		}
		if mtu != "" {
			netConfig += ",mtu=" + mtu
			log.Info().Str("mtu", mtu).Msg("MTU added to network configuration")
		}
		networkEnabled := r.FormValue("network_enabled_0") == "1"
		if !networkEnabled {
			netConfig += ",link_down=1"
		}
		params.Set("net0", netConfig)
		log.Info().Str("bridge", bridgeName).Str("model", networkModel).Str("mac", macAddress).Str("vlan", vlanTag).Str("rate", rateLimit).Str("mtu", mtu).Bool("enabled", networkEnabled).Str("netConfig", netConfig).Msg("Configured primary network card")
	} else {
		log.Warn().Str("component", "vm_create").Str("operation", "configure_network_cards").Str("reason", "bridge_name_empty").Msg("Bridge name is empty; skipping network card configuration")
	}

	if settings.MaxNetworkCards > 1 {
		for netIdx := 1; netIdx < settings.MaxNetworkCards; netIdx++ {
			additionalBridge := strings.TrimSpace(r.FormValue(fmt.Sprintf("bridge_%d", netIdx)))
			if additionalBridge == "" {
				continue
			}
			additionalModel := strings.TrimSpace(r.FormValue(fmt.Sprintf("network_model_%d", netIdx)))
			if additionalModel == "" {
				additionalModel = "virtio"
			}
			additionalMAC := strings.TrimSpace(r.FormValue(fmt.Sprintf("mac_address_%d", netIdx)))
			additionalVLANTag := strings.TrimSpace(r.FormValue(fmt.Sprintf("vlan_tag_%d", netIdx)))
			additionalRateLimit := strings.TrimSpace(r.FormValue(fmt.Sprintf("rate_limit_%d", netIdx)))
			additionalMTU := strings.TrimSpace(r.FormValue(fmt.Sprintf("mtu_%d", netIdx)))
			if additionalMAC != "" && !utils.ValidateMACAddress(additionalMAC) {
				h.preserveNetworkCardFormData(r, data)
				data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "VM.Create.Validation.InvalidMACAddress")
				renderVMCreateTempl(w, r, data)
				return
			}
			if additionalVLANTag != "" {
				if vlanID, err := strconv.Atoi(additionalVLANTag); err != nil || vlanID < 1 || vlanID > 4096 {
					h.preserveNetworkCardFormData(r, data)
					data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "Validation.VLANRange")
					renderVMCreateTempl(w, r, data)
					return
				}
			}
			if additionalRateLimit != "" {
				if rate, err := strconv.ParseFloat(additionalRateLimit, 64); err != nil || rate < 1 || rate > 10240 {
					h.preserveNetworkCardFormData(r, data)
					data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "Validation.RateLimitRange")
					renderVMCreateTempl(w, r, data)
					return
				}
			}
			if additionalMTU != "" {
				if mtuVal, err := strconv.Atoi(additionalMTU); err != nil || mtuVal < 576 || mtuVal > 9000 {
					h.preserveNetworkCardFormData(r, data)
					data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "Validation.MTURange")
					renderVMCreateTempl(w, r, data)
					return
				}
			}
			additionalMAC = utils.NormalizeMACAddress(additionalMAC)
			additionalEnabled := r.FormValue(fmt.Sprintf("network_enabled_%d", netIdx)) == "1"
			var additionalNetConfig string
			if additionalMAC != "" {
				additionalNetConfig = additionalModel + "=" + additionalMAC + ",bridge=" + additionalBridge
			} else {
				additionalNetConfig = additionalModel + ",bridge=" + additionalBridge
			}
			if additionalVLANTag != "" {
				additionalNetConfig += ",tag=" + additionalVLANTag
			}
			if additionalRateLimit != "" {
				additionalNetConfig += ",rate=" + additionalRateLimit
			}
			if additionalMTU != "" {
				additionalNetConfig += ",mtu=" + additionalMTU
			}
			if !additionalEnabled {
				additionalNetConfig += ",link_down=1"
			}
			netParam := fmt.Sprintf("net%d", netIdx)
			params.Set(netParam, additionalNetConfig)
			log.Info().Int("net_idx", netIdx).Str("bridge", additionalBridge).Str("model", additionalModel).Str("vlan", additionalVLANTag).Str("rate", additionalRateLimit).Str("mtu", additionalMTU).Bool("enabled", additionalEnabled).Msg("Added additional network card")
		}
	}

	params.Set("agent", "1")

	localizer := i18n.GetLocalizerFromRequest(r)
	if err := ValidateVMResourcesAgainstNodeLimits(ctx, h.stateManager, node, cpuSockets, cpuCores, memoryMB, localizer); err != nil {
		log.Warn().Err(err).Str("component", "vm_create").Str("operation", "validate_node_limits").Str("node", node).Str("reason", "resource_limits_exceeded").Msg("VM resources exceed aggregate node limits")
		data["ValidationError"] = err.Error()
		renderVMCreateTempl(w, r, data)
		return
	}

	path := "/nodes/" + url.PathEscape(node) + "/qemu"
	log.Info().Str("path", path).Str("params", params.Encode()).Msg("Sending VM creation request to Proxmox API")

	restyClient, restyErr := getDefaultRestyClient()
	if restyErr != nil {
		data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.ProxmoxClientUnavailable")
		renderVMCreateTempl(w, r, data)
		return
	}
	var createResp interface{}
	if err := restyClient.Post(ctx, path, params, &createResp); err != nil {
		logger.VMFailure("vm_create", vmid, node, "proxmox_api_error").Err(err).Str("vm_name", name).Int("cpu_sockets", cpuSockets).Int("cpu_cores", cpuCores).Int("memory_mb", memoryMB).Int("disk_gb", diskSizeMB/1024).Str("storage", storage).Str("network_model", networkModel).Str("pool", pool).Str("client_ip", r.RemoteAddr).Msg("VM creation failed")
		data["ValidationError"] = fmt.Sprintf("Failed to create VM: %v", err)
		renderVMCreateTempl(w, r, data)
		return
	}

	cloudInitEnabled := r.FormValue("cloudinit_enable") == "1"
	cloudInitWarning := ""
	if cloudInitEnabled {
		cloudInitWarning = h.applyCloudInitConfig(ctx, r, node, vmid, storage)
	}

	startVM := r.FormValue("start_vm") == "1"
	if startVM {
		log.Info().Int("vmid", vmid).Str("node", node).Msg("Starting VM after creation")
		if err := h.startVM(ctx, node, vmid); err != nil {
			log.Warn().Err(err).Int("vmid", vmid).Str("node", node).Msg("Failed to start VM after creation")
		} else {
			log.Info().Int("vmid", vmid).Str("node", node).Msg("VM started successfully")
		}
	}

	username := ""
	isAdmin := false
	if sessionManager := security.GetSession(r); sessionManager != nil {
		if user, ok := sessionManager.Get(r.Context(), "username").(string); ok {
			username = user
		}
		if admin, ok := sessionManager.Get(r.Context(), "is_admin").(bool); ok {
			isAdmin = admin
		}
	}

	logger.VMEvent("vm_create", vmid, node).Str("vm_name", name).Str("username", username).Bool("is_admin", isAdmin).Int("cpu_sockets", cpuSockets).Int("cpu_cores", cpuCores).Int("memory_mb", memoryMB).Int("disk_gb", diskSizeMB/1024).Str("storage", storage).Str("network_model", networkModel).Str("pool", pool).Str("client_ip", r.RemoteAddr).Msg("VM created successfully")

	time.Sleep(2 * time.Second)

	redirectURL := fmt.Sprintf("/vm/details/%d?created=1&refresh=1", vmid)
	if cloudInitWarning != "" {
		redirectURL += "&ci_warning=" + url.QueryEscape(cloudInitWarning)
	}

	redirectURL = strings.ReplaceAll(redirectURL, "\\", "/")
	parsedURL, err := url.Parse(redirectURL)
	if err != nil || parsedURL.Hostname() != "" {
		log.Warn().Str("redirect_url", redirectURL).Msg("Invalid redirect URL, falling back to profile")
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, parsedURL.String(), http.StatusSeeOther)
}

// startVM starts a VM after creation.
func (h *VMCreateOptimizedHandler) startVM(ctx context.Context, node string, vmid int) error {
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		return fmt.Errorf("failed to create resty client: %w", err)
	}
	_, err = proxmox.VMActionResty(ctx, restyClient, node, strconv.Itoa(vmid), "start")
	return err
}

// applyCloudInitConfig applies cloud-init configuration to a newly created VM.
// Returns a warning message if the cloud-init template upload failed.
func (h *VMCreateOptimizedHandler) applyCloudInitConfig(ctx context.Context, r *http.Request, node string, vmid int, storage string) string {
	log := CreateHandlerLogger("applyCloudInitConfig", r)
	warning := ""

	ciDNS := strings.TrimSpace(r.FormValue("cloudinit_dns"))
	ciGateway := strings.TrimSpace(r.FormValue("cloudinit_gateway"))
	ciIP := strings.TrimSpace(r.FormValue("cloudinit_ip"))
	ciIPConfig := strings.TrimSpace(r.FormValue("cloudinit_ipconfig"))
	ciPassword := r.FormValue("cloudinit_password")
	ciSSHKeys := strings.TrimSpace(r.FormValue("cloudinit_sshkeys"))
	ciUser := strings.TrimSpace(r.FormValue("cloudinit_user"))
	templateID := strings.TrimSpace(r.FormValue("cloudinit_template"))

	ciParams := proxmox.CloudInitParams{CIUser: ciUser}
	if ciPassword != "" {
		ciParams.CIPassword = ciPassword
	}
	if ciSSHKeys != "" {
		ciParams.SSHKeys = ciSSHKeys
	}
	if ciIPConfig == "static" && ciIP != "" {
		ipConfig := "ip=" + ciIP
		if ciGateway != "" {
			ipConfig += ",gw=" + ciGateway
		}
		ciParams.IPConfig0 = ipConfig
	} else {
		ciParams.IPConfig0 = "ip=dhcp"
	}
	if ciDNS != "" {
		ciParams.Nameserver = ciDNS
	}

	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create resty client for cloud-init")
		return "resty-client-failed"
	}

	if templateID != "" {
		templateWarning := h.applyCloudInitTemplate(ctx, restyClient, node, vmid, &ciParams, templateID)
		if templateWarning != "" {
			warning = templateWarning
		}
	}

	if err := proxmox.EnsureCloudInitDriveResty(ctx, restyClient, node, vmid, storage); err != nil {
		log.Warn().Err(err).Str("node", node).Int("vmid", vmid).Str("storage", storage).Msg("Failed to ensure cloud-init drive, continuing with config")
	}

	if err := proxmox.UpdateVMCloudInitConfigResty(ctx, restyClient, node, vmid, ciParams); err != nil {
		log.Error().Err(err).Str("node", node).Int("vmid", vmid).Msg("Failed to apply cloud-init configuration")
		return "cloud-init-config-failed"
	}

	log.Info().Str("node", node).Int("vmid", vmid).Str("ci_user", ciUser).Bool("has_ssh_keys", ciSSHKeys != "").Bool("has_password", ciPassword != "").Str("ip_config", ciParams.IPConfig0).Str("cicustom", ciParams.CICustom).Msg("Cloud-init configuration applied successfully")

	return warning
}

// applyCloudInitTemplate uploads a cloud-init template to Proxmox storage and configures cicustom.
// Returns a warning message if the upload failed, empty string on success.
func (h *VMCreateOptimizedHandler) applyCloudInitTemplate(ctx context.Context, restyClient *proxmox.RestyClient, node string, vmid int, ciParams *proxmox.CloudInitParams, templateID string) string {
	log := CreateHandlerLogger("applyCloudInitTemplate", nil)

	settings := h.stateManager.GetSettings()
	if settings == nil {
		log.Warn().Msg("Settings not available, skipping cloud-init template")
		return "settings-unavailable"
	}

	template := settings.GetCloudInitTemplateByID(templateID)
	if template == nil {
		log.Warn().Str("template_id", templateID).Msg("Cloud-init template not found, skipping")
		return "template-not-found"
	}

	if strings.TrimSpace(template.YAMLContent) == "" {
		log.Warn().Str("template_id", templateID).Msg("Cloud-init template has empty YAML content, skipping")
		return "template-empty"
	}

	snippetStorage, err := h.selectSnippetStorageForNode(ctx, restyClient, node, template.Storage)
	if err != nil {
		log.Warn().Err(err).Str("template_id", templateID).Str("node", node).Msg("No suitable snippets storage found for cloud-init template, skipping")
		return "no-snippets-storage"
	}

	filename := fmt.Sprintf("%s%d.yml", state.CloudInitTemplatePrefix, vmid)

	uploadSuccess := false
	if settings.CloudInitSFTP.Enabled {
		log.Info().Msg("Attempting cloud-init snippet upload via SFTP")
		if err := proxmox.UploadSnippetFileSFTP(ctx, settings.CloudInitSFTP, filename, template.YAMLContent); err != nil {
			log.Warn().Err(err).Str("template_id", templateID).Str("sftp_host", settings.CloudInitSFTP.Host).Str("filename", filename).Msg("Failed to upload cloud-init template snippet via SFTP, falling back to HTTP API")
		} else {
			uploadSuccess = true
			log.Info().Str("template_id", templateID).Str("sftp_host", settings.CloudInitSFTP.Host).Str("filename", filename).Msg("Cloud-init template snippet uploaded successfully via SFTP")
		}
	}

	if !uploadSuccess {
		log.Info().Msg("Attempting cloud-init snippet upload via HTTP API")
		if err := proxmox.UploadSnippetFileResty(ctx, restyClient, node, snippetStorage, filename, template.YAMLContent); err != nil {
			log.Warn().Err(err).Str("template_id", templateID).Str("storage", snippetStorage).Str("filename", filename).Msg("Failed to upload cloud-init template snippet via both SFTP and HTTP API, skipping cicustom")
			return "upload-failed"
		}
		log.Info().Str("template_id", templateID).Str("storage", snippetStorage).Str("filename", filename).Msg("Cloud-init template snippet uploaded successfully via HTTP API")
	}

	ciParams.CICustom = fmt.Sprintf("user=%s:snippets/%s", snippetStorage, filename)

	log.Info().Str("template_id", templateID).Str("storage", snippetStorage).Str("filename", filename).Str("node", node).Int("vmid", vmid).Msg("Cloud-init template snippet uploaded and cicustom configured")

	return ""
}

// selectSnippetStorageForNode picks a snippets storage available on a specific node (or preferred storage).
func (h *VMCreateOptimizedHandler) selectSnippetStorageForNode(ctx context.Context, restyClient *proxmox.RestyClient, node string, preferredStorage string) (string, error) {
	storages, err := proxmox.GetSnippetsStoragesResty(ctx, restyClient)
	if err != nil {
		return "", err
	}

	var fallback string

	for _, s := range storages {
		if !storageAvailableOnNode(s, node) {
			continue
		}
		if preferredStorage != "" && s.Storage == preferredStorage {
			return s.Storage, nil
		}
		if fallback == "" {
			fallback = s.Storage
		}
	}

	if fallback == "" {
		if preferredStorage != "" {
			return "", fmt.Errorf("no snippets storage matching preferred %s for node %s", preferredStorage, node)
		}
		return "", fmt.Errorf("no snippets storage available for node %s", node)
	}

	return fallback, nil
}

// storageAvailableOnNode checks if a storage is available on the given node.
func storageAvailableOnNode(storage proxmox.Storage, node string) bool {
	if storage.Nodes == "" {
		return true
	}
	for _, n := range strings.Split(storage.Nodes, ",") {
		if strings.TrimSpace(n) == node {
			return true
		}
	}
	return false
}

// preserveNetworkCardFormData preserves dynamic network card fields during validation errors.
func (h *VMCreateOptimizedHandler) preserveNetworkCardFormData(r *http.Request, data map[string]interface{}) {
	settings := h.stateManager.GetSettings()
	if settings == nil {
		return
	}

	maxNetworkCards := settings.MaxNetworkCards
	if maxNetworkCards <= 0 {
		maxNetworkCards = 1
	}

	formData, ok := data["FormData"].(map[string]string)
	if !ok {
		formData = make(map[string]string)
		data["FormData"] = formData
	}

	for netIdx := 0; netIdx < maxNetworkCards; netIdx++ {
		if bridge := r.FormValue(fmt.Sprintf("bridge_%d", netIdx)); bridge != "" {
			formData[fmt.Sprintf("bridge_%d", netIdx)] = bridge
		}
		if mac := r.FormValue(fmt.Sprintf("mac_address_%d", netIdx)); mac != "" {
			formData[fmt.Sprintf("mac_address_%d", netIdx)] = mac
		}
		if vlan := r.FormValue(fmt.Sprintf("vlan_tag_%d", netIdx)); vlan != "" {
			formData[fmt.Sprintf("vlan_tag_%d", netIdx)] = vlan
		}
		if model := r.FormValue(fmt.Sprintf("network_model_%d", netIdx)); model != "" {
			formData[fmt.Sprintf("network_model_%d", netIdx)] = model
		}
		if enabled := r.FormValue(fmt.Sprintf("network_enabled_%d", netIdx)); enabled != "" {
			formData[fmt.Sprintf("network_enabled_%d", netIdx)] = enabled
		}
		if rate := r.FormValue(fmt.Sprintf("rate_limit_%d", netIdx)); rate != "" {
			formData[fmt.Sprintf("rate_limit_%d", netIdx)] = rate
		}
		if mtu := r.FormValue(fmt.Sprintf("mtu_%d", netIdx)); mtu != "" {
			formData[fmt.Sprintf("mtu_%d", netIdx)] = mtu
		}
	}
}

// getTPMDiskFormat returns the TPM disk format and compatibility for a given storage type.
func getTPMDiskFormat(storageType string) (string, bool) {
	blockStorages := map[string]bool{
		"iscsi":   true,
		"lvm":     true,
		"lvmthin": true,
		"rbd":     true,
		"zfs":     true,
	}

	fileStorages := map[string]bool{
		"cephfs": true,
		"cifs":   true,
		"dir":    true,
		"nfs":    true,
	}

	if blockStorages[storageType] || fileStorages[storageType] {
		logger.Get().Debug().Str("storage_type", storageType).Bool("is_block_storage", blockStorages[storageType]).Bool("is_file_storage", fileStorages[storageType]).Msg("Storage type compatible with TPM raw format")
		return "raw", true
	}

	logger.Get().Debug().Str("storage_type", storageType).Msg("Storage type NOT compatible with TPM raw format")
	return "raw", false
}

// countDisabledNodes counts disabled nodes in a map.
func countDisabledNodes(disabledNodes map[string]bool) int {
	count := 0
	for _, disabled := range disabledNodes {
		if disabled {
			count++
		}
	}
	return count
}
