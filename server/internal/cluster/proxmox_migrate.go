package cluster

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// MigrationPrecheck implements Migrator from GET .../migrate.
func (p Proxmox) MigrationPrecheck(ctx context.Context, node string, vmid int) (MigrationPrecheck, error) {
	raw, err := p.rest().do(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/qemu/%d/migrate", url.PathEscape(node), vmid), nil)
	if err != nil {
		return MigrationPrecheck{}, err
	}

	var data struct {
		AllowedNodes    []string `json:"allowed_nodes"`
		NotAllowedNodes map[string]struct {
			UnavailableStorages  []string `json:"unavailable_storages"`
			UnavailableResources []string `json:"unavailable-resources"`
		} `json:"not_allowed_nodes"`
		LocalDisks []struct {
			Volid string `json:"volid"`
		} `json:"local_disks"`
		LocalResources []string `json:"local_resources"`
	}
	if err := decodeData(raw, &data); err != nil {
		return MigrationPrecheck{}, fmt.Errorf("decode migration precheck: %w", err)
	}

	precheck := MigrationPrecheck{
		AllowedNodes:   data.AllowedNodes,
		NotAllowed:     make(map[string]string, len(data.NotAllowedNodes)),
		LocalResources: data.LocalResources,
	}

	for name, entry := range data.NotAllowedNodes {
		var reasons []string
		if len(entry.UnavailableStorages) > 0 {
			reasons = append(reasons, "unavailable storages: "+strings.Join(entry.UnavailableStorages, ", "))
		}

		if len(entry.UnavailableResources) > 0 {
			reasons = append(reasons, "unavailable resources: "+strings.Join(entry.UnavailableResources, ", "))
		}

		precheck.NotAllowed[name] = strings.Join(reasons, "; ")
	}

	for _, disk := range data.LocalDisks {
		precheck.LocalDisks = append(precheck.LocalDisks, disk.Volid)
	}

	slices.Sort(precheck.AllowedNodes)

	return precheck, nil
}

// Migrate implements Migrator, returning the dispatched task's UPID.
func (p Proxmox) Migrate(ctx context.Context, node string, vmid int, spec MigrateSpec) (string, error) {
	form := url.Values{"target": {spec.Target}}
	if spec.Online {
		form.Set("online", "1")
	}

	if spec.WithLocalDisks {
		form.Set("with-local-disks", "1")
	}

	return proxmoxSnapshotTask(ctx, p.rest(), http.MethodPost, fmt.Sprintf("/nodes/%s/qemu/%d/migrate", url.PathEscape(node), vmid), form)
}
