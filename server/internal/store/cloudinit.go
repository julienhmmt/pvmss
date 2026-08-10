package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const schemaV8 = `CREATE TABLE vm_cloudinit_snippets (
	cluster    TEXT NOT NULL,
	vmid       INTEGER NOT NULL,
	content    TEXT NOT NULL,
	storage    TEXT NOT NULL,
	filename   TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	updated_by TEXT NOT NULL,
	PRIMARY KEY (cluster, vmid)
)`

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
//
//nolint:wsl_v5 // query and timestamp parsing form one persistence boundary
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

// PutCloudInitSnippet upserts one snippet row. Empty content is retained to represent explicit clear.
//
//nolint:wsl_v5 // SQL arguments and upsert belong to one atomic repository operation
func (s *Store) PutCloudInitSnippet(ctx context.Context, cluster string, vmid int, storage, filename, content, actor string) error {
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO vm_cloudinit_snippets
			(cluster, vmid, content, storage, filename, updated_at, updated_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(cluster, vmid) DO UPDATE SET
			content = excluded.content,
			storage = excluded.storage,
			filename = excluded.filename,
			updated_at = excluded.updated_at,
			updated_by = excluded.updated_by`,
		cluster, vmid, content, storage, filename, stamp, actor,
	)
	if err != nil {
		return fmt.Errorf("upsert cloud-init snippet: %w", err)
	}

	return nil
}
