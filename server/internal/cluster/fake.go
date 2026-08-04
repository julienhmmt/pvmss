package cluster

import "context"

// Fake is the built-in cluster substitute (constitution XI). It requires no
// external service and serves a stable, hand-authored dataset. Neither this
// type nor Proxmox reports which one it is — callers cannot tell them apart.
type Fake struct{}

// ListNodes implements Client.
func (Fake) ListNodes(_ context.Context) ([]Node, error) {
	nodes := make([]Node, len(fakeNodes))
	copy(nodes, fakeNodes)
	return nodes, nil
}

// The dataset below is production code (constitution XI), reviewed and
// versioned like the rest. Later tranches extend it as they add features —
// only Node is surfaced by an endpoint at T01; VM, Storage, and Pool ride
// along so those tranches have something real to work with.

var fakeNodes = []Node{
	{
		Name:         "pve-node-01",
		Status:       NodeOnline,
		CPUCores:     32,
		CPUUsage:     0.42,
		MemoryTotal:  137438953472,
		MemoryUsed:   68719476736,
		StorageTotal: 2199023255552,
		StorageUsed:  879609302220,
	},
	{
		Name:         "pve-node-02",
		Status:       NodeOnline,
		CPUCores:     16,
		CPUUsage:     0.15,
		MemoryTotal:  68719476736,
		MemoryUsed:   17179869184,
		StorageTotal: 1099511627776,
		StorageUsed:  219902325555,
	},
	{
		Name:         "pve-node-03",
		Status:       NodeOffline,
		CPUCores:     16,
		CPUUsage:     0,
		MemoryTotal:  68719476736,
		MemoryUsed:   0,
		StorageTotal: 1099511627776,
		StorageUsed:  0,
	},
}

var fakePools = []Pool{
	{Name: "pool-alice", Comment: "Alice's personal pool"},
	{Name: "pool-bob", Comment: "Bob's personal pool"},
	{Name: "pool-carol", Comment: "Carol's personal pool"},
	{Name: "pool-shared", Comment: "Shared infrastructure pool"},
}

var fakeStorages = []Storage{
	{Name: "local", Node: "pve-node-01", Type: "dir", Total: 2199023255552, Used: 879609302220},
	{Name: "local-lvm", Node: "pve-node-01", Type: "lvm", Total: 549755813888, Used: 219902325555},
	{Name: "ceph-data", Node: "pve-node-02", Type: "cephfs", Total: 1099511627776, Used: 329853488332},
	{Name: "local", Node: "pve-node-02", Type: "dir", Total: 274877906944, Used: 68719476736},
	{Name: "backup-nfs", Node: "pve-node-03", Type: "nfs", Total: 5497558138880, Used: 1099511627776},
}

var fakeVMs = []VM{
	{VMID: 100, Name: "web-01", Node: "pve-node-01", Status: VMRunning, Pool: "pool-alice", Tags: []string{"pvmss", "web"}, CPUCores: 2, MemoryTotal: 4294967296},
	{VMID: 101, Name: "web-02", Node: "pve-node-01", Status: VMStopped, Pool: "pool-alice", Tags: []string{"pvmss", "web"}, CPUCores: 2, MemoryTotal: 4294967296},
	{VMID: 102, Name: "db-01", Node: "pve-node-01", Status: VMRunning, Pool: "pool-alice", Tags: []string{"pvmss", "db"}, CPUCores: 4, MemoryTotal: 8589934592},
	{VMID: 103, Name: "cache-01", Node: "pve-node-01", Status: VMRunning, Pool: "pool-bob", Tags: []string{"pvmss", "cache"}, CPUCores: 2, MemoryTotal: 2147483648},
	{VMID: 104, Name: "build-01", Node: "pve-node-01", Status: VMStopped, Pool: "pool-bob", Tags: []string{"pvmss", "ci"}, CPUCores: 4, MemoryTotal: 8589934592},
	{VMID: 105, Name: "test-01", Node: "pve-node-02", Status: VMRunning, Pool: "pool-bob", Tags: []string{"pvmss", "ci"}, CPUCores: 2, MemoryTotal: 4294967296},
	{VMID: 106, Name: "test-02", Node: "pve-node-02", Status: VMStopped, Pool: "pool-bob", Tags: []string{"pvmss", "ci"}, CPUCores: 2, MemoryTotal: 4294967296},
	{VMID: 107, Name: "mail-01", Node: "pve-node-02", Status: VMRunning, Pool: "pool-carol", Tags: []string{"pvmss", "mail"}, CPUCores: 2, MemoryTotal: 4294967296},
	{VMID: 108, Name: "proxy-01", Node: "pve-node-02", Status: VMRunning, Pool: "pool-carol", Tags: []string{"pvmss", "proxy"}, CPUCores: 1, MemoryTotal: 1073741824},
	{VMID: 109, Name: "legacy-01", Node: "pve-node-02", Status: VMStopped, Pool: "pool-carol", Tags: nil, CPUCores: 4, MemoryTotal: 8589934592},
	{VMID: 110, Name: "legacy-02", Node: "pve-node-02", Status: VMStopped, Pool: "pool-carol", Tags: nil, CPUCores: 4, MemoryTotal: 8589934592},
	{VMID: 111, Name: "backup-01", Node: "pve-node-03", Status: VMStopped, Pool: "pool-shared", Tags: []string{"pvmss", "backup"}, CPUCores: 2, MemoryTotal: 4294967296},
	{VMID: 112, Name: "monitor-01", Node: "pve-node-01", Status: VMRunning, Pool: "pool-shared", Tags: []string{"pvmss", "monitoring"}, CPUCores: 2, MemoryTotal: 4294967296},
	{VMID: 113, Name: "monitor-02", Node: "pve-node-01", Status: VMPaused, Pool: "pool-shared", Tags: []string{"pvmss", "monitoring"}, CPUCores: 2, MemoryTotal: 4294967296},
	{VMID: 114, Name: "sandbox-01", Node: "pve-node-02", Status: VMStopped, Pool: "pool-alice", Tags: []string{"pvmss", "sandbox"}, CPUCores: 1, MemoryTotal: 1073741824},
	{VMID: 115, Name: "sandbox-02", Node: "pve-node-02", Status: VMStopped, Pool: "pool-alice", Tags: []string{"pvmss", "sandbox"}, CPUCores: 1, MemoryTotal: 1073741824},
	{VMID: 116, Name: "app-01", Node: "pve-node-01", Status: VMRunning, Pool: "pool-bob", Tags: []string{"pvmss", "app"}, CPUCores: 4, MemoryTotal: 8589934592},
	{VMID: 117, Name: "app-02", Node: "pve-node-01", Status: VMRunning, Pool: "pool-bob", Tags: []string{"pvmss", "app"}, CPUCores: 4, MemoryTotal: 8589934592},
	{VMID: 118, Name: "app-03", Node: "pve-node-02", Status: VMRunning, Pool: "pool-bob", Tags: []string{"pvmss", "app"}, CPUCores: 4, MemoryTotal: 8589934592},
	{VMID: 119, Name: "queue-01", Node: "pve-node-02", Status: VMRunning, Pool: "pool-carol", Tags: []string{"pvmss", "queue"}, CPUCores: 2, MemoryTotal: 4294967296},
	{VMID: 120, Name: "search-01", Node: "pve-node-01", Status: VMRunning, Pool: "pool-carol", Tags: []string{"pvmss", "search"}, CPUCores: 4, MemoryTotal: 17179869184},
	{VMID: 121, Name: "archive-01", Node: "pve-node-03", Status: VMStopped, Pool: "pool-shared", Tags: nil, CPUCores: 2, MemoryTotal: 4294967296},
	{VMID: 122, Name: "archive-02", Node: "pve-node-03", Status: VMStopped, Pool: "pool-shared", Tags: nil, CPUCores: 2, MemoryTotal: 4294967296},
	{VMID: 123, Name: "dev-01", Node: "pve-node-01", Status: VMRunning, Pool: "pool-alice", Tags: []string{"pvmss", "dev"}, CPUCores: 2, MemoryTotal: 4294967296},
	{VMID: 124, Name: "dev-02", Node: "pve-node-01", Status: VMStopped, Pool: "pool-alice", Tags: []string{"pvmss", "dev"}, CPUCores: 2, MemoryTotal: 4294967296},
}
