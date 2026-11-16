package adoption

import (
	"context"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/pools"
	"pvmss/server/internal/store"
	"slices"
	"strconv"
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

	report := Report{Pools: []OrphanPool{}, LooseVMs: []VMRef{}, Tags: []OrphanTag{}, HiddenPools: []HiddenPool{}, HiddenLooseVMs: []HiddenLooseVM{}, HiddenTags: []HiddenTag{}}
	tagCounts := map[string]int{}

	report.Pools = orphanPools(poolList, index, managed, knownTags, tagCounts)

	for _, machine := range index.ByVMID {
		if !slices.Contains(machine.Tags, catalog.ProtectedTagName) || strings.HasPrefix(machine.Pool, pools.PoolPrefix) {
			continue
		}

		report.LooseVMs = append(report.LooseVMs, vmRef(machine))
		countUnknownTags(tagCounts, machine.Tags, knownTags)
	}

	slices.SortFunc(report.Pools, func(a, b OrphanPool) int { return strings.Compare(a.Name, b.Name) })
	slices.SortFunc(report.LooseVMs, func(a, b VMRef) int { return a.VMID - b.VMID })

	report.Tags = orphanTags(tagCounts)

	dismissed, err := st.RecoveryDismissals(ctx, clusterName)
	if err != nil {
		return Report{}, err
	}

	// Partition after tag counting: members of hidden pools still count -
	// the tag list reflects what exists cluster-side.
	report.Pools, report.HiddenPools = splitHiddenPools(report.Pools, dismissed)
	report.LooseVMs, report.HiddenLooseVMs = splitHiddenLooseVMs(report.LooseVMs, dismissed)
	report.Tags, report.HiddenTags = splitHiddenTags(report.Tags, dismissed)

	return report, nil
}

// orphanPools returns the cluster's unmanaged pvmss-* pools with their member
// VMs, counting the members' unknown tags on the way.
func orphanPools(poolList []cluster.Pool, index *inventory.Index, managed map[string]struct{}, knownTags []store.CatalogTag, tagCounts map[string]int) []OrphanPool {
	orphans := []OrphanPool{}

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

		orphans = append(orphans, orphan)
	}

	return orphans
}

// splitHiddenPools moves dismissed pools from the visible list to hidden
// summaries. Both lists keep the input order (already name-sorted).
func splitHiddenPools(pools []OrphanPool, dismissed map[store.RecoveryDismissal]struct{}) ([]OrphanPool, []HiddenPool) {
	visible := []OrphanPool{}
	hidden := []HiddenPool{}

	for _, pool := range pools {
		if _, ok := dismissed[store.RecoveryDismissal{Kind: store.RecoveryDismissalPool, Item: pool.Name}]; ok {
			hidden = append(hidden, HiddenPool{Name: pool.Name, Comment: pool.Comment, MemberCount: len(pool.Members)})
			continue
		}

		visible = append(visible, pool)
	}

	return visible, hidden
}

// splitHiddenLooseVMs moves dismissed loose VMs (matched by vmid) to the
// hidden list.
func splitHiddenLooseVMs(vms []VMRef, dismissed map[store.RecoveryDismissal]struct{}) ([]VMRef, []HiddenLooseVM) {
	visible := []VMRef{}
	hidden := []HiddenLooseVM{}

	for _, machine := range vms {
		if _, ok := dismissed[store.RecoveryDismissal{Kind: store.RecoveryDismissalVM, Item: strconv.Itoa(machine.VMID)}]; ok {
			hidden = append(hidden, HiddenLooseVM{VMID: machine.VMID, Name: machine.Name})
			continue
		}

		visible = append(visible, machine)
	}

	return visible, hidden
}

// splitHiddenTags moves dismissed tags to the hidden list; the suggested
// color is kept so a hidden tag can still be adopted.
func splitHiddenTags(tags []OrphanTag, dismissed map[store.RecoveryDismissal]struct{}) ([]OrphanTag, []HiddenTag) {
	visible := []OrphanTag{}
	hidden := []HiddenTag{}

	for _, tag := range tags {
		if _, ok := dismissed[store.RecoveryDismissal{Kind: store.RecoveryDismissalTag, Item: tag.Name}]; ok {
			hidden = append(hidden, tag)
			continue
		}

		visible = append(visible, tag)
	}

	return visible, hidden
}

// orphanTags builds the sorted tag list from the accumulated counts, giving
// each tag a palette color by position.
func orphanTags(tagCounts map[string]int) []OrphanTag {
	names := make([]string, 0, len(tagCounts))
	for name := range tagCounts {
		names = append(names, name)
	}

	slices.Sort(names)

	tags := make([]OrphanTag, 0, len(names))
	for i, name := range names {
		tags = append(tags, OrphanTag{Name: name, VMCount: tagCounts[name], Color: palette[i%len(palette)]})
	}

	return tags
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
