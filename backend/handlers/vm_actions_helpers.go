package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"pvmss/constants"
	"pvmss/proxmox"
)

// buildVMDetailsURL builds the VM details URL with a cache-busting timestamp.
func buildVMDetailsURL(vmid string) string {
	return fmt.Sprintf("/vm/details/%s?refresh=1&ts=%d", vmid, time.Now().Unix())
}

// agentStatus represents the QEMU guest agent availability status.
type agentStatus int

const (
	agentStatusUnknown agentStatus = iota
	agentStatusAvailable
	agentStatusUnavailable
)

// getGuestAgentStatus checks the guest agent availability for a VM.
func getGuestAgentStatus(r *http.Request, node string, vmid int) agentStatus {
	log := CreateHandlerLogger("GuestAgentStatus", r)
	start := time.Now()

	stateManager := getStateManager(r)
	if stateManager == nil {
		log.Error().
			Str("operation", "guest_agent_health_check").
			Str("node", node).
			Int("vmid", vmid).
			Str("result", "unknown").
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("Guest agent status check failed: state manager not available")
		return agentStatusUnknown
	}
	if stateManager.IsOfflineMode() {
		log.Info().
			Str("operation", "guest_agent_health_check").
			Str("node", node).
			Int("vmid", vmid).
			Str("result", "unknown").
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("Guest agent status: unknown (offline mode)")
		return agentStatusUnknown
	}

	if isGuestAgentUnavailableCached(node, vmid) {
		log.Debug().
			Str("operation", "guest_agent_health_check").
			Str("node", node).
			Int("vmid", vmid).
			Str("result", "unavailable").
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("Guest agent status: unavailable (cached)")
		return agentStatusUnavailable
	}

	client := stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().
			Str("operation", "guest_agent_health_check").
			Str("node", node).
			Int("vmid", vmid).
			Str("result", "unknown").
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("Guest agent status: unknown (Proxmox client not available)")
		return agentStatusUnknown
	}

	timeout := constants.GuestAgentTimeout
	if timeout <= 0 {
		timeout = time.Second
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	interfaces, err := proxmox.GetGuestAgentNetworkInterfaces(ctx, client, node, vmid)
	if err != nil || len(interfaces) == 0 {
		cacheGuestAgentUnavailable(node, vmid)
		log.Warn().
			Str("operation", "guest_agent_health_check").
			Str("node", node).
			Int("vmid", vmid).
			Str("result", "unavailable").
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Err(err).
			Msg("Guest agent status: unavailable (Proxmox agent call failed or no interfaces returned)")
		return agentStatusUnavailable
	}

	cacheGuestAgentIPs(node, vmid, interfaces)
	log.Info().
		Str("operation", "guest_agent_health_check").
		Str("node", node).
		Int("vmid", vmid).
		Str("result", "available").
		Int("interface_count", len(interfaces)).
		Int64("duration_ms", time.Since(start).Milliseconds()).
		Msg("Guest agent status: available")
	return agentStatusAvailable
}
