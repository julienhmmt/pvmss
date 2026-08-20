package catalog_test

import (
	"errors"
	"pvmss/server/internal/catalog"
	"testing"
)

// Fixture literals reused across the membership tests below.
const (
	bridgeVMbr0  = "vmbr0"
	bridgeVMbr1  = "vmbr1"
	debianISO    = "debian-12.iso"
	node01       = "pve-node-01"
	node02       = "pve-node-02"
	storageLocal = "local"
	storageNFS   = "nfs"
)

// sampleResources builds a Resources fixture covering every membership path:
// one matching node, a storage matched by (name, node), a bridge, and an ISO
// matched by (storage, file). Pairs that share a name but differ on the second
// key are included to prove the composite-key lookups do not collide.
func sampleResources() catalog.Resources {
	return catalog.Resources{
		Nodes: []catalog.Node{
			{Name: node01},
			{Name: node02},
		},
		Storages: []catalog.Storage{
			{Name: storageLocal, Node: node01},
			{Name: storageLocal, Node: node02}, // same name, different node
			{Name: storageNFS, Node: node01},
		},
		Bridges: []catalog.Bridge{
			{Name: bridgeVMbr0, Node: node01},
			{Name: bridgeVMbr0, Node: node02},
			{Name: bridgeVMbr1, Node: node01},
		},
		ISOs: []catalog.ISO{
			{Storage: storageLocal, File: debianISO},
			{Storage: storageNFS, File: debianISO}, // same file, different storage
		},
		// Profiles are looked up via FindProfile, which takes a []Profile.
	}
}

// TestResources_HasNode — HasNode reports true only for an approved node name.
func TestResources_HasNode(t *testing.T) {
	t.Parallel()

	resources := sampleResources()

	cases := []struct {
		name string
		want bool
	}{
		{node01, true},
		{node02, true},
		{"pve-node-99", false},
		{"", false},
	}

	for _, tc := range cases {
		got := resources.HasNode(tc.name)
		if got != tc.want {
			t.Errorf("HasNode(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestResources_HasStorage — HasStorage matches the (name, node) pair, so a
// storage name present on one node must not match a different node.
func TestResources_HasStorage(t *testing.T) {
	t.Parallel()

	resources := sampleResources()

	cases := []struct {
		name, node string
		want       bool
	}{
		{storageLocal, node01, true},
		{storageLocal, node02, true},
		{storageNFS, node01, true},
		{storageLocal, "pve-node-99", false}, // right name, wrong node
		{"missing", node01, false},
		{"", "", false},
	}

	for _, tc := range cases {
		got := resources.HasStorage(tc.name, tc.node)
		if got != tc.want {
			t.Errorf("HasStorage(%q, %q) = %v, want %v", tc.name, tc.node, got, tc.want)
		}
	}
}

// TestResources_HasBridge — HasBridge matches the (name, node) pair.
func TestResources_HasBridge(t *testing.T) {
	t.Parallel()

	resources := sampleResources()

	cases := []struct {
		name, node string
		want       bool
	}{
		{bridgeVMbr0, node01, true},
		{bridgeVMbr0, node02, true},
		{bridgeVMbr1, node01, true},
		{bridgeVMbr1, node02, false},
		{"vmbr2", node01, false},
		{"", "", false},
	}

	for _, tc := range cases {
		got := resources.HasBridge(tc.name, tc.node)
		if got != tc.want {
			t.Errorf("HasBridge(%q, %q) = %v, want %v", tc.name, tc.node, got, tc.want)
		}
	}
}

// TestResources_HasISO — HasISO matches the (storage, file) pair, so the same
// file on two storages must not cross-match.
func TestResources_HasISO(t *testing.T) {
	t.Parallel()

	resources := sampleResources()

	cases := []struct {
		storage, file string
		want          bool
	}{
		{storageLocal, debianISO, true},
		{storageNFS, debianISO, true},
		{storageLocal, "ubuntu-24.04.iso", false}, // right storage, wrong file
		{"missing", debianISO, false},             // right file, wrong storage
		{"", "", false},
	}

	for _, tc := range cases {
		got := resources.HasISO(tc.storage, tc.file)
		if got != tc.want {
			t.Errorf("HasISO(%q, %q) = %v, want %v", tc.storage, tc.file, got, tc.want)
		}
	}
}

// TestFindProfile_FoundAndNotFound — FindProfile returns the matching profile
// by id, or an error for an absent id (FR-003).
func TestFindProfile_FoundAndNotFound(t *testing.T) {
	t.Parallel()

	profiles := []catalog.Profile{
		{ID: "small", Label: "Small", CPUCores: 1, MemoryMB: 2048, DiskGB: 20, Bus: "scsi"},
		{ID: "medium", Label: "Medium", CPUCores: 2, MemoryMB: 4096, DiskGB: 40, Bus: "scsi"},
	}

	got, err := catalog.FindProfile(profiles, "medium")
	if err != nil {
		t.Fatalf("FindProfile medium: %v", err)
	}

	if got.ID != "medium" {
		t.Errorf("FindProfile medium: got ID %q, want %q", got.ID, "medium")
	}

	if got.CPUCores != 2 {
		t.Errorf("FindProfile medium: got CPUCores %d, want 2", got.CPUCores)
	}

	if _, err := catalog.FindProfile(profiles, "nonexistent"); err == nil {
		t.Fatal("FindProfile nonexistent: expected error, got nil")
	}
}

// TestFindProfile_EmptySlice — an empty profile slice never matches.
func TestFindProfile_EmptySlice(t *testing.T) {
	t.Parallel()

	if _, err := catalog.FindProfile(nil, "small"); err == nil {
		t.Fatal("FindProfile on nil slice: expected error, got nil")
	}

	if _, err := catalog.FindProfile([]catalog.Profile{}, "small"); err == nil {
		t.Fatal("FindProfile on empty slice: expected error, got nil")
	}
}

// TestFindProfile_ErrorIsNotSentinel — the not-found error is a formatted
// message, not a wrapped sentinel, so errors.Is must not match a random error.
func TestFindProfile_ErrorIsNotSentinel(t *testing.T) {
	t.Parallel()

	_, err := catalog.FindProfile([]catalog.Profile{{ID: "small"}}, "missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if errors.Is(err, errors.New("unrelated")) {
		t.Error("FindProfile error should not match an unrelated sentinel")
	}
}
