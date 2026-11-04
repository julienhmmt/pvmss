package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SnapshotSchedule is a VM's automatic snapshot policy. LastRunAt is the zero
// time until the sweeper has taken the first snapshot.
type SnapshotSchedule struct {
	Cluster   string
	VMID      int
	Pool      string
	Frequency string
	Retain    int
	LastRunAt time.Time
	// LastFailure is the failure code of the latest attempt, empty after a
	// success. LastFailureAt is when it happened.
	LastFailure   string
	LastFailureAt time.Time
}

// GetSnapshotSchedule returns found=false when the VM has no schedule row.
func (s *Store) GetSnapshotSchedule(ctx context.Context, cluster string, vmid int) (SnapshotSchedule, bool, error) {
	var (
		schedule      SnapshotSchedule
		lastRun       string
		lastFailureAt string
	)

	err := s.db.QueryRowContext(ctx,
		`SELECT cluster, vmid, pool, frequency, retain, last_run_at, last_failure, last_failure_at FROM vm_snapshot_schedules WHERE cluster = ? AND vmid = ?`,
		cluster, vmid,
	).Scan(&schedule.Cluster, &schedule.VMID, &schedule.Pool, &schedule.Frequency, &schedule.Retain, &lastRun, &schedule.LastFailure, &lastFailureAt)
	if errors.Is(err, sql.ErrNoRows) {
		return SnapshotSchedule{}, false, nil
	}

	if err != nil {
		return SnapshotSchedule{}, false, fmt.Errorf("query snapshot schedule: %w", err)
	}

	schedule.LastRunAt, err = parseScheduleTime(lastRun)
	if err != nil {
		return SnapshotSchedule{}, false, fmt.Errorf("parse snapshot schedule last run: %w", err)
	}

	schedule.LastFailureAt, err = parseScheduleTime(lastFailureAt)
	if err != nil {
		return SnapshotSchedule{}, false, fmt.Errorf("parse snapshot schedule last failure: %w", err)
	}

	return schedule, true, nil
}

// ListSnapshotSchedules returns every schedule row, for the sweeper.
func (s *Store) ListSnapshotSchedules(ctx context.Context) ([]SnapshotSchedule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT cluster, vmid, pool, frequency, retain, last_run_at, last_failure, last_failure_at FROM vm_snapshot_schedules ORDER BY cluster, vmid`,
	)
	if err != nil {
		return nil, fmt.Errorf("list snapshot schedules: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var schedules []SnapshotSchedule

	for rows.Next() {
		var (
			schedule      SnapshotSchedule
			lastRun       string
			lastFailureAt string
		)

		if err := rows.Scan(&schedule.Cluster, &schedule.VMID, &schedule.Pool, &schedule.Frequency, &schedule.Retain, &lastRun, &schedule.LastFailure, &lastFailureAt); err != nil {
			return nil, fmt.Errorf("scan snapshot schedule: %w", err)
		}

		schedule.LastRunAt, err = parseScheduleTime(lastRun)
		if err != nil {
			return nil, fmt.Errorf("parse snapshot schedule last run: %w", err)
		}

		schedule.LastFailureAt, err = parseScheduleTime(lastFailureAt)
		if err != nil {
			return nil, fmt.Errorf("parse snapshot schedule last failure: %w", err)
		}

		schedules = append(schedules, schedule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate snapshot schedules: %w", err)
	}

	return schedules, nil
}

// UpsertSnapshotSchedule stores the frequency, retention and owning pool for a
// VM. The pool pins the schedule to the VM it was set on: a VMID reused in
// another pool no longer matches. It keeps the last run time, so changing the
// policy does not re-trigger a snapshot.
func (s *Store) UpsertSnapshotSchedule(ctx context.Context, cluster string, vmid int, pool, frequency string, retain int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO vm_snapshot_schedules (cluster, vmid, pool, frequency, retain) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (cluster, vmid) DO UPDATE SET pool = excluded.pool, frequency = excluded.frequency, retain = excluded.retain`,
		cluster, vmid, pool, frequency, retain,
	)
	if err != nil {
		return fmt.Errorf("upsert snapshot schedule: %w", err)
	}

	return nil
}

// MarkSnapshotScheduleRun records the time of the last automatic snapshot and
// clears any earlier failure.
func (s *Store) MarkSnapshotScheduleRun(ctx context.Context, cluster string, vmid int, at time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE vm_snapshot_schedules SET last_run_at = ?, last_failure = '', last_failure_at = '' WHERE cluster = ? AND vmid = ?`,
		at.UTC().Format(time.RFC3339Nano), cluster, vmid,
	)
	if err != nil {
		return fmt.Errorf("mark snapshot schedule run: %w", err)
	}

	return nil
}

// MarkSnapshotScheduleFailure records why the latest attempt failed.
func (s *Store) MarkSnapshotScheduleFailure(ctx context.Context, cluster string, vmid int, failure string, at time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE vm_snapshot_schedules SET last_failure = ?, last_failure_at = ? WHERE cluster = ? AND vmid = ?`,
		failure, at.UTC().Format(time.RFC3339Nano), cluster, vmid,
	)
	if err != nil {
		return fmt.Errorf("mark snapshot schedule failure: %w", err)
	}

	return nil
}

// DeleteSnapshotSchedule removes a VM's schedule. No row is not an error.
func (s *Store) DeleteSnapshotSchedule(ctx context.Context, cluster string, vmid int) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM vm_snapshot_schedules WHERE cluster = ? AND vmid = ?`,
		cluster, vmid,
	)
	if err != nil {
		return fmt.Errorf("delete snapshot schedule: %w", err)
	}

	return nil
}

func parseScheduleTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}

	return time.Parse(time.RFC3339Nano, value)
}
