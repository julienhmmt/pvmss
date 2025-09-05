package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/logger"
	"pvmss/proxmox"
)

// GetAllISOsHandler retrieves all available ISO images
func (h *SettingsHandler) GetAllISOsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Short-circuit when offline to keep UI responsive
	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()
	client := h.stateManager.GetProxmoxClient()
	if !proxmoxConnected || client == nil {
		logger.Get().Info().Msg("GetAllISOsHandler: Proxmox not connected, returning empty ISO list")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "offline",
			"isos":   []interface{}{},
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Collect all ISOs (reuse logic from GetAllISOsHandler)
	nodes, err := proxmox.GetNodeNamesWithContext(ctx, client)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get nodes")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Failed to get nodes",
		})
		return
	}

	// Get all storages that can contain ISOs
	storages, err := proxmox.GetStoragesWithContext(ctx, client)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get storages")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Failed to get storages",
		})
		return
	}

	var allISOs []map[string]interface{}

	// For each node, get ISOs from each compatible storage
	for _, nodeName := range nodes {
		for _, storage := range storages {
			isNodeInStorage := storage.Nodes == "" || strings.Contains(storage.Nodes, nodeName)
			if !isNodeInStorage || !containsISO(storage.Content) {
				continue
			}

			isoList, err := proxmox.GetISOListWithContext(ctx, client, nodeName, storage.Storage)
			if err != nil {
				logger.Get().Debug().Err(err).Str("node", nodeName).Str("storage", storage.Storage).Msg("Failed to get ISO list for storage")
				continue
			}

			// Convert ISOs to response format
			for _, iso := range isoList {
				isoEntry := map[string]interface{}{
					"node":    nodeName,
					"storage": storage.Storage,
					"volid":   iso.VolID,
					"size":    iso.Size,
					"format":  iso.Format,
				}
				allISOs = append(allISOs, isoEntry)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"isos":   allISOs,
	})
}

// ISOPageHandler renders the ISO management page (server-rendered, no JS required)
func (h *SettingsHandler) ISOPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("ISOPageHandler", r)

	settings := h.stateManager.GetSettings()
	enabledMap := make(map[string]bool)
	if settings != nil {
		for _, v := range settings.ISOs {
			enabledMap[v] = true
		}
	}

	// Success banner via query params
	success := r.URL.Query().Get("success") != ""
	act := r.URL.Query().Get("action")
	isoName := r.URL.Query().Get("iso")
	var successMsg string
	if success {
		switch act {
		case "enable":
			successMsg = "ISO '" + isoName + "' enabled"
		case "disable":
			successMsg = "ISO '" + isoName + "' disabled"
		default:
			successMsg = "ISO settings updated"
		}
	}

	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()

	data := AdminPageDataWithMessage("ISO Management", "iso", successMsg, "")
	data["ISOsList"] = []ISOInfo{}
	data["EnabledISOs"] = enabledMap
	data["ProxmoxConnected"] = proxmoxConnected

	if !proxmoxConnected {
		data["Warning"] = "Proxmox connection unavailable. Displaying cached ISO data."
		data["AllISOs"] = []interface{}{}
		renderTemplateInternal(w, r, "admin_iso", data)
		return
	}

	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client is nil despite connection status being true")
		data["Warning"] = "Proxmox client unavailable."
		data["AllISOs"] = []interface{}{}
		renderTemplateInternal(w, r, "admin_iso", data)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get all nodes
	nodes, err := proxmox.GetNodeNamesWithContext(ctx, client)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get nodes for ISO page")
		data["Warning"] = "Failed to fetch nodes from Proxmox."
		data["AllISOs"] = []interface{}{}
		renderTemplateInternal(w, r, "admin_iso", data)
		return
	}

	// Get all storages that can contain ISOs
	storages, err := proxmox.GetStoragesWithContext(ctx, client)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get storages for ISO page")
		data["Warning"] = "Failed to fetch storages from Proxmox."
		data["AllISOs"] = []interface{}{}
		renderTemplateInternal(w, r, "admin_iso", data)
		return
	}

	var allISOs []map[string]interface{}

	// For each node, get ISOs from each compatible storage
	for _, nodeName := range nodes {
		for _, storage := range storages {
			isNodeInStorage := storage.Nodes == "" || strings.Contains(storage.Nodes, nodeName)
			if !isNodeInStorage || !containsISO(storage.Content) {
				continue
			}
			isoList, err := proxmox.GetISOListWithContext(ctx, client, nodeName, storage.Storage)
			if err != nil {
				log.Debug().Err(err).Str("node", nodeName).Str("storage", storage.Storage).Msg("Failed to get ISO list for storage")
				continue
			}

			// Convert ISOs to response format, check against current settings
			settings := h.stateManager.GetSettings()
			for _, iso := range isoList {
				enabled := false
				if settings != nil {
					for _, enabledISO := range settings.ISOs {
						if enabledISO == iso.VolID {
							enabled = true
							break
						}
					}
				}

				isoEntry := map[string]interface{}{
					"node":    nodeName,
					"storage": storage.Storage,
					"volid":   iso.VolID,
					"size":    iso.Size,
					"format":  iso.Format,
					"enabled": enabled,
				}
				allISOs = append(allISOs, isoEntry)
			}
		}
	}

	data["AllISOs"] = allISOs

	log.Debug().Int("iso_count", len(allISOs)).Msg("ISO page rendered")
	renderTemplateInternal(w, r, "admin_iso", data)
}

// ToggleISOHandler toggles a single ISO enabled state (auto-save per click, no JS)
func (h *SettingsHandler) ToggleISOHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("ToggleISOHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	volid := strings.TrimSpace(r.FormValue("volid"))
	action := strings.TrimSpace(r.FormValue("action"))

	if volid == "" {
		log.Error().Msg("Missing volid parameter")
		http.Error(w, "Missing volid parameter", http.StatusBadRequest)
		return
	}

	if action == "" {
		log.Error().Msg("Missing action parameter")
		http.Error(w, "Missing action parameter", http.StatusBadRequest)
		return
	}

	// Convert action to enabled boolean
	var enabled bool
	switch action {
	case "enable":
		enabled = true
	case "disable":
		enabled = false
	default:
		log.Error().Str("action", action).Msg("Invalid action parameter")
		http.Error(w, "Invalid action parameter", http.StatusBadRequest)
		return
	}

	log.Debug().Str("volid", volid).Bool("enabled", enabled).Msg("Toggling ISO")

	// Update settings
	settings := h.stateManager.GetSettings()
	if settings == nil {
		log.Error().Msg("Settings not available")
		http.Error(w, "Settings not available", http.StatusInternalServerError)
		return
	}

	// Create a new slice for ISOs
	var newISOs []string
	found := false
	for _, iso := range settings.ISOs {
		if iso == volid {
			found = true
			if enabled {
				newISOs = append(newISOs, iso) // Keep it
			}
			// If not enabled, we skip adding it (remove it)
		} else {
			newISOs = append(newISOs, iso) // Keep other ISOs
		}
	}

	// If we want to enable it and it wasn't found, add it
	if enabled && !found {
		newISOs = append(newISOs, volid)
	}

	// Update settings
	settings.ISOs = newISOs
	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save settings")
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	log.Info().Str("volid", volid).Bool("enabled", enabled).Msg("ISO toggle completed")

	// Redirect back to ISOs page (route base is /admin/iso)
	http.Redirect(w, r, "/admin/iso", http.StatusSeeOther)
}

// RegisterRoutes registers ISO-related routes
func (h *SettingsHandler) RegisterISORoutes(router *httprouter.Router) {
	routeHelpers := NewAdminPageRoutes()

	// Register admin ISO routes using helper
	routeHelpers.RegisterCRUDRoutes(router, "/admin/iso", map[string]func(w http.ResponseWriter, r *http.Request, ps httprouter.Params){
		"page":   h.ISOPageHandler,
		"toggle": h.ToggleISOHandler,
	})

	// API endpoint for fetching ISOs
	helpers := NewRouteHelpers()
	helpers.RegisterAuthRoute(router, "GET", "/api/settings/iso", h.GetAllISOsHandler)
}

// containsISO checks if a storage content type can contain ISOs
func containsISO(content string) bool {
	// Content types are separated by commas
	for _, part := range strings.Split(content, ",") {
		if strings.TrimSpace(part) == "iso" {
			return true
		}
	}
	return false
}
