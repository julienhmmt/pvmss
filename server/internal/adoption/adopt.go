package adoption

import (
	"context"
	"errors"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
)

// Adopt re-registers the selected orphans for one cluster: pools are marked
// managed again and tags are recreated in catalog_tags with their suggested
// palette color. The selection is filtered against a fresh Scan, so only
// genuine orphans are touched and re-running it is a no-op. Names that are
// not current orphans are ignored rather than erroring.
func Adopt(ctx context.Context, lister PoolLister, projection *inventory.Projection, st *store.Store, clusterName string, sel Selection) (Result, error) {
	report, err := Scan(ctx, lister, projection, st, clusterName)
	if err != nil {
		return Result{}, err
	}

	orphanPools := make(map[string]struct{}, len(report.Pools))
	for _, pool := range report.Pools {
		orphanPools[pool.Name] = struct{}{}
	}
	suggestedColors := make(map[string]string, len(report.Tags))
	for _, tag := range report.Tags {
		suggestedColors[tag.Name] = tag.Color
	}

	result := Result{Pools: []string{}, Tags: []AdoptedTag{}}

	for _, name := range sel.Pools {
		if _, ok := orphanPools[name]; !ok {
			continue
		}
		if err := st.RegisterManagedPool(ctx, clusterName, name); err != nil {
			return Result{}, err
		}
		result.Pools = append(result.Pools, name)
	}

	for _, name := range sel.Tags {
		color, ok := suggestedColors[name]
		if !ok {
			continue
		}
		if _, err := catalog.CreateTag(ctx, st, clusterName, name, color); err != nil && !errors.Is(err, catalog.ErrDuplicateTag) {
			return Result{}, err
		}
		result.Tags = append(result.Tags, AdoptedTag{Name: name, Color: color})
	}

	return result, nil
}
