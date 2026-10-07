package store_test

import (
	"context"
	"database/sql"
	"errors"
	"pvmss/server/internal/store"
	"testing"
	"time"
)

//nolint:paralleltest // serial: shared database fixture
func TestRegisterManagedPool_InsertAndIdempotent(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.RegisterManagedPool(ctx, testStoreCluster, "pool-alpha"); err != nil {
		t.Fatalf("RegisterManagedPool: %v", err)
	}

	managed, err := st.IsPoolManaged(ctx, testStoreCluster, "pool-alpha")
	if err != nil {
		t.Fatalf("IsPoolManaged: %v", err)
	}

	if !managed {
		t.Fatalf("IsPoolManaged = false, want true")
	}

	var firstCreated string
	if err := st.DB().QueryRowContext(ctx, "SELECT created_at FROM managed_pools WHERE cluster = ? AND name = ?", testStoreCluster, "pool-alpha").Scan(&firstCreated); err != nil {
		t.Fatalf("read created_at: %v", err)
	}

	time.Sleep(1100 * time.Millisecond)

	if err := st.RegisterManagedPool(ctx, testStoreCluster, "pool-alpha"); err != nil {
		t.Fatalf("RegisterManagedPool idempotent: %v", err)
	}

	var secondCreated string
	if err := st.DB().QueryRowContext(ctx, "SELECT created_at FROM managed_pools WHERE cluster = ? AND name = ?", testStoreCluster, "pool-alpha").Scan(&secondCreated); err != nil {
		t.Fatalf("read created_at after re-register: %v", err)
	}

	if firstCreated == secondCreated {
		t.Errorf("created_at unchanged after re-register: %q", secondCreated)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestUnregisterManagedPool(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.RegisterManagedPool(ctx, testStoreCluster, "pool-beta"); err != nil {
		t.Fatalf("RegisterManagedPool: %v", err)
	}

	if err := st.UnregisterManagedPool(ctx, testStoreCluster, "pool-beta"); err != nil {
		t.Fatalf("UnregisterManagedPool: %v", err)
	}

	managed, err := st.IsPoolManaged(ctx, testStoreCluster, "pool-beta")
	if err != nil {
		t.Fatalf("IsPoolManaged: %v", err)
	}

	if managed {
		t.Fatalf("IsPoolManaged = true after unregister, want false")
	}

	if err := st.UnregisterManagedPool(ctx, testStoreCluster, "pool-beta"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("UnregisterManagedPool absent err = %v, want sql.ErrNoRows", err)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestIsPoolManaged(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if err := st.RegisterManagedPool(ctx, testStoreCluster, "pool-gamma"); err != nil {
		t.Fatalf("RegisterManagedPool: %v", err)
	}

	cases := []struct {
		name    string
		cluster string
		pool    string
		want    bool
	}{
		{name: "managed pool", cluster: testStoreCluster, pool: "pool-gamma", want: true},
		{name: "unmanaged pool", cluster: testStoreCluster, pool: "pool-missing", want: false},
		{name: "wrong cluster", cluster: "other", pool: "pool-gamma", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := st.IsPoolManaged(ctx, tc.cluster, tc.pool)
			if err != nil {
				t.Fatalf("IsPoolManaged: %v", err)
			}

			if got != tc.want {
				t.Errorf("IsPoolManaged = %v, want %v", got, tc.want)
			}
		})
	}
}

func registerPools(ctx context.Context, t *testing.T, st *store.Store, names []string) {
	t.Helper()

	for _, n := range names {
		if err := st.RegisterManagedPool(ctx, testStoreCluster, n); err != nil {
			t.Fatalf("RegisterManagedPool %s: %v", n, err)
		}
	}
}

func assertManagedPoolEntry(t *testing.T, i int, pool store.ManagedPool, wantName string) {
	t.Helper()

	if pool.Name != wantName {
		t.Errorf("pool[%d].Name = %q, want %q", i, pool.Name, wantName)
	}

	if pool.Cluster != testStoreCluster {
		t.Errorf("pool[%d].Cluster = %q, want %q", i, pool.Cluster, testStoreCluster)
	}

	if pool.CreatedAt == "" {
		t.Errorf("pool[%d].CreatedAt empty", i)
	}
}

func assertPoolNamesContain(t *testing.T, got map[string]struct{}, names []string) {
	t.Helper()

	for _, n := range names {
		if _, ok := got[n]; !ok {
			t.Errorf("missing name %q in map", n)
		}
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestManagedPools(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	t.Run("multiple pools ordered by name", func(t *testing.T) {
		names := []string{"pool-zeta", "pool-alpha", "pool-mid"}
		registerPools(ctx, t, st, names)

		got, err := st.ManagedPools(ctx, testStoreCluster)
		if err != nil {
			t.Fatalf("ManagedPools: %v", err)
		}

		if len(got) != len(names) {
			t.Fatalf("pools = %d, want %d", len(got), len(names))
		}

		want := []string{"pool-alpha", "pool-mid", "pool-zeta"}
		for i, w := range want {
			assertManagedPoolEntry(t, i, got[i], w)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		got, err := st.ManagedPools(ctx, "no-such-cluster")
		if err != nil {
			t.Fatalf("ManagedPools: %v", err)
		}

		if len(got) != 0 {
			t.Errorf("pools = %d, want 0", len(got))
		}
	})
}

//nolint:paralleltest // serial: shared database fixture
func TestManagedPoolNames(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	t.Run("multiple names", func(t *testing.T) {
		names := []string{"pool-one", "pool-two", "pool-three"}
		registerPools(ctx, t, st, names)

		got, err := st.ManagedPoolNames(ctx, testStoreCluster)
		if err != nil {
			t.Fatalf("ManagedPoolNames: %v", err)
		}

		if len(got) != len(names) {
			t.Fatalf("names = %d, want %d", len(got), len(names))
		}

		assertPoolNamesContain(t, got, names)
	})

	t.Run("empty result", func(t *testing.T) {
		got, err := st.ManagedPoolNames(ctx, "empty-cluster")
		if err != nil {
			t.Fatalf("ManagedPoolNames: %v", err)
		}

		if len(got) != 0 {
			t.Errorf("names = %d, want 0", len(got))
		}
	})
}
