package handlers

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// buildSuccessMessage creates success message from query parameters
func buildSuccessMessage(r *http.Request) string {
	if r.URL.Query().Get("success") == "" {
		return ""
	}

	action := r.URL.Query().Get("action")
	storage := r.URL.Query().Get("storage")

	switch action {
	case "enable":
		return "Storage '" + storage + "' enabled"
	case "disable":
		return "Storage '" + storage + "' disabled"
	case "update_disk_config":
		return "Disk configuration updated successfully"
	default:
		return "Storage settings updated"
	}
}

// buildEnabledMap creates a map from a list of enabled storage names
func buildEnabledMap(enabledList []string) map[string]bool {
	enabledMap := make(map[string]bool, len(enabledList))
	for _, s := range enabledList {
		enabledMap[s] = true
	}
	return enabledMap
}

// ensureStoragesInitialized ensures EnabledStorages slice is initialized
func ensureStoragesInitialized(settings *state.AppSettings) {
	if settings.EnabledStorages == nil {
		settings.EnabledStorages = []string{}
	}
}

// projectEnabledFlags adds Enabled field to storage items and sorts them
func projectEnabledFlags(base []map[string]interface{}, enabled []string) []map[string]interface{} {
	enabledMap := buildEnabledMap(enabled)
	projected := make([]map[string]interface{}, 0, len(base))

	for _, item := range base {
		cpy := make(map[string]interface{}, len(item)+1)
		for k, v := range item {
			cpy[k] = v
		}
		name, _ := cpy["Storage"].(string)
		node, _ := cpy["Node"].(string)

		// Check if node:storage is enabled
		uniqueID := node + ":" + name
		cpy["Enabled"] = len(enabled) == 0 || enabledMap[uniqueID]
		projected = append(projected, cpy)
	}

	// Sort by node (asc), then storage name (asc)
	sort.Slice(projected, func(i, j int) bool {
		nodeI, _ := projected[i]["Node"].(string)
		nodeJ, _ := projected[j]["Node"].(string)
		nodeI = strings.ToLower(nodeI)
		nodeJ = strings.ToLower(nodeJ)
		if nodeI != nodeJ {
			return nodeI < nodeJ
		}

		si := strings.ToLower(projected[i]["Storage"].(string))
		sj := strings.ToLower(projected[j]["Storage"].(string))
		return si < sj
	})

	return projected
}

// projectEnabledFlagsWithCache adds Enabled field and handles cache flag for storage items
func projectEnabledFlagsWithCache(base []map[string]interface{}, enabled []string, isFromCache bool) []map[string]interface{} {
	enabledMap := buildEnabledMap(enabled)
	projected := make([]map[string]interface{}, 0, len(base))

	for _, item := range base {
		cpy := make(map[string]interface{}, len(item)+1)
		for k, v := range item {
			cpy[k] = v
		}
		name, _ := cpy["Storage"].(string)
		node, _ := cpy["Node"].(string)

		// Check if node:storage is enabled
		uniqueID := node + ":" + name
		cpy["Enabled"] = len(enabled) == 0 || enabledMap[uniqueID]

		// Mark as from cache if needed
		if isFromCache {
			cpy["IsFromCache"] = true
		}

		projected = append(projected, cpy)
	}

	// Sort by Enabled desc, then Storage asc
	sort.Slice(projected, func(i, j int) bool {
		if projected[i]["Enabled"].(bool) != projected[j]["Enabled"].(bool) {
			return projected[i]["Enabled"].(bool)
		}
		si := projected[i]["Storage"].(string)
		sj := projected[j]["Storage"].(string)
		return si < sj
	})

	return projected
}

// ToggleStorageHandler toggles a single storage enabled state (auto-save per click, no JS)
func (h *StorageHandler) ToggleStorageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("ToggleStorageHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	storageName := r.FormValue("storage")
	node := r.FormValue("node")
	action := r.FormValue("action") // "enable" or "disable"
	if storageName == "" || node == "" || (action != "enable" && action != "disable") {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Create unique identifier combining node and storage name
	uniqueID := node + ":" + storageName

	settings := h.stateManager.GetSettings()
	if settings.EnabledStorages == nil {
		settings.EnabledStorages = []string{}
	}

	enabled := make(map[string]bool, len(settings.EnabledStorages))
	for _, s := range settings.EnabledStorages {
		enabled[s] = true
	}

	changed := false
	if action == "enable" {
		if !enabled[uniqueID] {
			settings.EnabledStorages = append(settings.EnabledStorages, uniqueID)
			changed = true
		}
	} else { // disable
		if enabled[uniqueID] {
			// remove
			filtered := make([]string, 0, len(settings.EnabledStorages))
			for _, s := range settings.EnabledStorages {
				if s != uniqueID {
					filtered = append(filtered, s)
				}
			}
			settings.EnabledStorages = filtered
			changed = true
		}
	}

	if changed {
		if err := h.stateManager.SetSettings(settings); err != nil {
			log.Error().Err(err).Msg("Error saving settings")
			http.Error(w, "Error saving settings", http.StatusInternalServerError)
			return
		}
	}

	// Redirect back to storage page with context for success banner
	redirectURL := "/admin/storage?success=1&action=" + action + "&storage=" + url.QueryEscape(storageName)
	if node != "" {
		redirectURL += "&node=" + url.QueryEscape(node)
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// StorageHandler handles storage-related operations.
type StorageHandler struct {
	stateManager state.StateManager
}

// NewStorageHandler creates a new instance of StorageHandler
func NewStorageHandler(stateManager state.StateManager) *StorageHandler {
	return &StorageHandler{
		stateManager: stateManager,
	}
}

// StoragePageHandler handles the storage management page
func (h *StorageHandler) StoragePageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("StorageHandler", r)

	client := h.stateManager.GetProxmoxClient()
	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()

	// Get node filter from query parameter (optional, for frontend filtering)
	node := r.URL.Query().Get("node")

	settings := h.stateManager.GetSettings()
	ensureStoragesInitialized(settings)

	allStorages := make([]map[string]interface{}, 0)
	allNodes := make([]string, 0)

	if client == nil || !proxmoxConnected {
		log.Warn().Bool("connected", proxmoxConnected).Msg("Proxmox not available; rendering page with empty storage list")
	} else {
		nodeNames, err := proxmox.GetNodeNames(client)
		if err != nil {
			log.Error().Err(err).Msg("Error getting node names")
		} else {
			sort.Strings(nodeNames)
			allNodes = nodeNames
			storagesByID := make(map[string]map[string]interface{}, len(nodeNames)*4)

			// Collect raw storages from all nodes for deduplication
			for _, nodeName := range nodeNames {
				rawStorages, err := fetchRawStoragesFromNode(nodeName)
				if err != nil {
					log.Warn().Err(err).Str("node", nodeName).Msg("Failed to get raw storages from node")
					continue
				}
				for _, storage := range rawStorages {
					storageNode, _ := storage["Node"].(string)
					storageName, _ := storage["Storage"].(string)
					if storageNode == "" || storageName == "" {
						continue
					}
					uniqueID := storageNode + ":" + storageName
					storagesByID[uniqueID] = storage
				}
			}

			// Convert map to slice and sort
			allStorages = make([]map[string]interface{}, 0, len(storagesByID))
			for _, storage := range storagesByID {
				allStorages = append(allStorages, storage)
			}
			sort.Slice(allStorages, func(i, j int) bool {
				nodeI, _ := allStorages[i]["Node"].(string)
				nodeJ, _ := allStorages[j]["Node"].(string)
				if nodeI == nodeJ {
					nameI, _ := allStorages[i]["Storage"].(string)
					nameJ, _ := allStorages[j]["Storage"].(string)
					return nameI < nameJ
				}
				return nodeI < nodeJ
			})

			// Apply enabled flags after deduplication
			allStorages = projectEnabledFlagsWithCache(allStorages, settings.EnabledStorages, false)
		}
		log.Info().Int("storage_total", len(allStorages)).Msg("Total storages prepared for template")
	}

	enabledMap := make(map[string]bool, len(settings.EnabledStorages))
	for _, storageID := range settings.EnabledStorages {
		enabledMap[storageID] = true
	}

	successMsg := buildSuccessMessage(r)

	builder := NewTemplateData("").
		SetAdminActive("storage").
		SetAuth(r).
		SetProxmoxStatus(h.stateManager).
		ParseMessages(r).
		AddData("TitleKey", "Admin.Storage.Title").
		AddData("Node", node).
		AddData("Nodes", allNodes).
		AddData("Storages", allStorages).
		AddData("EnabledMap", enabledMap).
		AddData("MaxDiskPerVM", settings.MaxDiskPerVM)

	if successMsg != "" {
		builder.SetSuccess(successMsg)
	}

	data := builder.Build().ToMap()
	renderTemplateInternal(w, r, "admin_storage", data)
}

// RegisterRoutes registers storage-related routes
func (h *StorageHandler) RegisterRoutes(router *httprouter.Router) {
	routeHelpers := NewAdminPageRoutes()

	// Register admin storage routes using helper
	routeHelpers.RegisterCRUDRoutes(router, "/admin/storage", map[string]func(w http.ResponseWriter, r *http.Request, ps httprouter.Params){
		"page":   h.StoragePageHandler,
		"toggle": h.ToggleStorageHandler,
	})
}

// Storage utility functions (moved from storage_utils.go)

// simple cache for filtered storages per node (without Enabled flag)
var (
	storCache   = make(map[string]cachedStorages)
	storCacheMu sync.Mutex
	cacheTTL    = 15 * time.Second
)

type cachedStorages struct {
	items     []map[string]interface{} // without Enabled
	expiresAt time.Time
}

var vmDiskTypes = map[string]struct{}{
	"ceph":    {},
	"cephfs":  {},
	"dir":     {},
	"lvm":     {},
	"lvmthin": {},
	"nfs":     {},
	"rbd":     {},
	"zfs":     {},
}

func canHoldVMDisks(s proxmox.Storage) bool {
	// Exclude PBS
	if strings.EqualFold(s.Type, "pbs") {
		return false
	}
	// Explicit content includes images
	if s.Content != "" && strings.Contains(s.Content, "images") {
		return true
	}
	// Empty content but known VM disk backends
	if s.Content == "" {
		if _, ok := vmDiskTypes[strings.ToLower(s.Type)]; ok {
			return true
		}
	}
	return false
}

// FetchRenderableStorages fetches, merges, filters and prepares storages for rendering.
// - If node is empty, tries all available nodes to find one that's online
// - If refresh is true, bypass the short-lived cache.
// - Falls back to cached data if all nodes are offline
// Returns: storages (with Enabled already set from enabled list), enabledMap, chosenNode
func FetchRenderableStorages(client proxmox.ClientInterface, node string, enabled []string, refresh bool) ([]map[string]interface{}, map[string]bool, string, error) {
	log := logger.Get().With().Str("component", "storage_utils").Logger()

	// Get all available nodes
	allNodes, err := proxmox.GetNodeNames(client)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get node names")
		return nil, map[string]bool{}, "", err
	}

	if len(allNodes) == 0 {
		return nil, map[string]bool{}, "", nil
	}

	// Determine which nodes to try
	nodesToTry := []string{}
	if node != "" {
		// Try the specified node first
		nodesToTry = append(nodesToTry, node)
		// Add other nodes as fallbacks
		for _, n := range allNodes {
			if n != node {
				nodesToTry = append(nodesToTry, n)
			}
		}
	} else {
		// Use all nodes in order
		nodesToTry = allNodes
	}

	log.Debug().Strs("available_nodes", allNodes).Str("requested_node", node).Msg("Available nodes for storage")

	var lastError error
	var chosenNode string

	// Try each node until we find one that works
	for _, tryNode := range nodesToTry {
		if tryNode == "" {
			continue
		}

		chosenNode = tryNode
		log.Debug().Str("trying_node", chosenNode).Msg("Attempting to fetch storages from node")

		// Check cache first (unless refresh is forced)
		if !refresh {
			storCacheMu.Lock()
			cached, ok := storCache[chosenNode]
			storCacheMu.Unlock()
			if ok && time.Now().Before(cached.expiresAt) {
				log.Debug().Str("node", chosenNode).Time("expiresAt", cached.expiresAt).Msg("storage cache hit")
				return projectEnabledFlagsWithCache(cached.items, enabled, true), buildEnabledMap(enabled), chosenNode, nil
			}
		}

		// Try to fetch fresh data from this node
		storages, err := fetchStoragesFromNode(chosenNode, enabled)
		if err != nil {
			log.Warn().Err(err).Str("node", chosenNode).Msg("Failed to fetch storages from node")
			lastError = err

			// Try to use cached data as fallback for this node
			storCacheMu.Lock()
			cached, ok := storCache[chosenNode]
			storCacheMu.Unlock()
			if ok {
				log.Info().Str("node", chosenNode).Msg("Using cached storages as fallback for offline node")
				return projectEnabledFlagsWithCache(cached.items, enabled, true), buildEnabledMap(enabled), chosenNode, nil
			}

			continue // Try next node
		}

		// Success! Return the storages from this node
		return storages, buildEnabledMap(enabled), chosenNode, nil
	}

	// All nodes failed, return the last error
	log.Error().Err(lastError).Msg("All nodes failed to provide storages")
	return nil, map[string]bool{}, chosenNode, lastError
}

// fetchStoragesFromNode fetches storages from a specific node and caches the result
func fetchStoragesFromNode(node string, enabled []string) ([]map[string]interface{}, error) {
	log := logger.Get().With().Str("component", "storage_utils").Logger()

	// Create resty client
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		return nil, err
	}

	// fetch global config and node storages using resty
	globalStorages, err := proxmox.GetStoragesResty(context.Background(), restyClient)
	if err != nil {
		return nil, err
	}

	cfgByName := make(map[string]proxmox.Storage)
	for _, s := range globalStorages {
		cfgByName[s.Storage] = s
		log.Debug().
			Str("storage", s.Storage).
			Str("type", s.Type).
			Str("used", s.Used.String()).
			Str("total", s.Total.String()).
			Str("content", s.Content).
			Msg("Global storage config")
	}

	nodeStorages, err := proxmox.GetNodeStoragesResty(context.Background(), restyClient, node)
	if err != nil {
		return nil, err
	}
	log.Debug().Str("node", node).Int("global_count", len(globalStorages)).Int("node_count", len(nodeStorages)).Msg("fetched storages from Proxmox")

	// build base items (without Enabled)
	base := make([]map[string]interface{}, 0, len(nodeStorages))
	for _, st := range nodeStorages {
		if cfg, ok := cfgByName[st.Storage]; ok {
			if st.Content == "" && cfg.Content != "" {
				st.Content = cfg.Content
			}
			if st.Type == "" && cfg.Type != "" {
				st.Type = cfg.Type
			}
			if st.Description == "" && cfg.Description != "" {
				st.Description = cfg.Description
			}
		}

		if !canHoldVMDisks(st) {
			continue
		}

		// Extract used/total values with detailed logging
		usedRaw := st.Used.String()
		totalRaw := st.Total.String()
		used, usedErr := st.Used.Int64()
		total, totalErr := st.Total.Int64()

		log.Debug().
			Str("storage", st.Storage).
			Str("type", st.Type).
			Str("used_raw", usedRaw).
			Str("total_raw", totalRaw).
			Int64("used", used).
			Int64("total", total).
			Bool("used_err", usedErr != nil).
			Bool("total_err", totalErr != nil).
			Msg("Storage usage data")

		percent := 0
		hasValidData := totalErr == nil && usedErr == nil && total >= 0

		if hasValidData {
			if total > 0 {
				percent = int((used * 100) / total)
				if percent < 0 {
					percent = 0
				} else if percent > 100 {
					percent = 100
				}
			} else {
				// Storage with total = 0 is valid but empty
				percent = 0
			}
		} else {
			// Log warning when storage data is invalid
			if totalRaw != "" || usedRaw != "" {
				log.Warn().
					Str("storage", st.Storage).
					Str("used_raw", usedRaw).
					Str("total_raw", totalRaw).
					Msg("Invalid or missing storage usage data")
			}
		}

		item := map[string]interface{}{
			"Storage":      st.Storage,
			"Type":         st.Type,
			"Used":         used,
			"Total":        total,
			"Description":  st.Description,
			"Content":      st.Content,
			"UsedPercent":  percent,
			"HasValidData": hasValidData,
			"IsFromCache":  false, // Fresh data from online node
			"Node":         node,
		}
		if st.Avail.String() != "" {
			if avail, err := st.Avail.Int64(); err == nil {
				item["Available"] = avail
			}
		}
		base = append(base, item)
	}

	// Update cache
	storCacheMu.Lock()
	storCache[node] = cachedStorages{items: base, expiresAt: time.Now().Add(cacheTTL)}
	storCacheMu.Unlock()
	log.Debug().Str("node", node).Int("items", len(base)).Dur("ttl", cacheTTL).Msg("storage cache updated")

	return projectEnabledFlagsWithCache(base, enabled, false), nil
}

// fetchRawStoragesFromNode fetches raw storage data without enabled flags (for deduplication)
func fetchRawStoragesFromNode(node string) ([]map[string]interface{}, error) {
	// Try cache first
	storCacheMu.Lock()
	if cached, found := storCache[node]; found && time.Now().Before(cached.expiresAt) {
		storCacheMu.Unlock()
		return cached.items, nil
	}
	storCacheMu.Unlock()

	// Fetch fresh data
	client, err := getDefaultRestyClient()
	if err != nil {
		return nil, err
	}

	storages, err := proxmox.GetNodeStoragesResty(context.Background(), client, node)
	if err != nil {
		return nil, err
	}

	base := make([]map[string]interface{}, 0, len(storages))
	for _, st := range storages {
		if st.Shared == 1 || st.Enabled == 0 {
			continue
		}

		used, usedErr := st.Used.Int64()
		total, totalErr := st.Total.Int64()

		percent := 0
		hasValidData := totalErr == nil && usedErr == nil && total >= 0

		if hasValidData {
			if total > 0 {
				percent = int((used * 100) / total)
				if percent < 0 {
					percent = 0
				} else if percent > 100 {
					percent = 100
				}
			} else {
				percent = 0
			}
		}

		item := map[string]interface{}{
			"Storage":      st.Storage,
			"Type":         st.Type,
			"Used":         used,
			"Total":        total,
			"Description":  st.Description,
			"Content":      st.Content,
			"UsedPercent":  percent,
			"HasValidData": hasValidData,
			"IsFromCache":  false, // Fresh data
			"Node":         node,
		}
		if st.Avail.String() != "" {
			if avail, err := st.Avail.Int64(); err == nil {
				item["Available"] = avail
			}
		}
		base = append(base, item)
	}

	// Update cache
	storCacheMu.Lock()
	storCache[node] = cachedStorages{items: base, expiresAt: time.Now().Add(cacheTTL)}
	storCacheMu.Unlock()

	return base, nil
}
