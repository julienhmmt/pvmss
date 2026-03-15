package proxmox

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
