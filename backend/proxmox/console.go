package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// ConsoleAuthResult encapsulates the outcome of a console session request.
type ConsoleAuthResult struct {
	Port                string
	Ticket              string
	VNCPassword         string // The extracted password for the VNC handshake
	ProxmoxBase         string
	PVEAuthCookie       string
	CSRFPreventionToken string
}

// vncData is used to unmarshal the nested 'data' object from a VNC proxy response.
type vncData struct {
	Port   json.Number `json:"port"`
	Ticket string      `json:"ticket"`
}

// GetConsoleTicket requests a VNC proxy ticket using an authenticated client.
// It uses the client's existing session, removing the need for separate console credentials.
func GetConsoleTicket(ctx context.Context, client ClientInterface, node string, vmID int) (*ConsoleAuthResult, error) {
	if client == nil {
		return nil, fmt.Errorf("authenticated client is required for console access")
	}
	if client.GetPVEAuthCookie() == "" {
		return nil, fmt.Errorf("client must have a valid PVE auth cookie for console access")
	}

	raw, err := client.GetVNCProxy(ctx, node, vmID)
	if err != nil {
		return nil, fmt.Errorf("failed to get VNC ticket: %w", err)
	}

	var response struct {
		Data vncData `json:"data"`
	}
	// The raw response is a map[string]interface{}, so we marshal and unmarshal to get it into our struct.
	bytes, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to re-marshal VNC proxy response: %w", err)
	}
	if err := json.Unmarshal(bytes, &response); err != nil {
		return nil, fmt.Errorf("failed to decode VNC proxy response: %w", err)
	}

	port := response.Data.Port.String()
	ticket := response.Data.Ticket
	if port == "" || ticket == "" {
		return nil, fmt.Errorf("VNC proxy response missing port or ticket")
	}

	// The VNC ticket from Proxmox is prefixed with user info (e.g., "PVE:user@realm:TICKET_DATA").
	// The WebSocket proxy needs the full ticket, but the VNC client needs only the password part.
	vncPassword := ticket
	if parts := strings.Split(ticket, ":"); len(parts) > 1 {
		vncPassword = parts[len(parts)-1]
	}

	proxmoxBase, err := extractBaseURL(client.GetApiUrl())
	if err != nil {
		return nil, fmt.Errorf("could not determine Proxmox base URL: %w", err)
	}

	return &ConsoleAuthResult{
		Port:                port,
		Ticket:              ticket,      // Full ticket for WebSocket proxy
		VNCPassword:         vncPassword, // Password for VNC client handshake
		ProxmoxBase:         proxmoxBase,
		PVEAuthCookie:       client.GetPVEAuthCookie(),
		CSRFPreventionToken: client.GetCSRFPreventionToken(),
	}, nil
}

// GetConsoleTicketWithUserCredentials creates a console session using a username and password.
//
// Deprecated: This function is a fallback and should be avoided. Prefer GetConsoleTicket,
// which uses an existing authenticated client session.
func GetConsoleTicketWithUserCredentials(ctx context.Context, username, password, node string, vmID int) (*ConsoleAuthResult, error) {
	proxmoxURL := os.Getenv("PROXMOX_URL")
	insecureSkip := os.Getenv("PROXMOX_VERIFY_SSL") == "false"

	if username == "" || password == "" || proxmoxURL == "" {
		return nil, fmt.Errorf("username, password, and PROXMOX_URL are required")
	}

	consoleClient, err := NewClientCookieAuth(proxmoxURL, insecureSkip)
	if err != nil {
		return nil, fmt.Errorf("failed to create console client: %w", err)
	}
	if err := consoleClient.Login(ctx, username, password, ""); err != nil {
		return nil, fmt.Errorf("console login failed: %w", err)
	}

	return GetConsoleTicket(ctx, consoleClient, node, vmID)
}

// extractBaseURL parses a raw URL and returns just the scheme and host.
func extractBaseURL(rawURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	u.Path = ""
	return u.String(), nil
}
