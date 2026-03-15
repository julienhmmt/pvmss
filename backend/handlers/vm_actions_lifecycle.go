package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/constants"
	"pvmss/logger"
	"pvmss/proxmox"
)

// VMActionHandler handles VM lifecycle actions via server-side POST forms.
func (h *VMHandler) VMActionHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("VMActionHandler", r)
	start := time.Now()
	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}
	vmid := r.FormValue("vmid")
	node := r.FormValue("node")
	action := r.FormValue("action")
	if vmid == "" || node == "" || action == "" {
		log.Warn().Str("component", "vm_actions").Str("operation", "validate_action_request").Str("reason", "missing_fields").Str("vmid", vmid).Str("node", node).Str("action", action).Msg("Missing required fields for VM action")
		RespondWithError(w, r, ErrBadRequest)
		return
	}
	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		log.Error().Err(err).Str("vmid", vmid).Msg("invalid VM ID")
		RespondWithError(w, r, ErrBadRequest)
		return
	}
	stateManager := getStateManager(r)
	if stateManager == nil {
		log.Error().Msg("state manager not available")
		RespondWithError(w, r, ErrInternalServer)
		return
	}
	if stateManager.IsOfflineMode() {
		if action == "shutdown" {
			log.Warn().Str("action", action).Str("node", node).Int("vmid", vmidInt).Str("result", "guest_agent_offline").Int64("duration_ms", time.Since(start).Milliseconds()).Msg("Shutdown aborted: Proxmox is offline or PVMSS offline mode active")
			ctx := HandlerContextWith(w, r, "VMActionHandler")
			ctx.RedirectWithError(buildVMDetailsURL(vmid), "VMDetails.QemuGuestAgentOffline")
			return
		}
		log.Error().Str("action", action).Str("node", node).Int("vmid", vmidInt).Str("result", "proxmox_offline").Int64("duration_ms", time.Since(start).Milliseconds()).Msg("Proxmox is offline, VM action not available")
		RespondWithError(w, r, ErrProxmoxConnection)
		return
	}
	if action == "shutdown" {
		status := getGuestAgentStatus(r, node, vmidInt)
		if status == agentStatusUnavailable {
			log.Info().Str("action", action).Str("node", node).Int("vmid", vmidInt).Str("result", "guest_agent_unavailable_precheck").Int64("duration_ms", time.Since(start).Milliseconds()).Msg("Guest agent unavailable before shutdown, aborting graceful shutdown")
			ctx := HandlerContextWith(w, r, "VMActionHandler")
			ctx.RedirectWithError(buildVMDetailsURL(vmid), "VMDetails.QemuGuestAgentTimeout")
			return
		}
		log.Debug().Str("action", action).Str("node", node).Int("vmid", vmidInt).Str("result", "guest_agent_precheck_ok").Int64("duration_ms", time.Since(start).Milliseconds()).Msg("Guest agent precheck passed, proceeding with shutdown")
	}
	log.Info().Str("action", action).Int("vmid", vmidInt).Msg("executing VM action")
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create resty client")
		ctx := HandlerContextWith(w, r, "VMActionHandler")
		ctx.RedirectWithError("/vm/details/"+vmid, "Error.InternalServer")
		return
	}
	username := ""
	if sessionManager := getStateManager(r).GetSessionManager(); sessionManager != nil {
		if user, ok := sessionManager.Get(r.Context(), "username").(string); ok {
			username = user
		}
	}
	upid, err := proxmox.VMActionResty(r.Context(), restyClient, node, vmid, action)
	if err != nil {
		logger.VMFailure("vm_action", vmidInt, node, "proxmox_api_error").Err(err).Str("action", action).Str("username", username).Str("client_ip", r.RemoteAddr).Int64("duration_ms", time.Since(start).Milliseconds()).Msg("VM action failed")
		if action == "shutdown" && strings.Contains(strings.ToLower(err.Error()), "guest-ping") && (strings.Contains(strings.ToLower(err.Error()), "timeout") || strings.Contains(strings.ToLower(err.Error()), "failed")) {
			ctx := HandlerContextWith(w, r, "VMActionHandler")
			ctx.RedirectWithError(buildVMDetailsURL(vmid), "VMDetails.QemuGuestAgentTimeout")
			return
		}
		ctx := HandlerContextWith(w, r, "VMActionHandler")
		userMsg := ctx.Translate("Message.ActionFailed")
		if detail := formatProxmoxDetail(extractProxmoxTaskError(err)); detail != "" {
			userMsg = userMsg + ": " + detail
		}
		ctx.RedirectWithParams(buildVMDetailsURL(vmid), map[string]string{
			"warning":     "1",
			"warning_msg": userMsg,
		})
		return
	}
	// If Proxmox accepted the action but returns a UPID, check task exit status for user-facing errors.
	if upid != "" {
		if exitStatus, failed := checkTaskFailure(r.Context(), restyClient, node, upid); failed {
			ctx := HandlerContextWith(w, r, "VMActionHandler")
			userMsg := ctx.Translate("Message.ActionFailed")
			if exitStatus != "" {
				userMsg = userMsg + ": " + formatProxmoxDetail(exitStatus)
			}
			ctx.RedirectWithParams(buildVMDetailsURL(vmid), map[string]string{
				"warning":     "1",
				"warning_msg": userMsg,
			})
			return
		}
	}
	if action == "shutdown" {
		log.Info().Int("vmid", vmidInt).Msg("Waiting for VM to shutdown after guest agent request")
		vmStopped := false
		for i := 0; i < constants.GuestAgentShutdownMaxAttempts; i++ {
			if r.Context().Err() != nil {
				log.Warn().Str("component", "vm_actions").Str("operation", "shutdown_polling").Str("reason", "context_cancelled").Int("vmid", vmidInt).Msg("Shutdown polling cancelled by request context")
				break
			}
			if i > 0 {
				time.Sleep(constants.GuestAgentShutdownPollInterval)
			}
			currentStatus, statusErr := proxmox.GetVMCurrentResty(r.Context(), restyClient, node, vmidInt)
			if statusErr != nil {
				log.Warn().Err(statusErr).Str("component", "vm_actions").Str("operation", "shutdown_polling").Str("reason", "status_check_failed").Int("vmid", vmidInt).Int("attempt", i+1).Msg("Failed to get VM status during shutdown polling")
				continue
			}
			if currentStatus != nil && currentStatus.Status != "running" {
				vmStopped = true
				log.Info().Int("vmid", vmidInt).Int("attempt", i+1).Str("status", currentStatus.Status).Msg("VM stopped after guest agent shutdown")
				break
			}
			log.Debug().Int("vmid", vmidInt).Int("attempt", i+1).Msg("VM still running after guest agent shutdown, continuing to poll")
		}
		if !vmStopped && r.Context().Err() == nil {
			log.Warn().Str("action", action).Str("node", node).Int("vmid", vmidInt).Str("result", "guest_agent_shutdown_slow").Int64("duration_ms", time.Since(start).Milliseconds()).Msg("Guest agent shutdown did not complete within expected time window")
			ctx := HandlerContextWith(w, r, "VMActionHandler")
			ctx.RedirectWithError(buildVMDetailsURL(vmid), "VMDetails.QemuGuestAgentShutdownSlow")
			return
		}
	}
	logger.VMEvent("vm_action", vmidInt, node).Str("action", action).Str("username", username).Str("client_ip", r.RemoteAddr).Int64("duration_ms", time.Since(start).Milliseconds()).Msg("VM action completed successfully")
	ctx := HandlerContextWith(w, r, "VMActionHandler")
	ctx.RedirectWithParams(buildVMDetailsURL(vmid), map[string]string{"success": "1", "success_msg": ctx.Translate("VMDetails.Action.Success"), "action": action})
}
