package handlers

import (
	"net/http"
	"net/url"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/state"

	"github.com/julienschmidt/httprouter"
)

// VMBRHandler handles VMBR-related operations.
type VMBRHandler struct {
	stateManager state.StateManager
}

// ToggleVMBRHandler toggles a single VMBR enable state (auto-save without JS)
func (h *VMBRHandler) ToggleVMBRHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().Str("handler", "ToggleVMBRHandler").Logger()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data.", http.StatusBadRequest)
		return
	}

	name := r.FormValue("vmbr")
	action := r.FormValue("action") // enable|disable
	if name == "" || (action != "enable" && action != "disable") {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	gs := h.stateManager
	settings := gs.GetSettings()
	if settings.VMBRs == nil {
		settings.VMBRs = []string{}
	}

	enabled := make(map[string]bool, len(settings.VMBRs))
	for _, v := range settings.VMBRs {
		enabled[v] = true
	}

	changed := false
	if action == "enable" {
		if !enabled[name] {
			settings.VMBRs = append(settings.VMBRs, name)
			changed = true
		}
	} else { // disable
		if enabled[name] {
			filtered := make([]string, 0, len(settings.VMBRs))
			for _, v := range settings.VMBRs {
				if v != name {
					filtered = append(filtered, v)
				}
			}
			settings.VMBRs = filtered
			changed = true
		}
	}

	if changed {
		if err := gs.SetSettings(settings); err != nil {
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
	log := logger.Get().With().Str("handler", "VMBRPageHandler").Logger()

	// Get the global state
	gs := h.stateManager
	client := gs.GetProxmoxClient()
	proxmoxConnected, proxmoxMsg := gs.GetProxmoxStatus()

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

	// Get current settings to check which VMBRs are enabled
	settings := gs.GetSettings()
	enabledVMBRs := make(map[string]bool)
	for _, vmbr := range settings.VMBRs {
		enabledVMBRs[vmbr] = true
	}

	// Success banner via query params
	success := r.URL.Query().Get("success") != ""
	act := r.URL.Query().Get("action")
	vmbrName := r.URL.Query().Get("vmbr")
	var successMsg string
	if success {
		switch act {
		case "enable":
			successMsg = "VMBR '" + vmbrName + "' enabled"
		case "disable":
			successMsg = "VMBR '" + vmbrName + "' disabled"
		default:
			successMsg = "VMBR settings updated"
		}
	}

	// Prepare template data
	// Map to template shape used previously: name field instead of iface
	vmbrsForTemplate := make([]map[string]string, 0, len(allVMBRs))
	for _, v := range allVMBRs {
		vmbrsForTemplate = append(vmbrsForTemplate, map[string]string{
			"node":        v["node"],
			"name":        v["iface"],
			"description": v["description"],
		})
	}

	templateData := map[string]interface{}{
		"VMBRs":          vmbrsForTemplate,
		"EnabledVMBRs":   enabledVMBRs,
		"Success":        success,
		"SuccessMessage": successMsg,
		"AdminActive":    "vmbr",
	}
	if err != nil {
		templateData["Error"] = err.Error()
	}
	// Pass Proxmox status flags for consistent UI behavior
	templateData["ProxmoxConnected"] = proxmoxConnected
	if !proxmoxConnected && proxmoxMsg != "" {
		templateData["ProxmoxError"] = proxmoxMsg
	}

	// Render the template
	i18n.LocalizePage(w, r, templateData)
	renderTemplateInternal(w, r, "admin_vmbr", templateData)
}

// UpdateVMBRHandler handles updating enabled VMBRs.
func (h *VMBRHandler) UpdateVMBRHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().Str("handler", "UpdateVMBRHandler").Logger()

	if err := r.ParseForm(); err != nil {
		log.Warn().Err(err).Msg("Error parsing form data")
		http.Error(w, "Invalid form data.", http.StatusBadRequest)
		return
	}

	// Get the list of enabled VMBRs from the form
	enabledVMBRs := r.Form["enabled_vmbrs"]

	// Update settings
	gs := h.stateManager
	settings := gs.GetSettings()
	settings.VMBRs = enabledVMBRs

	if err := gs.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to update settings")
		http.Error(w, "Failed to update settings", http.StatusInternalServerError)
		return
	}

	// Redirect back to the VMBR page with a success banner
	http.Redirect(w, r, "/admin/vmbr?success=1", http.StatusSeeOther)
}

// RegisterRoutes registers the routes for VMBR management.
func (h *VMBRHandler) RegisterRoutes(router *httprouter.Router) {
	router.GET("/admin/vmbr", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.VMBRPageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
	router.POST("/admin/vmbr/update", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.UpdateVMBRHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
	router.POST("/admin/vmbr/toggle", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.ToggleVMBRHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
}
