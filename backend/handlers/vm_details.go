package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// VMStateManager defines the minimal state contract needed by VM details.
type VMStateManager interface {
	GetProxmoxClient() proxmox.ClientInterface
	GetSettings() *state.AppSettings
}

// UpdateVMDescriptionHandler updates the VM description (Markdown supported on display)
func (h *VMHandler) UpdateVMDescriptionHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().Str("handler", "UpdateVMDescriptionHandler").Logger()
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		log.Warn().Err(err).Msg("parse form failed")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	vmid := strings.TrimSpace(r.FormValue("vmid"))
	node := strings.TrimSpace(r.FormValue("node"))
	desc := r.FormValue("description")
	if vmid == "" || node == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	// Update description (empty allowed to clear)
	if err := proxmox.UpdateVMConfigWithContext(r.Context(), client, node, vmidInt, map[string]string{"description": desc}); err != nil {
		log.Error().Err(err).Msg("update description failed")
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		return
	}
	http.Redirect(w, r, "/vm/details/"+vmid+"?refresh=1", http.StatusSeeOther)
}

// UpdateVMTagsHandler updates the VM tags from selected checkboxes
func (h *VMHandler) UpdateVMTagsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().Str("handler", "UpdateVMTagsHandler").Logger()
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		log.Warn().Err(err).Msg("parse form failed")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	vmid := strings.TrimSpace(r.FormValue("vmid"))
	node := strings.TrimSpace(r.FormValue("node"))
	if vmid == "" || node == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	// Gather tags; ensure stable format for Proxmox: semicolon-separated, unique, trimmed
	sel := r.Form["tags"]
	seen := make(map[string]struct{}, len(sel))
	out := make([]string, 0, len(sel))
	for _, t := range sel {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	// Optionally enforce default tag "pvmss" if available in settings
	if settings := h.stateManager.GetSettings(); settings != nil {
		for _, at := range settings.Tags {
			if strings.EqualFold(at, "pvmss") {
				if _, ok := seen["pvmss"]; !ok {
					out = append(out, "pvmss")
				}
				break
			}
		}
	}
	tagsParam := strings.Join(out, ";")

	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if err := proxmox.UpdateVMConfigWithContext(r.Context(), client, node, vmidInt, map[string]string{"tags": tagsParam}); err != nil {
		log.Error().Err(err).Msg("update tags failed")
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		return
	}
	http.Redirect(w, r, "/vm/details/"+vmid+"?refresh=1", http.StatusSeeOther)
}

// VMHandler handles routes related to virtual machines
type VMHandler struct {
	stateManager VMStateManager
}

// NewVMHandler creates a new VMHandler instance
func NewVMHandler(stateManager VMStateManager) *VMHandler {
	return &VMHandler{
		stateManager: stateManager,
	}
}

// VMDetailsHandler handles the VM details page
func (h *VMHandler) VMDetailsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "VMHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	vmID := ps.ByName("id")
	if vmID == "" {
		log.Warn().Msg("VM ID not provided in request")
		// Localized generic error for bad request
		localizer := i18n.GetLocalizer(r)
		http.Error(w, i18n.Localize(localizer, "Error.Generic"), http.StatusBadRequest)
		return
	}

	log = log.With().Str("vm_id", vmID).Logger()
	log.Debug().Msg("Fetching VM details")

	// Get Proxmox client and search VM by ID
	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not initialized")
		localizer := i18n.GetLocalizer(r)
		http.Error(w, i18n.Localize(localizer, "Error.InternalServer"), http.StatusInternalServerError)
		return
	}

	// If refresh is requested, clear client cache to force fresh data
	if r.URL.Query().Get("refresh") == "1" {
		log.Debug().Msg("Refresh requested, invalidating Proxmox client cache")
		client.InvalidateCache("")
	}

	// Always auto-refresh this page every 5 seconds, navigating to the same VM with refresh=1
	w.Header().Set("Refresh", fmt.Sprintf("5; url=/vm/details/%s?refresh=1", vmID))

	vms, err := proxmox.GetVMsWithContext(r.Context(), client)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch VMs from Proxmox")
		localizer := i18n.GetLocalizer(r)
		http.Error(w, i18n.Localize(localizer, "Proxmox.ConnectionError"), http.StatusBadGateway)
		return
	}

	var found *proxmox.VM
	for i := range vms {
		if strconv.Itoa(vms[i].VMID) == vmID {
			found = &vms[i]
			break
		}
	}
	if found == nil {
		log.Warn().Str("vm_id", vmID).Msg("VM not found")
		localizer := i18n.GetLocalizer(r)
		http.Error(w, i18n.Localize(localizer, "Error.NotFound"), http.StatusNotFound)
		return
	}

	// Format a few fields for the template
	formatBytes := func(b int64) string {
		// Approx formatting: display in GB with one decimal
		if b <= 0 {
			return "0 B"
		}
		const gb = 1024 * 1024 * 1024
		val := float64(b) / float64(gb)
		if val < 1 {
			// show in MB
			const mb = 1024 * 1024
			return strconv.FormatInt(b/int64(mb), 10) + " MB"
		}
		// Round to one decimal place
		s := strconv.FormatFloat(val, 'f', 1, 64)
		return s + " GB"
	}
	formatUptime := func(seconds int64) string {
		if seconds <= 0 {
			return "0s"
		}
		d := time.Duration(seconds) * time.Second
		// Show up to hours/minutes/seconds compactly
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		s := int(d.Seconds()) % 60
		if h > 0 {
			return strconv.Itoa(h) + "h" + strconv.Itoa(m) + "m"
		}
		if m > 0 {
			return strconv.Itoa(m) + "m" + strconv.Itoa(s) + "s"
		}
		return strconv.Itoa(s) + "s"
	}

	// Attempt to fetch VM config to enrich details (description, tags, network bridges)
	var desc string
	var bridgesStr string
	var tagsStr string
	var currentTags []string
	if cfg, err := proxmox.GetVMConfigWithContext(r.Context(), client, found.Node, found.VMID); err != nil {
		log.Debug().Err(err).Msg("VM config fetch failed; proceeding with basic details")
	} else {
		if v, ok := cfg["description"].(string); ok {
			desc = v
		}
		// Proxmox stores tags as semicolon-separated string (e.g., "pvmss;foo;bar").
		if v, ok := cfg["tags"].(string); ok && v != "" {
			// Proxmox tags are typically semicolon-separated, may include spaces or empty segments
			parts := strings.Split(v, ";")
			cleaned := make([]string, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					cleaned = append(cleaned, p)
				}
			}
			if len(cleaned) > 0 {
				tagsStr = strings.Join(cleaned, ", ")
				currentTags = cleaned
			}
		}
		if brs := proxmox.ExtractNetworkBridges(cfg); len(brs) > 0 {
			bridgesStr = strings.Join(brs, ", ")
		}
	}

	// Render description as HTML from Markdown if present
	var descHTML template.HTML
	if desc != "" {
		descHTML = template.HTML(markdown.ToHTML([]byte(desc), nil, nil))
	}

	data := map[string]interface{}{
		"Title":           found.Name,
		"VMID":            vmID,
		"VMName":          found.Name,
		"Status":          found.Status,
		"Uptime":          formatUptime(found.Uptime),
		"Sockets":         1, // unknown here
		"Cores":           found.CPUs,
		"RAM":             formatBytes(found.MaxMem),
		"DiskCount":       0, // not available from this endpoint
		"DiskTotalSize":   formatBytes(found.MaxDisk),
		"NetworkBridges":  bridgesStr,
		"Description":     desc,
		"DescriptionHTML": descHTML,
		"Tags":            tagsStr,
		"CurrentTags":     currentTags,
		"Node":            found.Node,
	}

	// Toggle edit mode via query parameter without JS
	if q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("edit"))); q != "" {
		if q == "description" || q == "desc" {
			data["ShowDescriptionEditor"] = true
		}
		if q == "tags" || q == "tag" {
			data["ShowTagsEditor"] = true
		}
	}

	// Expose available tags from settings for selection UIs
	var availableTags []string
	if settings := h.stateManager.GetSettings(); settings != nil {
		availableTags = settings.Tags
		data["AvailableTags"] = availableTags
	}
	// Build union of available tags and currently set tags so users can uncheck tags not in available list
	if len(currentTags) > 0 || len(availableTags) > 0 {
		set := make(map[string]struct{}, len(currentTags)+len(availableTags))
		for _, t := range availableTags {
			t = strings.TrimSpace(t)
			if t != "" {
				set[t] = struct{}{}
			}
		}
		for _, t := range currentTags {
			t = strings.TrimSpace(t)
			if t != "" {
				set[t] = struct{}{}
			}
		}
		all := make([]string, 0, len(set))
		for k := range set {
			all = append(all, k)
		}
		sort.Strings(all)
		data["AllTags"] = all
	}

	log.Debug().Interface("vm_details", data).Msg("VM details fetched from Proxmox")

	// Add translation data
	i18n.LocalizePage(w, r, data)

	log.Debug().Msg("Rendering template vm_details")
	renderTemplateInternal(w, r, "vm_details", data)
}

// VMActionHandler handles VM lifecycle actions via server-side POST forms
func (h *VMHandler) VMActionHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().Str("handler", "VMActionHandler").Str("method", r.Method).Str("path", r.URL.Path).Logger()
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// Parse posted form values
	if err := r.ParseForm(); err != nil {
		log.Error().Err(err).Msg("failed to parse form")
		localizer := i18n.GetLocalizer(r)
		http.Error(w, i18n.Localize(localizer, "Error.Generic"), http.StatusBadRequest)
		return
	}

	vmid := r.FormValue("vmid")
	node := r.FormValue("node")
	action := r.FormValue("action")
	if vmid == "" || node == "" || action == "" {
		log.Warn().Str("vmid", vmid).Str("node", node).Str("action", action).Msg("missing required fields")
		localizer := i18n.GetLocalizer(r)
		http.Error(w, i18n.Localize(localizer, "Error.Generic"), http.StatusBadRequest)
		return
	}

	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not initialized")
		localizer := i18n.GetLocalizer(r)
		http.Error(w, i18n.Localize(localizer, "Error.InternalServer"), http.StatusInternalServerError)
		return
	}

	log = log.With().Str("vmid", vmid).Str("node", node).Str("action", action).Logger()
	log.Info().Msg("performing VM action")

	// Execute action against Proxmox
	if _, err := proxmox.VMActionWithContext(r.Context(), client, node, vmid, action); err != nil {
		log.Error().Err(err).Msg("VM action failed")
		localizer := i18n.GetLocalizer(r)
		http.Error(w, i18n.Localize(localizer, "Proxmox.ConnectionError"), http.StatusBadGateway)
		return
	}

	// Redirect back to details page for the same VM with refresh=1 to force fresh data
	http.Redirect(w, r, "/vm/details/"+vmid+"?refresh=1", http.StatusSeeOther)
}

// RegisterRoutes registers VM-related routes
func (h *VMHandler) RegisterRoutes(router *httprouter.Router) {
	// VM details (protected by authentication)
	router.GET("/vm/details/:id", h.VMDetailsHandler)
	// VM action endpoint (POST only)
	router.POST("/vm/action", h.VMActionHandler)
	// VM config update endpoints (POST only)
	router.POST("/vm/update/description", h.UpdateVMDescriptionHandler)
	router.POST("/vm/update/tags", h.UpdateVMTagsHandler)
	// VM creation page and submit
	router.GET("/vm/create", h.CreateVMPage)
	router.POST("/api/vm/create", h.CreateVMHandler)
}

// VMDetailsHandlerFunc is a wrapper function for compatibility with existing code
func VMDetailsHandlerFunc(w http.ResponseWriter, r *http.Request) {
	// Backward-compat wrapper: redirect to the canonical route
	vmid := r.URL.Query().Get("vmid")
	if vmid == "" {
		localizer := i18n.GetLocalizer(r)
		http.Error(w, i18n.Localize(localizer, "Error.Generic"), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/vm/details/"+vmid, http.StatusSeeOther)
}
