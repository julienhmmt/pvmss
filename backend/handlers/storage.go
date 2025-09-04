package handlers

import (
	"net/http"
	"net/url"

	"github.com/julienschmidt/httprouter"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// StateManager is an interface that both real and mock state managers implement
type StateManager interface {
	GetProxmoxClient() proxmox.ClientInterface
	GetSettings() *state.AppSettings
	SetSettings(settings *state.AppSettings) error
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

// StorageHandler handles storage-related routes
type StorageHandler struct {
	stateManager StateManager
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
		data := map[string]interface{}{
			"Title":           "Storage Management",
			"Node":            "",
			"Storages":        []map[string]interface{}{},
			"EnabledStorages": settings.EnabledStorages,
			"EnabledMap":      enabledMap,
			"Success":         success,
			"SuccessMessage":  successMsg,
			"AdminActive":     "storage",
		}

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

	// Prepare data for the template
	data := map[string]interface{}{
		"Title":           "Storage Management",
		"Node":            chosenNode,
		"Storages":        storages,
		"EnabledStorages": settings.EnabledStorages,
		"EnabledMap":      enabledMap,
		"Success":         success,
		"SuccessMessage":  successMsg,
		"AdminActive":     "storage",
	}

	// Add translations and render
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
	router.GET("/admin/storage", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.StoragePageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
	router.GET("/admin/storage/", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		logger.Get().Debug().Str("path", r.URL.Path).Msg("Redirecting /admin/storage/ to /admin/storage")
		http.Redirect(w, r, "/admin/storage", http.StatusSeeOther)
	})))
	router.POST("/admin/storage/update", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.UpdateStorageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
	router.POST("/admin/storage/toggle", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.ToggleStorageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
}
