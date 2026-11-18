package catalog_test

import (
	"context"
	"errors"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"testing"
)

const (
	sharedISOStorage = "ceph-data"
	sharedISOFile    = "debian-shared.iso"
)

// sharedISOClient is the fake cluster (ceph-data is shared) reporting one ISO
// on ceph-data from each node in nodes.
type sharedISOClient struct {
	cluster.Fake
	nodes []string
}

func (c sharedISOClient) ListISOs(_ context.Context) ([]cluster.ISOImage, error) {
	out := make([]cluster.ISOImage, 0, len(c.nodes))
	for _, n := range c.nodes {
		out = append(out, cluster.ISOImage{Storage: sharedISOStorage, Node: n, File: sharedISOFile, SizeBytes: 42})
	}

	return out, nil
}

func sharedISORows(t *testing.T, list []catalog.ISOApproval) []catalog.ISOApproval {
	t.Helper()

	var rows []catalog.ISOApproval

	for _, iso := range list {
		if iso.Storage == sharedISOStorage && iso.File == sharedISOFile {
			rows = append(rows, iso)
		}
	}

	return rows
}

func enabledISONodes(ctx context.Context, t *testing.T, st *store.Store) map[string]bool {
	t.Helper()

	rows, err := st.CatalogISOsEnabled(ctx, "default")
	if err != nil {
		t.Fatalf("CatalogISOsEnabled: %v", err)
	}

	out := map[string]bool{}

	for _, r := range rows {
		if r.Storage == sharedISOStorage && r.File == sharedISOFile {
			out[r.Node] = r.Enabled
		}
	}

	return out
}

func listShared(ctx context.Context, t *testing.T, st *store.Store, client cluster.Client) []catalog.ISOApproval {
	t.Helper()

	list, err := catalog.AdminListISOs(ctx, st, client, "default")
	if err != nil {
		t.Fatalf("AdminListISOs: %v", err)
	}

	return sharedISORows(t, list)
}

// TestAdminListISOs_SharedOnceAndFansOut: a shared-storage ISO seen by two
// nodes is one row, and one toggle approves it on both.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminListISOs_SharedOnceAndFansOut(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()
	client := sharedISOClient{nodes: []string{cluster.FakeNode01, cluster.FakeNode02}}

	rows := listShared(ctx, t, st, client)
	if len(rows) != 1 || !rows[0].Shared || rows[0].NodeCount != 2 || rows[0].Enabled {
		t.Fatalf("rows = %+v, want one disabled shared row on 2 nodes", rows)
	}

	ref := catalog.ISORef{Node: rows[0].Node, Storage: sharedISOStorage, File: sharedISOFile}
	if err := catalog.SetISOEnabled(ctx, st, client, "default", ref, true); err != nil {
		t.Fatalf("SetISOEnabled: %v", err)
	}

	got := enabledISONodes(ctx, t, st)
	if !got[cluster.FakeNode01] || !got[cluster.FakeNode02] {
		t.Fatalf("enabled rows = %v, want both nodes", got)
	}
}

// TestAdminListISOs_SharedExtendsToNewNode: an approved shared ISO reaches a
// node that appears later, without a new toggle.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminListISOs_SharedExtendsToNewNode(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	if err := st.SetISOEnabled(ctx, "default", cluster.FakeNode01, sharedISOStorage, sharedISOFile, true); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rows := listShared(ctx, t, st, sharedISOClient{nodes: []string{cluster.FakeNode01, cluster.FakeNode02}})
	if len(rows) != 1 || !rows[0].Enabled {
		t.Fatalf("rows = %+v, want one enabled shared row", rows)
	}

	if !enabledISONodes(ctx, t, st)[cluster.FakeNode02] {
		t.Fatal("approval should extend to the node that had no row")
	}
}

// TestSetISOEnabled_SharedRevokeSticks: disabling a shared ISO covers nodes
// absent from the snapshot, so a later list does not re-approve it.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestSetISOEnabled_SharedRevokeSticks(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	for _, n := range []string{cluster.FakeNode01, cluster.FakeNode02} {
		if err := st.SetISOEnabled(ctx, "default", n, sharedISOStorage, sharedISOFile, true); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// node-02 is gone from discovery when the admin revokes.
	one := sharedISOClient{nodes: []string{cluster.FakeNode01}}
	if err := catalog.SetISOEnabled(ctx, st, one, "default", catalog.ISORef{Storage: sharedISOStorage, File: sharedISOFile}, false); err != nil {
		t.Fatalf("SetISOEnabled disable: %v", err)
	}

	rows := listShared(ctx, t, st, sharedISOClient{nodes: []string{cluster.FakeNode01, cluster.FakeNode02}})
	if len(rows) != 1 || rows[0].Enabled {
		t.Fatalf("rows = %+v, want the shared ISO to stay disabled", rows)
	}
}

// TestAdminListISOs_SharedOrphanOnceAndDeleteAll: a disabled shared ISO gone
// from every node is one Missing row, and one delete removes every node row.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminListISOs_SharedOrphanOnceAndDeleteAll(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	for _, n := range []string{cluster.FakeNode01, cluster.FakeNode02} {
		if err := st.SetISOEnabled(ctx, "default", n, sharedISOStorage, sharedISOFile, false); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	gone := sharedISOClient{}

	rows := listShared(ctx, t, st, gone)
	if len(rows) != 1 || !rows[0].Missing || !rows[0].Shared {
		t.Fatalf("rows = %+v, want one missing shared row", rows)
	}

	ref := catalog.ISORef{Node: rows[0].Node, Storage: sharedISOStorage, File: sharedISOFile}
	if err := catalog.DeleteISO(ctx, st, gone, "default", ref); err != nil {
		t.Fatalf("DeleteISO: %v", err)
	}

	if got := enabledISONodes(ctx, t, st); len(got) != 0 {
		t.Fatalf("rows left = %v, want none", got)
	}

	if err := catalog.DeleteISO(ctx, st, gone, "default", ref); !errors.Is(err, catalog.ErrISONotFound) {
		t.Fatalf("second DeleteISO: got %v, want ErrISONotFound", err)
	}
}

// TestAdminListISOs_SharedEnabledOrphanAutoRemoved: an enabled shared ISO gone
// from every node is auto-removed on every node row.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminListISOs_SharedEnabledOrphanAutoRemoved(t *testing.T) {
	st := openAdminStore(t)
	ctx := context.Background()

	for _, n := range []string{cluster.FakeNode01, cluster.FakeNode02} {
		if err := st.SetISOEnabled(ctx, "default", n, sharedISOStorage, sharedISOFile, n == cluster.FakeNode01); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	if rows := listShared(ctx, t, st, sharedISOClient{}); len(rows) != 0 {
		t.Fatalf("rows = %+v, want none", rows)
	}

	if got := enabledISONodes(ctx, t, st); len(got) != 0 {
		t.Fatalf("rows left = %v, want none", got)
	}
}
