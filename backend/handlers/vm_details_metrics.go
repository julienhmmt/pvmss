package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"pvmss/i18n"
	"pvmss/proxmox"

	"github.com/julienschmidt/httprouter"
)

// VMMetricsHandler returns VM metrics as JSON for dynamic updates.
func (h *VMHandler) VMMetricsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := CreateHandlerLogger("VMMetricsHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodGet) {
		return
	}

	vmid := ps.ByName("vmid")
	if vmid == "" {
		log.Error().Msg("VM ID is required")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.MissingRequiredFields"), http.StatusBadRequest)
		return
	}

	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		log.Error().Err(err).Str("vmid", vmid).Msg("Invalid VM ID")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.Generic"), http.StatusBadRequest)
		return
	}

	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not available")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.ProxmoxClientUnavailable"), http.StatusServiceUnavailable)
		return
	}

	restyClient, err := proxmox.NewRestyClientFromEnv(30 * time.Second)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create resty client")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.ServerConfigError"), http.StatusInternalServerError)
		return
	}

	node := ""
	vms, err := proxmox.GetVMsResty(r.Context(), restyClient)
	if err == nil {
		for _, vm := range vms {
			if vm.VMID == vmidInt {
				node = vm.Node
				break
			}
		}
	}

	if node == "" {
		if nodes, err := proxmox.GetNodeNamesResty(r.Context(), restyClient); err == nil {
			for _, n := range nodes {
				if status, err := proxmox.GetVMCurrentResty(r.Context(), restyClient, n, vmidInt); err == nil && status != nil {
					node = n
					break
				}
			}
		}
	}

	if node == "" {
		log.Error().Int("vmid", vmidInt).Msg("VM not found on any node")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.NotFound"), http.StatusNotFound)
		return
	}

	vmCurrent, err := proxmox.GetVMCurrentResty(r.Context(), restyClient, node, vmidInt)
	if err != nil {
		log.Error().Err(err).Int("vmid", vmidInt).Str("node", node).Msg("Failed to get VM metrics")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.FailedToGetResources"), http.StatusInternalServerError)
		return
	}

	metrics := map[string]interface{}{
		"status": vmCurrent.Status,
		"cpu":    vmCurrent.CPU,
		"mem":    vmCurrent.Mem,
		"maxmem": vmCurrent.MaxMem,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		log.Error().Err(err).Msg("Failed to encode metrics response")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.InternalServer"), http.StatusInternalServerError)
		return
	}

	log.Debug().
		Int("vmid", vmidInt).
		Str("status", vmCurrent.Status).
		Float64("cpu", vmCurrent.CPU).
		Int64("mem", vmCurrent.Mem).
		Str("component", "vm_details").
		Str("operation", "serve_metrics").
		Str("reason", "metrics_served").
		Msg("VM metrics served")
}
