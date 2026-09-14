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

//nolint:paralleltest // migration fixtures are intentionally serial
func TestSetImageEnabled_AndList(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.SetImageEnabled(ctx, catalogTestCluster, catalogTestNode, catalogTestStorageHost, "ubuntu.qcow2", 2048, true); err != nil {
		t.Fatalf("SetImageEnabled: %v", err)
	}

	images, err := st.CatalogImagesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogImagesEnabled: %v", err)
	}

	if len(images) != 1 {
		t.Fatalf("images count = %d, want 1", len(images))
	}

	img := images[0]
	if img.Node != catalogTestNode || img.Storage != catalogTestStorageHost || img.File != "ubuntu.qcow2" {
		t.Errorf("image key = (%q, %q, %q)", img.Node, img.Storage, img.File)
	}

	if img.SizeBytes != 2048 {
		t.Errorf("size_bytes = %d, want 2048", img.SizeBytes)
	}

	if !img.Enabled {
		t.Error("enabled = false, want true")
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestSetImageEnabled_Upsert(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.SetImageEnabled(ctx, catalogTestCluster, catalogTestNode, catalogTestStorageHost, "ubuntu.qcow2", 2048, true); err != nil {
		t.Fatalf("SetImageEnabled: %v", err)
	}

	// Upsert flips enabled and refreshes the discovered size.
	if err := st.SetImageEnabled(ctx, catalogTestCluster, catalogTestNode, catalogTestStorageHost, "ubuntu.qcow2", 4096, false); err != nil {
		t.Fatalf("SetImageEnabled upsert: %v", err)
	}

	images, err := st.CatalogImagesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogImagesEnabled after upsert: %v", err)
	}

	if len(images) != 1 || images[0].Enabled || images[0].SizeBytes != 4096 {
		t.Errorf("after upsert: %+v, want one disabled image of 4096 bytes", images)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestDeleteApprovals_SuccessAndNotFound(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	seed := []struct {
		name string
		set  func() error
		del  func() error
	}{
		{
			"node",
			func() error { return st.SetNodeEnabled(ctx, catalogTestCluster, catalogTestNode, true) },
			func() error { return st.DeleteNode(ctx, catalogTestCluster, catalogTestNode) },
		},
		{
			"storage",
			func() error {
				return st.SetStorageEnabled(ctx, catalogTestCluster, catalogTestStorageName, catalogTestNode, true)
			},
			func() error {
				return st.DeleteStorage(ctx, catalogTestCluster, catalogTestStorageName, catalogTestNode)
			},
		},
		{
			"bridge",
			func() error {
				return st.SetBridgeEnabled(ctx, catalogTestCluster, catalogTestNode, catalogTestBridgeName, true)
			},
			func() error {
				return st.DeleteBridge(ctx, catalogTestCluster, catalogTestNode, catalogTestBridgeName)
			},
		},
		{
			"iso",
			func() error {
				return st.SetISOEnabled(ctx, catalogTestCluster, catalogTestNode, catalogTestStorageHost, catalogTestISOFile, true)
			},
			func() error {
				return st.DeleteISO(ctx, catalogTestCluster, catalogTestNode, catalogTestStorageHost, catalogTestISOFile)
			},
		},
		{
			"image",
			func() error {
				return st.SetImageEnabled(ctx, catalogTestCluster, catalogTestNode, catalogTestStorageHost, "ubuntu.qcow2", 1024, true)
			},
			func() error {
				return st.DeleteImage(ctx, catalogTestCluster, catalogTestNode, catalogTestStorageHost, "ubuntu.qcow2")
			},
		},
	}

	for _, tt := range seed {
		if err := tt.set(); err != nil {
			t.Fatalf("%s: seed: %v", tt.name, err)
		}

		if err := tt.del(); err != nil {
			t.Errorf("%s: delete existing: %v", tt.name, err)
		}

		if err := tt.del(); !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("%s: delete missing = %v, want sql.ErrNoRows", tt.name, err)
		}
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestInsertTemplate_AndList(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	values := store.TemplateValues{
		Node:              catalogTestNode,
		Name:              "debian-13-tmpl",
		CloudInitCapable:  true,
		DiskStorage:       catalogTestStorageName,
		DiskSizeGB:        8,
		DiskBus:           catalogTestStorageBus,
		OverrideDiscovery: true,
	}
	if err := st.InsertTemplate(ctx, catalogTestCluster, 9001, values, true); err != nil {
		t.Fatalf("InsertTemplate: %v", err)
	}

	templates, err := st.CatalogTemplatesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogTemplatesEnabled: %v", err)
	}

	if len(templates) != 1 {
		t.Fatalf("templates count = %d, want 1", len(templates))
	}

	tmpl := templates[0]
	if tmpl.VMID != 9001 || tmpl.Node != catalogTestNode || tmpl.Name != "debian-13-tmpl" {
		t.Errorf("template key = (%d, %q, %q)", tmpl.VMID, tmpl.Node, tmpl.Name)
	}

	if !tmpl.CloudInitCapable || !tmpl.OverrideDiscovery || !tmpl.Enabled {
		t.Errorf("flags = capable:%v override:%v enabled:%v, want all true", tmpl.CloudInitCapable, tmpl.OverrideDiscovery, tmpl.Enabled)
	}

	if tmpl.DiskStorage != catalogTestStorageName || tmpl.DiskSizeGB != 8 || tmpl.DiskBus != catalogTestStorageBus {
		t.Errorf("disk = (%q, %d, %q)", tmpl.DiskStorage, tmpl.DiskSizeGB, tmpl.DiskBus)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestInsertTemplate_Duplicate(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	values := store.TemplateValues{Node: catalogTestNode, Name: "tmpl"}
	if err := st.InsertTemplate(ctx, catalogTestCluster, 9002, values, true); err != nil {
		t.Fatalf("InsertTemplate first: %v", err)
	}

	if err := st.InsertTemplate(ctx, catalogTestCluster, 9002, values, true); !errors.Is(err, store.ErrDuplicate) {
		t.Errorf("InsertTemplate duplicate = %v, want ErrDuplicate", err)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestUpdateTemplate_SuccessAndNotFound(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	initial := store.TemplateValues{Node: catalogTestNode, Name: "tmpl-old"}
	if err := st.InsertTemplate(ctx, catalogTestCluster, 9003, initial, true); err != nil {
		t.Fatalf("InsertTemplate: %v", err)
	}

	updated := store.TemplateValues{
		Node:              catalogTestNodeAlpha,
		Name:              "tmpl-new",
		CloudInitCapable:  true,
		DiskStorage:       catalogTestStorageName,
		DiskSizeGB:        16,
		DiskBus:           catalogTestStorageBus,
		OverrideDiscovery: true,
	}
	if err := st.UpdateTemplate(ctx, catalogTestCluster, 9003, updated); err != nil {
		t.Fatalf("UpdateTemplate: %v", err)
	}

	templates, err := st.CatalogTemplatesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogTemplatesEnabled: %v", err)
	}

	if len(templates) != 1 {
		t.Fatalf("templates count = %d, want 1", len(templates))
	}

	if templates[0].Name != "tmpl-new" || !templates[0].OverrideDiscovery {
		t.Errorf("template after update = %+v", templates[0])
	}

	if err := st.UpdateTemplate(ctx, catalogTestCluster, 9999, updated); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("UpdateTemplate missing = %v, want sql.ErrNoRows", err)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestDeleteTemplate_SuccessAndNotFound(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.InsertTemplate(ctx, catalogTestCluster, 9004, store.TemplateValues{Node: catalogTestNode, Name: "tmpl"}, true); err != nil {
		t.Fatalf("InsertTemplate: %v", err)
	}

	if err := st.DeleteTemplate(ctx, catalogTestCluster, 9004); err != nil {
		t.Fatalf("DeleteTemplate: %v", err)
	}

	if err := st.DeleteTemplate(ctx, catalogTestCluster, 9004); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("DeleteTemplate missing = %v, want sql.ErrNoRows", err)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestSetTemplateEnabled_Upsert(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.SetTemplateEnabled(ctx, catalogTestCluster, 9005, true); err != nil {
		t.Fatalf("SetTemplateEnabled insert: %v", err)
	}

	if err := st.SetTemplateEnabled(ctx, catalogTestCluster, 9005, false); err != nil {
		t.Fatalf("SetTemplateEnabled update: %v", err)
	}

	templates, err := st.CatalogTemplatesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogTemplatesEnabled: %v", err)
	}

	if len(templates) != 1 || templates[0].Enabled {
		t.Errorf("templates = %+v, want one disabled template", templates)
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestCatalogTemplatesEnabled_Empty(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	templates, err := st.CatalogTemplatesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogTemplatesEnabled: %v", err)
	}

	if len(templates) != 0 {
		t.Fatalf("templates count = %d, want 0", len(templates))
	}
}

//nolint:paralleltest // migration fixtures are intentionally serial
func TestCatalogImagesEnabled_Empty(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	images, err := st.CatalogImagesEnabled(ctx, catalogTestCluster)
	if err != nil {
		t.Fatalf("CatalogImagesEnabled: %v", err)
	}

	if len(images) != 0 {
		t.Fatalf("images count = %d, want 0", len(images))
	}
}
