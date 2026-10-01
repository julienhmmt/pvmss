package cluster

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
)

// Proxmox is the real cluster implementation, talking to the Proxmox VE REST
// API (https://pve.proxmox.com/pve-docs/api-viewer/). BaseURL, APITokenName
// and APITokenValue are configured per cluster (server/internal/store's
// ClusterRow, wired in registry.go) or from PROXMOX_URL/PROXMOX_API_TOKEN_NAME/
// PROXMOX_API_TOKEN_VALUE in single-cluster setups.
//
// Every write here uses the service account's API token - it never needs the
// CSRF prevention token tickets require, which is exactly why Proxmox
// recommends tokens for service accounts (proxmox-permissions.md). The two
// exceptions are Authenticate and ChangePassword: both must act with the
// specific end user's own privileges, so they mint a short-lived ticket for
// that user internally (see proxmoxTicketAuth) instead of using the token.
type Proxmox struct {
	BaseURL               string
	APITokenName          string
	APITokenValue         string
	TLSInsecureSkipVerify bool
	// SnippetStorage is the Proxmox storage whose snippets/ content holds
	// the admin cloud-init documents, written by hand on each node (see
	// docs/cloud-init.md). Empty means the feature is off.
	SnippetStorage string
	// httpClient is the cached *http.Client reused across every REST call so
	// the underlying Transport's keep-alive connection pool is shared. Set at construction in
	// registry.go; rest() lazily initializes it
	// when nil so a zero-value Proxmox (tests) never panics.
	httpClient *http.Client
	// log and name feed the REST client's per-call Debug line; set by the
	// registry factory, zero in tests.
	log  *slog.Logger
	name string
}

// LogValue keeps the API token out of logs when a Proxmox value is logged.
func (Proxmox) LogValue() slog.Value { return slog.StringValue("[redacted]") }

// proxmoxResourceRow is one row of /cluster/resources?type=... - Proxmox's
// single call for nodes, VMs, and storages together, matching what Snapshot
// promises ("one call returns everything").
type proxmoxResourceRow struct {
	Type       string  `json:"type"` // "node", "qemu", "storage", ...
	Node       string  `json:"node"`
	Status     string  `json:"status"`
	VMID       int     `json:"vmid"`
	Name       string  `json:"name"`
	Pool       string  `json:"pool"`
	Tags       string  `json:"tags"`
	MaxCPU     float64 `json:"maxcpu"`
	CPU        float64 `json:"cpu"`
	MaxMem     int64   `json:"maxmem"`
	Mem        int64   `json:"mem"`
	MaxDisk    int64   `json:"maxdisk"`
	Disk       int64   `json:"disk"`
	Storage    string  `json:"storage"`
	PluginType string  `json:"plugintype"`
	Content    string  `json:"content"`
	Template   int     `json:"template"` // 1 when the qemu VM is a template
}

// StorageSnapshotCapability reports whether a (storage plugin, disk format)
// pair supports snapshots and RAM-state snapshots. The plugin
// alone decides for block-backed storages (zfspool, lvmthin, rbd, btrfs);
// file-backed storages (dir, nfs, cifs, cephfs) need qcow2 disks. Plain lvm
// (non-thin), iscsi and raw-on-file cannot snapshot at all.
//
// ponytail: the file-backed rows mirror PVE's documented per-plugin snapshot
// support but were not validated line-by-line against the PVE sources -
// Flags exactly this; revisit if a real cluster surprises us.
func StorageSnapshotCapability(pluginType, format string) (canSnapshot, canVMState bool) {
	switch pluginType {
	case "zfspool", "lvmthin", "rbd", "btrfs":
		return true, true
	case "dir", "nfs", "cifs", "cephfs":
		return format == "qcow2", format == "qcow2"
	default:
		return false, false
	}
}

// pluginSupportsVMState is the plugin-level view behind Storage.SupportsVMState
// (a storage can hold RAM state when its plugin snapshots natively). The
// per-disk decision also depends on the disk format - see
// StorageSnapshotCapability.
func pluginSupportsVMState(pluginType string) bool {
	switch pluginType {
	case "zfspool", "lvmthin", "rbd", "btrfs":
		return true
	default:
		return false
	}
}

// proxmoxClusterResourcesPath is the /cluster/resources endpoint, used by
// Snapshot, ListStorages and ListTemplates.
const proxmoxClusterResourcesPath = "/cluster/resources"

// proxmoxResourceTypeParam is the "type" query parameter that filters
//
//	/cluster/resources results ("vm", "storage", ...).
const proxmoxResourceTypeParam = "type"

// proxmoxStorageType is the /cluster/resources type of a storage row.
const proxmoxStorageType = "storage"

// Snapshot implements Client: one /cluster/resources call for the node,
// VM, and storage summary, then one /qemu/{vmid}/config (plus, for running
// VMs, one /status/current) call per VM to hydrate what the summary omits
// (see hydrateVM).
func (p Proxmox) Snapshot(ctx context.Context) (Snapshot, error) {
	rest := p.rest()

	raw, err := rest.do(ctx, http.MethodGet, proxmoxClusterResourcesPath, nil)
	if err != nil {
		return Snapshot{}, err
	}

	var rows []proxmoxResourceRow
	if err := decodeData(raw, &rows); err != nil {
		return Snapshot{}, fmt.Errorf("decode cluster resources: %w", err)
	}

	version, err := proxmoxVersion(ctx, rest)
	if err != nil {
		return Snapshot{}, err
	}

	snap := Snapshot{ProxmoxVersion: version}

	for _, row := range rows {
		switch row.Type {
		case "node":
			snap.Nodes = append(snap.Nodes, proxmoxNodeFromRow(row))
		case "qemu":
			snap.VMs = append(snap.VMs, proxmoxVMFromRow(row))
		case proxmoxStorageType:
			snap.Storages = append(snap.Storages, proxmoxStorageFromRow(row))
		}
	}

	for i := range snap.VMs {
		if err := hydrateVM(ctx, rest, &snap.VMs[i]); err != nil {
			return Snapshot{}, fmt.Errorf("hydrate vm %d: %w", snap.VMs[i].VMID, err)
		}
	}

	return snap, nil
}

// DisplayName implements Client. It calls /cluster/status and returns the name
// of the entry whose type is "cluster"; for a standalone node (no cluster
// configured) it falls back to the first node's hostname.
func (p Proxmox) DisplayName(ctx context.Context) (string, error) {
	rest := p.rest()

	raw, err := rest.do(ctx, http.MethodGet, "/cluster/status", nil)
	if err != nil {
		return "", err
	}

	var rows []proxmoxClusterStatusRow
	if err := decodeData(raw, &rows); err != nil {
		return "", fmt.Errorf("decode cluster status: %w", err)
	}

	for _, row := range rows {
		if row.Type == "cluster" {
			return row.Name, nil
		}
	}

	for _, row := range rows {
		if row.Type == "node" && row.Name != "" {
			return row.Name, nil
		}
	}

	return "", nil
}

// proxmoxClusterStatusRow is one row of /cluster/status.
type proxmoxClusterStatusRow struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	IP     string `json:"ip"`
	Online int    `json:"online"`
}

func proxmoxNodeFromRow(row proxmoxResourceRow) Node {
	status := NodeUnknown

	switch row.Status {
	case "online":
		status = NodeOnline
	case "offline":
		status = NodeOffline
	}

	return Node{
		Name:         row.Node,
		Status:       status,
		CPUCores:     int(row.MaxCPU),
		CPUUsage:     row.CPU,
		MemoryTotal:  row.MaxMem,
		MemoryUsed:   row.Mem,
		StorageTotal: row.MaxDisk,
		StorageUsed:  row.Disk,
	}
}

func proxmoxVMFromRow(row proxmoxResourceRow) VM {
	status := VMStopped

	switch row.Status {
	case string(VMRunning):
		status = VMRunning
	case "paused":
		status = VMPaused
	}

	return VM{
		VMID:        row.VMID,
		Name:        row.Name,
		Node:        row.Node,
		Status:      status,
		Pool:        row.Pool,
		Tags:        splitProxmoxTags(row.Tags),
		CPUCores:    int(row.MaxCPU),
		MemoryTotal: row.MaxMem,
	}
}

func proxmoxStorageFromRow(row proxmoxResourceRow) Storage {
	return Storage{
		Name:            row.Storage,
		Node:            row.Node,
		Type:            row.PluginType,
		PluginType:      row.PluginType,
		Content:         row.Content,
		Total:           row.MaxDisk,
		Used:            row.Disk,
		SupportsVMState: pluginSupportsVMState(row.PluginType),
	}
}

// proxmoxVersion reads the cluster's reported PVE version string.
func proxmoxVersion(ctx context.Context, rest proxmoxRESTClient) (string, error) {
	raw, err := rest.do(ctx, http.MethodGet, "/version", nil)
	if err != nil {
		return "", err
	}

	var v struct {
		Version string `json:"version"`
	}
	if err := decodeData(raw, &v); err != nil {
		return "", fmt.Errorf("decode version: %w", err)
	}

	return v.Version, nil
}
