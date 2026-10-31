package store

import (
	"context"
	"fmt"
)

// DeleteVMCloudInitDocument removes a VM's cloud-init document row; a missing
// row is fine. Kept for the legacy cleanup path on VM deletion - any row a
// pre-removal deployment left behind must not outlive its VM.
func (s *Store) DeleteVMCloudInitDocument(ctx context.Context, cluster string, vmid int) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM vm_cloudinit_documents WHERE cluster = ? AND vmid = ?`, cluster, vmid); err != nil {
		return fmt.Errorf("delete vm cloud-init document: %w", err)
	}

	return nil
}
