package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/security"
)

// GetVNCProxyTicket creates a VNC proxy ticket for the specified VM using the user's stored Proxmox credentials.
// This ticket is required to establish a WebSocket connection to the VM console.
//
// Parameters:
//   - r: HTTP request containing the user session with stored Proxmox credentials
//   - node: Proxmox node name
//   - vmid: VM ID string
//
// Returns:
//   - VNC ticket string
//   - VNC port number
//   - error if ticket creation fails
func GetVNCProxyTicket(r *http.Request, node, vmid string) (ticket string, port int, err error) {
	log := CreateHandlerLogger("GetVNCProxyTicket", r).With().
		Str("node", node).
		Str("vmid", vmid).
		Logger()

	// Get Proxmox authentication from session
	pveTicket, pveCSRF, _, ok := GetProxmoxTicketFromSession(r)
	if !ok {
		log.Warn().Msg("No Proxmox ticket found in session")
		return "", 0, fmt.Errorf("proxmox authentication required")
	}

	// Get Proxmox URL from environment
	proxmoxURL := os.Getenv("PROXMOX_URL")
	if proxmoxURL == "" {
		log.Error().Msg("PROXMOX_URL not configured")
		return "", 0, fmt.Errorf("proxmox URL not configured")
	}

	// Create a temporary Proxmox client with the user's stored credentials
	insecureSkipVerify := os.Getenv("PROXMOX_VERIFY_SSL") == "false"
	client, err := proxmox.NewClientCookieAuth(proxmoxURL, insecureSkipVerify)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Proxmox client")
		return "", 0, fmt.Errorf("failed to create proxmox client: %w", err)
	}

	// Set the user's authentication credentials
	client.PVEAuthCookie = pveTicket
	client.CSRFPreventionToken = pveCSRF

	// Parse vmid to integer
	vmidInt := 0
	if _, err := fmt.Sscanf(vmid, "%d", &vmidInt); err != nil || vmidInt <= 0 {
		log.Error().Str("vmid", vmid).Msg("Invalid VM ID")
		return "", 0, fmt.Errorf("invalid VM ID: %s", vmid)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Use the proxmox package function to get VNC proxy
	log.Debug().Msg("Requesting VNC proxy from Proxmox")
	vncProxy, err := proxmox.GetVNCProxy(ctx, client, node, vmidInt, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get VNC proxy")
		return "", 0, fmt.Errorf("failed to get VNC proxy: %w", err)
	}

	// Convert port string to int
	portInt, err := strconv.Atoi(vncProxy.Port)
	if err != nil {
		log.Error().Err(err).Str("port_string", vncProxy.Port).Msg("Failed to convert port to integer")
		return "", 0, fmt.Errorf("invalid port value: %s", vncProxy.Port)
	}

	log.Info().
		Int("port", portInt).
		Str("user", vncProxy.User).
		Msg("VNC proxy ticket created successfully")

	return vncProxy.Ticket, portInt, nil
}

// LogVNCConsoleAccess logs VNC console access attempts for auditing
func LogVNCConsoleAccess(r *http.Request, vmid, node string, success bool) {
	// Get username from session if available
	username := "anonymous"
	if sessionMgr := security.GetSession(r); sessionMgr != nil {
		if u, ok := sessionMgr.Get(r.Context(), "username").(string); ok && u != "" {
			username = u
		}
	}

	log := logger.Get()
	event := log.Info()
	if !success {
		event = log.Warn()
	}

	event.
		Str("username", username).
		Str("vmid", vmid).
		Str("node", node).
		Bool("success", success).
		Str("remote_addr", r.RemoteAddr).
		Msg("VNC console access attempt")
}
