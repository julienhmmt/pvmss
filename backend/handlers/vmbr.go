package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"

	"github.com/julienschmidt/httprouter"

	"pvmss/proxmox"
	"pvmss/state"
)

// removeFromList removes an item from a string list
func removeFromList(list []string, item string) []string {
	result := make([]string, 0, len(list))
	for _, v := range list {
		if v != item {
			result = append(result, v)
		}
	}
	return result
}

// buildVMBRSuccessMessage creates success message from query parameters
func buildVMBRSuccessMessage(r *http.Request) string {
	if r.URL.Query().Get("success") == "" {
		return ""
	}

	action := r.URL.Query().Get("action")
	name := r.URL.Query().Get("vmbr")

	switch action {
	case "enable":
		return "VMBR '" + name + "' enabled"
	case "disable":
		return "VMBR '" + name + "' disabled"
	case "update_network_cards":
		return "Network cards configuration updated successfully"
	default:
		return "VMBR settings updated"
	}
}

// VMBRHandler handles VMBR-related operations.
type VMBRHandler struct {
	stateManager state.StateManager
}

// UpdateNetworkCardsHandler updates the maximum number of network cards per VM
func (h *VMBRHandler) UpdateNetworkCardsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("UpdateNetworkCardsHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	maxNetworkCardsStr := r.FormValue("max_network_cards")
	if maxNetworkCardsStr == "" {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	maxNetworkCards := 1
	if _, err := fmt.Sscanf(maxNetworkCardsStr, "%d", &maxNetworkCards); err != nil {
		log.Error().Err(err).Str("value", maxNetworkCardsStr).Msg("Failed to parse max_network_cards")
		http.Error(w, "Invalid number", http.StatusBadRequest)
		return
	}

	// Validate range (1-10)
	if maxNetworkCards < 1 || maxNetworkCards > 10 {
		log.Warn().Int("value", maxNetworkCards).Msg("Max network cards out of range, clamping")
		if maxNetworkCards < 1 {
			maxNetworkCards = 1
		} else {
			maxNetworkCards = 10
		}
	}

	settings := h.stateManager.GetSettings()
	settings.MaxNetworkCards = maxNetworkCards

	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to update settings")
		http.Error(w, "Failed to update settings", http.StatusInternalServerError)
		return
	}

	log.Info().Int("max_network_cards", maxNetworkCards).Msg("Updated max network cards setting")
	redirectURL := "/admin/vmbr?success=1&action=update_network_cards"
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// ToggleVMBRHandler toggles a single VMBR enable state (auto-save without JS)
func (h *VMBRHandler) ToggleVMBRHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("ToggleVMBRHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	name := r.FormValue("vmbr")
	node := r.FormValue("node")
	action := r.FormValue("action") // enable|disable
	if name == "" || node == "" || (action != "enable" && action != "disable") {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Create unique identifier combining node and vmbr name
	uniqueID := node + ":" + name

	settings := h.stateManager.GetSettings()
	if settings.VMBRs == nil {
		settings.VMBRs = []string{}
	}

	enabled := make(map[string]bool, len(settings.VMBRs))
	for _, v := range settings.VMBRs {
		enabled[v] = true
	}

	changed := false
	if action == "enable" {
		if !enabled[uniqueID] {
			settings.VMBRs = append(settings.VMBRs, uniqueID)
			changed = true
		}
	} else { // disable
		if enabled[uniqueID] {
			settings.VMBRs = removeFromList(settings.VMBRs, uniqueID)
			changed = true
		}
	}

	if changed {
		if err := h.stateManager.SetSettings(settings); err != nil {
			log.Error().Err(err).Msg("Failed to update settings")
			http.Error(w, "Failed to update settings", http.StatusInternalServerError)
			return
		}
	}

	redirectURL := "/admin/vmbr?success=1&action=" + action + "&vmbr=" + url.QueryEscape(name)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// NewVMBRHandler creates a new instance of VMBRHandler.
func NewVMBRHandler(sm state.StateManager) *VMBRHandler {
	return &VMBRHandler{stateManager: sm}
}

// VMBRPageHandler renders the VMBR management page.
func (h *VMBRHandler) VMBRPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("VMBRPageHandler", r)

	client := h.stateManager.GetProxmoxClient()
	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()

	// Collect all VMBRs using common helper when online; otherwise short-circuit
	var allVMBRs []map[string]string
	var err error
	if client == nil || !proxmoxConnected {
		// Proceed gracefully in offline/read-only mode like AdminPageHandler
		log.Warn().Bool("connected", proxmoxConnected).Msg("Proxmox not available; rendering page with empty VMBR list")
		allVMBRs = []map[string]string{}
	} else {
		allVMBRs, err = collectAllVMBRs(h.stateManager)
		if err != nil {
			log.Warn().Err(err).Msg("collectAllVMBRs returned an error; continuing")
		}
		log.Info().Int("vmbr_total", len(allVMBRs)).Msg("Total VMBRs prepared for template")
	}

	// Get all nodes for the selector
	var allNodes []string
	if client != nil {
		allNodes, _ = proxmox.GetNodeNames(client)
	}

	// Get current settings to check which VMBRs are enabled
	settings := h.stateManager.GetSettings()
	enabledVMBRs := make(map[string]bool, len(settings.VMBRs))
	for _, vmbr := range settings.VMBRs {
		enabledVMBRs[vmbr] = true
	}

	successMsg := buildVMBRSuccessMessage(r)

	// Prepare template data
	// Map to template shape used previously: name field instead of iface
	vmbrsForTemplate := make([]map[string]string, 0, len(allVMBRs))
	for _, v := range allVMBRs {
		// Create unique identifier combining node and vmbr name
		uniqueID := v["node"] + ":" + v["iface"]
		vmbrsForTemplate = append(vmbrsForTemplate, map[string]string{
			"node":        v["node"],
			"name":        v["iface"],
			"description": v["description"],
			"unique_id":   uniqueID,
		})
	}

	nodeSet := make(map[string]struct{}, len(vmbrsForTemplate))
	for _, vmbr := range vmbrsForTemplate {
		node := vmbr["node"]
		if node != "" {
			nodeSet[node] = struct{}{}
		}
	}

	if len(nodeSet) <= 1 {
		sort.Slice(vmbrsForTemplate, func(i, j int) bool {
			return vmbrsForTemplate[i]["name"] < vmbrsForTemplate[j]["name"]
		})
	} else {
		sort.Slice(vmbrsForTemplate, func(i, j int) bool {
			if vmbrsForTemplate[i]["node"] == vmbrsForTemplate[j]["node"] {
				return vmbrsForTemplate[i]["name"] < vmbrsForTemplate[j]["name"]
			}
			return vmbrsForTemplate[i]["node"] < vmbrsForTemplate[j]["node"]
		})
	}

	builder := NewTemplateData("").
		SetAdminActive("vmbr").
		SetAuth(r).
		SetProxmoxStatus(h.stateManager).
		ParseMessages(r).
		AddData("TitleKey", "Admin.VMBR.Title").
		AddData("EnabledVMBRs", enabledVMBRs).
		AddData("Nodes", allNodes).
		AddData("MaxNetworkCards", settings.MaxNetworkCards).
		AddData("VMBRs", vmbrsForTemplate)

	if successMsg != "" {
		builder.SetSuccess(successMsg)
	}
	if err != nil {
		builder.AddData("Error", err.Error())
	}

	templateData := builder.Build().ToMap()
	renderTemplateInternal(w, r, "admin_vmbr", templateData)
}

// RegisterRoutes registers the routes for VMBR management.
func (h *VMBRHandler) RegisterRoutes(router *httprouter.Router) {
	routeHelpers := NewAdminPageRoutes()

	// Register admin VMBR routes using helper
	routeHelpers.RegisterCRUDRoutes(router, "/admin/vmbr", map[string]func(w http.ResponseWriter, r *http.Request, ps httprouter.Params){
		"page":   h.VMBRPageHandler,
		"toggle": h.ToggleVMBRHandler,
	})

	// Register custom route for network cards configuration
	routeHelpers.helpers.RegisterAdminRoute(router, "POST", "/admin/vmbr/update-network-cards", h.UpdateNetworkCardsHandler)
}
