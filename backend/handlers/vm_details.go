package handlers

import (
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
)

// VMStateManager defines the minimal state contract needed by VM details.
type VMStateManager interface {
	GetProxmoxClient() proxmox.ClientInterface
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

	data := map[string]interface{}{
		"Title":          "VM details " + vmID,
		"VMID":           vmID,
		"VMName":         found.Name,
		"Status":         found.Status,
		"Uptime":         found.Uptime, // seconds; template shows raw value for now
		"Sockets":        1,            // unknown here
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

// RegisterRoutes registers VM-related routes
func (h *VMHandler) RegisterRoutes(router *httprouter.Router) {
	// VM details (protected by authentication)
	router.GET("/vm/details/:id", h.VMDetailsHandler)
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
