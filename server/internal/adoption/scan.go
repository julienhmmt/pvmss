package adoption

import (
	"context"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/pools"
	"pvmss/server/internal/store"
	"slices"
	"strings"
)

// PoolLister is the single cluster read Scan needs - the pools that exist
// cluster-side, including ones with no member VMs.
type PoolLister interface {
	ListPools(ctx context.Context) ([]cluster.Pool, error)
}

// Scan builds the orphan report for one cluster: pvmss-* pools missing from
// managed_pools, pvmss-tagged VMs outside any pvmss-* pool, and the tags
// those VMs carry that catalog_tags does not know. Returns
// pools.ErrProjectionNotReady when the inventory has not refreshed yet.
func Scan(ctx context.Context, lister PoolLister, projection *inventory.Projection, st *store.Store, clusterName string) (Report, error) {
	poolList, err := lister.ListPools(ctx)
	if err != nil {
		return Report{}, err
	}
	index := projection.Load()
	if index == nil {
		return Report{}, pools.ErrProjectionNotReady
	}
	managed, err := st.ManagedPoolNames(ctx, clusterName)
	if err != nil {
		return Report{}, err
	}
	knownTags, err := st.CatalogTags(ctx, clusterName)
	if err != nil {
		return Report{}, err
	}

	report := Report{Pools: []OrphanPool{}, LooseVMs: []VMRef{}, Tags: []OrphanTag{}}
	tagCounts := map[string]int{}

	for _, pool := range poolList {
		if !strings.HasPrefix(pool.Name, pools.PoolPrefix) {
			continue
		}
		if _, ok := managed[pool.Name]; ok {
			continue
		}
		members := index.ByPool[pool.Name]
		orphan := OrphanPool{Name: pool.Name, Comment: pool.Comment, Members: make([]VMRef, len(members))}
		for i, machine := range members {
			orphan.Members[i] = vmRef(machine)
			countUnknownTags(tagCounts, machine.Tags, knownTags)
		}
		report.Pools = append(report.Pools, orphan)
	}

	for _, machine := range index.ByVMID {
		if !slices.Contains(machine.Tags, catalog.ProtectedTagName) || strings.HasPrefix(machine.Pool, pools.PoolPrefix) {
			continue
		}
		report.LooseVMs = append(report.LooseVMs, vmRef(machine))
		countUnknownTags(tagCounts, machine.Tags, knownTags)
	}

	slices.SortFunc(report.Pools, func(a, b OrphanPool) int { return strings.Compare(a.Name, b.Name) })
	slices.SortFunc(report.LooseVMs, func(a, b VMRef) int { return a.VMID - b.VMID })

	names := make([]string, 0, len(tagCounts))
	for name := range tagCounts {
		names = append(names, name)
	}
	slices.Sort(names)
	for i, name := range names {
		report.Tags = append(report.Tags, OrphanTag{Name: name, VMCount: tagCounts[name], Color: palette[i%len(palette)]})
	}

	return report, nil
}

// countUnknownTags adds the VM's tags absent from the catalog (and not the
// protected pvmss marker) to the running per-tag count.
func countUnknownTags(counts map[string]int, tags []string, known []store.CatalogTag) {
	for _, tag := range tags {
		if tag == catalog.ProtectedTagName {
			continue
		}
		if slices.ContainsFunc(known, func(row store.CatalogTag) bool { return row.Name == tag }) {
			continue
		}
		counts[tag]++
	}
}
