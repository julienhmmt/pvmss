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
	"golang.org/x/sync/errgroup"

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
	settingsPtr := h.stateManager.GetSettings()
	if settingsPtr == nil {
		log.Error().Msg("Settings not available")
		localizer := i18n.GetLocalizerFromRequest(r)
		http.Error(w, i18n.Localize(localizer, "Error.SettingsUnavailable"), http.StatusInternalServerError)
		return
	}
	settings := *settingsPtr

	// Get Proxmox client and connection status
	client := h.stateManager.GetProxmoxClient()
	proxmoxConnected := client != nil && !h.stateManager.IsOfflineMode()

	// Get node information
	log.Debug().Msg("Getting node information")
	nodes, disabledNodes, activeNode, err := h.getOptimizedNodeInfo(r.Context(), client)
	if err != nil {
		log.Warn().Err(err).Msg("Proxmox node information unavailable, falling back to settings")
		nodes, disabledNodes, activeNode = deriveNodesFromSettings(settingsPtr)
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
			option["reason"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "VM.Create.NodeLimitReached")
		}
		nodeOptions = append(nodeOptions, option)
	}
	log.Debug().Int("node_options_count", len(nodeOptions)).Msg("Node options built")

	// Get storages and bridges concurrently
	log.Debug().Msg("Getting resources (storages and bridges)")
	storages, storageNodes, bridgeDetails, err := h.getOptimizedResources(r.Context(), client, nodes, disabledNodes, settingsPtr)
	if err != nil {
		log.Warn().Err(err).Msg("Proxmox resources unavailable, using settings fallback")
		storages, storageNodes, bridgeDetails = buildResourcesFromSettings(settingsPtr)
	}
	if storages == nil {
		storages = []string{}
	}
	if storageNodes == nil {
		storageNodes = make(map[string]string)
	}
	if bridgeDetails == nil {
		bridgeDetails = []map[string]string{}
	}
	log.Debug().Int("storages_count", len(storages)).Int("bridges_count", len(bridgeDetails)).Msg("Resources retrieved")

	localizer := i18n.GetLocalizerFromRequest(r)

	// Prepare form data
	formData := map[string]string{
		"bridge_0":          "",
		"cores":             "1",
		"sockets":           "1",
		"description":       "",
		"disk_bus_type":     "virtio",
		"disk_size_0":       "12",
		"enable_efi":        "1",
		"enable_tpm":        "",
		"iso":               "",
		"memory":            "1024",
		"name":              "",
		"network_enabled_0": "1", // First network card enabled by default
		"network_model_0":   "virtio",
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

	// Prepare template data with safe defaults
	isos := settings.ISOs
	if isos == nil {
		isos = []string{}
	}
	limits := make(map[string]interface{})
	// Convert LimitsConfig to map[string]interface{} for template compatibility
	limits["vm"] = map[string]interface{}{
		"sockets": map[string]int{"min": settings.Limits.VM.Sockets.Min, "max": settings.Limits.VM.Sockets.Max},
		"cores":   map[string]int{"min": settings.Limits.VM.Cores.Min, "max": settings.Limits.VM.Cores.Max},
		"ram":     map[string]int{"min": settings.Limits.VM.RAM.Min, "max": settings.Limits.VM.RAM.Max},
		"disk":    map[string]int{"min": settings.Limits.VM.Disk.Min, "max": settings.Limits.VM.Disk.Max},
	}
	if len(settings.Limits.Nodes) > 0 {
		nodesLimits := make(map[string]interface{}, len(settings.Limits.Nodes))
		for nodeName, nodeLimits := range settings.Limits.Nodes {
			nodesLimits[nodeName] = map[string]interface{}{
				"sockets": map[string]int{"min": nodeLimits.Sockets.Min, "max": nodeLimits.Sockets.Max},
				"cores":   map[string]int{"min": nodeLimits.Cores.Min, "max": nodeLimits.Cores.Max},
				"ram":     map[string]int{"min": nodeLimits.RAM.Min, "max": nodeLimits.RAM.Max},
				"disk":    map[string]int{"min": nodeLimits.Disk.Min, "max": nodeLimits.Disk.Max},
			}
		}
		limits["nodes"] = nodesLimits
	} else {
		limits["nodes"] = map[string]interface{}{}
	}
	maxDiskPerVM := settings.MaxDiskPerVM
	if maxDiskPerVM == 0 {
		maxDiskPerVM = 1
	}
	maxNetworkCards := settings.MaxNetworkCards
	if maxNetworkCards == 0 {
		maxNetworkCards = 1
	}

	data := map[string]interface{}{
		"BridgeDetails":    bridgeDetails,
		"FormData":         formData,
		"ISOs":             isos,
		"IsAdmin":          isAdmin,
		"IsAuthenticated":  true,
		"Lang":             i18n.GetLanguage(r),
		"Limits":           limits,
		"MaxDiskPerVM":     maxDiskPerVM,
		"MaxNetworkCards":  maxNetworkCards,
		"NetworkModels":    getNetworkModels(),
		"NodeOptions":      nodeOptions,
		"Nodes":            nodes,
		"NodesLimits":      getNodeLimits(settingsPtr),
		"ProxmoxConnected": proxmoxConnected,
		"Storages":         storages,
		"StorageNodes":     storageNodes,
		"Success":          "",
		"SuccessMessage":   "",
		"Username":         username,
	}

	// Check for offline nodes and create notification
	offlineNodesCount := h.getOfflineNodesCount(r.Context(), client)
	if offlineNodesCount > 0 {
		title := i18n.Localize(localizer, "VM.Create.OfflineNodesTitle")
		message := i18n.Localize(localizer, "VM.Create.OfflineNodesMessage", offlineNodesCount)
		data["OfflineNodesNotification"] = map[string]interface{}{
			"type":  "info",
			"title": title,
			"text":  message,
		}
		log.Info().Int("offline_nodes", offlineNodesCount).Msg("Some cluster nodes are offline")
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
	availableTags := settings.Tags
	if availableTags == nil {
		availableTags = []string{}
	}
	data["AvailableTags"] = availableTags

	// Add CSRF token from request context
	if csrfToken, ok := r.Context().Value("csrf_token").(string); ok {
		data["CSRFToken"] = csrfToken
	}

	// Compute explicit VM limits for template (avoids nil map indexing in templates)
	// IMPORTANT: Limits MUST come from settings.json (NO hardcoded defaults)
	// In settings.json: ram and disk are in GB, sockets and cores are integers
	var vmRamMinMB, vmRamMaxMB int
	var socketsMin, socketsMax int
	var coresMin, coresMax int
	var diskMin, diskMax int

	if settings.Limits.VM.Sockets.Min > 0 {
		// Extract limits from typed structs
		vmRamMinMB = settings.Limits.VM.RAM.Min * 1024
		vmRamMaxMB = settings.Limits.VM.RAM.Max * 1024
		socketsMin = settings.Limits.VM.Sockets.Min
		socketsMax = settings.Limits.VM.Sockets.Max
		coresMin = settings.Limits.VM.Cores.Min
		coresMax = settings.Limits.VM.Cores.Max
		diskMin = settings.Limits.VM.Disk.Min
		diskMax = settings.Limits.VM.Disk.Max
	}

	// Verify we got all required limits from settings
	if vmRamMinMB == 0 || vmRamMaxMB == 0 || coresMin == 0 || coresMax == 0 || socketsMin == 0 || socketsMax == 0 || diskMin == 0 || diskMax == 0 {
		log.Error().Msg("Incomplete VM limits in settings.json")
		data["ValidationError"] = "Incomplete system configuration. Please contact administrator."
		renderTemplateInternal(w, r, "vm_create", data)
		return
	}

	// derive GB for display as ceiling of MB/1024
	vmRamMinGB := (vmRamMinMB + 1023) / 1024
	vmRamMaxGB := (vmRamMaxMB + 1023) / 1024
	data["VMSocketsMin"], data["VMSocketsMax"] = socketsMin, socketsMax
	data["VMCoresMin"], data["VMCoresMax"] = coresMin, coresMax
	data["VMRamMinGB"], data["VMRamMaxGB"] = vmRamMinGB, vmRamMaxGB
	data["VMRamMinMB"], data["VMRamMaxMB"] = vmRamMinMB, vmRamMaxMB
	data["VMDiskMin"], data["VMDiskMax"] = diskMin, diskMax

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
			"title": i18n.Localize(localizer, "VM.Create.ResourceLimitsTitle"),
			"text":  i18n.Localize(localizer, "VM.Create.ResourceLimitsMessage"),
		}
	}

	// Add no nodes available flag for template
	data["NoNodesAvailable"] = noNodesAvailable
	data["AllNodesSaturated"] = allNodesSaturated

	log.Debug().
		Int("data_keys", len(data)).
		Bool("all_nodes_saturated", allNodesSaturated).
		Msg("About to render vm_create template")

	// Handle POST requests for VM creation
	if r.Method == "POST" {
		log.Debug().Msg("Processing VM creation POST request")

		// Parse form
		if err := r.ParseForm(); err != nil {
			log.Error().Err(err).Msg("Failed to parse VM creation form")
			data["ValidationError"] = i18n.Localize(localizer, "Error.InvalidFormData")
			renderTemplateInternal(w, r, "vm_create", data)
			return
		}

		// Call the creation handler
		h.handleVMCreation(w, r, client, data)
		return
	}

	renderTemplateInternal(w, r, "vm_create", data)
	log.Debug().Msg("Template rendered successfully")
}

// getOptimizedNodeInfo retrieves node information with caching - only online nodes
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

	// Get online node names with timeout - skip offline/down nodes
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	nodes, err := proxmox.GetOnlineNodeNamesResty(ctx, restyClient)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to get online node names: %w", err)
	}
	if len(nodes) == 0 {
		return nil, nil, "", fmt.Errorf("no online nodes available")
	}

	log.Info().Int("node_count", len(nodes)).Msg("Retrieved online node names for VM creation")

	// Get settings to check node limits
	settings := h.stateManager.GetSettings()
	if settings == nil {
		log.Warn().Msg("Settings not available, using all online nodes as enabled")
		disabledNodes := make(map[string]bool, len(nodes))
		for _, nodeName := range nodes {
			disabledNodes[nodeName] = false
		}
		return nodes, disabledNodes, nodes[0], nil
	}

	// Check which nodes are disabled (saturated) - only among online nodes
	disabledNodes := make(map[string]bool)
	for _, nodeName := range nodes {
		// TODO: Implement actual resource checking logic here
		// For now, assume online nodes are enabled
		disabledNodes[nodeName] = false
	}

	// Select active node (first non-disabled online node)
	activeNode := ""
	for _, nodeName := range nodes {
		if !disabledNodes[nodeName] {
			activeNode = nodeName
			break
		}
	}
	if activeNode == "" && len(nodes) > 0 {
		activeNode = nodes[0] // Fallback to first online node
	}

	log.Info().
		Str("active_node", activeNode).
		Int("disabled_nodes", countDisabledNodes(disabledNodes)).
		Msg("Online node information retrieved for VM creation")

	return nodes, disabledNodes, activeNode, nil
}

func deriveNodesFromSettings(settings *state.AppSettings) ([]string, map[string]bool, string) {
	if settings == nil {
		return []string{}, make(map[string]bool), ""
	}

	nodeSet := make(map[string]struct{})
	for nodeName := range settings.Limits.Nodes {
		if nodeName != "" {
			nodeSet[nodeName] = struct{}{}
		}
	}
	for _, vmbr := range settings.VMBRs {
		if nodeName := extractPrefix(vmbr); nodeName != "" {
			nodeSet[nodeName] = struct{}{}
		}
	}
	for _, storage := range settings.EnabledStorages {
		if nodeName := extractPrefix(storage); nodeName != "" {
			nodeSet[nodeName] = struct{}{}
		}
	}

	nodes := make([]string, 0, len(nodeSet))
	for nodeName := range nodeSet {
		nodes = append(nodes, nodeName)
	}
	sort.Strings(nodes)

	disabled := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		disabled[node] = false
	}

	active := ""
	if len(nodes) > 0 {
		active = nodes[0]
	}

	return nodes, disabled, active
}

func buildResourcesFromSettings(settings *state.AppSettings) ([]string, map[string]string, []map[string]string) {
	if settings == nil {
		return []string{}, map[string]string{}, []map[string]string{}
	}

	storageSet := make(map[string]struct{})
	storageNodes := make(map[string]string)
	for _, entry := range settings.EnabledStorages {
		storageName := entry
		nodeName := ""
		if idx := strings.Index(entry, ":"); idx > -1 {
			nodeName = entry[:idx]
			storageName = entry[idx+1:]
		}
		if storageName == "" {
			continue
		}
		if nodeName != "" {
			storageNodes[storageName] = nodeName
		}
		storageSet[storageName] = struct{}{}
	}

	storages := make([]string, 0, len(storageSet))
	for storageName := range storageSet {
		storages = append(storages, storageName)
	}
	sort.Strings(storages)

	bridgeDetails := make([]map[string]string, 0, len(settings.VMBRs))
	for _, identifier := range settings.VMBRs {
		nodeName := ""
		bridgeName := identifier
		if idx := strings.Index(identifier, ":"); idx > -1 {
			nodeName = identifier[:idx]
			bridgeName = identifier[idx+1:]
		}
		if bridgeName == "" {
			continue
		}
		bridgeDetails = append(bridgeDetails, map[string]string{
			"name":        bridgeName,
			"node":        nodeName,
			"description": "",
		})
	}

	sort.Slice(bridgeDetails, func(i, j int) bool {
		if bridgeDetails[i]["name"] == bridgeDetails[j]["name"] {
			return bridgeDetails[i]["node"] < bridgeDetails[j]["node"]
		}
		return bridgeDetails[i]["name"] < bridgeDetails[j]["name"]
	})

	return storages, storageNodes, bridgeDetails
}

func extractPrefix(identifier string) string {
	if identifier == "" {
		return ""
	}
	if idx := strings.Index(identifier, ":"); idx > -1 {
		return identifier[:idx]
	}
	return ""
}

// getOfflineNodesCount counts the number of offline/down nodes in the cluster
func (h *VMCreateOptimizedHandler) getOfflineNodesCount(ctx context.Context, client proxmox.ClientInterface) int {
	log := CreateHandlerLogger("getOfflineNodesCount", nil)

	if client == nil {
		return 0
	}

	// Create resty client
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create resty client for offline node check")
		return 0
	}

	// Get ALL nodes (including offline) with timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	allNodes, err := proxmox.GetNodeNamesResty(ctx, restyClient)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get all node names for offline check")
		return 0
	}

	// Get online nodes only
	onlineNodes, err := proxmox.GetOnlineNodeNamesResty(ctx, restyClient)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get online node names for offline check")
		return len(allNodes) // Assume all are offline if we can't get online list
	}

	offlineCount := len(allNodes) - len(onlineNodes)
	log.Debug().Int("total_nodes", len(allNodes)).Int("online_nodes", len(onlineNodes)).Int("offline_nodes", offlineCount).Msg("Offline node count calculated")

	return offlineCount
}

// getNetworkModels returns the available network card models for VM creation
func getNetworkModels() []map[string]string {
	return []map[string]string{
		{"value": "virtio", "label": "VirtIO"},
		{"value": "e1000", "label": "E1000"},
		{"value": "e1000e", "label": "E1000E"},
		{"value": "rtl8139", "label": "RTL8139"},
		{"value": "vmxnet3", "label": "VMXNet3"},
	}
}

// getNodeLimits extracts node limits from settings for template compatibility
func getNodeLimits(settings *state.AppSettings) map[string]interface{} {
	if settings == nil || len(settings.Limits.Nodes) == 0 {
		return make(map[string]interface{})
	}

	// Convert typed structs to map[string]interface{} for template compatibility
	result := make(map[string]interface{})
	for nodeName, nodeLimits := range settings.Limits.Nodes {
		nodeMap := map[string]interface{}{
			"sockets": map[string]int{"min": nodeLimits.Sockets.Min, "max": nodeLimits.Sockets.Max},
			"cores":   map[string]int{"min": nodeLimits.Cores.Min, "max": nodeLimits.Cores.Max},
			"ram":     map[string]int{"min": nodeLimits.RAM.Min, "max": nodeLimits.RAM.Max},
			"disk":    map[string]int{"min": nodeLimits.Disk.Min, "max": nodeLimits.Disk.Max},
		}
		result[nodeName] = nodeMap
	}

	return result
}

// getOptimizedResources retrieves storages and bridges concurrently using errgroup pattern
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

	// Use errgroup for better concurrency control
	g, ctx := errgroup.WithContext(ctx)

	var storages []string
	var storageNodes map[string]string
	var bridgeDetails []map[string]string

	// Fetch storages concurrently
	g.Go(func() error {
		var err error
		storages, storageNodes, err = h.getOptimizedStorages(ctx, restyClient, nodes, disabledNodes, settings)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to retrieve storages")
			return fmt.Errorf("failed to get storages: %w", err)
		}
		return nil
	})

	// Fetch bridges concurrently
	g.Go(func() error {
		var err error
		bridgeDetails, err = h.getOptimizedBridges(ctx, restyClient, nodes, disabledNodes, settings)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to retrieve bridges")
			return fmt.Errorf("failed to get bridges: %w", err)
		}
		return nil
	})

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		return nil, nil, nil, err
	}

	log.Info().
		Int("storages_count", len(storages)).
		Int("bridges_count", len(bridgeDetails)).
		Msg("Resources retrieved concurrently with errgroup")

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
				// Check if node:storage is enabled
				uniqueID := nodeName + ":" + storage.Storage
				isEnabledStorage := len(settings.EnabledStorages) == 0 || enabledStorageMap[uniqueID]
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

// getTPMDiskFormat returns the appropriate disk format for TPM based on storage type
// TPM always requires raw format, but we check storage compatibility
// Returns (format, isCompatible)
func getTPMDiskFormat(storageType string) (string, bool) {
	// TPM disk format is always raw (4 MiB fixed size)
	// Check if storage type supports raw format

	// Block-based storages that support raw format
	blockStorages := map[string]bool{
		"ceph":    true,
		"iscsi":   true,
		"lvm":     true,
		"lvmthin": true,
		"rbd":     true,
		"zfs":     true,
	}

	// File-based storages that support raw format
	fileStorages := map[string]bool{
		"dir":  true,
		"nfs":  true,
		"cifs": true,
	}

	// Check if storage type is compatible
	if blockStorages[storageType] || fileStorages[storageType] {
		return "raw", true
	}

	// Unknown or incompatible storage type
	return "raw", false
}

// handleVMCreation processes the VM creation form submission
func (h *VMCreateOptimizedHandler) handleVMCreation(w http.ResponseWriter, r *http.Request, client proxmox.ClientInterface, data map[string]interface{}) {
	log := CreateHandlerLogger("handleVMCreation", r)
	ctx := r.Context()

	if client == nil {
		data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.ProxmoxClientUnavailable")
		renderTemplateInternal(w, r, "vm_create", data)
		return
	}

	// Extract form values
	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	vmidStr := strings.TrimSpace(r.FormValue("vmid"))
	node := strings.TrimSpace(r.FormValue("node"))
	pool := strings.TrimSpace(r.FormValue("pool"))
	storage := strings.TrimSpace(r.FormValue("storage"))
	isoImage := strings.TrimSpace(r.FormValue("iso"))
	bridgeName := strings.TrimSpace(r.FormValue("bridge_0"))
	networkModel := strings.TrimSpace(r.FormValue("network_model_0"))
	diskBus := strings.TrimSpace(r.FormValue("disk_bus_type"))
	tags := strings.TrimSpace(r.FormValue("tags"))
	enableEFI := r.FormValue("enable_efi")
	enableTPM := r.FormValue("enable_tpm")

	// Parse numeric values
	memoryStr := strings.TrimSpace(r.FormValue("memory"))
	memoryUnit := strings.TrimSpace(r.FormValue("memory_unit"))
	cpuSocketsStr := strings.TrimSpace(r.FormValue("sockets"))
	cpuCoresStr := strings.TrimSpace(r.FormValue("cores"))
	diskSizeStr := strings.TrimSpace(r.FormValue("disk_size_0"))
	diskSizeUnit := strings.TrimSpace(r.FormValue("disk_size_unit_0"))

	// Simple validation
	if name == "" {
		data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "VM.Create.Validation.VMNameRequired")
		renderTemplateInternal(w, r, "vm_create", data)
		return
	}

	if storage == "" {
		data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "VM.Create.Validation.StorageRequired")
		renderTemplateInternal(w, r, "vm_create", data)
		return
	}

	// Parse integers with robust extraction
	settings := h.stateManager.GetSettings()
	if settings == nil {
		log.Error().Msg("Settings not available for VM creation")
		data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.SettingsUnavailable")
		renderTemplateInternal(w, r, "vm_create", data)
		return
	}

	// Helper function to extract integer from string with validation
	extractInt := func(str string, defaultValue int, minVal, maxVal int, fieldName string) int {
		if str == "" {
			return defaultValue
		}
		// Remove any non-digit characters and parse
		cleanStr := strings.TrimSpace(str)
		var value int
		if val, err := fmt.Sscanf(cleanStr, "%d", &value); err != nil || val != 1 {
			log.Warn().Str("field", fieldName).Str("input", str).Int("default", defaultValue).Msg("Invalid numeric value, using default")
			return defaultValue
		}
		// Validate range
		if value < minVal || value > maxVal {
			log.Warn().Str("field", fieldName).Int("value", value).Int("min", minVal).Int("max", maxVal).Int("default", defaultValue).Msg("Value out of range, using default")
			return defaultValue
		}
		return value
	}

	// Helper function to extract and convert memory/disk size
	extractSize := func(str string, unit string, defaultValue int, minVal, maxVal int, fieldName string) int {
		if str == "" {
			return defaultValue
		}
		// Parse as float for GB values
		cleanStr := strings.TrimSpace(str)
		var value float64
		if val, err := fmt.Sscanf(cleanStr, "%f", &value); err != nil || val != 1 {
			log.Warn().Str("field", fieldName).Str("input", str).Str("unit", unit).Int("default", defaultValue).Msg("Invalid numeric value, using default")
			return defaultValue
		}

		// Convert to MB based on unit
		var mb int
		if unit == "GB" {
			mb = int(value * 1024)
		} else {
			mb = int(value)
		}

		// Validate range
		if mb < minVal || mb > maxVal {
			log.Warn().Str("field", fieldName).Str("unit", unit).Float64("value", value).Int("mb", mb).Int("min", minVal).Int("max", maxVal).Int("default", defaultValue).Msg("Value out of range, using default")
			return defaultValue
		}
		return mb
	}

	// Get limits from settings.json (NO hardcoded defaults)
	// In settings.json: ram and disk are in GB, sockets and cores are integers
	var memoryMin, memoryMax int // Will be in MB after conversion
	var socketsMin, socketsMax int
	var coresMin, coresMax int
	var diskMin, diskMax int // Will be in GB from settings

	if settings.Limits.VM.Sockets.Min > 0 {
		// Extract limits from typed structs
		memoryMin = settings.Limits.VM.RAM.Min * 1024
		memoryMax = settings.Limits.VM.RAM.Max * 1024
		socketsMin = settings.Limits.VM.Sockets.Min
		socketsMax = settings.Limits.VM.Sockets.Max
		coresMin = settings.Limits.VM.Cores.Min
		coresMax = settings.Limits.VM.Cores.Max
		diskMin = settings.Limits.VM.Disk.Min
		diskMax = settings.Limits.VM.Disk.Max
	} else {
		// Settings not available - cannot create VM
		log.Error().Msg("Settings or limits not available for VM creation POST")
		data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.SystemUnavailable")
		renderTemplateInternal(w, r, "vm_create", data)
		return
	}

	// Verify we got all required limits
	if memoryMin == 0 || memoryMax == 0 || coresMin == 0 || coresMax == 0 || socketsMin == 0 || socketsMax == 0 || diskMin == 0 || diskMax == 0 {
		log.Error().Msg("Incomplete VM limits in settings.json for POST")
		data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.SystemUnavailable")
		renderTemplateInternal(w, r, "vm_create", data)
		return
	}

	// Parse values with proper validation
	// For memory: use min from settings as default
	// For disk: use min from settings as default (convert GB to MB)
	memoryMB := extractSize(memoryStr, memoryUnit, memoryMin, memoryMin, memoryMax, "memory")
	cpuSockets := extractInt(cpuSocketsStr, socketsMin, socketsMin, socketsMax, "sockets")
	cpuCores := extractInt(cpuCoresStr, coresMin, coresMin, coresMax, "cores")
	diskSizeMB := extractSize(diskSizeStr, diskSizeUnit, diskMin*1024, diskMin*1024, diskMax*1024, "disk_size")

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
			data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "VM.Create.Error.FailedToGetNextVMID")
			renderTemplateInternal(w, r, "vm_create", data)
			return
		}
		nextID, err := proxmox.GetNextVMIDResty(ctx, restyClient)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get next VMID")
			data["ValidationError"] = i18n.Localize(i18n.GetLocalizerFromRequest(r), "VM.Create.Error.FailedToGetNextVMID")
			renderTemplateInternal(w, r, "vm_create", data)
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

	// Primary disk (disk 0)
	diskParam := fmt.Sprintf("%s0", diskBus)
	params.Set(diskParam, fmt.Sprintf("%s:%d", storage, diskSizeMB/1024))
	log.Info().Str("disk_param", diskParam).Int("size_mb", diskSizeMB).Int("size_gb", diskSizeMB/1024).Msg("Configured primary disk")

	// TPM (Trusted Platform Module)
	if enableTPM == "1" {
		log.Info().Msg("TPM requested, checking storage compatibility")

		// Get storage info to determine format
		restyClient, err := getDefaultRestyClient()
		if err != nil {
			log.Warn().Err(err).Msg("Failed to create resty client for TPM storage check, skipping TPM")
		} else {
			storageInfo, err := proxmox.GetStoragesResty(r.Context(), restyClient)
			if err != nil {
				log.Warn().Err(err).Msg("Failed to retrieve storage info for TPM, skipping TPM")
			} else {
				// Find the selected storage
				var selectedStorage *proxmox.Storage
				for i := range storageInfo {
					if storageInfo[i].Storage == storage {
						selectedStorage = &storageInfo[i]
						break
					}
				}

				if selectedStorage != nil {
					// Check if storage is compatible with raw format (required for TPM)
					format, compatible := getTPMDiskFormat(selectedStorage.Type)
					if compatible {
						// Create TPM disk: tpmstate0=<storage>:4,version=v2.0
						// TPM disk is always 4 MiB and uses raw format
						params.Set("tpmstate0", fmt.Sprintf("%s:4,version=v2.0", storage))
						log.Info().
							Str("storage", storage).
							Str("storage_type", selectedStorage.Type).
							Str("format", format).
							Msg("TPM disk configured successfully")
					} else {
						log.Warn().
							Str("storage", storage).
							Str("storage_type", selectedStorage.Type).
							Msg("Storage type not compatible with TPM (requires raw format support), skipping TPM")
					}
				} else {
					log.Warn().Str("storage", storage).Msg("Selected storage not found in storage list, skipping TPM")
				}
			}
		}
	}

	// Additional disks
	if settings.MaxDiskPerVM > 1 {
		for diskIdx := 1; diskIdx < settings.MaxDiskPerVM; diskIdx++ {
			diskSizeStr := strings.TrimSpace(r.FormValue(fmt.Sprintf("disk_size_%d", diskIdx)))
			diskSizeUnit := strings.TrimSpace(r.FormValue(fmt.Sprintf("disk_size_unit_%d", diskIdx)))
			if diskSizeStr == "" || diskSizeStr == "0" {
				// Optional disk not configured, skip
				continue
			}
			// Convert to MB based on unit
			additionalDiskSizeMB := extractSize(diskSizeStr, diskSizeUnit, 0, 1024, 1048576, fmt.Sprintf("disk_size_%d", diskIdx))
			if additionalDiskSizeMB <= 0 {
				// Invalid size, skip
				log.Warn().Int("disk_idx", diskIdx).Str("size", diskSizeStr).Str("unit", diskSizeUnit).Msg("Invalid additional disk size, skipping")
				continue
			}
			// Create additional disk with same bus type (convert MB to GB for Proxmox)
			additionalDiskParam := fmt.Sprintf("%s%d", diskBus, diskIdx)
			params.Set(additionalDiskParam, fmt.Sprintf("%s:%d", storage, additionalDiskSizeMB/1024))
			log.Info().Int("disk_idx", diskIdx).Int("size_mb", additionalDiskSizeMB).Int("size_gb", additionalDiskSizeMB/1024).Str("param", additionalDiskParam).Msg("Added additional disk")
		}
	}

	// Network - Primary network card (net0)
	networkEnabled := r.FormValue("network_enabled_0") == "1"
	if bridgeName != "" {
		netConfig := networkModel + ",bridge=" + bridgeName
		if !networkEnabled {
			netConfig += ",link_down=1"
		}
		params.Set("net0", netConfig)
		log.Info().Str("bridge", bridgeName).Str("model", networkModel).Bool("enabled", networkEnabled).Msg("Configured primary network card")
	}

	// Additional network cards (net1, net2, etc.) if configured
	if settings.MaxNetworkCards > 1 {
		for netIdx := 1; netIdx < settings.MaxNetworkCards; netIdx++ {
			additionalBridge := strings.TrimSpace(r.FormValue(fmt.Sprintf("bridge_%d", netIdx)))
			if additionalBridge == "" {
				// Optional network card not configured, skip
				continue
			}
			// Get network model for this card
			additionalModel := strings.TrimSpace(r.FormValue(fmt.Sprintf("network_model_%d", netIdx)))
			if additionalModel == "" {
				additionalModel = "virtio" // Default
			}
			// Check if network is enabled
			additionalEnabled := r.FormValue(fmt.Sprintf("network_enabled_%d", netIdx)) == "1"
			// Build config
			additionalNetConfig := additionalModel + ",bridge=" + additionalBridge
			if !additionalEnabled {
				additionalNetConfig += ",link_down=1"
			}
			// Set parameter
			netParam := fmt.Sprintf("net%d", netIdx)
			params.Set(netParam, additionalNetConfig)
			log.Info().Int("net_idx", netIdx).Str("bridge", additionalBridge).Str("model", additionalModel).Bool("enabled", additionalEnabled).Msg("Added additional network card")
		}
	}

	// Agent
	params.Set("agent", "1")

	// Validate against aggregate node limits before creating the VM
	localizer := i18n.GetLocalizerFromRequest(r)
	if err := ValidateVMResourcesAgainstNodeLimits(ctx, client, h.stateManager, node, cpuSockets, cpuCores, memoryMB, localizer); err != nil {
		log.Warn().Err(err).Str("node", node).Msg("VM resources exceed aggregate node limits")
		data["ValidationError"] = err.Error()
		renderTemplateInternal(w, r, "vm_create", data)
		return
	}

	// Create VM
	path := "/nodes/" + url.PathEscape(node) + "/qemu"
	if _, err := client.PostFormWithContext(ctx, path, params); err != nil {
		log.Error().Err(err).Str("node", node).Msg("VM create API call failed")
		data["ValidationError"] = fmt.Sprintf("Failed to create VM: %v", err)
		renderTemplateInternal(w, r, "vm_create", data)
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

	// Audit log for admin VM creation
	if sessionManager := security.GetSession(r); sessionManager != nil {
		if isAdmin, ok := sessionManager.Get(r.Context(), "is_admin").(bool); ok && isAdmin {
			username := "unknown"
			proxmoxUsername := "unknown"

			if user, ok := sessionManager.Get(r.Context(), "username").(string); ok && user != "" {
				username = user
			}
			if pxUser, ok := sessionManager.Get(r.Context(), "pve_username").(string); ok && pxUser != "" {
				proxmoxUsername = pxUser
			}

			log.Info().
				Str("action", "vm_create").
				Str("admin_username", username).
				Str("proxmox_username", proxmoxUsername).
				Int("vmid", vmid).
				Str("vm_name", name).
				Str("node", node).
				Int("cpu_cores", cpuCores).
				Int("cpu_sockets", cpuSockets).
				Int("memory_mb", memoryMB).
				Int("disk_size_gb", diskSizeMB/1024).
				Str("storage", storage).
				Str("network_model", networkModel).
				Str("client_ip", r.RemoteAddr).
				Time("create_time", time.Now()).
				Msg("ADMIN ACTION AUDIT - VM created by admin")
		}
	}

	// Redirect to VM details
	http.Redirect(w, r, fmt.Sprintf("/vm/details/%d?created=1", vmid), http.StatusSeeOther)
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
