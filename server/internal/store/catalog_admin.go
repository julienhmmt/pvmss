package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrDuplicate is returned by an INSERT that uses ON CONFLICT DO NOTHING when
// the conflict fired — the row already exists. The catalog layer maps this to
// its own ErrDuplicateProfile / ErrDuplicateTag.
var ErrDuplicate = errors.New("duplicate row")

// CatalogNodeEnabled is one catalog_nodes row with its enabled state.
type CatalogNodeEnabled struct {
	Name    string
	Enabled bool
}

// CatalogStorageEnabled is one catalog_storages row with its enabled state.
type CatalogStorageEnabled struct {
	Name    string
	Node    string
	Enabled bool
}

// CatalogBridgeEnabled is one catalog_bridges row with its enabled state.
type CatalogBridgeEnabled struct {
	Name    string
	Node    string
	Enabled bool
}

// CatalogISOEnabled is one catalog_isos row with its enabled state.
type CatalogISOEnabled struct {
	Node    string
	Storage string
	File    string
	Enabled bool
}

// CatalogProfileEnabled is one catalog_profiles row with its enabled state.
type CatalogProfileEnabled struct {
	ID       string
	Label    string
	CPUCores int
	MemoryMB int
	DiskGB   int
	Bus      string
	Enabled  bool
}

// CatalogTemplateEnabled is one catalog_templates row with its enabled state
// (US2/issue-02). Used by the admin listing endpoint.
type CatalogTemplateEnabled struct {
	VMID             int
	Node             string
	Name             string
	CloudInitCapable bool
	DiskStorage      string
	DiskSizeGB       int
	DiskBus          string
	Enabled          bool
}

// CatalogTag is one catalog_tags row.
type CatalogTag struct {
	Cluster   string
	Name      string
	Color     string
	CreatedAt string
}

// CatalogNodesEnabled returns all catalog_nodes rows (including disabled) with
// their enabled state, ordered by name.
func (s *Store) CatalogNodesEnabled(ctx context.Context, cluster string) ([]CatalogNodeEnabled, error) {
	return queryCatalog(ctx, s.db, "catalog nodes enabled",
		`SELECT name, enabled FROM catalog_nodes WHERE cluster = ? ORDER BY name`,
		[]any{cluster},
		func(rows *sql.Rows) (CatalogNodeEnabled, error) {
			var n CatalogNodeEnabled
			return n, rows.Scan(&n.Name, &n.Enabled)
		},
	)
}

// CatalogStoragesEnabled returns all catalog_storages rows (including disabled)
// with their enabled state, ordered by node then name.
func (s *Store) CatalogStoragesEnabled(ctx context.Context, cluster string) ([]CatalogStorageEnabled, error) {
	return queryCatalog(ctx, s.db, "catalog storages enabled",
		`SELECT name, node, enabled FROM catalog_storages WHERE cluster = ? ORDER BY node, name`,
		[]any{cluster},
		func(rows *sql.Rows) (CatalogStorageEnabled, error) {
			var st CatalogStorageEnabled
			return st, rows.Scan(&st.Name, &st.Node, &st.Enabled)
		},
	)
}

// CatalogBridgesEnabled returns all catalog_bridges rows (including disabled)
// with their enabled state, ordered by node then name.
func (s *Store) CatalogBridgesEnabled(ctx context.Context, cluster string) ([]CatalogBridgeEnabled, error) {
	return queryCatalog(ctx, s.db, "catalog bridges enabled",
		`SELECT name, node, enabled FROM catalog_bridges WHERE cluster = ? ORDER BY node, name`,
		[]any{cluster},
		func(rows *sql.Rows) (CatalogBridgeEnabled, error) {
			var b CatalogBridgeEnabled
			return b, rows.Scan(&b.Name, &b.Node, &b.Enabled)
		},
	)
}

// CatalogISOsEnabled returns all catalog_isos rows (including disabled) with
// their enabled state, ordered by node then file.
func (s *Store) CatalogISOsEnabled(ctx context.Context, cluster string) ([]CatalogISOEnabled, error) {
	return queryCatalog(ctx, s.db, "catalog isos enabled",
		`SELECT node, storage, file, enabled FROM catalog_isos WHERE cluster = ? ORDER BY node, file`,
		[]any{cluster},
		func(rows *sql.Rows) (CatalogISOEnabled, error) {
			var iso CatalogISOEnabled
			return iso, rows.Scan(&iso.Node, &iso.Storage, &iso.File, &iso.Enabled)
		},
	)
}

// CatalogProfilesEnabled returns all catalog_profiles rows (including disabled)
// with their full fields, ordered by id.
func (s *Store) CatalogProfilesEnabled(ctx context.Context, cluster string) ([]CatalogProfileEnabled, error) {
	return queryCatalog(ctx, s.db, "catalog profiles enabled",
		`SELECT id, label, cpu_cores, memory_mb, disk_gb, bus, enabled FROM catalog_profiles WHERE cluster = ? ORDER BY id`,
		[]any{cluster},
		func(rows *sql.Rows) (CatalogProfileEnabled, error) {
			var p CatalogProfileEnabled
			return p, rows.Scan(&p.ID, &p.Label, &p.CPUCores, &p.MemoryMB, &p.DiskGB, &p.Bus, &p.Enabled)
		},
	)
}

// SetNodeEnabled upserts the enabled state for one node (FR-006: never a delete).
func (s *Store) SetNodeEnabled(ctx context.Context, cluster, name string, enabled bool) error {
	return execWrite(ctx, s.db,
		`INSERT INTO catalog_nodes (cluster, name, enabled) VALUES (?, ?, ?)
		 ON CONFLICT(cluster, name) DO UPDATE SET enabled = excluded.enabled`,
		[]any{cluster, name, enabled},
	)
}

// SetStorageEnabled upserts the enabled state for one (name, node) pair.
func (s *Store) SetStorageEnabled(ctx context.Context, cluster, name, node string, enabled bool) error {
	return execWrite(ctx, s.db,
		`INSERT INTO catalog_storages (cluster, name, node, enabled) VALUES (?, ?, ?, ?)
		 ON CONFLICT(cluster, name, node) DO UPDATE SET enabled = excluded.enabled`,
		[]any{cluster, name, node, enabled},
	)
}

// SetBridgeEnabled upserts the enabled state for one (node, name) pair.
func (s *Store) SetBridgeEnabled(ctx context.Context, cluster, node, name string, enabled bool) error {
	return execWrite(ctx, s.db,
		`INSERT INTO catalog_bridges (cluster, node, name, enabled) VALUES (?, ?, ?, ?)
		 ON CONFLICT(cluster, node, name) DO UPDATE SET enabled = excluded.enabled`,
		[]any{cluster, node, name, enabled},
	)
}

// SetISOEnabled upserts the enabled state for one (node, storage, file) triple.
func (s *Store) SetISOEnabled(ctx context.Context, cluster, node, storage, file string, enabled bool) error {
	return execWrite(ctx, s.db,
		`INSERT INTO catalog_isos (cluster, node, storage, file, enabled) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(cluster, node, storage, file) DO UPDATE SET enabled = excluded.enabled`,
		[]any{cluster, node, storage, file, enabled},
	)
}

// SetProfileEnabled updates the enabled state for one profile. Returns
// sql.ErrNoRows if the profile does not exist (the catalog layer maps this to
// ErrProfileNotFound) — guards against a delete between the catalog layer's
// existence check and this UPDATE.
func (s *Store) SetProfileEnabled(ctx context.Context, cluster, id string, enabled bool) error {
	return execUpdateOne(ctx, s.db,
		`UPDATE catalog_profiles SET enabled = ? WHERE cluster = ? AND id = ?`,
		[]any{enabled, cluster, id},
	)
}

// CatalogTemplatesEnabled returns all catalog_templates rows (including
// disabled) with their enabled state, ordered by vmid (US2/issue-02).
func (s *Store) CatalogTemplatesEnabled(ctx context.Context, cluster string) ([]CatalogTemplateEnabled, error) {
	return queryCatalog(ctx, s.db, "catalog templates enabled",
		`SELECT vmid, node, name, cloud_init_capable, disk_storage, disk_size_gb, disk_bus, enabled
		 FROM catalog_templates WHERE cluster = ? ORDER BY vmid`,
		[]any{cluster},
		func(rows *sql.Rows) (CatalogTemplateEnabled, error) {
			var t CatalogTemplateEnabled

			var cloudInit int
			if err := rows.Scan(&t.VMID, &t.Node, &t.Name, &cloudInit, &t.DiskStorage, &t.DiskSizeGB, &t.DiskBus, &t.Enabled); err != nil {
				return t, err
			}

			t.CloudInitCapable = cloudInit == 1

			return t, nil
		},
	)
}

// TemplateValues is the editable field set of a catalog_templates row (US2).
type TemplateValues struct {
	Node             string
	Name             string
	CloudInitCapable bool
	DiskStorage      string
	DiskSizeGB       int
	DiskBus          string
}

// InsertTemplate inserts a new template row with the given enabled state.
// Returns ErrDuplicate if the vmid already exists for the cluster
// (US2/issue-02).
func (s *Store) InsertTemplate(ctx context.Context, cluster string, vmid int, values TemplateValues, enabled bool) error {
	return execInsertOne(ctx, s.db,
		`INSERT INTO catalog_templates (cluster, node, vmid, name, cloud_init_capable, disk_storage, disk_size_gb, disk_bus, enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(cluster, vmid) DO NOTHING`,
		[]any{cluster, values.Node, vmid, values.Name, values.CloudInitCapable, values.DiskStorage, values.DiskSizeGB, values.DiskBus, enabled},
	)
}

// UpdateTemplate updates an existing template's values. Returns sql.ErrNoRows
// if the template does not exist (US2/issue-02).
func (s *Store) UpdateTemplate(ctx context.Context, cluster string, vmid int, values TemplateValues) error {
	return execUpdateOne(ctx, s.db,
		`UPDATE catalog_templates SET node = ?, name = ?, cloud_init_capable = ?, disk_storage = ?, disk_size_gb = ?, disk_bus = ?
		 WHERE cluster = ? AND vmid = ?`,
		[]any{values.Node, values.Name, values.CloudInitCapable, values.DiskStorage, values.DiskSizeGB, values.DiskBus, cluster, vmid},
	)
}

// DeleteTemplate removes a template row. Returns sql.ErrNoRows if the template
// did not exist (US2/issue-02).
func (s *Store) DeleteTemplate(ctx context.Context, cluster string, vmid int) error {
	return execUpdateOne(ctx, s.db,
		`DELETE FROM catalog_templates WHERE cluster = ? AND vmid = ?`,
		[]any{cluster, vmid},
	)
}

// SetTemplateEnabled upserts the enabled state for one template (US2/issue-02).
func (s *Store) SetTemplateEnabled(ctx context.Context, cluster string, vmid int, enabled bool) error {
	return execWrite(ctx, s.db,
		`INSERT INTO catalog_templates (cluster, node, vmid, name, cloud_init_capable, disk_storage, disk_size_gb, enabled)
		 VALUES (?, '', ?, '', 0, '', 0, ?)
		 ON CONFLICT(cluster, vmid) DO UPDATE SET enabled = excluded.enabled`,
		[]any{cluster, vmid, enabled},
	)
}

// ProfileValues is the editable field set of a catalog profile row. Grouping it
// collapses the five positional field parameters InsertProfile/UpdateProfile used
// to take (SonarQube go:S107).
type ProfileValues struct {
	Label    string
	CPUCores int
	MemoryMB int
	DiskGB   int
	Bus      string
}

// InsertProfile inserts a new profile row. Returns ErrDuplicate if the id
// already exists for the cluster (ON CONFLICT DO NOTHING guards against a
// concurrent insert between the catalog layer's existence check and this
// INSERT).
func (s *Store) InsertProfile(ctx context.Context, cluster, id string, values ProfileValues) error {
	return execInsertOne(ctx, s.db,
		`INSERT INTO catalog_profiles (cluster, id, label, cpu_cores, memory_mb, disk_gb, bus, enabled) VALUES (?, ?, ?, ?, ?, ?, ?, 1)
		 ON CONFLICT(cluster, id) DO NOTHING`,
		[]any{cluster, id, values.Label, values.CPUCores, values.MemoryMB, values.DiskGB, values.Bus},
	)
}

// UpdateProfile updates an existing profile's values. Returns sql.ErrNoRows if
// the profile does not exist (guards against a delete between the catalog
// layer's existence check and this UPDATE).
func (s *Store) UpdateProfile(ctx context.Context, cluster, id string, values ProfileValues) error {
	return execUpdateOne(ctx, s.db,
		`UPDATE catalog_profiles SET label = ?, cpu_cores = ?, memory_mb = ?, disk_gb = ?, bus = ? WHERE cluster = ? AND id = ?`,
		[]any{values.Label, values.CPUCores, values.MemoryMB, values.DiskGB, values.Bus, cluster, id},
	)
}

// DeleteProfile removes a profile row. Returns sql.ErrNoRows if the profile
// did not exist.
func (s *Store) DeleteProfile(ctx context.Context, cluster, id string) error {
	return execUpdateOne(ctx, s.db,
		`DELETE FROM catalog_profiles WHERE cluster = ? AND id = ?`,
		[]any{cluster, id},
	)
}

// ProfileExists checks whether a profile with the given id exists for the cluster.
func (s *Store) ProfileExists(ctx context.Context, cluster, id string) (bool, error) {
	var count int

	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM catalog_profiles WHERE cluster = ? AND id = ?`,
		cluster, id).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("query profile exists: %w", err)
	}

	return count > 0, nil
}

// CatalogTags returns all tag rows for a cluster, ordered by name.
func (s *Store) CatalogTags(ctx context.Context, cluster string) ([]CatalogTag, error) {
	return queryCatalog(ctx, s.db, "catalog tags",
		`SELECT cluster, name, color, created_at FROM catalog_tags WHERE cluster = ? ORDER BY name`,
		[]any{cluster},
		func(rows *sql.Rows) (CatalogTag, error) {
			var t CatalogTag
			return t, rows.Scan(&t.Cluster, &t.Name, &t.Color, &t.CreatedAt)
		},
	)
}

// InsertTag inserts a new tag row. Returns ErrDuplicate if the (cluster, name)
// pair already exists (ON CONFLICT DO NOTHING guards against a concurrent
// insert between the catalog layer's existence check and this INSERT).
func (s *Store) InsertTag(ctx context.Context, cluster, name, color, createdAt string) error {
	return execInsertOne(ctx, s.db,
		`INSERT INTO catalog_tags (cluster, name, color, created_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(cluster, name) DO NOTHING`,
		[]any{cluster, name, color, createdAt},
	)
}

// UpdateTagColor updates the color of an existing tag. Returns sql.ErrNoRows
// if the tag does not exist (guards against a delete between the catalog
// layer's existence check and this UPDATE).
func (s *Store) UpdateTagColor(ctx context.Context, cluster, name, color string) error {
	return execUpdateOne(ctx, s.db,
		`UPDATE catalog_tags SET color = ? WHERE cluster = ? AND name = ?`,
		[]any{color, cluster, name},
	)
}

// DeleteTag removes a tag row. Returns sql.ErrNoRows if the tag did not exist.
func (s *Store) DeleteTag(ctx context.Context, cluster, name string) error {
	return execUpdateOne(ctx, s.db,
		`DELETE FROM catalog_tags WHERE cluster = ? AND name = ?`,
		[]any{cluster, name},
	)
}

// TagExists checks whether a tag with the given name exists for the cluster.
func (s *Store) TagExists(ctx context.Context, cluster, name string) (bool, error) {
	var count int

	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM catalog_tags WHERE cluster = ? AND name = ?`,
		cluster, name).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("query tag exists: %w", err)
	}

	return count > 0, nil
}

// execWrite runs a single write statement and surfaces the error. Use for
// statements where the rows-affected count is not meaningful (e.g. INSERT ...
// ON CONFLICT DO UPDATE upserts that always affect exactly one row).
func execWrite(ctx context.Context, db *sql.DB, query string, args []any) error {
	if _, err := db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("exec write: %w", err)
	}

	return nil
}

// execInsertOne runs an INSERT ... ON CONFLICT DO NOTHING and returns
// store.ErrDuplicate when the conflict fired (rows-affected == 0).
func execInsertOne(ctx context.Context, db *sql.DB, query string, args []any) error {
	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec insert: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count inserted rows: %w", err)
	}

	if affected == 0 {
		return ErrDuplicate
	}

	return nil
}

// execUpdateOne runs an UPDATE or DELETE and returns sql.ErrNoRows when zero
// rows were affected — the row vanished between the caller's existence check
// and this statement.
func execUpdateOne(ctx context.Context, db *sql.DB, query string, args []any) error {
	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec update: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count affected rows: %w", err)
	}

	if affected == 0 {
		return sql.ErrNoRows
	}

	return nil
}
