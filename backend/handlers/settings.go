package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// SettingsHandler handles settings-related routes
type SettingsHandler struct {
	stateManager state.StateManager
}

// NewSettingsHandler creates a new instance of SettingsHandler
func NewSettingsHandler(sm state.StateManager) *SettingsHandler {
	return &SettingsHandler{stateManager: sm}
}

// GetSettingsHandler returns the current application settings
func (h *SettingsHandler) GetSettingsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	settings := h.stateManager.GetSettings()
	if settings == nil {
		logger.Get().Error().Msg("Settings not available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Settings not available",
		})
		return
	}

	// Do not return the admin password
	settingsResponse := map[string]interface{}{
		"tags":   settings.Tags,
		"isos":   settings.ISOs,
		"vmbrs":  settings.VMBRs,
		"limits": settings.Limits,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settingsResponse)
}

// GetAllISOsHandler retrieves all available ISO images
func (h *SettingsHandler) GetAllISOsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Short-circuit when offline to keep UI responsive
	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()
	client := h.stateManager.GetProxmoxClient()
	if client == nil || !proxmoxConnected {
		logger.Get().Warn().Msg("Proxmox offline or client unavailable; returning empty ISO list")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string][]ISOInfo{"isos": {}})
		return
	}

	// Use a short timeout for all Proxmox calls
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	appSettings := h.stateManager.GetSettings()
	enabledISOsMap := make(map[string]bool)
	for _, enabledISO := range appSettings.ISOs { // Correction: iterate over ISOs, not EnabledISOs
		enabledISOsMap[enabledISO] = true
	}

	// Get all nodes
	nodes, err := proxmox.GetNodeNamesWithContext(ctx, client)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get nodes from Proxmox")
		http.Error(w, "Failed to get nodes", http.StatusInternalServerError)
		return
	}

	// Get all storages
	storages, err := proxmox.GetStoragesWithContext(ctx, client)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get storages from Proxmox")
		http.Error(w, "Failed to get storages", http.StatusInternalServerError)
		return
	}

	var allISOs []ISOInfo
	logger.Get().Debug().Int("storage_count", len(storages)).Msg("Fetching ISOs from storages")

	for _, nodeName := range nodes {
		for _, storage := range storages {
			isNodeInStorage := storage.Nodes == "" || strings.Contains(storage.Nodes, nodeName)
			if !isNodeInStorage || !containsISO(storage.Content) {
				continue
			}

			logger.Get().Debug().Str("node", nodeName).Str("storage", storage.Storage).Msg("Fetching ISO list for storage")
			// Get ISO list for this storage
			isoList, err := proxmox.GetISOListWithContext(ctx, client, nodeName, storage.Storage)
			if err != nil {
				logger.Get().Warn().Err(err).Str("node", nodeName).Str("storage", storage.Storage).Msg("Could not get ISO list for storage, skipping")
				continue
			}

			for _, iso := range isoList {
				// We only process .iso files, ignoring other formats like .img
				if !strings.HasSuffix(iso.VolID, ".iso") {
					logger.Get().Debug().Str("volid", iso.VolID).Msg("Skipping non-ISO file")
					continue
				}

				_, isEnabled := enabledISOsMap[iso.VolID]
				isoInfo := ISOInfo{
					VolID:   iso.VolID,
					Format:  "iso", // We force the format to "iso" because we have already filtered
					Size:    iso.Size,
					Node:    nodeName,
					Storage: storage.Storage,
					Enabled: isEnabled,
				}
				allISOs = append(allISOs, isoInfo)
				logger.Get().Debug().Str("volid", iso.VolID).Bool("enabled", isEnabled).Msg("Found ISO")
			}
		}
	}

	logger.Get().Info().Int("total_isos_found", len(allISOs)).Msg("Finished fetching all ISOs")
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string][]ISOInfo{"isos": allISOs}); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to encode ISOs to JSON")
		http.Error(w, "Failed to encode ISOs", http.StatusInternalServerError)
	}
}

// GetAllVMBRsHandler retrieves all available network bridges
func (h *SettingsHandler) GetAllVMBRsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Use shared helper to collect VMBRs
	vmbrs, err := collectAllVMBRs(h.stateManager)
	if err != nil {
		logger.Get().Warn().Err(err).Msg("collectAllVMBRs returned an error")
	}

	// Format for API response
	formatted := make([]map[string]interface{}, 0, len(vmbrs))
	for _, v := range vmbrs {
		formatted = append(formatted, map[string]interface{}{
			"name":        v["iface"],
			"description": v["description"],
			"node":        v["node"],
			"type":        v["type"],
			"method":      v["method"],
			"address":     v["address"],
			"netmask":     v["netmask"],
			"gateway":     v["gateway"],
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"vmbrs":  formatted,
	}); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to encode JSON response")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// ISOPageHandler renders the ISO management page (server-rendered, no JS required)
func (h *SettingsHandler) ISOPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "ISOPageHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Logger()

	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()
	client := h.stateManager.GetProxmoxClient()
	if client == nil || !proxmoxConnected {
		// Offline-friendly: render page with empty ISO list and enabled settings map
		log.Warn().Msg("Proxmox offline or client unavailable; rendering ISO page in offline/read-only mode")

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

		data := map[string]interface{}{
			"Title":          "ISO Management",
			"ISOsList":       []ISOInfo{},
			"EnabledISOs":    enabledMap,
			"Success":        success,
			"SuccessMessage": successMsg,
			"AdminActive":    "iso",
		}
		renderTemplateInternal(w, r, "admin_iso", data)
		return
	}

	// Use a short timeout for all Proxmox calls
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	settings := h.stateManager.GetSettings()
	enabledMap := make(map[string]bool)
	for _, v := range settings.ISOs {
		enabledMap[v] = true
	}

	// Collect all ISOs (reuse logic from GetAllISOsHandler)
	nodes, err := proxmox.GetNodeNamesWithContext(ctx, client)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get nodes from Proxmox; continuing with empty node list")
		nodes = []string{}
	}

	storages, err := proxmox.GetStoragesWithContext(ctx, client)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get storages from Proxmox; continuing with empty storage list")
		storages = []proxmox.Storage{}
	}

	var allISOs []ISOInfo
	for _, nodeName := range nodes {
		for _, storage := range storages {
			isNodeInStorage := storage.Nodes == "" || strings.Contains(storage.Nodes, nodeName)
			if !isNodeInStorage || !containsISO(storage.Content) {
				continue
			}
			isoList, err := proxmox.GetISOListWithContext(ctx, client, nodeName, storage.Storage)
			if err != nil {
				log.Warn().Err(err).Str("node", nodeName).Str("storage", storage.Storage).Msg("Could not get ISO list for storage, skipping")
				continue
			}
			for _, iso := range isoList {
				if !strings.HasSuffix(iso.VolID, ".iso") {
					continue
				}
				_, isEnabled := enabledMap[iso.VolID]
				allISOs = append(allISOs, ISOInfo{
					VolID:   iso.VolID,
					Format:  "iso",
					Size:    iso.Size,
					Node:    nodeName,
					Storage: storage.Storage,
					Enabled: isEnabled,
				})
			}
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

	// Build data and render
	data := map[string]interface{}{
		"Title":          "ISO Management",
		"ISOsList":       allISOs,
		"EnabledISOs":    enabledMap,
		"Success":        success,
		"SuccessMessage": successMsg,
		"AdminActive":    "iso",
	}
	renderTemplateInternal(w, r, "admin_iso", data)
}

// LimitsPageHandler renders the Resource Limits page (server-rendered)
func (h *SettingsHandler) LimitsPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "LimitsPageHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Logger()

	settings := h.stateManager.GetSettings()
	if settings == nil {
		http.Error(w, "Settings not available", http.StatusInternalServerError)
		return
	}

	// Fetch node names if Proxmox client is available
	var nodeNames []string
	selectedNode := ""
	client := h.stateManager.GetProxmoxClient()
	if client != nil {
		if pc, ok := client.(*proxmox.Client); ok {
			n, err := proxmox.GetNodeNames(pc)
			if err != nil {
				log.Warn().Err(err).Msg("Unable to retrieve Proxmox nodes; continuing with empty node list")
			} else {
				nodeNames = n
			}
		}
	}

	// Selected node from query param if present and exists
	if qn := r.URL.Query().Get("node"); qn != "" {
		for _, n := range nodeNames {
			if n == qn {
				selectedNode = qn
				break
			}
		}
	}
	if selectedNode == "" && len(nodeNames) > 0 {
		selectedNode = nodeNames[0]
	}

	// Success banner via query params
	success := r.URL.Query().Get("success") != ""
	entity := r.URL.Query().Get("entity")
	nodeParam := r.URL.Query().Get("node")
	var successMsg string
	if success {
		switch entity {
		case "vm":
			successMsg = "VM limits updated"
		case "nodes":
			if nodeParam != "" {
				successMsg = "Limits updated for node '" + nodeParam + "'"
			} else {
				successMsg = "Node limits updated"
			}
		default:
			successMsg = "Limits updated"
		}
	}

	data := map[string]interface{}{
		"Title":          "Resource Limits",
		"Limits":         settings.Limits,
		"NodeNames":      nodeNames,
		"Node":           selectedNode,
		"Success":        success,
		"SuccessMessage": successMsg,
		"AdminActive":    "limits",
	}
	renderTemplateInternal(w, r, "admin_limits", data)
}

// ToggleISOHandler toggles a single ISO enabled state (auto-save per click, no JS)
func (h *SettingsHandler) ToggleISOHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "ToggleISOHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Logger()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	volid := r.FormValue("volid")
	action := r.FormValue("action") // enable|disable
	if volid == "" || (action != "enable" && action != "disable") {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	settings := h.stateManager.GetSettings()
	if settings.ISOs == nil {
		settings.ISOs = []string{}
	}

	enabled := make(map[string]bool, len(settings.ISOs))
	for _, v := range settings.ISOs {
		enabled[v] = true
	}

	changed := false
	if action == "enable" {
		if !enabled[volid] {
			settings.ISOs = append(settings.ISOs, volid)
			changed = true
		}
	} else { // disable
		if enabled[volid] {
			filtered := make([]string, 0, len(settings.ISOs))
			for _, v := range settings.ISOs {
				if v != volid {
					filtered = append(filtered, v)
				}
			}
			settings.ISOs = filtered
			changed = true
		}
	}

	if changed {
		if err := h.stateManager.SetSettings(settings); err != nil {
			log.Error().Err(err).Msg("Failed to save settings")
			http.Error(w, "Failed to save settings", http.StatusInternalServerError)
			return
		}
	}

	redirectURL := "/admin/iso?success=1&action=" + action + "&iso=" + url.QueryEscape(volid)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// UpdateLimitsFormHandler handles POST from admin_limits.html to update VM/Node limits
func (h *SettingsHandler) UpdateLimitsFormHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "UpdateLimitsFormHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Logger()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	entity := r.FormValue("entityId") // "vm" or "node"
	if entity == "" {
		http.Error(w, "Missing entityId", http.StatusBadRequest)
		return
	}

	// Helper to parse an int field safely
	parseInt := func(name string, fallback int) int {
		v := r.FormValue(name)
		if v == "" {
			return fallback
		}
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
		return fallback
	}

	// Extract values
	socketsMin := parseInt("sockets-min", 1)
	socketsMax := parseInt("sockets-max", socketsMin)
	coresMin := parseInt("cores-min", 1)
	coresMax := parseInt("cores-max", coresMin)
	ramMin := parseInt("ram-min", 1)
	ramMax := parseInt("ram-max", ramMin)
	diskMin := parseInt("disk-min", 1)
	diskMax := parseInt("disk-max", diskMin)

	// Clamp minimums to at least 1 (no zeros/negatives/letters already handled)
	if socketsMin < 1 {
		socketsMin = 1
	}
	if coresMin < 1 {
		coresMin = 1
	}
	if ramMin < 1 {
		ramMin = 1
	}
	if diskMin < 1 {
		diskMin = 1
	}

	// Normalize to ensure min <= max
	if socketsMin > socketsMax {
		socketsMin, socketsMax = socketsMax, socketsMin
	}
	if coresMin > coresMax {
		coresMin, coresMax = coresMax, coresMin
	}
	if ramMin > ramMax {
		ramMin, ramMax = ramMax, ramMin
	}
	if diskMin > diskMax {
		diskMin, diskMax = diskMax, diskMin
	}

	// Ensure max are at least 1 as well
	if socketsMax < 1 {
		socketsMax = 1
	}
	if coresMax < 1 {
		coresMax = 1
	}
	if ramMax < 1 {
		ramMax = 1
	}
	if diskMax < 1 {
		diskMax = 1
	}

	// Load settings
	settings := h.stateManager.GetSettings()
	if settings == nil {
		http.Error(w, "Settings not available", http.StatusInternalServerError)
		return
	}
	if settings.Limits == nil {
		settings.Limits = make(map[string]interface{})
	}

	// Persist limits
	switch entity {
	case "vm":
		// Flat VM limits
		entityMap, _ := settings.Limits["vm"].(map[string]interface{})
		if entityMap == nil {
			entityMap = make(map[string]interface{})
		}
		entityMap["sockets"] = map[string]int{"min": socketsMin, "max": socketsMax}
		entityMap["cores"] = map[string]int{"min": coresMin, "max": coresMax}
		entityMap["ram"] = map[string]int{"min": ramMin, "max": ramMax}
		entityMap["disk"] = map[string]int{"min": diskMin, "max": diskMax}
		settings.Limits["vm"] = entityMap

	case "node", "nodes":
		// Per-node limits under limits.nodes[<nodeName>]
		nodeName := strings.TrimSpace(r.FormValue("nodeName"))
		if nodeName == "" {
			http.Error(w, "Missing nodeName for node limits", http.StatusBadRequest)
			return
		}
		nodesMap, _ := settings.Limits["nodes"].(map[string]interface{})
		if nodesMap == nil {
			nodesMap = make(map[string]interface{})
		}
		nodeEntry, _ := nodesMap[nodeName].(map[string]interface{})
		if nodeEntry == nil {
			nodeEntry = make(map[string]interface{})
		}
		nodeEntry["sockets"] = map[string]int{"min": socketsMin, "max": socketsMax}
		nodeEntry["cores"] = map[string]int{"min": coresMin, "max": coresMax}
		nodeEntry["ram"] = map[string]int{"min": ramMin, "max": ramMax}
		nodesMap[nodeName] = nodeEntry
		settings.Limits["nodes"] = nodesMap
		entity = "nodes" // normalize for redirect

	default:
		http.Error(w, "Unsupported entity", http.StatusBadRequest)
		return
	}

	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save limits settings")
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	// Redirect back to limits page with success banner and context
	redirect := "/admin/limits?success=1&entity=" + url.QueryEscape(entity)
	if entity == "nodes" {
		redirect += "&node=" + url.QueryEscape(strings.TrimSpace(r.FormValue("nodeName")))
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

// RegisterRoutes enregistre les routes liées aux paramètres
func (h *SettingsHandler) RegisterRoutes(router *httprouter.Router) {
	// Admin ISO page and toggle (protected)
	router.GET("/admin/iso", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.ISOPageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
	// Trailing-slash variant: redirect to canonical path
	router.GET("/admin/iso/", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		logger.Get().Debug().Str("path", r.URL.Path).Msg("Redirecting /admin/iso/ to /admin/iso")
		http.Redirect(w, r, "/admin/iso", http.StatusSeeOther)
	})))
	router.POST("/admin/iso/toggle", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.ToggleISOHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	// Server-rendered limits form (no JS)
	router.POST("/admin/limits", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.UpdateLimitsFormHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	// Limits page (protected)
	router.GET("/admin/limits", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.LimitsPageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
	// Trailing-slash variant: redirect to canonical path
	router.GET("/admin/limits/", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		logger.Get().Debug().Str("path", r.URL.Path).Msg("Redirecting /admin/limits/ to /admin/limits")
		http.Redirect(w, r, "/admin/limits", http.StatusSeeOther)
	})))

	// API routes protected by authentication
	router.GET("/api/settings", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.GetSettingsHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	router.GET("/api/settings/iso", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.GetAllISOsHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	router.GET("/api/vmbr/all", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.GetAllVMBRsHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
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
