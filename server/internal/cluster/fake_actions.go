package cluster

import (
	"context"
	"fmt"
	"slices"
)

// Action implements Writer - a power transition on the Index-resolved node.
// It mutates the VM's Status so a subsequent Snapshot reflects it (the fake
// demonstrates the feature), and records the call.
//
// status-incompatible transitions are rejected - start on an
// already-running VM, stop/shutdown on an already-stopped one, reboot/reset on
// a stopped one. This mirrors what real Proxmox rejects natively; never
// built it because no single-VM caller needed it, but the bulk
// 2 is the first caller that does.
func (fake Fake) Action(_ context.Context, node string, vmid int, action string) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := slices.IndexFunc(state.vms, func(v VM) bool { return v.VMID == vmid && v.Node == node })
	if idx < 0 {
		return ErrNotFound
	}

	status := state.vms[idx].Status
	if err := validateTransition(action, status); err != nil {
		return err
	}

	switch action {
	case actionStart, actionReboot, actionReset, actionResume:
		state.vms[idx].Status = VMRunning
		state.vms[idx].Uptime = fakeUptimeOnStart
	case actionStop, actionShutdown:
		state.vms[idx].Status = VMStopped
		state.vms[idx].Uptime = 0
	case actionPause:
		state.vms[idx].Status = VMPaused
	default:
		return ErrInvalidAction
	}

	state.record(FakeCall{Node: node, VMID: vmid, Action: action})

	return nil
}

// VMStatus implements VMStatusReader. The fake's in-memory state already tracks
// each VM's status and uptime; the lock comes from the injectable vmLocks map
// (empty by default = unlocked, matching Proxmox's /status/current).
func (fake Fake) VMStatus(_ context.Context, node string, vmid int) (VMLiveStatus, error) {
	state := fake.stateOrDefault()
	state.vmMu.RLock()
	defer state.vmMu.RUnlock()

	idx := slices.IndexFunc(state.vms, func(v VM) bool { return v.VMID == vmid && v.Node == node })
	if idx < 0 {
		return VMLiveStatus{}, ErrNotFound
	}

	return VMLiveStatus{
		Status: state.vms[idx].Status,
		Lock:   state.vmLocks[vmid],
		Uptime: state.vms[idx].Uptime,
	}, nil
}

// SetVMLock injects a Proxmox lock name on a VM for testing retry-on-lock
// and the lock field in VMLiveStatus. An empty lockName clears it.
func (fake Fake) SetVMLock(vmid int, lockName string) {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()
	if lockName == "" {
		delete(state.vmLocks, vmid)
	} else {
		state.vmLocks[vmid] = lockName
	}
}

// validateTransition rejects a power action that makes no sense for the VM's
// current status. Real Proxmox rejects these natively; the fake mirrors that
// so the bulk scenarios produce the same per-target error entries a real
// cluster would.
func validateTransition(action string, status VMStatus) error {
	switch action {
	case actionStart:
		if status == VMRunning {
			return fmt.Errorf("%w: vm already running", ErrInvalidStateTransition)
		}
	case actionStop, actionShutdown:
		if status == VMStopped {
			return fmt.Errorf("%w: vm already stopped", ErrInvalidStateTransition)
		}
	case actionReboot, actionReset:
		if status == VMStopped {
			return fmt.Errorf("%w: vm is not running", ErrInvalidStateTransition)
		}
	case actionPause:
		if status != VMRunning {
			return fmt.Errorf("%w: vm is not running", ErrInvalidStateTransition)
		}
	case actionResume:
		if status != VMPaused {
			return fmt.Errorf("%w: vm is not paused", ErrInvalidStateTransition)
		}
	}

	return nil
}

// Delete implements Writer - the VM and its disks are removed from the
// dataset. Irreversible: no soft-delete, no undo. A running VM is
// rejected with ErrVMRunning, mirroring real Proxmox (which returns HTTP 500
// "VM X is running - destroy failed"); callers must stop it first.
func (fake Fake) Delete(_ context.Context, node string, vmid int) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := slices.IndexFunc(state.vms, func(v VM) bool { return v.VMID == vmid && v.Node == node })
	if idx < 0 {
		return ErrNotFound
	}

	if state.vms[idx].Status == VMRunning {
		return ErrVMRunning
	}

	state.vms = slices.Delete(state.vms, idx, idx+1)

	state.record(FakeCall{Node: node, VMID: vmid, Action: actionDelete})

	return nil
}

// Patch implements Writer - name and/or description update. Empty arguments
// are ignored; the caller (vm.Patch) decides which fields to send.
func (fake Fake) Patch(_ context.Context, node string, vmid int, name, description string) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := slices.IndexFunc(state.vms, func(v VM) bool { return v.VMID == vmid && v.Node == node })
	if idx < 0 {
		return ErrNotFound
	}

	if name != "" {
		state.vms[idx].Name = name
	}

	if description != "" {
		state.vms[idx].Description = description
	}

	state.record(FakeCall{Node: node, VMID: vmid, Action: "patch", Name: name})

	return nil
}

// AddDisk implements Writer and appends a disk to the requested VM.
func (fake Fake) AddDisk(_ context.Context, node string, vmid int, bus, storage string, sizeGB int) (string, error) {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return "", ErrNotFound
	}

	busIndex := nextBusIndex(state.vms[idx].Disks, DiskBus(bus))
	key := fmt.Sprintf("%s%d", bus, busIndex)
	state.vms[idx].Disks = append(state.vms[idx].Disks, Disk{Key: key, Bus: DiskBus(bus), BusIndex: busIndex, Storage: storage, SizeGB: sizeGB})
	state.vms[idx].DiskTotal += int64(sizeGB) * 1024 * 1024 * 1024
	state.record(FakeCall{Node: node, VMID: vmid, Action: "add_disk", DiskKey: key, Bus: bus, Storage: storage, SizeGB: sizeGB})

	return key, nil
}

// ResizeDisk implements Writer and grows an existing disk in the fake dataset.
func (fake Fake) ResizeDisk(_ context.Context, node string, vmid int, diskKey string, sizeGB int) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	for diskIndex := range state.vms[idx].Disks {
		if state.vms[idx].Disks[diskIndex].Key != diskKey {
			continue
		}

		previous := state.vms[idx].Disks[diskIndex].SizeGB
		state.vms[idx].Disks[diskIndex].SizeGB = sizeGB
		state.vms[idx].DiskTotal += int64(sizeGB-previous) * 1024 * 1024 * 1024
		state.record(FakeCall{Node: node, VMID: vmid, Action: "resize_disk", DiskKey: diskKey, SizeGB: sizeGB})

		return nil
	}

	return ErrNotFound
}

// DeleteDisk implements Writer and removes a disk from the fake dataset.
func (fake Fake) DeleteDisk(_ context.Context, node string, vmid int, diskKey string) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	for diskIndex, disk := range state.vms[idx].Disks {
		if disk.Key != diskKey {
			continue
		}

		state.vms[idx].Disks = slices.Delete(state.vms[idx].Disks, diskIndex, diskIndex+1)
		state.vms[idx].DiskTotal -= int64(disk.SizeGB) * 1024 * 1024 * 1024

		state.record(FakeCall{Node: node, VMID: vmid, Action: "delete_disk", DiskKey: diskKey})

		return nil
	}

	return ErrNotFound
}

// SetCDROM implements Writer and changes the fake VM's CD-ROM state.
func (fake Fake) SetCDROM(_ context.Context, node string, vmid int, cdrom CDROMState) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	state.vms[idx].CDROM = cdrom

	state.record(FakeCall{Node: node, VMID: vmid, Action: "set_cdrom"})

	return nil
}

// SetBootOrder implements Writer and replaces the fake VM's boot order.
// An empty order clears the explicit boot config (Proxmox default behavior).
func (fake Fake) SetBootOrder(_ context.Context, node string, vmid int, order []string) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	state.vms[idx].BootOrder = append([]string(nil), order...)

	state.record(FakeCall{Node: node, VMID: vmid, Action: "set_boot_order", BootOrder: append([]string(nil), order...)})

	return nil
}

// UpdateNetwork implements Writer and replaces the fake VM's network interfaces.
func (fake Fake) UpdateNetwork(_ context.Context, node string, vmid int, interfaces []NetworkInterface) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	state.vms[idx].NetworkInterfaces = cloneNetworkInterfaces(interfaces)

	state.record(FakeCall{Node: node, VMID: vmid, Action: "update_network"})

	return nil
}

// UpdateHardware implements Writer and updates the fake VM's CPU, memory, and tags.
func (fake Fake) UpdateHardware(_ context.Context, node string, vmid, sockets, cores, memoryMB int, tags []string) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	state.vms[idx].Sockets = sockets
	state.vms[idx].Cores = cores
	state.vms[idx].CPUCores = sockets * cores
	state.vms[idx].MemoryTotal = int64(memoryMB) * 1024 * 1024

	state.vms[idx].Tags = append([]string(nil), tags...)

	state.record(FakeCall{Node: node, VMID: vmid, Action: "update_hardware", Sockets: sockets, Cores: cores, MemoryMB: memoryMB})

	return nil
}

// SetTags implements Writer and updates only the fake VM's tags, leaving
// hardware untouched. Used by the clone path when no hardware override was
// requested but the mandatory pvmss tag still needs to be stamped.
func (fake Fake) SetTags(_ context.Context, node string, vmid int, tags []string) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	state.vms[idx].Tags = append([]string(nil), tags...)

	state.record(FakeCall{Node: node, VMID: vmid, Action: "set_tags"})

	return nil
}

// EnableSerial implements Writer and flips the fake VM's HasSerial flag on.
func (fake Fake) EnableSerial(_ context.Context, node string, vmid int) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	state.vms[idx].HasSerial = true

	state.record(FakeCall{Node: node, VMID: vmid, Action: "enable_serial"})

	return nil
}

// ReadFirmwareConfig returns the live firmware config of a fake VM. The fake
// stores BIOS/Machine/EFIDisk/TPMState/SecureBoot on the VM struct at create
// time, so this just reads them back.
func (fake Fake) ReadFirmwareConfig(_ context.Context, node string, vmid int) (FirmwareConfig, error) {
	state := fake.stateOrDefault()
	state.vmMu.RLock()
	defer state.vmMu.RUnlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return FirmwareConfig{}, ErrNotFound
	}

	v := state.vms[idx]
	return FirmwareConfig{
		BIOS:       v.BIOS,
		Machine:    v.Machine,
		HasEFIDisk: v.EFIDisk,
		HasTPM:     v.TPMState,
		SecureBoot: v.SecureBoot,
	}, nil
}

// SetFakeSecureBoot marks one fake VM as carrying Secure Boot (efidisk0 with
// pre-enrolled-keys=1). PVMSS never creates such a VM any more, but ones made
// before Secure Boot was dropped - or by hand in Proxmox - still exist, so
// the SeaBIOS retrofit's refusal path keeps its coverage.
func SetFakeSecureBoot(vmid int) {
	state := defaultState()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	for i := range state.vms {
		if state.vms[i].VMID == vmid {
			state.vms[i].SecureBoot = true

			return
		}
	}
}

// RetrofitToSeaBIOS removes the UEFI firmware keys from a fake VM: clears
// BIOS, Machine, EFIDisk, TPMState, and SecureBoot. The caller
// must have already refused VMs with TPM state or Secure Boot and stopped
// the VM.
func (fake Fake) RetrofitToSeaBIOS(_ context.Context, node string, vmid int) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	state.vms[idx].BIOS = ""
	state.vms[idx].Machine = ""
	state.vms[idx].EFIDisk = false
	state.vms[idx].TPMState = false
	state.vms[idx].SecureBoot = false

	state.record(FakeCall{Node: node, VMID: vmid, Action: "retrofit_seabios"})

	return nil
}

func nextBusIndex(disks []Disk, bus DiskBus) int {
	index := 0
	for _, disk := range disks {
		if disk.Bus == bus && disk.BusIndex >= index {
			index = disk.BusIndex + 1
		}
	}

	return index
}

func cloneNetworkInterfaces(interfaces []NetworkInterface) []NetworkInterface {
	cloned := make([]NetworkInterface, len(interfaces))
	for i, iface := range interfaces {
		cloned[i] = iface
		cloned[i].IPAddresses = append([]string(nil), iface.IPAddresses...)
	}

	return cloned
}

// FakeCalls returns a copy of the recorded write calls since the last reset.
// Tests assert on this to prove a forbidden request reached the cluster zero
// times.
func FakeCalls() []FakeCall {
	return defaultState().calls()
}

// ClearFakeCalls empties the default fake's call log without touching its
// dataset: fixtures that set state up (a published document) start the test
// with a clean log.
func ClearFakeCalls() {
	state := defaultState()
	state.callMu.Lock()
	state.callLog = nil
	state.callMu.Unlock()
}

// FakeCallsFor returns the calls recorded for one VMID.
func FakeCallsFor(vmid int) []FakeCall {
	all := FakeCalls()

	out := make([]FakeCall, 0, len(all))
	for _, c := range all {
		if c.VMID == vmid {
			out = append(out, c)
		}
	}

	return out
}
