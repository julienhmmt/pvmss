package store

import (
	"context"
	"fmt"
	"time"
)

// ManagedPool is one managed_pools row: a pool PVMSS provisioned and may delete.
type ManagedPool struct {
	Cluster   string
	Name      string
	CreatedAt string
}

// RegisterManagedPool records that PVMSS created the named pool on the cluster.
// Idempotent: re-registering an already-managed pool refreshes created_at.
func (s *Store) RegisterManagedPool(ctx context.Context, cluster, name string) error {
	return execWrite(ctx, s.db,
		`INSERT INTO managed_pools (cluster, name, created_at) VALUES (?, ?, ?)
		 ON CONFLICT(cluster, name) DO UPDATE SET created_at = excluded.created_at`,
		[]any{cluster, name, time.Now().UTC().Format(time.RFC3339)},
	)
}

// UnregisterManagedPool removes the managed marker. Returns sql.ErrNoRows if
// the pool was not managed — guards against a delete between the managed check
// and this DELETE.
func (s *Store) UnregisterManagedPool(ctx context.Context, cluster, name string) error {
	return execUpdateOne(ctx, s.db,
		`DELETE FROM managed_pools WHERE cluster = ? AND name = ?`,
		[]any{cluster, name},
	)
}

// IsPoolManaged reports whether PVMSS recorded the named pool on the cluster.
func (s *Store) IsPoolManaged(ctx context.Context, cluster, name string) (bool, error) {
	var count int

	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM managed_pools WHERE cluster = ? AND name = ?`,
		cluster, name).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("query managed pool: %w", err)
	}

	return count > 0, nil
}

// ManagedPools returns all managed pool names for the cluster.
func (s *Store) ManagedPools(ctx context.Context, cluster string) ([]ManagedPool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT cluster, name, created_at FROM managed_pools WHERE cluster = ? ORDER BY name`,
		cluster)
	if err != nil {
		return nil, fmt.Errorf("query managed pools: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []ManagedPool

	for rows.Next() {
		var row ManagedPool
		if err := rows.Scan(&row.Cluster, &row.Name, &row.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan managed pool: %w", err)
		}

		out = append(out, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate managed pools: %w", err)
	}

	return out, nil
}

// ManagedPoolNames returns the set of managed pool names for the cluster as a
// map for O(1) membership checks during list projection.
func (s *Store) ManagedPoolNames(ctx context.Context, cluster string) (map[string]struct{}, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name FROM managed_pools WHERE cluster = ?`,
		cluster)
	if err != nil {
		return nil, fmt.Errorf("query managed pool names: %w", err)
	}
	defer func() { _ = rows.Close() }()

	names := make(map[string]struct{})

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan managed pool name: %w", err)
		}

		names[name] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate managed pool names: %w", err)
	}

	return names, nil
}
