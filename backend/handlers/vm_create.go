package handlers

import (
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
	"pvmss/proxmox"
	"pvmss/security"
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

// VMCreateFormData holds the form data for VM creation (used for session storage)
type VMCreateFormData struct {
	Name        string
	Description string
	VMID        string
	Sockets     string
	Cores       string
	Memory      string
	DiskSize    string
	ISO         string
	Bridge      string
	Node        string
	Pool        string
	Storage     string
	Tags        []string
}

// Register VMCreateFormData with gob for session serialization
func init() {
	gob.Register(VMCreateFormData{})
}

type NodeOption struct {
	Name           string
	Disabled       bool
	DisabledReason string
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
		if list, err := proxmox.GetNodeNamesWithContext(r.Context(), client); err == nil && len(list) > 0 {
			nodes = list
			activeNode = list[0]
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
		for _, option := range nodeOptions {
			if !option.Disabled {
				activeNode = option.Name
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
				"name":        savedFormData.Name,
				"description": savedFormData.Description,
				"vmid":        savedFormData.VMID,
				"sockets":     savedFormData.Sockets,
				"cores":       savedFormData.Cores,
				"memory":      savedFormData.Memory,
				"disk_size":   savedFormData.DiskSize,
				"iso":         savedFormData.ISO,
				"bridge":      savedFormData.Bridge,
				"node":        savedFormData.Node,
				"pool":        savedFormData.Pool,
				"storage":     savedFormData.Storage,
				"tags":        savedFormData.Tags,
			}
			sessionManager.Remove(ctx, "vm_create_form_data") // Clear after reading
		}
	}

	// If no saved form data, use defaults
	if formData == nil {
		formData = map[string]interface{}{
			"tags": []string{"pvmss"},
		}
	}

	if value, ok := formData["node"].(string); ok && value != "" {
		if disabledNodes[value] {
			formData["node"] = ""
		}
	}

	bridgeDetails := make([]map[string]string, 0)
	bridgeDescriptions := make(map[string]string)
	if sm != nil {
		if client == nil {
			log.Warn().Msg("Proxmox client unavailable; skipping bridge description fetch")
		} else {
			for _, nodeName := range nodes {
				vmbrs, err := proxmox.GetVMBRsWithContext(r.Context(), client, nodeName)
				if err != nil {
					log.Warn().Err(err).Str("node", nodeName).Msg("Failed to retrieve VMBRs; continuing with remaining nodes")
					continue
				}
				for _, vmbr := range vmbrs {
					name := getVMBRInterface(vmbr)
					if name == "" {
						continue
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
			"name":        bridgeName,
			"description": bridgeDescriptions[bridgeName],
		})
	}

	data := map[string]interface{}{
		"Title":           "Create VM",
		"ISOs":            settings.ISOs,
		"Bridges":         settings.VMBRs,
		"BridgeDetails":   bridgeDetails,
		"BridgeDescriptions": bridgeDescriptions,
		"AvailableTags":   settings.Tags,
		"Limits":          settings.Limits,
		"Nodes":           nodes,
		"NodeOptions":     nodeOptions,
		"ActiveNode":      activeNode,
		"DefaultPool":     defaultPool,
		"FormData":        formData,
		"ValidationError": validationError,
	}

	// Get available storages for the selected node
	storages := []string{}
	if client != nil && activeNode != "" {
		if storageList, err := proxmox.GetNodeStoragesWithContext(r.Context(), client, activeNode); err == nil {
			// Create a map of enabled storages for quick lookup
			enabledStorageMap := make(map[string]bool)
			for _, enabledStorage := range settings.EnabledStorages {
				enabledStorageMap[enabledStorage] = true
			}

			for _, storage := range storageList {
				// Only include storages that are in enabled_storages list, enabled, and can hold VM disks
				if enabledStorageMap[storage.Storage] && storage.Enabled == 1 && strings.Contains(storage.Content, "images") {
					storages = append(storages, storage.Storage)
				}
			}
		}
	}
	data["Storages"] = storages

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

	// Extract form fields
	name := r.FormValue("name")
	description := r.FormValue("description")
	vmidStr := r.FormValue("vmid")
	socketsStr := r.FormValue("sockets")
	coresStr := r.FormValue("cores")
	memoryMBStr := r.FormValue("memory") // MB
	diskSizeGBStr := r.FormValue("disk_size")
	isoPath := r.FormValue("iso") // settings provides full volid or path string
	bridgeName := r.FormValue("bridge")
	selectedNode := r.FormValue("node")
	poolName := r.FormValue("pool")
	selectedTags := r.Form["tags"]
	selectedStorage := r.FormValue("storage")
	tags := ensureMandatoryTag(selectedTags)

	// Validate mandatory fields
	validationErrors := validateRequiredFields(map[string]string{
		"VM name":        name,
		"Proxmox node":   selectedNode,
		"CPU sockets":    socketsStr,
		"CPU cores":      coresStr,
		"Memory":         memoryMBStr,
		"Disk size":      diskSizeGBStr,
		"Storage":        selectedStorage,
		"ISO image":      isoPath,
		"Network bridge": bridgeName,
	})

	// If validation fails, redirect back to form with errors
	if len(validationErrors) > 0 {
		log.Warn().Strs("validation_errors", validationErrors).Msg("VM creation validation failed")

		// Store form data and errors in session for re-display
		if session := security.GetSession(r); session != nil {
			ctx := r.Context()
			session.Put(ctx, "vm_create_errors", strings.Join(validationErrors, "; "))
			// Preserve form data using concrete struct (gob-serializable)
			formData := VMCreateFormData{
				Name:        name,
				Description: description,
				VMID:        vmidStr,
				Sockets:     socketsStr,
				Cores:       coresStr,
				Memory:      memoryMBStr,
				DiskSize:    diskSizeGBStr,
				ISO:         isoPath,
				Bridge:      bridgeName,
				Node:        selectedNode,
				Pool:        poolName,
				Storage:     selectedStorage,
				Tags:        selectedTags,
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
	nodes, err := proxmox.GetNodeNamesWithContext(ctx, client)
	if err != nil || len(nodes) == 0 {
		log.Error().Err(err).Msg("unable to get Proxmox nodes")
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
	diskSizeGB, err := strconv.Atoi(diskSizeGBStr)
	if err != nil {
		http.Error(w, "invalid disk size", http.StatusBadRequest)
		return
	}

	// Validate against settings limits (vm and optional node-specific)
	if settings := h.stateManager.GetSettings(); settings != nil && settings.Limits != nil {
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
			if min, max, ok2 := readMinMax(rawVM, "disk"); ok2 {
				if diskSizeGB < min || diskSizeGB > max {
					http.Error(w, fmt.Sprintf("disk size must be between %d and %d GB", min, max), http.StatusBadRequest)
					return
				}
			}
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
		v, err := proxmox.GetNextVMID(ctx, client)
		if err != nil {
			log.Error().Err(err).Msg("failed to get next VMID")
			localizer := i18n.GetLocalizerFromRequest(r)
			http.Error(w, i18n.Localize(localizer, "Error.InternalServer"), http.StatusInternalServerError)
			return
		}
		vmid = v
	}

	// Build Proxmox create parameters
	params := map[string]string{
		"vmid":    strconv.Itoa(vmid),
		"name":    name,
		"sockets": strconv.Itoa(sockets),
		"cores":   strconv.Itoa(cores),
		"memory":  strconv.Itoa(memoryMB), // MB
	}

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
		// Set boot order to cdrom first then disk
		params["boot"] = "order=ide2;scsi0"
	}

	// Network: virtio on selected bridge
	if bridgeName != "" {
		params["net0"] = "virtio,bridge=" + bridgeName
	}

	// Disk: use selected storage for VM disk
	if selectedStorage != "" && diskSizeGBStr != "" {
		params["scsi0"] = selectedStorage + ":" + strconv.Itoa(diskSizeGB)
		params["scsihw"] = "virtio-scsi-pci"
	}

	// Perform API call: POST /nodes/{node}/qemu
	path := "/nodes/" + url.PathEscape(node) + "/qemu"

	values := make(url.Values)
	for k, v := range params {
		values.Set(k, v)
	}

	if _, err := client.PostFormWithContext(ctx, path, values); err != nil {
		log.Error().Err(err).Str("node", node).Msg("VM create API call failed")
		localizer := i18n.GetLocalizerFromRequest(r)
		http.Error(w, i18n.Localize(localizer, "Proxmox.ConnectionError"), http.StatusBadGateway)
		return
	}

	// Optional: ensure VM is running. Query current status and start if needed.
	if cur, err := proxmox.GetVMCurrentWithContext(ctx, client, node, vmid); err != nil {
		log.Warn().Err(err).Int("vmid", vmid).Str("node", node).Msg("Could not fetch VM current status after creation")
	} else if strings.ToLower(cur.Status) != "running" {
		if _, err := proxmox.VMActionWithContext(ctx, client, node, strconv.Itoa(vmid), "start"); err != nil {
			log.Warn().Err(err).Int("vmid", vmid).Str("node", node).Msg("Failed to start VM after creation")
		} else {
			log.Info().Int("vmid", vmid).Str("node", node).Msg("VM started after creation")
		}
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
