package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CloudInitSnippet is the persisted custom cloud-init document and its server-owned target.
type CloudInitSnippet struct {
	Cluster   string
	VMID      int
	Content   string
	Storage   string
	Filename  string
	UpdatedAt time.Time
	UpdatedBy string
}

// GetCloudInitSnippet returns found=false when no snippet row exists.
func (s *Store) GetCloudInitSnippet(ctx context.Context, cluster string, vmid int) (CloudInitSnippet, bool, error) {
	var (
		snippet CloudInitSnippet
		stamp   string
	)

	err := s.db.QueryRowContext(ctx,
		`SELECT cluster, vmid, content, storage, filename, updated_at, updated_by
		 FROM vm_cloudinit_snippets WHERE cluster = ? AND vmid = ?`,
		cluster, vmid,
	).Scan(
		&snippet.Cluster,
		&snippet.VMID,
		&snippet.Content,
		&snippet.Storage,
		&snippet.Filename,
		&stamp,
		&snippet.UpdatedBy,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return CloudInitSnippet{}, false, nil
	}

	if err != nil {
		return CloudInitSnippet{}, false, fmt.Errorf("query cloud-init snippet: %w", err)
	}

	snippet.UpdatedAt, err = time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return CloudInitSnippet{}, false, fmt.Errorf("parse cloud-init snippet timestamp: %w", err)
	}

	return snippet, true, nil
}

// DeleteCloudInitSnippet removes the VM's document row. No row is not an error.
func (s *Store) DeleteCloudInitSnippet(ctx context.Context, cluster string, vmid int) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM vm_cloudinit_snippets WHERE cluster = ? AND vmid = ?`,
		cluster, vmid,
	)
	if err != nil {
		return fmt.Errorf("delete cloud-init snippet: %w", err)
	}

	return nil
}
