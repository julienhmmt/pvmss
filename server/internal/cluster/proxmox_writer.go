package cluster

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// proxmoxValidActions mirrors fake.go's validActions — the exhaustive set of
// power transitions FR-006 accepts. vm.IsValidAction already gates this
// upstream; checked again here defensively, before any HTTP call, matching
// the fake's own defense-in-depth.
var proxmoxValidActions = map[string]bool{
	actionStart: true, actionStop: true, actionShutdown: true,
	actionReboot: true, actionReset: true,
	actionPause: true, actionResume: true,
}

// vmConfigPath builds the PUT /nodes/{node}/qemu/{vmid}/config endpoint used
// by every Writer method that mutates a VM's config.
func vmConfigPath(node string, vmid int) string {
	return fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid)
}

// Action implements Writer via POST /nodes/{node}/qemu/{vmid}/status/{action}.
// Proxmox returns a task UPID; it is discarded — the Writer contract is
// synchronous (error only), matching how the fake and every caller (vm.Action)
// already treat power transitions as immediate.
func (p Proxmox) Action(ctx context.Context, node string, vmid int, action string) error {
	if !proxmoxValidActions[action] {
		return ErrInvalidAction
	}

	_, err := p.rest().do(ctx, http.MethodPost,
		fmt.Sprintf("/nodes/%s/qemu/%d/status/%s", url.PathEscape(node), vmid, action), nil)

	return err
}

// Delete implements Writer via DELETE /nodes/{node}/qemu/{vmid}. purge=1 also
// removes references from backup jobs and pools — matching the product's own
// "irreversible, no soft-delete, no undo" contract (client.go: VM.Description
// doc, V14). Proxmox rejects deleting a running VM with HTTP 500 ("VM X is
// running - destroy failed"); that is mapped to ErrVMRunning so callers can
// distinguish it from a genuine cluster fault and decide whether to force-stop
// first (see vm.Delete's Force flag) rather than papering over it here.
func (p Proxmox) Delete(ctx context.Context, node string, vmid int) error {
	_, err := p.rest().do(ctx, http.MethodDelete,
		fmt.Sprintf("/nodes/%s/qemu/%d", url.PathEscape(node), vmid), url.Values{"purge": {"1"}})
	if err != nil && strings.Contains(err.Error(), "is running") {
		return fmt.Errorf("%w: %w", ErrVMRunning, err)
	}

	return err
}

// Patch implements Writer. Empty arguments are ignored, matching the fake's
// contract — the caller (vm.Patch) decides which fields to send.
func (p Proxmox) Patch(ctx context.Context, node string, vmid int, name, description string) error {
	form := url.Values{}
	if name != "" {
		form.Set("name", name)
	}

	if description != "" {
		form.Set("description", description)
	}

	if len(form) == 0 {
		return nil
	}

	_, err := p.rest().do(ctx, http.MethodPut, vmConfigPath(node, vmid), form)

	return err
}

// AddDisk implements Writer: finds the next free slot on bus from the VM's
// live config (not the caller's cached view — vm.AddDisk already checked slot
// availability against its own cache before calling this, so a live re-check
// only helps, never conflicts) and allocates a new disk there.
func (p Proxmox) AddDisk(ctx context.Context, node string, vmid int, bus, storage string, sizeGB int) (string, error) {
	rest := p.rest()

	cfg, err := fetchVMConfig(ctx, rest, node, vmid)
	if err != nil {
		return "", err
	}

	key, err := nextProxmoxDiskKey(cfg, DiskBus(bus))
	if err != nil {
		return "", err
	}

	_, err = rest.do(ctx, http.MethodPut, vmConfigPath(node, vmid), url.Values{
		key: {fmt.Sprintf("%s:%d", storage, sizeGB)},
	})
	if err != nil {
		return "", err
	}

	return key, nil
}

func nextProxmoxDiskKey(cfg proxmoxVMConfig, bus DiskBus) (string, error) {
	maxIndex, ok := proxmoxBusRange[bus]
	if !ok {
		return "", fmt.Errorf("%w: unknown disk bus %q", ErrInvalidAction, bus)
	}

	for index := 0; index <= maxIndex; index++ {
		key := fmt.Sprintf("%s%d", bus, index)
		if key == cdromDiskKey || key == cloudInitDiskKey {
			continue
		}

		if _, exists := cfg[key]; !exists {
			return key, nil
		}
	}

	return "", fmt.Errorf("no free %s disk slot", bus)
}

// ResizeDisk implements Writer via the dedicated resize endpoint. sizeGB is
// the new absolute size (matching the fake's contract); Proxmox rejects a
// size smaller than the disk's current size.
func (p Proxmox) ResizeDisk(ctx context.Context, node string, vmid int, diskKey string, sizeGB int) error {
	_, err := p.rest().do(ctx, http.MethodPut, fmt.Sprintf("/nodes/%s/qemu/%d/resize", url.PathEscape(node), vmid), url.Values{
		"disk": {diskKey},
		"size": {fmt.Sprintf("%dG", sizeGB)},
	})

	return err
}

// DeleteDisk implements Writer via /unlink with force=1, which also purges
// the underlying volume rather than leaving it as an "unused" entry.
func (p Proxmox) DeleteDisk(ctx context.Context, node string, vmid int, diskKey string) error {
	_, err := p.rest().do(ctx, http.MethodPut, fmt.Sprintf("/nodes/%s/qemu/%d/unlink", url.PathEscape(node), vmid), url.Values{
		"idlist": {diskKey},
		"force":  {"1"},
	})

	return err
}

// SetCDROM implements Writer against the fixed ide2 slot (client.go).
func (p Proxmox) SetCDROM(ctx context.Context, node string, vmid int, cdrom CDROMState) error {
	path := vmConfigPath(node, vmid)
	rest := p.rest()

	var err error

	switch cdrom.State {
	case CDROMAbsent:
		_, err = rest.do(ctx, http.MethodPut, path, url.Values{"delete": {cdromDiskKey}})
	case CDROMEmpty:
		_, err = rest.do(ctx, http.MethodPut, path, url.Values{cdromDiskKey: {"none,media=cdrom"}})
	case CDROMMounted:
		_, err = rest.do(ctx, http.MethodPut, path, url.Values{cdromDiskKey: {cdrom.ISOVolID + ",media=cdrom"}})
	default:
		return fmt.Errorf("%w: unknown cdrom state %q", ErrInvalidAction, cdrom.State)
	}

	return err
}

// UpdateNetwork implements Writer with full-replace semantics (matching the
// fake's "replaces the fake VM's network interfaces" contract): every netN
// index present in the live config but absent from interfaces is deleted in
// the same call that writes the new set.
func (p Proxmox) UpdateNetwork(ctx context.Context, node string, vmid int, interfaces []NetworkInterface) error {
	rest := p.rest()

	cfg, err := fetchVMConfig(ctx, rest, node, vmid)
	if err != nil {
		return err
	}

	keep := make(map[int]bool, len(interfaces))
	form := url.Values{}

	for _, iface := range interfaces {
		keep[iface.Index] = true
		form.Set(fmt.Sprintf("net%d", iface.Index), encodeNetValue(iface))
	}

	var toDelete []string

	for index := range 32 {
		if keep[index] {
			continue
		}

		key := fmt.Sprintf("net%d", index)
		if _, exists := cfg[key]; exists {
			toDelete = append(toDelete, key)
		}
	}

	if len(toDelete) > 0 {
		form.Set("delete", strings.Join(toDelete, ","))
	}

	_, err = rest.do(ctx, http.MethodPut, vmConfigPath(node, vmid), form)

	return err
}

// UpdateHardware implements Writer. tags is always written, even when empty —
// matching the fake's unconditional overwrite (`Tags = append(nil, tags...)`)
// rather than treating a nil/empty slice as "leave tags unchanged".
func (p Proxmox) UpdateHardware(ctx context.Context, node string, vmid, sockets, cores, memoryMB int, tags []string) error {
	form := url.Values{
		"sockets": {strconv.Itoa(sockets)},
		"cores":   {strconv.Itoa(cores)},
		"memory":  {strconv.Itoa(memoryMB)},
		"tags":    {strings.Join(tags, ";")},
	}

	_, err := p.rest().do(ctx, http.MethodPut, vmConfigPath(node, vmid), form)

	return err
}

// EnableSerial implements Writer: provisions a socket-backed serial port
// (serial0) on an existing VM so the PVMSS Text/serial console works for VMs
// created before serial0 was added at create time. A socket-backed port needs
// no host device and is safe to add to any VM unconditionally.
func (p Proxmox) EnableSerial(ctx context.Context, node string, vmid int) error {
	_, err := p.rest().do(ctx, http.MethodPut, vmConfigPath(node, vmid), url.Values{
		"serial0": {"socket"},
	})

	return err
}
