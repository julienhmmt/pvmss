//nolint:wsl_v5 // fake state methods keep mutation and locking adjacent
package cluster

import (
	"slices"
	"sync"
)

// fakeState is the mutable dataset of one Fake instance. NewFake allocates a
// private state so two clusters (or two tests) cannot corrupt each other.
// A zero-value Fake{} still works: methods fall back to the process-wide
// default state, which is what ResetFake / FakeCalls inspect.
type fakeState struct {
	vmMu     sync.RWMutex
	callMu   sync.Mutex
	createMu sync.Mutex
	identMu  sync.RWMutex
	pushMu   sync.RWMutex
	sshMu    sync.RWMutex

	vms           []VM
	nodes         []Node
	storages      []Storage
	pools         []Pool
	callLog       []FakeCall
	roleState     map[string][]string
	roleCallLog   []RoleCall
	acls          []ACLEntry
	errDeleteUser error
	pushErr       error
	sshErr        error
	// createErr, when set, is returned by the next CreateVM call instead of
	// dispatching (US5/issue-05: tests inject cluster.ErrVMIDTaken to exercise
	// the retry loop, or a generic error to exercise the failure path).
	createErr error
	// createErrCount limits how many consecutive CreateVM calls return
	// createErr before clearing it (0 = unlimited). Used to simulate a
	// transient collision that succeeds on retry.
	createErrCount int
	// taskErr, when set, makes the next registered task report TaskError on
	// its first TaskStatus poll instead of TaskRunning (US5/issue-05: tests
	// inject a task error to exercise the rollback path).
	taskErr            string
	cloudInitConfigs   map[fakeCloudInitKey]CloudInitConfig
	cloudInitDrives    map[fakeCloudInitKey]bool
	snapshots          map[fakeSnapshotKey][]VMSnapshot
	identities         map[string]fakeIdentity
	nextVMID           int
	nextSnapshotTaskID uint64
	tasks              map[string]*fakeTask
	// vmLocks maps vmid → Proxmox lock name ("backup", "migrate", ...). Empty
	// or absent means unlocked. Tests inject a lock to exercise retry-on-lock
	// (ticket 08) and the lock field in VMLiveStatus (ticket 01b).
	vmLocks map[int]string
}

var (
	defaultFakeState     *fakeState
	defaultFakeStateOnce sync.Once
)

func defaultState() *fakeState {
	defaultFakeStateOnce.Do(func() {
		defaultFakeState = newFakeState("")
	})
	return defaultFakeState
}

// NewFake returns a Fake with its own dataset. Mutations on one instance are
// invisible to every other instance, including the zero-value Fake{} that
// tests still construct for the default cluster.
func NewFake(clusterName string) Fake {
	return Fake{ClusterName: clusterName, state: newFakeState(clusterName)}
}

func (fake Fake) stateOrDefault() *fakeState {
	if fake.state != nil {
		return fake.state
	}
	return defaultState()
}

func newFakeState(clusterName string) *fakeState {
	state := &fakeState{
		cloudInitConfigs: originalFakeCloudInitConfigs(),
		cloudInitDrives:  make(map[fakeCloudInitKey]bool),
		snapshots:        make(map[fakeSnapshotKey][]VMSnapshot),
		identities:       originalFakeIdentities(),
		roleState:        make(map[string][]string),
		vmLocks:          make(map[int]string),
	}
	if clusterName == "secondary" {
		state.nodes = slices.Clone(secondaryNodes)
		state.storages = slices.Clone(secondaryStorages)
		state.vms = secondaryVMs()
		state.pools = originalFakePools()
		return state
	}
	state.nodes = slices.Clone(fakeNodes)
	state.storages = slices.Clone(fakeStorages)
	state.vms = originalFakeVMs()
	state.pools = originalFakePools()
	return state
}

func (s *fakeState) reset(clusterName string) {
	s.vmMu.Lock()
	defer s.vmMu.Unlock()
	s.callMu.Lock()
	defer s.callMu.Unlock()
	s.createMu.Lock()
	defer s.createMu.Unlock()

	fresh := newFakeState(clusterName)
	s.vms = fresh.vms
	s.nodes = fresh.nodes
	s.storages = fresh.storages
	s.pools = fresh.pools
	s.cloudInitConfigs = fresh.cloudInitConfigs
	s.cloudInitDrives = fresh.cloudInitDrives
	s.snapshots = fresh.snapshots
	s.roleState = fresh.roleState
	s.roleCallLog = nil
	s.acls = nil
	s.errDeleteUser = nil
	s.callLog = nil
	s.nextVMID = 0
	s.nextSnapshotTaskID = 0
	s.tasks = nil
	s.createErr = nil
	s.createErrCount = 0
	s.taskErr = ""
	s.vmLocks = make(map[int]string)

	s.identMu.Lock()
	s.identities = fresh.identities
	s.identMu.Unlock()

	s.pushMu.Lock()
	s.pushErr = nil
	s.pushMu.Unlock()
}

func (s *fakeState) record(call FakeCall) {
	s.callMu.Lock()
	s.callLog = append(s.callLog, call)
	s.callMu.Unlock()
}

func (s *fakeState) findVM(node string, vmid int) int {
	return slices.IndexFunc(s.vms, func(v VM) bool { return v.VMID == vmid && v.Node == node })
}

func (s *fakeState) vmidFloor() int {
	floor := 100
	for _, vm := range s.vms {
		if vm.VMID >= floor {
			floor = vm.VMID + 1
		}
	}
	return floor
}

func (s *fakeState) calls() []FakeCall {
	s.callMu.Lock()
	defer s.callMu.Unlock()
	return append([]FakeCall(nil), s.callLog...)
}

func (s *fakeState) roleCalls() []RoleCall {
	s.vmMu.RLock()
	defer s.vmMu.RUnlock()
	calls := make([]RoleCall, len(s.roleCallLog))
	for index, call := range s.roleCallLog {
		calls[index] = RoleCall{Privileges: slices.Clone(call.Privileges), At: call.At}
	}
	return calls
}
