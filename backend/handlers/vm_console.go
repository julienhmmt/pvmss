package handlers

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/proxmox"
	"pvmss/security"
)

func init() {
	gob.Register(consoleSessionPayload{})
}

type consoleSessionPayload struct {
	ConsoleURL   string
	WebsocketURL string
	Ticket       string
	AuthCookie   string
	CsrfToken    string
	Host         string
	Port         int
	ExpiresAt    int64
}

// VMConsoleHandler handles VM console requests
func (h *VMHandler) VMConsoleHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("VMConsoleHandler", r)

	if !IsAuthenticated(r) {
		log.Warn().Msg("Console request rejected: not authenticated")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	vmid := strings.TrimSpace(r.FormValue("vmid"))
	node := strings.TrimSpace(r.FormValue("node"))
	vmname := strings.TrimSpace(r.FormValue("vmname"))

	if vmid == "" {
		log.Error().Msg("VM ID is required")
		http.Error(w, "VM ID is required", http.StatusBadRequest)
		return
	}

	log.Info().Str("vmid", vmid).Str("node", node).Str("vmname", vmname).Msg("Processing console request")

	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		log.Error().Err(err).Str("vmid", vmid).Msg("Invalid VM ID")
		http.Error(w, "Invalid VM ID", http.StatusBadRequest)
		return
	}

	stateManager := getStateManager(r)
	if stateManager == nil {
		log.Error().Msg("State manager not available")
		http.Error(w, "Service unavailable", http.StatusInternalServerError)
		return
	}

	client := stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not available")
		http.Error(w, "Proxmox connection not available", http.StatusServiceUnavailable)
		return
	}

	sessionManager := security.GetSession(r)
	if sessionManager == nil {
		log.Error().Msg("Session manager not available")
		http.Error(w, "Session not available", http.StatusUnauthorized)
		return
	}

	pveAuthCookie := strings.TrimSpace(sessionManager.GetString(r.Context(), "pve_auth_cookie"))
	csrfToken := strings.TrimSpace(sessionManager.GetString(r.Context(), "csrf_prevention_token"))
	username := strings.TrimSpace(sessionManager.GetString(r.Context(), "username"))
	isAdmin := sessionManager.GetBool(r.Context(), "is_admin")

	log.Info().
		Str("username", username).
		Bool("is_admin", isAdmin).
		Bool("has_pve_cookie", pveAuthCookie != "").
		Bool("has_csrf_token", csrfToken != "").
		Str("session_id", sessionManager.Token(r.Context())).
		Msg("Console access attempt - session details")

	if pveAuthCookie == "" {
		log.Warn().Str("username", username).Msg("Missing PVE auth cookie in session")
		http.Error(w, "Session expired. Please log in again to access the console.", http.StatusUnauthorized)
		return
	}

	if username == "" {
		log.Warn().Msg("Missing username in session")
		http.Error(w, "Session expired. Please log in again to access the console.", http.StatusUnauthorized)
		return
	}

	// Resolve node if not provided
	actualNode := node
	if actualNode == "" {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		vms, err := proxmox.GetVMsWithContext(ctx, client)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get VM list")
			http.Error(w, "Failed to get VM information", http.StatusInternalServerError)
			return
		}

		for _, vm := range vms {
			if vm.VMID == vmidInt {
				actualNode = vm.Node
				break
			}
		}

		if actualNode == "" {
			log.Error().Int("vmid", vmidInt).Msg("Unable to determine VM node")
			http.Error(w, "Unable to determine VM node", http.StatusNotFound)
			return
		}
	}

	apiURL := client.GetApiUrl()

	log.Info().
		Str("username", username).
		Str("api_url", apiURL).
		Str("node", actualNode).
		Int("vmid", vmidInt).
		Msg("Requesting VNC console access from Proxmox")

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	access, err := buildConsoleAccess(ctx, apiURL, actualNode, vmidInt, pveAuthCookie, csrfToken, username)
	if err != nil {
		if errors.Is(err, errProxmoxUnauthorized) {
			log.Warn().
				Str("username", username).
				Str("node", actualNode).
				Int("vmid", vmidInt).
				Msg("Proxmox returned unauthorized for console access")
			http.Error(w, "Authentication with Proxmox failed. Please log in again.", http.StatusUnauthorized)
			return
		}
		if errors.Is(err, errProxmoxForbidden) {
			log.Warn().
				Str("username", username).
				Str("node", actualNode).
				Int("vmid", vmidInt).
				Msg("Proxmox denied console access - insufficient permissions")
			http.Error(w, "Insufficient permissions for console access. Ensure your user has 'VM.Console' permission.", http.StatusForbidden)
			return
		}
		log.Error().
			Err(err).
			Str("username", username).
			Str("node", actualNode).
			Int("vmid", vmidInt).
			Str("api_url", apiURL).
			Msg("Failed to get console access from Proxmox")
		http.Error(w, "Failed to access console", http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("username", username).
		Str("node", actualNode).
		Int("vmid", vmidInt).
		Str("host", access.Host).
		Int("port", access.Port).
		Str("websocket_url", access.WebsocketURL).
		Str("console_url", access.ConsoleURL).
		Msg("Successfully obtained console access from Proxmox")

	// Set cookies for cross-domain authentication
	setProxmoxAuthCookies(w, pveAuthCookie, csrfToken, access.Host)

	// Create a console session that includes authentication
	consoleSession := consoleSessionPayload{
		ConsoleURL:   access.ConsoleURL,
		WebsocketURL: access.WebsocketURL,
		Ticket:       access.Ticket,
		AuthCookie:   pveAuthCookie,
		CsrfToken:    csrfToken,
		Host:         access.Host,
		Port:         access.Port,
		ExpiresAt:    time.Now().Add(8 * time.Second).Unix(), // 8 seconds to be safe
	}

	// Store session temporarily for WebSocket proxy
	sessionManager.Put(r.Context(), fmt.Sprintf("console_session_%d_%s", vmidInt, actualNode), consoleSession)

	response := map[string]interface{}{
		"success":     true,
		"console_url": access.ConsoleURL, // Use the FULL URL with all parameters
		"message":     "Console access granted",
		"node":        actualNode,
		"vmid":        vmidInt,
		"expires_in":  8, // seconds
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode console response")
		return
	}
}
