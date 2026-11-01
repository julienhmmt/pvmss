package store_test

import (
	"context"
	"testing"
	"time"
)

// The cloud-init feature is removed but the table stays so the VM-delete path
// can warn about files left on the nodes: rows must be readable and deletable.
//
//nolint:paralleltest // serial: shared database fixture
func TestCloudInitSnippet_GetAndDelete(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if _, found, err := st.GetCloudInitSnippet(ctx, testStoreCluster, 101); err != nil || found {
		t.Fatalf("GetCloudInitSnippet on empty = (found=%v, err=%v), want (false, nil)", found, err)
	}

	updated := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO vm_cloudinit_snippets (cluster, vmid, content, storage, filename, updated_at, updated_by) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		testStoreCluster, 101, "#cloud-config", "local", "ci-101.yaml", updated.Format(time.RFC3339Nano), "admin"); err != nil {
		t.Fatalf("insert snippet row: %v", err)
	}

	snippet, found, err := st.GetCloudInitSnippet(ctx, testStoreCluster, 101)
	if err != nil || !found {
		t.Fatalf("GetCloudInitSnippet = (found=%v, err=%v), want found", found, err)
	}
	if snippet.Cluster != testStoreCluster || snippet.VMID != 101 || snippet.Content != "#cloud-config" ||
		snippet.Storage != "local" || snippet.Filename != "ci-101.yaml" || snippet.UpdatedBy != "admin" {
		t.Errorf("snippet = %+v, want the inserted row", snippet)
	}
	if !snippet.UpdatedAt.Equal(updated) {
		t.Errorf("UpdatedAt = %v, want %v", snippet.UpdatedAt, updated)
	}

	if err := st.DeleteCloudInitSnippet(ctx, testStoreCluster, 101); err != nil {
		t.Fatalf("DeleteCloudInitSnippet: %v", err)
	}
	if _, found, err := st.GetCloudInitSnippet(ctx, testStoreCluster, 101); err != nil || found {
		t.Fatalf("GetCloudInitSnippet after delete = (found=%v, err=%v), want (false, nil)", found, err)
	}

	if err := st.DeleteCloudInitSnippet(ctx, testStoreCluster, 101); err != nil {
		t.Fatalf("deleting an absent snippet must not error: %v", err)
	}
}
