package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
	"pvmss/proxmox"
	"pvmss/security"
	"pvmss/state"
)

// vmDiskCompatibleStorageTypes defines storage types that support VM disk images
// These storage types can store VM disks even if their content string doesn't explicitly list "images"
var vmDiskCompatibleStorageTypes = map[string]bool{
	"lvmthin": true,
	"lvm":     true,
	"zfs":     true,
	"ceph":    true,
	"iscsi":   true,
	"dir":     true,
	"nfs":     true,
	"cifs":    true,
}

// VMCreateOptimizedHandler handles VM creation with optimized cluster performance
type VMCreateOptimizedHandler struct {
	stateManager state.StateManager
}

// NewVMCreateOptimizedHandler creates a new instance of VMCreateOptimizedHandler
func NewVMCreateOptimizedHandler(sm state.StateManager) *VMCreateOptimizedHandler {
	return &VMCreateOptimizedHandler{stateManager: sm}
}

// RegisterRoutes registers VM creation routes
func (h *VMCreateOptimizedHandler) RegisterRoutes(router *httprouter.Router) {
	log := CreateHandlerLogger("VMCreateOptimizedHandler", nil)

	if router == nil {
		log.Error().Msg("Router is nil, cannot register VM creation routes")
		return
	}

	log.Debug().Msg("Registering optimized VM creation routes")

	// Register both /create-vm and /vm/create routes for compatibility
	router.GET("/create-vm", RequireAuthHandle(h.VMCreatePageHandler))
	router.POST("/create-vm", SecureFormHandler("VM Create",
		RequireAuthHandle(h.VMCreatePageHandler),
	))

	router.GET("/vm/create", RequireAuthHandle(h.VMCreatePageHandler))
	router.POST("/vm/create", SecureFormHandler("VM Create",
		RequireAuthHandle(h.VMCreatePageHandler),
	))

	log.Info().
		Strs("routes", []string{"GET /create-vm", "POST /create-vm", "GET /vm/create", "POST /vm/create"}).
		Msg("Optimized VM creation routes registered successfully")
}

// VMCreatePageHandler handles both GET and POST requests for VM creation page with optimizations
func (h *VMCreateOptimizedHandler) VMCreatePageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("VMCreatePageHandler", r)

	// Get user info from session
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

	log.Info().
		Str("username", username).
		Bool("is_admin", isAdmin).
		Msg("Optimized VM create request started")

	// Get settings
	settings := h.stateManager.GetSettings()
	if settings == nil {
		log.Error().Msg("Settings not available")
		http.Error(w, "Settings unavailable", http.StatusInternalServerError)
		return
	}

	// Get Proxmox client
	client := h.stateManager.GetProxmoxClient()

	// Get node information
	log.Debug().Msg("Getting node information")
	nodes, disabledNodes, activeNode, err := h.getOptimizedNodeInfo(r.Context(), client)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get node information")
		http.Error(w, "Failed to get node information", http.StatusInternalServerError)
		return
	}
	log.Debug().Strs("nodes", nodes).Str("active_node", activeNode).Msg("Node information retrieved")

	// Build node options for template
	nodeOptions := make([]map[string]interface{}, 0, len(nodes))
	for _, nodeName := range nodes {
		option := map[string]interface{}{
			"value":    nodeName,
			"text":     nodeName,
			"disabled": disabledNodes[nodeName],
		}
		if disabledNodes[nodeName] {
			option["reason"] = "This node has reached its PVMSS resource limits"
		}
		nodeOptions = append(nodeOptions, option)
	}
	log.Debug().Int("node_options_count", len(nodeOptions)).Msg("Node options built")

	// Get storages and bridges concurrently
	log.Debug().Msg("Getting resources (storages and bridges)")
	storages, storageNodes, bridgeDetails, err := h.getOptimizedResources(r.Context(), client, nodes, disabledNodes, settings)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get resources")
		http.Error(w, "Failed to get resources", http.StatusInternalServerError)
		return
	}
	log.Debug().Int("storages_count", len(storages)).Int("bridges_count", len(bridgeDetails)).Msg("Resources retrieved")

	// Prepare form data
	formData := map[string]string{
		"bridge":            "",
		"cpu_cores":         "2",
		"cpu_sockets":       "1",
		"description":       "",
		"disk_bus":          "virtio",
		"disk_gb":           "20",
		"enable_efi":        "1",
		"enable_tpm":        "",
		"iso_image":         "",
		"memory_mb":         "2048",
		"name":              "",
		"network_enabled_0": "1", // First network card enabled by default
		"network_model":     "virtio",
		"node":              activeNode,
		"pool":              fmt.Sprintf("pvmss_%s", username),
		"storage":           "",
		"tags":              "",
		"vmid":              "",
	}

	// Override with session data if available (for form repopulation after validation errors)
	if sessionManager := security.GetSession(r); sessionManager != nil {
		if sessionData, ok := sessionManager.Get(r.Context(), "vm_create_form").(map[string]string); ok {
			for key, value := range sessionData {
				if _, exists := formData[key]; exists {
					formData[key] = value
				}
			}
			// Clear session data after use
			sessionManager.Remove(r.Context(), "vm_create_form")
		}
	}

	// Prepare template data
	data := map[string]interface{}{
		"TitleKey":         "VM.Create.Title",
		"Lang":             i18n.GetLanguage(r),
		"IsAuthenticated":  true,
		"IsAdmin":          isAdmin,
		"Username":         username,
		"ISOs":             settings.ISOs,
		"Limits":           settings.Limits,
		"MaxDiskPerVM":     settings.MaxDiskPerVM,
		"MaxNetworkCards":  settings.MaxNetworkCards,
		"NodeOptions":      nodeOptions,
		"Nodes":            nodes,
		"Storages":         storages,
		"StorageNodes":     storageNodes,
		"BridgeDetails":    bridgeDetails,
		"Tags":             settings.Tags,
		"FormData":         formData,
		"ValidationError":  "",
		"ProxmoxConnected": client != nil,
	}

	// Extract bridges from BridgeDetails for template compatibility
	var bridges []string
	bridgeNodes := make(map[string]string)
	bridgeDescriptions := make(map[string]string)

	for _, detail := range bridgeDetails {
		bridgeName := detail["name"]
		if bridgeName != "" {
			bridges = append(bridges, bridgeName)
			bridgeNodes[bridgeName] = detail["node"]
			bridgeDescriptions[bridgeName] = detail["description"]
		}
	}

	// Add bridge data for template
	data["Bridges"] = bridges
	data["BridgeNodes"] = bridgeNodes
	data["BridgeDescriptions"] = bridgeDescriptions

	// Add active node for template compatibility
	data["ActiveNode"] = activeNode

	// Add default pool and available tags for template compatibility
	data["DefaultPool"] = fmt.Sprintf("pvmss_%s", username)
	data["AvailableTags"] = settings.Tags

	// Add CSRF token from request context
	if csrfToken, ok := r.Context().Value("csrf_token").(string); ok {
		data["CSRFToken"] = csrfToken
	}

	// Check if all nodes are disabled (saturated)
	allNodesSaturated := len(nodeOptions) > 0
	for _, option := range nodeOptions {
		disabled, ok := option["disabled"].(bool)
		if !ok || !disabled {
			allNodesSaturated = false
			break
		}
	}

	// Check if there are no nodes available at all
	noNodesAvailable := len(nodes) == 0

	if allNodesSaturated {
		data["Notification"] = map[string]interface{}{
			"type":  "warning",
			"title": "Resource Limits Reached",
			"text":  "All nodes have reached their PVMSS resource limits. Cannot create new VMs.",
		}
	}

	// Add no nodes available flag for template
	data["NoNodesAvailable"] = noNodesAvailable

	log.Debug().
		Int("data_keys", len(data)).
		Bool("all_nodes_saturated", allNodesSaturated).
		Msg("About to render create_vm template")

	// Handle POST requests for VM creation
	if r.Method == "POST" {
		log.Debug().Msg("Processing VM creation POST request")

		// Parse form
		if err := r.ParseForm(); err != nil {
			log.Error().Err(err).Msg("Failed to parse VM creation form")
			data["ValidationError"] = "Invalid form data"
			renderTemplateInternal(w, r, "create_vm", data)
			return
		}

		// Call the creation handler
		h.handleVMCreation(w, r, client, data)
		return
	}

	renderTemplateInternal(w, r, "create_vm", data)
	log.Debug().Msg("Template rendered successfully")
}

// getOptimizedNodeInfo retrieves node information with caching
func (h *VMCreateOptimizedHandler) getOptimizedNodeInfo(ctx context.Context, client proxmox.ClientInterface) ([]string, map[string]bool, string, error) {
	log := CreateHandlerLogger("getOptimizedNodeInfo", nil)

	if client == nil {
		return nil, nil, "", fmt.Errorf("proxmox client not available")
	}

	// Create resty client
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to create resty client: %w", err)
	}

	// Get node names with timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	nodes, err := proxmox.GetNodeNamesResty(ctx, restyClient)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to get node names: %w", err)
	}

	log.Info().Int("node_count", len(nodes)).Msg("Retrieved node names")

	// Get settings to check node limits
	settings := h.stateManager.GetSettings()
	if settings == nil {
		log.Warn().Msg("Settings not available, using all nodes as enabled")
		return nodes, make(map[string]bool), nodes[0], nil
	}

	// Check which nodes are disabled (saturated)
	disabledNodes := make(map[string]bool)
	for _, nodeName := range nodes {
		// TODO: Implement actual resource checking logic here
		// For now, assume nodes are enabled
		disabledNodes[nodeName] = false
	}

	// Select active node (first non-disabled)
	activeNode := ""
	for _, nodeName := range nodes {
		if !disabledNodes[nodeName] {
			activeNode = nodeName
			break
		}
	}
	if activeNode == "" && len(nodes) > 0 {
		activeNode = nodes[0] // Fallback to first node
	}

	log.Info().
		Str("active_node", activeNode).
		Int("disabled_nodes", countDisabledNodes(disabledNodes)).
		Msg("Node information retrieved")

	return nodes, disabledNodes, activeNode, nil
}

// getOptimizedResources retrieves storages and bridges concurrently with optimizations
func (h *VMCreateOptimizedHandler) getOptimizedResources(ctx context.Context, client proxmox.ClientInterface, nodes []string, disabledNodes map[string]bool, settings *state.AppSettings) ([]string, map[string]string, []map[string]string, error) {
	log := CreateHandlerLogger("getOptimizedResources", nil)

	if client == nil || len(nodes) == 0 {
		return nil, nil, nil, fmt.Errorf("proxmox client not available or no nodes")
	}

	// Create resty client
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create resty client: %w", err)
	}

	// Use shorter timeout for better UX
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var storages []string
	var storageNodes map[string]string
	var bridgeDetails []map[string]string
	var storagesErr, bridgesErr error

	// Get storages concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		storages, storageNodes, storagesErr = h.getOptimizedStorages(ctx, restyClient, nodes, disabledNodes, settings)
	}()

	// Get bridges concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		bridgeDetails, bridgesErr = h.getOptimizedBridges(ctx, restyClient, nodes, disabledNodes, settings)
	}()

	wg.Wait()

	if storagesErr != nil {
		return nil, nil, nil, fmt.Errorf("failed to get storages: %w", storagesErr)
	}
	if bridgesErr != nil {
		return nil, nil, nil, fmt.Errorf("failed to get bridges: %w", bridgesErr)
	}

	log.Info().
		Int("storages_count", len(storages)).
		Int("bridges_count", len(bridgeDetails)).
		Msg("Resources retrieved concurrently")

	return storages, storageNodes, bridgeDetails, nil
}

// getOptimizedStorages retrieves storage information with batch processing
func (h *VMCreateOptimizedHandler) getOptimizedStorages(ctx context.Context, restyClient *proxmox.RestyClient, nodes []string, disabledNodes map[string]bool, settings *state.AppSettings) ([]string, map[string]string, error) {
	log := CreateHandlerLogger("getOptimizedStorages", nil)

	// Get global storage list once
	globalList, err := proxmox.GetStoragesResty(ctx, restyClient)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to fetch global storage list")
		// Continue without global metadata
	}

	// Create global storage info map for quick lookup
	globalStorageInfo := make(map[string]proxmox.Storage)
	for _, item := range globalList {
		globalStorageInfo[item.Storage] = item
	}

	// Create enabled storage map for quick lookup
	enabledStorageMap := make(map[string]bool)
	for _, enabledStorage := range settings.EnabledStorages {
		enabledStorageMap[enabledStorage] = true
	}

	// Collect storages from all enabled nodes
	storageMap := make(map[string]string) // storage -> node
	var mu sync.Mutex

	// Use semaphore to limit concurrent API calls
	semaphore := make(chan struct{}, 5) // Max 5 concurrent storage calls

	var wg sync.WaitGroup
	for _, nodeName := range nodes {
		if disabledNodes[nodeName] {
			continue // Skip disabled nodes
		}

		wg.Add(1)
		go func(nodeName string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			storageList, err := proxmox.GetNodeStoragesResty(ctx, restyClient, nodeName)
			if err != nil {
				log.Warn().Err(err).Str("node", nodeName).Msg("Failed to retrieve storages for node")
				return
			}

			for _, storage := range storageList {
				// Enrich with global info if available
				storageInfo := storage
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

				// Check if storage should be included
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
					mu.Lock()
					// Only add if not already present (avoid duplicates across nodes)
					if _, exists := storageMap[storage.Storage]; !exists {
						storageMap[storage.Storage] = nodeName
					}
					mu.Unlock()
				}
			}
		}(nodeName)
	}

	wg.Wait()

	// Convert map to sorted slice
	storages := make([]string, 0, len(storageMap))
	for storage := range storageMap {
		storages = append(storages, storage)
	}
	sort.Strings(storages)

	log.Info().
		Int("unique_storages", len(storages)).
		Int("nodes_checked", len(nodes)).
		Msg("Storages retrieved with optimization")

	return storages, storageMap, nil
}

// getOptimizedBridges retrieves bridge information with batch processing
func (h *VMCreateOptimizedHandler) getOptimizedBridges(ctx context.Context, restyClient *proxmox.RestyClient, nodes []string, disabledNodes map[string]bool, settings *state.AppSettings) ([]map[string]string, error) {
	log := CreateHandlerLogger("getOptimizedBridges", nil)

	bridgeNodes := make(map[string]string)
	bridgeDescriptions := make(map[string]string)
	var mu sync.Mutex

	// Use semaphore to limit concurrent API calls
	semaphore := make(chan struct{}, 5) // Max 5 concurrent bridge calls

	var wg sync.WaitGroup
	for _, nodeName := range nodes {
		if disabledNodes[nodeName] {
			continue // Skip disabled nodes
		}

		wg.Add(1)
		go func(nodeName string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			vmbrs, err := proxmox.GetVMBRsResty(ctx, restyClient, nodeName)
			if err != nil {
				log.Warn().Err(err).Str("node", nodeName).Msg("Failed to retrieve VMBRs")
				return
			}

			for _, vmbr := range vmbrs {
				name := getVMBRInterface(vmbr)
				if name == "" {
					continue
				}

				mu.Lock()
				if _, exists := bridgeNodes[name]; !exists {
					bridgeNodes[name] = nodeName
				}
				if desc, exists := bridgeDescriptions[name]; exists && desc != "" {
					// Description already exists, skip
				} else {
					bridgeDescriptions[name] = buildVMBRDescription(vmbr)
				}
				mu.Unlock()
			}
		}(nodeName)
	}

	wg.Wait()

	// Build bridge details
	var bridgeDetails []map[string]string
	for _, bridgeIdentifier := range settings.VMBRs {
		// Extract bridge name from node:vmbr format
		bridgeName := bridgeIdentifier
		if colonIndex := strings.Index(bridgeIdentifier, ":"); colonIndex != -1 {
			bridgeName = bridgeIdentifier[colonIndex+1:]
		}

		bridgeDetails = append(bridgeDetails, map[string]string{
			"description": bridgeDescriptions[bridgeName],
			"name":        bridgeName,
			"node":        bridgeNodes[bridgeName],
		})
	}

	log.Info().
		Int("unique_bridges", len(bridgeDetails)).
		Int("nodes_checked", len(nodes)).
		Msg("Bridges retrieved with optimization")

	return bridgeDetails, nil
}

// handleVMCreation processes the VM creation form submission
func (h *VMCreateOptimizedHandler) handleVMCreation(w http.ResponseWriter, r *http.Request, client proxmox.ClientInterface, data map[string]interface{}) {
	log := CreateHandlerLogger("handleVMCreation", r)
	ctx := r.Context()

	if client == nil {
		data["ValidationError"] = "Proxmox client not available"
		renderTemplateInternal(w, r, "create_vm", data)
		return
	}

	// Extract form values
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	vmidStr := strings.TrimSpace(r.FormValue("vmid"))
	node := strings.TrimSpace(r.FormValue("node"))
	pool := strings.TrimSpace(r.FormValue("pool"))
	storage := strings.TrimSpace(r.FormValue("storage"))
	isoImage := strings.TrimSpace(r.FormValue("iso_image"))
	bridgeName := strings.TrimSpace(r.FormValue("bridge_0"))
	networkModel := strings.TrimSpace(r.FormValue("network_model"))
	diskBus := strings.TrimSpace(r.FormValue("disk_bus"))
	tags := strings.TrimSpace(r.FormValue("tags"))
	enableEFI := r.FormValue("enable_efi")
	enableTPM := r.FormValue("enable_tpm")

	// Parse numeric values
	memoryMBStr := strings.TrimSpace(r.FormValue("memory_mb"))
	cpuSocketsStr := strings.TrimSpace(r.FormValue("cpu_sockets"))
	cpuCoresStr := strings.TrimSpace(r.FormValue("cpu_cores"))
	diskSizeGBStr := strings.TrimSpace(r.FormValue("disk_gb"))

	// Simple validation
	if name == "" {
		data["ValidationError"] = "VM name is required"
		renderTemplateInternal(w, r, "create_vm", data)
		return
	}

	if storage == "" {
		data["ValidationError"] = "Storage is required"
		renderTemplateInternal(w, r, "create_vm", data)
		return
	}

	// Parse integers
	memoryMB := 2048
	if memoryMBStr != "" {
		if val, err := fmt.Sscanf(memoryMBStr, "%d", &memoryMB); err != nil || val != 1 {
			memoryMB = 2048
		}
	}

	cpuSockets := 1
	if cpuSocketsStr != "" {
		if val, err := fmt.Sscanf(cpuSocketsStr, "%d", &cpuSockets); err != nil || val != 1 {
			cpuSockets = 1
		}
	}

	cpuCores := 2
	if cpuCoresStr != "" {
		if val, err := fmt.Sscanf(cpuCoresStr, "%d", &cpuCores); err != nil || val != 1 {
			cpuCores = 2
		}
	}

	diskSizeGB := 20
	if diskSizeGBStr != "" {
		if val, err := fmt.Sscanf(diskSizeGBStr, "%d", &diskSizeGB); err != nil || val != 1 {
			diskSizeGB = 20
		}
	}

	// Defaults
	if diskBus == "" {
		diskBus = "virtio"
	}
	if networkModel == "" {
		networkModel = "virtio"
	}

	// Get or generate VMID
	vmid := 0
	if vmidStr != "" {
		if val, err := fmt.Sscanf(vmidStr, "%d", &vmid); err != nil || val != 1 {
			vmid = 0
		}
	}
	if vmid == 0 {
		restyClient, err := getDefaultRestyClient()
		if err != nil {
			data["ValidationError"] = "Failed to get next VMID"
			renderTemplateInternal(w, r, "create_vm", data)
			return
		}
		nextID, err := proxmox.GetNextVMIDResty(ctx, restyClient)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get next VMID")
			data["ValidationError"] = "Failed to get next VMID"
			renderTemplateInternal(w, r, "create_vm", data)
			return
		}
		vmid = nextID
	}

	// Build Proxmox parameters
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

	// Boot and ISO
	if isoImage != "" {
		params.Set("ide2", isoImage+",media=cdrom")
		params.Set("boot", "order=ide2;"+diskBus+"0")
	} else {
		params.Set("boot", "order="+diskBus+"0")
	}

	// EFI
	if enableEFI == "1" {
		params.Set("bios", "ovmf")
		params.Set("efidisk0", storage+":1,format=raw,efitype=4m")
	}

	// TPM
	if enableTPM == "1" {
		params.Set("tpmstate0", storage+":4,version=v2.0")
	}

	// Disk
	diskParam := diskBus + "0"
	params.Set(diskParam, fmt.Sprintf("%s:%d", storage, diskSizeGB))
	if diskBus == "scsi" {
		params.Set("scsihw", "virtio-scsi-pci")
	}

	// Network - create if bridge selected, enable/disable based on checkbox
	networkEnabled := r.FormValue("network_enabled_0") == "1"
	if bridgeName != "" {
		netConfig := networkModel + ",bridge=" + bridgeName
		if !networkEnabled {
			netConfig += ",link_down=1"
		}
		params.Set("net0", netConfig)
	}

	// Agent
	params.Set("agent", "1")

	// Create VM
	path := "/nodes/" + url.PathEscape(node) + "/qemu"
	if _, err := client.PostFormWithContext(ctx, path, params); err != nil {
		log.Error().Err(err).Str("node", node).Msg("VM create API call failed")
		data["ValidationError"] = fmt.Sprintf("Failed to create VM: %v", err)
		renderTemplateInternal(w, r, "create_vm", data)
		return
	}

	// Invalidate caches
	client.InvalidateCache("/nodes/" + url.PathEscape(node) + "/qemu")
	if pool != "" {
		client.InvalidateCache("/pools/" + url.PathEscape(pool))
	}

	log.Info().
		Int("vmid", vmid).
		Str("name", name).
		Str("node", node).
		Msg("VM created successfully")

	// Redirect to VM details
	http.Redirect(w, r, fmt.Sprintf("/vm/details/%d?refresh=1", vmid), http.StatusSeeOther)
}

// Helper functions
func countDisabledNodes(disabledNodes map[string]bool) int {
	count := 0
	for _, disabled := range disabledNodes {
		if disabled {
			count++
		}
	}
	return count
}
