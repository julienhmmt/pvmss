package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// BaselineState is the persisted delivery state of the generated cloud-init
// baseline for an image-mode VM (cloud-image-console issue 03).
type BaselineState struct {
	Cluster   string
	VMID      int
	State     string // "applied", "override", "not_delivered"
	Error     string // reason when State is "not_delivered"
	UpdatedAt time.Time
}

// GetBaselineState returns found=false when no row exists.
func (s *Store) GetBaselineState(ctx context.Context, cluster string, vmid int) (BaselineState, bool, error) {
	var (
		state BaselineState
		stamp string
	)

	err := s.db.QueryRowContext(ctx,
		`SELECT cluster, vmid, state, error, updated_at
		 FROM vm_baseline_state WHERE cluster = ? AND vmid = ?`,
		cluster, vmid,
	).Scan(
		&state.Cluster,
		&state.VMID,
		&state.State,
		&state.Error,
		&stamp,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return BaselineState{}, false, nil
	}

	if err != nil {
		return BaselineState{}, false, fmt.Errorf("query baseline state: %w", err)
	}

	state.UpdatedAt, err = time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return BaselineState{}, false, fmt.Errorf("parse baseline state timestamp: %w", err)
	}

	return state, true, nil
}

// PutBaselineState upserts the baseline delivery state for one VM.
func (s *Store) PutBaselineState(ctx context.Context, cluster string, vmid int, state, errMsg string) error {
	stamp := time.Now().UTC().Format(time.RFC3339Nano)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO vm_baseline_state
			(cluster, vmid, state, error, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(cluster, vmid) DO UPDATE SET
			state = excluded.state,
			error = excluded.error,
			updated_at = excluded.updated_at`,
		cluster, vmid, state, errMsg, stamp,
	)
	if err != nil {
		return fmt.Errorf("upsert baseline state: %w", err)
	}

	return nil
}
