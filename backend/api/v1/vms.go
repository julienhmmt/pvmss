package apiv1

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// VMHandler handles VM listing and detail endpoints.
type VMHandler struct {
	state state.StateManager
}

// MakeVMHandler creates a new VMHandler.
func MakeVMHandler(s state.StateManager) *VMHandler {
	return &VMHandler{state: s}
}

// isOffline returns true when PVMSS_OFFLINE=true or state signals offline.
func (h *VMHandler) isOffline() bool {
	return strings.EqualFold(os.Getenv("PVMSS_OFFLINE"), "true") ||
		h.state == nil || h.state.IsOfflineMode()
}

// restyClient returns a fresh Resty client from env vars with a 30s timeout.
func restyClient() (*proxmox.RestyClient, error) {
	return proxmox.MakeRestyClientFromEnv(30 * time.Second)
}

// ListVMs handles GET /api/v1/vms.
func (h *VMHandler) ListVMs(w http.ResponseWriter, r *http.Request) {
	if h.isOffline() {
		writeJSON(w, VMListResponse{VMs: []VMSummary{}, Total: 0})
		return
	}

	client, err := restyClient()
	if err != nil {
		logger.Get().Error().Err(err).Msg("api/v1: failed to create resty client for ListVMs")
		errInternal(w)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	vms, err := proxmox.GetVMsResty(ctx, client)
	if err != nil {
		logger.Get().Error().Err(err).Msg("api/v1: GetVMsResty failed")
		errInternal(w)
		return
	}

	summaries := make([]VMSummary, 0, len(vms))
	for _, vm := range vms {
		summaries = append(summaries, vmToSummary(vm))
	}
	writeJSON(w, VMListResponse{VMs: summaries, Total: len(summaries)})
}

// GetVM handles GET /api/v1/vms/:id.
func (h *VMHandler) GetVM(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	vmid, err := strconv.Atoi(ps.ByName("id"))
	if err != nil || vmid <= 0 {
		errBadRequest(w, "invalid vm id")
		return
	}

	if h.isOffline() {
		errOffline(w)
		return
	}

	client, err := restyClient()
	if err != nil {
		errInternal(w)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	vms, err := proxmox.GetVMsResty(ctx, client)
	if err != nil {
		errInternal(w)
		return
	}
	for _, vm := range vms {
		if vm.VMID == vmid {
			writeJSON(w, vmToSummary(vm))
			return
		}
	}
	writeError(w, http.StatusNotFound, "not_found", "VM not found")
}

// vmToSummary converts a proxmox.VM to the API VMSummary.
// Proxmox reports Mem/MaxMem/MaxDisk in bytes; we convert to MB.
func vmToSummary(vm proxmox.VM) VMSummary {
	const bytesPerMB = int64(1024 * 1024)
	return VMSummary{
		VMID:     vm.VMID,
		Name:     vm.Name,
		Node:     vm.Node,
		Status:   vm.Status,
		CPU:      vm.CPU,
		CPUs:     vm.CPUs,
		MemMB:    vm.Mem / bytesPerMB,
		MaxMemMB: vm.MaxMem / bytesPerMB,
		DiskMB:   vm.MaxDisk / bytesPerMB,
		Uptime:   vm.Uptime,
		Tags:     vm.Tags,
	}
}
