package handlers

import (
	"fmt"
	"net/http"

	"pvmss/components"
	"pvmss/i18n"
	"pvmss/state"
)

// renderVMCreateTempl renders the VM create page using the Templ component.
func renderVMCreateTempl(w http.ResponseWriter, r *http.Request, data map[string]interface{}) {
	log := CreateHandlerLogger("renderVMCreateTempl", r)

	// Convert map data to typed VMCreateData struct
	templData := convertToVMCreateData(data, r)

	// Get translation function (must match components.TranslationFunc signature)
	localizer := i18n.GetLocalizerFromRequest(r)
	T := func(key string) string {
		return i18n.Localize(localizer, key)
	}

	// Render the Templ component
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	setNoCacheHeaders(w)

	err := components.VMCreatePage(templData, T).Render(r.Context(), w)
	if err != nil {
		log.Error().Err(err).Msg("Failed to render VMCreatePage templ component")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// convertToVMCreateData converts the map[string]interface{} data to components.VMCreateData.
func convertToVMCreateData(data map[string]interface{}, _ *http.Request) components.VMCreateData {
	result := components.VMCreateData{
		FormData: components.VMCreateFormData{
			DiskSizes:      make(map[int]string),
			DiskUnits:      make(map[int]string),
			Bridges:        make(map[int]string),
			MACAddresses:   make(map[int]string),
			VLANTags:       make(map[int]string),
			RateLimits:     make(map[int]string),
			MTUs:           make(map[int]string),
			NetworkModels:  make(map[int]string),
			NetworkEnabled: make(map[int]string),
		},
		StorageNodes:       make(map[string]string),
		BridgeDescriptions: make(map[string]string),
		BridgeNodes:        make(map[string]string),
	}

	// Basic fields
	result.Username = getStringFromMap(data, "Username")
	result.Lang = getStringFromMap(data, "Lang")
	result.CSRFToken = getStringFromMap(data, "CSRFToken")
	result.ProxmoxConnected = getBoolFromMap(data, "ProxmoxConnected")
	result.IsAuthenticated = getBoolFromMap(data, "IsAuthenticated")
	result.IsAdmin = getBoolFromMap(data, "IsAdmin")
	result.AllNodesSaturated = getBoolFromMap(data, "AllNodesSaturated")
	result.NoNodesAvailable = getBoolFromMap(data, "NoNodesAvailable")
	result.ValidationError = getStringFromMap(data, "ValidationError")
	result.ActiveNode = getStringFromMap(data, "ActiveNode")
	result.DefaultPool = getStringFromMap(data, "DefaultPool")
	result.AllowCustomYAML = getBoolFromMap(data, "AllowCustomYAML")

	// Limits
	result.VMSocketsMin = getIntFromMap(data, "VMSocketsMin")
	result.VMSocketsMax = getIntFromMap(data, "VMSocketsMax")
	result.VMCoresMin = getIntFromMap(data, "VMCoresMin")
	result.VMCoresMax = getIntFromMap(data, "VMCoresMax")
	result.VMRamMinGB = getIntFromMap(data, "VMRamMinGB")
	result.VMRamMaxGB = getIntFromMap(data, "VMRamMaxGB")
	result.VMRamMinMB = getIntFromMap(data, "VMRamMinMB")
	result.VMRamMaxMB = getIntFromMap(data, "VMRamMaxMB")
	result.VMDiskMin = getIntFromMap(data, "VMDiskMin")
	result.VMDiskMax = getIntFromMap(data, "VMDiskMax")
	result.MaxDiskPerVM = getIntFromMap(data, "MaxDiskPerVM")
	result.MaxNetworkCards = getIntFromMap(data, "MaxNetworkCards")

	// Slices
	result.Nodes = getStringSliceFromMap(data, "Nodes")
	result.Storages = getStringSliceFromMap(data, "Storages")
	result.ISOs = getStringSliceFromMap(data, "ISOs")
	result.Bridges = getStringSliceFromMap(data, "Bridges")
	result.AvailableTags = getStringSliceFromMap(data, "AvailableTags")

	// Maps
	if storageNodes, ok := data["StorageNodes"].(map[string]string); ok {
		result.StorageNodes = storageNodes
	}
	if bridgeDescs, ok := data["BridgeDescriptions"].(map[string]string); ok {
		result.BridgeDescriptions = bridgeDescs
	}
	if bridgeNodes, ok := data["BridgeNodes"].(map[string]string); ok {
		result.BridgeNodes = bridgeNodes
	}

	// Node options
	if nodeOptions, ok := data["NodeOptions"].([]map[string]interface{}); ok {
		result.NodeOptions = make([]components.NodeOption, 0, len(nodeOptions))
		for _, opt := range nodeOptions {
			nodeOpt := components.NodeOption{
				Value:    getStringFromMap(opt, "value"),
				Text:     getStringFromMap(opt, "text"),
				Disabled: getBoolFromMap(opt, "disabled"),
				Reason:   getStringFromMap(opt, "reason"),
			}
			if coresMax, ok := opt["cores_max"].(int); ok {
				nodeOpt.CoresMax = coresMax
			}
			if ramMaxGB, ok := opt["ram_max_gb"].(int); ok {
				nodeOpt.RamMaxGB = ramMaxGB
			}
			// Check if this node is selected
			if result.ActiveNode != "" && nodeOpt.Value == result.ActiveNode {
				nodeOpt.IsSelected = true
			}
			result.NodeOptions = append(result.NodeOptions, nodeOpt)
		}
	}

	// Cloud-init templates
	if templates, ok := data["CloudInitTemplates"].([]state.CloudInitTemplate); ok {
		result.CloudInitTemplates = make([]components.CloudInitTemplateOption, 0, len(templates))
		for _, tpl := range templates {
			result.CloudInitTemplates = append(result.CloudInitTemplates, components.CloudInitTemplateOption{
				ID:          tpl.ID,
				Name:        tpl.Name,
				Description: tpl.Description,
			})
		}
	}

	// Offline nodes notification
	if notification, ok := data["OfflineNodesNotification"].(map[string]interface{}); ok {
		result.OfflineNodesNotification = &components.VMCreateNotificationData{
			Type:  getStringFromMap(notification, "type"),
			Title: getStringFromMap(notification, "title"),
			Text:  getStringFromMap(notification, "text"),
		}
	}

	// Form data
	if formData, ok := data["FormData"].(map[string]string); ok {
		result.FormData.Name = formData["name"]
		result.FormData.Description = formData["description"]
		result.FormData.VMID = formData["vmid"]
		result.FormData.EnableTPM = formData["enable_tpm"]
		result.FormData.StartVM = formData["start_vm"]
		result.FormData.Node = formData["node"]
		result.FormData.Pool = formData["pool"]
		result.FormData.Sockets = formData["sockets"]
		result.FormData.Cores = formData["cores"]
		result.FormData.Memory = formData["memory"]
		result.FormData.MemoryUnit = formData["memory_unit"]
		result.FormData.DiskBusType = formData["disk_bus_type"]
		result.FormData.Storage = formData["storage"]
		result.FormData.EnableEFI = formData["enable_efi"]
		result.FormData.ISO = formData["iso"]
		result.FormData.CloudInitEnable = formData["cloudinit_enable"]
		result.FormData.CloudInitTemplate = formData["cloudinit_template"]
		result.FormData.CloudInitUser = formData["cloudinit_user"]
		result.FormData.CloudInitPassword = formData["cloudinit_password"]
		result.FormData.CloudInitSSHKeys = formData["cloudinit_sshkeys"]
		result.FormData.CloudInitIPConfig = formData["cloudinit_ipconfig"]
		result.FormData.CloudInitIP = formData["cloudinit_ip"]
		result.FormData.CloudInitGateway = formData["cloudinit_gateway"]
		result.FormData.CloudInitDNS = formData["cloudinit_dns"]

		// Extract indexed form data (bridges, disks, network cards)
		maxDisks := result.MaxDiskPerVM
		if maxDisks == 0 {
			maxDisks = 1
		}
		maxNetCards := result.MaxNetworkCards
		if maxNetCards == 0 {
			maxNetCards = 1
		}

		for i := 0; i < maxDisks; i++ {
			if v, exists := formData[fmt.Sprintf("disk_size_%d", i)]; exists {
				result.FormData.DiskSizes[i] = v
			}
			if v, exists := formData[fmt.Sprintf("disk_size_unit_%d", i)]; exists {
				result.FormData.DiskUnits[i] = v
			}
		}

		for i := 0; i < maxNetCards; i++ {
			if v, exists := formData[fmt.Sprintf("bridge_%d", i)]; exists {
				result.FormData.Bridges[i] = v
			}
			if v, exists := formData[fmt.Sprintf("mac_address_%d", i)]; exists {
				result.FormData.MACAddresses[i] = v
			}
			if v, exists := formData[fmt.Sprintf("vlan_tag_%d", i)]; exists {
				result.FormData.VLANTags[i] = v
			}
			if v, exists := formData[fmt.Sprintf("rate_limit_%d", i)]; exists {
				result.FormData.RateLimits[i] = v
			}
			if v, exists := formData[fmt.Sprintf("mtu_%d", i)]; exists {
				result.FormData.MTUs[i] = v
			}
			if v, exists := formData[fmt.Sprintf("network_model_%d", i)]; exists {
				result.FormData.NetworkModels[i] = v
			}
			if v, exists := formData[fmt.Sprintf("network_enabled_%d", i)]; exists {
				result.FormData.NetworkEnabled[i] = v
			}
		}
	}

	return result
}

// Helper functions for type conversion
func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBoolFromMap(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func getIntFromMap(m map[string]interface{}, key string) int {
	if v, ok := m[key].(int); ok {
		return v
	}
	return 0
}

func getIntFromMapWithDefault(m map[string]interface{}, key string, defaultVal int) int {
	if v, ok := m[key].(int); ok {
		return v
	}
	return defaultVal
}

func getStringSliceFromMap(m map[string]interface{}, key string) []string {
	if v, ok := m[key].([]string); ok {
		return v
	}
	return nil
}
