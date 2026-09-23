package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// BaselineTemplateID is the cloudinit_publications key of the standalone
// baseline document (image VMs created without a template).
const BaselineTemplateID = "__baseline__"

// NodePublication is the outcome of publishing one file on one node.
type NodePublication struct {
	Node  string `json:"node"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// CloudInitPublication is the latest published file of one template (or of
// the baseline) on one cluster, with the per-node outcome.
type CloudInitPublication struct {
	Cluster     string
	TemplateID  string
	Filename    string
	ContentHash string
	PublishedAt time.Time
	Nodes       []NodePublication
}

// PutCloudInitPublication upserts the latest publication of a template.
func (s *Store) PutCloudInitPublication(ctx context.Context, p CloudInitPublication) error {
	nodes := p.Nodes
	if nodes == nil {
		nodes = []NodePublication{}
	}

	raw, err := json.Marshal(nodes)
	if err != nil {
		return fmt.Errorf("encode publication nodes: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO cloudinit_publications (cluster, template_id, filename, content_hash, published_at, nodes_json)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(cluster, template_id) DO UPDATE SET
			filename = excluded.filename,
			content_hash = excluded.content_hash,
			published_at = excluded.published_at,
			nodes_json = excluded.nodes_json`,
		p.Cluster, p.TemplateID, p.Filename, p.ContentHash, p.PublishedAt.UTC().Format(time.RFC3339Nano), string(raw),
	)
	if err != nil {
		return fmt.Errorf("upsert cloud-init publication: %w", err)
	}

	return nil
}

// GetCloudInitPublication returns found=false when the template was never
// published on the cluster.
func (s *Store) GetCloudInitPublication(ctx context.Context, cluster, templateID string) (CloudInitPublication, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT cluster, template_id, filename, content_hash, published_at, nodes_json
		 FROM cloudinit_publications WHERE cluster = ? AND template_id = ?`, cluster, templateID)

	p, err := scanPublication(row)
	if errors.Is(err, sql.ErrNoRows) {
		return CloudInitPublication{}, false, nil
	}

	if err != nil {
		return CloudInitPublication{}, false, err
	}

	return p, true, nil
}

// ListCloudInitPublications returns every publication of the cluster keyed
// by template id.
func (s *Store) ListCloudInitPublications(ctx context.Context, cluster string) (map[string]CloudInitPublication, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT cluster, template_id, filename, content_hash, published_at, nodes_json
		 FROM cloudinit_publications WHERE cluster = ?`, cluster)
	if err != nil {
		return nil, fmt.Errorf("query cloud-init publications: %w", err)
	}
	defer rows.Close()

	out := make(map[string]CloudInitPublication)

	for rows.Next() {
		p, err := scanPublication(rows)
		if err != nil {
			return nil, err
		}

		out[p.TemplateID] = p
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cloud-init publications: %w", err)
	}

	return out, nil
}

// DeleteCloudInitPublication forgets a template's publication (the template
// was deleted). The file itself stays on the nodes: VMs may still use it.
func (s *Store) DeleteCloudInitPublication(ctx context.Context, cluster, templateID string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM cloudinit_publications WHERE cluster = ? AND template_id = ?`, cluster, templateID); err != nil {
		return fmt.Errorf("delete cloud-init publication: %w", err)
	}

	return nil
}

func scanPublication(scanner clusterScanner) (CloudInitPublication, error) {
	var (
		p     CloudInitPublication
		stamp string
		raw   string
	)

	if err := scanner.Scan(&p.Cluster, &p.TemplateID, &p.Filename, &p.ContentHash, &stamp, &raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return CloudInitPublication{}, err
		}

		return CloudInitPublication{}, fmt.Errorf("scan cloud-init publication: %w", err)
	}

	var err error

	p.PublishedAt, err = time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return CloudInitPublication{}, fmt.Errorf("parse publication timestamp: %w", err)
	}

	if err := json.Unmarshal([]byte(raw), &p.Nodes); err != nil {
		return CloudInitPublication{}, fmt.Errorf("decode publication nodes: %w", err)
	}

	return p, nil
}

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
