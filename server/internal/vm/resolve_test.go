package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/vm"
	"testing"
	"time"
)

// buildResolveIndex builds an Index from the fake dataset, so Resolve tests
// run against the same 25-VM set the handlers and E2E suite use.
func buildResolveIndex(t *testing.T) *inventory.Index {
	t.Helper()

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := inventory.BuildIndex(snap)
	idx.RefreshedAt = time.Now()

	return &idx
}

// TestResolve is the literal table from data-model.md: the five combinations
// that prove Resolve is the single ownership gate (FR-001, FR-002, FR-004).
// This is the most important test in T05 — it is the structural proof S01 is
// closed (spec: "resolve_test.go + this phase's tests are the structural proof").
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestResolve(t *testing.T) {
	idx := buildResolveIndex(t)

	alice := auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}
	bob := auth.Identity{Username: "bob@pve", Pool: cluster.FakePoolBob}
	admin := auth.Identity{Username: testAdminUser, IsAdmin: true}

	cases := []struct {
		name     string
		actor    auth.Identity
		vmid     int
		wantErr  error
		wantNode string
		wantPool string
	}{
		{
			name:     "found + tagged + owner returns entity",
			actor:    alice,
			vmid:     100,
			wantErr:  nil,
			wantNode: cluster.FakeNode01,
			wantPool: cluster.FakePoolAlice,
		},
		{
			name:    "found + tagged + non-owner forbidden",
			actor:   bob,
			vmid:    100,
			wantErr: vm.ErrForbidden,
		},
		{
			name:    "found + untagged any caller not found (legacy-01, pool-carol, no pvmss tag)",
			actor:   alice,
			vmid:    109,
			wantErr: vm.ErrNotFound,
		},
		{
			name:    "not found",
			actor:   alice,
			vmid:    999,
			wantErr: vm.ErrNotFound,
		},
		{
			name:     "admin sees any tagged VM regardless of pool",
			actor:    admin,
			vmid:     103, // pool-bob, tagged pvmss
			wantErr:  nil,
			wantNode: cluster.FakeNode01,
			wantPool: cluster.FakePoolBob,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			entity, err := vm.Resolve(idx, c.actor, testClusterName, c.vmid)
			assertResolveResult(t, c.vmid, c.wantErr, c.wantNode, c.wantPool, entity, err)
		})
	}
}

// assertResolveResult checks the Resolve error and the returned Entity's
// identity fields (vmid, node, pool, cluster) against the expected values.
// Extracted from TestResolve to keep its Cognitive Complexity under the
// SonarQube go:S3776 threshold.
func assertResolveResult(
	t *testing.T,
	vmid int,
	wantErr error,
	wantNode, wantPool string,
	entity vm.Entity,
	err error,
) {
	t.Helper()

	if wantErr != nil {
		if !errors.Is(err, wantErr) {
			t.Fatalf("err = %v, want %v", err, wantErr)
		}

		return
	}

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}

	if entity.VMID != vmid {
		t.Errorf("vmid = %d, want %d", entity.VMID, vmid)
	}

	if entity.Node != wantNode {
		t.Errorf("node = %q, want %q (FR-003: server-resolved, never client-supplied)", entity.Node, wantNode)
	}

	if entity.Pool != wantPool {
		t.Errorf("pool = %q, want %q", entity.Pool, wantPool)
	}

	if entity.Cluster != testClusterName {
		t.Errorf("cluster = %q, want default", entity.Cluster)
	}
}

// TestResolve_AdminStillRequiresPvmssTag — an admin bypasses the pool check
// but never the tag check (FR-004): an untagged VM is 404 for everyone,
// including an admin.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestResolve_AdminStillRequiresPvmssTag(t *testing.T) {
	idx := buildResolveIndex(t)
	admin := auth.Identity{Username: testAdminUser, IsAdmin: true}

	_, err := vm.Resolve(idx, admin, testClusterName, 109) // legacy-01, untagged
	if !errors.Is(err, vm.ErrNotFound) {
		t.Fatalf("admin on untagged VM: err = %v, want ErrNotFound", err)
	}
}

// TestResolve_NodeAlwaysFromIndex — the returned node is exactly what the
// Index recorded, never re-derived from request input (FR-003, S01 root cause).
// This is the one-line fix at the center of S01: there is no node parameter
// to forge because Resolve does not accept one.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestResolve_NodeAlwaysFromIndex(t *testing.T) {
	idx := buildResolveIndex(t)
	alice := auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}

	entity, err := vm.Resolve(idx, alice, testClusterName, 102)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	indexed := idx.ByVMID[102]
	if entity.Node != indexed.Node {
		t.Fatalf("node = %q, want %q (the Index value, nothing else)", entity.Node, indexed.Node)
	}
}

// TestResolve_EntityCarriesDetailFields — the Entity returned to a detail
// request carries the metrics the V15 stat cards need (CPU, RAM, disk, uptime).
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestResolve_EntityCarriesDetailFields(t *testing.T) {
	idx := buildResolveIndex(t)
	alice := auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}

	entity, err := vm.Resolve(idx, alice, testClusterName, 100) // web-01, running
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if entity.CPUCores != 2 {
		t.Errorf("cpuCores = %d, want 2", entity.CPUCores)
	}

	if entity.MemoryTotal != 4294967296 {
		t.Errorf("memoryTotal = %d, want 4294967296", entity.MemoryTotal)
	}

	if entity.DiskTotal != 34359738368 {
		t.Errorf("diskTotal = %d, want 34359738368", entity.DiskTotal)
	}

	if entity.Uptime <= 0 {
		t.Errorf("uptime = %v, want > 0 for a running VM", entity.Uptime)
	}

	if entity.Name != "web-01" {
		t.Errorf("name = %q, want web-01", entity.Name)
	}

	if entity.Status != cluster.VMRunning {
		t.Errorf("status = %q, want running", entity.Status)
	}
}

// TestResolve_StoppedVmHasZeroUptime — uptime is absent (zero) when the VM is
// not running (contracts: uptimeSeconds absent when not running).
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestResolve_StoppedVmHasZeroUptime(t *testing.T) {
	idx := buildResolveIndex(t)
	alice := auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}

	entity, err := vm.Resolve(idx, alice, testClusterName, 101) // web-02, stopped
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if entity.Uptime != 0 {
		t.Errorf("uptime = %v, want 0 for a stopped VM", entity.Uptime)
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestResolve_EntityCarriesHardwareFields(t *testing.T) {
	idx := buildResolveIndex(t)
	alice := auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}

	entity, err := vm.Resolve(idx, alice, testClusterName, 101)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if entity.Sockets != 1 || entity.Cores != 2 {
		t.Errorf("hardware = %d sockets x %d cores, want 1 x 2", entity.Sockets, entity.Cores)
	}

	if len(entity.Disks) != 2 || entity.Disks[0].Key != "scsi0" {
		t.Errorf("disks = %+v, want scsi0 and scsi1", entity.Disks)
	}

	if !entity.Disks[0].IsBoot || entity.Disks[1].IsBoot {
		t.Errorf("boot flags = %+v, want only scsi0 boot", entity.Disks)
	}

	if entity.CDROM.State != cluster.CDROMMounted || entity.CDROM.ISOVolID == "" {
		t.Errorf("cdrom = %+v, want mounted media", entity.CDROM)
	}

	if len(entity.NetworkInterfaces) != 1 || entity.NetworkInterfaces[0].Bridge != testBridgeVMbr0 {
		t.Errorf("network interfaces = %+v, want vmbr0", entity.NetworkInterfaces)
	}
}
