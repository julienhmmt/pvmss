package proxmox

import (
	"context"
	"fmt"
	"net/url"

	"pvmss/logger"
)

// GetVNCProxyResty creates a VNC proxy ticket for the specified VM using the Resty client.
// This requires cookie-based authentication (PVEAuthCookie + CSRFPreventionToken).
//
// POST /nodes/{node}/qemu/{vmid}/vncproxy
func GetVNCProxyResty(ctx context.Context, restyClient *RestyClient, node string, vmid int, opts *VNCProxyOptions) (*VNCProxyResponse, error) {
	if restyClient == nil {
		return nil, fmt.Errorf("restyClient is nil")
	}
	if node == "" {
		return nil, fmt.Errorf("node is required")
	}
	if vmid <= 0 {
		return nil, fmt.Errorf("invalid vmid: %d", vmid)
	}

	if opts == nil {
		opts = &VNCProxyOptions{
			Websocket: true,
		}
	}

	path := fmt.Sprintf("/nodes/%s/qemu/%d/vncproxy", url.PathEscape(node), vmid)

	formData := url.Values{}
	if opts.Websocket {
		formData.Set("websocket", "1")
	}

	var respData struct {
		Data VNCProxyResponse `json:"data"`
	}

	if err := restyClient.Post(ctx, path, formData, &respData); err != nil {
		logger.Get().Error().
			Err(err).
			Str("node", node).
			Int("vmid", vmid).
			Msg("Failed to get VNC proxy (resty)")
		return nil, fmt.Errorf("failed to get VNC proxy for VM %d on node %s: %w", vmid, node, err)
	}

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
		Msg("VNC proxy created successfully (resty)")

	return &respData.Data, nil
}
