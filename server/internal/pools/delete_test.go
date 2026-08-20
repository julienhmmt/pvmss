//nolint:paralleltest,wsl_v5,goconst,sloglint // tests use shared fake fixtures and audit setup
package pools_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/pools"
	"pvmss/server/internal/store"
	"testing"
	"time"
)

func TestDelete_CascadeAuditTrailAndUserResult(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	client := cluster.Fake{}
	admin := auth.Identity{Username: "admin", IsAdmin: true}
	st := openAuditStore(t)
	if _, err := pools.CreateWithRecorder(context.Background(), admin, client, st, "default", "carol", "S0meLongPW!"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	projection, err := projectionFromFake(t, client)
	if err != nil {
		t.Fatalf("projection: %v", err)
	}
	worker := inventory.NewWorker(client, projection, time.Hour, testLogger(t))
	result, err := pools.Delete(context.Background(), pools.CascadeDeps{Actor: admin, Client: client, Projection: projection, ClusterName: "default", Writer: client, Audit: st, Refresher: worker, Managed: st}, "carol")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if result.Status != "deleted" || !result.UserDeleted {
		t.Fatalf("result = %+v, want deleted/userDeleted=true", result)
	}
	remaining, err := client.ListPools(context.Background())
	if err != nil {
		t.Fatalf("ListPools: %v", err)
	}
	for _, pool := range remaining {
		if pool.Name == "carol" {
			t.Fatal("deleted pool remains")
		}
	}
	managed, err := st.IsPoolManaged(context.Background(), "default", "carol")
	if err != nil {
		t.Fatalf("IsPoolManaged: %v", err)
	}
	if managed {
		t.Fatal("managed marker remains after delete")
	}
}

func TestDelete_CascadeAuditTrailUsesAdminActor(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	client := cluster.Fake{}
	projection, err := projectionFromFake(t, client)
	if err != nil {
		t.Fatalf("projection: %v", err)
	}
	auditStore := openAuditStore(t)
	if err := auditStore.RegisterManagedPool(context.Background(), "default", cluster.FakePoolBob); err != nil {
		t.Fatalf("register managed: %v", err)
	}
	worker := inventory.NewWorker(client, projection, time.Hour, testLogger(t))
	result, err := pools.Delete(context.Background(), pools.CascadeDeps{Actor: auth.Identity{Username: "admin", IsAdmin: true}, Client: client, Projection: projection, ClusterName: "default", Writer: client, Audit: auditStore, Refresher: worker, Managed: auditStore}, cluster.FakePoolBob)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if result.Status != "deleted" {
		t.Fatalf("result = %+v", result)
	}
	entries, err := auditStore.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}
	if len(entries) != 12 {
		t.Fatalf("audit entries = %d, want 12 (5 stops + 7 deletes)", len(entries))
	}
	var stops, deletes int
	for _, entry := range entries {
		if entry.Actor != "admin" {
			t.Errorf("audit actor = %q, want admin", entry.Actor)
		}
		switch entry.Action {
		case "stop":
			stops++
		case "delete":
			deletes++
		}
	}
	if stops != 5 || deletes != 7 {
		t.Fatalf("audit actions = stop:%d delete:%d, want stop:5 delete:7", stops, deletes)
	}
}

func TestDelete_UserDeletionFailureIsReported(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	client := cluster.Fake{}
	admin := auth.Identity{Username: "admin", IsAdmin: true}
	st := openAuditStore(t)
	if _, err := pools.CreateWithRecorder(context.Background(), admin, client, st, "default", "carol", "S0meLongPW!"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	cluster.SetFakeDeleteUserError(errors.New("user deletion failed"))

	projection, err := projectionFromFake(t, client)
	if err != nil {
		t.Fatalf("projection: %v", err)
	}
	result, err := pools.Delete(context.Background(), pools.CascadeDeps{Actor: admin, Client: client, Projection: projection, ClusterName: "default", Writer: client, Audit: st, Refresher: inventory.NewWorker(client, projection, time.Hour, testLogger(t)), Managed: st}, "carol")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if result.Status != "deleted" || result.UserDeleted {
		t.Fatalf("result = %+v, want deleted/userDeleted=false", result)
	}
}

func TestDelete_NonexistentPoolAndNonAdminAreRejected(t *testing.T) {
	cluster.ResetFake()
	client := cluster.Fake{}
	projection, err := projectionFromFake(t, client)
	if err != nil {
		t.Fatalf("projection: %v", err)
	}
	admin := auth.Identity{Username: "admin", IsAdmin: true}
	deps := func(t *testing.T) (pools.DeleteResult, error) {
		t.Helper()
		return pools.Delete(context.Background(), pools.CascadeDeps{Actor: admin, Client: client, Projection: projection, ClusterName: "default", Writer: client, Audit: openAuditStore(t), Refresher: inventory.NewWorker(client, projection, time.Hour, testLogger(t))}, "missing")
	}
	if _, err := deps(t); !errors.Is(err, pools.ErrNotFound) {
		t.Fatalf("missing error = %v, want ErrNotFound", err)
	}
	if _, err := pools.Delete(context.Background(), pools.CascadeDeps{Actor: auth.Identity{Username: "alice"}, Client: client, Projection: projection, ClusterName: "default", Writer: client, Audit: openAuditStore(t), Refresher: inventory.NewWorker(client, projection, time.Hour, testLogger(t))}, "missing"); !errors.Is(err, pools.ErrForbidden) {
		t.Fatalf("non-admin error = %v, want ErrForbidden", err)
	}
}

// TestDelete_UnmanagedProxmoxPoolIsRejected verifies that an existing Proxmox
// pool PVMSS did not provision cannot be cascade-deleted.
//
//nolint:paralleltest // serial: shared fake fixtures
func TestDelete_UnmanagedProxmoxPoolIsRejected(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	client := cluster.Fake{}
	projection, err := projectionFromFake(t, client)
	if err != nil {
		t.Fatalf("projection: %v", err)
	}
	st := openAuditStore(t)
	admin := auth.Identity{Username: "admin", IsAdmin: true}
	_, err = pools.Delete(context.Background(), pools.CascadeDeps{Actor: admin, Client: client, Projection: projection, ClusterName: "default", Writer: client, Audit: st, Refresher: inventory.NewWorker(client, projection, time.Hour, testLogger(t)), Managed: st}, cluster.FakePoolAlice)
	if !errors.Is(err, pools.ErrNotManaged) {
		t.Fatalf("error = %v, want ErrNotManaged", err)
	}
	remaining, err := client.ListPools(context.Background())
	if err != nil {
		t.Fatalf("ListPools: %v", err)
	}
	for _, pool := range remaining {
		if pool.Name == cluster.FakePoolAlice {
			return
		}
	}
	t.Fatalf("unmanaged pool was deleted: %+v", remaining)
}

// TestDelete_ManagedCascadeSucceedsAndUnregisters verifies a managed pool
// cascade completes and the managed marker is removed.
//
//nolint:paralleltest // serial: shared fake fixtures
func TestDelete_ManagedCascadeSucceedsAndUnregisters(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	client := cluster.Fake{}
	admin := auth.Identity{Username: "admin", IsAdmin: true}
	st := openAuditStore(t)
	if _, err := pools.CreateWithRecorder(context.Background(), admin, client, st, "default", "team-x", "S0meLongPW!"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	managed, err := st.IsPoolManaged(context.Background(), "default", "team-x")
	if err != nil {
		t.Fatalf("IsPoolManaged: %v", err)
	}
	if !managed {
		t.Fatal("pool not recorded as managed after create")
	}
	projection, err := projectionFromFake(t, client)
	if err != nil {
		t.Fatalf("projection: %v", err)
	}
	result, err := pools.Delete(context.Background(), pools.CascadeDeps{Actor: admin, Client: client, Projection: projection, ClusterName: "default", Writer: client, Audit: st, Refresher: inventory.NewWorker(client, projection, time.Hour, testLogger(t)), Managed: st}, "team-x")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if result.Status != "deleted" {
		t.Fatalf("result = %+v", result)
	}
	managed, err = st.IsPoolManaged(context.Background(), "default", "team-x")
	if err != nil {
		t.Fatalf("IsPoolManaged after delete: %v", err)
	}
	if managed {
		t.Fatal("managed marker remains after successful cascade")
	}
}

func testLogger(_ *testing.T) *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func projectionFromFake(t *testing.T, client cluster.Client) (*inventory.Projection, error) {
	t.Helper()
	snapshot, err := client.Snapshot(context.Background())
	if err != nil {
		return nil, err
	}
	index := inventory.BuildIndex(snapshot)
	return inventory.NewProjectionFromIndex(&index), nil
}

func openAuditStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "pools-audit.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}
