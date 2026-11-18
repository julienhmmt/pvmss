package httpapi

import (
	"cmp"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"slices"
)

// busyVMLimit is how many VMs each busiest-VM list on the dashboard shows.
const busyVMLimit = 5

// busyVMDTO is one row of the dashboard's busiest PVMSS VM lists.
type busyVMDTO struct {
	ClusterKey       string `json:"clusterKey"`
	Cluster          string `json:"cluster"`
	Node             string `json:"node"`
	VMID             int    `json:"vmid"`
	Name             string `json:"name"`
	Pool             string `json:"pool"`
	CPUPercent       int    `json:"cpuPercent"`
	MemoryPercent    int    `json:"memoryPercent"`
	MemoryUsedBytes  int64  `json:"memoryUsedBytes"`
	MemoryTotalBytes int64  `json:"memoryTotalBytes"`
}

// addBusyVM appends a running PVMSS VM to both candidate lists. VMs without
// allocated memory are left out of the memory list (no meaningful percent).
func addBusyVM(cpu, mem []busyVMDTO, clusterKey, label string, vm cluster.VM) ([]busyVMDTO, []busyVMDTO) {
	if vm.Status != cluster.VMRunning || !slices.Contains(vm.Tags, catalog.ProtectedTagName) {
		return cpu, mem
	}

	row := busyVMDTO{
		ClusterKey: clusterKey, Cluster: label, Node: vm.Node, VMID: vm.VMID, Name: vm.Name, Pool: vm.Pool,
		CPUPercent: int(vm.CPUUsage*100 + 0.5), MemoryPercent: percentOf(vm.MemoryUsed, vm.MemoryTotal),
		MemoryUsedBytes: vm.MemoryUsed, MemoryTotalBytes: vm.MemoryTotal,
	}

	cpu = append(cpu, row)
	if vm.MemoryTotal > 0 {
		mem = append(mem, row)
	}

	return cpu, mem
}

func byCPU(r busyVMDTO) int    { return r.CPUPercent }
func byMemory(r busyVMDTO) int { return r.MemoryPercent }

// rankBusyVMs returns a sorted copy of rows (value desc, then name, cluster,
// VMID) trimmed to busyVMLimit.
func rankBusyVMs(rows []busyVMDTO, value func(busyVMDTO) int) []busyVMDTO {
	out := slices.Clone(rows)
	slices.SortFunc(out, func(a, b busyVMDTO) int {
		return cmp.Or(cmp.Compare(value(b), value(a)), cmp.Compare(a.Name, b.Name),
			cmp.Compare(a.ClusterKey, b.ClusterKey), cmp.Compare(a.VMID, b.VMID))
	})

	return append([]busyVMDTO{}, out[:min(len(out), busyVMLimit)]...)
}
