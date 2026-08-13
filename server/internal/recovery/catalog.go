package recovery

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// MapCatalog reads the four legacy enabled_* tables and returns rows for
// the v0.4 catalog_* tables. Storage node expansion is performed by the
// optional resolver (data-model.md §1); when nil, every storage is skipped
// with a named reason.
func MapCatalog(ctx context.Context, legacyDB *sql.DB, cluster string, resolver StorageNodeResolver) (
	nodes []NodeRow, storages []StorageRow, skips []SkipReason,
	bridges []BridgeRow, isos []ISORow, isoSkips []SkipReason,
	err error,
) {
	nodes, err = mapNodes(ctx, legacyDB)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("map nodes: %w", err)
	}

	storages, skips, err = mapStorages(ctx, legacyDB, cluster, resolver)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("map storages: %w", err)
	}

	bridges, err = mapBridges(ctx, legacyDB)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("map bridges: %w", err)
	}

	isos, isoSkips, err = mapISOs(ctx, legacyDB)
	if err != nil {
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("map isos: %w", err)
	}

	return nodes, storages, skips, bridges, isos, isoSkips, nil
}

//nolint:dupl // per-table mappers are intentionally separate (spec: one file per source table)
func mapNodes(ctx context.Context, legacyDB *sql.DB) ([]NodeRow, error) {
	rows, err := legacyDB.QueryContext(ctx,
		`SELECT name, enabled FROM enabled_nodes ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query enabled_nodes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []NodeRow

	for rows.Next() {
		var r NodeRow
		if err := rows.Scan(&r.Name, &r.Enabled); err != nil {
			return nil, fmt.Errorf("scan enabled_nodes: %w", err)
		}

		out = append(out, r)
	}

	return out, rows.Err()
}

func mapStorages(ctx context.Context, legacyDB *sql.DB, _ string, resolver StorageNodeResolver) ([]StorageRow, []SkipReason, error) {
	rows, err := legacyDB.QueryContext(ctx,
		`SELECT storage_id, enabled FROM enabled_storages ORDER BY storage_id`)
	if err != nil {
		return nil, nil, fmt.Errorf("query enabled_storages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var (
		out   []StorageRow
		skips []SkipReason
	)

	for rows.Next() {
		var (
			name    string
			enabled bool
		)
		if err := rows.Scan(&name, &enabled); err != nil {
			return nil, nil, fmt.Errorf("scan enabled_storages: %w", err)
		}

		if resolver == nil {
			skips = append(skips, SkipReason{
				Row:    name,
				Reason: "no Proxmox credentials — storage-node expansion skipped",
			})

			continue
		}

		nodes, err := resolver.StorageNodes(ctx, name)
		if err != nil {
			skips = append(skips, SkipReason{
				Row:    name,
				Reason: fmt.Sprintf("live discovery error: %v", err),
			})

			continue
		}

		if len(nodes) == 0 {
			skips = append(skips, SkipReason{
				Row:    name,
				Reason: "no node reports it in live discovery",
			})

			continue
		}

		for _, node := range nodes {
			out = append(out, StorageRow{Name: name, Node: node, Enabled: enabled})
		}
	}

	return out, skips, rows.Err()
}

//nolint:dupl // per-table mappers are intentionally separate (spec: one file per source table)
func mapBridges(ctx context.Context, legacyDB *sql.DB) ([]BridgeRow, error) {
	rows, err := legacyDB.QueryContext(ctx,
		`SELECT name, enabled FROM enabled_vmbrs ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query enabled_vmbrs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []BridgeRow

	for rows.Next() {
		var r BridgeRow
		if err := rows.Scan(&r.Name, &r.Enabled); err != nil {
			return nil, fmt.Errorf("scan enabled_vmbrs: %w", err)
		}

		out = append(out, r)
	}

	return out, rows.Err()
}

func mapISOs(ctx context.Context, legacyDB *sql.DB) ([]ISORow, []SkipReason, error) {
	rows, err := legacyDB.QueryContext(ctx,
		`SELECT name, enabled FROM enabled_isos ORDER BY name`)
	if err != nil {
		return nil, nil, fmt.Errorf("query enabled_isos: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var (
		out   []ISORow
		skips []SkipReason
	)

	for rows.Next() {
		var (
			name    string
			enabled bool
		)
		if err := rows.Scan(&name, &enabled); err != nil {
			return nil, nil, fmt.Errorf("scan enabled_isos: %w", err)
		}

		storage, file, ok := splitISOVolid(name)
		if !ok {
			skips = append(skips, SkipReason{
				Row:    name,
				Reason: "does not match storage:iso/file volid shape",
			})

			continue
		}

		out = append(out, ISORow{Storage: storage, File: file, Enabled: enabled})
	}

	return out, skips, rows.Err()
}

// splitISOVolid splits a legacy ISO name on the first colon after the
// storage segment, following Proxmox's "storage:iso/filename" convention.
// Returns ok=false if the name does not contain a colon.
func splitISOVolid(name string) (storage, file string, ok bool) {
	idx := strings.Index(name, ":")
	if idx <= 0 || idx >= len(name)-1 {
		return "", "", false
	}

	return name[:idx], name[idx+1:], true
}

// --- Upsert helpers ---

func upsertNode(ctx context.Context, v04DB *sql.DB, cluster string, r NodeRow) error {
	_, err := v04DB.ExecContext(ctx, `
		INSERT INTO catalog_nodes (cluster, name, enabled) VALUES (?, ?, ?)
		ON CONFLICT(cluster, name) DO UPDATE SET enabled = excluded.enabled`,
		cluster, r.Name, r.Enabled)
	if err != nil {
		return fmt.Errorf("upsert catalog_nodes: %w", err)
	}

	return nil
}

func upsertStorage(ctx context.Context, v04DB *sql.DB, cluster string, r StorageRow) error {
	_, err := v04DB.ExecContext(ctx, `
		INSERT INTO catalog_storages (cluster, name, node, enabled) VALUES (?, ?, ?, ?)
		ON CONFLICT(cluster, name, node) DO UPDATE SET enabled = excluded.enabled`,
		cluster, r.Name, r.Node, r.Enabled)
	if err != nil {
		return fmt.Errorf("upsert catalog_storages: %w", err)
	}

	return nil
}

func upsertBridge(ctx context.Context, v04DB *sql.DB, cluster string, r BridgeRow) error {
	_, err := v04DB.ExecContext(ctx, `
		INSERT INTO catalog_bridges (cluster, name, enabled) VALUES (?, ?, ?)
		ON CONFLICT(cluster, name) DO UPDATE SET enabled = excluded.enabled`,
		cluster, r.Name, r.Enabled)
	if err != nil {
		return fmt.Errorf("upsert catalog_bridges: %w", err)
	}

	return nil
}

func upsertISO(ctx context.Context, v04DB *sql.DB, cluster string, r ISORow) error {
	_, err := v04DB.ExecContext(ctx, `
		INSERT INTO catalog_isos (cluster, storage, file, enabled) VALUES (?, ?, ?, ?)
		ON CONFLICT(cluster, storage, file) DO UPDATE SET enabled = excluded.enabled`,
		cluster, r.Storage, r.File, r.Enabled)
	if err != nil {
		return fmt.Errorf("upsert catalog_isos: %w", err)
	}

	return nil
}
