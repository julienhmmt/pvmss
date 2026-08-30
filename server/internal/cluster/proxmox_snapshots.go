package cluster

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// ListSnapshots implements SnapshotReader. Proxmox always includes a
// pseudo-entry named "current" representing the live state, not a real
// snapshot — filtered out here so callers never see it.
func (p Proxmox) ListSnapshots(ctx context.Context, node string, vmid int) ([]VMSnapshot, error) {
	raw, err := p.rest().do(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/qemu/%d/snapshot", url.PathEscape(node), vmid), nil)
	if err != nil {
		return nil, err
	}

	var rows []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		SnapTime    int64  `json:"snaptime"`
		VMState     int    `json:"vmstate"`
	}
	if err := decodeData(raw, &rows); err != nil {
		return nil, fmt.Errorf("decode snapshots: %w", err)
	}

	snapshots := make([]VMSnapshot, 0, len(rows))

	for _, row := range rows {
		if row.Name == "current" {
			continue
		}

		snapshots = append(snapshots, VMSnapshot{
			Name:        row.Name,
			Description: row.Description,
			CreatedAt:   time.Unix(row.SnapTime, 0).UTC(),
			VMState:     row.VMState == 1,
		})
	}

	return snapshots, nil
}

// CreateSnapshot implements SnapshotWriter, returning the dispatched task's UPID.
func (p Proxmox) CreateSnapshot(ctx context.Context, node string, vmid int, name, description string, vmstate bool) (string, error) {
	form := url.Values{"snapname": {name}}
	if description != "" {
		form.Set("description", description)
	}

	if vmstate {
		form.Set("vmstate", "1")
	}

	return proxmoxSnapshotTask(ctx, p.rest(), http.MethodPost, fmt.Sprintf("/nodes/%s/qemu/%d/snapshot", url.PathEscape(node), vmid), form)
}

// RollbackSnapshot implements SnapshotWriter, returning the dispatched task's UPID.
func (p Proxmox) RollbackSnapshot(ctx context.Context, node string, vmid int, name string) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot/%s/rollback", url.PathEscape(node), vmid, url.PathEscape(name))
	return proxmoxSnapshotTask(ctx, p.rest(), http.MethodPost, path, nil)
}

// DeleteSnapshot implements SnapshotWriter, returning the dispatched task's UPID.
// force=1 tells Proxmox to drop the snapshot's config entry even when the
// underlying storage delete fails. Without it, an NFS/qcow2 ESTALE leaves the
// VM at lock=snapshot-delete, after which Proxmox refuses every subsequent
// operation on it — including the delete that would clear it. The volume can
// remain orphaned on storage; a blocked VM is worse (pegaprox incident #422).
func (p Proxmox) DeleteSnapshot(ctx context.Context, node string, vmid int, name string) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot/%s", url.PathEscape(node), vmid, url.PathEscape(name))
	return proxmoxSnapshotTask(ctx, p.rest(), http.MethodDelete, path, url.Values{"force": {"1"}})
}

func proxmoxSnapshotTask(ctx context.Context, rest proxmoxRESTClient, method, path string, form url.Values) (string, error) {
	raw, err := rest.do(ctx, method, path, form)
	if err != nil {
		return "", err
	}

	var upid string
	if err := decodeData(raw, &upid); err != nil {
		return "", fmt.Errorf("decode snapshot task: %w", err)
	}

	return upid, nil
}

// SnapshotConfig implements SnapshotConfigReader. Proxmox stores the VM
// config as it was at snapshot time under /snapshot/{name}/config; the
// pseudo-entry "current" maps to /config?current=1 (the live config).
func (p Proxmox) SnapshotConfig(ctx context.Context, node string, vmid int, name string) (map[string]string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/snapshot/%s/config", url.PathEscape(node), vmid, url.PathEscape(name))
	if name == "current" {
		path = fmt.Sprintf("/nodes/%s/qemu/%d/config?current=1", url.PathEscape(node), vmid)
	}

	raw, err := p.rest().do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var cfg proxmoxVMConfig
	if err := decodeData(raw, &cfg); err != nil {
		return nil, fmt.Errorf("decode snapshot config: %w", err)
	}

	flattened := make(map[string]string, len(cfg))
	for key := range cfg {
		flattened[key] = cfg.str(key)
	}

	return flattened, nil
}
