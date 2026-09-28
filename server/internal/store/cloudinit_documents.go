package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// BaselineTemplateID is the document key of the standalone baseline
// document (image VMs created without a template).
const BaselineTemplateID = "__baseline__"

// VMCloudInitDocument records which published (shared) cloud-init file a
// VM uses. The file is shared between VMs: deleting the VM deletes only this
// row, never the file.
type VMCloudInitDocument struct {
	Cluster    string
	VMID       int
	TemplateID string
	Filename   string
	UpdatedAt  time.Time
	UpdatedBy  string
}

// GetVMCloudInitDocument returns found=false when the VM has no published
// document recorded.
func (s *Store) GetVMCloudInitDocument(ctx context.Context, cluster string, vmid int) (VMCloudInitDocument, bool, error) {
	var (
		d     VMCloudInitDocument
		stamp string
	)

	err := s.db.QueryRowContext(ctx,
		`SELECT cluster, vmid, template_id, filename, updated_at, updated_by
		 FROM vm_cloudinit_documents WHERE cluster = ? AND vmid = ?`, cluster, vmid,
	).Scan(&d.Cluster, &d.VMID, &d.TemplateID, &d.Filename, &stamp, &d.UpdatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return VMCloudInitDocument{}, false, nil
	}

	if err != nil {
		return VMCloudInitDocument{}, false, fmt.Errorf("query vm cloud-init document: %w", err)
	}

	d.UpdatedAt, err = time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return VMCloudInitDocument{}, false, fmt.Errorf("parse vm cloud-init document timestamp: %w", err)
	}

	return d, true, nil
}

// PutVMCloudInitDocument upserts the document a VM uses.
func (s *Store) PutVMCloudInitDocument(ctx context.Context, cluster string, vmid int, templateID, filename, actor string) error {
	stamp := time.Now().UTC().Format(time.RFC3339Nano)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO vm_cloudinit_documents (cluster, vmid, template_id, filename, updated_at, updated_by)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(cluster, vmid) DO UPDATE SET
			template_id = excluded.template_id,
			filename = excluded.filename,
			updated_at = excluded.updated_at,
			updated_by = excluded.updated_by`,
		cluster, vmid, templateID, filename, stamp, actor,
	)
	if err != nil {
		return fmt.Errorf("upsert vm cloud-init document: %w", err)
	}

	return nil
}

// DeleteVMCloudInitDocument removes the VM's row. No row is not an error.
func (s *Store) DeleteVMCloudInitDocument(ctx context.Context, cluster string, vmid int) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM vm_cloudinit_documents WHERE cluster = ? AND vmid = ?`, cluster, vmid); err != nil {
		return fmt.Errorf("delete vm cloud-init document: %w", err)
	}

	return nil
}
