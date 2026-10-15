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
	pool, err := findPool(ctx, client, name)
	if err != nil {
		return PoolDetail{}, err
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
	if err := detail.markManaged(ctx, checker, clusterName); err != nil {
		return PoolDetail{}, err
	}
	return detail, nil
}

// findPool resolves name against the cluster's pool list, or returns
// ErrNotFound when no pool with that name exists.
func findPool(ctx context.Context, client cluster.Client, name string) (cluster.Pool, error) {
	poolList, err := client.ListPools(ctx)
	if err != nil {
		return cluster.Pool{}, err
	}
	for _, pool := range poolList {
		if pool.Name == name {
			return pool, nil
		}
	}
	return cluster.Pool{}, fmt.Errorf("%w: %q", ErrNotFound, name)
}

// markManaged joins the managed-pool registration (marker and creation
// timestamp) into the detail when PVMSS provisioned the pool.
func (detail *PoolDetail) markManaged(ctx context.Context, checker *store.Store, clusterName string) error {
	if checker == nil || clusterName == "" {
		return nil
	}
	managed, err := checker.ManagedPools(ctx, clusterName)
	if err != nil {
		return err
	}
	for _, entry := range managed {
		if entry.Name == detail.Name {
			detail.Managed = true
			detail.CreatedAt = entry.CreatedAt
			break
		}
	}
	return nil
}
