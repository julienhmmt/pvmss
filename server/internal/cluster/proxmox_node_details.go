//nolint:wsl_v5 // node response decoding and mapping stay adjacent
package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type proxmoxNodeHealthResponse struct {
	CPU     float64 `json:"cpu"`
	CPUInfo struct {
		Cores   int    `json:"cores"`
		CPUs    int    `json:"cpus"`
		Model   string `json:"model"`
		Sockets int    `json:"sockets"`
	} `json:"cpuinfo"`
	LoadAverage []string `json:"loadavg"`
	Memory      struct {
		Total int64 `json:"total"`
		Used  int64 `json:"used"`
		Free  int64 `json:"free"`
	} `json:"memory"`
	Swap struct {
		Total int64 `json:"total"`
		Used  int64 `json:"used"`
		Free  int64 `json:"free"`
	} `json:"swap"`
	RootFS struct {
		Total int64 `json:"total"`
		Used  int64 `json:"used"`
		Free  int64 `json:"free"`
		Avail int64 `json:"avail"`
	} `json:"rootfs"`
	Uptime        int64  `json:"uptime"`
	PVEVersion    string `json:"pveversion"`
	KVersion      string `json:"kversion"`
	CurrentKernel struct {
		Release string `json:"release"`
	} `json:"current-kernel"`
}

type proxmoxNodeNetworkRow struct {
	Name            string      `json:"iface"`
	Type            string      `json:"type"`
	Active          proxmoxBool `json:"active"`
	Address         string      `json:"address"`
	CIDR            string      `json:"cidr"`
	Gateway         string      `json:"gateway"`
	Address6        string      `json:"address6"`
	CIDR6           string      `json:"cidr6"`
	Gateway6        string      `json:"gateway6"`
	BridgePorts     string      `json:"bridge_ports"`
	BondSlaves      string      `json:"slaves"`
	BondMode        string      `json:"bond_mode"`
	VLANID          int         `json:"vlan-id"`
	BridgeVLANAware bool        `json:"bridge_vlan_aware"`
	MTU             int         `json:"mtu"`
}

type proxmoxPCIRow struct {
	ID             string `json:"id"`
	Class          string `json:"class"`
	VendorID       string `json:"vendor"`
	VendorName     string `json:"vendor_name"`
	DeviceID       string `json:"device"`
	DeviceName     string `json:"device_name"`
	IOMMUGroup     int    `json:"iommugroup"`
	MediatedDevice bool   `json:"mdev"`
}

type proxmoxLXCRaw struct {
	VMID     int     `json:"vmid"`
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	MaxCPU   float64 `json:"maxcpu"`
	CPUs     int     `json:"cpus"`
	CPUUsage float64 `json:"cpu"`
	MaxMem   int64   `json:"maxmem"`
	Mem      int64   `json:"mem"`
	MaxDisk  int64   `json:"maxdisk"`
	Disk     int64   `json:"disk"`
}

type proxmoxBool bool

func proxmoxNodeAPIPath(node, resource string) string {
	return fmt.Sprintf("/nodes/%s/%s", url.PathEscape(node), resource)
}

func readProxmoxNodeData[T any](ctx context.Context, proxmox Proxmox, node, resource string) (T, error) {
	var data T
	raw, err := proxmox.rest().do(ctx, http.MethodGet, proxmoxNodeAPIPath(node, resource), nil)
	if err != nil {
		return data, err
	}
	if err := decodeData(raw, &data); err != nil {
		return data, fmt.Errorf("decode %s: %w", resource, err)
	}
	return data, nil
}

func (response proxmoxNodeHealthResponse) nodeHealth() NodeHealth {
	kernel := response.KVersion
	if kernel == "" {
		kernel = response.CurrentKernel.Release
	}
	return NodeHealth{
		CPUUsage: response.CPU, CPUModel: response.CPUInfo.Model, CPUCores: response.CPUInfo.Cores,
		CPUTotalThreads: response.CPUInfo.CPUs, CPUSockets: response.CPUInfo.Sockets, LoadAverage: response.LoadAverage,
		MemoryTotal: response.Memory.Total, MemoryUsed: response.Memory.Used, MemoryFree: response.Memory.Free,
		SwapTotal: response.Swap.Total, SwapUsed: response.Swap.Used, SwapFree: response.Swap.Free,
		RootFSTotal: response.RootFS.Total, RootFSUsed: response.RootFS.Used,
		RootFSFree: response.RootFS.Free, RootFSAvailable: response.RootFS.Avail,
		UptimeSeconds: response.Uptime, ProxmoxVersion: response.PVEVersion, KernelVersion: kernel,
	}
}

func (row proxmoxNodeNetworkRow) nodeNetwork() NodeNetworkInterface {
	return NodeNetworkInterface{
		Name: row.Name, Type: row.Type, Active: bool(row.Active), Address: row.Address, CIDR: row.CIDR,
		Gateway: row.Gateway, Address6: row.Address6, CIDR6: row.CIDR6, Gateway6: row.Gateway6,
		BridgePorts: row.BridgePorts, BondSlaves: row.BondSlaves, BondMode: row.BondMode,
		VLANID: row.VLANID, BridgeVLANAware: row.BridgeVLANAware, MTU: row.MTU,
	}
}

func (row proxmoxPCIRow) pciDevice() PCIDevice {
	return PCIDevice(row)
}

func (row proxmoxLXCRaw) lxcContainer() LXCContainer {
	cpuCount := int(row.MaxCPU)
	if cpuCount == 0 {
		cpuCount = row.CPUs
	}
	return LXCContainer{
		VMID: row.VMID, Name: row.Name, Status: row.Status, CPUCount: cpuCount, CPUUsage: row.CPUUsage,
		MemoryTotal: row.MaxMem, MemoryUsed: row.Mem, DiskTotal: row.MaxDisk, DiskUsed: row.Disk,
	}
}

// ReadNodeHealth reads CPU, memory, filesystem, uptime, and software versions from one node.
func (proxmox Proxmox) ReadNodeHealth(ctx context.Context, node string) (NodeHealth, error) {
	response, err := readProxmoxNodeData[proxmoxNodeHealthResponse](ctx, proxmox, node, "status")
	if err != nil {
		return NodeHealth{}, fmt.Errorf("read node %q status: %w", node, err)
	}
	return response.nodeHealth(), nil
}

// ReadNodeNetwork reads the configured interfaces of one node without changing them.
func (proxmox Proxmox) ReadNodeNetwork(ctx context.Context, node string) ([]NodeNetworkInterface, error) {
	rows, err := readProxmoxNodeData[[]proxmoxNodeNetworkRow](ctx, proxmox, node, "network")
	if err != nil {
		return nil, fmt.Errorf("read node %q network: %w", node, err)
	}
	interfaces := make([]NodeNetworkInterface, len(rows))
	for i, row := range rows {
		interfaces[i] = row.nodeNetwork()
	}
	return interfaces, nil
}

// ReadNodePCI reads the inventory of PCI devices reported by one node.
func (proxmox Proxmox) ReadNodePCI(ctx context.Context, node string) ([]PCIDevice, error) {
	rows, err := readProxmoxNodeData[[]proxmoxPCIRow](ctx, proxmox, node, "hardware/pci")
	if err != nil {
		return nil, fmt.Errorf("read node %q PCI devices: %w", node, err)
	}
	devices := make([]PCIDevice, len(rows))
	for i, row := range rows {
		devices[i] = row.pciDevice()
	}
	return devices, nil
}

// ListNodeContainers reads the LXC containers currently hosted by one node.
func (proxmox Proxmox) ListNodeContainers(ctx context.Context, node string) ([]LXCContainer, error) {
	rows, err := readProxmoxNodeData[[]proxmoxLXCRaw](ctx, proxmox, node, "lxc")
	if err != nil {
		return nil, fmt.Errorf("list node %q containers: %w", node, err)
	}
	containers := make([]LXCContainer, len(rows))
	for i, row := range rows {
		containers[i] = row.lxcContainer()
	}
	return containers, nil
}

func (value *proxmoxBool) UnmarshalJSON(data []byte) error {
	var parsed bool
	if err := json.Unmarshal(data, &parsed); err == nil {
		*value = proxmoxBool(parsed)
		return nil
	}
	var numeric int
	if err := json.Unmarshal(data, &numeric); err != nil {
		return err
	}
	*value = proxmoxBool(numeric != 0)
	return nil
}
