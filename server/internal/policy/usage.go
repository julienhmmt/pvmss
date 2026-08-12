package policy

import "slices"

func (service *Policy) ramGBExcluding(node string, vmid int) int {
	if service.projection == nil || service.projection.Load() == nil {
		return 0
	}

	var bytes int64

	for _, machine := range service.projection.Load().ByNode[node] {
		if machine.VMID == vmid || !slices.Contains(machine.Tags, "pvmss") {
			continue
		}

		bytes += machine.MemoryTotal
	}

	return int(bytes / bytesPerGB)
}
