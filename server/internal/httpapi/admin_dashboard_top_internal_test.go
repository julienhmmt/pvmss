package httpapi

import (
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"testing"
)

func TestBusiestVMsRanksRunningPVMSSVMs(t *testing.T) {
	t.Parallel()

	const gib = int64(1 << 30)

	pvmss := []string{catalog.ProtectedTagName}
	vms := []cluster.VM{
		{VMID: 1, Name: "a", Status: cluster.VMRunning, Tags: pvmss, CPUUsage: 0.5, MemoryUsed: gib, MemoryTotal: 4 * gib},
		{VMID: 2, Name: "b", Status: cluster.VMRunning, Tags: pvmss, CPUUsage: 0.9, MemoryUsed: 3 * gib, MemoryTotal: 4 * gib},
		{VMID: 3, Name: "c", Status: cluster.VMStopped, Tags: pvmss, CPUUsage: 1},
		{VMID: 4, Name: "d", Status: cluster.VMRunning, CPUUsage: 1},
		{VMID: 5, Name: "a", Status: cluster.VMRunning, Tags: pvmss, CPUUsage: 0.5, MemoryTotal: 0},
	}

	var cpu, mem []busyVMDTO
	for _, vm := range vms {
		cpu, mem = addBusyVM(cpu, mem, "k", "Label", vm)
	}

	cpu, mem = rankBusyVMs(cpu, byCPU), rankBusyVMs(mem, byMemory)

	if got := ids(cpu); len(got) != 3 || got[0] != 2 || got[1] != 1 || got[2] != 5 {
		t.Fatalf("cpu order = %v, want [2 1 5]", got)
	}

	if got := ids(mem); got[0] != 2 || mem[0].MemoryPercent != 75 {
		t.Fatalf("mem top = %v (%d%%), want vm 2 at 75%%", got, mem[0].MemoryPercent)
	}

	many := make([]busyVMDTO, 9)
	if len(rankBusyVMs(many, byCPU)) != busyVMLimit {
		t.Fatalf("not trimmed to %d", busyVMLimit)
	}
}

func ids(rows []busyVMDTO) []int {
	out := make([]int, len(rows))
	for i, r := range rows {
		out[i] = r.VMID
	}

	return out
}
