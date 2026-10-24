package store

import (
	"context"
	"fmt"
)

// demoCatalogSQL holds the demo catalog fixture rows for cluster "default":
// pre-approved nodes/storages/profiles, the mandatory pvmss tag, the shipped
// vm_limits gabarit, and one approved cloud image so the create path works
// out of the box. Demo mode only - a real deployment discovers and approves
// its own resources; seeding these on a Proxmox cluster would approve nodes
// and files that do not exist there.
//
// Bridges and ISOs are intentionally absent: the fixture deliberately does
// not approve the whole fake dataset, so the rejection path stays visible in
// the demo.
//
// OR IGNORE keeps the seed re-runnable on every boot without clobbering an
// admin's enabled toggles on rows that already exist.
const demoCatalogSQL = `
INSERT OR IGNORE INTO catalog_nodes (cluster, name) VALUES
	('default', 'pve-node-01'),
	('default', 'pve-node-02');

INSERT OR IGNORE INTO catalog_storages (cluster, name, node) VALUES
	('default', 'local-lvm', 'pve-node-01'),
	('default', 'local', 'pve-node-02'),
	('default', 'ceph-data', 'pve-node-02');

INSERT OR IGNORE INTO catalog_profiles (cluster, id, label, cpu_cores, memory_mb, disk_gb, bus) VALUES
	('default', 'small', 'Small (1 vCPU, 2 GB, 20 GB)', 1, 2048, 20, 'scsi'),
	('default', 'medium', 'Medium (2 vCPU, 4 GB, 40 GB)', 2, 4096, 40, 'scsi'),
	('default', 'large', 'Large (4 vCPU, 8 GB, 80 GB)', 4, 8192, 80, 'scsi');

INSERT OR IGNORE INTO catalog_tags (cluster, name, color, created_at)
	VALUES ('default', 'pvmss', '#4f46e5', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));

INSERT OR IGNORE INTO vm_limits (
	cluster, max_sockets, max_cores, max_memory_mb, max_disk_per_vm_gb,
	max_network_cards, max_snapshots, max_vm_per_user, isolation_vlan_tag
) VALUES ('default', 4, 8, 16384, 500, 4, 5, -1, 0);

INSERT OR IGNORE INTO catalog_images (cluster, node, storage, file, size_bytes) VALUES
	('default', 'pve-node-01', 'local', 'ubuntu-24.04-server-cloudimg-amd64.qcow2', 644245094);
`

// SeedDemoCatalog writes the demo catalog fixture rows. Called by Open for
// the fake cluster source; tests that open a bare store call it explicitly
// when they need the seeded approvals.
func (s *Store) SeedDemoCatalog(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, demoCatalogSQL); err != nil {
		return fmt.Errorf("seed demo catalog: %w", err)
	}

	return nil
}
