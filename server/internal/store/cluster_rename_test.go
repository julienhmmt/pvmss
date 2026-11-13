//nolint:wsl_v5 // SQLite fixtures keep seeds and assertions adjacent
package store_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"pvmss/server/internal/store"
	"testing"
)

// Table names that already appear as literals in other store tests must come
// from constants here or goconst counts the package over its limit.
const (
	tableManagedPools   = "managed_pools"
	tableProfileSSHKeys = "profile_ssh_keys"
	tableVMLimits       = "vm_limits"
	renameSrcCluster    = "rename-src"
)

// clusterScopedSeeds writes one minimal row per cluster-keyed table under the
// given cluster id. vm_limits is absent on purpose: CreateCluster seeds it.
var clusterScopedSeeds = map[string]string{
	"sessions":              `INSERT INTO sessions (token_hash, username, is_admin, expires_at, created_at, cluster) VALUES (x'01', 'alice@pve', 0, '2030-01-01T00:00:00Z', '2026-01-01T00:00:00Z', ?)`,
	"audit_log":             `INSERT INTO audit_log (actor, cluster, action, timestamp) VALUES ('alice@pve', ?, 'vm.start', '2026-01-01T00:00:00Z')`,
	"catalog_nodes":         `INSERT INTO catalog_nodes (cluster, name) VALUES (?, 'node-a')`,
	"catalog_storages":      `INSERT INTO catalog_storages (cluster, name, node) VALUES (?, 'local-lvm', 'node-a')`,
	"catalog_bridges":       `INSERT INTO catalog_bridges (cluster, node, name) VALUES (?, 'node-a', 'vmbr0')`,
	"catalog_isos":          `INSERT INTO catalog_isos (cluster, node, storage, file) VALUES (?, 'node-a', 'local', 'debian.iso')`,
	"catalog_templates":     `INSERT INTO catalog_templates (cluster, node, vmid) VALUES (?, 'node-a', 9000)`,
	"catalog_profiles":      `INSERT INTO catalog_profiles (cluster, id, label, cpu_cores, memory_mb, disk_gb, bus) VALUES (?, 'small', 'Small', 1, 512, 8, 'scsi')`,
	"catalog_tags":          `INSERT INTO catalog_tags (cluster, name, color, created_at) VALUES (?, 'env', '#aabbcc', '2026-01-01T00:00:00Z')`,
	tableManagedPools:       `INSERT INTO managed_pools (cluster, name, created_at) VALUES (?, 'team-a', '2026-01-01T00:00:00Z')`,
	"node_limits":           `INSERT INTO node_limits (cluster, node, max_vms, max_vcpus, max_ram_gb, max_disk_gb) VALUES (?, 'node-a', 10, 8, 64, 500)`,
	tableProfileSSHKeys:     `INSERT INTO profile_ssh_keys (id, cluster, username, label, public_key, created_at) VALUES ('key-1', ?, 'alice@pve', 'laptop', 'ssh-ed25519 AAAA', '2026-01-01T00:00:00Z')`,
	"quota_requests":        `INSERT INTO quota_requests (requester, cluster, pool, current_quota, requested_quota, reason, status, created_at) VALUES ('alice@pve', ?, 'team-a', 1, 3, 'need more', 'pending', '2026-01-01T00:00:00Z')`,
	"vm_leases":             `INSERT INTO vm_leases (cluster, vmid, expires_at) VALUES (?, 100, '2030-01-01T00:00:00Z')`,
	"vm_snapshot_schedules": `INSERT INTO vm_snapshot_schedules (cluster, vmid, frequency, retain) VALUES (?, 100, 'daily', 2)`,
	"pool_quota_overrides":  `INSERT INTO pool_quota_overrides (cluster, pool, max_vms, updated_by, updated_at) VALUES (?, 'team-a', 5, 'admin', '2026-01-01T00:00:00Z')`,
}

func createTestCluster(t *testing.T, st *store.Store, name string) {
	t.Helper()
	row := store.ClusterRow{Name: name, URL: "https://" + name + ".invalid/api2/json", TokenID: testClusterTokenID, TokenSecret: testClusterTokenSecret}
	if err := st.CreateCluster(context.Background(), row); err != nil {
		t.Fatalf("CreateCluster(%q): %v", name, err)
	}
}

func countClusterRows(t *testing.T, st *store.Store, table, clusterName string) int {
	t.Helper()
	var count int
	if err := st.DB().QueryRowContext(context.Background(), fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE cluster = ?`, table), clusterName).Scan(&count); err != nil {
		t.Fatalf("count %s rows for %q: %v", table, clusterName, err)
	}
	return count
}

// seedClusterScopedRows writes one row keyed by clusterName into every
// cluster-scoped table so the rename has something to move everywhere.
func seedClusterScopedRows(t *testing.T, st *store.Store, clusterName string) {
	t.Helper()
	for _, table := range store.ClusterScopedTables() {
		if table == tableVMLimits {
			// CreateCluster seeds the policy row; assert it is really there.
			if got := countClusterRows(t, st, table, clusterName); got != 1 {
				t.Fatalf("%s: seeded rows under %q = %d, want 1", table, clusterName, got)
			}
			continue
		}
		statement, ok := clusterScopedSeeds[table]
		if !ok {
			t.Fatalf("no seed statement for cluster-keyed table %s", table)
		}
		if _, err := st.DB().ExecContext(context.Background(), statement, clusterName); err != nil {
			t.Fatalf("seed %s: %v", table, err)
		}
	}
}

// assertClusterRowsMoved checks every cluster-scoped table holds exactly one
// row under newName and nothing left under oldName.
func assertClusterRowsMoved(t *testing.T, st *store.Store, oldName, newName string) {
	t.Helper()
	for _, table := range store.ClusterScopedTables() {
		t.Run(table, func(t *testing.T) {
			if got := countClusterRows(t, st, table, oldName); got != 0 {
				t.Errorf("%s: rows under %q = %d, want 0", table, oldName, got)
			}
			if got := countClusterRows(t, st, table, newName); got != 1 {
				t.Errorf("%s: rows under %q = %d, want 1", table, newName, got)
			}
		})
	}
}

// TestRenameCluster_MovesEveryClusterKeyedRow - every table in the
// clusterScopedTables list must move its rows in the rename transaction; a
// table forgotten in that list fails here. The clusters row itself moves too.
//
//nolint:paralleltest // serial: shared database fixture
func TestRenameCluster_MovesEveryClusterKeyedRow(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()
	const oldName, newName = "rename-old", "rename-new"
	createTestCluster(t, st, oldName)
	seedClusterScopedRows(t, st, oldName)

	if err := st.RenameCluster(ctx, oldName, newName); err != nil {
		t.Fatalf("RenameCluster: %v", err)
	}

	if _, err := st.GetCluster(ctx, oldName); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetCluster(%q) = %v, want sql.ErrNoRows", oldName, err)
	}
	renamed, err := st.GetCluster(ctx, newName)
	if err != nil {
		t.Fatalf("GetCluster(%q): %v", newName, err)
	}
	if renamed.TokenSecret != testClusterTokenSecret {
		t.Fatalf("renamed TokenSecret = %q, want %q", renamed.TokenSecret, testClusterTokenSecret)
	}

	assertClusterRowsMoved(t, st, oldName, newName)
}

// TestRenameCluster_AuditHistoryFollowsNewID - the rename rewrites
// audit_log.cluster too, so the renamed cluster's audit view keeps the old
// history instead of starting empty.
//
//nolint:paralleltest // serial: shared database fixture
func TestRenameCluster_AuditHistoryFollowsNewID(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()
	const oldName, newName = "audit-old", "audit-new"
	createTestCluster(t, st, oldName)

	if err := st.RecordAction(ctx, "alice@pve", oldName, 100, "vm.start"); err != nil {
		t.Fatalf("RecordAction: %v", err)
	}
	if err := st.RenameCluster(ctx, oldName, newName); err != nil {
		t.Fatalf("RenameCluster: %v", err)
	}

	page, err := st.ListAuditLog(ctx, store.AuditFilter{Cluster: newName, Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("ListAuditLog(%q): %v", newName, err)
	}
	if page.Total != 1 {
		t.Fatalf("audit rows under %q = %d, want 1", newName, page.Total)
	}
	if page.Items[0].Cluster != newName {
		t.Fatalf("audit row cluster = %q, want %q", page.Items[0].Cluster, newName)
	}
	page, err = st.ListAuditLog(ctx, store.AuditFilter{Cluster: oldName, Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("ListAuditLog(%q): %v", oldName, err)
	}
	if page.Total != 0 {
		t.Fatalf("audit rows under %q = %d, want 0", oldName, page.Total)
	}
}

// TestRenameCluster_Errors - validation, not-found and collision branches.
//
//nolint:paralleltest // serial: shared database fixture
func TestRenameCluster_Errors(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()
	createTestCluster(t, st, renameSrcCluster)
	createTestCluster(t, st, "rename-taken")
	createTestCluster(t, st, "rename-ghost")
	if err := st.SoftDeleteCluster(ctx, "rename-ghost"); err != nil {
		t.Fatalf("SoftDeleteCluster: %v", err)
	}
	createTestCluster(t, st, "rename-orphan-src")
	// Same-keyed rows under source and a target that is not a clusters row:
	// renaming must trip the (cluster, name) uniqueness constraint.
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO catalog_nodes (cluster, name) VALUES ('rename-orphan-src', 'node-a'), ('rename-orphan-dst', 'node-a')`); err != nil {
		t.Fatalf("seed orphan collision: %v", err)
	}

	tests := []struct {
		name    string
		oldName string
		newName string
		wantErr error
	}{
		{"same name", renameSrcCluster, renameSrcCluster, store.ErrInvalidClusterName},
		{"invalid name", renameSrcCluster, "Not_Valid", store.ErrInvalidClusterName},
		{"empty name", renameSrcCluster, "", store.ErrInvalidClusterName},
		{"missing source", "no-such-cluster", "rename-x", sql.ErrNoRows},
		{"soft-deleted source", "rename-ghost", "rename-y", sql.ErrNoRows},
		{"active target", renameSrcCluster, "rename-taken", store.ErrDuplicateCluster},
		{"soft-deleted target", renameSrcCluster, "rename-ghost", store.ErrDuplicateCluster},
		{"orphan rows under target", "rename-orphan-src", "rename-orphan-dst", store.ErrDuplicateCluster},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if err := st.RenameCluster(ctx, testCase.oldName, testCase.newName); !errors.Is(err, testCase.wantErr) {
				t.Fatalf("RenameCluster(%q, %q) = %v, want %v", testCase.oldName, testCase.newName, err, testCase.wantErr)
			}
		})
	}

	// A failed rename must leave every row under the source id.
	if _, err := st.GetCluster(ctx, renameSrcCluster); err != nil {
		t.Fatalf("source cluster after failed renames: %v", err)
	}
	if got := countClusterRows(t, st, "catalog_nodes", "rename-orphan-src"); got != 1 {
		t.Fatalf("catalog_nodes under rename-orphan-src = %d, want 1 (rollback)", got)
	}
}
