package cluster_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"testing"
)

// TestFake_PoolLifecycle exercises the pool management methods of the fake
// cluster client (EnsurePoolRole, CreatePool, SetPoolACL, ListPools, DeletePool)
// so every branch of the built-in substitute is covered (constitution XI: the
// fake must demonstrate every feature).
func TestFake_PoolLifecycle(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-pool-lifecycle")
	ctx := context.Background()

	if err := fake.EnsurePoolRole(ctx); err != nil {
		t.Fatalf("EnsurePoolRole: %v", err)
	}

	// Idempotent — second call is a no-op but must not error.
	if err := fake.EnsurePoolRole(ctx); err != nil {
		t.Fatalf("EnsurePoolRole (idempotent): %v", err)
	}

	if err := fake.CreatePool(ctx, "demopool", "demo pool"); err != nil {
		t.Fatalf("CreatePool: %v", err)
	}

	// Creating an existing pool is a no-op.
	if err := fake.CreatePool(ctx, "demopool", "demo pool"); err != nil {
		t.Fatalf("CreatePool (existing): %v", err)
	}

	if err := fake.SetPoolACL(ctx, "demopool@pve", "demopool", "PVMSSUser"); err != nil {
		t.Fatalf("SetPoolACL: %v", err)
	}

	pools, err := fake.ListPools(ctx)
	if err != nil {
		t.Fatalf("ListPools: %v", err)
	}

	if len(pools) == 0 {
		t.Error("expected at least one pool")
	}

	if err := fake.DeletePool(ctx, "demopool"); err != nil {
		t.Fatalf("DeletePool: %v", err)
	}

	// Deleting a missing pool returns ErrNotFound.
	if err := fake.DeletePool(ctx, "demopool"); !errors.Is(err, cluster.ErrNotFound) {
		t.Errorf("DeletePool (missing) = %v, want ErrNotFound", err)
	}
}

// TestFake_UserLifecycle exercises EnsurePoolUser, DeleteUser and the forced
// deletion-error branch of the fake cluster client.
func TestFake_UserLifecycle(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-user-lifecycle")
	ctx := context.Background()

	user, err := fake.EnsurePoolUser(ctx, "demopool", "secret")
	if err != nil {
		t.Fatalf("EnsurePoolUser: %v", err)
	}

	if user != "demopool@pve" {
		t.Errorf("EnsurePoolUser returned %q, want demopool@pve", user)
	}

	if err := fake.DeleteUser(ctx, user); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	// Force a deterministic deletion failure on the zero-value default fake.
	cluster.SetFakeDeleteUserError(context.Canceled)
	defer cluster.SetFakeDeleteUserError(nil)

	if err := (cluster.Fake{}).DeleteUser(ctx, user); err == nil {
		t.Error("DeleteUser with forced error: expected error, got nil")
	}
}

// TestFake_Authenticate exercises Authenticate and ChangePassword against the
// fake client's in-memory identity table — uncovered branches of the fake.
func TestFake_Authenticate(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-auth-ci")
	ctx := context.Background()

	user, err := fake.EnsurePoolUser(ctx, "authpool", "pvmss")
	if err != nil {
		t.Fatalf("EnsurePoolUser: %v", err)
	}

	if _, err := fake.Authenticate(ctx, user, "pvmss"); err != nil {
		t.Fatalf("Authenticate (valid): %v", err)
	}

	if _, err := fake.Authenticate(ctx, user, "wrong"); !errors.Is(err, cluster.ErrNotFound) {
		t.Errorf("Authenticate (bad password) = %v, want ErrNotFound", err)
	}

	if err := fake.ChangePassword(ctx, user, "pvmss", "newpass"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	if _, err := fake.Authenticate(ctx, user, "newpass"); err != nil {
		t.Errorf("Authenticate after ChangePassword: %v", err)
	}

	if err := fake.ChangePassword(ctx, user, "nope", "x"); !errors.Is(err, cluster.ErrNotFound) {
		t.Errorf("ChangePassword (bad old) = %v, want ErrNotFound", err)
	}
}

func TestFake_CloudInitIO(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-cloudinit-io")
	ctx := context.Background()

	snippet, err := fake.FindSnippetStorage(ctx, cluster.FakeNode01)
	if err != nil {
		t.Fatalf("FindSnippetStorage: %v", err)
	}

	if snippet == "" {
		t.Error("FindSnippetStorage returned empty storage")
	}

	if _, err := fake.FindSnippetStorage(ctx, "ghost-node"); !errors.Is(err, cluster.ErrNotFound) {
		t.Errorf("FindSnippetStorage (unknown node) = %v, want ErrNotFound", err)
	}

	cfg, err := fake.GetCloudInitConfig(ctx, cluster.FakeNode01, 100)
	if err != nil {
		t.Fatalf("GetCloudInitConfig: %v", err)
	}

	if cfg.IPMode != cluster.CloudInitIPModeDHCP {
		t.Errorf("GetCloudInitConfig IPMode = %q, want DHCP", cfg.IPMode)
	}

	if err := fake.SetCloudInitConfig(ctx, cluster.FakeNode01, 100, cluster.CloudInitConfig{IPMode: cluster.CloudInitIPModeStatic}); err != nil {
		t.Fatalf("SetCloudInitConfig: %v", err)
	}

	if err := fake.EnsureCloudInitDrive(ctx, cluster.FakeNode01, 100); err != nil {
		t.Fatalf("EnsureCloudInitDrive: %v", err)
	}

	if err := fake.PushCloudInitSnippet(ctx, cluster.FakeNode01, "local", "snippet.yaml", 100, "#cloud-config\n"); err != nil {
		t.Fatalf("PushCloudInitSnippet: %v", err)
	}

	ticket, err := fake.GetVNCTicket(ctx, cluster.FakeNode01, 100, "ticket")
	if err != nil {
		t.Fatalf("GetVNCTicket: %v", err)
	}

	if ticket.Ticket == "" {
		t.Error("GetVNCTicket returned empty ticket")
	}
}

func TestFake_HardwareNetwork(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-hw-net")
	ctx := context.Background()

	if err := fake.UpdateHardware(ctx, cluster.FakeNode01, 100, 1, 2, 2048, []string{"prod"}); err != nil {
		t.Fatalf("UpdateHardware: %v", err)
	}

	if err := fake.UpdateNetwork(ctx, cluster.FakeNode01, 100, []cluster.NetworkInterface{{Index: 0, Bridge: "vmbr0", Model: "virtio"}}); err != nil {
		t.Fatalf("UpdateNetwork: %v", err)
	}
}

// TestFake_VMStatus verifies the fake's VMStatusReader returns the live status
// from its in-memory state, including the injectable lock field.
func TestFake_VMStatus(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-vmstatus")
	ctx := context.Background()

	// VM 100 is running in the default fake dataset.
	live, err := fake.VMStatus(ctx, cluster.FakeNode01, 100)
	if err != nil {
		t.Fatalf("VMStatus: %v", err)
	}

	if live.Status != cluster.VMRunning {
		t.Errorf("status = %q, want %q", live.Status, cluster.VMRunning)
	}

	if live.Lock != "" {
		t.Errorf("lock = %q, want empty", live.Lock)
	}

	// Inject a lock and verify it is reported.
	fake.SetVMLock(100, "backup")

	live, err = fake.VMStatus(ctx, cluster.FakeNode01, 100)
	if err != nil {
		t.Fatalf("VMStatus after lock: %v", err)
	}

	if live.Lock != "backup" {
		t.Errorf("lock = %q, want %q", live.Lock, "backup")
	}

	// Clear the lock.
	fake.SetVMLock(100, "")

	live, _ = fake.VMStatus(ctx, cluster.FakeNode01, 100)
	if live.Lock != "" {
		t.Errorf("lock after clear = %q, want empty", live.Lock)
	}
}

// TestFake_VMStatus_NotFound verifies the fake returns ErrNotFound for an
// unknown VM.
func TestFake_VMStatus_NotFound(t *testing.T) {
	t.Parallel()

	fake := cluster.NewFake("test-vmstatus-notfound")

	_, err := fake.VMStatus(context.Background(), cluster.FakeNode01, 99999)
	if !errors.Is(err, cluster.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}
