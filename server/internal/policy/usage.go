package policy

import (
	"pvmss/server/internal/inventory"
	"slices"
)

func ramGBExcluding(index *inventory.Index, node string, vmid int) int {
	if index == nil {
		return 0
	}

	var bytes int64

	for _, machine := range index.ByNode[node] {
		if machine.VMID == vmid || !slices.Contains(machine.Tags, "pvmss") {
			continue
		}

		bytes += machine.MemoryTotal
	}

	return int(bytes / bytesPerGB)
}

func diskGBExcluding(index *inventory.Index, node string, vmid int) int {
	if index == nil {
		return 0
	}

	var bytes int64

	for _, machine := range index.ByNode[node] {
		if machine.VMID == vmid || !slices.Contains(machine.Tags, "pvmss") {
			continue
		}

		bytes += machine.DiskTotal
	}

	return int(bytes / bytesPerGB)
}

// StorageFreeBytes returns the available bytes on a storage backend on a node,
// read from the named cluster's in-memory inventory projection. Returns 0 when
// the projection is empty or the storage is not found - callers that need a
// hard check must use the live cluster.StorageFreeSpace instead.
func (service *Policy) StorageFreeBytes(clusterName, node, storage string) (int64, error) {
	index, err := service.indexFor(clusterName)
	if err != nil {
		return 0, err
	}

	if index == nil {
		return 0, nil
	}

	for _, s := range index.StoragesByNode[node] {
		if s.Name == storage {
			return s.Total - s.Used, nil
		}
	}

	return 0, nil
}
