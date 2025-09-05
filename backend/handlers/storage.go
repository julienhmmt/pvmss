package handlers

import (
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

// ToggleStorageHandler toggles a single storage enabled state (auto-save per click, no JS)
func (h *StorageHandler) ToggleStorageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("ToggleStorageHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	storageName := r.FormValue("storage")
	action := r.FormValue("action") // "enable" or "disable"
	if storageName == "" || (action != "enable" && action != "disable") {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	settings := h.stateManager.GetSettings()
	if settings.EnabledStorages == nil {
		settings.EnabledStorages = []string{}
	}

	enabledMap := make(map[string]bool, len(settings.EnabledStorages))
	for _, s := range settings.EnabledStorages {
		enabledMap[s] = true
	}

	changed := false
	if action == "enable" {
		if !enabledMap[storageName] {
			settings.EnabledStorages = append(settings.EnabledStorages, storageName)
			changed = true
		}
	} else { // disable
		if enabledMap[storageName] {
			// remove
			filtered := make([]string, 0, len(settings.EnabledStorages))
			for _, s := range settings.EnabledStorages {
				if s != storageName {
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
	log := logger.Get().With().
		Str("handler", "StorageHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Logger()

	// Get the Proxmox client
	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		// Offline-friendly: render page with empty storages and existing settings
		log.Warn().Msg("Proxmox client not available; rendering Storage page in offline/read-only mode")

		// Get settings
		settings := h.stateManager.GetSettings()
		if settings.EnabledStorages == nil {
			settings.EnabledStorages = []string{}
		}

		// Build enabled map from settings for toggles
		enabledMap := make(map[string]bool, len(settings.EnabledStorages))
		for _, s := range settings.EnabledStorages {
			enabledMap[s] = true
		}

		// Success banner via query params
		success := r.URL.Query().Get("success") != ""
		act := r.URL.Query().Get("action")
		stor := r.URL.Query().Get("storage")
		var successMsg string
		if success {
			switch act {
			case "enable":
				successMsg = "Storage '" + stor + "' enabled"
			case "disable":
				successMsg = "Storage '" + stor + "' disabled"
			default:
				successMsg = "Storage settings updated"
			}
		}

		// Prepare data for the template (empty Storages)
		data := AdminPageDataWithMessage("Storage Management", "storage", successMsg, "")
		data["Storages"] = []map[string]interface{}{}
		data["EnabledStorages"] = enabledMap

		// Add translations and render
		renderTemplateInternal(w, r, "admin_storage", data)
		return
	}

	// Get settings
	node := r.URL.Query().Get("node")
	refresh := r.URL.Query().Get("refresh") == "1"

	// Get settings
	settings := h.stateManager.GetSettings()
	if settings.EnabledStorages == nil {
		settings.EnabledStorages = []string{}
	}

	// Use the shared utility to retrieve renderable storages
	storages, enabledMap, chosenNode, err := FetchRenderableStorages(client, node, settings.EnabledStorages, refresh)
	if err != nil {
		log.Error().Err(err).Msg("Error retrieving storages")
		http.Error(w, "Error retrieving storages: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Success banner via query params
	success := r.URL.Query().Get("success") != ""
	act := r.URL.Query().Get("action")
	stor := r.URL.Query().Get("storage")
	var successMsg string
	if success {
		switch act {
		case "enable":
			successMsg = "Storage '" + stor + "' enabled"
		case "disable":
			successMsg = "Storage '" + stor + "' disabled"
		default:
			successMsg = "Storage settings updated"
		}
	}

	data := AdminPageDataWithMessage("Storage Management", "storage", successMsg, "")
	data["Node"] = chosenNode
	data["Storages"] = storages
	data["EnabledStorages"] = settings.EnabledStorages
	data["EnabledMap"] = enabledMap

	renderTemplateInternal(w, r, "admin_storage", data)
}

// UpdateStorageHandler handles updating enabled storages
func (h *StorageHandler) UpdateStorageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("UpdateStorageHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	// Get checked storages from the form
	enabledStoragesList := r.Form["enabled_storages"]

	// Update settings
	settings := h.stateManager.GetSettings()

	// Update the list of enabled storages
	settings.EnabledStorages = enabledStoragesList

	// Save settings
	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Error saving settings")
		http.Error(w, "Error saving settings", http.StatusInternalServerError)
		return
	}

	// Redirect to storage page with success banner
	http.Redirect(w, r, "/admin/storage?success=1", http.StatusSeeOther)
}

// RegisterRoutes registers storage-related routes
func (h *StorageHandler) RegisterRoutes(router *httprouter.Router) {
	routeHelpers := NewAdminPageRoutes()

	// Register admin storage routes using helper
	routeHelpers.RegisterCRUDRoutes(router, "/admin/storage", map[string]func(w http.ResponseWriter, r *http.Request, ps httprouter.Params){
		"page":   h.StoragePageHandler,
		"update": h.UpdateStorageHandler,
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
	"dir":       {},
	"lvm":       {},
	"lvmthin":   {},
	"zfs":       {},
	"rbd":       {},
	"ceph":      {},
	"cephfs":    {},
	"nfs":       {},
	"glusterfs": {},
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
// - If node is empty, the first available node is used.
// - If refresh is true, bypass the short-lived cache.
// Returns: storages (with Enabled already set from enabled list), enabledMap, chosenNode
func FetchRenderableStorages(client proxmox.ClientInterface, node string, enabled []string, refresh bool) ([]map[string]interface{}, map[string]bool, string, error) {
	log := logger.Get().With().Str("component", "storage_utils").Logger()

	// detect node if empty
	chosen := node
	if chosen == "" {
		if names, err := proxmox.GetNodeNames(client); err == nil && len(names) > 0 {
			chosen = names[0]
		}
	}

	if chosen == "" {
		return nil, map[string]bool{}, "", nil
	}

	// small cache (per node, without Enabled)
	storCacheMu.Lock()
	cached, ok := storCache[chosen]
	storCacheMu.Unlock()
	if ok && time.Now().Before(cached.expiresAt) && !refresh {
		log.Debug().Str("node", chosen).Time("expiresAt", cached.expiresAt).Msg("storage cache hit")
		// project enabled flags on top
		enabledMap := make(map[string]bool, len(enabled))
		for _, s := range enabled {
			enabledMap[s] = true
		}
		projected := make([]map[string]interface{}, 0, len(cached.items))
		for _, item := range cached.items {
			cpy := make(map[string]interface{}, len(item)+1)
			for k, v := range item {
				cpy[k] = v
			}
			name, _ := cpy["Storage"].(string)
			cpy["Enabled"] = len(enabled) == 0 || enabledMap[name]
			projected = append(projected, cpy)
		}
		// sort by Enabled desc then Storage asc
		sort.Slice(projected, func(i, j int) bool {
			if projected[i]["Enabled"].(bool) != projected[j]["Enabled"].(bool) {
				return projected[i]["Enabled"].(bool)
			}
			si := projected[i]["Storage"].(string)
			sj := projected[j]["Storage"].(string)
			return si < sj
		})
		return projected, enabledMap, chosen, nil
	}

	// fetch global config and node storages
	globalStorages, err := proxmox.GetStorages(client)
	if err != nil {
		return nil, nil, chosen, err
	}
	cfgByName := make(map[string]proxmox.Storage, len(globalStorages))
	for _, s := range globalStorages {
		cfgByName[s.Storage] = s
	}

	nodePathEscaped := url.PathEscape(chosen)
	_ = nodePathEscaped // not used directly, kept as doc to ensure safety if path built manually
	nodeStorages, err := proxmox.GetNodeStorages(client, chosen)
	if err != nil {
		return nil, nil, chosen, err
	}
	log.Debug().Str("node", chosen).Int("global_count", len(globalStorages)).Int("node_count", len(nodeStorages)).Msg("fetched storages from Proxmox")

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

		used, _ := st.Used.Int64()
		total, _ := st.Total.Int64()
		percent := 0
		if total > 0 {
			percent = int((used * 100) / total)
			if percent < 0 {
				percent = 0
			} else if percent > 100 {
				percent = 100
			}
		}

		item := map[string]interface{}{
			"Storage":     st.Storage,
			"Type":        st.Type,
			"Used":        used,
			"Total":       total,
			"Description": st.Description,
			"Content":     st.Content,
			"UsedPercent": percent,
		}
		if st.Avail.String() != "" {
			if avail, err := st.Avail.Int64(); err == nil {
				item["Available"] = avail
			}
		}
		base = append(base, item)
	}

	// update cache
	storCacheMu.Lock()
	storCache[chosen] = cachedStorages{items: base, expiresAt: time.Now().Add(cacheTTL)}
	storCacheMu.Unlock()
	log.Debug().Str("node", chosen).Int("items", len(base)).Dur("ttl", cacheTTL).Msg("storage cache updated")

	// project enabled flags and sort
	enabledMap := make(map[string]bool, len(enabled))
	for _, s := range enabled {
		enabledMap[s] = true
	}
	projected := make([]map[string]interface{}, 0, len(base))
	for _, item := range base {
		cpy := make(map[string]interface{}, len(item)+1)
		for k, v := range item {
			cpy[k] = v
		}
		name, _ := cpy["Storage"].(string)
		cpy["Enabled"] = len(enabled) == 0 || enabledMap[name]
		projected = append(projected, cpy)
	}

	sort.Slice(projected, func(i, j int) bool {
		if projected[i]["Enabled"].(bool) != projected[j]["Enabled"].(bool) {
			return projected[i]["Enabled"].(bool)
		}
		si := projected[i]["Storage"].(string)
		sj := projected[j]["Storage"].(string)
		return si < sj
	})

	return projected, enabledMap, chosen, nil
}
