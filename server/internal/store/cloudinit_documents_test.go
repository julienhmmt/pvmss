package store_test

import (
	"context"
	"testing"
	"time"
)

// vm_cloudinit_documents rows are per-VM runtime state: deleting the VM must
// not leave them behind.
//
//nolint:paralleltest // serial: shared database fixture
func TestDeleteVMCloudInitDocument(t *testing.T) {
	st := openClusterStore(t)
	ctx := context.Background()

	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO vm_cloudinit_documents (cluster, vmid, template_id, filename, updated_at, updated_by) VALUES (?, ?, ?, ?, ?, ?)`,
		testStoreCluster, 101, "debian-12", "ci-101.yaml", time.Now().UTC().Format(time.RFC3339Nano), "admin"); err != nil {
		t.Fatalf("insert document row: %v", err)
	}

	if err := st.DeleteVMCloudInitDocument(ctx, testStoreCluster, 101); err != nil {
		t.Fatalf("DeleteVMCloudInitDocument: %v", err)
	}

	var count int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM vm_cloudinit_documents WHERE cluster = ? AND vmid = ?`, testStoreCluster, 101).Scan(&count); err != nil {
		t.Fatalf("count documents: %v", err)
	}
	if count != 0 {
		t.Fatalf("documents left = %d, want 0", count)
	}

	if err := st.DeleteVMCloudInitDocument(ctx, testStoreCluster, 101); err != nil {
		t.Fatalf("deleting an absent document must not error: %v", err)
	}
}
