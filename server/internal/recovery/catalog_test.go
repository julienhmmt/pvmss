//nolint:goconst // test fixtures reuse cluster/storage string literals across seed and assertion sites
package recovery_test

import (
	"context"
	"pvmss/server/internal/recovery"
	"testing"
)

// mockStorageResolver is a test StorageNodeResolver that returns a fixed node
// list (or error) for every storage name, exercising the live-expansion branch
// of recovery.MapCatalog.
type mockStorageResolver struct {
	nodes []string
	err   error
}

func (m mockStorageResolver) StorageNodes(_ context.Context, _ string) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}

	return m.nodes, nil
}

// TestMapCatalog_NilResolver skips storages with a named reason when no
// Proxmox connection is available, while still mapping nodes, bridges and isos.
func TestMapCatalog_NilResolver(t *testing.T) {
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
		Storages: []struct {
			name    string
			enabled bool
		}{
			{name: "local-lvm", enabled: true},
		},
		Bridges: []struct {
			name    string
			enabled bool
		}{
			{name: "vmbr0", enabled: true},
		},
		ISOs: []struct {
			name    string
			enabled bool
		}{
			{name: "local:iso/ubuntu-22.04.iso", enabled: true},
		},
	})

	nodes, storages, skips, bridges, isos, isoSkips, err := recovery.MapCatalog(
		context.Background(), legacyDB, "default", nil)
	if err != nil {
		t.Fatalf("MapCatalog: %v", err)
	}

	if len(nodes) != 2 {
		t.Errorf("len(nodes) = %d, want 2", len(nodes))
	}

	if len(storages) != 0 {
		t.Errorf("len(storages) = %d, want 0 (nil resolver skips all)", len(storages))
	}

	if len(skips) != 1 {
		t.Errorf("len(skips) = %d, want 1", len(skips))
	}

	if len(bridges) != 1 || bridges[0].Name != "vmbr0" {
		t.Errorf("bridges = %+v, want [vmbr0]", bridges)
	}

	if len(isos) != 1 || isos[0].Storage != "local" || isos[0].File != "iso/ubuntu-22.04.iso" {
		t.Errorf("isos = %+v, want [local/iso/ubuntu-22.04.iso]", isos)
	}

	if len(isoSkips) != 0 {
		t.Errorf("len(isoSkips) = %d, want 0", len(isoSkips))
	}
}

// TestMapCatalog_WithResolver expands storages onto the nodes returned by the
// resolver, covering the live-discovery branch of recovery.MapCatalog.
func TestMapCatalog_WithResolver(t *testing.T) {
	t.Parallel()

	legacyDB := openLegacyDB(t)
	seedLegacyDB(t, legacyDB, legacySeed{
		Nodes: []struct {
			name    string
			enabled bool
		}{
			{name: "pve-a", enabled: true},
		},
		Storages: []struct {
			name    string
			enabled bool
		}{
			{name: "local-lvm", enabled: true},
			{name: "nfs-share", enabled: false},
		},
	})

	resolver := mockStorageResolver{nodes: []string{"pve-a", "pve-b"}}

	_, storages, skips, _, _, _, err := recovery.MapCatalog(
		context.Background(), legacyDB, "default", resolver)
	if err != nil {
		t.Fatalf("MapCatalog: %v", err)
	}

	if len(storages) != 4 {
		t.Fatalf("len(storages) = %d, want 4 (2 storages × 2 resolved nodes)", len(storages))
	}

	for _, s := range storages {
		if s.Node != "pve-a" && s.Node != "pve-b" {
			t.Errorf("storage %q expanded onto unexpected node %q", s.Name, s.Node)
		}

		if s.Name == "nfs-share" && s.Enabled {
			t.Errorf("nfs-share should stay disabled, got Enabled=true")
		}
	}

	if len(skips) != 0 {
		t.Errorf("len(skips) = %d, want 0 (resolver resolved every storage)", len(skips))
	}
}

// TestMapCatalog_ResolverError skips a storage when live discovery fails,
// covering the error branch of recovery.MapCatalog.
func TestMapCatalog_ResolverError(t *testing.T) {
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

	resolver := mockStorageResolver{err: context.Canceled}

	_, storages, skips, _, _, _, err := recovery.MapCatalog(
		context.Background(), legacyDB, "default", resolver)
	if err != nil {
		t.Fatalf("MapCatalog: %v", err)
	}

	if len(storages) != 0 {
		t.Errorf("len(storages) = %d, want 0", len(storages))
	}

	if len(skips) != 1 {
		t.Errorf("len(skips) = %d, want 1 (discovery error)", len(skips))
	}
}
