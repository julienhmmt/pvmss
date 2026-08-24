package store_test

import (
	"context"
	"database/sql"
	"errors"
	"pvmss/server/internal/store"
	"testing"
)

const (
	catalogTestCluster     = "catalog-test"
	catalogTestNode        = "pve-node-01"
	catalogTestNodeAlpha   = "node-alpha"
	catalogTestNodeZeta    = "node-zeta"
	catalogTestNodeMid     = "node-mid"
	catalogTestStorageBus  = "scsi"
	catalogTestStorageName = "local-lvm"
	catalogTestStorageHost = "local"
	catalogTestISOFile     = "debian-12.iso"
	catalogTestBridgeName  = "vmbr0"
)

//nolint:paralleltest // migration fixtures are intentionally serial
func TestSetNodeEnabled_EnableAndDisable(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.SetNodeEnabled(ctx, catalogTestCluster, catalogTestNode, true); err != nil {
		t.Fatalf("SetNodeEnabled(true): %v", err)
	}

	nodes, err := st.CatalogNodesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogNodesEnabled: %v", err)
	}

	if len(nodes) != 1 {
		t.Fatalf("nodes count = %d, want 1", len(nodes))
	}

	if nodes[0].Name != catalogTestNode {
		t.Errorf("name = %q, want pve-node-01", nodes[0].Name)
	}

	if !nodes[0].Enabled {
		t.Error("enabled = false, want true")
	}

	if err := st.SetNodeEnabled(ctx, catalogTestCluster, catalogTestNode, false); err != nil {
		t.Fatalf("SetNodeEnabled(false): %v", err)
	}

	nodes, err = st.CatalogNodesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogNodesEnabled after disable: %v", err)
	}

	if nodes[0].Enabled {
		t.Error("enabled = true, want false after disable")
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestSetNodeEnabled_UpsertExisting(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.SetNodeEnabled(ctx, catalogTestCluster, "node-a", true); err != nil {
		t.Fatalf("SetNodeEnabled first: %v", err)
	}

	if err := st.SetNodeEnabled(ctx, catalogTestCluster, "node-a", true); err != nil {
		t.Fatalf("SetNodeEnabled second: %v", err)
	}

	nodes, err := st.CatalogNodesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogNodesEnabled: %v", err)
	}

	if len(nodes) != 1 {
		t.Fatalf("nodes count = %d, want 1 (upsert)", len(nodes))
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestCatalogNodesEnabled_Empty(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	nodes, err := st.CatalogNodesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogNodesEnabled: %v", err)
	}

	if len(nodes) != 0 {
		t.Fatalf("nodes count = %d, want 0", len(nodes))
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestCatalogNodesEnabled_MultipleSortedByName(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	for _, name := range []string{catalogTestNodeZeta, catalogTestNodeAlpha, catalogTestNodeMid} {
		if err := st.SetNodeEnabled(ctx, catalogTestCluster, name, true); err != nil {
			t.Fatalf("SetNodeEnabled(%q): %v", name, err)
		}
	}

	nodes, err := st.CatalogNodesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogNodesEnabled: %v", err)
	}

	if len(nodes) != 3 {
		t.Fatalf("nodes count = %d, want 3", len(nodes))
	}

	if nodes[0].Name != catalogTestNodeAlpha {
		t.Errorf("nodes[0].Name = %q, want node-alpha (sorted)", nodes[0].Name)
	}

	if nodes[1].Name != catalogTestNodeMid {
		t.Errorf("nodes[1].Name = %q, want node-mid", nodes[1].Name)
	}

	if nodes[2].Name != catalogTestNodeZeta {
		t.Errorf("nodes[2].Name = %q, want node-zeta", nodes[2].Name)
	}
}

//nolint:paralleltest,dupl // migration fixtures are intentionally serial; storage and bridge enable/disable cycles are intentionally parallel
func TestSetStorageEnabled_EnableAndDisable(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.SetStorageEnabled(ctx, catalogTestCluster, catalogTestStorageName, catalogTestNode, true); err != nil {
		t.Fatalf("SetStorageEnabled(true): %v", err)
	}

	storages, err := st.CatalogStoragesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogStoragesEnabled: %v", err)
	}

	if len(storages) != 1 {
		t.Fatalf("storages count = %d, want 1", len(storages))
	}

	if storages[0].Name != catalogTestStorageName {
		t.Errorf("name = %q, want local-lvm", storages[0].Name)
	}

	if storages[0].Node != catalogTestNode {
		t.Errorf("node = %q, want pve-node-01", storages[0].Node)
	}

	if !storages[0].Enabled {
		t.Error("enabled = false, want true")
	}

	if err := st.SetStorageEnabled(ctx, catalogTestCluster, catalogTestStorageName, catalogTestNode, false); err != nil {
		t.Fatalf("SetStorageEnabled(false): %v", err)
	}

	storages, err = st.CatalogStoragesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogStoragesEnabled after disable: %v", err)
	}

	if storages[0].Enabled {
		t.Error("enabled = true, want false after disable")
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestCatalogStoragesEnabled_Empty(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	storages, err := st.CatalogStoragesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogStoragesEnabled: %v", err)
	}

	if len(storages) != 0 {
		t.Fatalf("storages count = %d, want 0", len(storages))
	}
}

//nolint:paralleltest,dupl // migration fixtures are intentionally serial; storage and bridge enable/disable cycles are intentionally parallel
func TestSetBridgeEnabled_EnableAndDisable(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.SetBridgeEnabled(ctx, catalogTestCluster, catalogTestNode, catalogTestBridgeName, true); err != nil {
		t.Fatalf("SetBridgeEnabled(true): %v", err)
	}

	bridges, err := st.CatalogBridgesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogBridgesEnabled: %v", err)
	}

	if len(bridges) != 1 {
		t.Fatalf("bridges count = %d, want 1", len(bridges))
	}

	if bridges[0].Name != catalogTestBridgeName {
		t.Errorf("name = %q, want vmbr0", bridges[0].Name)
	}

	if bridges[0].Node != catalogTestNode {
		t.Errorf("node = %q, want pve-node-01", bridges[0].Node)
	}

	if !bridges[0].Enabled {
		t.Error("enabled = false, want true")
	}

	if err := st.SetBridgeEnabled(ctx, catalogTestCluster, catalogTestNode, catalogTestBridgeName, false); err != nil {
		t.Fatalf("SetBridgeEnabled(false): %v", err)
	}

	bridges, err = st.CatalogBridgesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogBridgesEnabled after disable: %v", err)
	}

	if bridges[0].Enabled {
		t.Error("enabled = true, want false after disable")
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestCatalogBridgesEnabled_Empty(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	bridges, err := st.CatalogBridgesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogBridgesEnabled: %v", err)
	}

	if len(bridges) != 0 {
		t.Fatalf("bridges count = %d, want 0", len(bridges))
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestSetISOEnabled_EnableAndDisable(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.SetISOEnabled(ctx, catalogTestCluster, catalogTestNode, catalogTestStorageHost, catalogTestISOFile, true); err != nil {
		t.Fatalf("SetISOEnabled(true): %v", err)
	}

	isos, err := st.CatalogISOsEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogISOsEnabled: %v", err)
	}

	if len(isos) != 1 {
		t.Fatalf("isos count = %d, want 1", len(isos))
	}

	if isos[0].File != "debian-12.iso" {
		t.Errorf("file = %q, want debian-12.iso", isos[0].File)
	}

	if isos[0].Node != catalogTestNode {
		t.Errorf("node = %q, want pve-node-01", isos[0].Node)
	}

	if isos[0].Storage != catalogTestStorageHost {
		t.Errorf("storage = %q, want local", isos[0].Storage)
	}

	if !isos[0].Enabled {
		t.Error("enabled = false, want true")
	}

	if err := st.SetISOEnabled(ctx, catalogTestCluster, catalogTestNode, catalogTestStorageHost, catalogTestISOFile, false); err != nil {
		t.Fatalf("SetISOEnabled(false): %v", err)
	}

	isos, err = st.CatalogISOsEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogISOsEnabled after disable: %v", err)
	}

	if isos[0].Enabled {
		t.Error("enabled = true, want false after disable")
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestCatalogISOsEnabled_Empty(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	isos, err := st.CatalogISOsEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogISOsEnabled: %v", err)
	}

	if len(isos) != 0 {
		t.Fatalf("isos count = %d, want 0", len(isos))
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestInsertProfile_AndList(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	values := store.ProfileValues{Label: "Small", CPUCores: 2, MemoryMB: 4096, DiskGB: 20, Bus: catalogTestStorageBus}
	if err := st.InsertProfile(ctx, catalogTestCluster, "small", values); err != nil {
		t.Fatalf("InsertProfile: %v", err)
	}

	profiles, err := st.CatalogProfilesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogProfilesEnabled: %v", err)
	}

	if len(profiles) != 1 {
		t.Fatalf("profiles count = %d, want 1", len(profiles))
	}

	if profiles[0].ID != "small" {
		t.Errorf("ID = %q, want small", profiles[0].ID)
	}

	if profiles[0].Label != "Small" {
		t.Errorf("Label = %q, want Small", profiles[0].Label)
	}

	if !profiles[0].Enabled {
		t.Error("Enabled = false, want true (default on insert)")
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestInsertProfile_Duplicate(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	values := store.ProfileValues{Label: "Small", CPUCores: 2, MemoryMB: 4096, DiskGB: 20, Bus: catalogTestStorageBus}
	if err := st.InsertProfile(ctx, catalogTestCluster, "small", values); err != nil {
		t.Fatalf("InsertProfile first: %v", err)
	}

	if err := st.InsertProfile(ctx, catalogTestCluster, "small", values); !errors.Is(err, store.ErrDuplicate) {
		t.Fatalf("InsertProfile duplicate error = %v, want ErrDuplicate", err)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestSetProfileEnabled_SuccessAndNotFound(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	values := store.ProfileValues{Label: "Medium", CPUCores: 4, MemoryMB: 8192, DiskGB: 40, Bus: catalogTestStorageBus}
	if err := st.InsertProfile(ctx, catalogTestCluster, "medium", values); err != nil {
		t.Fatalf("InsertProfile: %v", err)
	}

	if err := st.SetProfileEnabled(ctx, catalogTestCluster, "medium", false); err != nil {
		t.Fatalf("SetProfileEnabled(false): %v", err)
	}

	profiles, err := st.CatalogProfilesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogProfilesEnabled: %v", err)
	}

	if profiles[0].Enabled {
		t.Error("Enabled = true, want false")
	}

	if err := st.SetProfileEnabled(ctx, catalogTestCluster, "nonexistent", true); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("SetProfileEnabled(nonexistent) error = %v, want sql.ErrNoRows", err)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestUpdateProfile_SuccessAndNotFound(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	values := store.ProfileValues{Label: "Original", CPUCores: 2, MemoryMB: 2048, DiskGB: 10, Bus: catalogTestStorageBus}
	if err := st.InsertProfile(ctx, catalogTestCluster, "update-test", values); err != nil {
		t.Fatalf("InsertProfile: %v", err)
	}

	updated := store.ProfileValues{Label: "Updated", CPUCores: 8, MemoryMB: 16384, DiskGB: 80, Bus: "virtio"}
	if err := st.UpdateProfile(ctx, catalogTestCluster, "update-test", updated); err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}

	profiles, err := st.CatalogProfilesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogProfilesEnabled: %v", err)
	}

	p := profiles[0]
	if p.Label != "Updated" {
		t.Errorf("Label = %q, want Updated", p.Label)
	}

	if p.CPUCores != 8 {
		t.Errorf("CPUCores = %d, want 8", p.CPUCores)
	}

	if p.MemoryMB != 16384 {
		t.Errorf("MemoryMB = %d, want 16384", p.MemoryMB)
	}

	if p.DiskGB != 80 {
		t.Errorf("DiskGB = %d, want 80", p.DiskGB)
	}

	if p.Bus != "virtio" {
		t.Errorf("Bus = %q, want virtio", p.Bus)
	}

	if err := st.UpdateProfile(ctx, catalogTestCluster, "nonexistent", updated); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("UpdateProfile(nonexistent) error = %v, want sql.ErrNoRows", err)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestDeleteProfile_SuccessAndNotFound(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	values := store.ProfileValues{Label: "Delete Me", CPUCores: 1, MemoryMB: 1024, DiskGB: 5, Bus: catalogTestStorageBus}
	if err := st.InsertProfile(ctx, catalogTestCluster, "delete-test", values); err != nil {
		t.Fatalf("InsertProfile: %v", err)
	}

	if err := st.DeleteProfile(ctx, catalogTestCluster, "delete-test"); err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}

	profiles, err := st.CatalogProfilesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogProfilesEnabled: %v", err)
	}

	if len(profiles) != 0 {
		t.Fatalf("profiles count = %d, want 0 after delete", len(profiles))
	}

	if err := st.DeleteProfile(ctx, catalogTestCluster, "delete-test"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("DeleteProfile(nonexistent) error = %v, want sql.ErrNoRows", err)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestProfileExists_TrueAndFalse(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	values := store.ProfileValues{Label: "Exists", CPUCores: 1, MemoryMB: 1024, DiskGB: 5, Bus: catalogTestStorageBus}
	if err := st.InsertProfile(ctx, catalogTestCluster, "exists-test", values); err != nil {
		t.Fatalf("InsertProfile: %v", err)
	}

	exists, err := st.ProfileExists(ctx, catalogTestCluster, "exists-test")
	if err != nil {
		t.Fatalf("ProfileExists(true): %v", err)
	}

	if !exists {
		t.Error("ProfileExists = false, want true")
	}

	exists, err = st.ProfileExists(ctx, catalogTestCluster, "nonexistent")
	if err != nil {
		t.Fatalf("ProfileExists(false): %v", err)
	}

	if exists {
		t.Error("ProfileExists = true, want false")
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestInsertTag_AndList(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.InsertTag(ctx, catalogTestCluster, "env-prod", "#ff0000", "2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("InsertTag: %v", err)
	}

	tags, err := st.CatalogTags(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogTags: %v", err)
	}

	if len(tags) != 1 {
		t.Fatalf("tags count = %d, want 1", len(tags))
	}

	if tags[0].Name != "env-prod" {
		t.Errorf("Name = %q, want env-prod", tags[0].Name)
	}

	if tags[0].Color != "#ff0000" {
		t.Errorf("Color = %q, want #ff0000", tags[0].Color)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestInsertTag_Duplicate(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.InsertTag(ctx, catalogTestCluster, "env-prod", "#ff0000", "2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("InsertTag first: %v", err)
	}

	if err := st.InsertTag(ctx, catalogTestCluster, "env-prod", "#00ff00", "2024-01-02T00:00:00Z"); !errors.Is(err, store.ErrDuplicate) {
		t.Fatalf("InsertTag duplicate error = %v, want ErrDuplicate", err)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestUpdateTagColor_SuccessAndNotFound(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.InsertTag(ctx, catalogTestCluster, "env-dev", "#0000ff", "2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("InsertTag: %v", err)
	}

	if err := st.UpdateTagColor(ctx, catalogTestCluster, "env-dev", "#00ff00"); err != nil {
		t.Fatalf("UpdateTagColor: %v", err)
	}

	tags, err := st.CatalogTags(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogTags: %v", err)
	}

	if tags[0].Color != "#00ff00" {
		t.Errorf("Color = %q, want #00ff00", tags[0].Color)
	}

	if err := st.UpdateTagColor(ctx, catalogTestCluster, "nonexistent", "#ffffff"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("UpdateTagColor(nonexistent) error = %v, want sql.ErrNoRows", err)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestDeleteTag_SuccessAndNotFound(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.InsertTag(ctx, catalogTestCluster, "env-test", "#aaaaaa", "2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("InsertTag: %v", err)
	}

	if err := st.DeleteTag(ctx, catalogTestCluster, "env-test"); err != nil {
		t.Fatalf("DeleteTag: %v", err)
	}

	tags, err := st.CatalogTags(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogTags: %v", err)
	}

	if len(tags) != 0 {
		t.Fatalf("tags count = %d, want 0 after delete", len(tags))
	}

	if err := st.DeleteTag(ctx, catalogTestCluster, "env-test"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("DeleteTag(nonexistent) error = %v, want sql.ErrNoRows", err)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestTagExists_TrueAndFalse(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.InsertTag(ctx, catalogTestCluster, "env-staging", "#ffaa00", "2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("InsertTag: %v", err)
	}

	exists, err := st.TagExists(ctx, catalogTestCluster, "env-staging")
	if err != nil {
		t.Fatalf("TagExists(true): %v", err)
	}

	if !exists {
		t.Error("TagExists = false, want true")
	}

	exists, err = st.TagExists(ctx, catalogTestCluster, "nonexistent")
	if err != nil {
		t.Fatalf("TagExists(false): %v", err)
	}

	if exists {
		t.Error("TagExists = true, want false")
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestCatalogTags_Empty(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	tags, err := st.CatalogTags(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogTags: %v", err)
	}

	if len(tags) != 0 {
		t.Fatalf("tags count = %d, want 0", len(tags))
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestCatalogProfilesEnabled_Empty(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	profiles, err := st.CatalogProfilesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogProfilesEnabled: %v", err)
	}

	if len(profiles) != 0 {
		t.Fatalf("profiles count = %d, want 0", len(profiles))
	}
}
