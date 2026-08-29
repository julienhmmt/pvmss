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

func (service *Policy) diskGBExcluding(node string, vmid int) int {
	if service.projection == nil || service.projection.Load() == nil {
		return 0
	}

	var bytes int64

	for _, machine := range service.projection.Load().ByNode[node] {
		if machine.VMID == vmid || !slices.Contains(machine.Tags, "pvmss") {
			continue
		}

		bytes += machine.DiskTotal
	}

	return int(bytes / bytesPerGB)
}

// StorageFreeBytes returns the available bytes on a storage backend on a node,
// read from the in-memory inventory projection (US3/issue-04). Returns 0 when
// the projection is empty or the storage is not found — callers that need a
// hard check must use the live cluster.StorageFreeSpace instead.
func (service *Policy) StorageFreeBytes(node, storage string) int64 {
	if service.projection == nil || service.projection.Load() == nil {
		return 0
	}

	for _, s := range service.projection.Load().StoragesByNode[node] {
		if s.Name == storage {
			return s.Total - s.Used
		}
	}

	return 0
}
