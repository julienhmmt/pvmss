//nolint:goconst // test fixtures reuse cluster/tag/profile string literals across seed and assertion sites
package recovery_test

import (
	"context"
	"pvmss/server/internal/recovery"
	"testing"
)

// T007: enabled_nodes/enabled_storages/enabled_vmbrs/enabled_isos → catalog_* mapping.
func TestMapNodes_DirectCopy(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Nodes: []struct {
			name    string
			enabled bool
		}{
			{name: "pve-a", enabled: true},
			{name: "pve-b", enabled: false},
		},
	})

	nodes, err := recovery.MapNodesForTest(context.Background(), legacyDB)
	if err != nil {
		t.Fatalf("MapNodes: %v", err)
	}

	if len(nodes) != 2 {
		t.Fatalf("len(nodes) = %d, want 2", len(nodes))
	}

	if nodes[0].Name != "pve-a" || !nodes[0].Enabled {
		t.Errorf("nodes[0] = %+v, want {pve-a true}", nodes[0])
	}

	if nodes[1].Name != "pve-b" || nodes[1].Enabled {
		t.Errorf("nodes[1] = %+v, want {pve-b false}", nodes[1])
	}
}

func TestMapBridges_DirectCopy(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Bridges: []struct {
			name    string
			enabled bool
		}{
			{name: "vmbr0", enabled: true},
			{name: "vmbr1", enabled: true},
		},
	})

	bridges, err := recovery.MapBridgesForTest(context.Background(), legacyDB)
	if err != nil {
		t.Fatalf("MapBridges: %v", err)
	}

	if len(bridges) != 2 {
		t.Fatalf("len(bridges) = %d, want 2", len(bridges))
	}
}

// T007: ISO volid-split failure case.
func TestMapISOs_VolidSplit(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		ISOs: []struct {
			name    string
			enabled bool
		}{
			{name: "local:iso/ubuntu-22.04.iso", enabled: true},
			{name: "nfs:iso/debian-12.iso", enabled: true},
			{name: "bad-iso-name-no-colon", enabled: true},
		},
	})

	isos, skips, err := recovery.MapISOsForTest(context.Background(), legacyDB)
	if err != nil {
		t.Fatalf("MapISOs: %v", err)
	}

	if len(isos) != 2 {
		t.Fatalf("len(isos) = %d, want 2 (valid volids)", len(isos))
	}

	if isos[0].Storage != "local" || isos[0].File != "iso/ubuntu-22.04.iso" {
		t.Errorf("isos[0] = %+v, want {local iso/ubuntu-22.04.iso}", isos[0])
	}

	if len(skips) != 1 {
		t.Fatalf("len(skips) = %d, want 1 (no-colon row)", len(skips))
	}

	if skips[0].Row != "bad-iso-name-no-colon" {
		t.Errorf("skips[0].Row = %q, want %q", skips[0].Row, "bad-iso-name-no-colon")
	}
}

func TestSplitISOVolid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		storage string
		file    string
		ok      bool
	}{
		{"standard", "local:iso/ubuntu.iso", "local", "iso/ubuntu.iso", true},
		{"nfs", "nfs:iso/debian-12.iso", "nfs", "iso/debian-12.iso", true},
		{"no colon", "plain-name", "", "", false},
		{"colon at start", ":iso/file.iso", "", "", false},
		{"colon at end", "local:", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, f, ok := recovery.SplitISOVolidForTest(tt.input)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}

			if s != tt.storage || f != tt.file {
				t.Errorf("storage=%q file=%q, want storage=%q file=%q", s, f, tt.storage, tt.file)
			}
		})
	}
}

// T007: storage-node expansion skipped when unreachable (resolver returns error).
func TestMapStorages_WithResolver(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Storages: []struct {
			name    string
			enabled bool
		}{
			{name: "local-lvm", enabled: true},
			{name: "nfs-share", enabled: true},
		},
	})

	resolver := stubStorageResolver{
		nodes: map[string][]string{
			"local-lvm": {"pve-a", "pve-b"},
			"nfs-share": {"pve-a"},
		},
	}

	storages, skips, err := recovery.MapStoragesForTest(context.Background(), legacyDB, "default", resolver)
	if err != nil {
		t.Fatalf("MapStorages: %v", err)
	}

	if len(storages) != 3 {
		t.Fatalf("len(storages) = %d, want 3 (2+1 node expansions)", len(storages))
	}

	if len(skips) != 0 {
		t.Fatalf("len(skips) = %d, want 0", len(skips))
	}
}

func TestMapStorages_NilResolver_SkipsAll(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Storages: []struct {
			name    string
			enabled bool
		}{
			{name: "local-lvm", enabled: true},
			{name: "nfs-share", enabled: true},
		},
	})

	storages, skips, err := recovery.MapStoragesForTest(context.Background(), legacyDB, "default", nil)
	if err != nil {
		t.Fatalf("MapStorages: %v", err)
	}

	if len(storages) != 0 {
		t.Fatalf("len(storages) = %d, want 0 (no resolver)", len(storages))
	}

	if len(skips) != 2 {
		t.Fatalf("len(skips) = %d, want 2", len(skips))
	}
}

func TestMapStorages_ResolverError_SkipsWithError(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Storages: []struct {
			name    string
			enabled bool
		}{
			{name: "local-lvm", enabled: true},
		},
	})

	resolver := stubStorageResolver{err: context.DeadlineExceeded}

	storages, skips, err := recovery.MapStoragesForTest(context.Background(), legacyDB, "default", resolver)
	if err != nil {
		t.Fatalf("MapStorages: %v", err)
	}

	if len(storages) != 0 {
		t.Fatalf("len(storages) = %d, want 0", len(storages))
	}

	if len(skips) != 1 {
		t.Fatalf("len(skips) = %d, want 1", len(skips))
	}
}

func TestMapStorages_NoNodesReporting_Skips(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Storages: []struct {
			name    string
			enabled bool
		}{
			{name: "old-nfs", enabled: true},
		},
	})

	resolver := stubStorageResolver{nodes: map[string][]string{"old-nfs": {}}}

	storages, skips, err := recovery.MapStoragesForTest(context.Background(), legacyDB, "default", resolver)
	if err != nil {
		t.Fatalf("MapStorages: %v", err)
	}

	if len(storages) != 0 {
		t.Fatalf("len(storages) = %d, want 0", len(storages))
	}

	if len(skips) != 1 {
		t.Fatalf("len(skips) = %d, want 1", len(skips))
	}
}

// T007: upsert into v0.4 catalog tables.
func TestUpsertNode_WritesAndIsIdempotent(t *testing.T) {
	t.Parallel()

	v04DB := openV04DB(t)
	ctx := context.Background()

	r := recovery.NodeRow{Name: "pve-a", Enabled: true}
	if err := recovery.UpsertNodeForTest(ctx, v04DB, "default", r); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	if err := recovery.UpsertNodeForTest(ctx, v04DB, "default", r); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if count := countRows(t, v04DB, `SELECT COUNT(*) FROM catalog_nodes WHERE cluster = ? AND name = ?`, "default", "pve-a"); count != 1 {
		t.Errorf("row count = %d, want 1", count)
	}
}

func TestUpsertStorage_WritesAndIsIdempotent(t *testing.T) {
	t.Parallel()

	v04DB := openV04DB(t)
	ctx := context.Background()

	r := recovery.StorageRow{Name: "local-lvm", Node: "pve-a", Enabled: true}
	if err := recovery.UpsertStorageForTest(ctx, v04DB, "default", r); err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	if err := recovery.UpsertStorageForTest(ctx, v04DB, "default", r); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if count := countRows(t, v04DB, `SELECT COUNT(*) FROM catalog_storages WHERE cluster = ? AND name = ? AND node = ?`, "default", "local-lvm", "pve-a"); count != 1 {
		t.Errorf("row count = %d, want 1", count)
	}
}
