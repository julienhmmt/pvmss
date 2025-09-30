package proxmox

import (
	"context"
	"fmt"
	"net/url"

	"pvmss/logger"
)

// VNCProxyResponse represents the response from the vncproxy endpoint.
// POST /api2/json/nodes/{node}/qemu/{vmid}/vncproxy
type VNCProxyResponse struct {
	User   string `json:"user"`   // Username for VNC connection
	Ticket string `json:"ticket"` // VNC ticket (valid for 2 hours)
	Cert   string `json:"cert"`   // SSL certificate
	Port   string `json:"port"`   // VNC WebSocket port (5900-5999) - returned as string by Proxmox
	Upid   string `json:"upid"`   // Unique process ID
}

// VNCProxyOptions holds optional parameters for VNC proxy creation.
type VNCProxyOptions struct {
	// Generate VNC ticket with WebSocket support (default: true for modern clients)
	Websocket bool
}

// GetVNCProxy creates a VNC proxy ticket for the specified VM.
// This ticket is required to establish a VNC console connection to the VM.
//
// POST /api2/json/nodes/{node}/qemu/{vmid}/vncproxy
//
// Parameters:
//   - ctx: Context for the request
//   - client: Authenticated Proxmox client
//   - node: Proxmox node name where the VM is located
//   - vmid: Virtual Machine ID
//   - opts: Optional parameters (nil for defaults)
//
// Returns:
//   - VNCProxyResponse containing ticket, port, and connection details
//   - error if the request fails
//
// The VNC ticket is valid for 2 hours and provides access to the VM console.
// The port is dynamically assigned by Proxmox in the range 5900-5999.
//
// Example:
//
//	vncProxy, err := proxmox.GetVNCProxy(ctx, client, "pve1", 100, nil)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("VNC Port: %d, Ticket: %s\n", vncProxy.Port, vncProxy.Ticket)
func GetVNCProxy(ctx context.Context, client ClientInterface, node string, vmid int, opts *VNCProxyOptions) (*VNCProxyResponse, error) {
	if err := validateClientAndParams(client, param{"node", node}); err != nil {
		return nil, err
	}
	if vmid <= 0 {
		return nil, fmt.Errorf("invalid vmid: %d", vmid)
	}

	ctx, cancel := withDefaultTimeout(ctx, client.GetTimeout())
	defer cancel()

	// Apply defaults
	if opts == nil {
		opts = &VNCProxyOptions{
			Websocket: true, // Enable WebSocket by default for modern browsers
		}
	}

	// Build API path
	path := fmt.Sprintf("/nodes/%s/qemu/%d/vncproxy", url.PathEscape(node), vmid)

	// Prepare form data
	formData := url.Values{}
	if opts.Websocket {
		formData.Set("websocket", "1")
	}

	// Make POST request
	var respData struct {
		Data VNCProxyResponse `json:"data"`
	}

	if err := client.PostFormAndGetJSON(ctx, path, formData, &respData); err != nil {
		logger.Get().Error().
			Err(err).
			Str("node", node).
			Int("vmid", vmid).
			Msg("Failed to get VNC proxy")
		return nil, fmt.Errorf("failed to get VNC proxy for VM %d on node %s: %w", vmid, node, err)
	}

	// Validate response
	if respData.Data.Ticket == "" {
		return nil, fmt.Errorf("VNC proxy response missing ticket")
	}
	if respData.Data.Port == "" {
		return nil, fmt.Errorf("VNC proxy response missing port")
	}

	logger.Get().Info().
		Str("node", node).
		Int("vmid", vmid).
		Str("port", respData.Data.Port).
		Str("user", respData.Data.User).
		Msg("VNC proxy created successfully")

	return &respData.Data, nil
}

// GetVNCProxyWithContext is an alias for GetVNCProxy for backward compatibility.
// Deprecated: Use GetVNCProxy directly as it already accepts a context.
func GetVNCProxyWithContext(ctx context.Context, client ClientInterface, node string, vmid int) (*VNCProxyResponse, error) {
	return GetVNCProxy(ctx, client, node, vmid, nil)
}
