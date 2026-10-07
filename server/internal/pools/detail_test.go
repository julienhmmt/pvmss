//nolint:paralleltest,wsl_v5 // tests use shared fake fixtures and table assertions
package pools_test

import (
	"context"
	"errors"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/pools"
	"pvmss/server/internal/store"
	"testing"
)

func TestDetail_ReturnsMembersAndIdentity(t *testing.T) {
	t.Cleanup(cluster.ResetFake)
	client := cluster.Fake{}
	snapshot, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	index := inventory.BuildIndex(snapshot)
	projection := inventory.NewProjectionFromIndex(&index)

	detail, err := pools.Detail(context.Background(), client, projection, nil, "", cluster.FakePoolAlice)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if detail.Name != cluster.FakePoolAlice {
		t.Fatalf("name = %q, want %q", detail.Name, cluster.FakePoolAlice)
	}
	if detail.Username != cluster.FakePoolAlice+"@pve" {
		t.Fatalf("username = %q, want %q@pve", detail.Username, cluster.FakePoolAlice)
	}
	if detail.Comment != "Alice's personal pool" {
		t.Fatalf("comment = %q", detail.Comment)
	}
	if len(detail.Members) != 7 {
		t.Fatalf("members = %d, want 7", len(detail.Members))
	}
	if detail.Managed || detail.CreatedAt != "" {
		t.Fatalf("unmanaged pool should carry no managed marker: %+v", detail)
	}
}

func TestDetail_ManagedPoolCarriesRegistration(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	client := cluster.Fake{}
	snapshot, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	index := inventory.BuildIndex(snapshot)
	projection := inventory.NewProjectionFromIndex(&index)

	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "pools-detail.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if err := st.RegisterManagedPool(context.Background(), "default", cluster.FakePoolBob); err != nil {
		t.Fatalf("RegisterManagedPool: %v", err)
	}

	detail, err := pools.Detail(context.Background(), client, projection, st, "default", cluster.FakePoolBob)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if !detail.Managed || detail.CreatedAt == "" {
		t.Fatalf("managed pool should carry marker and timestamp: %+v", detail)
	}
}

func TestDetail_NotFound(t *testing.T) {
	client := cluster.Fake{}
	projection := inventory.NewProjectionFromIndex(&inventory.Index{ByPool: map[string][]cluster.VM{}})

	_, err := pools.Detail(context.Background(), client, projection, nil, "", "no-such-pool")
	if !errors.Is(err, pools.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDetail_ProjectionNotReady(t *testing.T) {
	client := cluster.Fake{}
	projection := inventory.NewProjection()

	_, err := pools.Detail(context.Background(), client, projection, nil, "", cluster.FakePoolAlice)
	if !errors.Is(err, pools.ErrProjectionNotReady) {
		t.Fatalf("err = %v, want ErrProjectionNotReady", err)
	}
}
