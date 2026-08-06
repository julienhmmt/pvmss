package proxmox

import (
	"context"
	"fmt"
	"net/url"
)

// VMSnapshot represents a Proxmox VM snapshot
type VMSnapshot struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Snaptime    int64  `json:"snaptime"`
	Vmstate     int    `json:"vmstate"`
	Parent      string `json:"parent,omitempty"`
	Snapstate   string `json:"snapstate,omitempty"`
}

// VMSnapshotConfig represents the configuration for creating a snapshot
type VMSnapshotConfig struct {
	Name        string `json:"snapname"`
	Description string `json:"description,omitempty"`
	Vmstate     bool   `json:"vmstate,omitempty"`
}

// CreateVMSnapshotResty creates a snapshot for a VM
func CreateVMSnapshotResty(ctx context.Context, client *RestyClient, node, vmid string, config VMSnapshotConfig) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%s/snapshot", node, vmid)

	// Convert config to url.Values for form-encoded request
	values := url.Values{}
	values.Set("snapname", config.Name)
	if config.Description != "" {
		values.Set("description", config.Description)
	}
	if config.Vmstate {
		values.Set("vmstate", "1")
	}

	var response interface{}
	err := client.Post(ctx, path, values, &response)

	if err != nil {
		return fmt.Errorf("failed to create VM snapshot: %w", err)
	}

	return nil
}

// GetVMSnapshotsResty retrieves all snapshots for a VM
func GetVMSnapshotsResty(ctx context.Context, client *RestyClient, node, vmid string) ([]VMSnapshot, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%s/snapshot", node, vmid)

	var response struct {
		Data []VMSnapshot `json:"data"`
	}

	err := client.Get(ctx, path, &response)

	if err != nil {
		return nil, fmt.Errorf("failed to get VM snapshots: %w", err)
	}

	return response.Data, nil
}

// DeleteVMSnapshotResty deletes a snapshot
func DeleteVMSnapshotResty(ctx context.Context, client *RestyClient, node, vmid, snapname string) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%s/snapshot/%s", node, vmid, snapname)

	var response interface{}
	err := client.Delete(ctx, path, &response)

	if err != nil {
		return fmt.Errorf("failed to delete VM snapshot: %w", err)
	}

	return nil
}

// RollbackVMSnapshotResty rolls back a VM to a specific snapshot
func RollbackVMSnapshotResty(ctx context.Context, client *RestyClient, node, vmid, snapname string) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%s/snapshot/%s/rollback", node, vmid, snapname)

	// Use PostEmpty for rollback (no parameters needed)
	var response interface{}
	err := client.PostEmpty(ctx, path, &response)

	if err != nil {
		return fmt.Errorf("failed to rollback VM snapshot: %w", err)
	}

	return nil
}

// IsValidSnapshotName validates snapshot name format (a-zA-Z0-9-_)
func IsValidSnapshotName(name string) bool {
	if len(name) == 0 || len(name) > 40 {
		return false
	}

	// Only allow alphanumeric characters, hyphens, and underscores
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return false
		}
	}

	return true
}
