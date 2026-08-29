package cluster

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Proxmox is the real cluster implementation, talking to the Proxmox VE REST
// API (https://pve.proxmox.com/pve-docs/api-viewer/). BaseURL, APITokenName
// and APITokenValue are configured per cluster (server/internal/store's
// ClusterRow, wired in registry.go) or from PROXMOX_URL/PROXMOX_API_TOKEN_NAME/
// PROXMOX_API_TOKEN_VALUE in single-cluster setups.
//
// Every write here uses the service account's API token — it never needs the
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
}

// proxmoxResourceRow is one row of /cluster/resources?type=... — Proxmox's
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

// proxmoxSnapshotCapableStorage lists the Proxmox storage plugin types that
// support internal (RAM-included) snapshots — the real capability behind
// Storage.SupportsVMState. Not derivable from the API directly (Proxmox does
// not expose a "supports vmstate" flag), so this mirrors PVE's own documented
// per-plugin snapshot support.
var proxmoxSnapshotCapableStorage = map[string]bool{
	"zfspool": true,
	"lvmthin": true,
	"rbd":     true,
	"btrfs":   true,
}

// Snapshot implements Client: one /cluster/resources call for the node,
// VM, and storage summary, then one /qemu/{vmid}/config (plus, for running
// VMs, one /status/current) call per VM to hydrate what the summary omits
// (see hydrateVM).
func (p Proxmox) Snapshot(ctx context.Context) (Snapshot, error) {
	rest := p.rest()

	raw, err := rest.do(ctx, http.MethodGet, "/cluster/resources", nil)
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
		case "storage":
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
	Type string `json:"type"`
	Name string `json:"name"`
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
		SupportsVMState: proxmoxSnapshotCapableStorage[row.PluginType],
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

// Authenticate implements Client by exchanging username/password for a PVE
// ticket (proving the credentials are correct — ErrNotFound on a rejection,
// matching the fake's contract), then using that ticket, as that user, to
// determine admin status (Permissions.Modify at "/", granted by the
// PVMSS_Admin role per proxmox-permissions.md) and — for non-admins — the
// personal pool PVMSS provisioned for them (EnsurePoolUser's own
// "<pool>@pve" convention, mirrored here in reverse).
func (p Proxmox) Authenticate(ctx context.Context, username, password string) (Identity, error) {
	rest := p.rest()

	ticket, csrf, err := proxmoxTicketAuth(ctx, rest, username, password)
	if err != nil {
		return Identity{}, err
	}

	userRest := rest.withTicket(ticket, csrf)

	isAdmin, err := proxmoxHasPermission(ctx, userRest, "/", "Permissions.Modify")
	if err != nil {
		return Identity{}, err
	}

	var pool string

	if !isAdmin {
		pool, err = proxmoxOwnedPool(ctx, rest, username)
		if err != nil {
			return Identity{}, err
		}
	}

	return Identity{Username: username, Pool: pool, IsAdmin: isAdmin}, nil
}

// proxmoxTicketAuth exchanges credentials for a PVE ticket + CSRF prevention
// token via POST /access/ticket. Deliberately does not reuse
// proxmoxRESTClient.do: this call must not carry any prior authentication
// (it IS the credential check), and a rejected login is a 401 that must map
// to ErrNotFound (wrong credentials), not the generic wrapped-error path.
func proxmoxTicketAuth(ctx context.Context, rest proxmoxRESTClient, username, password string) (ticket, csrf string, err error) {
	form := url.Values{"username": {username}, "password": {password}}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rest.base+"/access/ticket", strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", fmt.Errorf("build ticket request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := rest.http.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", "", ErrNotFound
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return "", "", fmt.Errorf("proxmox ticket auth: HTTP %d", resp.StatusCode)
	}

	var envelope struct {
		Data struct {
			Ticket              string `json:"ticket"`
			CSRFPreventionToken string `json:"CSRFPreventionToken"`
		} `json:"data"`
	}
	if err := decodeJSONBody(resp, &envelope); err != nil {
		return "", "", err
	}

	if envelope.Data.Ticket == "" {
		return "", "", ErrNotFound
	}

	return envelope.Data.Ticket, envelope.Data.CSRFPreventionToken, nil
}

// proxmoxHasPermission checks one privilege at one path for the ticket's own
// user via GET /access/permissions?path=... — any authenticated user may
// query their own effective permissions, no elevated privilege required.
// PVE always nests the response as path -> {privilege: propagate-bool}, even
// for a single requested path (pve-access-control's AccessControl.pm
// `permissions` method: `$res = { $path => $perms }`) — never a flat
// privilege map.
func proxmoxHasPermission(ctx context.Context, rest proxmoxRESTClient, path, privilege string) (bool, error) {
	raw, err := rest.do(ctx, http.MethodGet, "/access/permissions", url.Values{"path": {path}})
	if err != nil {
		return false, err
	}

	var perms map[string]map[string]int
	if err := decodeData(raw, &perms); err != nil {
		return false, fmt.Errorf("decode permissions: %w", err)
	}

	return perms[path][privilege] == 1, nil
}

// proxmoxOwnedPool derives the caller's personal pool from PVMSS's own
// provisioning convention (pools/provision.go: EnsurePoolUser creates
// "<pool>@pve"), verified against the live pool list with the service
// account's Pool.Audit privilege — an end user's own PVMSSUser role does not
// carry that privilege (fake.go's rolePrivileges), so this always uses rest,
// not the user's ticket.
func proxmoxOwnedPool(ctx context.Context, rest proxmoxRESTClient, username string) (string, error) {
	candidate, ok := strings.CutSuffix(username, "@pve")
	if !ok {
		return "", nil
	}

	pools, err := proxmoxListPools(ctx, rest)
	if err != nil {
		return "", err
	}

	for _, pool := range pools {
		if pool.Name == candidate {
			return candidate, nil
		}
	}

	return "", nil
}

// ChangePassword implements Client by re-authenticating as username with
// oldPassword (ErrNotFound if that fails, matching Authenticate's contract)
// and then setting the new password with that same ticket — a genuine
// self-service change, requiring no elevated privilege.
func (p Proxmox) ChangePassword(ctx context.Context, username, oldPassword, newPassword string) error {
	rest := p.rest()

	ticket, csrf, err := proxmoxTicketAuth(ctx, rest, username, oldPassword)
	if err != nil {
		return err
	}

	userRest := rest.withTicket(ticket, csrf)

	_, err = userRest.do(ctx, http.MethodPut, "/access/password", url.Values{
		"userid":   {username},
		"password": {newPassword},
	})

	return err
}

// ListBridges implements Client. Bridges are per-node network configuration
// in Proxmox — there is no cluster-wide listing — so this enumerates nodes
// first, then each node's /network, keeping every node's own view (including
// duplicate bridge names across nodes, e.g. vmbr0 on every node).
func (p Proxmox) ListBridges(ctx context.Context) ([]Bridge, error) {
	rest := p.rest()

	nodes, err := proxmoxNodeNames(ctx, rest)
	if err != nil {
		return nil, err
	}

	var bridges []Bridge

	for _, node := range nodes {
		raw, err := rest.do(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/network", url.PathEscape(node)), nil)
		if err != nil {
			return nil, fmt.Errorf("list network interfaces on %q: %w", node, err)
		}

		var rows []struct {
			Iface    string `json:"iface"`
			Type     string `json:"type"`
			Active   int    `json:"active"`
			Comments string `json:"comments"`
		}
		if err := decodeData(raw, &rows); err != nil {
			return nil, fmt.Errorf("decode network interfaces on %q: %w", node, err)
		}

		for _, row := range rows {
			if row.Type != "bridge" {
				continue
			}

			bridges = append(bridges, Bridge{Name: row.Iface, Node: node, Active: row.Active == 1, Comment: row.Comments})
		}
	}

	return bridges, nil
}

// ListISOs implements Client, enumerating ISO content on every storage that
// offers it. Node scoping matches ListBridges: one row per (node, storage)
// pairing Proxmox itself reports, not deduplicated across nodes.
func (p Proxmox) ListISOs(ctx context.Context) ([]ISOImage, error) {
	rest := p.rest()

	raw, err := rest.do(ctx, http.MethodGet, "/cluster/resources", url.Values{"type": {"storage"}})
	if err != nil {
		return nil, err
	}

	var rows []proxmoxResourceRow
	if err := decodeData(raw, &rows); err != nil {
		return nil, fmt.Errorf("decode storages: %w", err)
	}

	var isos []ISOImage

	for _, row := range rows {
		found, err := proxmoxListISOContent(ctx, rest, row.Node, row.Storage)
		if err != nil {
			return nil, fmt.Errorf("list iso content on %q/%q: %w", row.Node, row.Storage, err)
		}

		isos = append(isos, found...)
	}

	return isos, nil
}

func proxmoxListISOContent(ctx context.Context, rest proxmoxRESTClient, node, storage string) ([]ISOImage, error) {
	raw, err := rest.do(ctx, http.MethodGet,
		fmt.Sprintf("/nodes/%s/storage/%s/content", url.PathEscape(node), url.PathEscape(storage)),
		url.Values{"content": {"iso"}})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}

		return nil, err
	}

	var rows []struct {
		VolID string `json:"volid"`
		Size  int64  `json:"size"`
	}
	if err := decodeData(raw, &rows); err != nil {
		return nil, fmt.Errorf("decode iso content: %w", err)
	}

	isos := make([]ISOImage, 0, len(rows))

	for _, row := range rows {
		_, file, ok := strings.Cut(row.VolID, ":")
		if !ok {
			continue
		}

		file = strings.TrimPrefix(file, "iso/")
		isos = append(isos, ISOImage{Storage: storage, Node: node, File: file, SizeBytes: row.Size})
	}

	return isos, nil
}

// ListTemplates implements Client, enumerating template VMs (template=1) via
// /cluster/resources?type=vm (US2/issue-02 T056). Each row is hydrated with
// its primary disk's storage, size, and bus via /nodes/{node}/qemu/{vmid}/config
// so the clone path can decide linked vs full and target the correct resize key.
// CloudInitCapable is detected by the presence of a cloud-init drive in the
// fixed ide3 slot (proxmox_config.go's cloudInitDiskKey) — the same slot
// EnsureCloudInitDrive writes, so a template that already has one is cloud-init
// capable.
func (p Proxmox) ListTemplates(ctx context.Context) ([]TemplateVM, error) {
	rest := p.rest()

	raw, err := rest.do(ctx, http.MethodGet, "/cluster/resources", url.Values{"type": {"vm"}})
	if err != nil {
		return nil, err
	}

	var rows []proxmoxResourceRow
	if err := decodeData(raw, &rows); err != nil {
		return nil, fmt.Errorf("decode template vms: %w", err)
	}

	var templates []TemplateVM

	for _, row := range rows {
		if row.Template != 1 {
			continue
		}

		diskStorage, diskSizeGB, diskBus, cloudInitCapable, err := proxmoxTemplateDisk(ctx, rest, row.Node, row.VMID)
		if err != nil {
			return nil, fmt.Errorf("read template %d disk on %q: %w", row.VMID, row.Node, err)
		}

		templates = append(templates, TemplateVM{
			VMID:             row.VMID,
			Node:             row.Node,
			Name:             row.Name,
			CloudInitCapable: cloudInitCapable,
			DiskStorage:      diskStorage,
			DiskSizeGB:       diskSizeGB,
			DiskBus:          diskBus,
		})
	}

	return templates, nil
}

// proxmoxTemplateDisk reads the template's primary disk (the first scsi/virtio/
// sata/ide key) from its config, returning (storage, sizeGB, bus, cloudInitCapable).
// The primary disk is the first non-cdrom, non-cloudinit disk found in bus-family
// priority order (scsi → virtio → sata → ide); Proxmox templates typically have a
// single disk. Disk parsing reuses parseDiskValue (proxmox_config.go) so the format
// "local-lvm:vm-101-disk-0,size=32G" is handled the same way as the disk tab and
// the create wizard. CloudInitCapable is true when the fixed ide3 slot
// (cloudInitDiskKey) holds a cloud-init drive — the same slot EnsureCloudInitDrive
// writes, so a template that already has one is cloud-init capable.
func proxmoxTemplateDisk(ctx context.Context, rest proxmoxRESTClient, node string, vmid int) (storage string, sizeGB int, bus string, cloudInitCapable bool, err error) {
	cfg, err := fetchVMConfig(ctx, rest, node, vmid)
	if err != nil {
		return "", 0, "", false, err
	}

	for _, b := range []string{"scsi", "virtio", "sata", "ide"} {
		for i := range 16 {
			key := fmt.Sprintf("%s%d", b, i)
			if key == cdromDiskKey || key == cloudInitDiskKey {
				continue
			}

			val, ok := cfg[key].(string)
			if !ok || val == "" || val == "none" {
				continue
			}

			diskStorage, diskSizeGB := parseDiskValue(val)
			if diskStorage == "" {
				continue
			}

			cloudInitCapable = proxmoxConfigHasCloudInitDrive(cfg)

			return diskStorage, diskSizeGB, b, cloudInitCapable, nil
		}
	}

	cloudInitCapable = proxmoxConfigHasCloudInitDrive(cfg)

	return "", 0, "", cloudInitCapable, nil
}

// proxmoxConfigHasCloudInitDrive reports whether cfg's fixed cloudInitDiskKey
// slot holds a cloud-init drive. Mirrors EnsureCloudInitDrive's own detection
// (proxmox_cloudinit.go): the slot value contains "cloudinit" when the drive
// is present.
func proxmoxConfigHasCloudInitDrive(cfg proxmoxVMConfig) bool {
	if value, ok := cfg[cloudInitDiskKey].(string); ok && strings.Contains(value, "cloudinit") {
		return true
	}

	return false
}

// proxmoxNodeNames lists every node name in the cluster.
func proxmoxNodeNames(ctx context.Context, rest proxmoxRESTClient) ([]string, error) {
	raw, err := rest.do(ctx, http.MethodGet, "/nodes", nil)
	if err != nil {
		return nil, err
	}

	var rows []struct {
		Node string `json:"node"`
	}
	if err := decodeData(raw, &rows); err != nil {
		return nil, fmt.Errorf("decode nodes: %w", err)
	}

	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Node)
	}

	return names, nil
}
