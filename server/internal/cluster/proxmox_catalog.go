package cluster

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

// ListBridges implements Client. Bridges are per-node network configuration
// in Proxmox - there is no cluster-wide listing - so this enumerates nodes
// first, then each node's /network, keeping every node's own view (including
// duplicate bridge names across nodes, e.g. vmbr0 on every node).
func (p Proxmox) ListBridges(ctx context.Context) ([]Bridge, error) {
	rest := p.rest()

	nodes, err := proxmoxNodes(ctx, rest)
	if err != nil {
		return nil, err
	}

	var bridges []Bridge

	for _, node := range nodes {
		if !proxmoxNodeOnline(node.Status) {
			continue
		}

		nodeBridges, err := proxmoxListNodeBridges(ctx, rest, node.Name)
		if err != nil {
			return nil, err
		}

		bridges = append(bridges, nodeBridges...)
	}

	return bridges, nil
}

// proxmoxListNodeBridges fetches and decodes one node's network interfaces,
// returning only the bridge-typed rows. A node that is temporarily
// unavailable (RejectionError 595) is skipped - the caller keeps the other
// nodes' bridges. Extracted from ListBridges to keep its cognitive
// complexity under the go:S3776 limit.
func proxmoxListNodeBridges(ctx context.Context, rest proxmoxRESTClient, node string) ([]Bridge, error) {
	raw, err := rest.do(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/network", url.PathEscape(node)), nil)
	if err != nil {
		if isNodeUnavailable(err) {
			return nil, nil
		}

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

	var bridges []Bridge

	for _, row := range rows {
		if row.Type != "bridge" {
			continue
		}

		bridges = append(bridges, Bridge{Name: row.Iface, Node: node, Active: row.Active == 1, Comment: row.Comments})
	}

	return bridges, nil
}

// ListISOs implements Client, enumerating ISO content on every storage that
// offers it. Node scoping matches ListBridges: one row per (node, storage)
// pairing Proxmox itself reports, not deduplicated across nodes.
func (p Proxmox) ListISOs(ctx context.Context) ([]ISOImage, error) {
	rest := p.rest()

	raw, err := rest.do(ctx, http.MethodGet, proxmoxClusterResourcesPath, url.Values{proxmoxResourceTypeParam: {proxmoxStorageType}})
	if err != nil {
		return nil, err
	}

	var rows []proxmoxResourceRow
	if err := decodeData(raw, &rows); err != nil {
		return nil, fmt.Errorf("decode storages: %w", err)
	}

	var isos []ISOImage

	for _, row := range rows {
		if !proxmoxStorageAvailable(row.Status) {
			continue
		}

		found, err := proxmoxListContent(ctx, rest, row.Node, row.Storage, "iso")
		if err != nil {
			if isNodeUnavailable(err) {
				continue
			}

			return nil, fmt.Errorf("list iso content on %q/%q: %w", row.Node, row.Storage, err)
		}

		isos = append(isos, found...)
	}

	return isos, nil
}

// ListCloudImages implements Client, enumerating cloud images on every
// import-capable storage. Proxmox lists .qcow2, .raw, .vmdk and .ova files
// under content=import; PVMSS keeps only the disk formats import-from takes
// as a plain volid (.ova/.ovf need the nested <file>.ova/<disk> form and are
// skipped). .img files are vtype 'iso' and rejected, and absolute filesystem
// paths are root@pam-only. Node scoping matches ListISOs: one row per
// (node, storage) pairing.
// proxmoxListContent lists files of a given content type on a storage. The
// content parameter is "iso" or "import". For iso content, volids are
// <storage>:iso/<file>; for import content, <storage>:import/<file>.
func proxmoxListContent(ctx context.Context, rest proxmoxRESTClient, node, storage, content string) ([]ISOImage, error) {
	raw, err := rest.do(ctx, http.MethodGet,
		fmt.Sprintf("/nodes/%s/storage/%s/content", url.PathEscape(node), url.PathEscape(storage)),
		url.Values{"content": {content}})
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
		return nil, fmt.Errorf("decode %s content: %w", content, err)
	}

	isos := make([]ISOImage, 0, len(rows))

	for _, row := range rows {
		_, file, ok := strings.Cut(row.VolID, ":")
		if !ok {
			continue
		}

		file = strings.TrimPrefix(file, content+"/")
		isos = append(isos, ISOImage{Storage: storage, Node: node, File: file, SizeBytes: row.Size})
	}

	return isos, nil
}

// ListTemplates implements Client, enumerating template VMs (template=1) via
// /cluster/resources?type=vm. Each row is hydrated with
// its primary disk's storage, size, and bus via /nodes/{node}/qemu/{vmid}/config
// so the clone path can decide linked vs full and target the correct resize key.
// CloudInitCapable is detected by the presence of a cloud-init drive in the
// fixed ide3 slot (proxmox_config.go's cloudInitDiskKey) - the same slot
// EnsureCloudInitDrive writes, so a template that already has one is cloud-init
// capable.
func (p Proxmox) ListTemplates(ctx context.Context) ([]TemplateVM, error) {
	rest := p.rest()

	raw, err := rest.do(ctx, http.MethodGet, proxmoxClusterResourcesPath, url.Values{proxmoxResourceTypeParam: {"vm"}})
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

		// Degrade per template instead of aborting the whole list
		// one unreadable config must not blank the admin page or block every
		// toggle.
		diskStorage, diskSizeGB, diskBus, err := proxmoxTemplateDisk(ctx, rest, row.Node, row.VMID)
		if err != nil {
			//nolint:sloglint // Proxmox has no injected logger yet; ticket 06 (observability) adds one
			slog.WarnContext(ctx, "template config unreadable, keeping row without disk fields",
				"component", "cluster", "vmid", row.VMID, "node", row.Node, "error", err)

			templates = append(templates, TemplateVM{
				VMID: row.VMID, Node: row.Node, Name: row.Name, DiskUnreadable: true,
			})

			continue
		}

		templates = append(templates, TemplateVM{
			VMID:        row.VMID,
			Node:        row.Node,
			Name:        row.Name,
			DiskStorage: diskStorage,
			DiskSizeGB:  diskSizeGB,
			DiskBus:     diskBus,
		})
	}

	return templates, nil
}

// TemplateByVMID implements Client: one /cluster/resources call plus one
// config read for a single template (no full re-hydration per toggle or clone). Unknown VMIDs
// are ErrNotFound; an unreadable config
// degrades to a DiskUnreadable row with the discovered node kept.
func (p Proxmox) TemplateByVMID(ctx context.Context, vmid int) (TemplateVM, error) {
	rest := p.rest()

	raw, err := rest.do(ctx, http.MethodGet, proxmoxClusterResourcesPath, url.Values{proxmoxResourceTypeParam: {"vm"}})
	if err != nil {
		return TemplateVM{}, err
	}

	var rows []proxmoxResourceRow
	if err := decodeData(raw, &rows); err != nil {
		return TemplateVM{}, fmt.Errorf("decode template vms: %w", err)
	}

	var found *proxmoxResourceRow

	for i := range rows {
		if rows[i].Template == 1 && rows[i].VMID == vmid {
			found = &rows[i]
			break
		}
	}

	if found == nil {
		return TemplateVM{}, ErrNotFound
	}

	cfg, err := fetchVMConfig(ctx, rest, found.Node, found.VMID)
	if err != nil {
		//nolint:sloglint // Proxmox has no injected logger yet; ticket 06 (observability) adds one
		slog.WarnContext(ctx, "template config unreadable, returning row without disk fields",
			"component", "cluster", "vmid", found.VMID, "node", found.Node, "error", err)

		return TemplateVM{VMID: found.VMID, Node: found.Node, Name: found.Name, DiskUnreadable: true}, nil
	}

	diskStorage, diskSizeGB, diskBus := templateDiskFromConfig(cfg)

	tmpl := TemplateVM{
		VMID:        found.VMID,
		Node:        found.Node,
		Name:        found.Name,
		DiskStorage: diskStorage,
		DiskSizeGB:  diskSizeGB,
		DiskBus:     diskBus,
	}
	templateHardware(&tmpl, cfg)

	return tmpl, nil
}

// StorageFreeSpace returns the available bytes on a storage backend on a node.
// Queries GET /nodes/{node}/storage/{storage}/status and
// extracts the `avail` field from the response.
func (p Proxmox) StorageFreeSpace(ctx context.Context, node, storage string) (int64, error) {
	raw, err := p.rest().do(ctx, http.MethodGet,
		fmt.Sprintf("/nodes/%s/storage/%s/status", url.PathEscape(node), url.PathEscape(storage)), nil)
	if err != nil {
		return 0, err
	}

	var row struct {
		Avail int64 `json:"avail"`
	}
	if err := decodeData(raw, &row); err != nil {
		return 0, fmt.Errorf("decode storage status: %w", err)
	}

	return row.Avail, nil
}

// proxmoxTemplateDisk reads the template's primary disk (the first scsi/virtio/
// sata/ide key) from its config, returning (storage, sizeGB, bus, cloudInitCapable).
// The primary disk is the first non-cdrom, non-cloudinit disk found in bus-family
// priority order (scsi → virtio → sata → ide); Proxmox templates typically have a
// single disk. Disk parsing reuses parseDiskValue (proxmox_config.go) so the format
// "local-lvm:vm-101-disk-0,size=32G" is handled the same way as the disk tab and
// the create wizard. CloudInitCapable is true when the fixed ide3 slot
// (cloudInitDiskKey) holds a cloud-init drive - the same slot EnsureCloudInitDrive
// writes, so a template that already has one is cloud-init capable.
func proxmoxTemplateDisk(ctx context.Context, rest proxmoxRESTClient, node string, vmid int) (storage string, sizeGB int, bus string, err error) {
	cfg, err := fetchVMConfig(ctx, rest, node, vmid)
	if err != nil {
		return "", 0, "", err
	}

	storage, sizeGB, bus = templateDiskFromConfig(cfg)

	return storage, sizeGB, bus, nil
}

// templateDiskFromConfig picks the primary disk from an already-read config.
func templateDiskFromConfig(cfg proxmoxVMConfig) (storage string, sizeGB int, bus string) {
	for _, b := range []string{"scsi", "virtio", "sata", "ide"} {
		for i := range 16 {
			key := fmt.Sprintf("%s%d", b, i)
			if key == cdromDiskKey || key == cloudInitDiskKey {
				continue
			}

			val, ok := cfg[key].(string)
			if !ok || val == "" || val == proxmoxEmptyVolume {
				continue
			}

			diskStorage, diskSizeGB, _ := parseDiskValue(val)
			if diskStorage == "" {
				continue
			}

			return diskStorage, diskSizeGB, b
		}
	}

	return "", 0, ""
}

// templateHardware fills the hardware and NIC fields of a template row from
// its config. Sockets and cores default to 1, as hydrateVM does for VMs.
func templateHardware(tmpl *TemplateVM, cfg proxmoxVMConfig) {
	tmpl.Sockets = max(cfg.int("sockets"), 1)
	tmpl.Cores = max(cfg.int("cores"), 1)
	tmpl.MemoryTotal = int64(cfg.int("memory")) * bytesPerMiBConfig
	_, tmpl.DiskTotal = parseDisks(cfg)
	tmpl.NetworkInterfaces = parseNetworkInterfaces(cfg)
}

// proxmoxNodeRow is one row of /nodes: the node name plus the cluster's own
// view of its availability ("online"/"offline").
type proxmoxNodeRow struct {
	Name   string `json:"node"`
	Status string `json:"status"`
}

// proxmoxNodes lists every node in the cluster with its status.
func proxmoxNodes(ctx context.Context, rest proxmoxRESTClient) ([]proxmoxNodeRow, error) {
	raw, err := rest.do(ctx, http.MethodGet, "/nodes", nil)
	if err != nil {
		return nil, err
	}

	var rows []proxmoxNodeRow
	if err := decodeData(raw, &rows); err != nil {
		return nil, fmt.Errorf("decode nodes: %w", err)
	}

	return rows, nil
}

// proxmoxNodeOnline reports whether a /nodes status row describes a node
// whose pveproxy is expected to answer. An empty status (older PVE releases)
// is treated as online - the per-call 595 skip remains the safety net.
func proxmoxNodeOnline(status string) bool {
	return status == "" || status == "online"
}

// proxmoxStorageAvailable reports whether a /cluster/resources storage row
// can serve content. "unknown" means the node's pvestatd cannot report it
// (node offline); "inactive" means Proxmox itself cannot read it - asking
// either for ISO content wastes seconds per storage and 595s/500s the call.
func proxmoxStorageAvailable(status string) bool {
	return status == "" || status == "available"
}
