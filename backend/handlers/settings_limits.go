package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"pvmss/proxmox"
)

// LimitsPageHandler renders the Resource Limits page (server-rendered)
func (h *SettingsHandler) LimitsPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("LimitsPageHandler", r)

	settings := h.stateManager.GetSettings()
	if settings == nil {
		http.Error(w, "Settings not available", http.StatusInternalServerError)
		return
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

	// Use standard admin page helper
	data := AdminPageDataWithMessage("Resource Limits", "limits", successMsg, "")

	// Add limits data
	data["Limits"] = settings.Limits

	// Add selected node from query params
	data["Node"] = nodeParam

	// Get node names for dropdown
	var nodeNames []string
	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()
	client := h.stateManager.GetProxmoxClient()

	if proxmoxConnected && client != nil {
		pc, ok := client.(*proxmox.Client)
		if ok {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			nodes, err := proxmox.GetNodeNamesWithContext(ctx, pc)
			if err != nil {
				log.Warn().Err(err).Msg("Unable to retrieve Proxmox nodes for limits page")
			} else {
				nodeNames = nodes
			}
		}
	}

	// Always provide NodeNames (empty array if no nodes available)
	if nodeNames == nil {
		nodeNames = []string{}
	}
	data["NodeNames"] = nodeNames

	renderTemplateInternal(w, r, "admin_limits", data)
}

// UpdateLimitsFormHandler handles POST from admin_limits.html to update VM/Node limits
func (h *SettingsHandler) UpdateLimitsFormHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("UpdateLimitsFormHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
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
	redirect := "/admin/limits?success=1&entity=" + entity
	if entity == "nodes" {
		redirect += "&node=" + strings.TrimSpace(r.FormValue("nodeName"))
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

// RegisterLimitsRoutes registers limits-related routes
func (h *SettingsHandler) RegisterLimitsRoutes(router *httprouter.Router) {
	routeHelpers := NewAdminPageRoutes()

	// Register admin limits routes using helper
	routeHelpers.RegisterCRUDRoutes(router, "/admin/limits", map[string]func(w http.ResponseWriter, r *http.Request, ps httprouter.Params){
		"page":   h.LimitsPageHandler,
		"update": h.UpdateLimitsFormHandler,
	})
}
