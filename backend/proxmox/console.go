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

// GetConsoleTicket logs into Proxmox using PROXMOX_CONSOLE_USER/PROXMOX_CONSOLE_PASSWORD
// and requests a VNC proxy ticket for the provided node/vmID. It returns a client
// preloaded with cookie-based auth, the port and ticket for noVNC, and a normalized
// base URL (scheme+host) for direct Proxmox redirects if needed.
func GetConsoleTicket(ctx context.Context, node string, vmID int) (*ConsoleAuthResult, error) {
	consoleUser := strings.TrimSpace(os.Getenv("PROXMOX_CONSOLE_USER"))
	consolePass := strings.TrimSpace(os.Getenv("PROXMOX_CONSOLE_PASSWORD"))
	proxmoxURL := strings.TrimSpace(os.Getenv("PROXMOX_URL"))
	insecureSkip := strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL")) == "false"

	if consoleUser == "" || consolePass == "" || proxmoxURL == "" {
		return nil, fmt.Errorf("console credentials or PROXMOX_URL missing; set PROXMOX_CONSOLE_USER and PROXMOX_CONSOLE_PASSWORD")
	}

	// Cookie-auth client then login
	consoleClient, err := NewClientCookieAuth(proxmoxURL, insecureSkip)
	if err != nil {
		return nil, fmt.Errorf("failed to create console Proxmox client: %w", err)
	}
	if err := consoleClient.Login(ctx, consoleUser, consolePass, ""); err != nil {
		return nil, fmt.Errorf("console Proxmox login failed: %w", err)
	}

	// Request VNC ticket
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
	proxmoxBase := proxmoxURL
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
