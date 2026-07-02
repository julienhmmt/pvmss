package proxmox

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// VMCloneConfig holds the parameters for cloning a QEMU VM.
type VMCloneConfig struct {
	NewID       int    // target VMID (required)
	Name        string // hostname of the clone
	Target      string // destination node (empty = same node as source)
	Full        bool   // true = full copy, false = linked clone
	Storage     string // target storage for a full clone
	Pool        string // resource pool to add the clone to
	Description string // notes
}

// CloneVMResty clones a VM via POST /nodes/{node}/qemu/{vmid}/clone.
// Returns the Proxmox task UPID (clone is asynchronous; full clones are long-running).
func CloneVMResty(ctx context.Context, client *RestyClient, node, vmid string, cfg VMCloneConfig) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%s/clone", url.PathEscape(node), url.PathEscape(vmid))

	values := url.Values{}
	values.Set("newid", strconv.Itoa(cfg.NewID))
	if cfg.Name != "" {
		values.Set("name", cfg.Name)
	}
	if cfg.Target != "" {
		values.Set("target", cfg.Target)
	}
	if cfg.Full {
		values.Set("full", "1")
	}
	if cfg.Storage != "" {
		values.Set("storage", cfg.Storage)
	}
	if cfg.Pool != "" {
		values.Set("pool", cfg.Pool)
	}
	if cfg.Description != "" {
		values.Set("description", cfg.Description)
	}

	var resp Response[string] // data = UPID
	if err := client.Post(ctx, path, values, &resp); err != nil {
		return "", fmt.Errorf("failed to clone VM %s on node %s: %w", vmid, node, err)
	}
	InvalidateVMCache(node)
	return resp.Data, nil
}
