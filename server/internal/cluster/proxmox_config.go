package cluster

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// proxmoxVMConfig is a decoded /nodes/{node}/qemu/{vmid}/config response.
// Proxmox mixes types across keys — cores/sockets/memory are numbers, disk
// and network entries are strings — so it is decoded loosely and read through
// the str/int helpers below rather than a fixed struct.
type proxmoxVMConfig map[string]any

func (c proxmoxVMConfig) str(key string) string {
	v, ok := c[key]
	if !ok {
		return ""
	}

	if s, ok := v.(string); ok {
		return s
	}

	return fmt.Sprintf("%v", v)
}

func (c proxmoxVMConfig) int(key string) int {
	v, ok := c[key]
	if !ok {
		return 0
	}

	switch t := v.(type) {
	case float64:
		return int(t)
	case string:
		n, _ := strconv.Atoi(t)
		return n
	default:
		return 0
	}
}

// fetchVMConfig reads a VM's full configuration.
func fetchVMConfig(ctx context.Context, rest proxmoxRESTClient, node string, vmid int) (proxmoxVMConfig, error) {
	raw, err := rest.do(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid), nil)
	if err != nil {
		return nil, err
	}

	var cfg proxmoxVMConfig
	if err := decodeData(raw, &cfg); err != nil {
		return nil, fmt.Errorf("decode vm config: %w", err)
	}

	return cfg, nil
}

// proxmoxBusRange is the real hardware slot range per bus (virtio 0-15, scsi
// 0-30, sata 0-5, ide 0-3), mirroring the usable-slot counts vm/disks.go
// enforces on write (maxDisksForBus) — kept as a separate table because
// package vm already imports package cluster, so the reverse import is not
// available here. Keep the two in sync if Proxmox's own limits ever change.
var proxmoxBusRange = map[DiskBus]int{
	DiskBusVirtio: 15,
	DiskBusSCSI:   30,
	DiskBusSATA:   5,
	DiskBusIDE:    3,
}

// cdromDiskKey is the fixed slot PVMSS always uses for the CD-ROM drive
// (client.go: "CDROMState describes the fixed ide2 CD-ROM drive").
const cdromDiskKey = "ide2"

// cloudInitDiskKey is the fixed slot PVMSS always uses for the cloud-init
// drive (see EnsureCloudInitDrive in proxmox_cloudinit.go). Deliberately not
// ide2: that slot is reserved for the CD-ROM feature above, and the two would
// otherwise silently overwrite each other on the same VM. vm/disks.go's
// maxDisksForBus[IDE] is 2, not the hardware's 3 non-cdrom slots, to keep the
// regular-disk allocator (nextProxmoxDiskKey below) from ever offering this
// slot to a caller.
const cloudInitDiskKey = "ide3"

// parseDisks reads every attached data disk from cfg in a deterministic
// order (bus, then index) — map iteration order is random and callers (the
// disk tab, the create wizard) expect a stable listing.
func parseDisks(cfg proxmoxVMConfig) ([]Disk, int64) {
	var disks []Disk

	var total int64

	for _, bus := range []DiskBus{DiskBusVirtio, DiskBusSCSI, DiskBusSATA, DiskBusIDE} {
		for index := 0; index <= proxmoxBusRange[bus]; index++ {
			key := fmt.Sprintf("%s%d", bus, index)
			if key == cdromDiskKey || key == cloudInitDiskKey {
				continue
			}

			value, ok := cfg[key].(string)
			if !ok || value == "" || value == "none" {
				continue
			}

			storage, sizeGB := parseDiskValue(value)
			if storage == "" {
				continue
			}

			disks = append(disks, Disk{Key: key, Bus: bus, BusIndex: index, Storage: storage, SizeGB: sizeGB})
			total += int64(sizeGB) * 1024 * 1024 * 1024
		}
	}

	return disks, total
}

// parseDiskValue splits a Proxmox disk config value ("local-lvm:vm-101-disk-0,size=32G")
// into its storage and size in whole GB.
func parseDiskValue(value string) (storage string, sizeGB int) {
	volume, options, _ := strings.Cut(value, ",")

	storage, _, ok := strings.Cut(volume, ":")
	if !ok {
		return "", 0
	}

	for opt := range strings.SplitSeq(options, ",") {
		key, val, ok := strings.Cut(opt, "=")
		if ok && key == "size" {
			sizeGB = parseProxmoxSizeGB(val)
		}
	}

	return storage, sizeGB
}

// parseProxmoxSizeGB converts a Proxmox size value (a trailing-unit string
// like "32G"/"512M", or a bare byte count) to whole GB.
func parseProxmoxSizeGB(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}

	unit := raw[len(raw)-1]

	var multiplier float64

	var numPart string

	switch unit {
	case 'T', 't':
		multiplier, numPart = 1024, raw[:len(raw)-1]
	case 'G', 'g':
		multiplier, numPart = 1, raw[:len(raw)-1]
	case 'M', 'm':
		multiplier, numPart = 1.0/1024, raw[:len(raw)-1]
	case 'K', 'k':
		multiplier, numPart = 1.0/(1024*1024), raw[:len(raw)-1]
	default:
		if n, err := strconv.ParseFloat(raw, 64); err == nil {
			return int(n / (1024 * 1024 * 1024))
		}

		return 0
	}

	n, err := strconv.ParseFloat(numPart, 64)
	if err != nil {
		return 0
	}

	return int(n * multiplier)
}

// parseCDROM reads the fixed ide2 slot's state.
func parseCDROM(cfg proxmoxVMConfig) CDROMState {
	value, ok := cfg[cdromDiskKey].(string)
	if !ok || value == "" {
		return CDROMState{State: CDROMAbsent}
	}

	volume, _, _ := strings.Cut(value, ",")
	if volume == "" || volume == "none" {
		return CDROMState{State: CDROMEmpty}
	}

	return CDROMState{State: CDROMMounted, ISOVolID: volume}
}

// proxmoxNICModels are the network card models Proxmox accepts as the
// key-less first segment of a netN value.
var proxmoxNICModels = map[string]bool{
	"virtio": true, "e1000": true, "e1000e": true, "rtl8139": true, "vmxnet3": true,
}

// parseNetworkInterfaces reads every attached NIC from cfg (net0..net31 —
// Proxmox's own hardware limit). IPAddresses is deliberately left empty:
// populating it needs a live QEMU guest agent call correlated by MAC against
// each NIC, a per-VM extra round trip this reader does not make.
func parseNetworkInterfaces(cfg proxmoxVMConfig) []NetworkInterface {
	var nics []NetworkInterface

	for index := range 32 {
		value, ok := cfg[fmt.Sprintf("net%d", index)].(string)
		if !ok || value == "" {
			continue
		}

		nics = append(nics, parseNetValue(index, value))
	}

	return nics
}

func parseNetValue(index int, value string) NetworkInterface {
	nic := NetworkInterface{Index: index}

	for opt := range strings.SplitSeq(value, ",") {
		key, val, hasEquals := strings.Cut(opt, "=")

		switch {
		case !hasEquals:
			// A bare model with no MAC ("virtio" alone, auto-assigned by Proxmox).
			if proxmoxNICModels[key] {
				nic.Model = key
			}
		case key == "bridge":
			nic.Bridge = val
		case key == "tag":
			if n, err := strconv.Atoi(val); err == nil {
				nic.VLAN = &n
			}
		case key == "rate":
			if n, err := strconv.ParseFloat(val, 64); err == nil {
				r := int(n)
				nic.RateMbps = &r
			}
		case proxmoxNICModels[key]:
			nic.Model = key
			nic.MAC = val
		}
	}

	return nic
}

// parseBootOrder reads the modern "order=scsi0;ide2;net0" boot form. Older
// Proxmox installs may still carry the legacy "bootdisk"/flag-string form;
// this reader does not translate it, matching the fake's own scope (no boot
// order fixture beyond the order list itself).
func parseBootOrder(cfg proxmoxVMConfig) []string {
	raw := cfg.str("boot")

	_, order, ok := strings.Cut(raw, "order=")
	if !ok {
		return nil
	}

	var result []string

	for entry := range strings.SplitSeq(order, ";") {
		if entry = strings.TrimSpace(entry); entry != "" {
			result = append(result, entry)
		}
	}

	return result
}

// splitProxmoxTags splits Proxmox's semicolon-joined tag string.
func splitProxmoxTags(raw string) []string {
	if raw == "" {
		return nil
	}

	var result []string

	for tag := range strings.SplitSeq(raw, ";") {
		if tag = strings.TrimSpace(tag); tag != "" {
			result = append(result, tag)
		}
	}

	return result
}

// hydrateVM fills in the config-level fields Snapshot's /cluster/resources
// call cannot provide (Sockets, Cores, Disks, CDROM, NetworkInterfaces,
// BootOrder, Description) and, for a running VM, its live uptime.
//
// ponytail: one config fetch (plus one status fetch for running VMs) per VM,
// sequential. Fine for a lab-sized cluster; a large fleet would want bounded
// concurrency here — add it if inventory refresh starts taking noticeably
// long.
func hydrateVM(ctx context.Context, rest proxmoxRESTClient, vm *VM) error {
	cfg, err := fetchVMConfig(ctx, rest, vm.Node, vm.VMID)
	if err != nil {
		return err
	}

	vm.Sockets = cfg.int("sockets")
	if vm.Sockets == 0 {
		vm.Sockets = 1
	}

	vm.Cores = cfg.int("cores")
	if vm.Cores == 0 {
		vm.Cores = 1
	}

	vm.Disks, vm.DiskTotal = parseDisks(cfg)
	vm.CDROM = parseCDROM(cfg)
	vm.NetworkInterfaces = parseNetworkInterfaces(cfg)
	vm.BootOrder = parseBootOrder(cfg)
	vm.Description = cfg.str("description")

	if len(vm.Tags) == 0 {
		vm.Tags = splitProxmoxTags(cfg.str("tags"))
	}

	if vm.Status != VMRunning {
		return nil
	}

	uptime, err := fetchUptime(ctx, rest, vm.Node, vm.VMID)
	if err != nil {
		return nil //nolint:nilerr // best-effort: a stale/racing status read should not fail the whole snapshot
	}

	vm.Uptime = uptime

	return nil
}

func fetchUptime(ctx context.Context, rest proxmoxRESTClient, node string, vmid int) (time.Duration, error) {
	raw, err := rest.do(ctx, http.MethodGet, fmt.Sprintf("/nodes/%s/qemu/%d/status/current", url.PathEscape(node), vmid), nil)
	if err != nil {
		return 0, err
	}

	var status struct {
		Uptime int64 `json:"uptime"`
	}
	if err := decodeData(raw, &status); err != nil {
		return 0, fmt.Errorf("decode vm status: %w", err)
	}

	return time.Duration(status.Uptime) * time.Second, nil
}

// encodeNetValue renders a NetworkInterface back to Proxmox's netN grammar.
func encodeNetValue(iface NetworkInterface) string {
	model := iface.Model
	if model == "" {
		model = "virtio"
	}

	parts := []string{model}
	if iface.MAC != "" {
		parts[0] = model + "=" + iface.MAC
	}

	if iface.Bridge != "" {
		parts = append(parts, "bridge="+iface.Bridge)
	}

	if iface.VLAN != nil {
		parts = append(parts, fmt.Sprintf("tag=%d", *iface.VLAN))
	}

	if iface.RateMbps != nil {
		parts = append(parts, fmt.Sprintf("rate=%d", *iface.RateMbps))
	}

	return strings.Join(parts, ",")
}
