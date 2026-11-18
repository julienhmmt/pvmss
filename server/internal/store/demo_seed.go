package store

import (
	"context"
	"fmt"
	"time"
)

// demoCatalogSQL holds the demo catalog fixture rows for cluster "default":
// pre-approved nodes/storages/profiles, the mandatory pvmss tag, the shipped
// vm_limits gabarit, so the create path works out of the box. Demo mode only - a real deployment discovers and approves
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

// demoClusterTestPattern is the seeded test history per demo cluster, oldest
// first; offline-demo always fails.
var demoClusterTestPattern = []string{"ok", "ok", "unreachable", "ok", "ok", "ok"}

// seedDemoClusterTests gives each fake cluster a short test history. It runs
// at every startup, so it skips clusters that already have history.
func (s *Store) seedDemoClusterTests(ctx context.Context) error {
	now := time.Now().UTC()

	for _, name := range []string{"default", "secondary", "offline-demo"} {
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cluster_test_results WHERE cluster = ?`, name).Scan(&count); err != nil {
			return fmt.Errorf("count demo cluster tests: %w", err)
		}

		if count > 0 {
			continue
		}

		for i, status := range demoClusterTestPattern {
			test := ClusterTest{Cluster: name, Status: status, Version: "8.2.4", Actor: "admin", TestedAt: now.Add(time.Duration(i-len(demoClusterTestPattern)+1) * 6 * time.Hour)}
			if name == "offline-demo" || status != "ok" {
				test.Status, test.Version, test.Message = "unreachable", "", "connection refused"
			}

			duration := int64(120 + 37*i)

			test.DurationMs = &duration
			if err := s.RecordClusterTest(ctx, test); err != nil {
				return err
			}
		}
	}

	return nil
}
