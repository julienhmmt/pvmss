package vm

import (
	"context"
	"errors"
	"fmt"
)

// Proxmox accepts VMIDs 100..999999999.
const (
	MinProxmoxVMID = 100
	MaxProxmoxVMID = 999999999
)

// ErrVMIDRangeExhausted means every id of the cluster's configured VMID range
// is in use. Mapped to 409 by the handler.
var ErrVMIDRangeExhausted = errors.New("vmid range exhausted")

// IDLister lists every VM and container id in use on a cluster, whoever
// created it. Creators that do not implement it cannot honour a VMID range.
type IDLister interface {
	UsedVMIDs(ctx context.Context) ([]int, error)
}

// AllocateVMID returns the lowest id in [lo, hi] absent from used.
func AllocateVMID(lo, hi int, used []int) (int, error) {
	taken := make(map[int]struct{}, len(used))
	for _, id := range used {
		taken[id] = struct{}{}
	}

	for id := lo; id <= hi; id++ {
		if _, busy := taken[id]; !busy {
			return id, nil
		}
	}

	return 0, fmt.Errorf("%w: all ids from %d to %d are in use", ErrVMIDRangeExhausted, lo, hi)
}

// allocateVMID is the single allocation point of the create paths: Proxmox's
// next free id by default, the lowest free id of the cluster's range when one
// is configured.
func allocateVMID(ctx context.Context, deps CreateDeps) (int, error) {
	if deps.VMIDMin == 0 && deps.VMIDMax == 0 {
		return deps.Creator.NextVMID(ctx)
	}

	lister, ok := deps.Creator.(IDLister)
	if !ok {
		return 0, errors.New("cluster client cannot list used vmids")
	}

	used, err := lister.UsedVMIDs(ctx)
	if err != nil {
		return 0, err
	}

	return AllocateVMID(deps.VMIDMin, deps.VMIDMax, used)
}
