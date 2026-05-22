package apiv1

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/logger"
	"pvmss/proxmox"
)

// ToggleNIC handles POST /api/v1/vms/:id/network/:iface/toggle.
// Toggles the link_down flag on the specified network interface (net0..net31),
// effectively enabling or disabling the NIC without removing it from the config.
func (h *VMDetailsHandler) ToggleNIC(w http.ResponseWriter, r *http.Request) {
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}
	ps := httprouter.ParamsFromContext(r.Context())
	iface := strings.ToLower(strings.TrimSpace(ps.ByName("iface")))
	if matched, _ := regexp.MatchString(`^net\d+$`, iface); !matched {
		errBadRequest(w, "invalid network interface name; expected net0..net31")
		return
	}

	if h.isOffline() {
		errOffline(w)
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	client, err := restyClient()
	if err != nil {
		writeAppError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Resolve node from VM list and verify pvmss tag.
	allVMs, err := proxmox.GetVMsResty(ctx, client)
	if err != nil {
		writeAppError(w, err)
		return
	}

	var targetNode string
	for _, vm := range allVMs {
		if vm.VMID == vmid && hasTag(vm.Tags, "pvmss") {
			targetNode = vm.Node
			break
		}
	}
	if targetNode == "" {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	// Ownership check: pool membership (or admin).
	if !ownsVM(ctx, client, username, isAdmin, vmid) {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	cfg, err := proxmox.GetVMConfigResty(ctx, client, targetNode, vmid)
	if err != nil {
		writeAppError(w, err)
		return
	}

	raw, ok := cfg[iface].(string)
	if !ok || raw == "" {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("network interface %s not found", iface))
		return
	}

	// Toggle link_down: present → remove; absent → add.
	parts := strings.Split(raw, ",")
	var wasDown bool
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "link_down=1" {
			wasDown = true
		} else {
			filtered = append(filtered, p)
		}
	}

	var newRaw string
	if wasDown {
		newRaw = strings.Join(filtered, ",")
	} else {
		newRaw = raw + ",link_down=1"
	}

	if err := proxmox.UpdateVMConfigResty(ctx, client, targetNode, vmid, map[string]string{iface: newRaw}); err != nil {
		logger.Get().Error().Err(err).Int("vmid", vmid).Str("iface", iface).Msg("api/v1: failed to toggle NIC")
		writeError(w, http.StatusBadGateway, "proxmox_error", "Failed to toggle network interface")
		return
	}

	nowDown := !wasDown
	logger.VMEvent("api_vm_nic_toggle", vmid, targetNode).
		Str("username", username).Str("iface", iface).Bool("link_down", nowDown).
		Msg("NIC toggled")

	writeJSON(w, map[string]any{"success": true, "link_down": nowDown})
}
