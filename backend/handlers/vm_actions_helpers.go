package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
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

// extractProxmoxTaskError returns a user-friendly task error message when available.
func extractProxmoxTaskError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return ""
	}

	lower := strings.ToLower(msg)
	needle := "task error:"
	if idx := strings.Index(lower, needle); idx >= 0 {
		extracted := strings.TrimSpace(msg[idx+len(needle):])
		if extracted != "" {
			return extracted
		}
	}

	// Fall back to the full error message without truncation.
	return msg
}

// checkTaskFailure polls a Proxmox task and returns exit status when it fails.
func checkTaskFailure(ctx context.Context, restyClient *proxmox.RestyClient, node string, upid string) (string, bool) {
	if restyClient == nil || upid == "" || node == "" {
		return "", false
	}
	for i := 0; i < 5; i++ {
		status, err := proxmox.GetTaskStatusResty(ctx, restyClient, node, upid)
		if err == nil && status != nil {
			if exit := strings.TrimSpace(status.ExitStatus); exit != "" {
				if strings.ToUpper(exit) != "OK" {
					return exit, true
				}
				return "", false
			}
		}
		time.Sleep(1 * time.Second)
	}
	return "", false
}

// formatProxmoxDetail prefixes the message to clearly indicate its origin.
func formatProxmoxDetail(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}
	return "Proxmox: " + msg
}
