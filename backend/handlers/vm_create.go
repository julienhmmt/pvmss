package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"pvmss/i18n"
	"pvmss/proxmox"
	"pvmss/security"
	"pvmss/state"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/sync/errgroup"
)

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
	clusterSnapshot := h.stateManager.GetProxmoxSnapshot()

	// Get node information
	log.Debug().Msg("Getting node information")
	nodes, disabledNodes, activeNode, err := h.getOptimizedNodeInfo(r.Context(), client, clusterSnapshot)
	if err != nil {
		log.Warn().
			Err(err).
			Str("component", "vm_create").
			Str("operation", "get_node_info").
			Str("fallback", "settings").
			Msg("Proxmox node information unavailable; using settings fallback")
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
	storages, storageNodes, bridgeDetails, err := h.getOptimizedResources(r.Context(), client, nodes, disabledNodes, settingsPtr, clusterSnapshot)
	if err != nil {
		log.Warn().
			Err(err).
			Str("component", "vm_create").
			Str("operation", "get_resources").
			Str("fallback", "settings").
			Msg("Proxmox resources unavailable; using settings fallback")
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
		"bridge_0":           "",
		"cores":              "1",
		"sockets":            "1",
		"description":        "",
		"disk_bus_type":      "virtio",
		"disk_size_0":        "12",
		"enable_efi":         "1",
		"enable_tpm":         "",
		"iso":                "",
		"memory":             "1024",
		"name":               "",
		"network_enabled_0":  "1", // First network card enabled by default
		"network_model_0":    "virtio",
		"mac_address_0":      "",
		"vlan_tag_0":         "", // VLAN tag for first network card (optional)
		"node":               activeNode,
		"pool":               fmt.Sprintf("pvmss_%s", username),
		"storage":            "",
		"tags":               "",
		"vmid":               "",
		"cloudinit_enable":   "",
		"cloudinit_template": "",
		"cloudinit_user":     "",
		"cloudinit_sshkeys":  "",
		"cloudinit_ipconfig": "dhcp",
		"cloudinit_ip":       "",
		"cloudinit_gateway":  "",
		"cloudinit_dns":      "",
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
	} else {
		sort.Strings(isos)
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

	// Get enabled cloud-init templates
	cloudInitTemplates := settings.GetEnabledCloudInitTemplates()

	data := map[string]interface{}{
		"BridgeDetails":      bridgeDetails,
		"CloudInitTemplates": cloudInitTemplates,
		"AllowCustomYAML":    settings.AllowCustomYAML,
		"FormData":           formData,
		"ISOs":               isos,
		"IsAdmin":            isAdmin,
		"IsAuthenticated":    true,
		"Lang":               i18n.GetLanguage(r),
		"Limits":             limits,
		"MaxDiskPerVM":       maxDiskPerVM,
		"MaxNetworkCards":    maxNetworkCards,
		"NetworkModels":      getNetworkModels(),
		"NodeOptions":        nodeOptions,
		"Nodes":              nodes,
		"NodesLimits":        getNodeLimits(settingsPtr),
		"ProxmoxConnected":   proxmoxConnected,
		"Storages":           storages,
		"StorageNodes":       storageNodes,
		"Success":            "",
		"SuccessMessage":     "",
		"Username":           username,
	}

	// Check for offline nodes and create notification
	offlineNodesCount := h.getOfflineNodesCount(r.Context(), client, clusterSnapshot)
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
	sort.Strings(bridges)

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
	} else {
		sort.Strings(availableTags)
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
func (h *VMCreateOptimizedHandler) getOptimizedNodeInfo(ctx context.Context, client proxmox.ClientInterface, snapshot *state.ProxmoxClusterSnapshot) ([]string, map[string]bool, string, error) {
	log := CreateHandlerLogger("getOptimizedNodeInfo", nil)

	if snapshot != nil {
		candidates := snapshot.OnlineNodes
		if len(candidates) == 0 {
			candidates = snapshot.NodeNames
		}
		if len(candidates) > 0 {
			nodes := append([]string(nil), candidates...)
			sort.Strings(nodes)
			disabledNodes := make(map[string]bool, len(nodes))
			for _, node := range nodes {
				disabledNodes[node] = false
			}
			log.Info().Int("node_count", len(nodes)).Msg("Using cached Proxmox snapshot for node list")
			return nodes, disabledNodes, nodes[0], nil
		}
	}

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
func (h *VMCreateOptimizedHandler) getOfflineNodesCount(ctx context.Context, client proxmox.ClientInterface, snapshot *state.ProxmoxClusterSnapshot) int {
	log := CreateHandlerLogger("getOfflineNodesCount", nil)

	if snapshot != nil {
		totalNodes := len(snapshot.NodeNames)
		onlineNodes := len(snapshot.OnlineNodes)
		if totalNodes > 0 {
			if onlineNodes > totalNodes {
				onlineNodes = totalNodes
			}
			offline := totalNodes - onlineNodes
			if offline < 0 {
				offline = 0
			}
			log.Debug().Int("offline_nodes", offline).Msg("Offline node count served from snapshot")
			return offline
		}
	}

	if client == nil {
		return 0
	}

	// Create resty client
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Warn().
			Err(err).
			Str("component", "vm_create").
			Str("operation", "offline_node_check").
			Str("reason", "resty_client_failed").
			Msg("Failed to create resty client for offline node check")
		return 0
	}

	// Get ALL nodes (including offline) with timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	allNodes, err := proxmox.GetNodeNamesResty(ctx, restyClient)
	if err != nil {
		log.Warn().
			Err(err).
			Str("component", "vm_create").
			Str("operation", "offline_node_check").
			Str("reason", "all_nodes_failed").
			Msg("Failed to get all node names for offline check")
		return 0
	}

	// Get online nodes only
	onlineNodes, err := proxmox.GetOnlineNodeNamesResty(ctx, restyClient)
	if err != nil {
		log.Warn().
			Err(err).
			Str("component", "vm_create").
			Str("operation", "offline_node_check").
			Str("reason", "online_nodes_failed").
			Int("total_nodes", len(allNodes)).
			Msg("Failed to get online node names for offline check, assuming all nodes are offline")
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
func (h *VMCreateOptimizedHandler) getOptimizedResources(ctx context.Context, client proxmox.ClientInterface, nodes []string, disabledNodes map[string]bool, settings *state.AppSettings, snapshot *state.ProxmoxClusterSnapshot) ([]string, map[string]string, []map[string]string, error) {
	log := CreateHandlerLogger("getOptimizedResources", nil)

	if len(nodes) == 0 {
		return nil, nil, nil, fmt.Errorf("no nodes available")
	}

	if snapshot != nil {
		if storages, storageNodes, err := h.getOptimizedStoragesFromSnapshot(snapshot, nodes, disabledNodes, settings); err == nil {
			if bridgeDetails, err := h.getOptimizedBridgesFromSnapshot(snapshot, nodes, disabledNodes, settings); err == nil {
				log.Info().Msg("Served VM creation resources from cached Proxmox snapshot")
				return storages, storageNodes, bridgeDetails, nil
			}
			log.Warn().
				Err(err).
				Str("component", "vm_create").
				Str("operation", "get_optimized_resources").
				Str("reason", "snapshot_bridges_failed").
				Str("fallback", "live_calls").
				Msg("Bridge details unavailable in snapshot; using live calls fallback")
		} else {
			log.Warn().
				Err(err).
				Str("component", "vm_create").
				Str("operation", "get_optimized_resources").
				Str("reason", "snapshot_storages_failed").
				Str("fallback", "live_calls").
				Msg("Storages unavailable in snapshot; using live calls fallback")
		}
	}

	if client == nil {
		return nil, nil, nil, fmt.Errorf("proxmox client not available and cached data missing")
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
			log.Warn().
				Err(err).
				Str("component", "vm_create").
				Str("operation", "get_optimized_resources").
				Str("reason", "storages_retrieval_failed").
				Msg("Failed to retrieve storages")
			return fmt.Errorf("failed to get storages: %w", err)
		}
		return nil
	})

	// Fetch bridges concurrently
	g.Go(func() error {
		var err error
		bridgeDetails, err = h.getOptimizedBridges(ctx, restyClient, nodes, disabledNodes, settings)
		if err != nil {
			log.Warn().
				Err(err).
				Str("component", "vm_create").
				Str("operation", "get_optimized_resources").
				Str("reason", "bridges_retrieval_failed").
				Msg("Failed to retrieve bridges")
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

func (h *VMCreateOptimizedHandler) getOptimizedStoragesFromSnapshot(snapshot *state.ProxmoxClusterSnapshot, nodes []string, disabledNodes map[string]bool, settings *state.AppSettings) ([]string, map[string]string, error) {
	log := CreateHandlerLogger("getOptimizedStoragesFromSnapshot", nil)
	if snapshot == nil || len(snapshot.NodeStorages) == 0 {
		return nil, nil, fmt.Errorf("snapshot does not include storage data")
	}

	globalStorageInfo := make(map[string]proxmox.Storage)
	for _, item := range snapshot.GlobalStorages {
		globalStorageInfo[item.Storage] = item
	}

	enabledStorageMap := make(map[string]bool)
	for _, enabledStorage := range settings.EnabledStorages {
		enabledStorageMap[enabledStorage] = true
	}

	storageMap := make(map[string]string)
	for _, nodeName := range nodes {
		if disabledNodes[nodeName] {
			continue
		}
		nodeStorages, ok := snapshot.NodeStorages[nodeName]
		if !ok {
			continue
		}
		for _, storage := range nodeStorages {
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
				if _, exists := storageMap[storage.Storage]; !exists {
					// For shared storage types like RBD, don't associate with a specific node
					if storageType == "rbd" {
						storageMap[storage.Storage] = "" // Empty string indicates shared storage
					} else {
						storageMap[storage.Storage] = nodeName // Local storage
					}
				}
			}
		}
	}

	if len(storageMap) == 0 {
		return nil, nil, fmt.Errorf("no storages found in snapshot")
	}

	storages := make([]string, 0, len(storageMap))
	for storage := range storageMap {
		storages = append(storages, storage)
	}
	sort.Strings(storages)

	log.Info().Int("snapshot_storages", len(storages)).Msg("Storages served from snapshot cache")
	return storages, storageMap, nil
}

// getOptimizedStorages retrieves storage information with batch processing
func (h *VMCreateOptimizedHandler) getOptimizedStorages(ctx context.Context, restyClient *proxmox.RestyClient, nodes []string, disabledNodes map[string]bool, settings *state.AppSettings) ([]string, map[string]string, error) {
	log := CreateHandlerLogger("getOptimizedStorages", nil)

	// Get global storage list once
	globalList, err := proxmox.GetStoragesResty(ctx, restyClient)
	if err != nil {
		log.Warn().
			Err(err).
			Str("component", "vm_create").
			Str("operation", "get_optimized_storages").
			Str("reason", "global_storage_list_failed").
			Msg("Failed to fetch global storage list; continuing without metadata")
		// Continue without global metadata
	} else {
		log.Debug().Int("global_storages_count", len(globalList)).Msg("Retrieved global storage list")
	}

	// Create global storage info map for quick lookup
	globalStorageInfo := make(map[string]proxmox.Storage)
	for _, item := range globalList {
		globalStorageInfo[item.Storage] = item
		log.Debug().
			Str("storage_name", item.Storage).
			Str("storage_type", item.Type).
			Str("storage_content", item.Content).
			Int("storage_enabled", item.Enabled).
			Msg("Processing global storage info")
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
				log.Warn().
					Err(err).
					Str("component", "vm_create").
					Str("operation", "get_optimized_storages").
					Str("node", nodeName).
					Str("reason", "node_storages_failed").
					Msg("Failed to retrieve storages for node")
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

				log.Debug().
					Str("node", nodeName).
					Str("storage", storage.Storage).
					Str("storage_type", storageType).
					Str("storage_content", storageContent).
					Bool("is_enabled_storage", isEnabledStorage).
					Int("storage_enabled", storage.Enabled).
					Bool("supports_vm_disk", supportsVMDisk).
					Msg("Evaluating storage for inclusion")

				if isEnabledStorage && storage.Enabled == 1 && supportsVMDisk {
					mu.Lock()
					// Only add if not already present (avoid duplicates across nodes)
					if _, exists := storageMap[storage.Storage]; !exists {
						// For shared storage types like RBD, don't associate with a specific node
						if storageType == "rbd" {
							storageMap[storage.Storage] = "" // Empty string indicates shared storage
							log.Debug().
								Str("storage", storage.Storage).
								Str("storage_type", storageType).
								Msg("Shared RBD storage accepted (available on all cluster nodes)")
						} else {
							storageMap[storage.Storage] = nodeName // Local storage
							log.Debug().
								Str("storage", storage.Storage).
								Str("node", nodeName).
								Msg("Local storage accepted and added to available list")
						}
					} else {
						log.Debug().
							Str("storage", storage.Storage).
							Str("existing_node", storageMap[storage.Storage]).
							Str("current_node", nodeName).
							Msg("Storage already exists from another node, skipping duplicate")
					}
					mu.Unlock()
				} else {
					log.Debug().
						Str("node", nodeName).
						Str("storage", storage.Storage).
						Msg("Storage rejected - does not meet criteria")
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

func (h *VMCreateOptimizedHandler) getOptimizedBridgesFromSnapshot(snapshot *state.ProxmoxClusterSnapshot, nodes []string, disabledNodes map[string]bool, settings *state.AppSettings) ([]map[string]string, error) {
	log := CreateHandlerLogger("getOptimizedBridgesFromSnapshot", nil)
	if snapshot == nil || len(snapshot.NetworkBridges) == 0 {
		return nil, fmt.Errorf("snapshot does not include network data")
	}

	bridgeNodes := make(map[string]string)
	bridgeDescriptions := make(map[string]string)
	nodeSet := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		nodeSet[n] = struct{}{}
	}

	for nodeName, vmbrs := range snapshot.NetworkBridges {
		if _, ok := nodeSet[nodeName]; !ok || disabledNodes[nodeName] {
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
			if desc := bridgeDescriptions[name]; desc == "" {
				bridgeDescriptions[name] = buildVMBRDescription(vmbr)
			}
		}
	}

	var bridgeDetails []map[string]string
	for _, bridgeIdentifier := range settings.VMBRs {
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

	log.Info().Int("snapshot_bridges", len(bridgeDetails)).Msg("Bridges served from snapshot cache")
	return bridgeDetails, nil
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
