package store

import (
	"context"
	"fmt"
)

// CatalogNode is one approved cluster node (catalog_nodes row).
type CatalogNode struct {
	Cluster string
	Name    string
}

// CatalogStorage is one approved storage on a node (catalog_storages row).
type CatalogStorage struct {
	Cluster string
	Name    string
	Node    string
}

// CatalogBridge is one approved network bridge (catalog_bridges row).
type CatalogBridge struct {
	Cluster string
	Name    string
}

// CatalogISO is one approved ISO image (catalog_isos row).
type CatalogISO struct {
	Cluster string
	Storage string
	File    string
}

// CatalogProfile is one VM hardware profile (catalog_profiles row).
type CatalogProfile struct {
	Cluster  string
	ID       string
	Label    string
	CPUCores int
	MemoryMB int
	DiskGB   int
	Bus      string
}

// CatalogNodes returns the approved nodes for a cluster, ordered by name so
// simple-mode auto-selection (first approved entry, FR-010) is deterministic.
func (s *Store) CatalogNodes(ctx context.Context, cluster string) ([]CatalogNode, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT cluster, name FROM catalog_nodes WHERE cluster = ? ORDER BY name`, cluster)
	if err != nil {
		return nil, fmt.Errorf("query catalog nodes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []CatalogNode

	for rows.Next() {
		var n CatalogNode
		if err := rows.Scan(&n.Cluster, &n.Name); err != nil {
			return nil, fmt.Errorf("scan catalog node: %w", err)
		}

		out = append(out, n)
	}

	return out, rows.Err()
}

// CatalogStorages returns the approved storages for a cluster, ordered by
// node then name (deterministic auto-selection, FR-010).
func (s *Store) CatalogStorages(ctx context.Context, cluster string) ([]CatalogStorage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT cluster, name, node FROM catalog_storages WHERE cluster = ? ORDER BY node, name`, cluster)
	if err != nil {
		return nil, fmt.Errorf("query catalog storages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []CatalogStorage

	for rows.Next() {
		var st CatalogStorage
		if err := rows.Scan(&st.Cluster, &st.Name, &st.Node); err != nil {
			return nil, fmt.Errorf("scan catalog storage: %w", err)
		}

		out = append(out, st)
	}

	return out, rows.Err()
}

// CatalogBridges returns the approved bridges for a cluster, ordered by name.
func (s *Store) CatalogBridges(ctx context.Context, cluster string) ([]CatalogBridge, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT cluster, name FROM catalog_bridges WHERE cluster = ? ORDER BY name`, cluster)
	if err != nil {
		return nil, fmt.Errorf("query catalog bridges: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []CatalogBridge

	for rows.Next() {
		var b CatalogBridge
		if err := rows.Scan(&b.Cluster, &b.Name); err != nil {
			return nil, fmt.Errorf("scan catalog bridge: %w", err)
		}

		out = append(out, b)
	}

	return out, rows.Err()
}

// CatalogISOs returns the approved ISO images for a cluster, ordered by file.
func (s *Store) CatalogISOs(ctx context.Context, cluster string) ([]CatalogISO, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT cluster, storage, file FROM catalog_isos WHERE cluster = ? ORDER BY file`, cluster)
	if err != nil {
		return nil, fmt.Errorf("query catalog isos: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []CatalogISO

	for rows.Next() {
		var iso CatalogISO
		if err := rows.Scan(&iso.Cluster, &iso.Storage, &iso.File); err != nil {
			return nil, fmt.Errorf("scan catalog iso: %w", err)
		}

		out = append(out, iso)
	}

	return out, rows.Err()
}

// CatalogProfiles returns the VM profiles for a cluster, ordered by id.
func (s *Store) CatalogProfiles(ctx context.Context, cluster string) ([]CatalogProfile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT cluster, id, label, cpu_cores, memory_mb, disk_gb, bus FROM catalog_profiles WHERE cluster = ? ORDER BY id`, cluster)
	if err != nil {
		return nil, fmt.Errorf("query catalog profiles: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []CatalogProfile

	for rows.Next() {
		var p CatalogProfile
		if err := rows.Scan(&p.Cluster, &p.ID, &p.Label, &p.CPUCores, &p.MemoryMB, &p.DiskGB, &p.Bus); err != nil {
			return nil, fmt.Errorf("scan catalog profile: %w", err)
		}

		out = append(out, p)
	}

	return out, rows.Err()
}
