package handlers

import (
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
	"pvmss/proxmox"
	"pvmss/security"
	"pvmss/state"
)

// readMinMax extracts min/max values from a nested map structure
// Ensures that min and max values are at least 1 (no zero or negative values)
func readMinMax(m map[string]interface{}, key string) (min, max int, ok bool) {
	if raw, exists := m[key]; exists {
		if mm, ok2 := raw.(map[string]interface{}); ok2 {
			vMin, vMax := 1, 1
			if v, ok3 := mm["min"]; ok3 {
				if f, ok4 := v.(float64); ok4 {
					vMin = int(f)
					// Ensure minimum value is at least 1
					if vMin < 1 {
						vMin = 1
					}

				}
			}
			if v, ok3 := mm["max"]; ok3 {
				if f, ok4 := v.(float64); ok4 {
					vMax = int(f)
					// Ensure maximum value is at least 1
					if vMax < 1 {
						vMax = 1
					}
				}
			}
			// Ensure max is at least equal to min
			if vMax < vMin {
				vMax = vMin
			}
			return vMin, vMax, true
		}
	}
	return 0, 0, false
}

// validateRequiredFields checks if required form fields are present
func validateRequiredFields(fields map[string]string) []string {
	var errors []string
	for fieldName, value := range fields {
		if value == "" {
			errors = append(errors, fieldName+" is required")
		}
	}
	return errors
}

// ensureMandatoryTag ensures "pvmss" tag is present and deduplicates tags
func ensureMandatoryTag(selectedTags []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(selectedTags)+1)

	// Always include pvmss first
	if _, ok := seen["pvmss"]; !ok {
		seen["pvmss"] = struct{}{}
		out = append(out, "pvmss")
	}

	for _, t := range selectedTags {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

// getEFIDiskFormat determines the appropriate format for EFI disk based on storage type
// Block-based storages (LVM, LVM-thin, ZFS, Ceph) require 'raw' format
// File-based storages (dir, nfs, cifs) can use 'qcow2' format
// Returns the format string to use for efidisk0 parameter
func getEFIDiskFormat(storageType string) string {
	// Normalize storage type to lowercase for comparison
	storageType = strings.ToLower(storageType)

	// Block-based storages require raw format
	blockBasedStorages := map[string]bool{
		"lvmthin": true,
		"lvm":     true,
		"zfs":     true,
		"ceph":    true,
		"iscsi":   true,
	}

	// If it's a block-based storage, use raw format
	if blockBasedStorages[storageType] {
		return "raw"
	}

	// For file-based storages (dir, nfs, cifs, etc.), use qcow2
	// qcow2 provides better space efficiency and snapshot support
	return "qcow2"
}

// getTPMDiskFormat determines the appropriate format for TPM disk based on storage type
// TPM disks always use 'raw' format according to Proxmox documentation
// Block-based storages work natively, file-based storages will use raw disk images
// Returns true if the storage type is compatible with TPM
func getTPMDiskFormat(storageType string) (format string, compatible bool) {
	// Normalize storage type to lowercase for comparison
	storageType = strings.ToLower(storageType)

	// TPM format is always raw (fixed by Proxmox)
	format = "raw"

	// Check if storage supports raw disk images
	// Block-based storages natively support raw
	blockBasedStorages := map[string]bool{
		"lvmthin": true,
		"lvm":     true,
		"zfs":     true,
		"ceph":    true,
		"iscsi":   true,
	}

	// File-based storages that support raw images
	fileBasedStorages := map[string]bool{
		"dir":  true,
		"nfs":  true,
		"cifs": true,
	}

	// Compatible if either block-based or file-based with raw support
	compatible = blockBasedStorages[storageType] || fileBasedStorages[storageType]

	return format, compatible
}

// VMCreateFormData holds the form data for VM creation (used for session storage)
type VMCreateFormData struct {
	Bridge       string
	Cores        string
	Description  string
	DiskBusType  string
	EnableEFI    string
	EnableTPM    string
	ISO          string
	Memory       string
	Name         string
	NetworkModel string
	Node         string
	Pool         string
	Sockets      string
	StartVM      string
	Storage      string
	Tags         []string
	VMID         string
}

// Register VMCreateFormData with gob for session serialization
func init() {
	gob.Register(VMCreateFormData{})
}

type NodeOption struct {
	Disabled       bool
	DisabledReason string
	Name           string
}

var vmDiskCompatibleStorageTypes = map[string]struct{}{
	"ceph":    {},
	"cephfs":  {},
	"dir":     {},
	"lvm":     {},
	"lvmthin": {},
	"nfs":     {},
	"rbd":     {},
	"zfs":     {},
}

// CreateVMPage renders the VM creation form with pre-populated settings from settings.json
// This includes ISOs, VMBRs, Tags, Limits, and available nodes from Proxmox
func (h *VMHandler) CreateVMPage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("CreateVMPage", r)
	sm := h.stateManager
	settings := sm.GetSettings()
	client := sm.GetProxmoxClient()
	// Get nodes list (best effort)
	nodes := []string{}
	activeNode := ""
	if client != nil {
		if restyClient, err := getDefaultRestyClient(); err == nil {
			if list, err := proxmox.GetNodeNamesResty(r.Context(), restyClient); err == nil && len(list) > 0 {
				nodes = list
				activeNode = list[0]
			}
		}
	}

	nodeOptions := make([]NodeOption, 0, len(nodes))
	disabledNodes := make(map[string]bool)
	var nodeUsage map[string]*NodeResourceUsage
	if client != nil && len(nodes) > 0 {
		usageCtx, usageCancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer usageCancel()
		usage, err := CalculateNodeResourceUsage(usageCtx, client, h.stateManager)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to calculate node resource usage for create page")
		} else {
			nodeUsage = usage
		}
	}
	for _, nodeName := range nodes {
		option := NodeOption{Name: nodeName}
		if nodeUsage != nil {
			if usageEntry, ok := nodeUsage[nodeName]; ok && usageEntry != nil {
				saturated := false
				if usageEntry.MaxCores > 0 && usageEntry.Cores >= usageEntry.MaxCores {
					saturated = true
				}
				if usageEntry.MaxRamGB > 0 && usageEntry.RamGB >= usageEntry.MaxRamGB {
					saturated = true
				}
				if saturated {
					option.Disabled = true
					option.DisabledReason = "VM.Create.NodeLimitReached"
					disabledNodes[nodeName] = true
				}
			}
		}
		nodeOptions = append(nodeOptions, option)
	}

	if activeNode != "" {
		for _, option := range nodeOptions {
			if option.Name == activeNode {
				if option.Disabled {
					activeNode = ""
				}
				break
			}
		}
	}
	if activeNode == "" {
		log.Info().
			Int("nodeOptions_count", len(nodeOptions)).
			Msg("DEBUG: activeNode is empty, looking for enabled node")
		for _, option := range nodeOptions {
			log.Info().
				Str("node", option.Name).
				Bool("disabled", option.Disabled).
				Str("reason", option.DisabledReason).
				Msg("DEBUG: Checking node")
			if !option.Disabled {
				activeNode = option.Name
				log.Info().Str("activeNode", activeNode).Msg("DEBUG: Selected enabled node")
				break
			}
		}
	}

	// Get username from session to pre-fill pool
	defaultPool := ""
	ctx := r.Context()
	var validationError string
	var formData map[string]interface{}

	if sessionManager := security.GetSession(r); sessionManager != nil {
		if username, ok := sessionManager.Get(ctx, "username").(string); ok && username != "" {
			defaultPool = fmt.Sprintf("pvmss_%s", username)
		}

		// Check for validation errors from previous submission
		if errMsg, ok := sessionManager.Get(ctx, "vm_create_errors").(string); ok && errMsg != "" {
			validationError = errMsg
			sessionManager.Remove(ctx, "vm_create_errors") // Clear after reading
		}

		// Retrieve preserved form data
		if savedFormData, ok := sessionManager.Get(ctx, "vm_create_form_data").(VMCreateFormData); ok {
			// Convert struct to map for template
			formData = map[string]interface{}{
				"bridge":        savedFormData.Bridge,
				"cores":         savedFormData.Cores,
				"description":   savedFormData.Description,
				"disk_bus_type": savedFormData.DiskBusType,
				"enable_efi":    savedFormData.EnableEFI,
				"enable_tpm":    savedFormData.EnableTPM,
				"iso":           savedFormData.ISO,
				"memory":        savedFormData.Memory,
				"name":          savedFormData.Name,
				"network_model": savedFormData.NetworkModel,
				"node":          savedFormData.Node,
				"pool":          savedFormData.Pool,
				"sockets":       savedFormData.Sockets,
				"start_vm":      savedFormData.StartVM,
				"storage":       savedFormData.Storage,
				"tags":          savedFormData.Tags,
				"vmid":          savedFormData.VMID,
			}
			sessionManager.Remove(ctx, "vm_create_form_data") // Clear after reading
		}
	}

	// If no saved form data, use defaults
	if formData == nil {
		formData = map[string]interface{}{
			"enable_efi":    "1",      // EFI enabled by default
			"enable_tpm":    "",       // TPM disabled by default
			"network_model": "virtio", // virtio is the default network model
			"start_vm":      "1",      // Start VM enabled by default
			"tags":          []string{"pvmss"},
		}
	}

	if value, ok := formData["node"].(string); ok && value != "" {
		if disabledNodes[value] {
			formData["node"] = ""
		}
	}

	bridgeDetails := make([]map[string]string, 0)
	bridgeDescriptions := make(map[string]string)
	bridgeNodes := make(map[string]string)
	if sm != nil {
		if client == nil {
			log.Warn().Msg("Proxmox client unavailable; skipping bridge description fetch")
		} else if restyClient, err := getDefaultRestyClient(); err == nil {
			for _, nodeName := range nodes {
				// Skip disabled (saturated) nodes
				if disabledNodes[nodeName] {
					log.Debug().Str("node", nodeName).Msg("Skipping disabled node for VMBR retrieval")
					continue
				}
				vmbrs, err := proxmox.GetVMBRsResty(r.Context(), restyClient, nodeName)
				if err != nil {
					log.Warn().Err(err).Str("node", nodeName).Msg("Failed to retrieve VMBRs; continuing with remaining nodes")
					continue
				}
				for _, vmbr := range vmbrs {
					name := getVMBRInterface(vmbr)
					if name == "" {
						continue
					}
					if _, exists := bridgeNodes[name]; !exists {
						bridgeNodes[name] = nodeName
					}
					if desc, exists := bridgeDescriptions[name]; exists && desc != "" {
						continue
					}
					bridgeDescriptions[name] = buildVMBRDescription(vmbr)
				}
			}
		}
	}
	for _, bridgeName := range settings.VMBRs {
		bridgeDetails = append(bridgeDetails, map[string]string{
			"description": bridgeDescriptions[bridgeName],
			"name":        bridgeName,
			"node":        bridgeNodes[bridgeName],
		})
	}

	// Sort lists for better UX
	sort.Strings(settings.ISOs)
	sort.Strings(settings.VMBRs)
	sort.Strings(settings.Tags)

	// Check if all nodes are disabled (saturated)
	allNodesSaturated := len(nodeOptions) > 0
	for _, option := range nodeOptions {
		if !option.Disabled {
			allNodesSaturated = false
			break
		}
	}

	data := map[string]interface{}{
		"ActiveNode":         activeNode,
		"AllNodesSaturated":  allNodesSaturated,
		"AvailableTags":      settings.Tags,
		"BridgeDescriptions": bridgeDescriptions,
		"BridgeDetails":      bridgeDetails,
		"BridgeNodes":        bridgeNodes,
		"Bridges":            settings.VMBRs,
		"DefaultPool":        defaultPool,
		"FormData":           formData,
		"ISOs":               settings.ISOs,
		"Limits":             settings.Limits,
		"MaxDiskPerVM":       settings.MaxDiskPerVM,
		"MaxNetworkCards":    settings.MaxNetworkCards,
		"NodeOptions":        nodeOptions,
		"Nodes":              nodes,
		"TitleKey":           "VM.Create.Title",
		"ValidationError":    validationError,
	}

	// Get available storages for all non-saturated nodes (cluster support)
	storages := []string{}
	storageNodes := make(map[string]string)
	log.Info().
		Str("activeNode", activeNode).
		Bool("client_nil", client == nil).
		Int("disabled_nodes_count", len(disabledNodes)).
		Msg("DEBUG: Before storage retrieval")
	if client != nil && len(nodes) > 0 {
		if restyClient, err := getDefaultRestyClient(); err == nil {
			var globalStorageInfo map[string]proxmox.Storage
			if globalList, err := proxmox.GetStoragesResty(r.Context(), restyClient); err != nil {
				log.Warn().Err(err).Msg("Failed to fetch global storage list; continuing without additional metadata")
			} else {
				globalStorageInfo = make(map[string]proxmox.Storage, len(globalList))
				for _, item := range globalList {
					globalStorageInfo[item.Storage] = item
				}
			}

			// Create a map of enabled storages for quick lookup
			enabledStorageMap := make(map[string]bool)
			for _, enabledStorage := range settings.EnabledStorages {
				enabledStorageMap[enabledStorage] = true
			}

			// Iterate through all non-saturated nodes to collect storages
			for _, nodeName := range nodes {
				// Skip disabled (saturated) nodes
				if disabledNodes[nodeName] {
					log.Debug().Str("node", nodeName).Msg("Skipping disabled node for storage retrieval")
					continue
				}

				storageList, err := proxmox.GetNodeStoragesResty(r.Context(), restyClient, nodeName)
				if err != nil {
					log.Warn().Err(err).Str("node", nodeName).Msg("Failed to retrieve storages for node; continuing with remaining nodes")
					continue
				}

				for _, storage := range storageList {
					storageInfo := storage
					if globalStorageInfo != nil {
						if global, exists := globalStorageInfo[storage.Storage]; exists {
							if storageInfo.Content == "" && global.Content != "" {
								storageInfo.Content = global.Content
							}
							if storageInfo.Type == "" && global.Type != "" {
								storageInfo.Type = global.Type
							}
							if storageInfo.Description == "" && global.Description != "" {
								storageInfo.Description = global.Description
							}
						}
					}

					// Only include storages that are enabled and can hold VM disks
					// If EnabledStorages is empty, all storages are considered enabled
					isEnabledStorage := len(settings.EnabledStorages) == 0 || enabledStorageMap[storage.Storage]
					storageType := strings.ToLower(storageInfo.Type)
					storageContent := strings.ToLower(storageInfo.Content)

					supportsVMDisk := strings.Contains(storageContent, "images")
					if !supportsVMDisk {
						if _, ok := vmDiskCompatibleStorageTypes[storageType]; ok {
							supportsVMDisk = true
						}
					}

					if isEnabledStorage && storage.Enabled == 1 && supportsVMDisk {
						// Only add if not already present (avoid duplicates across nodes)
						alreadyExists := false
						for _, existing := range storages {
							if existing == storage.Storage {
								alreadyExists = true
								break
							}
						}
						if !alreadyExists {
							storages = append(storages, storage.Storage)
						}
						// Always store the node mapping (prefer first node if storage exists on multiple nodes)
						if _, exists := storageNodes[storage.Storage]; !exists {
							storageNodes[storage.Storage] = nodeName
						}
					}
				}
			}
			// Sort storages alphabetically
			sort.Strings(storages)
		}
	}
	log.Info().
		Int("storage_count", len(storages)).
		Strs("storages", storages).
		Msg("DEBUG: After storage retrieval")
	data["Storages"] = storages
	data["StorageNodes"] = storageNodes

	// Proxmox connection status for template (also provided by middleware, but ensure here)
	if sm != nil {
		connected, message := sm.GetProxmoxStatus()
		data["ProxmoxConnected"] = connected
		if !connected {
			data["ProxmoxError"] = message
		}
	}

	// Add i18n data
	RenderTemplate(w, r, "create_vm", data)
}

// CreateVMHandler processes POST /api/vm/create to create a VM in Proxmox
// Validates form data, applies limits from settings, and creates the VM via Proxmox API
func (h *VMHandler) CreateVMHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("CreateVMHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	// Get settings early for network cards configuration
	settings := h.stateManager.GetSettings()

	// Extract form fields
	coresStr := r.FormValue("cores")
	description := r.FormValue("description")
	diskBusType := r.FormValue("disk_bus_type")
	enableEFI := r.FormValue("enable_efi")         // "1" if checked, "" otherwise
	enableTPM := r.FormValue("enable_tpm")         // "1" if checked, "" otherwise
	firstDiskSizeStr := r.FormValue("disk_size_0") // First disk is mandatory
	isoPath := r.FormValue("iso")                  // settings provides full volid or path string
	memoryMBStr := r.FormValue("memory")           // MB
	name := r.FormValue("name")
	poolName := r.FormValue("pool")
	selectedNode := r.FormValue("node")
	selectedStorage := r.FormValue("storage")
	selectedTags := r.Form["tags"]
	socketsStr := r.FormValue("sockets")
	startVM := r.FormValue("start_vm") // "1" if checked, "" otherwise
	tags := ensureMandatoryTag(selectedTags)
	vmidStr := r.FormValue("vmid")

	// Validate at least the first network bridge is specified
	firstBridge := r.FormValue("bridge_0")

	// Validate mandatory fields
	validationErrors := validateRequiredFields(map[string]string{
		"CPU cores":      coresStr,
		"CPU sockets":    socketsStr,
		"Disk size":      firstDiskSizeStr,
		"ISO image":      isoPath,
		"Memory":         memoryMBStr,
		"Network bridge": firstBridge,
		"Proxmox node":   selectedNode,
		"Storage":        selectedStorage,
		"VM name":        name,
	})

	// If validation fails, redirect back to form with errors
	if len(validationErrors) > 0 {
		log.Warn().Strs("validation_errors", validationErrors).Msg("VM creation validation failed")

		// Store form data and errors in session for re-display
		if session := security.GetSession(r); session != nil {
			ctx := r.Context()
			session.Put(ctx, "vm_create_errors", strings.Join(validationErrors, "; "))
			// Preserve form data using concrete struct (gob-serializable)
			// Note: Bridge and NetworkModel kept for backward compatibility (first card)
			formData := VMCreateFormData{
				Bridge:       firstBridge,
				Cores:        coresStr,
				Description:  description,
				DiskBusType:  diskBusType,
				EnableEFI:    enableEFI,
				EnableTPM:    enableTPM,
				ISO:          isoPath,
				Memory:       memoryMBStr,
				Name:         name,
				NetworkModel: r.FormValue("network_model_0"),
				Node:         selectedNode,
				Pool:         poolName,
				Sockets:      socketsStr,
				StartVM:      startVM,
				Storage:      selectedStorage,
				Tags:         selectedTags,
				VMID:         vmidStr,
			}
			session.Put(ctx, "vm_create_form_data", formData)
		}

		// Redirect back to form
		http.Redirect(w, r, "/vm/create", http.StatusSeeOther)
		return
	}

	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not initialized")
		localizer := i18n.GetLocalizerFromRequest(r)
		http.Error(w, i18n.Localize(localizer, "Error.InternalServer"), http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	// Determine node: use selected if provided, otherwise pick the first available node
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create resty client")
		localizer := i18n.GetLocalizerFromRequest(r)
		http.Error(w, i18n.Localize(localizer, "Error.FailedCreateVM"), http.StatusInternalServerError)
		return
	}

	nodes, err := proxmox.GetNodeNamesResty(ctx, restyClient)
	if err != nil || len(nodes) == 0 {
		log.Error().Err(err).Msg("unable to get Proxmox nodes (resty)")
		localizer := i18n.GetLocalizerFromRequest(r)
		http.Error(w, i18n.Localize(localizer, "Proxmox.ConnectionError"), http.StatusBadGateway)
		return
	}
	node := nodes[0]
	if selectedNode != "" {
		// ensure selected node exists
		for _, n := range nodes {
			if n == selectedNode {
				node = selectedNode
				break
			}
		}
	}

	// Parse numeric fields
	sockets, err := strconv.Atoi(socketsStr)
	if err != nil {
		http.Error(w, "invalid sockets", http.StatusBadRequest)
		return
	}
	cores, err := strconv.Atoi(coresStr)
	if err != nil {
		http.Error(w, "invalid cores", http.StatusBadRequest)
		return
	}
	memoryMB, err := strconv.Atoi(memoryMBStr)
	if err != nil {
		http.Error(w, "invalid memory", http.StatusBadRequest)
		return
	}

	// Validate against settings limits (vm and optional node-specific)
	if settings != nil && settings.Limits != nil {
		// VM limits
		if rawVM, ok := settings.Limits["vm"].(map[string]interface{}); ok {
			if min, max, ok2 := readMinMax(rawVM, "sockets"); ok2 {
				if sockets < min || sockets > max {
					http.Error(w, fmt.Sprintf("sockets must be between %d and %d", min, max), http.StatusBadRequest)
					return
				}
			}
			if min, max, ok2 := readMinMax(rawVM, "cores"); ok2 {
				if cores < min || cores > max {
					http.Error(w, fmt.Sprintf("cores must be between %d and %d", min, max), http.StatusBadRequest)
					return
				}
			}
			if minGB, maxGB, ok2 := readMinMax(rawVM, "ram"); ok2 {
				minMB := minGB * 1024
				maxMB := maxGB * 1024
				if memoryMB < minMB || memoryMB > maxMB {
					http.Error(w, fmt.Sprintf("memory must be between %d and %d MB", minMB, maxMB), http.StatusBadRequest)
					return
				}
			}
			// Disk size validation is now handled per-disk in the disk configuration loop
			// if min, max, ok2 := readMinMax(rawVM, "disk"); ok2 {
			// 	if diskSizeGB < min || diskSizeGB > max {
			// 		http.Error(w, fmt.Sprintf("disk size must be between %d and %d GB", min, max), http.StatusBadRequest)
			// 		return
			// 	}
			// }
		}

		// Node-specific caps (optional) - per-VM limits
		if rawNodes, ok := settings.Limits["nodes"].(map[string]interface{}); ok {
			if rawNode, ok2 := rawNodes[node].(map[string]interface{}); ok2 {
				if _, max, ok3 := readMinMax(rawNode, "sockets"); ok3 {
					// Enforce only upper bound from node limits; VM lower bound is validated earlier
					if sockets > max {
						http.Error(w, fmt.Sprintf("sockets exceed node '%s' max (%d)", node, max), http.StatusBadRequest)
						return
					}
				}
				if _, max, ok3 := readMinMax(rawNode, "cores"); ok3 {
					// Enforce only upper bound from node limits; VM lower bound is validated earlier
					if cores > max {
						http.Error(w, fmt.Sprintf("cores exceed node '%s' max (%d)", node, max), http.StatusBadRequest)
						return
					}
				}
				if _, maxGB, ok3 := readMinMax(rawNode, "ram"); ok3 {
					// Enforce only upper bound from node limits; VM lower bound is validated earlier
					maxMB := maxGB * 1024
					if memoryMB > maxMB {
						http.Error(w, fmt.Sprintf("memory exceeds node '%s' max (%d MB)", node, maxMB), http.StatusBadRequest)
						return
					}
				}
			}
		}

		// Validate aggregate node limits (sum of all pvmss VMs)
		if err := ValidateVMResourcesAgainstNodeLimits(ctx, client, h.stateManager, node, sockets, cores, memoryMB); err != nil {
			log.Warn().Err(err).Str("node", node).Msg("VM creation would exceed aggregate node limits")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Ensure VMID
	vmid := 0
	if vmidStr != "" {
		if v, err := strconv.Atoi(vmidStr); err == nil {
			vmid = v
		}
	}
	if vmid == 0 {
		v, err := proxmox.GetNextVMIDResty(ctx, restyClient)
		if err != nil {
			log.Error().Err(err).Msg("failed to get next VMID (resty)")
			localizer := i18n.GetLocalizerFromRequest(r)
			http.Error(w, i18n.Localize(localizer, "Error.FailedCreateVM"), http.StatusInternalServerError)
			return
		}
		vmid = v
	}

	// Build Proxmox create parameters
	params := map[string]string{
		"cores":   strconv.Itoa(cores),
		"memory":  strconv.Itoa(memoryMB),
		"name":    name,
		"sockets": strconv.Itoa(sockets),
		"vmid":    strconv.Itoa(vmid),
	}

	params["agent"] = "enabled=1"

	// Assign to pool if provided
	if poolName != "" {
		params["pool"] = poolName
	}

	// Tags (Proxmox supports 'tags': csv)
	if len(tags) > 0 {
		params["tags"] = strings.Join(tags, ",")
	}
	if description != "" {
		params["description"] = description
	}

	// Attach ISO if provided (ide2 with media=cdrom)
	if isoPath != "" {
		// Expect iso to be a Proxmox volid like 'local:iso/debian.iso'
		params["ide2"] = isoPath + ",media=cdrom"
	}

	// Network: configure multiple network cards based on admin settings
	maxNetworkCards := settings.MaxNetworkCards
	if maxNetworkCards <= 0 {
		maxNetworkCards = 1 // Default to 1 if not configured
	}
	if maxNetworkCards > 10 {
		maxNetworkCards = 10 // Cap at 10 for safety
	}

	// Validate network models (security: prevent injection)
	validModels := map[string]bool{
		"e1000":   true,
		"e1000e":  true,
		"rtl8139": true,
		"virtio":  true,
		"vmxnet3": true,
	}

	networkCardsConfigured := 0
	for cardIdx := 0; cardIdx < maxNetworkCards; cardIdx++ {
		bridgeParam := fmt.Sprintf("bridge_%d", cardIdx)
		modelParam := fmt.Sprintf("network_model_%d", cardIdx)

		bridgeName := r.FormValue(bridgeParam)
		networkModel := r.FormValue(modelParam)

		// Skip if no bridge specified (optional cards)
		if bridgeName == "" {
			continue
		}

		// Default to virtio if no model specified
		if networkModel == "" {
			networkModel = "virtio"
		}

		// Validate model
		if !validModels[networkModel] {
			log.Warn().
				Int("card_index", cardIdx).
				Str("network_model", networkModel).
				Msg("Invalid network model, defaulting to virtio")
			networkModel = "virtio"
		}

		// Configure network interface netX
		netParam := fmt.Sprintf("net%d", cardIdx)
		params[netParam] = networkModel + ",bridge=" + bridgeName
		networkCardsConfigured++

		log.Info().
			Int("card_index", cardIdx).
			Str("net_param", netParam).
			Str("network_model", networkModel).
			Str("bridge", bridgeName).
			Msg("Configured network interface")
	}

	log.Info().Int("network_cards_configured", networkCardsConfigured).Msg("Network configuration complete")

	// Disk Bus Type: validate and set default if not specified
	if diskBusType == "" {
		diskBusType = state.DiskBusVirtIO // Default to VirtIO if not specified
	}

	// Validate disk bus type (security: prevent injection)
	// Use constants from state package for bus type validation
	validBusTypes := map[string]bool{
		state.DiskBusIDE:    true,
		state.DiskBusSATA:   true,
		state.DiskBusSCSI:   true,
		state.DiskBusVirtIO: true,
	}
	if !validBusTypes[diskBusType] {
		log.Warn().Str("disk_bus_type", diskBusType).Msg("Invalid disk bus type, defaulting to VirtIO")
		diskBusType = state.DiskBusVirtIO
	}

	// Get maximum disks for the selected bus type using state constants
	maxDiskPerVM := settings.MaxDiskPerVM
	if maxDiskPerVM <= 0 {
		maxDiskPerVM = 1 // Default to 1 if not configured
	}

	// Enforce hard limit based on selected bus type
	busLimit := state.GetMaxDisksForBus(diskBusType)
	if maxDiskPerVM > busLimit {
		log.Warn().
			Int("max_disk_per_vm", maxDiskPerVM).
			Int("bus_limit", busLimit).
			Str("bus_type", diskBusType).
			Msg("MaxDiskPerVM exceeds bus limit, clamping to bus limit")
		maxDiskPerVM = busLimit
	}

	// Disk: configure multiple disks based on admin settings
	disksConfigured := 0
	firstDisk := ""
	for diskIdx := 0; diskIdx < maxDiskPerVM; diskIdx++ {
		diskSizeParam := fmt.Sprintf("disk_size_%d", diskIdx)
		diskSizeStr := r.FormValue(diskSizeParam)

		// Skip if no size specified (optional disks)
		if diskSizeStr == "" || diskSizeStr == "0" {
			continue
		}

		diskSize, err := strconv.Atoi(diskSizeStr)
		if err != nil || diskSize <= 0 {
			log.Warn().
				Int("disk_index", diskIdx).
				Str("disk_size", diskSizeStr).
				Msg("Invalid disk size, skipping")
			continue
		}

		// Configure disk with appropriate bus prefix
		diskParam := fmt.Sprintf("%s%d", diskBusType, diskIdx)
		params[diskParam] = selectedStorage + ":" + strconv.Itoa(diskSize)

		// Track first disk for boot configuration
		if firstDisk == "" {
			firstDisk = diskParam
		}

		disksConfigured++

		log.Info().
			Int("disk_index", diskIdx).
			Str("disk_param", diskParam).
			Str("bus_type", diskBusType).
			Int("disk_size_gb", diskSize).
			Str("storage", selectedStorage).
			Msg("Configured disk")
	}

	// Configure bus-specific hardware if SCSI is used
	if diskBusType == "scsi" && disksConfigured > 0 {
		params["scsihw"] = "virtio-scsi-pci"
	}

	// Configure boot disk (first disk configured)
	if firstDisk != "" {
		params["bootdisk"] = firstDisk
		if isoPath != "" {
			params["boot"] = "order=" + firstDisk + ";ide2"
		} else {
			params["boot"] = "order=" + firstDisk
		}
	}

	log.Info().
		Int("disks_configured", disksConfigured).
		Str("disk_bus_type", diskBusType).
		Str("boot_disk", firstDisk).
		Msg("Disk configuration complete")

	// EFI Boot: Enable UEFI firmware (OVMF) and create EFI disk
	if enableEFI == "1" {
		// Set BIOS to OVMF (UEFI firmware)
		params["bios"] = "ovmf"

		// Create EFI disk in the same storage as the main disk
		// Determine appropriate format based on storage type
		if selectedStorage != "" {
			// Fetch storage information to determine the correct format
			storageInfo, err := proxmox.GetNodeStoragesResty(ctx, restyClient, node)
			if err != nil {
				log.Warn().Err(err).Str("node", node).Msg("Failed to fetch storage info for EFI disk format detection, using default format")
				// Fallback to qcow2 if we can't determine storage type
				params["efidisk0"] = selectedStorage + ":1,format=qcow2,efitype=4m"
			} else {
				// Find the selected storage and get its type
				storageType := "dir" // Default fallback
				for _, storage := range storageInfo {
					if storage.Storage == selectedStorage {
						storageType = storage.Type
						break
					}
				}

				// Determine format based on storage type
				efiFormat := getEFIDiskFormat(storageType)
				params["efidisk0"] = selectedStorage + ":1,format=" + efiFormat + ",efitype=4m"

				log.Info().
					Str("storage", selectedStorage).
					Str("storage_type", storageType).
					Str("efi_format", efiFormat).
					Msg("EFI boot enabled: creating EFI disk with appropriate format")
			}
		}
	}

	// TPM: Enable Trusted Platform Module (required for Windows 11, Secure Boot, etc.)
	if enableTPM == "1" {
		// Create TPM state disk in the same storage as the main disk
		// TPM format is always 'raw' (fixed size of 4 MiB)
		if selectedStorage != "" {
			// Fetch storage information to verify compatibility
			storageInfo, err := proxmox.GetNodeStoragesResty(ctx, restyClient, node)
			if err != nil {
				log.Warn().Err(err).Str("node", node).Msg("Failed to fetch storage info for TPM disk, skipping TPM creation")
			} else {
				// Find the selected storage and get its type
				storageType := "dir" // Default fallback
				for _, storage := range storageInfo {
					if storage.Storage == selectedStorage {
						storageType = storage.Type
						break
					}
				}

				// Check if storage is compatible with TPM (raw format)
				tpmFormat, compatible := getTPMDiskFormat(storageType)
				if compatible {
					// Create TPM state disk with v2.0 (required for Windows 11)
					params["tpmstate0"] = selectedStorage + ":4,version=v2.0"

					log.Info().
						Str("storage", selectedStorage).
						Str("storage_type", storageType).
						Str("tpm_format", tpmFormat).
						Str("tpm_version", "v2.0").
						Msg("TPM enabled: creating TPM state disk")
				} else {
					log.Warn().
						Str("storage", selectedStorage).
						Str("storage_type", storageType).
						Msg("Storage type not compatible with TPM (raw format required), skipping TPM creation")
				}
			}
		}
	}

	// Start VM after creation if user selected this option (default: enabled)
	if startVM == "1" {
		params["start"] = "1"
	}

	// Perform API call: POST /nodes/{node}/qemu
	path := "/nodes/" + url.PathEscape(node) + "/qemu"

	values := make(url.Values)
	for k, v := range params {
		values.Set(k, v)
	}

	// Create VM (Proxmox handles auto-start when start=1)
	if _, err := client.PostFormWithContext(ctx, path, values); err != nil {
		log.Error().Err(err).Str("node", node).Msg("VM create API call failed")
		localizer := i18n.GetLocalizerFromRequest(r)
		http.Error(w, i18n.Localize(localizer, "Proxmox.ConnectionError"), http.StatusBadGateway)
		return
	}

	// Invalidate caches so the new VM appears immediately in profile and search
	client.InvalidateCache("/nodes/" + url.PathEscape(node) + "/qemu")
	if poolName != "" {
		client.InvalidateCache("/pools/" + url.PathEscape(poolName))
		log.Info().Str("pool", poolName).Msg("Invalidated pool cache after VM creation")
	}

	// Redirect to details
	redirectURL := "/vm/details/" + strconv.Itoa(vmid) + "?refresh=1"
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}
