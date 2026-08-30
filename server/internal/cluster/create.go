package cluster

import "context"

// Creator is the contract for allocating and tracking a new VM (T06). It is
// deliberately separate from Client (reads) and Writer (mutations of existing
// VMs) — creation is the only operation that produces an asynchronous task a
// caller must then poll.
type Creator interface {
	// NextVMID allocates the next free VMID. It is the single allocation
	// point (FR-012), delegated to Proxmox's GET /cluster/nextid. That
	// endpoint returns the smallest free ID at call time without reserving
	// it, so two concurrent creations can receive the same integer — the
	// caller must handle ErrVMIDTaken by retrying with a fresh VMID
	// (US5/issue-05 D5c: max 3 attempts).
	NextVMID(ctx context.Context) (int, error)
	// CreateVM dispatches creation of spec and returns the task's UPID.
	// The VM materializes as the task progresses; a successful return means
	// accepted, not finished.
	CreateVM(ctx context.Context, spec VMSpec) (upid string, err error)
	// CloneVM dispatches a clone of an existing template VM and returns the
	// task's UPID (US2/issue-02). The clone is asynchronous like CreateVM;
	// the caller polls TaskStatus until completion before post-clone config.
	CloneVM(ctx context.Context, spec CloneSpec) (upid string, err error)
	// TaskStatus reads a task's current state live (FR-014). Returns
	// ErrNotFound for an unknown or expired UPID.
	TaskStatus(ctx context.Context, upid string) (TaskStatus, error)
}

// CloneSpec is the fully-resolved specification of a clone operation
// (US2/issue-02). SourceVMID is the Proxmox VMID of the template; SourceNode
// is the node the template lives on. NewVMID is the allocated VMID for the
// clone. Full selects full vs linked clone. Storage is the target storage for
// a full clone (empty = same as source). Pool is the tenancy anchor that owns
// the cloned VM (FR-004: always the actor's own, never client-supplied).
// TargetNode is reserved for future cross-node clone support (D2b: currently
// always empty, clone stays on SourceNode).
type CloneSpec struct {
	SourceVMID int
	SourceNode string
	NewVMID    int
	Name       string
	Full       bool
	Storage    string
	Pool       string
	DiskBus    string
	TargetNode string
}

// VMSpec is the fully-resolved specification of a VM to create (T06
// data-model.md). Every field is already validated and resolved server-side —
// the pool is the actor's own, never client-supplied (FR-004).
//
// BIOS, Machine, and TPM carry the UEFI/TPM 2.0 options (US6/issue-06 D6a).
// BIOS defaults to "seabios"; Machine defaults to "i440fx". When BIOS is
// "ovmf", the create path forces Machine to "q35" (pegaprox rule: UEFI
// requires q35) and emits efidisk0 (+ tpmstate0 when TPM is set).
type VMSpec struct {
	VMID             int
	Node             string
	Name             string
	Pool             string
	Tags             []string
	Sockets          int
	CPUCores         int
	MemoryMB         int
	Disk             DiskSpec
	Network          NetworkSpec
	ISO              *ISOSpec
	BIOS             string
	Machine          string
	TPM              bool
	StartAfterCreate bool
}

// DiskSpec is the VM's single initial disk (multi-disk is T07).
type DiskSpec struct {
	Storage string
	SizeGB  int
	Bus     string
}

// NICSpec is one network interface card to attach at creation. VLAN is the
// imposed isolation tag (US6/issue-06 D6b: per-cluster, admin-configured —
// never client-chosen). Firewall is always true on PVMSS-created VMs (D6a:
// the Proxmox per-VM firewall is armed by default, not user-exposed). MAC and
// RateMbps pass through to the encoder when set.
type NICSpec struct {
	Bridge   string
	Model    string
	VLAN     *int
	Firewall bool
	MAC      string
	RateMbps *int
}

// NetworkSpec is the VM's initial set of NICs (US2/D3a: multi-NIC). A
// NetworkSpec with zero entries produces no netN keys — the caller ensures at
// least one NIC before dispatch.
type NetworkSpec []NICSpec

// ISOSpec is an optional installation ISO attached at creation.
type ISOSpec struct {
	Storage string
	File    string
}

// TaskState is the lifecycle state of an asynchronous cluster task.
type TaskState string

// Task states reported by the cluster client during async operations.
const (
	TaskRunning TaskState = "running"
	TaskOK      TaskState = "ok"
	TaskError   TaskState = "error"
)

// TaskStatus is a live snapshot of an asynchronous task (FR-014).
type TaskStatus struct {
	UPID  string
	State TaskState
	// Log is human-readable progress, newest line last.
	Log []string
	// ExitMessage is present only when State == TaskError.
	ExitMessage string
	// Warnings carries the task's exitstatus when it succeeded with non-fatal
	// warnings (Proxmox "WARNINGS: N" — a success, not a failure). Present
	// only when State == TaskOK; empty otherwise. The tray surfaces it so a
	// success-with-warnings is distinguishable from a plain success.
	Warnings string
}
