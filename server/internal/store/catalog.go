package store

import (
	"context"
	"database/sql"
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
	Node    string
	Name    string
}

// CatalogISO is one approved ISO image (catalog_isos row).
type CatalogISO struct {
	Cluster string
	Node    string
	Storage string
	File    string
}

// CatalogImage is one approved cloud image (catalog_images row). A cloud
// image is a bootable disk image (.qcow2/.raw/.vmdk/.ova) an admin placed
// on a Proxmox storage's import/ directory themselves; SizeBytes carries
// the discovered image size so the create path can reject a disk size
// below it.
type CatalogImage struct {
	Cluster   string
	Node      string
	Storage   string
	File      string
	SizeBytes int64
}

// CatalogProfile is one VM hardware profile (catalog_profiles row).
type CatalogProfile struct {
	Cluster  string
	ID       string
	Label    string
	Sockets  int
	CPUCores int
	MemoryMB int
	DiskGB   int
	Bus      string
}

// CatalogTemplate is one approved Proxmox template (catalog_templates row,
// US2/issue-02). The VMID is the Proxmox VMID of the template VM; the node
// determines where the clone lands (D2b: cross-node clone forbidden).
type CatalogTemplate struct {
	Cluster          string
	Node             string
	VMID             int
	Name             string
	CloudInitCapable bool
	DiskStorage      string
	DiskSizeGB       int
	DiskBus          string
}

// queryCatalog runs a parameterised catalog query and collects the rows into a
// slice using the provided scan function. It closes the rows and surfaces
// scan/query errors with the given label for diagnostics.
func queryCatalog[T any](ctx context.Context, db *sql.DB, label, query string, args []any, scan func(*sql.Rows) (T, error)) ([]T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", label, err)
	}

	defer func() { _ = rows.Close() }()

	var out []T

	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("scan %s: %w", label, err)
		}

		out = append(out, item)
	}

	return out, rows.Err()
}

// CatalogNodes returns the approved nodes for a cluster, ordered by name so
// simple-mode auto-selection (first approved entry, FR-010) is deterministic.
func (s *Store) CatalogNodes(ctx context.Context, cluster string) ([]CatalogNode, error) {
	return queryCatalog(ctx, s.db, "catalog nodes",
		`SELECT cluster, name FROM catalog_nodes WHERE cluster = ? AND enabled = 1 ORDER BY name`,
		[]any{cluster},
		func(rows *sql.Rows) (CatalogNode, error) {
			var n CatalogNode
			return n, rows.Scan(&n.Cluster, &n.Name)
		},
	)
}

// CatalogStorages returns the approved storages for a cluster, ordered by
// node then name (deterministic auto-selection, FR-010).
func (s *Store) CatalogStorages(ctx context.Context, cluster string) ([]CatalogStorage, error) {
	return queryCatalog(ctx, s.db, "catalog storages",
		`SELECT cluster, name, node FROM catalog_storages WHERE cluster = ? AND enabled = 1 ORDER BY node, name`,
		[]any{cluster},
		func(rows *sql.Rows) (CatalogStorage, error) {
			var st CatalogStorage
			return st, rows.Scan(&st.Cluster, &st.Name, &st.Node)
		},
	)
}

// CatalogBridges returns the approved bridges for a cluster, ordered by node then name.
func (s *Store) CatalogBridges(ctx context.Context, cluster string) ([]CatalogBridge, error) {
	return queryCatalog(ctx, s.db, "catalog bridges",
		`SELECT cluster, node, name FROM catalog_bridges WHERE cluster = ? AND enabled = 1 ORDER BY node, name`,
		[]any{cluster},
		func(rows *sql.Rows) (CatalogBridge, error) {
			var b CatalogBridge
			return b, rows.Scan(&b.Cluster, &b.Node, &b.Name)
		},
	)
}

// CatalogISOs returns the approved ISO images for a cluster, one row per node
// (D1b), ordered by node then file. An ISO on shared storage has N rows so
// each node's locality can be validated independently.
func (s *Store) CatalogISOs(ctx context.Context, cluster string) ([]CatalogISO, error) {
	return queryCatalog(ctx, s.db, "catalog isos",
		`SELECT cluster, node, storage, file FROM catalog_isos WHERE cluster = ? AND enabled = 1 ORDER BY node, file`,
		[]any{cluster},
		func(rows *sql.Rows) (CatalogISO, error) {
			var iso CatalogISO
			return iso, rows.Scan(&iso.Cluster, &iso.Node, &iso.Storage, &iso.File)
		},
	)
}

// CatalogImages returns the approved cloud images for a cluster, one row per
// node (D1b), ordered by node then file. An image on shared storage has N
// rows so each node's locality can be validated independently.
func (s *Store) CatalogImages(ctx context.Context, cluster string) ([]CatalogImage, error) {
	return queryCatalog(ctx, s.db, "catalog images",
		`SELECT cluster, node, storage, file, size_bytes FROM catalog_images WHERE cluster = ? AND enabled = 1 ORDER BY node, file`,
		[]any{cluster},
		func(rows *sql.Rows) (CatalogImage, error) {
			var img CatalogImage
			return img, rows.Scan(&img.Cluster, &img.Node, &img.Storage, &img.File, &img.SizeBytes)
		},
	)
}

// CatalogProfiles returns the VM profiles for a cluster, ordered by id.
func (s *Store) CatalogProfiles(ctx context.Context, cluster string) ([]CatalogProfile, error) {
	return queryCatalog(ctx, s.db, "catalog profiles",
		`SELECT cluster, id, label, sockets, cpu_cores, memory_mb, disk_gb, bus FROM catalog_profiles WHERE cluster = ? AND enabled = 1 ORDER BY id`,
		[]any{cluster},
		func(rows *sql.Rows) (CatalogProfile, error) {
			var p CatalogProfile
			return p, rows.Scan(&p.Cluster, &p.ID, &p.Label, &p.Sockets, &p.CPUCores, &p.MemoryMB, &p.DiskGB, &p.Bus)
		},
	)
}

// CatalogTemplates returns the approved Proxmox templates for a cluster,
// ordered by vmid (US2/issue-02). Each row is one approved template; the
// admin curates which templates discovered via template=1 are offered.
func (s *Store) CatalogTemplates(ctx context.Context, cluster string) ([]CatalogTemplate, error) {
	return queryCatalog(ctx, s.db, "catalog templates",
		`SELECT cluster, node, vmid, name, cloud_init_capable, disk_storage, disk_size_gb, disk_bus
		 FROM catalog_templates WHERE cluster = ? AND enabled = 1 ORDER BY vmid`,
		[]any{cluster},
		func(rows *sql.Rows) (CatalogTemplate, error) {
			var t CatalogTemplate

			var cloudInit int
			if err := rows.Scan(&t.Cluster, &t.Node, &t.VMID, &t.Name, &cloudInit, &t.DiskStorage, &t.DiskSizeGB, &t.DiskBus); err != nil {
				return t, err
			}

			t.CloudInitCapable = cloudInit == 1

			return t, nil
		},
	)
}
