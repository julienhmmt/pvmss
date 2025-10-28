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
	"pvmss/i18n"
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
		cpy["Enabled"] = len(enabled) == 0 || enabledMap[name]
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
	log := CreateHandlerLogger("StorageHandler", r)

	// Get the Proxmox client
	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		// Offline-friendly: render page with empty storages and existing settings
		log.Warn().Msg("Proxmox client not available; rendering Storage page in offline/read-only mode")

		// Get settings
		settings := h.stateManager.GetSettings()
		ensureStoragesInitialized(settings)
		enabledMap := buildEnabledMap(settings.EnabledStorages)
		successMsg := buildSuccessMessage(r)

		// Prepare data for the template (empty Storages)
		data := AdminPageDataWithMessage("", "storage", successMsg, "")
		data["TitleKey"] = "Admin.Storage.Title"
		data["Storages"] = []map[string]interface{}{}
		data["EnabledStorages"] = enabledMap
		data["MaxDiskPerVM"] = settings.MaxDiskPerVM

		// Add translations and render
		renderTemplateInternal(w, r, "admin_storage", data)
		return
	}

	node := r.URL.Query().Get("node")
	refresh := r.URL.Query().Get("refresh") == "1"

	// Get settings
	settings := h.stateManager.GetSettings()
	ensureStoragesInitialized(settings)

	// Use the shared utility to retrieve renderable storages
	storages, enabledMap, chosenNode, err := FetchRenderableStorages(client, node, settings.EnabledStorages, refresh)
	if err != nil {
		log.Error().Err(err).Msg("Error retrieving storages")
		localizer := i18n.GetLocalizerFromRequest(r)
		http.Error(w, i18n.Localize(localizer, "Error.InternalServer"), http.StatusInternalServerError)
		return
	}

	successMsg := buildSuccessMessage(r)

	data := AdminPageDataWithMessage("", "storage", successMsg, "")
	data["TitleKey"] = "Admin.Storage.Title"
	data["Node"] = chosenNode
	data["Storages"] = storages
	data["EnabledMap"] = enabledMap
	data["MaxDiskPerVM"] = settings.MaxDiskPerVM

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

	// Check cache
	storCacheMu.Lock()
	cached, ok := storCache[chosen]
	storCacheMu.Unlock()
	if ok && time.Now().Before(cached.expiresAt) && !refresh {
		log.Debug().Str("node", chosen).Time("expiresAt", cached.expiresAt).Msg("storage cache hit")
		return projectEnabledFlags(cached.items, enabled), buildEnabledMap(enabled), chosen, nil
	}

	// Create resty client
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		return nil, nil, chosen, err
	}

	// fetch global config and node storages using resty
	globalStorages, err := proxmox.GetStoragesResty(context.Background(), restyClient)
	if err != nil {
		return nil, nil, chosen, err
	}

	cfgByName := make(map[string]proxmox.Storage)
	for _, s := range globalStorages {
		cfgByName[s.Storage] = s
	}

	nodeStorages, err := proxmox.GetNodeStoragesResty(context.Background(), restyClient, chosen)
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

	// Update cache
	storCacheMu.Lock()
	storCache[chosen] = cachedStorages{items: base, expiresAt: time.Now().Add(cacheTTL)}
	storCacheMu.Unlock()
	log.Debug().Str("node", chosen).Int("items", len(base)).Dur("ttl", cacheTTL).Msg("storage cache updated")

	return projectEnabledFlags(base, enabled), buildEnabledMap(enabled), chosen, nil
}
