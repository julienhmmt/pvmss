//nolint:wsl_v5 // domain steps are kept adjacent for readable orchestration
package pools

import (
	"context"
	"fmt"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
)

// PoolDetail is the admin-facing per-pool aggregate: identity, the managed
// marker with its registration timestamp, and the current member VMs drawn
// from the inventory projection.
type PoolDetail struct {
	Name string
	// Username is the Proxmox sign-in account provisioned with the pool,
	// following the "<pool>@pve" convention used at create and delete time.
	Username string
	Comment  string
	// Managed reports whether PVMSS provisioned the pool; unmanaged pools are
	// still returned so the page can show them read-only.
	Managed bool
	// CreatedAt is the RFC3339 managed-pool registration time, "" when
	// unmanaged or when no store is wired.
	CreatedAt string
	Members   []cluster.VM
}

// Detail returns the aggregate for one named pool on the cluster. Unlike
// ListWithManaged it does not require the pool to be managed: lookup is
// against Proxmox, with the managed marker joined in when present. Returns
// ErrNotFound when no pool with name exists on the cluster, and
// ErrProjectionNotReady when the inventory has not completed its first
// refresh.
func Detail(ctx context.Context, client cluster.Client, projection *inventory.Projection, checker *store.Store, clusterName, name string) (PoolDetail, error) {
	poolList, err := client.ListPools(ctx)
	if err != nil {
		return PoolDetail{}, err
	}
	var pool *cluster.Pool
	for i := range poolList {
		if poolList[i].Name == name {
			pool = &poolList[i]
			break
		}
	}
	if pool == nil {
		return PoolDetail{}, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	index := projection.Load()
	if index == nil {
		return PoolDetail{}, ErrProjectionNotReady
	}
	detail := PoolDetail{
		Name:     pool.Name,
		Username: pool.Name + "@pve",
		Comment:  pool.Comment,
		Members:  index.ByPool[pool.Name],
	}
	if detail.Members == nil {
		detail.Members = []cluster.VM{}
	}
	if checker != nil && clusterName != "" {
		managed, err := checker.ManagedPools(ctx, clusterName)
		if err != nil {
			return PoolDetail{}, err
		}
		for _, entry := range managed {
			if entry.Name == name {
				detail.Managed = true
				detail.CreatedAt = entry.CreatedAt
				break
			}
		}
	}
	return detail, nil
}
