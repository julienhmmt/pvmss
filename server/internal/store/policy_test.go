package store_test

import (
	"context"
	"database/sql"
	"errors"
	"pvmss/server/internal/store"
	"testing"
)

func samplePolicyRow(cluster string) store.PolicyRow {
	return store.PolicyRow{
		Cluster:         cluster,
		MaxSockets:      4,
		MaxCores:        8,
		MaxMemoryMB:     16384,
		MaxDiskPerVMGB:  100,
		MaxNetworkCards: 2,
		MaxSnapshots:    3,
		MaxVMPerUser:    10,
		AllowCustomYAML: true,
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestPolicyRow_FoundAndNotFound(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	t.Run("not found wraps sql.ErrNoRows", func(t *testing.T) {
		_, err := st.PolicyRow(ctx, "missing-cluster")
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("PolicyRow missing err = %v, want sql.ErrNoRows", err)
		}
	})

	t.Run("found returns row", func(t *testing.T) {
		row := samplePolicyRow(testStoreCluster)
		if err := st.UpsertPolicyRow(ctx, row); err != nil {
			t.Fatalf("UpsertPolicyRow: %v", err)
		}

		got, err := st.PolicyRow(ctx, testStoreCluster)
		if err != nil {
			t.Fatalf("PolicyRow: %v", err)
		}

		if got != row {
			t.Errorf("PolicyRow = %+v, want %+v", got, row)
		}
	})
}

//nolint:paralleltest // serial: shared database fixture
func TestUpsertPolicyRow_InsertThenUpdate(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	row := samplePolicyRow(testStoreCluster)
	if err := st.UpsertPolicyRow(ctx, row); err != nil {
		t.Fatalf("UpsertPolicyRow insert: %v", err)
	}

	got, err := st.PolicyRow(ctx, testStoreCluster)
	if err != nil {
		t.Fatalf("PolicyRow: %v", err)
	}

	if got.MaxVMPerUser != 10 {
		t.Errorf("MaxVMPerUser = %d, want 10", got.MaxVMPerUser)
	}

	row.MaxVMPerUser = 20

	row.AllowCustomYAML = false
	if err := st.UpsertPolicyRow(ctx, row); err != nil {
		t.Fatalf("UpsertPolicyRow update: %v", err)
	}

	got, err = st.PolicyRow(ctx, testStoreCluster)
	if err != nil {
		t.Fatalf("PolicyRow after update: %v", err)
	}

	if got.MaxVMPerUser != 20 {
		t.Errorf("MaxVMPerUser = %d, want 20", got.MaxVMPerUser)
	}

	if got.AllowCustomYAML {
		t.Errorf("AllowCustomYAML = true, want false")
	}
}

func sampleNodePolicyRow(cluster, node string) store.NodePolicyRow {
	return store.NodePolicyRow{
		Cluster:   cluster,
		Node:      node,
		MaxVMs:    50,
		MaxVCPUs:  200,
		MaxRAMGB:  512,
		MaxDiskGB: 4096,
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestNodePolicyRow_FoundAndNotFound(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	t.Run("not found", func(t *testing.T) {
		_, err := st.NodePolicyRow(ctx, testStoreCluster, "missing-node")
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("NodePolicyRow missing err = %v, want sql.ErrNoRows", err)
		}
	})

	t.Run("found returns row", func(t *testing.T) {
		row := sampleNodePolicyRow(testStoreCluster, "node-1")
		if err := st.UpsertNodePolicyRow(ctx, row); err != nil {
			t.Fatalf("UpsertNodePolicyRow: %v", err)
		}

		got, err := st.NodePolicyRow(ctx, testStoreCluster, "node-1")
		if err != nil {
			t.Fatalf("NodePolicyRow: %v", err)
		}

		if got != row {
			t.Errorf("NodePolicyRow = %+v, want %+v", got, row)
		}
	})
}

func upsertNodePolicyRows(ctx context.Context, t *testing.T, st *store.Store, nodes []string) {
	t.Helper()

	for _, n := range nodes {
		row := sampleNodePolicyRow(testStoreCluster, n)
		if err := st.UpsertNodePolicyRow(ctx, row); err != nil {
			t.Fatalf("UpsertNodePolicyRow %s: %v", n, err)
		}
	}
}

func assertNodePolicyOrder(t *testing.T, got []store.NodePolicyRow, want []string) {
	t.Helper()

	for i, w := range want {
		if got[i].Node != w {
			t.Errorf("row[%d].Node = %q, want %q", i, got[i].Node, w)
		}
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestNodePolicyRows(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	t.Run("multiple rows ordered by node", func(t *testing.T) {
		nodes := []string{catalogTestNodeZeta, catalogTestNodeAlpha, catalogTestNodeMid}
		upsertNodePolicyRows(ctx, t, st, nodes)

		got, err := st.NodePolicyRows(ctx, testStoreCluster)
		if err != nil {
			t.Fatalf("NodePolicyRows: %v", err)
		}

		if len(got) != len(nodes) {
			t.Fatalf("rows = %d, want %d", len(got), len(nodes))
		}

		want := []string{catalogTestNodeAlpha, catalogTestNodeMid, catalogTestNodeZeta}
		assertNodePolicyOrder(t, got, want)
	})

	t.Run("empty result", func(t *testing.T) {
		got, err := st.NodePolicyRows(ctx, "no-such-cluster")
		if err != nil {
			t.Fatalf("NodePolicyRows: %v", err)
		}

		if len(got) != 0 {
			t.Errorf("rows = %d, want 0", len(got))
		}
	})
}

//nolint:paralleltest // serial: shared database fixture
func TestUpsertNodePolicyRow_InsertThenUpdate(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	row := sampleNodePolicyRow(testStoreCluster, "node-up")
	if err := st.UpsertNodePolicyRow(ctx, row); err != nil {
		t.Fatalf("UpsertNodePolicyRow insert: %v", err)
	}

	got, err := st.NodePolicyRow(ctx, testStoreCluster, "node-up")
	if err != nil {
		t.Fatalf("NodePolicyRow: %v", err)
	}

	if got.MaxVMs != 50 {
		t.Errorf("MaxVMs = %d, want 50", got.MaxVMs)
	}

	row.MaxVMs = 100

	row.MaxDiskGB = 8192
	if err := st.UpsertNodePolicyRow(ctx, row); err != nil {
		t.Fatalf("UpsertNodePolicyRow update: %v", err)
	}

	got, err = st.NodePolicyRow(ctx, testStoreCluster, "node-up")
	if err != nil {
		t.Fatalf("NodePolicyRow after update: %v", err)
	}

	if got.MaxVMs != 100 {
		t.Errorf("MaxVMs = %d, want 100", got.MaxVMs)
	}

	if got.MaxDiskGB != 8192 {
		t.Errorf("MaxDiskGB = %d, want 8192", got.MaxDiskGB)
	}
}
