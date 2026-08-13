//nolint:wsl_v5 // parallel reader methods keep the scan closure and query adjacent
package store

import (
	"context"
	"database/sql"
	"fmt"
)

// DocumentationPageRow is one documentation_pages row (issue #53). The same
// struct is returned by both the all-rows (admin) and enabled-only (public)
// readers; each audience's handler calls the reader appropriate to its own
// audience, mirroring T18's cloud-init template split.
type DocumentationPageRow struct {
	ID        string
	Lang      string
	Title     string
	Category  string
	BodyMD    string
	Audience  string
	Enabled   bool
	IsSystem  bool
	SortOrder int
	CreatedAt string
	UpdatedAt string
}

// DocumentationPagesAll returns every documentation page row (including
// disabled ones), ordered by sort_order then title — the admin list's data
// source.
//
//nolint:dupl // intentionally parallel to DocumentationPagesEnabled (same shape, different filter)
func (s *Store) DocumentationPagesAll(ctx context.Context) ([]DocumentationPageRow, error) {
	return queryCatalog(ctx, s.db, "documentation pages all",
		`SELECT id, lang, title, category, body_md, audience, enabled, is_system, sort_order, created_at, updated_at
		 FROM documentation_pages ORDER BY sort_order, title`,
		nil,
		func(rows *sql.Rows) (DocumentationPageRow, error) {
			var p DocumentationPageRow
			return p, rows.Scan(&p.ID, &p.Lang, &p.Title, &p.Category, &p.BodyMD,
				&p.Audience, &p.Enabled, &p.IsSystem, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt)
		},
	)
}

// DocumentationPagesEnabled returns only enabled documentation page rows,
// ordered by sort_order then title — the public reader's data source.
//
//nolint:dupl // intentionally parallel to DocumentationPagesAll (same shape, different filter)
func (s *Store) DocumentationPagesEnabled(ctx context.Context) ([]DocumentationPageRow, error) {
	return queryCatalog(ctx, s.db, "documentation pages enabled",
		`SELECT id, lang, title, category, body_md, audience, enabled, is_system, sort_order, created_at, updated_at
		 FROM documentation_pages WHERE enabled = 1 ORDER BY sort_order, title`,
		nil,
		func(rows *sql.Rows) (DocumentationPageRow, error) {
			var p DocumentationPageRow
			return p, rows.Scan(&p.ID, &p.Lang, &p.Title, &p.Category, &p.BodyMD,
				&p.Audience, &p.Enabled, &p.IsSystem, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt)
		},
	)
}

// GetDocumentationPage returns a single row by (id, lang) or sql.ErrNoRows when
// absent — the catalog layer's en-fallback lookup uses this directly.
func (s *Store) GetDocumentationPage(ctx context.Context, id, lang string) (DocumentationPageRow, error) {
	var p DocumentationPageRow
	err := s.db.QueryRowContext(ctx,
		`SELECT id, lang, title, category, body_md, audience, enabled, is_system, sort_order, created_at, updated_at
		 FROM documentation_pages WHERE id = ? AND lang = ?`,
		id, lang,
	).Scan(&p.ID, &p.Lang, &p.Title, &p.Category, &p.BodyMD,
		&p.Audience, &p.Enabled, &p.IsSystem, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return DocumentationPageRow{}, fmt.Errorf("query documentation page: %w", err)
	}

	return p, nil
}

// InsertDocumentationPage inserts a new page row. Returns ErrDuplicate if the
// (id, lang) pair already exists (ON CONFLICT DO NOTHING guards against a
// concurrent insert between the catalog layer's existence check and this
// INSERT).
func (s *Store) InsertDocumentationPage(ctx context.Context, p DocumentationPageRow) error {
	return execInsertOne(ctx, s.db,
		`INSERT INTO documentation_pages (id, lang, title, category, body_md, audience, enabled, is_system, sort_order, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id, lang) DO NOTHING`,
		[]any{p.ID, p.Lang, p.Title, p.Category, p.BodyMD, p.Audience, p.Enabled, p.IsSystem, p.SortOrder, p.CreatedAt, p.UpdatedAt},
	)
}

// UpdateDocumentationPage updates an existing page's mutable fields. The id,
// lang, and is_system columns are never changed here — the catalog layer
// refuses id/lang changes and is_system edits before calling this. Returns
// sql.ErrNoRows if the row does not exist.
func (s *Store) UpdateDocumentationPage(ctx context.Context, id, lang, title, category, bodyMD, audience string, enabled bool, sortOrder int, updatedAt string) error {
	return execUpdateOne(ctx, s.db,
		`UPDATE documentation_pages SET title = ?, category = ?, body_md = ?, audience = ?, enabled = ?, sort_order = ?, updated_at = ?
		 WHERE id = ? AND lang = ? AND is_system = 0`,
		[]any{title, category, bodyMD, audience, enabled, sortOrder, updatedAt, id, lang},
	)
}

// UpdateSystemDocumentationPage updates a built-in (is_system=1) page's mutable
// fields. System pages may be edited (title/category/body/audience/enabled/
// sort_order) but never deleted or have their id/lang changed. Returns
// sql.ErrNoRows if the row does not exist.
func (s *Store) UpdateSystemDocumentationPage(ctx context.Context, id, lang, title, category, bodyMD, audience string, enabled bool, sortOrder int, updatedAt string) error {
	return execUpdateOne(ctx, s.db,
		`UPDATE documentation_pages SET title = ?, category = ?, body_md = ?, audience = ?, enabled = ?, sort_order = ?, updated_at = ?
		 WHERE id = ? AND lang = ? AND is_system = 1`,
		[]any{title, category, bodyMD, audience, enabled, sortOrder, updatedAt, id, lang},
	)
}

// DeleteDocumentationPage removes a non-system page row. Returns sql.ErrNoRows
// if the row does not exist or is a system page (is_system=1) — the WHERE
// clause guards system pages at the storage layer too, defense in depth.
func (s *Store) DeleteDocumentationPage(ctx context.Context, id, lang string) error {
	return execUpdateOne(ctx, s.db,
		`DELETE FROM documentation_pages WHERE id = ? AND lang = ? AND is_system = 0`,
		[]any{id, lang},
	)
}

// SetDocumentationPageEnabled updates the enabled state for one page. Returns
// sql.ErrNoRows if the page does not exist — a toggle is an upsert on the
// enabled column, never a delete.
func (s *Store) SetDocumentationPageEnabled(ctx context.Context, id, lang string, enabled bool, updatedAt string) error {
	return execUpdateOne(ctx, s.db,
		`UPDATE documentation_pages SET enabled = ?, updated_at = ? WHERE id = ? AND lang = ?`,
		[]any{enabled, updatedAt, id, lang},
	)
}

// DocumentationPageExists checks whether a page with the given (id, lang)
// exists (regardless of enabled state).
func (s *Store) DocumentationPageExists(ctx context.Context, id, lang string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM documentation_pages WHERE id = ? AND lang = ?`,
		id, lang).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("query documentation page exists: %w", err)
	}

	return count > 0, nil
}
