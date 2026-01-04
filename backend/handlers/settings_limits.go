package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/components"
	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/security"
	"pvmss/state"
)

// LimitsPageHandler renders the Resource Limits page (server-rendered)
func (h *SettingsHandler) LimitsPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("LimitsPageHandler", r)

	settings := h.stateManager.GetSettings()
	localizer := i18n.GetLocalizerFromRequest(r)
	if settings == nil {
		http.Error(w, i18n.Localize(localizer, "Error.SettingsUnavailable"), http.StatusInternalServerError)
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
			successMsg = i18n.Localize(localizer, "Admin.Limits.Success.VM")
		case "user":
			successMsg = i18n.Localize(localizer, "Admin.Limits.Success.User")
		case "nodes":
			if nodeParam != "" {
				tmpl := i18n.Localize(localizer, "Admin.Limits.Success.NodeWithName")
				successMsg = fmt.Sprintf(tmpl, nodeParam)
			} else {
				successMsg = i18n.Localize(localizer, "Admin.Limits.Success.Nodes")
			}
		default:
			successMsg = i18n.Localize(localizer, "Admin.Limits.Success.Generic")
		}
	} else if r.URL.Query().Get("error") == "1" {
		errorMsg = r.URL.Query().Get("errorMsg")
		if errorMsg == "" {
			errorMsg = i18n.Localize(localizer, "Admin.Limits.Error.Generic")
		}
	}

	// Build template data with functional options
	limitsData := h.stateManager.GetLimits()
	opts := []TemplateOption{
		WithAdminActive("limits"),
		WithAuth(r),
		WithProxmoxStatus(h.stateManager),
		WithMessages(r),
		WithData("TitleKey", "Admin.Limits.Title"),
		WithData("Limits", limitsData),
		WithData("Settings", settings),
		WithData("Node", r.URL.Query().Get("node")),
	}

	if successMsg != "" {
		opts = append(opts, WithSuccess(successMsg))
	}
	if errorMsg != "" {
		opts = append(opts, WithError(errorMsg))
	}

	snapshot := h.stateManager.GetProxmoxSnapshot()

	// Get node names for dropdown
	var nodeNames []string
	client := h.stateManager.GetProxmoxClient()
	offlineMode := h.stateManager.IsOfflineMode()
	proxmoxConnected := client != nil && !offlineMode

	if snapshot != nil && len(snapshot.NodeNames) > 0 {
		nodeNames = append(nodeNames, snapshot.NodeNames...)
	} else if proxmoxConnected {
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
	} else {
		settingsNodes, _, _ := deriveNodesFromSettings(settings)
		nodeNames = settingsNodes
	}

	nodeNames = uniqueNonEmptyStrings(nodeNames)
	// Always provide NodeNames (empty array if no nodes available)
	// Ensure alphabetical order of nodes in the dropdown
	if len(nodeNames) > 1 {
		sort.Strings(nodeNames)
	}
	_ = append(opts, WithData("NodeNames", nodeNames))

	// Get resource usage for all nodes
	var nodeUsage map[string]*NodeResourceUsage
	var nodeCapacities map[string]*NodeCapacity
	if snapshot != nil {
		nodeCapacities = buildNodeCapacitiesFromSnapshot(snapshot)
	}
	if proxmoxConnected {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if usage, err := CalculateNodeResourceUsage(ctx, client, h.stateManager); err != nil {
			log.Warn().Err(err).Msg("Failed to calculate node resource usage")
		} else {
			nodeUsage = usage
		}

		// Get node capacities
		if nodeNames != nil {
			if nodeCapacities == nil {
				nodeCapacities = make(map[string]*NodeCapacity)
			}
			for _, nodeName := range nodeNames {
				if nodeCapacities[nodeName] != nil {
					continue
				}
				if capacity, err := GetNodeCapacity(ctx, client, nodeName); err == nil {
					nodeCapacities[nodeName] = capacity
				}
			}
		}
	}
	if nodeUsage == nil {
		nodeUsage = make(map[string]*NodeResourceUsage)
	}
	if nodeCapacities == nil {
		_ = make(map[string]*NodeCapacity)
	}
	// Convert nodeUsage to components.NodeUsage
	templNodeUsage := make(map[string]*components.NodeUsage)
	for nodeName, usage := range nodeUsage {
		if usage != nil {
			templNodeUsage[nodeName] = &components.NodeUsage{
				TotalVMs: usage.TotalVMs,
				Cores:    usage.Cores,
				MaxCores: usage.MaxCores,
				RamGB:    usage.RamGB,
				MaxRamGB: usage.MaxRamGB,
			}
		}
	}

	// Build Templ data
	limitsTemplData := components.AdminLimitsData{
		Username:         getUsernameFromSession(r),
		Lang:             i18n.GetLanguage(r),
		CSRFToken:        getCSRFTokenFromContext(r),
		ProxmoxConnected: proxmoxConnected,
		Node:             r.URL.Query().Get("node"),
		NodeNames:        nodeNames,
		NodeUsage:        templNodeUsage,
		Limits:           limitsData,
		Settings: &components.LimitsSettings{
			MaxVMPerUser: 0, // Not in current settings structure
			MaxSnapshots: settings.Limits.MaxSnapshots,
		},
	}

	T := getTranslationFunc(r)
	if err := components.AdminLimitsPage(limitsTemplData, T).Render(r.Context(), w); err != nil {
		log.Error().Err(err).Msg("Failed to render admin limits page")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// UpdateLimitsFormHandler handles POST from admin_limits.html to update VM/Node limits
// TODO Telmate migration: this handler uses Telmate-based node helpers to validate limits; migrate it to the Resty node helpers and drop the ClientInterface usage.
func (h *SettingsHandler) UpdateLimitsFormHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("UpdateLimitsFormHandler", r)

	localizer := i18n.GetLocalizerFromRequest(r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	entity := r.FormValue("entityId") // "vm" or "node"
	if entity == "" {
		redirect := "/admin/limits?error=1&errorMsg=" + url.QueryEscape(i18n.Localize(localizer, "Admin.Limits.Error.MissingEntity"))
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
		redirect := "/admin/limits?error=1&errorMsg=" + url.QueryEscape(i18n.Localize(localizer, "Admin.Limits.Error.SettingsUnavailable"))
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}
	if settings.Limits.VM.Sockets.Min == 0 {
		settings.Limits = state.LimitsConfig{
			VM: state.VMResourceLimits{
				Sockets: state.ResourceRange{Min: 1, Max: 1},
				Cores:   state.ResourceRange{Min: 1, Max: 2},
				RAM:     state.ResourceRange{Min: 1, Max: 4},
				Disk:    state.ResourceRange{Min: 1, Max: 10},
			},
			Nodes: make(map[string]state.NodeResourceLimits),
		}
	}

	// Persist limits
	switch entity {
	case "vm":
		// Flat VM limits
		settings.Limits.VM.Sockets = state.ResourceRange{Min: 1, Max: socketsMax}
		settings.Limits.VM.Cores = state.ResourceRange{Min: 1, Max: coresMax}
		settings.Limits.VM.RAM = state.ResourceRange{Min: ramMin, Max: ramMax}
		settings.Limits.VM.Disk = state.ResourceRange{Min: diskMin, Max: diskMax}

	case "node", "nodes":
		// Per-node limits under limits.nodes[<nodeName>]
		nodeName := strings.TrimSpace(r.FormValue("nodeName"))
		if nodeName == "" {
			redirect := "/admin/limits?error=1&entity=nodes&errorMsg=" + url.QueryEscape(i18n.Localize(localizer, "Admin.Limits.Error.MissingNodeName"))
			http.Redirect(w, r, redirect, http.StatusSeeOther)
			return
		}

		// Validate that limits don't exceed node physical capacity
		client := h.stateManager.GetProxmoxClient()
		if client != nil && coresMax > 0 && ramMax > 0 {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			if err := ValidateNodeLimitsAgainstCapacity(ctx, client, nodeName, coresMax, ramMax, localizer); err != nil {
				log.Warn().Err(err).Str("node", nodeName).Msg("Node limits validation failed")
				// Redirect back with error message
				redirect := "/admin/limits?error=1&entity=nodes&node=" + url.QueryEscape(nodeName) + "&errorMsg=" + url.QueryEscape(err.Error())
				http.Redirect(w, r, redirect, http.StatusSeeOther)
				return
			}
		}

		if settings.Limits.Nodes == nil {
			settings.Limits.Nodes = make(map[string]state.NodeResourceLimits)
		}

		settings.Limits.Nodes[nodeName] = state.NodeResourceLimits{
			Sockets: state.ResourceRange{Min: 1, Max: socketsMax},
			Cores:   state.ResourceRange{Min: 1, Max: coresMax},
			RAM:     state.ResourceRange{Min: ramMin, Max: ramMax},
			Disk:    state.ResourceRange{Min: diskMin, Max: diskMax},
		}
		entity = "nodes" // normalize for redirect

	default:
		redirect := "/admin/limits?error=1&errorMsg=" + url.QueryEscape(i18n.Localize(localizer, "Admin.Limits.Error.UnsupportedEntity"))
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}

	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save limits settings")
		redirect := "/admin/limits?error=1&entity=" + entity
		if entity == "nodes" {
			redirect += "&node=" + url.QueryEscape(strings.TrimSpace(r.FormValue("nodeName")))
		}
		base := i18n.Localize(localizer, "Admin.Limits.Error.SaveFailed")
		redirect += "&errorMsg=" + url.QueryEscape(fmt.Sprintf(base, err.Error()))
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}

	// Audit log for admin limits update with structured event
	username := ""
	if sessionManager := security.GetSession(r); sessionManager != nil {
		if user, ok := sessionManager.Get(r.Context(), "username").(string); ok {
			username = user
		}
	}

	switch entity {
	case "vm":
		logger.AdminEvent("limits_update_vm", username).
			Int("sockets_max", socketsMax).
			Int("cores_max", coresMax).
			Int("ram_min", ramMin).
			Int("ram_max", ramMax).
			Int("disk_min", diskMin).
			Int("disk_max", diskMax).
			Str("client_ip", r.RemoteAddr).
			Msg("VM limits updated")
	case "nodes":
		nodeName := strings.TrimSpace(r.FormValue("nodeName"))
		logger.AdminEvent("limits_update_node", username).
			Str("node_name", nodeName).
			Int("sockets_max", socketsMax).
			Int("cores_max", coresMax).
			Int("ram_min", ramMin).
			Int("ram_max", ramMax).
			Int("disk_min", diskMin).
			Int("disk_max", diskMax).
			Str("client_ip", r.RemoteAddr).
			Msg("Node limits updated")
	}

	// Redirect back to limits page with success banner and context
	redirect := "/admin/limits?success=1&entity=" + entity
	if entity == "nodes" {
		redirect += "&node=" + strings.TrimSpace(r.FormValue("nodeName"))
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

func uniqueNonEmptyStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, exists := seen[v]; exists {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	return result
}

func buildNodeCapacitiesFromSnapshot(snapshot *state.ProxmoxClusterSnapshot) map[string]*NodeCapacity {
	if snapshot == nil || len(snapshot.NodeDetails) == 0 {
		return nil
	}

	capacities := make(map[string]*NodeCapacity, len(snapshot.NodeDetails))
	for _, detail := range snapshot.NodeDetails {
		if detail == nil || detail.Node == "" {
			continue
		}
		memoryBytes := int64(detail.MaxMemory)
		capacities[detail.Node] = &NodeCapacity{
			Node:     detail.Node,
			CPUs:     detail.MaxCPU,
			MemoryMB: memoryBytes / (1024 * 1024),
			MemoryGB: int(memoryBytes / (1024 * 1024 * 1024)),
		}
	}
	return capacities
}

// UpdateUserLimitsHandler handles POST from admin_limits.html to update user limits (max VMs per user)
func (h *SettingsHandler) UpdateUserLimitsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("UpdateUserLimitsHandler", r)

	localizer := i18n.GetLocalizerFromRequest(r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	// Parse max_vm_per_user value
	maxVMPerUserStr := r.FormValue("max_vm_per_user")
	if maxVMPerUserStr == "" {
		redirect := "/admin/limits?error=1&errorMsg=" + url.QueryEscape(i18n.Localize(localizer, "Admin.Limits.Error.MissingMaxVMPerUser"))
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}

	maxVMPerUser, err := strconv.Atoi(maxVMPerUserStr)
	if err != nil || maxVMPerUser < state.MinVMPerUser || maxVMPerUser > state.MaxVMPerUser {
		log.Warn().
			Str("value", maxVMPerUserStr).
			Int("min", state.MinVMPerUser).
			Int("max", state.MaxVMPerUser).
			Msg("Invalid max_vm_per_user value")
		redirect := "/admin/limits?error=1&errorMsg=" + url.QueryEscape(i18n.Localize(localizer, "Admin.Limits.Error.InvalidMaxVMPerUser"))
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}

	// Parse max_snapshots_per_vm value
	maxSnapshotsPerVMStr := r.FormValue("max_snapshots_per_vm")
	if maxSnapshotsPerVMStr == "" {
		redirect := "/admin/limits?error=1&errorMsg=" + url.QueryEscape(i18n.Localize(localizer, "Admin.Limits.Error.MissingMaxSnapshotsPerVM"))
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}

	maxSnapshotsPerVM, err := strconv.Atoi(maxSnapshotsPerVMStr)
	if err != nil || maxSnapshotsPerVM < state.MinSnapshotsPerVM || maxSnapshotsPerVM > state.MaxSnapshotsPerVM {
		log.Warn().
			Str("value", maxSnapshotsPerVMStr).
			Int("min", state.MinSnapshotsPerVM).
			Int("max", state.MaxSnapshotsPerVM).
			Msg("Invalid max_snapshots_per_vm value")
		redirect := "/admin/limits?error=1&errorMsg=" + url.QueryEscape(i18n.Localize(localizer, "Admin.Limits.Error.InvalidMaxSnapshotsPerVM"))
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}

	// Load settings
	settings := h.stateManager.GetSettings()
	if settings == nil {
		redirect := "/admin/limits?error=1&errorMsg=" + url.QueryEscape(i18n.Localize(localizer, "Admin.Limits.Error.SettingsUnavailable"))
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}

	// Update max_vm_per_user
	settings.MaxVMPerUser = maxVMPerUser
	// Update max_snapshots_per_vm (save to limits.max_snapshots)
	settings.Limits.MaxSnapshots = maxSnapshotsPerVM

	// Save settings
	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save user limits settings")
		base := i18n.Localize(localizer, "Admin.Limits.Error.SaveFailed")
		redirect := "/admin/limits?error=1&errorMsg=" + url.QueryEscape(fmt.Sprintf(base, err.Error()))
		http.Redirect(w, r, redirect, http.StatusSeeOther)
		return
	}

	// Audit log for user limits update
	username := ""
	if sessionManager := security.GetSession(r); sessionManager != nil {
		if user, ok := sessionManager.Get(r.Context(), "username").(string); ok {
			username = user
		}
	}
	logger.AdminEvent("limits_update_user", username).
		Int("max_vm_per_user", maxVMPerUser).
		Str("client_ip", r.RemoteAddr).
		Msg("User limits updated")

	// Redirect back to limits page with success banner
	redirect := "/admin/limits?success=1&entity=user"
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

	// Register user limits route manually (custom key not handled by RegisterCRUDRoutes)
	routeHelpers.helpers.RegisterAdminRoute(router, "POST", "/admin/limits/update-user", h.UpdateUserLimitsHandler)
}
