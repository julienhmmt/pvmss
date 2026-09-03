package catalog_test

import (
	"context"
	"errors"
	"path/filepath"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/store"
	"testing"
)

const storageLocalLVM = "local-lvm"

// openAdminStore opens a fully-migrated store (V9) with the T06 seed and the
// pvmss tag, ready for admin catalog operations.
func openAdminStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.Open(config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "admin.db"),
		LogLevel:  "info",
		LogFormat: "json",
		LogOutput: "stdout",
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	return st
}

// findApprovalNode returns the NodeApproval with the given name, or fails.
func findApprovalNode(t *testing.T, nodes []catalog.NodeApproval, name string) catalog.NodeApproval {
	t.Helper()

	for _, n := range nodes {
		if n.Name == name {
			return n
		}
	}

	t.Fatalf("node %q not found in approval list", name)

	return catalog.NodeApproval{}
}

// findApprovalStorage returns the StorageApproval matching (name, node).
func findApprovalStorage(t *testing.T, storages []catalog.StorageApproval, name, node string) catalog.StorageApproval {
	t.Helper()

	for _, s := range storages {
		if s.Name == name && s.Node == node {
			return s
		}
	}

	t.Fatalf("storage %q@%q not found in approval list", name, node)

	return catalog.StorageApproval{}
}

func findApprovalBridge(t *testing.T, bridges []catalog.BridgeApproval, name, node string) catalog.BridgeApproval {
	t.Helper()

	for _, bridge := range bridges {
		if bridge.Name == name && bridge.Node == node {
			return bridge
		}
	}

	t.Fatalf("bridge %q on %q not found in approval list", name, node)

	return catalog.BridgeApproval{}
}

// TestAdminListNodes_IncludesAllDiscoveredNodes verifies that AdminListNodes
// returns every node from the cluster snapshot (3 fake nodes), not just the
// approved ones — the unapproved node reports enabled=false.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminListNodes_IncludesAllDiscoveredNodes(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	nodes, err := catalog.AdminListNodes(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListNodes: %v", err)
	}

	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d: %+v", len(nodes), nodes)
	}

	approved := findApprovalNode(t, nodes, "pve-node-01")
	if !approved.Enabled {
		t.Error("pve-node-01 should be enabled (T06 seed)")
	}

	approved2 := findApprovalNode(t, nodes, "pve-node-02")
	if !approved2.Enabled {
		t.Error("pve-node-02 should be enabled (T06 seed)")
	}

	unapproved := findApprovalNode(t, nodes, "pve-node-03")
	if unapproved.Enabled {
		t.Error("pve-node-03 should not be enabled (not in T06 seed)")
	}
}

// TestSetNodeEnabled_UpsertNeverDeletes verifies toggling is an idempotent
// upsert — a resource discovered but never toggled reports enabled=false;
// toggling on then off persists the row.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestSetNodeEnabled_UpsertNeverDeletes(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	// Before toggle: pve-node-03 is discovered but not approved.
	nodes, err := catalog.AdminListNodes(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListNodes: %v", err)
	}

	before := findApprovalNode(t, nodes, "pve-node-03")
	if before.Enabled {
		t.Fatal("pve-node-03 should start disabled")
	}

	// Toggle on.
	if err := catalog.SetNodeEnabled(ctx, st, cluster.Fake{}, "default", "pve-node-03", true); err != nil {
		t.Fatalf("SetNodeEnabled true: %v", err)
	}

	after, err := catalog.AdminListNodes(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListNodes after toggle: %v", err)
	}

	on := findApprovalNode(t, after, "pve-node-03")
	if !on.Enabled {
		t.Fatal("pve-node-03 should be enabled after toggle on")
	}

	// Toggle off — row persists, enabled=false.
	if err := catalog.SetNodeEnabled(ctx, st, cluster.Fake{}, "default", "pve-node-03", false); err != nil {
		t.Fatalf("SetNodeEnabled false: %v", err)
	}

	off, err := catalog.AdminListNodes(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListNodes after toggle off: %v", err)
	}

	offNode := findApprovalNode(t, off, "pve-node-03")
	if offNode.Enabled {
		t.Fatal("pve-node-03 should be disabled after toggle off")
	}

	// Cross-tranche proof: ApprovedResources (T06) excludes disabled nodes.
	resources, err := catalog.ApprovedResources(ctx, st, "default")
	if err != nil {
		t.Fatalf("ApprovedResources: %v", err)
	}

	for _, n := range resources.Nodes {
		if n.Name == "pve-node-03" {
			t.Fatal("pve-node-03 should not be in ApprovedResources when disabled")
		}
	}
}

// TestAdminListStorages_ExcludesPBSAndBackupOnly verifies non-VM storage is hidden.
//
//nolint:paralleltest // serial: shared fake dataset
func TestAdminListStorages_ExcludesPBSAndBackupOnly(t *testing.T) {
	st := openAdminStore(t)

	storages, err := catalog.AdminListStorages(context.Background(), st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListStorages: %v", err)
	}

	for _, storage := range storages {
		if storage.Name == cluster.FakeStorageBackupNFS || storage.Name == cluster.FakeStoragePBS {
			t.Errorf("ineligible storage %q returned in admin catalog: %+v", storage.Name, storage)
		}
	}

	if len(storages) != 4 {
		t.Errorf("got %d VM-capable storages, want 4: %+v", len(storages), storages)
	}
}

// TestSetStorageEnabled_PerPairIsolation verifies toggling one storage+node
// pair does not affect a same-named pair on a different node (Acceptance
// Scenario 1.3).
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestSetStorageEnabled_PerPairIsolation(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	// local@pve-node-02 is approved (T06 seed); local@pve-node-01 is not.
	storages, err := catalog.AdminListStorages(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListStorages: %v", err)
	}

	node02Local := findApprovalStorage(t, storages, "local", "pve-node-02")
	if !node02Local.Enabled {
		t.Error("local@pve-node-02 should be enabled (T06 seed)")
	}

	node01Local := findApprovalStorage(t, storages, "local", "pve-node-01")
	if node01Local.Enabled {
		t.Error("local@pve-node-01 should not be enabled")
	}

	// Toggle local@pve-node-02 off.
	if err := catalog.SetStorageEnabled(ctx, st, cluster.Fake{}, "default", "local", "pve-node-02", false); err != nil {
		t.Fatalf("SetStorageEnabled: %v", err)
	}

	// local@pve-node-01 is unaffected — still disabled (its own state).
	if err := catalog.SetStorageEnabled(ctx, st, cluster.Fake{}, "default", "local", "pve-node-01", true); err != nil {
		t.Fatalf("SetStorageEnabled pve-node-01: %v", err)
	}

	after, err := catalog.AdminListStorages(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListStorages after: %v", err)
	}

	node02After := findApprovalStorage(t, after, "local", "pve-node-02")
	if node02After.Enabled {
		t.Error("local@pve-node-02 should be disabled after toggle off")
	}

	node01After := findApprovalStorage(t, after, "local", "pve-node-01")
	if !node01After.Enabled {
		t.Error("local@pve-node-01 should be enabled after toggle on")
	}
}

// TestAdminListBridges_IncludesSuperset verifies the admin bridge list
// includes the fake superset after the node-aware migration resets approvals.
//
//nolint:paralleltest // serial: shared fake dataset
func TestAdminListBridges_IncludesSuperset(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	bridges, err := catalog.AdminListBridges(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListBridges: %v", err)
	}

	if len(bridges) != 3 {
		t.Fatalf("expected 3 bridges, got %d: %+v", len(bridges), bridges)
	}

	for _, b := range bridges {
		if b.Enabled {
			t.Errorf("bridge %q on %q should not be enabled", b.Name, b.Node)
		}
	}
}

type duplicateBridgeClient struct {
	cluster.Fake
}

func (duplicateBridgeClient) ListBridges(_ context.Context) ([]cluster.Bridge, error) {
	return []cluster.Bridge{
		{Name: bridgeVMbr0, Node: node01, Active: true},
		{Name: bridgeVMbr0, Node: node02, Active: true},
	}, nil
}

//nolint:paralleltest // serial: shared database fixture
func TestSetBridgeEnabled_SameNameOnTwoNodesTogglesIndependently(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()
	client := duplicateBridgeClient{}

	if err := catalog.SetBridgeEnabled(ctx, st, client, "default", node01, bridgeVMbr0, true); err != nil {
		t.Fatalf("SetBridgeEnabled vmbr0: %v", err)
	}

	bridges, err := catalog.AdminListBridges(ctx, st, client, "default")
	if err != nil {
		t.Fatalf("AdminListBridges: %v", err)
	}

	if !findApprovalBridge(t, bridges, bridgeVMbr0, node01).Enabled {
		t.Error("vmbr0 on pve-node-01 should be enabled")
	}

	if findApprovalBridge(t, bridges, bridgeVMbr0, node02).Enabled {
		t.Error("vmbr0 on pve-node-02 should remain disabled")
	}
}

// TestAdminListISOs_IncludesSuperset verifies the admin ISO list includes the
// fake superset after the node-aware migration resets approvals.
//
//nolint:paralleltest // serial: shared fake dataset
func TestAdminListISOs_IncludesSuperset(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	isos, err := catalog.AdminListISOs(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListISOs: %v", err)
	}

	if len(isos) != 3 {
		t.Fatalf("expected 3 ISOs, got %d: %+v", len(isos), isos)
	}

	for _, i := range isos {
		if i.Enabled {
			t.Errorf("ISO %q on %q should not be enabled", i.File, i.Node)
		}
	}
}

// TestSetISOEnabled_ToggleIsolatesByStorageFile verifies toggling one ISO does
// not affect another.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestSetISOEnabled_ToggleIsolatesByStorageFile(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	if err := catalog.SetISOEnabled(ctx, st, cluster.Fake{}, "default", catalog.ISORef{Node: node02, Storage: storageLocal, File: "rocky-9-generic-x86_64.iso"}, true); err != nil {
		t.Fatalf("SetISOEnabled rocky-9: %v", err)
	}

	isos, err := catalog.AdminListISOs(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListISOs: %v", err)
	}

	for _, i := range isos {
		if i.File == "rocky-9-generic-x86_64.iso" && !i.Enabled {
			t.Error("rocky-9 should be enabled after toggle")
		}

		if i.File == "debian-12-generic-amd64.iso" && i.Enabled {
			t.Error("debian-12 should still be disabled (unaffected)")
		}
	}
}

// TestSetNodeEnabled_UnknownNodeReturnsError verifies that toggling a node not
// in the current discovery set returns an error (404 at the handler level).
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestSetNodeEnabled_UnknownNodeReturnsError(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	err := catalog.SetNodeEnabled(ctx, st, cluster.Fake{}, "default", "pve-node-99", true)
	if err == nil {
		t.Fatal("expected error for unknown node, got nil")
	}
}

// TestSetStorageEnabled_UnknownReturnsError — toggling a (name, node) pair not
// in the current discovery set returns cluster.ErrNotFound (404 at the handler
// level), mirroring SetNodeEnabled's contract.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestSetStorageEnabled_UnknownReturnsError(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	if err := catalog.SetStorageEnabled(ctx, st, cluster.Fake{}, "default", "missing", "pve-node-01", true); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("SetStorageEnabled unknown: got %v, want cluster.ErrNotFound", err)
	}

	// Right name, wrong node — still not discovered.
	if err := catalog.SetStorageEnabled(ctx, st, cluster.Fake{}, "default", "local", "pve-node-99", true); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("SetStorageEnabled wrong node: got %v, want cluster.ErrNotFound", err)
	}
}

// TestSetBridgeEnabled_UnknownReturnsError — toggling a bridge not in the
// current discovery set returns cluster.ErrNotFound.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestSetBridgeEnabled_UnknownReturnsError(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	if err := catalog.SetBridgeEnabled(ctx, st, cluster.Fake{}, "default", "pve-node-01", "vmbr99", true); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("SetBridgeEnabled unknown: got %v, want cluster.ErrNotFound", err)
	}

	if err := catalog.SetBridgeEnabled(ctx, st, cluster.Fake{}, "default", "pve-node-02", "vmbr0", true); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("SetBridgeEnabled wrong node: got %v, want cluster.ErrNotFound", err)
	}
}

// TestSetISOEnabled_UnknownReturnsError — toggling an ISO (node, storage, file)
// triple not in the current discovery set returns cluster.ErrNotFound.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestSetISOEnabled_UnknownReturnsError(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	if err := catalog.SetISOEnabled(ctx, st, cluster.Fake{}, "default", catalog.ISORef{Node: node01, Storage: storageLocal, File: "missing.iso"}, true); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("SetISOEnabled unknown file: got %v, want cluster.ErrNotFound", err)
	}

	// Right file, wrong storage — still not discovered.
	if err := catalog.SetISOEnabled(ctx, st, cluster.Fake{}, "default", catalog.ISORef{Node: node01, Storage: "missing", File: debianGenericISO}, true); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("SetISOEnabled wrong storage: got %v, want cluster.ErrNotFound", err)
	}

	// Right file and storage, wrong node — still not discovered.
	if err := catalog.SetISOEnabled(ctx, st, cluster.Fake{}, "default", catalog.ISORef{Node: "pve-node-99", Storage: storageLocal, File: debianGenericISO}, true); !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("SetISOEnabled wrong node: got %v, want cluster.ErrNotFound", err)
	}
}

// TestSetStorageEnabled_TogglePersists — toggling a discovered storage off then
// on persists the enabled state across reads (the upsert-never-deletes
// contract), mirroring TestSetNodeEnabled_UpsertNeverDeletes.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestSetStorageEnabled_TogglePersists(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	// local@pve-node-01 is discovered but not approved (T06 seed approves only
	// local@pve-node-02). Toggle it on, then off, asserting each step.
	if err := catalog.SetStorageEnabled(ctx, st, cluster.Fake{}, "default", "local", "pve-node-01", true); err != nil {
		t.Fatalf("SetStorageEnabled on: %v", err)
	}

	after, err := catalog.AdminListStorages(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListStorages: %v", err)
	}

	if !findApprovalStorage(t, after, "local", "pve-node-01").Enabled {
		t.Error("local@pve-node-01 should be enabled after toggle on")
	}

	if err := catalog.SetStorageEnabled(ctx, st, cluster.Fake{}, "default", "local", "pve-node-01", false); err != nil {
		t.Fatalf("SetStorageEnabled off: %v", err)
	}

	after, err = catalog.AdminListStorages(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListStorages after off: %v", err)
	}

	if findApprovalStorage(t, after, "local", "pve-node-01").Enabled {
		t.Error("local@pve-node-01 should be disabled after toggle off")
	}
}

// TestSetBridgeEnabled_ToggleOffPersists — toggling an approved bridge off keeps
// the row (enabled=false), then re-enabling restores it.
//
//nolint:paralleltest // serial: shared fixture
func TestSetBridgeEnabled_ToggleOffPersists(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	if err := catalog.SetBridgeEnabled(ctx, st, cluster.Fake{}, "default", "pve-node-01", "vmbr0", false); err != nil {
		t.Fatalf("SetBridgeEnabled off: %v", err)
	}

	bridges, err := catalog.AdminListBridges(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListBridges: %v", err)
	}

	for _, b := range bridges {
		if b.Name == "vmbr0" && b.Enabled {
			t.Error("vmbr0 should be disabled after toggle off")
		}
	}

	if err := catalog.SetBridgeEnabled(ctx, st, cluster.Fake{}, "default", "pve-node-01", "vmbr0", true); err != nil {
		t.Fatalf("SetBridgeEnabled on: %v", err)
	}

	bridges, err = catalog.AdminListBridges(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListBridges after on: %v", err)
	}

	for _, b := range bridges {
		if b.Name == "vmbr0" && !b.Enabled {
			t.Error("vmbr0 should be enabled after re-toggle on")
		}
	}
}

// TestSetISOEnabled_ToggleOffPersists — toggling an approved ISO off keeps the
// row (enabled=false), then re-enabling restores it.
//
//nolint:paralleltest // serial: shared fixture
func TestSetISOEnabled_ToggleOffPersists(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	if err := catalog.SetISOEnabled(ctx, st, cluster.Fake{}, "default", catalog.ISORef{Node: node01, Storage: storageLocal, File: debianGenericISO}, false); err != nil {
		t.Fatalf("SetISOEnabled off: %v", err)
	}

	isos, err := catalog.AdminListISOs(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListISOs: %v", err)
	}

	for _, i := range isos {
		if i.File == "debian-12-generic-amd64.iso" && i.Enabled {
			t.Error("debian-12 should be disabled after toggle off")
		}
	}

	if err := catalog.SetISOEnabled(ctx, st, cluster.Fake{}, "default", catalog.ISORef{Node: node01, Storage: storageLocal, File: debianGenericISO}, true); err != nil {
		t.Fatalf("SetISOEnabled on: %v", err)
	}

	isos, err = catalog.AdminListISOs(ctx, st, cluster.Fake{}, "default")
	if err != nil {
		t.Fatalf("AdminListISOs after on: %v", err)
	}

	for _, i := range isos {
		if i.File == "debian-12-generic-amd64.iso" && !i.Enabled {
			t.Error("debian-12 should be enabled after re-toggle on")
		}
	}
}

// pvmssDefaultColor is the indigo hex ensurePvmssTag seeds for the mandatory
// pvmss tag (FR-014). Centralized as a test const so repeated literals do not
// trip goconst.
const pvmssDefaultColor = "#4f46e5"

// TestEnsurePvmssTag_InsertsForNonDefaultCluster — the V9 migration seeds the
// mandatory pvmss tag only for the "default" cluster. ListTags lazily inserts
// it for any other cluster via ensurePvmssTag (FR-014), so the admin surface
// never lists a cluster without it. Idempotent on repeat calls.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestEnsurePvmssTag_InsertsForNonDefaultCluster(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	const otherCluster = "other-cluster"

	// First call: ensurePvmssTag inserts the missing pvmss tag.
	tags, err := catalog.ListTags(ctx, st, nil, otherCluster)
	if err != nil {
		t.Fatalf("ListTags first call: %v", err)
	}

	assertPvmssTagLazilySeeded(t, tags)

	// Second call: ensurePvmssTag's exists-early-return path — the tag is
	// already present, so no re-insert. The list must still contain exactly one
	// pvmss row.
	tags, err = catalog.ListTags(ctx, st, nil, otherCluster)
	if err != nil {
		t.Fatalf("ListTags second call: %v", err)
	}

	if count := countTagsByName(tags, catalog.ProtectedTagName); count != 1 {
		t.Errorf("expected exactly 1 pvmss tag after repeat ListTags, got %d", count)
	}
}

// assertPvmssTagLazilySeeded fails the test unless tags contains one pvmss row that
// is protected and carries the default indigo color — the shape ensurePvmssTag
// inserts for a cluster the V9 seed did not cover. Extracted from
// TestEnsurePvmssTag_InsertsForNonDefaultCluster to keep its cognitive
// complexity under the SonarQube threshold (go:S3776).
func assertPvmssTagLazilySeeded(t *testing.T, tags []catalog.TagWithCount) {
	t.Helper()

	for _, tag := range tags {
		if tag.Name != catalog.ProtectedTagName {
			continue
		}

		if !tag.Protected {
			t.Error("lazily-inserted pvmss tag should be protected")
		}

		if tag.Color != pvmssDefaultColor {
			t.Errorf("lazily-inserted pvmss color = %q, want %q", tag.Color, pvmssDefaultColor)
		}

		return
	}

	t.Fatal("ensurePvmssTag did not insert pvmss for non-default cluster")
}

// countTagsByName returns how many tags in the list carry the given name.
func countTagsByName(tags []catalog.TagWithCount, name string) int {
	count := 0

	for _, tag := range tags {
		if tag.Name == name {
			count++
		}
	}

	return count
}

// templateSetClient overrides the fake's template discovery with a fixed set,
// so tests can simulate drift (a template moved/resized after approval) and
// orphans (an approval whose template Proxmox no longer reports).
type templateSetClient struct {
	cluster.Fake
	templates []cluster.TemplateVM
}

func (c templateSetClient) ListTemplates(_ context.Context) ([]cluster.TemplateVM, error) {
	return c.templates, nil
}

// findApprovalTemplate returns the TemplateApproval with the given VMID.
func findApprovalTemplate(t *testing.T, templates []catalog.TemplateApproval, vmid int) catalog.TemplateApproval {
	t.Helper()

	for _, tmpl := range templates {
		if tmpl.VMID == vmid {
			return tmpl
		}
	}

	t.Fatalf("template %d not found in approval list", vmid)

	return catalog.TemplateApproval{}
}

// TestAdminListTemplates_DiscoveryWinsOnValues — a stored row's field values
// are a snapshot taken at approval time; when discovery reports different
// values, the list shows the discovered ones and the stored row is reconciled
// (write-back). The stored enabled flag stays authoritative.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminListTemplates_DiscoveryWinsOnValues(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	// Approval row with stale values (VMID 9100 is unused by the other
	// tests, so InsertTemplate creates it).
	stale := store.TemplateValues{
		Node: "old-node", Name: "old-name", CloudInitCapable: false,
		DiskStorage: "old-storage", DiskSizeGB: 4, DiskBus: "virtio",
	}
	if err := st.InsertTemplate(ctx, "default", 9100, stale, true); err != nil {
		t.Fatalf("InsertTemplate: %v", err)
	}

	// Discovery reports the template has since been renamed, migrated,
	// resized, and given a cloud-init drive.
	fresh := cluster.TemplateVM{
		VMID: 9100, Node: "pve-node-01", Name: "debian-13-cloud", CloudInitCapable: true,
		DiskStorage: storageLocalLVM, DiskSizeGB: 40, DiskBus: testProfileBus,
	}
	client := templateSetClient{Fake: cluster.Fake{}, templates: []cluster.TemplateVM{fresh}}

	approvals, err := catalog.AdminListTemplates(ctx, st, client, "default")
	if err != nil {
		t.Fatalf("AdminListTemplates: %v", err)
	}

	got := findApprovalTemplate(t, approvals, 9100)
	assertTemplateMatchesDiscovery(t, got, fresh)

	// The stored row was reconciled to the discovered values, enabled untouched.
	rows, err := st.CatalogTemplatesEnabled(ctx, "default")
	if err != nil {
		t.Fatalf("CatalogTemplatesEnabled: %v", err)
	}

	for _, row := range rows {
		if row.VMID != 9100 {
			continue
		}

		assertTemplateMatchesDiscovery(t, catalog.TemplateApproval{
			Node: row.Node, Name: row.Name, CloudInitCapable: row.CloudInitCapable,
			DiskStorage: row.DiskStorage, DiskSizeGB: row.DiskSizeGB, DiskBus: row.DiskBus,
		}, fresh)

		if !row.Enabled {
			t.Error("reconciliation must not touch enabled")
		}

		return
	}

	t.Fatal("stored row 9100 vanished")
}

// assertTemplateMatchesDiscovery checks that an approval carries exactly the
// discovered field values. Shared by the list and store-reconciliation checks
// of TestAdminListTemplates_DiscoveryWinsOnValues.
func assertTemplateMatchesDiscovery(t *testing.T, got catalog.TemplateApproval, want cluster.TemplateVM) {
	t.Helper()

	if got.Node != want.Node || got.Name != want.Name || !got.CloudInitCapable ||
		got.DiskStorage != want.DiskStorage || got.DiskSizeGB != want.DiskSizeGB || got.DiskBus != want.DiskBus {
		t.Errorf("approval = %+v, want discovered values %+v", got, want)
	}
}

// TestAdminListTemplates_SurfacesOrphanApprovals — a stored approval whose
// template Proxmox no longer reports must stay visible to the admin (with
// Missing=true), not silently disappear from the list.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminListTemplates_SurfacesOrphanApprovals(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	orphan := store.TemplateValues{
		Node: node02, Name: "deleted-template", CloudInitCapable: false,
		DiskStorage: storageLocal, DiskSizeGB: 2, DiskBus: testProfileBus,
	}
	if err := st.InsertTemplate(ctx, "default", 9999, orphan, true); err != nil {
		t.Fatalf("InsertTemplate: %v", err)
	}

	// Discovery only reports 9000 — 9999 is gone from Proxmox.
	client := templateSetClient{Fake: cluster.Fake{}, templates: []cluster.TemplateVM{
		{VMID: 9000, Node: node02, Name: "debian-12-cloud", CloudInitCapable: true, DiskStorage: storageLocalLVM, DiskSizeGB: 8, DiskBus: testProfileBus},
	}}

	approvals, err := catalog.AdminListTemplates(ctx, st, client, "default")
	if err != nil {
		t.Fatalf("AdminListTemplates: %v", err)
	}

	live := findApprovalTemplate(t, approvals, 9000)
	if live.Missing {
		t.Error("discovered template 9000 must not be flagged missing")
	}

	ghost := findApprovalTemplate(t, approvals, 9999)
	if !ghost.Missing {
		t.Error("orphan approval 9999 must be flagged missing")
	}

	if !ghost.Enabled {
		t.Error("orphan approval keeps its stored enabled state")
	}
}

// unreadableTemplateClient simulates a template whose disk config cannot be
// read (issue 03): TemplateByVMID returns a DiskUnreadable row.
type unreadableTemplateClient struct {
	cluster.Fake
	vmid int
}

func (c unreadableTemplateClient) TemplateByVMID(_ context.Context, vmid int) (cluster.TemplateVM, error) {
	if vmid == c.vmid {
		return cluster.TemplateVM{VMID: vmid, Node: node02, Name: "unreadable", DiskUnreadable: true}, nil
	}

	return cluster.TemplateVM{}, cluster.ErrNotFound
}

// TestSetTemplateEnabled_RejectsUnreadableOnApprove — approving a template
// whose disk could not be read would store empty disk_bus/disk_storage and
// break the post-clone resize (issue 03): refuse with ErrTemplateUnreadable
// and write no row.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestSetTemplateEnabled_RejectsUnreadableOnApprove(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	client := unreadableTemplateClient{Fake: cluster.Fake{}, vmid: 9100}

	err := catalog.SetTemplateEnabled(ctx, st, client, "default", catalog.TemplateRef{VMID: 9100}, true)
	if !errors.Is(err, catalog.ErrTemplateUnreadable) {
		t.Fatalf("err = %v, want ErrTemplateUnreadable", err)
	}

	rows, err := st.CatalogTemplatesEnabled(ctx, "default")
	if err != nil {
		t.Fatalf("CatalogTemplatesEnabled: %v", err)
	}

	for _, row := range rows {
		if row.VMID == 9100 {
			t.Error("no row must be written for an unreadable template approval")
		}
	}
}

// TestSetTemplateEnabled_AllowsDisablingUnreadable — an approved template
// whose disk later becomes unreadable must stay disable-able (users still see
// it; only the enable direction is blocked).
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestSetTemplateEnabled_AllowsDisablingUnreadable(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	values := store.TemplateValues{
		Node: node02, Name: "unreadable", CloudInitCapable: false,
		DiskStorage: storageLocal, DiskSizeGB: 2, DiskBus: testProfileBus,
	}
	if err := st.InsertTemplate(ctx, "default", 9100, values, true); err != nil {
		t.Fatalf("InsertTemplate: %v", err)
	}

	client := unreadableTemplateClient{Fake: cluster.Fake{}, vmid: 9100}

	if err := catalog.SetTemplateEnabled(ctx, st, client, "default", catalog.TemplateRef{VMID: 9100}, false); err != nil {
		t.Fatalf("SetTemplateEnabled(disable) = %v, want nil", err)
	}

	rows, err := st.CatalogTemplatesEnabled(ctx, "default")
	if err != nil {
		t.Fatalf("CatalogTemplatesEnabled: %v", err)
	}

	for _, row := range rows {
		if row.VMID == 9100 && row.Enabled {
			t.Error("disable must persist on an unreadable template")
		}
	}
}

// TestAdminListTemplates_UnreadableDoesNotClobberStoredRow — an unreadable
// discovery (issue 03) reports empty disk fields; reconciliation must not
// write them over the stored, approval-time values (T17's clone-time
// fallback relies on those fields being non-empty).
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminListTemplates_UnreadableDoesNotClobberStoredRow(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	values := store.TemplateValues{
		Node: node02, Name: "approved-template", CloudInitCapable: true,
		DiskStorage: storageLocalLVM, DiskSizeGB: 8, DiskBus: testProfileBus,
	}
	if err := st.InsertTemplate(ctx, "default", 9100, values, true); err != nil {
		t.Fatalf("InsertTemplate: %v", err)
	}

	// Discovery still reports the template but cannot read its config.
	client := templateSetClient{Fake: cluster.Fake{}, templates: []cluster.TemplateVM{
		{VMID: 9100, Node: node02, Name: "approved-template", DiskUnreadable: true},
	}}

	approvals, err := catalog.AdminListTemplates(ctx, st, client, "default")
	if err != nil {
		t.Fatalf("AdminListTemplates: %v", err)
	}

	got := findApprovalTemplate(t, approvals, 9100)
	if !got.DiskUnreadable {
		t.Error("approval should carry DiskUnreadable")
	}

	rows, err := st.CatalogTemplatesEnabled(ctx, "default")
	if err != nil {
		t.Fatalf("CatalogTemplatesEnabled: %v", err)
	}

	for _, row := range rows {
		if row.VMID != 9100 {
			continue
		}

		if row.DiskStorage != storageLocalLVM || row.DiskSizeGB != 8 || row.DiskBus != testProfileBus {
			t.Errorf("stored row = %+v, want the approval-time disk fields untouched", row)
		}

		return
	}

	t.Fatal("stored row 9100 vanished")
}
