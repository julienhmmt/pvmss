package cluster

import "context"

// Creator is the contract for allocating and tracking a new VM (T06). It is
// deliberately separate from Client (reads) and Writer (mutations of existing
// VMs) — creation is the only operation that produces an asynchronous task a
// caller must then poll.
type Creator interface {
	// NextVMID allocates the next free VMID. It is the single allocation
	// point (FR-012): distinct and monotonically increasing even under
	// concurrent calls, so two simultaneous creations never collide.
	NextVMID(ctx context.Context) (int, error)
	// CreateVM dispatches creation of spec and returns the task's UPID.
	// The VM materializes as the task progresses; a successful return means
	// accepted, not finished.
	CreateVM(ctx context.Context, spec VMSpec) (upid string, err error)
	// TaskStatus reads a task's current state live (FR-014). Returns
	// ErrNotFound for an unknown or expired UPID.
	TaskStatus(ctx context.Context, upid string) (TaskStatus, error)
}

// VMSpec is the fully-resolved specification of a VM to create (T06
// data-model.md). Every field is already validated and resolved server-side —
// the pool is the actor's own, never client-supplied (FR-004).
type VMSpec struct {
	VMID             int
	Node             string
	Name             string
	Pool             string
	Tags             []string
	CPUCores         int
	MemoryMB         int
	Disk             DiskSpec
	Network          NetworkSpec
	ISO              *ISOSpec
	StartAfterCreate bool
}

// DiskSpec is the VM's single initial disk (multi-disk is T07).
type DiskSpec struct {
	Storage string
	SizeGB  int
	Bus     string
}

// NetworkSpec is the VM's single initial NIC (multi-NIC is T07).
type NetworkSpec struct {
	Bridge string
	Model  string
}

// ISOSpec is an optional installation ISO attached at creation.
type ISOSpec struct {
	Storage string
	File    string
}

// TaskState is the lifecycle state of an asynchronous cluster task.
type TaskState string

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
}
