package adoption

import (
	"context"
	"errors"
	"fmt"
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

	orphanPools, suggestedColors := orphanLookup(report)

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

	if err := clearAdoptedDismissals(ctx, st, clusterName, sel); err != nil {
		return Result{}, err
	}

	return result, nil
}

// orphanLookup indexes the report for selection filtering. Hidden items
// stay adoptable - the admin can adopt them straight from the hidden
// section - so both partitions feed the lookup maps.
func orphanLookup(report Report) (map[string]struct{}, map[string]string) {
	orphanPools := make(map[string]struct{}, len(report.Pools)+len(report.HiddenPools))
	for _, pool := range report.Pools {
		orphanPools[pool.Name] = struct{}{}
	}

	for _, pool := range report.HiddenPools {
		orphanPools[pool.Name] = struct{}{}
	}

	suggestedColors := make(map[string]string, len(report.Tags)+len(report.HiddenTags))
	for _, tag := range report.Tags {
		suggestedColors[tag.Name] = tag.Color
	}

	for _, tag := range report.HiddenTags {
		suggestedColors[tag.Name] = tag.Color
	}

	return orphanPools, suggestedColors
}

// clearAdoptedDismissals drops dismissal rows for the adopted items: the
// triage decision is spent, so a pool or tag that becomes an orphan again
// later resurfaces in the visible report instead of staying hidden.
func clearAdoptedDismissals(ctx context.Context, st *store.Store, clusterName string, sel Selection) error {
	if err := st.RestoreRecoveryItems(ctx, clusterName, store.RecoveryDismissalPool, sel.Pools); err != nil {
		return fmt.Errorf("restore adopted pools: %w", err)
	}

	if err := st.RestoreRecoveryItems(ctx, clusterName, store.RecoveryDismissalTag, sel.Tags); err != nil {
		return fmt.Errorf("restore adopted tags: %w", err)
	}

	return nil
}
