//nolint:wsl_v5 // compatibility source methods remain intentionally small
package inventory

import "pvmss/server/internal/cluster"

// All returns this index under an empty compatibility key. Registry.All uses
// real cluster names; this method keeps the pre-T15 single-index API usable.
func (index *Index) All() map[string]*Index {
	return map[string]*Index{"": index}
}

// All returns the projection's current index under a compatibility key.
func (projection *Projection) All() map[string]*Index {
	return map[string]*Index{"": projection.Load()}
}

// LookupSource resolves one VM without exposing the backing map to callers.
type LookupSource interface {
	Lookup(clusterName string, vmid int) (cluster.VM, bool)
}

// Lookup returns a VM from this single-cluster compatibility index.
func (index *Index) Lookup(_ string, vmid int) (cluster.VM, bool) {
	if index == nil {
		return cluster.VM{}, false
	}
	machine, ok := index.ByVMID[vmid]
	return machine, ok
}

// Lookup returns a VM from the index belonging to clusterName.
func (registry *Registry) Lookup(clusterName string, vmid int) (cluster.VM, bool) {
	index, err := registry.Index(clusterName)
	if err != nil || index == nil {
		return cluster.VM{}, false
	}
	machine, ok := index.ByVMID[vmid]
	return machine, ok
}

var (
	_ Source       = (*Index)(nil)
	_ Source       = (*Projection)(nil)
	_ Source       = (*Registry)(nil)
	_ LookupSource = (*Index)(nil)
	_ LookupSource = (*Registry)(nil)
)
