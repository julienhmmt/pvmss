package handlers

import (
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog/log"

	"pvmss/proxmox"
)

// VMStatusPartialHandler returns just the VM status badge HTML for HTMX polling.
func (h *VMHandler) VMStatusPartialHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	vmidStr := ps.ByName("vmid")
	vmid, err := strconv.Atoi(vmidStr)
	if err != nil {
		http.Error(w, "Invalid VMID", http.StatusBadRequest)
		return
	}

	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create resty client for status check")
		http.Error(w, "Proxmox unavailable", http.StatusServiceUnavailable)
		return
	}

	// Find VM node
	vms, err := proxmox.GetVMsResty(r.Context(), restyClient)
	if err != nil {
		log.Error().Err(err).Int("vmid", vmid).Msg("Failed to get VMs for status check")
		http.Error(w, "Failed to get VM status", http.StatusInternalServerError)
		return
	}

	var node string
	for _, vm := range vms {
		if vm.VMID == vmid {
			node = vm.Node
			break
		}
	}

	if node == "" {
		http.Error(w, "VM not found", http.StatusNotFound)
		return
	}

	// Get current status
	vmStatus, err := proxmox.GetVMCurrentResty(r.Context(), restyClient, node, vmid)
	if err != nil {
		log.Error().Err(err).Int("vmid", vmid).Msg("Failed to get VM current status")
		http.Error(w, "Failed to get VM status", http.StatusInternalServerError)
		return
	}

	status := "stopped"
	if vmStatus != nil {
		status = vmStatus.Status
	}

	// Return minimal HTML for the status badge
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	var class, label string
	switch status {
	case "running":
		class = "is-success"
		label = "Running"
	case "stopped":
		class = "is-danger"
		label = "Stopped"
	case "paused":
		class = "is-warning"
		label = "Paused"
	default:
		class = "is-light"
		label = status
	}

	// Return just the badge HTML - HTMX will swap this in
	html := `<span class="tag ` + class + `" id="vm-status-badge" data-status="` + status + `" hx-get="/api/vm/` + vmidStr + `/status" hx-trigger="every 5s" hx-swap="outerHTML">` + label + `</span>`
	_, _ = w.Write([]byte(html))
}
