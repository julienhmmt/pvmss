//nolint:wsl_v5 // fixture construction keeps related data transformations together
package cluster

import "slices"

const fakeProxmoxVersion = "8.2.4"

var secondaryNodes = []Node{
	{Name: FakeNode01, Status: NodeOnline, CPUCores: 24, CPUUsage: 0.31, MemoryTotal: 103079215104, MemoryUsed: 34359738368, StorageTotal: 1099511627776, StorageUsed: 329853488332},
	{Name: FakeNode02, Status: NodeOnline, CPUCores: 16, CPUUsage: 0.18, MemoryTotal: 68719476736, MemoryUsed: 17179869184, StorageTotal: 1099511627776, StorageUsed: 219902325555},
}

var secondaryStorages = []Storage{
	{Name: FakeStorageLocal, Node: FakeNode01, Type: "dir", Total: 1099511627776, Used: 329853488332, SupportsVMState: false},
	{Name: FakeStorageLocalLVM, Node: FakeNode01, Type: "lvm", Total: 549755813888, Used: 109951162777, SupportsVMState: true},
	{Name: "ceph-data", Node: FakeNode02, Type: "cephfs", Total: 1099511627776, Used: 219902325555, SupportsVMState: true},
}

func (fake Fake) unavailable() bool {
	return fake.ClusterName == "offline-demo"
}

func (fake Fake) snapshotSources() ([]Node, []VM, []Storage, string) {
	if fake.ClusterName != "secondary" {
		return fakeNodes, fakeVMs, fakeStorages, fakeProxmoxVersion
	}
	return secondaryNodes, secondaryVMs(), secondaryStorages, fakeProxmoxVersion
}

func secondaryVMs() []VM {
	vms := originalFakeVMs()
	vms = slices.Clone(vms[:18])
	for index := range vms {
		vms[index].Disks = slices.Clone(vms[index].Disks)
		vms[index].Tags = slices.Clone(vms[index].Tags)
		vms[index].BootOrder = slices.Clone(vms[index].BootOrder)
		vms[index].NetworkInterfaces = cloneNetworkInterfaces(vms[index].NetworkInterfaces)
	}
	for index := range vms {
		if vms[index].VMID != 101 {
			continue
		}
		vms[index].Name = "secondary-web-02"
		vms[index].Node = FakeNode02
		vms[index].Description = "Secondary cluster web server"
	}
	return vms
}
