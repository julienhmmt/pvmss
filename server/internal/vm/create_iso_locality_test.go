package vm

import (
	"errors"
	"pvmss/server/internal/catalog"
	"testing"
)

// Fixture literals for the ISO locality tests (goconst: extracted as constants
// so repeated string literals do not trigger the linter).
const (
	isoNode01        = "pve-node-01"
	isoNode02        = "pve-node-02"
	isoNode03        = "pve-node-03"
	isoBridgeVMbr0   = "vmbr0"
	isoStorageLocal  = "local"
	isoStorageShared = "shared-nfs"
	isoStorageLVM    = "local-lvm"
	isoFileDebian    = "debian-12.iso"
	isoFileUbuntu    = "ubuntu-24.iso"
)

// isoLocalityResources builds a three-node catalog where debian-12 is on
// pve-node-02 only (node-local storage), ubuntu-24 is on shared storage across
// all three nodes, and each node has at least one storage and bridge so
// auto-selection can complete.
func isoLocalityResources() catalog.Resources {
	return catalog.Resources{
		Nodes: []catalog.Node{
			{Name: isoNode01},
			{Name: isoNode02},
			{Name: isoNode03},
		},
		Storages: []catalog.Storage{
			{Name: isoStorageLVM, Node: isoNode01},
			{Name: isoStorageLVM, Node: isoNode02},
			{Name: isoStorageLVM, Node: isoNode03},
		},
		Bridges: []catalog.Bridge{
			{Name: isoBridgeVMbr0, Node: isoNode01},
			{Name: isoBridgeVMbr0, Node: isoNode02},
			{Name: isoBridgeVMbr0, Node: isoNode03},
		},
		ISOs: []catalog.ISO{
			// Node-local: only on pve-node-02.
			{Storage: isoStorageLocal, Node: isoNode02, File: isoFileDebian},
			// Shared storage: one row per node.
			{Storage: isoStorageShared, Node: isoNode01, File: isoFileUbuntu},
			{Storage: isoStorageShared, Node: isoNode02, File: isoFileUbuntu},
			{Storage: isoStorageShared, Node: isoNode03, File: isoFileUbuntu},
		},
	}
}

// TestResolveResources_ISOLocalityAutoSelectsHoldingNode — when the request
// carries a node-local ISO and no explicit node, auto-selection must pick a
// node that holds the ISO, not Nodes[0] (US1).
func TestResolveResources_ISOLocalityAutoSelectsHoldingNode(t *testing.T) {
	t.Parallel()

	req := CreateRequest{
		ISO: &ISORequest{Storage: isoStorageLocal, File: isoFileDebian},
	}

	node, _, _, err := resolveResources(req, isoLocalityResources())
	if err != nil {
		t.Fatalf("resolveResources: %v", err)
	}

	if node != isoNode02 {
		t.Errorf("node = %q, want %q (the only node holding the ISO)", node, isoNode02)
	}
}

// TestValidateCatalog_ISOLocalityRejectsMismatchedNode — when the request
// specifies a node that does not hold the ISO, validateCatalog must reject
// with ErrNotApproved (US1).
func TestValidateCatalog_ISOLocalityRejectsMismatchedNode(t *testing.T) {
	t.Parallel()

	resources := isoLocalityResources()
	req := CreateRequest{
		ISO: &ISORequest{Storage: isoStorageLocal, File: isoFileDebian},
	}
	nics := []nicPlan{{bridge: isoBridgeVMbr0, model: "virtio"}}

	err := validateCatalog(req, resources, isoNode01, isoStorageLVM, nics)
	if !errors.Is(err, ErrNotApproved) {
		t.Fatalf("error = %v, want ErrNotApproved", err)
	}
}

// TestResolveResources_NoNodeHoldsISO — when the request carries an ISO that
// no approved node holds, resolveResources must return ErrNotApproved naming
// the ISO (US1, Q19).
func TestResolveResources_NoNodeHoldsISO(t *testing.T) {
	t.Parallel()

	resources := isoLocalityResources()
	req := CreateRequest{
		ISO: &ISORequest{Storage: isoStorageLocal, File: "missing.iso"},
	}

	_, _, _, err := resolveResources(req, resources)
	if !errors.Is(err, ErrNotApproved) {
		t.Fatalf("error = %v, want ErrNotApproved", err)
	}
}

// TestResolveResources_SharedStorageISOAllNodesCandidates — when the ISO is on
// shared storage (one row per node), every approved node is a candidate, so
// auto-selection falls back to Nodes[0] (US1, D1b).
func TestResolveResources_SharedStorageISOAllNodesCandidates(t *testing.T) {
	t.Parallel()

	req := CreateRequest{
		ISO: &ISORequest{Storage: isoStorageShared, File: isoFileUbuntu},
	}

	node, _, _, err := resolveResources(req, isoLocalityResources())
	if err != nil {
		t.Fatalf("resolveResources: %v", err)
	}

	if node != isoNode01 {
		t.Errorf("node = %q, want %q (Nodes[0] — all nodes hold the shared ISO)", node, isoNode01)
	}
}

// TestResolveResources_NoISOUnchanged — a request without an ISO must behave
// exactly as before: auto-select Nodes[0], no ISO locality filtering (US1
// regression guard).
func TestResolveResources_NoISOUnchanged(t *testing.T) {
	t.Parallel()

	node, _, _, err := resolveResources(CreateRequest{}, isoLocalityResources())
	if err != nil {
		t.Fatalf("resolveResources: %v", err)
	}

	if node != isoNode01 {
		t.Errorf("node = %q, want %q (Nodes[0], no ISO to filter on)", node, isoNode01)
	}
}
