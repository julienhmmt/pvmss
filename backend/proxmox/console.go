package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// ConsoleAuthResult encapsulates the outcome of creating a console session
// using cookie-based auth: the client (with PVEAuthCookie), the VNC port and
// ticket, plus a normalized Proxmox base URL suitable for redirects.
type ConsoleAuthResult struct {
	Client              *Client
	Port                string
	Ticket              string
	ProxmoxBase         string
	PVEAuthCookie       string
	CSRFPreventionToken string
}

// GetConsoleTicket requests a VNC proxy ticket using the provided authenticated client.
// It returns the port and ticket for noVNC, and a normalized base URL for redirects.
// This version uses the user's existing authenticated session instead of separate console credentials.
func GetConsoleTicket(ctx context.Context, client ClientInterface, node string, vmID int) (*ConsoleAuthResult, error) {
	if client == nil {
		return nil, fmt.Errorf("authenticated client is required for console access")
	}

	// Cast to concrete client to access cookie auth methods
	consoleClient, ok := client.(*Client)
	if !ok {
		return nil, fmt.Errorf("client must support cookie authentication for console access")
	}

	// Ensure we have cookie auth for console access
	if consoleClient.PVEAuthCookie == "" {
		return nil, fmt.Errorf("client must have valid PVE authentication cookie for console access")
	}

	// Request VNC ticket using the authenticated client
	raw, err := consoleClient.GetVNCProxy(ctx, node, vmID)
	if err != nil {
		return nil, fmt.Errorf("failed to get VNC ticket: %w", err)
	}

	// Extract nested data { data: { port, ticket, upid } }
	var (
		portVal   string
		ticketVal string
	)
	if d, ok := raw["data"].(map[string]interface{}); ok {
		switch pv := d["port"].(type) {
		case string:
			portVal = pv
		case float64:
			portVal = strconv.Itoa(int(pv))
		case int:
			portVal = strconv.Itoa(pv)
		case json.Number:
			portVal = pv.String()
		}
		if t, ok := d["ticket"].(string); ok {
			ticketVal = t
		}
	}
	if portVal == "" || ticketVal == "" {
		return nil, fmt.Errorf("unexpected VNC proxy response; missing port or ticket")
	}

	// Compute proxmox_base (scheme://host) normalized from client ApiUrl
	proxmoxBase := consoleClient.GetApiUrl()
	if u, uErr := url.Parse(strings.TrimSpace(consoleClient.GetApiUrl())); uErr == nil {
		u.Path = ""
		proxmoxBase = u.Scheme + "://" + u.Host
	}

	return &ConsoleAuthResult{
		Client:              consoleClient,
		Port:                portVal,
		Ticket:              ticketVal,
		ProxmoxBase:         proxmoxBase,
		PVEAuthCookie:       consoleClient.PVEAuthCookie,
		CSRFPreventionToken: consoleClient.CSRFPreventionToken,
	}, nil
}

// GetConsoleTicketWithUserCredentials creates a console session using username/password.
// This is a fallback method for admin users or when the main client doesn't have cookie auth.
// DEPRECATED: Prefer using GetConsoleTicket with an authenticated client.
func GetConsoleTicketWithUserCredentials(ctx context.Context, username, password, node string, vmID int) (*ConsoleAuthResult, error) {
	proxmoxURL := strings.TrimSpace(os.Getenv("PROXMOX_URL"))
	insecureSkip := strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL")) == "false"

	if username == "" || password == "" || proxmoxURL == "" {
		return nil, fmt.Errorf("username, password, and PROXMOX_URL are required")
	}

	// Cookie-auth client then login
	consoleClient, err := NewClientCookieAuth(proxmoxURL, insecureSkip)
	if err != nil {
		return nil, fmt.Errorf("failed to create console Proxmox client: %w", err)
	}
	if err := consoleClient.Login(ctx, username, password, ""); err != nil {
		return nil, fmt.Errorf("console Proxmox login failed: %w", err)
	}

	// Use the new method with the authenticated client
	return GetConsoleTicket(ctx, consoleClient, node, vmID)
}
