package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

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

	data := map[string]interface{}{
		"Title":          found.Name,
		"VMID":           vmID,
		"VMName":         found.Name,
		"Status":         found.Status,
		"Uptime":         formatUptime(found.Uptime),
		"Sockets":        1, // unknown here
		"Cores":          found.CPUs,
		"RAM":            formatBytes(found.MaxMem),
		"DiskCount":      0, // not available from this endpoint
		"DiskTotalSize":  formatBytes(found.MaxDisk),
		"NetworkBridges": "", // not available here
		"Description":    "",
		"Node":           found.Node,
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
