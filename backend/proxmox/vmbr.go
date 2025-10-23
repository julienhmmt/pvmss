package proxmox

// VMBR represents a network interface in Proxmox
type VMBR struct {
	Iface       string `json:"iface"`
	IfaceName   string `json:"name"`
	Type        string `json:"type"`
	Method      string `json:"method"`
	Address     string `json:"address"`
	Netmask     string `json:"netmask"`
	Gateway     string `json:"gateway"`
	BridgePorts string `json:"bridge_ports"`
	Comments    string `json:"comments"`
	Active      any    `json:"active"`
	BridgeFD    any    `json:"bridge_fd"`
	BridgeSTP   any    `json:"bridge_stp"`
}

// LEGACY FUNCTIONS REMOVED - Use resty versions instead:
// - GetVMBRs → GetVMBRsResty
// - GetVMBRsWithContext → GetVMBRsResty
// See resty_network.go for modern implementations
