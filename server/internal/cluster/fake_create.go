package cluster

import (
	"context"
	"fmt"
)

// Fake implementation of Creator (T06). Task completion is poll-count-based,
// not wall-clock (plan.md research decisions): a task reports running for its
// first two TaskStatus queries and ok from the third, so go test -race stays
// instant and deterministic while the browser demo still shows a visible
// in-progress state (the frontend's poll interval paces observation).

// fakeTask tracks one in-flight creation's poll count.
type fakeTask struct {
	upid       string
	polls      int
	onComplete func()
	log        []string
}

// NextVMID implements Creator. The counter starts above the dataset's highest
// VMID and only ever increases, so concurrent creations get distinct ids even
// before their CreateVM calls land (edge case: two tabs submitting at once).
func (fake Fake) NextVMID(_ context.Context) (int, error) {
	state := fake.stateOrDefault()
	state.createMu.Lock()
	defer state.createMu.Unlock()

	state.vmMu.RLock()

	floor := state.vmidFloor()

	state.vmMu.RUnlock()

	if state.nextVMID < floor {
		state.nextVMID = floor
	}

	vmid := state.nextVMID
	state.nextVMID++

	return vmid, nil
}

// CreateVM implements Creator: the VM enters the fake's mutable dataset
// immediately (running when StartAfterCreate is set — FR-022 folds the start
// into this same task) and a poll-counted task is registered under the
// returned UPID.
func (fake Fake) CreateVM(_ context.Context, spec VMSpec) (string, error) {
	state := fake.stateOrDefault()

	// US5/issue-05: inject a CreateVM error when configured by the test.
	state.createMu.Lock()
	if state.createErr != nil {
		err := state.createErr
		if state.createErrCount > 0 {
			state.createErrCount--
			if state.createErrCount == 0 {
				state.createErr = nil
			}
		}
		state.createMu.Unlock()

		return "", err
	}
	state.createMu.Unlock()

	state.vmMu.Lock()

	status := VMStopped
	if spec.StartAfterCreate {
		status = VMRunning
	}

	var diskTotal int64
	if spec.Disk.SizeGB > 0 {
		diskTotal = int64(spec.Disk.SizeGB) * 1024 * 1024 * 1024
	}

	nics := make([]NetworkInterface, 0, len(spec.Network))
	for _, nic := range spec.Network {
		nics = append(nics, NetworkInterface{
			Model: nic.Model, Bridge: nic.Bridge, VLAN: nic.VLAN,
			Firewall: nic.Firewall, MAC: nic.MAC, RateMbps: nic.RateMbps,
		})
	}

	state.vms = append(state.vms, VM{
		VMID:              spec.VMID,
		Name:              spec.Name,
		Node:              spec.Node,
		Status:            status,
		Pool:              spec.Pool,
		Tags:              append([]string(nil), spec.Tags...),
		Sockets:           spec.Sockets,
		Cores:             spec.CPUCores,
		CPUCores:          spec.Sockets * spec.CPUCores,
		MemoryTotal:       int64(spec.MemoryMB) * 1024 * 1024,
		DiskTotal:         diskTotal,
		NetworkInterfaces: nics,
		CDROM:             cdromFromSpec(spec),
		BootOrder:         bootOrderFromSpec(spec),
	})

	// The real create path always sends agent=1 (proxmox_create.go), so a
	// fake-created VM carries an enabled guest agent even before any
	// cloud-init config is written — the password pre-flight reads it from
	// the config (ticket 05).
	if state.cloudInitConfigs == nil {
		state.cloudInitConfigs = make(map[fakeCloudInitKey]CloudInitConfig)
	}

	state.cloudInitConfigs[fakeCloudInitKey{node: spec.Node, vmid: spec.VMID}] = CloudInitConfig{Agent: true, IPMode: CloudInitIPModeDHCP}

	state.vmMu.Unlock()

	upid := fmt.Sprintf("UPID:%s:%08X:%08X:%08X:qmcreate:%d:pvmss@pve:", spec.Node, spec.VMID, 0x10000000+spec.VMID, 0x20000000+spec.VMID, spec.VMID)

	state.createMu.Lock()
	if state.tasks == nil {
		state.tasks = make(map[string]*fakeTask)
	}

	state.tasks[upid] = &fakeTask{upid: upid, log: []string{"allocating disk...", "starting qmcreate..."}}
	state.createMu.Unlock()

	state.record(FakeCall{Node: spec.Node, VMID: spec.VMID, Action: "create", Name: spec.Name})

	return upid, nil
}

// TaskStatus implements Creator. Poll-count-based: running for a UPID's first
// two queries, ok from the third onward (SC-006 — deterministic, no sleeps).
// US5/issue-05: when SetFakeTaskError was called, the next registered task
// reports TaskError on its first poll instead of running.
func (fake Fake) TaskStatus(_ context.Context, upid string) (TaskStatus, error) {
	state := fake.stateOrDefault()
	state.createMu.Lock()

	task, ok := state.tasks[upid]
	if !ok {
		state.createMu.Unlock()
		return TaskStatus{}, ErrNotFound
	}

	// US5/issue-05: inject a task error on the first poll when configured.
	if state.taskErr != "" && task.polls == 0 {
		task.polls++

		exitMsg := state.taskErr
		state.taskErr = ""

		log := append([]string(nil), task.log...)
		log = append(log, "TASK ERROR: "+exitMsg)

		state.createMu.Unlock()

		return TaskStatus{UPID: upid, State: TaskError, ExitMessage: exitMsg, Log: log}, nil
	}

	task.polls++

	log := append([]string(nil), task.log...)
	if task.polls < 3 {
		state.createMu.Unlock()
		return TaskStatus{UPID: upid, State: TaskRunning, Log: log}, nil
	}

	onComplete := task.onComplete
	task.onComplete = nil
	state.createMu.Unlock()

	if onComplete != nil {
		onComplete()
	}

	log = append(log, "TASK OK")

	return TaskStatus{UPID: upid, State: TaskOK, Log: log}, nil
}

// CloneVM implements Creator: registers a poll-counted clone task under the
// returned UPID (US2/issue-02). The cloned VM enters the fake's mutable dataset
// on the third poll (TaskOK), via the task's onComplete callback — mirroring
// how a real Proxmox clone materializes the VM only after the task finishes.
// The fake VM inherits a template-sized disk so post-clone ResizeDisk can find
// it; the caller applies the actual resize via Writer.ResizeDisk after
// waitCreateTask returns.
func (fake Fake) CloneVM(_ context.Context, spec CloneSpec) (string, error) {
	state := fake.stateOrDefault()

	upid := fmt.Sprintf("UPID:%s:%08X:%08X:%08X:qmclone:%d:pvmss@pve:", spec.SourceNode, spec.NewVMID, 0x10000000+spec.NewVMID, 0x20000000+spec.NewVMID, spec.NewVMID)

	// The cloned VM's disk: a primary disk on the clone's storage (or the
	// source node's local-lvm if no target storage was specified). The bus
	// family comes from the template (spec.DiskBus) — the clone inherits it.
	// The size is a small default (8 GB) — the caller resizes via
	// Writer.ResizeDisk after the task completes.
	diskStorage := spec.Storage
	if diskStorage == "" {
		diskStorage = "local-lvm"
	}

	diskBus := DiskBus(spec.DiskBus)
	if diskBus == "" {
		diskBus = DiskBusSCSI
	}

	diskKey := string(diskBus) + "0"

	state.createMu.Lock()
	if state.tasks == nil {
		state.tasks = make(map[string]*fakeTask)
	}

	state.tasks[upid] = &fakeTask{
		upid: upid,
		log:  []string{"cloning disk...", "starting qmclone..."},
		onComplete: func() {
			state.vmMu.Lock()
			state.vms = append(state.vms, VM{
				VMID:    spec.NewVMID,
				Name:    spec.Name,
				Node:    spec.SourceNode,
				Pool:    spec.Pool,
				Status:  VMStopped,
				Tags:    []string{"pvmss"},
				Sockets: 1,
				Disks: []Disk{{
					Key:      diskKey,
					Bus:      diskBus,
					BusIndex: 0,
					Storage:  diskStorage,
					SizeGB:   8,
				}},
				DiskTotal: 8 * 1024 * 1024 * 1024,
			})
			state.vmMu.Unlock()
		},
	}
	state.createMu.Unlock()

	state.record(FakeCall{Node: spec.SourceNode, VMID: spec.NewVMID, Action: "clone", Name: spec.Name, Pool: spec.Pool, Full: spec.Full, Storage: spec.Storage})

	return upid, nil
}

// cdromFromSpec builds the CD-ROM state a real Proxmox create would leave:
// mounted with the ISO volume ID when an ISO is provided, absent otherwise.
func cdromFromSpec(spec VMSpec) CDROMState {
	if spec.ISO == nil {
		return CDROMState{State: CDROMAbsent}
	}

	return CDROMState{State: CDROMMounted, ISOVolID: spec.ISO.Storage + ":iso/" + spec.ISO.File}
}

// bootOrderFromSpec mirrors proxmox_create.go's setBootOrderForm: CD-ROM first
// when an ISO is present (so the VM boots from the installer), then disk.
func bootOrderFromSpec(spec VMSpec) []string {
	var order []string

	if spec.ISO != nil {
		order = append(order, cdromDiskKey)
	}

	if spec.Disk.Storage != "" {
		order = append(order, spec.Disk.Bus+"0")
	}

	return order
}
