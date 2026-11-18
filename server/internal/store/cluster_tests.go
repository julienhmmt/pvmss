package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// MaxClusterTestHistory is how many connection tests are kept per cluster.
const MaxClusterTestHistory = 100

// ClusterTest is one explicit connection test. Actor and DurationMs are
// empty/nil for rows backfilled from before history existed.
type ClusterTest struct {
	Cluster    string
	Status     string
	Message    string
	Version    string
	Actor      string
	DurationMs *int64
	TestedAt   time.Time
}

// RecordClusterTest updates the cluster's last-test columns and appends the
// test to its history, pruning beyond MaxClusterTestHistory, in one
// transaction. sql.ErrNoRows when the cluster is unknown or removed.
func (s *Store) RecordClusterTest(ctx context.Context, test ClusterTest) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cluster test result: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	testedAt := test.TestedAt.UTC().Format(time.RFC3339Nano)

	result, err := tx.ExecContext(ctx, `UPDATE clusters SET last_test_status = ?, last_test_at = ?, last_test_message = ?, proxmox_version = ? WHERE name = ? AND removed_at IS NULL`,
		test.Status, testedAt, nullableString(test.Message), nullableString(test.Version), test.Cluster)
	if err != nil {
		return fmt.Errorf("save cluster test result: %w", err)
	}

	if affected, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("count cluster test result: %w", err)
	} else if affected != 1 {
		return sql.ErrNoRows
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO cluster_test_results (cluster, tested_at, status, message, version, actor, duration_ms) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		test.Cluster, testedAt, test.Status, nullableString(test.Message), nullableString(test.Version), nullableString(test.Actor), test.DurationMs); err != nil {
		return fmt.Errorf("insert cluster test history: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM cluster_test_results WHERE cluster = ? AND id NOT IN (SELECT id FROM cluster_test_results WHERE cluster = ? ORDER BY tested_at DESC, id DESC LIMIT ?)`,
		test.Cluster, test.Cluster, MaxClusterTestHistory); err != nil {
		return fmt.Errorf("prune cluster test history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cluster test result: %w", err)
	}

	return nil
}

// ListClusterTests returns the newest tests of a cluster, newest first.
func (s *Store) ListClusterTests(ctx context.Context, name string, limit int) ([]ClusterTest, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT tested_at, status, message, version, actor, duration_ms FROM cluster_test_results WHERE cluster = ? ORDER BY tested_at DESC, id DESC LIMIT ?`, name, limit)
	if err != nil {
		return nil, fmt.Errorf("list cluster tests: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tests := []ClusterTest{}

	for rows.Next() {
		var (
			testedAt                string
			message, version, actor sql.NullString
			duration                sql.NullInt64
		)

		test := ClusterTest{Cluster: name}
		if err := rows.Scan(&testedAt, &test.Status, &message, &version, &actor, &duration); err != nil {
			return nil, fmt.Errorf("scan cluster test: %w", err)
		}

		parsed, err := time.Parse(time.RFC3339Nano, testedAt)
		if err != nil {
			return nil, fmt.Errorf("parse cluster test time: %w", err)
		}

		test.TestedAt = parsed

		test.Message, test.Version, test.Actor = message.String, version.String, actor.String
		if duration.Valid {
			test.DurationMs = &duration.Int64
		}

		tests = append(tests, test)
	}

	return tests, rows.Err()
}
