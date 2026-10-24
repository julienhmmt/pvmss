package cluster

import "context"

// NodeHealth contains live resource and software details from one Proxmox node.
type NodeHealth struct {
	CPUUsage        float64  `json:"cpuUsage"`
	CPUModel        string   `json:"cpuModel"`
	CPUCores        int      `json:"cpuCores"`
	CPUTotalThreads int      `json:"cpuTotalThreads"`
	CPUSockets      int      `json:"cpuSockets"`
	LoadAverage     []string `json:"loadAverage"`
	MemoryTotal     int64    `json:"memoryTotalBytes"`
	MemoryUsed      int64    `json:"memoryUsedBytes"`
	MemoryFree      int64    `json:"memoryFreeBytes"`
	SwapTotal       int64    `json:"swapTotalBytes"`
	SwapUsed        int64    `json:"swapUsedBytes"`
	SwapFree        int64    `json:"swapFreeBytes"`
	RootFSTotal     int64    `json:"rootfsTotalBytes"`
	RootFSUsed      int64    `json:"rootfsUsedBytes"`
	RootFSFree      int64    `json:"rootfsFreeBytes"`
	RootFSAvailable int64    `json:"rootfsAvailableBytes"`
	UptimeSeconds   int64    `json:"uptimeSeconds"`
	ProxmoxVersion  string   `json:"proxmoxVersion"`
	KernelVersion   string   `json:"kernelVersion"`
}

// NodeNetworkInterface contains the useful read-only configuration for one host interface.
type NodeNetworkInterface struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	Active          bool   `json:"active"`
	Address         string `json:"address"`
	CIDR            string `json:"cidr"`
	Gateway         string `json:"gateway"`
	Address6        string `json:"address6"`
	CIDR6           string `json:"cidr6"`
	Gateway6        string `json:"gateway6"`
	BridgePorts     string `json:"bridgePorts"`
	BondSlaves      string `json:"bondSlaves"`
	BondMode        string `json:"bondMode"`
	VLANID          int    `json:"vlanId"`
	BridgeVLANAware bool   `json:"bridgeVlanAware"`
	MTU             int    `json:"mtu"`
}

// PCIDevice describes one PCI device reported by Proxmox.
type PCIDevice struct {
	ID             string `json:"id"`
	Class          string `json:"class"`
	VendorID       string `json:"vendorId"`
	VendorName     string `json:"vendorName"`
	DeviceID       string `json:"deviceId"`
	DeviceName     string `json:"deviceName"`
	IOMMUGroup     int    `json:"iommuGroup"`
	MediatedDevice bool   `json:"mediatedDevice"`
}

// LXCContainer is a read-only summary of a container hosted by a node.
type LXCContainer struct {
	VMID        int     `json:"vmid"`
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	CPUCount    int     `json:"cpuCount"`
	CPUUsage    float64 `json:"cpuUsage"`
	MemoryTotal int64   `json:"memoryTotalBytes"`
	MemoryUsed  int64   `json:"memoryUsedBytes"`
	DiskTotal   int64   `json:"diskTotalBytes"`
	DiskUsed    int64   `json:"diskUsedBytes"`
}

// NodeDetailsReader reads live sections on the administrator's node detail page.
type NodeDetailsReader interface {
	ReadNodeHealth(ctx context.Context, node string) (NodeHealth, error)
	ReadNodeNetwork(ctx context.Context, node string) ([]NodeNetworkInterface, error)
	ReadNodePCI(ctx context.Context, node string) ([]PCIDevice, error)
	ListNodeContainers(ctx context.Context, node string) ([]LXCContainer, error)
}
