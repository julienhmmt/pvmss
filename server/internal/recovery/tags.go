package recovery

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// defaultPalette is the small fixed set of colors assigned to migrated tags
// by insertion order. Legacy stored no per-tag color locally (spec
// Assumptions); an admin can re-color any tag through T11's UI after cutover.
var defaultPalette = []string{
	"#3b82f6", // blue
	"#10b981", // emerald
	"#f59e0b", // amber
	"#ef4444", // red
	"#8b5cf6", // violet
	"#ec4899", // pink
	"#14b8a6", // teal
	"#f97316", // orange
}

// MapTags reads the legacy tags table and returns rows for catalog_tags,
// assigning each tag a deterministic color from the default palette by
// insertion order. The mandatory "pvmss" row is included — the upsert is
// a no-op for it if it already exists in the v0.4 database (FR-007).
func MapTags(ctx context.Context, legacyDB *sql.DB) ([]TagRow, error) {
	rows, err := legacyDB.QueryContext(ctx,
		`SELECT name FROM tags ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var names []string

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan tags: %w", err)
		}

		names = append(names, name)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	out := make([]TagRow, 0, len(names))
	for i, name := range names {
		color := defaultPalette[i%len(defaultPalette)]
		out = append(out, TagRow{Name: name, Color: color, CreatedAt: now})
	}

	return out, nil
}

func upsertTag(ctx context.Context, v04DB *sql.DB, cluster string, r TagRow) error {
	_, err := v04DB.ExecContext(ctx, `
		INSERT INTO catalog_tags (cluster, name, color, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(cluster, name) DO UPDATE SET color = excluded.color`,
		cluster, r.Name, r.Color, r.CreatedAt)
	if err != nil {
		return fmt.Errorf("upsert catalog_tags: %w", err)
	}

	return nil
}
