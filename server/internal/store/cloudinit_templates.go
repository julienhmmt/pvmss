package store

import (
	"context"
	"database/sql"
	"fmt"
)

// CatalogCloudInitTemplate is one catalog_cloudinit_templates row with its
// enabled state and full content (T18).
type CatalogCloudInitTemplate struct {
	ID        string
	Label     string
	Content   string
	Enabled   bool
	CreatedAt string
	UpdatedAt string
}

// CatalogCloudInitTemplatesAll returns every cloud-init template row for a
// cluster (including disabled), ordered by id — the admin list's data source.
func (s *Store) CatalogCloudInitTemplatesAll(ctx context.Context, cluster string) ([]CatalogCloudInitTemplate, error) {
	return queryCatalog(ctx, s.db, "catalog cloudinit templates all",
		`SELECT id, label, content, enabled, created_at, updated_at FROM catalog_cloudinit_templates WHERE cluster = ? ORDER BY id`,
		[]any{cluster},
		func(rows *sql.Rows) (CatalogCloudInitTemplate, error) {
			var t CatalogCloudInitTemplate
			return t, rows.Scan(&t.ID, &t.Label, &t.Content, &t.Enabled, &t.CreatedAt, &t.UpdatedAt)
		},
	)
}

// CatalogCloudInitTemplatesEnabled returns only enabled cloud-init template
// rows for a cluster, ordered by id — T06's catalog reader's data source.
func (s *Store) CatalogCloudInitTemplatesEnabled(ctx context.Context, cluster string) ([]CatalogCloudInitTemplate, error) {
	return queryCatalog(ctx, s.db, "catalog cloudinit templates enabled",
		`SELECT id, label, content, enabled, created_at, updated_at FROM catalog_cloudinit_templates WHERE cluster = ? AND enabled = 1 ORDER BY id`,
		[]any{cluster},
		func(rows *sql.Rows) (CatalogCloudInitTemplate, error) {
			var t CatalogCloudInitTemplate
			return t, rows.Scan(&t.ID, &t.Label, &t.Content, &t.Enabled, &t.CreatedAt, &t.UpdatedAt)
		},
	)
}

// InsertCloudInitTemplate inserts a new template row. Returns ErrDuplicate if
// the id already exists for the cluster (ON CONFLICT DO NOTHING guards against
// a concurrent insert between the catalog layer's existence check and this
// INSERT).
func (s *Store) InsertCloudInitTemplate(ctx context.Context, cluster, id, label, content, createdAt, updatedAt string) error {
	return execInsertOne(ctx, s.db,
		`INSERT INTO catalog_cloudinit_templates (cluster, id, label, content, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, 1, ?, ?)
		 ON CONFLICT(cluster, id) DO NOTHING`,
		[]any{cluster, id, label, content, createdAt, updatedAt},
	)
}

// UpdateCloudInitTemplate updates an existing template's label and content.
// Returns sql.ErrNoRows if the template does not exist.
func (s *Store) UpdateCloudInitTemplate(ctx context.Context, cluster, id, label, content, updatedAt string) error {
	return execUpdateOne(ctx, s.db,
		`UPDATE catalog_cloudinit_templates SET label = ?, content = ?, updated_at = ? WHERE cluster = ? AND id = ?`,
		[]any{label, content, updatedAt, cluster, id},
	)
}

// DeleteCloudInitTemplate removes a template row. Returns sql.ErrNoRows if the
// template did not exist. No cascade (FR-009).
func (s *Store) DeleteCloudInitTemplate(ctx context.Context, cluster, id string) error {
	return execUpdateOne(ctx, s.db,
		`DELETE FROM catalog_cloudinit_templates WHERE cluster = ? AND id = ?`,
		[]any{cluster, id},
	)
}

// SetCloudInitTemplateEnabled updates the enabled state for one template.
// Returns sql.ErrNoRows if the template does not exist — a toggle is an upsert
// on the enabled column, never a delete.
func (s *Store) SetCloudInitTemplateEnabled(ctx context.Context, cluster, id string, enabled bool, updatedAt string) error {
	return execUpdateOne(ctx, s.db,
		`UPDATE catalog_cloudinit_templates SET enabled = ?, updated_at = ? WHERE cluster = ? AND id = ?`,
		[]any{enabled, updatedAt, cluster, id},
	)
}

// CloudInitTemplateExists checks whether a template with the given id exists
// for the cluster (regardless of enabled state).
func (s *Store) CloudInitTemplateExists(ctx context.Context, cluster, id string) (bool, error) {
	var count int

	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM catalog_cloudinit_templates WHERE cluster = ? AND id = ?`,
		cluster, id).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("query cloudinit template exists: %w", err)
	}

	return count > 0, nil
}
