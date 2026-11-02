package policy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"pvmss/server/internal/store"
)

// validatePoolAllowance accepts -1 (unlimited) or a count up to the sanity
// bound. The same rule applies to the cluster default and to pool overrides.
func validatePoolAllowance(maxVMs int) error {
	if maxVMs < -1 || maxVMs > maxVMPerUserLimit {
		return fmt.Errorf("%w: maxVmPerUser must be between -1 and %d", ErrInvalidPolicy, maxVMPerUserLimit)
	}

	return nil
}

// poolAllowance resolves the allowance that applies to pool: its override when
// one exists, otherwise the cluster-wide quota. It also returns the override
// row (nil when there is none) so a caller can CAS against it.
func (service *Policy) poolAllowance(ctx context.Context, clusterName, pool string) (int, *store.PoolQuotaOverride, error) {
	override, err := service.store.PoolQuotaOverride(ctx, clusterName, pool)
	if err == nil {
		return override.MaxVMs, &override, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return 0, nil, err
	}

	row, err := service.policyRowOrDefault(ctx, clusterName)
	if err != nil {
		return 0, nil, err
	}

	return row.MaxVMPerUser, nil, nil
}

// PoolAllowance returns the VM allowance that applies to pool on the cluster.
func (service *Policy) PoolAllowance(ctx context.Context, clusterName, pool string) (int, error) {
	allowed, _, err := service.poolAllowance(ctx, clusterName, pool)

	return allowed, err
}

// SetPoolQuotaOverride sets the absolute VM allowance of one pool, replacing
// the cluster-wide quota for that pool. actor is recorded as the last writer.
func (service *Policy) SetPoolQuotaOverride(ctx context.Context, clusterName, pool string, maxVMs int, actor string) error {
	if err := validatePoolAllowance(maxVMs); err != nil {
		return err
	}

	return service.store.UpsertPoolQuotaOverride(ctx, store.PoolQuotaOverride{
		Cluster: clusterName, Pool: pool, MaxVMs: maxVMs, UpdatedBy: actor,
	})
}

// DeletePoolQuotaOverride removes a pool's override so it falls back to the
// cluster-wide quota. Returns sql.ErrNoRows when the pool has none.
func (service *Policy) DeletePoolQuotaOverride(ctx context.Context, clusterName, pool string) error {
	return service.store.DeletePoolQuotaOverride(ctx, clusterName, pool)
}
