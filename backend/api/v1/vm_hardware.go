package apiv1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/utils"
)

// NetworkUpdateRequest describes a network card to set or update.
// Index is the key (e.g. "net0", "net1"). An empty Index means add a new card.
type NetworkUpdateRequest struct {
	Index    string `json:"index"`    // "net0" … "net9"; empty = new card
	Model    string `json:"model"`    // virtio, e1000, e1000e, rtl8139, vmxnet3
	Bridge   string `json:"bridge"`   // e.g. "vmbr0"
	MAC      string `json:"mac"`      // optional; Proxmox auto-assigns if empty
	VLAN     int    `json:"vlan"`     // 0 = none
	Rate     string `json:"rate"`     // MB/s, e.g. "10" or "" for unlimited
	Firewall bool   `json:"firewall"` // enable firewall
}

// VMHardwareUpdateRequest is the body for PUT /api/v1/vms/:id/hardware.
type VMHardwareUpdateRequest struct {
	Node           string                 `json:"node"`
	Sockets        int                    `json:"sockets"`
	Cores          int                    `json:"cores"`
	MemoryMB       int                    `json:"memory_mb"`
	Tags           *string                `json:"tags,omitempty"`
	Networks       []NetworkUpdateRequest `json:"networks,omitempty"`
	DeleteNetworks []string               `json:"delete_networks,omitempty"` // e.g. ["net1","net2"]
}

// VMHardwareUpdateResponse is the response for a successful hardware update.
type VMHardwareUpdateResponse struct {
	Success   bool   `json:"success"`
	Restarted bool   `json:"restarted"`
	Message   string `json:"message"`
}

// validNetModels is the set of accepted NIC model identifiers.
var validNetModels = map[string]bool{
	"virtio": true, "e1000": true, "e1000e": true,
	"rtl8139": true, "vmxnet3": true,
}

// buildNetLine serialises a NetworkUpdateRequest into a Proxmox net* config string.
// Preserves the existing MAC when provided; Proxmox auto-generates one otherwise.
func buildNetLine(n NetworkUpdateRequest) string {
	model := strings.ToLower(strings.TrimSpace(n.Model))
	if !validNetModels[model] {
		model = "virtio"
	}
	var sb strings.Builder
	mac := strings.TrimSpace(n.MAC)
	if mac != "" {
		sb.WriteString(model)
		sb.WriteByte('=')
		sb.WriteString(strings.ToUpper(mac))
	} else {
		sb.WriteString(model)
	}
	sb.WriteString(",bridge=")
	sb.WriteString(strings.TrimSpace(n.Bridge))
	if n.VLAN > 0 {
		sb.WriteString(",tag=")
		sb.WriteString(strconv.Itoa(n.VLAN))
	}
	rate := strings.TrimSpace(n.Rate)
	if rate != "" && rate != "0" {
		sb.WriteString(",rate=")
		sb.WriteString(rate)
	}
	if n.Firewall {
		sb.WriteString(",firewall=1")
	}
	return sb.String()
}

// UpdateVMHardware handles PUT /api/v1/vms/:id/hardware.
// Atomically updates CPU, RAM, tags, and network cards.
// The VM is stopped only when hardware or network changes require it;
// tags are always updated live (Proxmox accepts tag changes while running).
func (h *VMDetailsHandler) UpdateVMHardware(w http.ResponseWriter, r *http.Request) {
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

	var req VMHardwareUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid JSON body")
		return
	}
	if req.Node == "" {
		errBadRequest(w, "node is required")
		return
	}

	settings := h.state.GetSettings()
	if settings == nil {
		errInternal(w)
		return
	}
	limits := settings.Limits.VM

	if req.Sockets < limits.Sockets.Min || req.Sockets > limits.Sockets.Max {
		errBadRequest(w, fmt.Sprintf("sockets must be between %d and %d", limits.Sockets.Min, limits.Sockets.Max))
		return
	}
	if req.Cores < limits.Cores.Min || req.Cores > limits.Cores.Max {
		errBadRequest(w, fmt.Sprintf("cores must be between %d and %d", limits.Cores.Min, limits.Cores.Max))
		return
	}
	minMemMB := limits.RAM.Min * 1024
	maxMemMB := limits.RAM.Max * 1024
	if req.MemoryMB < minMemMB || req.MemoryMB > maxMemMB {
		errBadRequest(w, fmt.Sprintf("memory_mb must be between %d and %d", minMemMB, maxMemMB))
		return
	}

	// Validate requested network card count
	maxNICs := settings.MaxNetworkCards
	if maxNICs <= 0 {
		maxNICs = 1
	}
	// Validate each network card request
	for i, n := range req.Networks {
		if n.Bridge == "" {
			errBadRequest(w, fmt.Sprintf("network[%d]: bridge is required", i))
			return
		}
		if n.MAC != "" && !utils.ValidateMACAddress(n.MAC) {
			errBadRequest(w, fmt.Sprintf("network[%d]: invalid MAC address", i))
			return
		}
		if n.VLAN != 0 && (n.VLAN < 1 || n.VLAN > 4096) {
			errBadRequest(w, fmt.Sprintf("network[%d]: VLAN tag must be between 1 and 4096", i))
			return
		}
		if n.Rate != "" {
			rate := strings.TrimSpace(n.Rate)
			rateLimit, err := strconv.ParseFloat(rate, 64)
			if err != nil || rateLimit < 0 {
				errBadRequest(w, fmt.Sprintf("network[%d]: rate limit must be a non-negative number", i))
				return
			}
		}
	}

	// Validate tags: pvmss must be present
	if req.Tags != nil {
		tags := *req.Tags
		hasPvmss := false
		for _, t := range strings.Split(tags, ";") {
			if strings.TrimSpace(strings.ToLower(t)) == "pvmss" {
				hasPvmss = true
				break
			}
		}
		if !hasPvmss {
			errBadRequest(w, "tags must include 'pvmss'")
			return
		}
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	client, err := restyClient()
	if err != nil {
		errInternal(w)
		return
	}

	// Worst-case: 90s graceful stop + 30s force-stop + config update + 30s start + network round-trips.
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	// Validate network card count properly: existing + new - deleted
	if len(req.Networks)+len(req.DeleteNetworks) > 0 {
		existingCfg, cfgErr := proxmox.GetVMConfigResty(ctx, client, req.Node, vmid)
		if cfgErr != nil {
			errInternal(w)
			return
		}
		// Count existing NICs (net0, net1, etc.)
		existingNICCount := 0
		for i := 0; i < 32; i++ {
			key := fmt.Sprintf("net%d", i)
			if _, exists := existingCfg[key]; exists {
				existingNICCount++
			}
		}
		// Count new NICs (those with empty Index)
		newNICCount := 0
		for _, n := range req.Networks {
			if n.Index == "" {
				newNICCount++
			}
		}
		// Calculate final NIC count: existing + new - deleted
		finalNICCount := existingNICCount + newNICCount - len(req.DeleteNetworks)
		if finalNICCount > maxNICs {
			errBadRequest(w, fmt.Sprintf("too many network cards: max %d (would have %d after operation)", maxNICs, finalNICCount))
			return
		}
	}

	// Ownership check: pool membership (or admin) AND pvmss tag.
	if !ownsVM(ctx, client, username, isAdmin, vmid) {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}
	if !isAdmin {
		allVMs, listErr := proxmox.GetVMsResty(ctx, client)
		if listErr != nil {
			errInternal(w)
			return
		}
		tagged := false
		for _, vm := range allVMs {
			if vm.VMID == vmid {
				tagged = hasTag(vm.Tags, "pvmss")
				break
			}
		}
		if !tagged {
			writeError(w, http.StatusNotFound, "not_found", "VM not found")
			return
		}
	}

	vmidStr := strconv.Itoa(vmid)

	// Get current VM status
	current, err := proxmox.GetVMCurrentResty(ctx, client, req.Node, vmid)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "VM not found on specified node")
		return
	}

	wasRunning := current.Status == "running"

	// Get full config to detect actual hardware changes.
	cfgForDiff, cfgDiffErr := proxmox.GetVMConfigResty(ctx, client, req.Node, vmid)

	// ── 1. Determine whether a stop is needed ──────────────────────────────────
	hardwareChanged := true // default: assume changed when config unreadable
	if cfgDiffErr == nil {
		currentSockets := 1
		if v, ok := cfgForDiff["sockets"].(float64); ok {
			currentSockets = int(v)
		}
		currentCores := 1
		if v, ok := cfgForDiff["cores"].(float64); ok {
			currentCores = int(v)
		}
		currentMemMB := int(current.MaxMem / (1024 * 1024))
		hardwareChanged = req.Sockets != currentSockets ||
			req.Cores != currentCores ||
			req.MemoryMB != currentMemMB
	}
	networkChanged := len(req.Networks) > 0 || len(req.DeleteNetworks) > 0
	needsStop := wasRunning && (hardwareChanged || networkChanged)

	// ── 2. Stop VM if required ─────────────────────────────────────────────────
	if needsStop {
		// Re-check to avoid race condition
		current, err = proxmox.GetVMCurrentResty(ctx, client, req.Node, vmid)
		if err != nil {
			errInternal(w)
			return
		}
		if current.Status != "running" {
			wasRunning = false
			needsStop = false
		}
	}

	if needsStop {
		// Prefer graceful shutdown when qemu-agent is configured.
		// Reuse the config we already fetched above.
		useShutdown := false
		if cfgDiffErr == nil {
			switch v := cfgForDiff["agent"].(type) {
			case string:
				useShutdown = v != "" && v != "0"
			case float64:
				useShutdown = v != 0
			}
		}

		action := "stop"
		if useShutdown {
			action = "shutdown"
		}

		logger.VMEvent("api_vm_hardware_update", vmid, req.Node).
			Str("username", username).Str("action", action).
			Msg("Stopping VM before hardware update")

		if _, err := proxmox.VMActionResty(ctx, client, req.Node, vmidStr, action); err != nil {
			logger.Get().Error().Err(err).Int("vmid", vmid).Str("node", req.Node).Str("action", action).Msg("api/v1: failed to stop VM for hardware update")
			writeError(w, http.StatusBadGateway, "proxmox_error", "Failed to stop VM")
			return
		}

		if err := waitForVMStopped(ctx, client, req.Node, vmid, 90*time.Second); err != nil {
			// Force stop as fallback
			logger.Get().Warn().Int("vmid", vmid).Msg("vm_hardware: graceful stop timed out, forcing stop")
			if _, ferr := proxmox.VMActionResty(ctx, client, req.Node, vmidStr, "stop"); ferr != nil {
				writeError(w, http.StatusBadGateway, "proxmox_error", "VM could not be stopped")
				return
			}
			if err2 := waitForVMStopped(ctx, client, req.Node, vmid, 30*time.Second); err2 != nil {
				writeError(w, http.StatusBadGateway, "proxmox_error", "VM could not be stopped")
				return
			}
		}
	}

	// ── 3. Update tags live (no stop required) ─────────────────────────────────
	if req.Tags != nil {
		liveParams := map[string]string{"tags": *req.Tags}
		if err := proxmox.UpdateVMConfigResty(ctx, client, req.Node, vmid, liveParams); err != nil {
			logger.Get().Error().Err(err).Int("vmid", vmid).Msg("api/v1: failed to update tags")
			writeError(w, http.StatusBadGateway, "proxmox_error", "Failed to update tags")
			return
		}
	}

	// ── 4. Apply hardware + network changes ────────────────────────────────────
	params := map[string]string{
		"sockets": strconv.Itoa(req.Sockets),
		"cores":   strconv.Itoa(req.Cores),
		"memory":  strconv.Itoa(req.MemoryMB),
	}

	// Build delete set first so we can guard against update/delete overlap.
	deleteSet := make(map[string]bool, len(req.DeleteNetworks))
	deleteKeys := make([]string, 0, len(req.DeleteNetworks))
	for _, key := range req.DeleteNetworks {
		key = strings.TrimSpace(strings.ToLower(key))
		if strings.HasPrefix(key, "net") {
			deleteSet[key] = true
			deleteKeys = append(deleteKeys, key)
		}
	}

	// Fetch current config once for new-NIC index allocation.
	existingCfg, err := proxmox.GetVMConfigResty(ctx, client, req.Node, vmid)
	if err != nil {
		errInternal(w)
		return
	}
	assignedIdxs := make(map[string]bool) // track indexes assigned in this request

	// Assign net* keys for created/updated NICs.
	for _, n := range req.Networks {
		idx := strings.TrimSpace(n.Index)
		if idx == "" {
			// Find first slot unused by Proxmox and not already assigned in this request.
			for i := 0; i < 32; i++ {
				key := fmt.Sprintf("net%d", i)
				if _, exists := existingCfg[key]; !exists && !assignedIdxs[key] {
					idx = key
					break
				}
			}
			if idx == "" {
				writeError(w, http.StatusConflict, "limit_reached", "no free network slot available")
				return
			}
		}
		// Guard: an index cannot be both updated and deleted in the same request.
		if deleteSet[strings.ToLower(idx)] {
			errBadRequest(w, fmt.Sprintf("network index %s appears in both networks and delete_networks", idx))
			return
		}
		assignedIdxs[idx] = true
		params[idx] = buildNetLine(n)
	}

	if len(deleteKeys) > 0 {
		params["delete"] = strings.Join(deleteKeys, ",")
	}

	logger.VMEvent("api_vm_hardware_update", vmid, req.Node).
		Str("username", username).
		Int("sockets", req.Sockets).Int("cores", req.Cores).Int("memory_mb", req.MemoryMB).
		Int("net_updates", len(req.Networks)).Int("net_deletes", len(req.DeleteNetworks)).
		Msg("Applying hardware configuration")

	if err := proxmox.UpdateVMConfigResty(ctx, client, req.Node, vmid, params); err != nil {
		logger.Get().Error().Err(err).Int("vmid", vmid).Msg("vm_hardware: failed to update config")
		if wasRunning && needsStop {
			if _, rerr := proxmox.VMActionResty(ctx, client, req.Node, vmidStr, "start"); rerr != nil {
				logger.Get().Error().Err(rerr).Int("vmid", vmid).Msg("vm_hardware: failed to restart VM after failed config")
			}
		}
		writeError(w, http.StatusBadGateway, "proxmox_error", "Failed to update VM config")
		return
	}

	// ── 5. Restart VM if it was running ────────────────────────────────────────
	if wasRunning && needsStop {
		logger.VMEvent("api_vm_hardware_update", vmid, req.Node).
			Str("username", username).Msg("Restarting VM after hardware update")

		if _, err := proxmox.VMActionResty(ctx, client, req.Node, vmidStr, "start"); err != nil {
			logger.Get().Error().Err(err).Int("vmid", vmid).Str("node", req.Node).Msg("api/v1: failed to restart VM after hardware update")
			writeError(w, http.StatusBadGateway, "proxmox_error", "Failed to restart VM")
			return
		}
		if err := waitForVMStarted(ctx, client, req.Node, vmid, 30*time.Second); err != nil {
			logger.Get().Warn().Err(err).Int("vmid", vmid).Msg("vm_hardware: VM did not start in time")
		}
	}

	writeJSON(w, VMHardwareUpdateResponse{
		Success:   true,
		Restarted: wasRunning && needsStop,
		Message:   "Hardware updated successfully",
	})
}

// pollVMStatus polls Proxmox until the VM reaches the desired status or the
// deadline is reached. Errors from GetVMCurrentResty are treated as transient
// and backed off for the same 2-second interval to avoid hammering the API.
func pollVMStatus(ctx context.Context, client *proxmox.RestyClient, node string, vmid int, wantStatus string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
		current, err := proxmox.GetVMCurrentResty(ctx, client, node, vmid)
		if err == nil && current.Status == wantStatus {
			return nil
		}
	}
	return fmt.Errorf("VM %d on node %s did not reach status %q within %s", vmid, node, wantStatus, timeout)
}

// waitForVMStopped polls until the VM is stopped or the deadline is reached.
func waitForVMStopped(ctx context.Context, client *proxmox.RestyClient, node string, vmid int, timeout time.Duration) error {
	return pollVMStatus(ctx, client, node, vmid, "stopped", timeout)
}

// waitForVMStarted polls until the VM is running or the deadline is reached.
func waitForVMStarted(ctx context.Context, client *proxmox.RestyClient, node string, vmid int, timeout time.Duration) error {
	return pollVMStatus(ctx, client, node, vmid, "running", timeout)
}
