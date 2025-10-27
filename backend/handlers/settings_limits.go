package handlers

import (
	"context"
	"net/http"
	"net/url"
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

	// Extract success/error message from query params
	successMsg := ""
	errorMsg := ""
	if r.URL.Query().Get("success") == "1" {
		entity := r.URL.Query().Get("entity")
		nodeParam := r.URL.Query().Get("node")
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
	} else if r.URL.Query().Get("error") == "1" {
		errorMsg = r.URL.Query().Get("errorMsg")
		if errorMsg == "" {
			errorMsg = "An error occurred while updating limits"
		}
	}

	// Use standard admin page helper
	data := AdminPageDataWithMessage("", "limits", successMsg, errorMsg)
	data["TitleKey"] = "Admin.Limits.Title"

	// Add limits data
	data["Limits"] = settings.Limits

	// Add selected node from query params
	nodeParam := r.URL.Query().Get("node")
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

	// Get resource usage for all nodes
	var nodeUsage map[string]*NodeResourceUsage
	var nodeCapacities map[string]*NodeCapacity
	if proxmoxConnected && client != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if usage, err := CalculateNodeResourceUsage(ctx, client, h.stateManager); err != nil {
			log.Warn().Err(err).Msg("Failed to calculate node resource usage")
		} else {
			nodeUsage = usage
		}

		// Get node capacities
		if nodeNames != nil {
			nodeCapacities = make(map[string]*NodeCapacity)
			for _, nodeName := range nodeNames {
				if capacity, err := GetNodeCapacity(ctx, client, nodeName); err == nil {
					nodeCapacities[nodeName] = capacity
				}
			}
		}
	}
	data["NodeUsage"] = nodeUsage
	data["NodeCapacities"] = nodeCapacities

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
		redirect := "/admin/limits?error=1&errorMsg=" + url.QueryEscape("Missing entity type")
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}

	// Helper to parse an int field safely with minimum value validation
	parseInt := func(name string, fallback int) int {
		v := r.FormValue(name)
		if v == "" {
			return fallback
		}
		if n, err := strconv.Atoi(v); err == nil {
			// Ensure value is at least 1 (no zero or negative values)
			if n < 1 {
				return 1
			}
			return n
		}
		return fallback
	}

	// Extract values
	// Note: sockets and cores min are always 1, no need for user input
	socketsMax := parseInt("sockets-max", 1)
	coresMax := parseInt("cores-max", 1)
	ramMin := parseInt("ram-min", 1)
	ramMax := parseInt("ram-max", ramMin)
	diskMin := parseInt("disk-min", 1)
	diskMax := parseInt("disk-max", diskMin)

	// Validate that max values are >= min values
	if ramMax < ramMin {
		ramMax = ramMin
	}
	if diskMax < diskMin {
		diskMax = diskMin
	}

	// Load settings
	settings := h.stateManager.GetSettings()
	if settings == nil {
		redirect := "/admin/limits?error=1&errorMsg=" + url.QueryEscape("Settings not available")
		http.Redirect(w, r, redirect, http.StatusSeeOther)
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
		entityMap["sockets"] = map[string]int{"min": 1, "max": socketsMax}
		entityMap["cores"] = map[string]int{"min": 1, "max": coresMax}
		entityMap["ram"] = map[string]int{"min": ramMin, "max": ramMax}
		entityMap["disk"] = map[string]int{"min": diskMin, "max": diskMax}
		settings.Limits["vm"] = entityMap

	case "node", "nodes":
		// Per-node limits under limits.nodes[<nodeName>]
		nodeName := strings.TrimSpace(r.FormValue("nodeName"))
		if nodeName == "" {
			redirect := "/admin/limits?error=1&entity=nodes&errorMsg=" + url.QueryEscape("Missing node name")
			http.Redirect(w, r, redirect, http.StatusSeeOther)
			return
		}

		// Validate that limits don't exceed node physical capacity
		client := h.stateManager.GetProxmoxClient()
		if client != nil && coresMax > 0 && ramMax > 0 {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			if err := ValidateNodeLimitsAgainstCapacity(ctx, client, nodeName, coresMax, ramMax); err != nil {
				log.Warn().Err(err).Str("node", nodeName).Msg("Node limits validation failed")
				// Redirect back with error message
				redirect := "/admin/limits?error=1&entity=nodes&node=" + url.QueryEscape(nodeName) + "&errorMsg=" + url.QueryEscape(err.Error())
				http.Redirect(w, r, redirect, http.StatusSeeOther)
				return
			}
		}

		nodesMap, _ := settings.Limits["nodes"].(map[string]interface{})
		if nodesMap == nil {
			nodesMap = make(map[string]interface{})
		}
		nodeEntry, _ := nodesMap[nodeName].(map[string]interface{})
		if nodeEntry == nil {
			nodeEntry = make(map[string]interface{})
		}
		nodeEntry["sockets"] = map[string]int{"min": 1, "max": socketsMax}
		nodeEntry["cores"] = map[string]int{"min": 1, "max": coresMax}
		nodeEntry["ram"] = map[string]int{"min": ramMin, "max": ramMax}
		nodesMap[nodeName] = nodeEntry
		settings.Limits["nodes"] = nodesMap
		entity = "nodes" // normalize for redirect

	default:
		redirect := "/admin/limits?error=1&errorMsg=" + url.QueryEscape("Unsupported entity type")
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}

	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save limits settings")
		redirect := "/admin/limits?error=1&entity=" + entity
		if entity == "nodes" {
			redirect += "&node=" + url.QueryEscape(strings.TrimSpace(r.FormValue("nodeName")))
		}
		redirect += "&errorMsg=" + url.QueryEscape("Failed to save settings: "+err.Error())
		http.Redirect(w, r, redirect, http.StatusSeeOther)
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
