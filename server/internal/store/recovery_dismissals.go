package store

import (
	"context"
	"fmt"
	"time"
)

// Recovery dismissal kinds stored in recovery_dismissals.kind.
const (
	RecoveryDismissalPool = "pool"
	RecoveryDismissalTag  = "tag"
	RecoveryDismissalVM   = "vm"
)

// RecoveryDismissal identifies one recovery report item the administrator
// dismissed: a pool or tag name, or (kind "vm") a VMID rendered as text.
type RecoveryDismissal struct {
	Kind string
	Item string
}

// RecoveryDismissals returns the dismissed (kind, item) pairs for a cluster
// as a set for O(1) membership checks during the adoption scan.
func (s *Store) RecoveryDismissals(ctx context.Context, cluster string) (map[RecoveryDismissal]struct{}, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT kind, item FROM recovery_dismissals WHERE cluster = ?`, cluster)
	if err != nil {
		return nil, fmt.Errorf("query recovery dismissals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[RecoveryDismissal]struct{})

	for rows.Next() {
		var row RecoveryDismissal
		if err := rows.Scan(&row.Kind, &row.Item); err != nil {
			return nil, fmt.Errorf("scan recovery dismissal: %w", err)
		}

		out[row] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recovery dismissals: %w", err)
	}

	return out, nil
}

// DismissRecoveryItems records items of one kind as hidden for the cluster.
// Idempotent: re-hiding an item refreshes dismissed_by/dismissed_at.
func (s *Store) DismissRecoveryItems(ctx context.Context, cluster, kind string, items []string, actor string) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin dismiss recovery items: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	at := time.Now().UTC().Format(time.RFC3339)

	for _, item := range items {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO recovery_dismissals (cluster, kind, item, dismissed_by, dismissed_at)
			 VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT(cluster, kind, item) DO UPDATE
			 SET dismissed_by = excluded.dismissed_by, dismissed_at = excluded.dismissed_at`,
			cluster, kind, item, actor, at); err != nil {
			return fmt.Errorf("dismiss recovery item: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit dismiss recovery items: %w", err)
	}

	return nil
}

// RestoreRecoveryItems removes dismissal rows so the items reappear in the
// adoption report. Restoring a non-dismissed item is a no-op.
func (s *Store) RestoreRecoveryItems(ctx context.Context, cluster, kind string, items []string) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin restore recovery items: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	for _, item := range items {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM recovery_dismissals WHERE cluster = ? AND kind = ? AND item = ?`,
			cluster, kind, item); err != nil {
			return fmt.Errorf("restore recovery item: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit restore recovery items: %w", err)
	}

	return nil
}
