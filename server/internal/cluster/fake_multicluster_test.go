//nolint:wsl_v5 // table-driven client contract keeps calls together
package cluster_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"testing"
)

//nolint:paralleltest // fake contract tests share package fixture state
func TestFake_OfflineDemoRejectsEveryClientMethod(t *testing.T) {
	fake := cluster.Fake{ClusterName: "offline-demo"}
	ctx := context.Background()
	tests := []struct {
		name string
		call func() error
	}{
		{name: "snapshot", call: func() error { _, err := fake.Snapshot(ctx); return err }},
		{name: "authenticate", call: func() error { _, err := fake.Authenticate(ctx, cluster.FakePoolAliceShort, "password"); return err }},
		{name: "change password", call: func() error { return fake.ChangePassword(ctx, cluster.FakePoolAliceShort, "old", "new") }},
		{name: "bridges", call: func() error { _, err := fake.ListBridges(ctx); return err }},
		{name: "isos", call: func() error { _, err := fake.ListISOs(ctx); return err }},
		{name: "pools", call: func() error { _, err := fake.ListPools(ctx); return err }},
		{name: "ensure role", call: func() error { return fake.EnsurePoolRole(ctx) }},
		{name: "ensure user", call: func() error { _, err := fake.EnsurePoolUser(ctx, "pool", "password"); return err }},
		{name: "create pool", call: func() error { return fake.CreatePool(ctx, "pool", "comment") }},
		{name: "set ACL", call: func() error { return fake.SetPoolACL(ctx, "user", "pool", "role") }},
		{name: "delete pool", call: func() error { return fake.DeletePool(ctx, "pool") }},
		{name: "delete user", call: func() error { return fake.DeleteUser(ctx, "user") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if err := test.call(); !errors.Is(err, cluster.ErrUnreachable) {
				t.Fatalf("error = %v, want ErrUnreachable", err)
			}
		})
	}
}

// firstStoppedVM returns the first stopped VM in vms, or vms[0] if none is
// stopped. The fake's Delete rejects a running VM with ErrVMRunning (mirroring
// real Proxmox); tests that exercise isolation, not the force-stop path, need
// a stopped target.
func firstStoppedVM(vms []cluster.VM) cluster.VM {
	for _, v := range vms {
		if v.Status == cluster.VMStopped {
			return v
		}
	}

	return vms[0]
}

// TestFake_InstancesDoNotShareMutations is the isolation guarantee NewFake
// exists for: a delete on one cluster must not shrink another cluster's
// snapshot, and a zero-value Fake{} must stay on the default dataset.
//
//nolint:paralleltest // constructs isolated NewFake instances; stays serial with other fake tests
func TestFake_InstancesDoNotShareMutations(t *testing.T) {
	defaultCluster := cluster.NewFake("default")
	secondary := cluster.NewFake("secondary")
	ctx := context.Background()

	beforeDefault, err := defaultCluster.Snapshot(ctx)
	if err != nil {
		t.Fatalf("default snapshot: %v", err)
	}
	beforeSecondary, err := secondary.Snapshot(ctx)
	if err != nil {
		t.Fatalf("secondary snapshot: %v", err)
	}
	if len(beforeDefault.VMs) == 0 || len(beforeSecondary.VMs) == 0 {
		t.Fatal("expected both clusters to have VMs")
	}

	target := firstStoppedVM(beforeDefault.VMs)
	if err := defaultCluster.Delete(ctx, target.Node, target.VMID); err != nil {
		t.Fatalf("delete on default: %v", err)
	}

	afterDefault, err := defaultCluster.Snapshot(ctx)
	if err != nil {
		t.Fatalf("default snapshot after delete: %v", err)
	}
	afterSecondary, err := secondary.Snapshot(ctx)
	if err != nil {
		t.Fatalf("secondary snapshot after delete: %v", err)
	}
	if len(afterDefault.VMs) != len(beforeDefault.VMs)-1 {
		t.Fatalf("default VM count = %d, want %d", len(afterDefault.VMs), len(beforeDefault.VMs)-1)
	}
	if len(afterSecondary.VMs) != len(beforeSecondary.VMs) {
		t.Fatalf("secondary VM count changed after default delete: %d -> %d", len(beforeSecondary.VMs), len(afterSecondary.VMs))
	}

	zero := cluster.Fake{}
	zeroSnap, err := zero.Snapshot(ctx)
	if err != nil {
		t.Fatalf("zero-value snapshot: %v", err)
	}
	if len(zeroSnap.VMs) != 25 {
		t.Fatalf("zero-value Fake still shares state: got %d VMs, want 25", len(zeroSnap.VMs))
	}
}
