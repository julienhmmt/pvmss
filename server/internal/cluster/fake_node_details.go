//nolint:wsl_v5 // fake node reads keep fixture values near their source
package cluster

import "context"

const (
	fakeNodeUptimeSeconds  int64 = 86400
	fakeNodeMemoryDivider  int64 = 4
	fakeNodeSwapTotalBytes int64 = 1073741824
)

func (fake Fake) onlineDetailsNode(name string) (Node, error) {
	if fake.unavailable() {
		return Node{}, ErrUnreachable
	}
	state := fake.stateOrDefault()
	state.vmMu.RLock()
	defer state.vmMu.RUnlock()
	for _, node := range state.nodes {
		if node.Name != name {
			continue
		}
		if node.Status != NodeOnline {
			return Node{}, ErrUnreachable
		}
		return node, nil
	}
	return Node{}, ErrNotFound
}

// ReadNodeHealth returns deterministic fake health for an online node.
func (fake Fake) ReadNodeHealth(_ context.Context, name string) (NodeHealth, error) {
	node, err := fake.onlineDetailsNode(name)
	if err != nil {
		return NodeHealth{}, err
	}
	return NodeHealth{
		CPUUsage: node.CPUUsage, CPUModel: "Generic Proxmox CPU", CPUCores: node.CPUCores,
		CPUTotalThreads: node.CPUCores, CPUSockets: 1, LoadAverage: []string{"0.21", "0.16", "0.10"},
		MemoryTotal: node.MemoryTotal, MemoryUsed: node.MemoryUsed, MemoryFree: node.MemoryTotal - node.MemoryUsed,
		SwapTotal: fakeNodeSwapTotalBytes, SwapUsed: fakeNodeSwapTotalBytes / fakeNodeMemoryDivider,
		SwapFree:    fakeNodeSwapTotalBytes - fakeNodeSwapTotalBytes/fakeNodeMemoryDivider,
		RootFSTotal: node.StorageTotal, RootFSUsed: node.StorageUsed, RootFSFree: node.StorageTotal - node.StorageUsed,
		RootFSAvailable: node.StorageTotal - node.StorageUsed, UptimeSeconds: fakeNodeUptimeSeconds,
		ProxmoxVersion: fakeProxmoxVersion, KernelVersion: "Linux 6.8.12-pve",
	}, nil
}

// ReadNodeNetwork returns deterministic fake host interfaces for an online node.
func (fake Fake) ReadNodeNetwork(_ context.Context, name string) ([]NodeNetworkInterface, error) {
	if _, err := fake.onlineDetailsNode(name); err != nil {
		return nil, err
	}
	return []NodeNetworkInterface{
		{Name: "vmbr0", Type: "bridge", Active: true, Address: "192.0.2.10", CIDR: "192.0.2.10/24", Gateway: "192.0.2.1", BridgePorts: "enp1s0", BridgeVLANAware: true, MTU: 1500},
		{Name: "enp1s0", Type: "eth", Active: true, MTU: 1500},
	}, nil
}

// ReadNodePCI returns one fake PCI device for an online node.
func (fake Fake) ReadNodePCI(_ context.Context, name string) ([]PCIDevice, error) {
	if _, err := fake.onlineDetailsNode(name); err != nil {
		return nil, err
	}
	return []PCIDevice{{
		ID: "0000:03:00.0", Class: "0x020000", VendorID: "0x8086", VendorName: "Intel Corporation",
		DeviceID: "0x1572", DeviceName: "Ethernet Controller", IOMMUGroup: 14,
	}}, nil
}

// ListNodeContainers returns a fake LXC container for an online node.
func (fake Fake) ListNodeContainers(_ context.Context, name string) ([]LXCContainer, error) {
	if _, err := fake.onlineDetailsNode(name); err != nil {
		return nil, err
	}
	return []LXCContainer{{
		VMID: 9001, Name: "demo-container", Status: "running", CPUCount: 2, CPUUsage: 0.08,
		MemoryTotal: 2147483648, MemoryUsed: 536870912, DiskTotal: 8589934592, DiskUsed: 2147483648,
	}}, nil
}
