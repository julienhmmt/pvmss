//nolint:goconst // test fixtures reuse cluster/storage string literals across seed and assertion sites
package recovery_test

import (
	"context"
	"database/sql"
	"pvmss/server/internal/recovery"
	"strings"
	"testing"
)

// --- Run: MapCluster error path (invalid cluster name) ---

func TestRun_InvalidClusterName_ReturnsError(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	v04DB := openV04DB(t)
	ctx := context.Background()

	_, err := recovery.Run(ctx, legacyDB, v04DB, recovery.RunOptions{
		ClusterName: "Not Valid!",
		DryRun:      true,
	})
	if err == nil {
		t.Fatal("expected error for invalid cluster name, got nil")
	}
}

// --- Run: write cluster error path (non-dry-run with a bad v0.4 db) ---

func TestRun_WriteClusterError_ReturnsWrappedError(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	ctx := context.Background()

	// A v0.4 DB that has not been migrated — upsertCluster will fail.
	badDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open bad db: %v", err)
	}

	t.Cleanup(func() { _ = badDB.Close() })

	_, runErr := recovery.Run(ctx, legacyDB, badDB, recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        false,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if runErr == nil {
		t.Fatal("expected write cluster error, got nil")
	}

	if !strings.Contains(runErr.Error(), "write cluster") {
		t.Errorf("error = %v, want it to contain 'write cluster'", runErr)
	}
}

// --- Run: stepNodes error path (bad legacy db) ---

func TestRun_StepNodesError_BadLegacyDB(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// A legacy DB with no enabled_nodes table — mapNodes will fail.
	badLegacy, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open bad legacy db: %v", err)
	}

	t.Cleanup(func() { _ = badLegacy.Close() })

	v04DB := openV04DB(t)

	_, runErr := recovery.Run(ctx, badLegacy, v04DB, recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        true,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if runErr == nil {
		t.Fatal("expected error from mapNodes on empty legacy db, got nil")
	}
}

// openV04DBWithoutTable opens a fully-migrated v0.4 DB and then drops the
// named table so that upserts targeting it fail. This lets each step's write
// error path be exercised independently — the cluster upsert (which targets
// the clusters table) still succeeds, but the specific step's upsert fails.
func openV04DBWithoutTable(t *testing.T, tableName string) *sql.DB {
	t.Helper()

	db := openV04DB(t)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, `DROP TABLE `+tableName); err != nil {
		t.Fatalf("drop %s: %v", tableName, err)
	}

	return db
}

// --- Run: stepNodes write error path (catalog_nodes dropped) ---

func TestRun_StepNodesWriteError_BadV04DB(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Nodes: []struct {
			name    string
			enabled bool
		}{{name: "pve-a", enabled: true}},
		VMLimits: &legacyVMLimits{maxVMS: 10, maxVMPerUser: 5},
	})

	ctx := context.Background()

	// Migrated v0.4 DB with catalog_nodes dropped — upsertCluster succeeds
	// (clusters table intact), but upsertNode fails.
	badV04 := openV04DBWithoutTable(t, "catalog_nodes")

	_, runErr := recovery.Run(ctx, legacyDB, badV04, recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        false,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if runErr == nil {
		t.Fatal("expected upsertNode error, got nil")
	}
}

// --- Run: live storage resolver wiring (resolved creds trigger live resolver) ---

func TestRun_LiveStorageResolverWired_WhenCredsAvailable(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Storages: []struct {
			name    string
			enabled bool
		}{{name: "local-lvm", enabled: true}},
		VMLimits: &legacyVMLimits{maxVMS: 10, maxVMPerUser: 5},
	})
	v04DB := openV04DB(t)
	ctx := context.Background()

	// Provide Proxmox creds via flags so Run wires a liveStorageResolver.
	// The resolver will attempt a live Snapshot, which fails (no real
	// Proxmox), so the storage is skipped — but the run must not abort.
	sum, err := recovery.Run(ctx, legacyDB, v04DB, recovery.RunOptions{
		ClusterName: "default",
		DryRun:      false,
		ProxmoxCreds: recovery.ProxmoxCreds{
			URL:         "https://nonexistent.example.com:8006",
			TokenID:     "test@pve!service",
			TokenSecret: "secret-token-value-1234567890",
		},
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if err != nil {
		t.Fatalf("Run with live resolver should not abort: %v", err)
	}

	if sum.CatalogStorages.Skipped == 0 {
		t.Error("expected at least one skipped storage from failed live discovery")
	}
}

// --- Run: custom Environ is used (not os.Getenv) ---

func TestRun_CustomEnvironUsed(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, defaultSeed())
	v04DB := openV04DB(t)
	ctx := context.Background()

	env := stubEnviron{
		"PROXMOX_URL":             "https://env.example.com:8006",
		"PROXMOX_API_TOKEN_NAME":  "env-token",
		"PROXMOX_API_TOKEN_VALUE": "env-secret-value-1234567890",
	}

	sum, err := recovery.Run(ctx, legacyDB, v04DB, recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        true,
		Environ:       env,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if err != nil {
		t.Fatalf("Run with custom environ: %v", err)
	}

	if sum.Cluster.Written != 1 {
		t.Errorf("cluster written = %d, want 1", sum.Cluster.Written)
	}
}

// --- Run: full write path with all tables populated ---

func TestRun_WritePath_AllTablesPopulated(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, defaultSeed())
	v04DB := openV04DB(t)
	ctx := context.Background()

	sum, err := recovery.Run(ctx, legacyDB, v04DB, recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        false,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if err != nil {
		t.Fatalf("Run write path: %v", err)
	}

	if sum.CatalogNodes.Written == 0 {
		t.Error("expected catalog_nodes written > 0")
	}

	if sum.CatalogBridges.Written == 0 {
		t.Error("expected catalog_bridges written > 0")
	}

	if sum.CatalogISOs.Written == 0 {
		t.Error("expected catalog_isos written > 0")
	}

	if sum.CatalogProfiles.Written == 0 {
		t.Error("expected catalog_profiles written > 0")
	}

	if sum.CatalogTags.Written == 0 {
		t.Error("expected catalog_tags written > 0")
	}

	if sum.VMLimits.Written != 1 {
		t.Errorf("vm_limits written = %d, want 1", sum.VMLimits.Written)
	}

	if sum.NodeLimits.Written == 0 {
		t.Error("expected node_limits written > 0")
	}
}

// --- Run: stepStorages with resolver that returns nodes ---

func TestRun_StepStoragesWithResolver_ExpandsNodes(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Storages: []struct {
			name    string
			enabled bool
		}{
			{name: "local-lvm", enabled: true},
		},
		VMLimits: &legacyVMLimits{maxVMS: 10, maxVMPerUser: 5},
	})
	v04DB := openV04DB(t)
	ctx := context.Background()

	resolver := mockStorageResolver{nodes: []string{"pve-a"}}

	sum, err := recovery.Run(ctx, legacyDB, v04DB, recovery.RunOptions{
		ClusterName:     "default",
		DryRun:          false,
		StorageResolver: resolver,
		SessionSecret:   "test-session-secret-at-least-32-bytes!!",
	})
	if err != nil {
		t.Fatalf("Run with resolver: %v", err)
	}

	if sum.CatalogStorages.Written == 0 {
		t.Error("expected storages written > 0 with resolver")
	}
}

// --- Run: stepISOs with skip reasons ---

func TestRun_StepISOsWithSkips_RecordsSkipReasons(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		ISOs: []struct {
			name    string
			enabled bool
		}{
			{name: "invalid-no-colon-volid", enabled: true},
			{name: "local:iso/valid.iso", enabled: true},
		},
		VMLimits: &legacyVMLimits{maxVMS: 10, maxVMPerUser: 5},
	})
	v04DB := openV04DB(t)
	ctx := context.Background()

	sum, err := recovery.Run(ctx, legacyDB, v04DB, recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        true,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if err != nil {
		t.Fatalf("Run with iso skips: %v", err)
	}

	if sum.CatalogISOs.Skipped == 0 {
		t.Error("expected at least one skipped ISO")
	}
}

// --- Run: stepProfiles with skip reasons (malformed JSON) ---

func TestRun_StepProfilesWithSkips_RecordsSkipReasons(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Profiles: []legacyProfile{
			{id: "bad", name: "Bad", config: `{not valid json`, enabled: true},
			{id: "good", name: "Good", config: `{"sockets":1,"cores":2,"ram_gb":4,"disk_gb":20,"disk_bus":"virtio"}`, enabled: true},
		},
		VMLimits: &legacyVMLimits{maxVMS: 10, maxVMPerUser: 5},
	})
	v04DB := openV04DB(t)
	ctx := context.Background()

	sum, err := recovery.Run(ctx, legacyDB, v04DB, recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        true,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if err != nil {
		t.Fatalf("Run with profile skips: %v", err)
	}

	if sum.CatalogProfiles.Skipped == 0 {
		t.Error("expected at least one skipped profile")
	}
}

// --- RenderSummary: full output with skip reasons and vm_limits note ---

func TestRenderSummary_FullOutputWithSkipsAndNote(t *testing.T) {
	t.Parallel()

	sum := recovery.Summary{
		Cluster:      recovery.TableResult{Written: 1},
		CatalogNodes: recovery.TableResult{Read: 2, Written: 2},
		CatalogStorages: recovery.TableResult{
			Read: 3, Written: 2, Skipped: 1,
			SkipReasons: []string{`storage "bad": no nodes`},
		},
		CatalogBridges:  recovery.TableResult{Read: 1, Written: 1},
		CatalogISOs:     recovery.TableResult{Read: 2, Written: 1, Skipped: 1, SkipReasons: []string{`row "bad": invalid volid`}},
		CatalogProfiles: recovery.TableResult{Read: 2, Written: 2},
		CatalogTags:     recovery.TableResult{Read: 3, Written: 3},
		VMLimits: recovery.TableResult{
			Read: 1, Written: 1,
			Note: "max_disk_per_vm_gb, max_network_cards, max_snapshots, max_vm_per_user, allow_custom_yaml",
		},
		NodeLimits: recovery.TableResult{Read: 2, Written: 2},
	}

	out := recovery.RenderSummary(sum, "/path/legacy.db", "/path/v04.db", "default")
	if out == "" {
		t.Fatal("RenderSummary returned empty output")
	}

	if !strings.Contains(out, "pvmss-recover:") {
		t.Errorf("output missing header: %q", out)
	}

	if !strings.Contains(out, "clusters        1 written") {
		t.Errorf("output missing clusters line: %q", out)
	}

	if !strings.Contains(out, `storage "bad": no nodes`) {
		t.Errorf("output missing storage skip reason: %q", out)
	}

	if !strings.Contains(out, `row "bad": invalid volid`) {
		t.Errorf("output missing iso skip reason: %q", out)
	}

	if !strings.Contains(out, "vm_limits       1 row updated") {
		t.Errorf("output missing vm_limits note line: %q", out)
	}

	if !strings.Contains(out, "SUMMARY: written=") {
		t.Errorf("output missing SUMMARY line: %q", out)
	}
}

// --- RenderSummary: vm_limits without note uses the read/written/skipped format ---

func TestRenderSummary_VMLimitsWithoutNote_UsesCountFormat(t *testing.T) {
	t.Parallel()

	sum := recovery.Summary{
		VMLimits: recovery.TableResult{Read: 1, Written: 1, Skipped: 0},
	}

	out := recovery.RenderSummary(sum, "legacy.db", "v04.db", "default")
	if !strings.Contains(out, "vm_limits       1 read,  1 written,  0 skipped") {
		t.Errorf("output missing vm_limits count format: %q", out)
	}
}

// --- RenderSummary: table result with Note renders the note ---

func TestRenderSummary_TableResultWithNote_RendersNote(t *testing.T) {
	t.Parallel()

	sum := recovery.Summary{
		CatalogNodes: recovery.TableResult{Read: 1, Written: 1, Note: "custom annotation"},
	}

	out := recovery.RenderSummary(sum, "legacy.db", "v04.db", "default")
	if !strings.Contains(out, "(custom annotation)") {
		t.Errorf("output missing table note: %q", out)
	}
}

// --- RenderSummary: empty summary does not panic ---

func TestRenderSummary_EmptySummary_DoesNotPanic(t *testing.T) {
	t.Parallel()

	out := recovery.RenderSummary(recovery.Summary{}, "legacy.db", "v04.db", "default")
	if out == "" {
		t.Fatal("RenderSummary returned empty output for empty summary")
	}

	if !strings.Contains(out, "SUMMARY: written=0 skipped=0 errors=0") {
		t.Errorf("output missing zero SUMMARY line: %q", out)
	}
}

// --- Run: stepBridges write error path ---

func TestRun_StepBridgesWriteError_BadV04DB(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Nodes: []struct {
			name    string
			enabled bool
		}{{name: "pve-a", enabled: true}},
		Storages: []struct {
			name    string
			enabled bool
		}{{name: "local-lvm", enabled: true}},
		Bridges: []struct {
			name    string
			enabled bool
		}{{name: "vmbr0", enabled: true}},
		VMLimits: &legacyVMLimits{maxVMS: 10, maxVMPerUser: 5},
	})

	ctx := context.Background()

	badV04 := openV04DBWithoutTable(t, "catalog_bridges")
	resolver := mockStorageResolver{nodes: []string{"pve-a"}}

	_, runErr := recovery.Run(ctx, legacyDB, badV04, recovery.RunOptions{
		ClusterName:     "default",
		DryRun:          false,
		StorageResolver: resolver,
		SessionSecret:   "test-session-secret-at-least-32-bytes!!",
	})
	if runErr == nil {
		t.Fatal("expected upsertBridge error, got nil")
	}
}

// --- Run: stepTags write error path ---

func TestRun_StepTagsWriteError_BadV04DB(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Nodes: []struct {
			name    string
			enabled bool
		}{{name: "pve-a", enabled: true}},
		Tags:     []string{"pvmss", "prod"},
		VMLimits: &legacyVMLimits{maxVMS: 10, maxVMPerUser: 5},
	})

	ctx := context.Background()

	badV04 := openV04DBWithoutTable(t, "catalog_tags")

	_, runErr := recovery.Run(ctx, legacyDB, badV04, recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        false,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if runErr == nil {
		t.Fatal("expected upsertTag error, got nil")
	}
}

// --- Run: stepNodeLimits write error path ---

func TestRun_StepNodeLimitsWriteError_BadV04DB(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		NodeLimits: []legacyNodeLimits{
			{node: "pve-a", maxVMs: 10, maxVCPUs: 32, maxRAMGB: 64, maxDiskGB: 500},
		},
		VMLimits: &legacyVMLimits{maxVMS: 10, maxVMPerUser: 5},
	})

	ctx := context.Background()

	badV04 := openV04DBWithoutTable(t, "node_limits")

	_, runErr := recovery.Run(ctx, legacyDB, badV04, recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        false,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if runErr == nil {
		t.Fatal("expected upsertNodeLimits error, got nil")
	}
}

// --- Run: stepVMLimits write error path ---

func TestRun_StepVMLimitsWriteError_BadV04DB(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		VMLimits: &legacyVMLimits{maxVMS: 10, maxVMPerUser: 5},
	})

	ctx := context.Background()

	badV04 := openV04DBWithoutTable(t, "vm_limits")

	_, runErr := recovery.Run(ctx, legacyDB, badV04, recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        false,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if runErr == nil {
		t.Fatal("expected upsertVMLimits error, got nil")
	}
}

// --- Run: stepProfiles write error path ---

func TestRun_StepProfilesWriteError_BadV04DB(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Profiles: []legacyProfile{
			{id: "small", name: "Small", config: `{"sockets":1,"cores":2,"ram_gb":4,"disk_gb":20,"disk_bus":"virtio"}`, enabled: true},
		},
		VMLimits: &legacyVMLimits{maxVMS: 10, maxVMPerUser: 5},
	})

	ctx := context.Background()

	badV04 := openV04DBWithoutTable(t, "catalog_profiles")

	_, runErr := recovery.Run(ctx, legacyDB, badV04, recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        false,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if runErr == nil {
		t.Fatal("expected upsertProfile error, got nil")
	}
}

// --- Run: stepISOs write error path ---

func TestRun_StepISOsWriteError_BadV04DB(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Nodes: []struct {
			name    string
			enabled bool
		}{{name: "pve-a", enabled: true}},
		ISOs: []struct {
			name    string
			enabled bool
		}{{name: "local:iso/ubuntu.iso", enabled: true}},
		VMLimits: &legacyVMLimits{maxVMS: 10, maxVMPerUser: 5},
	})

	ctx := context.Background()

	badV04 := openV04DBWithoutTable(t, "catalog_isos")

	_, runErr := recovery.Run(ctx, legacyDB, badV04, recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        false,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if runErr == nil {
		t.Fatal("expected upsertISO error, got nil")
	}
}

// --- Run: stepStorages write error path ---

func TestRun_StepStoragesWriteError_BadV04DB(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Nodes: []struct {
			name    string
			enabled bool
		}{{name: "pve-a", enabled: true}},
		Storages: []struct {
			name    string
			enabled bool
		}{{name: "local-lvm", enabled: true}},
		VMLimits: &legacyVMLimits{maxVMS: 10, maxVMPerUser: 5},
	})

	ctx := context.Background()

	badV04 := openV04DBWithoutTable(t, "catalog_storages")
	resolver := mockStorageResolver{nodes: []string{"pve-a"}}

	_, runErr := recovery.Run(ctx, legacyDB, badV04, recovery.RunOptions{
		ClusterName:     "default",
		DryRun:          false,
		StorageResolver: resolver,
		SessionSecret:   "test-session-secret-at-least-32-bytes!!",
	})
	if runErr == nil {
		t.Fatal("expected upsertStorage error, got nil")
	}
}

// --- Run: stepStorages error from mapStorages (bad legacy db) ---

func TestRun_StepStoragesMapError_BadLegacyDB(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	badLegacy, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open bad legacy db: %v", err)
	}

	t.Cleanup(func() { _ = badLegacy.Close() })

	// Create only the enabled_nodes table so stepNodes succeeds, but not
	// enabled_storages so stepStorages fails.
	if _, err := badLegacy.ExecContext(ctx,
		`CREATE TABLE enabled_nodes (name TEXT PRIMARY KEY, enabled BOOLEAN)`); err != nil {
		t.Fatalf("create nodes table: %v", err)
	}

	v04DB := openV04DB(t)

	_, runErr := recovery.Run(ctx, badLegacy, v04DB, recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        true,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if runErr == nil {
		t.Fatal("expected mapStorages error, got nil")
	}
}

// --- Run: stepBridges error from mapBridges (bad legacy db) ---

func TestRun_StepBridgesMapError_BadLegacyDB(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	badLegacy, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open bad legacy db: %v", err)
	}

	t.Cleanup(func() { _ = badLegacy.Close() })

	// Create nodes and storages tables so those steps pass, but not bridges.
	for _, ddl := range []string{
		`CREATE TABLE enabled_nodes (name TEXT PRIMARY KEY, enabled BOOLEAN)`,
		`CREATE TABLE enabled_storages (storage_id TEXT PRIMARY KEY, enabled BOOLEAN)`,
	} {
		if _, err := badLegacy.ExecContext(ctx, ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	v04DB := openV04DB(t)

	_, runErr := recovery.Run(ctx, badLegacy, v04DB, recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        true,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if runErr == nil {
		t.Fatal("expected mapBridges error, got nil")
	}
}

// --- Run: stepISOs error from mapISOs (bad legacy db) ---

func TestRun_StepISOsMapError_BadLegacyDB(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	badLegacy, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open bad legacy db: %v", err)
	}

	t.Cleanup(func() { _ = badLegacy.Close() })

	for _, ddl := range []string{
		`CREATE TABLE enabled_nodes (name TEXT PRIMARY KEY, enabled BOOLEAN)`,
		`CREATE TABLE enabled_storages (storage_id TEXT PRIMARY KEY, enabled BOOLEAN)`,
		`CREATE TABLE enabled_vmbrs (name TEXT PRIMARY KEY, enabled BOOLEAN)`,
	} {
		if _, err := badLegacy.ExecContext(ctx, ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	v04DB := openV04DB(t)

	_, runErr := recovery.Run(ctx, badLegacy, v04DB, recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        true,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if runErr == nil {
		t.Fatal("expected mapISOs error, got nil")
	}
}

// --- Run: stepProfiles error from MapProfiles (bad legacy db) ---

func TestRun_StepProfilesMapError_BadLegacyDB(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	badLegacy, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open bad legacy db: %v", err)
	}

	t.Cleanup(func() { _ = badLegacy.Close() })

	for _, ddl := range []string{
		`CREATE TABLE enabled_nodes (name TEXT PRIMARY KEY, enabled BOOLEAN)`,
		`CREATE TABLE enabled_storages (storage_id TEXT PRIMARY KEY, enabled BOOLEAN)`,
		`CREATE TABLE enabled_vmbrs (name TEXT PRIMARY KEY, enabled BOOLEAN)`,
		`CREATE TABLE enabled_isos (name TEXT PRIMARY KEY, enabled BOOLEAN)`,
	} {
		if _, err := badLegacy.ExecContext(ctx, ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	v04DB := openV04DB(t)

	_, runErr := recovery.Run(ctx, badLegacy, v04DB, recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        true,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if runErr == nil {
		t.Fatal("expected MapProfiles error, got nil")
	}
}

// --- Run: stepTags error from MapTags (bad legacy db) ---

func TestRun_StepTagsMapError_BadLegacyDB(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	badLegacy, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open bad legacy db: %v", err)
	}

	t.Cleanup(func() { _ = badLegacy.Close() })

	for _, ddl := range []string{
		`CREATE TABLE enabled_nodes (name TEXT PRIMARY KEY, enabled BOOLEAN)`,
		`CREATE TABLE enabled_storages (storage_id TEXT PRIMARY KEY, enabled BOOLEAN)`,
		`CREATE TABLE enabled_vmbrs (name TEXT PRIMARY KEY, enabled BOOLEAN)`,
		`CREATE TABLE enabled_isos (name TEXT PRIMARY KEY, enabled BOOLEAN)`,
		`CREATE TABLE vm_profiles (id TEXT PRIMARY KEY, name TEXT, description TEXT, config TEXT, enabled BOOLEAN)`,
	} {
		if _, err := badLegacy.ExecContext(ctx, ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	v04DB := openV04DB(t)

	_, runErr := recovery.Run(ctx, badLegacy, v04DB, recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        true,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if runErr == nil {
		t.Fatal("expected MapTags error, got nil")
	}
}

// --- Run: stepVMLimits error from mapVMLimits (bad legacy db) ---

func TestRun_StepVMLimitsMapError_BadLegacyDB(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	badLegacy, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open bad legacy db: %v", err)
	}

	t.Cleanup(func() { _ = badLegacy.Close() })

	for _, ddl := range []string{
		`CREATE TABLE enabled_nodes (name TEXT PRIMARY KEY, enabled BOOLEAN)`,
		`CREATE TABLE enabled_storages (storage_id TEXT PRIMARY KEY, enabled BOOLEAN)`,
		`CREATE TABLE enabled_vmbrs (name TEXT PRIMARY KEY, enabled BOOLEAN)`,
		`CREATE TABLE enabled_isos (name TEXT PRIMARY KEY, enabled BOOLEAN)`,
		`CREATE TABLE vm_profiles (id TEXT PRIMARY KEY, name TEXT, description TEXT, config TEXT, enabled BOOLEAN)`,
		`CREATE TABLE tags (name TEXT PRIMARY KEY)`,
	} {
		if _, err := badLegacy.ExecContext(ctx, ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	v04DB := openV04DB(t)

	_, runErr := recovery.Run(ctx, badLegacy, v04DB, recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        true,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if runErr == nil {
		t.Fatal("expected mapVMLimits error, got nil")
	}
}

// --- Run: stepNodeLimits error from mapNodeLimits (bad legacy db) ---

func TestRun_StepNodeLimitsMapError_BadLegacyDB(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	badLegacy, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open bad legacy db: %v", err)
	}

	t.Cleanup(func() { _ = badLegacy.Close() })

	for _, ddl := range []string{
		`CREATE TABLE enabled_nodes (name TEXT PRIMARY KEY, enabled BOOLEAN)`,
		`CREATE TABLE enabled_storages (storage_id TEXT PRIMARY KEY, enabled BOOLEAN)`,
		`CREATE TABLE enabled_vmbrs (name TEXT PRIMARY KEY, enabled BOOLEAN)`,
		`CREATE TABLE enabled_isos (name TEXT PRIMARY KEY, enabled BOOLEAN)`,
		`CREATE TABLE vm_profiles (id TEXT PRIMARY KEY, name TEXT, description TEXT, config TEXT, enabled BOOLEAN)`,
		`CREATE TABLE tags (name TEXT PRIMARY KEY)`,
		`CREATE TABLE vm_limits (id INTEGER PRIMARY KEY CHECK (id = 1), max_vms INTEGER, max_vm_per_user INTEGER, max_network_cards INTEGER, max_disk_per_vm INTEGER, allow_custom_yaml BOOLEAN, max_snapshots INTEGER)`,
	} {
		if _, err := badLegacy.ExecContext(ctx, ddl); err != nil {
			t.Fatalf("create table: %v", err)
		}
	}

	v04DB := openV04DB(t)

	_, runErr := recovery.Run(ctx, badLegacy, v04DB, recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        true,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if runErr == nil {
		t.Fatal("expected mapNodeLimits error, got nil")
	}
}

// --- Run: DryRun skips all writes but still reads ---

func TestRun_DryRunSkipsWrites(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, defaultSeed())
	v04DB := openV04DB(t)
	ctx := context.Background()

	// Use a non-"default" cluster name so the v0.4 schema's seeded
	// catalog_nodes rows (which target "default") don't interfere with
	// the dry-run write-skip assertion.
	sum, err := recovery.Run(ctx, legacyDB, v04DB, recovery.RunOptions{
		ClusterName:   "recover-test",
		DryRun:        true,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	})
	if err != nil {
		t.Fatalf("Run dry-run: %v", err)
	}

	// In dry-run mode, Written counts are populated but no rows are persisted.
	if sum.CatalogNodes.Read == 0 {
		t.Error("expected catalog_nodes read > 0 in dry-run")
	}

	// Verify nothing was actually written to the v0.4 DB for this cluster.
	if n := countRows(t, v04DB, `SELECT COUNT(*) FROM catalog_nodes WHERE cluster = ?`, "recover-test"); n != 0 {
		t.Errorf("catalog_nodes rows = %d, want 0 (dry-run)", n)
	}
}

// --- Run: idempotence (two non-dry runs produce identical counts) ---

func TestRun_IdempotentTwoRuns(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, defaultSeed())
	v04DB := openV04DB(t)
	ctx := context.Background()

	opts := recovery.RunOptions{
		ClusterName:   "default",
		DryRun:        false,
		SessionSecret: "test-session-secret-at-least-32-bytes!!",
	}

	if _, err := recovery.Run(ctx, legacyDB, v04DB, opts); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	firstCounts := snapshotCounts(t, v04DB)

	if _, err := recovery.Run(ctx, legacyDB, v04DB, opts); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	secondCounts := snapshotCounts(t, v04DB)
	for table, count := range firstCounts {
		if secondCounts[table] != count {
			t.Errorf("table %q: second run count = %d, want %d (idempotence)", table, secondCounts[table], count)
		}
	}
}
