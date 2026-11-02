package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// PoolQuotaOverride is one pool_quota_overrides row: a per-pool VM allowance
// that replaces the cluster-wide quota for that pool. MaxVMs -1 means unlimited.
type PoolQuotaOverride struct {
	Cluster   string
	Pool      string
	MaxVMs    int
	UpdatedBy string
	UpdatedAt string
}

const poolQuotaOverrideColumns = `cluster, pool, max_vms, updated_by, updated_at`

// ListPoolQuotaOverrides returns the overrides of one cluster, or of every
// cluster when cluster is empty, ordered by cluster then pool.
func (s *Store) ListPoolQuotaOverrides(ctx context.Context, cluster string) ([]PoolQuotaOverride, error) {
	query := `SELECT ` + poolQuotaOverrideColumns + ` FROM pool_quota_overrides`
	args := []any{}

	if cluster != "" {
		query += ` WHERE cluster = ?`

		args = append(args, cluster)
	}

	rows, err := s.db.QueryContext(ctx, query+` ORDER BY cluster, pool`, args...)
	if err != nil {
		return nil, fmt.Errorf("query pool quota overrides: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var out []PoolQuotaOverride

	for rows.Next() {
		var row PoolQuotaOverride
		if err := rows.Scan(&row.Cluster, &row.Pool, &row.MaxVMs, &row.UpdatedBy, &row.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan pool quota override: %w", err)
		}

		out = append(out, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pool quota overrides: %w", err)
	}

	return out, nil
}

// PoolQuotaOverride returns the override for one pool, or sql.ErrNoRows.
func (s *Store) PoolQuotaOverride(ctx context.Context, cluster, pool string) (PoolQuotaOverride, error) {
	var row PoolQuotaOverride

	err := s.db.QueryRowContext(ctx,
		`SELECT `+poolQuotaOverrideColumns+` FROM pool_quota_overrides WHERE cluster = ? AND pool = ?`,
		cluster, pool).Scan(&row.Cluster, &row.Pool, &row.MaxVMs, &row.UpdatedBy, &row.UpdatedAt)
	if err != nil {
		return PoolQuotaOverride{}, fmt.Errorf("query pool quota override: %w", err)
	}

	return row, nil
}

// UpsertPoolQuotaOverride sets the pool's override and stamps updated_at now.
func (s *Store) UpsertPoolQuotaOverride(ctx context.Context, row PoolQuotaOverride) error {
	_, err := s.db.ExecContext(ctx, poolQuotaUpsertSQL, row.Cluster, row.Pool, row.MaxVMs, row.UpdatedBy, nowRFC3339())
	if err != nil {
		return fmt.Errorf("upsert pool quota override: %w", err)
	}

	return nil
}

const poolQuotaUpsertSQL = `INSERT INTO pool_quota_overrides (cluster, pool, max_vms, updated_by, updated_at)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(cluster, pool) DO UPDATE SET
		max_vms = excluded.max_vms,
		updated_by = excluded.updated_by,
		updated_at = excluded.updated_at`

// DeletePoolQuotaOverride removes the pool's override. Returns sql.ErrNoRows
// when the pool has none.
func (s *Store) DeletePoolQuotaOverride(ctx context.Context, cluster, pool string) error {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM pool_quota_overrides WHERE cluster = ? AND pool = ?`, cluster, pool)
	if err != nil {
		return fmt.Errorf("delete pool quota override: %w", err)
	}

	if n, _ := result.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// replacePoolQuotaOverride writes next only when the stored override still
// equals expected (max_vms and updated_at). A nil expected means no override
// may exist yet. Either guard failing returns ErrPolicyConflict so callers
// retry the read-modify-write cycle, as the policy row CAS does.
func replacePoolQuotaOverride(ctx context.Context, db execer, expected *PoolQuotaOverride, next PoolQuotaOverride) error {
	now := nowRFC3339()

	if expected == nil {
		res, err := db.ExecContext(ctx,
			`INSERT INTO pool_quota_overrides (cluster, pool, max_vms, updated_by, updated_at)
			 VALUES (?, ?, ?, ?, ?) ON CONFLICT(cluster, pool) DO NOTHING`,
			next.Cluster, next.Pool, next.MaxVMs, next.UpdatedBy, now)
		if err != nil {
			return fmt.Errorf("insert pool quota override: %w", err)
		}

		if n, _ := res.RowsAffected(); n == 0 {
			return ErrPolicyConflict
		}

		return nil
	}

	res, err := db.ExecContext(ctx,
		`UPDATE pool_quota_overrides SET max_vms = ?, updated_by = ?, updated_at = ?
		 WHERE cluster = ? AND pool = ? AND max_vms = ? AND updated_at = ?`,
		next.MaxVMs, next.UpdatedBy, now, expected.Cluster, expected.Pool, expected.MaxVMs, expected.UpdatedAt)
	if err != nil {
		return fmt.Errorf("replace pool quota override: %w", err)
	}

	if n, _ := res.RowsAffected(); n == 0 {
		return ErrPolicyConflict
	}

	return nil
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
