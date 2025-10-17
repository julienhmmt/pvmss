package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/logger"
	"pvmss/proxmox"
)

// ISOEntry represents an ISO file entry
type ISOEntry struct {
	Node    string      `json:"node"`
	Storage string      `json:"storage"`
	Volid   string      `json:"volid"`
	Size    interface{} `json:"size"`
	Format  string      `json:"format"`
	Enabled bool        `json:"enabled,omitempty"`
}

// fetchAllISOs retrieves all ISOs from all nodes and storages
func (h *SettingsHandler) fetchAllISOs(ctx context.Context, client proxmox.ClientInterface, checkEnabled bool) ([]ISOEntry, error) {
	nodes, err := proxmox.GetNodeNamesWithContext(ctx, client)
	if err != nil {
		return nil, err
	}

	storages, err := proxmox.GetStoragesWithContext(ctx, client)
	if err != nil {
		return nil, err
	}

	// Use a map to deduplicate ISOs by Volid (same storage = same ISOs across nodes)
	isoMap := make(map[string]ISOEntry)
	var settings = h.stateManager.GetSettings()
	if !checkEnabled {
		settings = nil // Don't check enabled status if not requested
	}

	// For each node, get ISOs from each compatible storage
	for _, nodeName := range nodes {
		for _, storage := range storages {
			// Check if storage is available on this node and supports ISO
			isNodeInStorage := storage.Nodes == "" || strings.Contains(storage.Nodes, nodeName)
			if !isNodeInStorage || !containsISO(storage.Content) {
				continue
			}

			isoList, err := proxmox.GetISOListWithContext(ctx, client, nodeName, storage.Storage)
			if err != nil {
				logger.Get().Debug().Err(err).
					Str("node", nodeName).
					Str("storage", storage.Storage).
					Msg("Failed to get ISO list for storage")
				continue
			}

			for _, iso := range isoList {
				// Skip if we already have this ISO (same Volid = same storage/file)
				if _, exists := isoMap[iso.VolID]; exists {
					continue
				}

				entry := ISOEntry{
					Node:    nodeName,
					Storage: storage.Storage,
					Volid:   iso.VolID,
					Size:    iso.Size,
					Format:  iso.Format,
				}

				// Check if enabled (if requested)
				if checkEnabled && settings != nil {
					for _, enabledISO := range settings.ISOs {
						if enabledISO == iso.VolID {
							entry.Enabled = true
							break
						}
					}
				}

				isoMap[iso.VolID] = entry
			}
		}
	}

	// Convert map to slice
	allISOs := make([]ISOEntry, 0, len(isoMap))
	for _, entry := range isoMap {
		allISOs = append(allISOs, entry)
	}

	return allISOs, nil
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
	data["AllISOs"] = []interface{}{}

	// Return early if Proxmox not connected
	if !proxmoxConnected {
		data["Warning"] = "Proxmox connection unavailable. Displaying cached ISO data."
		renderTemplateInternal(w, r, "admin_iso", data)
		return
	}

	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client is nil despite connection status being true")
		data["Warning"] = "Proxmox client unavailable."
		renderTemplateInternal(w, r, "admin_iso", data)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Fetch all ISOs with enabled check
	isos, err := h.fetchAllISOs(ctx, client, true)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch ISOs for page")
		data["Warning"] = "Failed to fetch ISOs from Proxmox."
		renderTemplateInternal(w, r, "admin_iso", data)
		return
	}

	data["AllISOs"] = isos

	log.Debug().Int("iso_count", len(isos)).Msg("ISO page rendered")
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

// RegisterISORoutes registers ISO-related routes
func (h *SettingsHandler) RegisterISORoutes(router *httprouter.Router) {
	routeHelpers := NewAdminPageRoutes()

	// Register admin ISO routes using helper
	routeHelpers.RegisterCRUDRoutes(router, "/admin/iso", map[string]func(w http.ResponseWriter, r *http.Request, ps httprouter.Params){
		"page":   h.ISOPageHandler,
		"toggle": h.ToggleISOHandler,
	})
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
