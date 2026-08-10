// Package cluster defines the contract for reading cluster data and its two
// production implementations: a real Proxmox client and a fake substitute.
package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"
)

// Sentinel errors so callers can distinguish failure modes without string matching.
var (
	ErrUnreachable    = errors.New("cluster unreachable")
	ErrNotFound       = errors.New("not found")
	ErrNotImplemented = errors.New("not implemented")
	ErrInvalidAction  = errors.New("invalid action")
)

// Client is the single contract for reading cluster data. Every implementation
// must behave identically from a caller's perspective (constitution XI).
type Client interface {
	Snapshot(ctx context.Context) (Snapshot, error)
	Authenticate(ctx context.Context, username, password string) (Identity, error)
	ChangePassword(ctx context.Context, username, oldPassword, newPassword string) error
}

// CloudInitReader reads per-VM cloud-init state and server-side snippet targets.
type CloudInitReader interface {
	GetCloudInitConfig(ctx context.Context, node string, vmid int) (CloudInitConfig, error)
	FindSnippetStorage(ctx context.Context, node string) (string, error)
}

// CloudInitIPMode identifies the network mode used by cloud-init.
type CloudInitIPMode string

const (
	// CloudInitIPModeDHCP requests automatic network configuration.
	CloudInitIPModeDHCP CloudInitIPMode = "dhcp"
	// CloudInitIPModeStatic requests explicit address and gateway values.
	CloudInitIPModeStatic CloudInitIPMode = "static"
)

// CloudInitConfig is the live structured cloud-init configuration of a VM.
type CloudInitConfig struct {
	User         string
	Password     string
	SSHKeys      []string
	IPMode       CloudInitIPMode
	IPAddress    string
	Gateway      string
	DNSServer    string
	SearchDomain string
}

// CloudInitUpdate is a partial cloud-init update. Nil fields remain unchanged.
type CloudInitUpdate struct {
	User         *string
	Password     *string
	SSHKeys      *[]string
	IPMode       *CloudInitIPMode
	IPAddress    *string
	Gateway      *string
	DNSServer    *string
	SearchDomain *string
}

// Writer is the contract for mutating a single VM. It is deliberately separate
// from Client (constitution IV: reads and writes are separated) — a handler
// that writes never reads the cluster directly, it reads the inventory
// projection and writes through this interface. The node is always
// server-resolved by Resolve() (FR-003); callers cannot supply it.
type Writer interface {
	Action(ctx context.Context, node string, vmid int, action string) error
	Delete(ctx context.Context, node string, vmid int) error
	Patch(ctx context.Context, node string, vmid int, name, description string) error
	AddDisk(ctx context.Context, node string, vmid int, bus, storage string, sizeGB int) (string, error)
	ResizeDisk(ctx context.Context, node string, vmid int, diskKey string, sizeGB int) error
	DeleteDisk(ctx context.Context, node string, vmid int, diskKey string) error
	SetCDROM(ctx context.Context, node string, vmid int, state CDROMState) error
	UpdateNetwork(ctx context.Context, node string, vmid int, interfaces []NetworkInterface) error
	UpdateHardware(ctx context.Context, node string, vmid, sockets, cores, memoryMB int, tags []string) error
	EnsureCloudInitDrive(ctx context.Context, node string, vmid int) error
	SetCloudInitConfig(ctx context.Context, node string, vmid int, config CloudInitConfig) error
	PushCloudInitSnippet(ctx context.Context, node, storage, filename string, vmid int, content string) error
}

// Snapshot is the complete result of one cluster read — all nodes, VMs, and
// storages at that instant (T03, AC02). It mirrors /cluster/resources' real
// shape: one call returns everything, instead of one call per entity type.
type Snapshot struct {
	Nodes    []Node
	VMs      []VM
	Storages []Storage
}

// Identity is the principal verified by the configured cluster identity provider.
type Identity struct {
	Username string
	// Pool is the tenancy anchor owning this user's VMs (PD00: one pool per
	// user). Empty for a cluster administrator with no personal pool.
	Pool    string
	IsAdmin bool
}

// NodeStatus is the operational state of a cluster node.
type NodeStatus string

// Node operational states reported by the cluster client.
const (
	NodeOnline  NodeStatus = "online"
	NodeOffline NodeStatus = "offline"
	NodeUnknown NodeStatus = "unknown"
)

// Node is a machine in the cluster.
type Node struct {
	Name         string
	Status       NodeStatus
	CPUCores     int
	CPUUsage     float64
	MemoryTotal  int64
	MemoryUsed   int64
	StorageTotal int64
	StorageUsed  int64
}

// VMStatus is the run state of a virtual machine.
type VMStatus string

// VM run states reported by the cluster client.
const (
	VMRunning VMStatus = "running"
	VMStopped VMStatus = "stopped"
	VMPaused  VMStatus = "paused"
)

// DiskBus identifies the Proxmox bus family used by a virtual disk.
type DiskBus string

// Disk bus constants identify the supported Proxmox bus families.
const (
	DiskBusVirtio DiskBus = "virtio" // DiskBusVirtio is the virtio bus.
	DiskBusSCSI   DiskBus = "scsi"   // DiskBusSCSI is the SCSI bus.
	DiskBusSATA   DiskBus = "sata"   // DiskBusSATA is the SATA bus.
	DiskBusIDE    DiskBus = "ide"    // DiskBusIDE is the IDE bus.
)

// Disk describes one virtual disk attached to a VM.
type Disk struct {
	Key      string  `json:"key"`
	Bus      DiskBus `json:"bus"`
	BusIndex int     `json:"busIndex"`
	Storage  string  `json:"storage"`
	SizeGB   int     `json:"sizeGB"`
	IsBoot   bool    `json:"isBoot"`
}

// CDROMState describes the fixed ide2 CD-ROM drive.
type CDROMState struct {
	State    string `json:"state"`
	ISOVolID string `json:"isoVolId,omitempty"`
}

// CD-ROM state constants describe the fixed drive lifecycle.
const (
	CDROMAbsent  = "absent"  // CDROMAbsent means no CD-ROM drive exists.
	CDROMEmpty   = "empty"   // CDROMEmpty means the drive has no media.
	CDROMMounted = "mounted" // CDROMMounted means approved media is attached.
)

// NetworkInterface describes one VM network interface and guest-agent data.
type NetworkInterface struct {
	Index       int      `json:"index"`
	Bridge      string   `json:"bridge"`
	Model       string   `json:"model"`
	MAC         string   `json:"mac"`
	VLAN        *int     `json:"vlan"`
	RateMbps    *int     `json:"rateMbps"`
	IPAddresses []string `json:"ipAddresses"`
}

// MarshalJSON ensures IPAddresses always encodes as [] rather than null when
// unset, matching the frontend's non-nullable string[] contract.
func (n NetworkInterface) MarshalJSON() ([]byte, error) {
	type alias NetworkInterface

	dto := alias(n)

	if dto.IPAddresses == nil {
		dto.IPAddresses = []string{}
	}

	return json.Marshal(dto)
}

// VM is a guest belonging to a node. Carried by the fake dataset from T01 so
// later tranches have data to work with, but not surfaced by any endpoint
// until T04.
type VM struct {
	VMID              int
	Name              string
	Node              string
	Status            VMStatus
	Pool              string
	Tags              []string
	CPUCores          int
	Sockets           int
	Cores             int
	MemoryTotal       int64
	BootOrder         []string
	Disks             []Disk
	CDROM             CDROMState
	NetworkInterfaces []NetworkInterface
	// DiskTotal is the guest's total disk size in bytes (V15 detail stat card).
	DiskTotal int64
	// Uptime is how long the guest has been running. Zero when Status != running
	// (contracts/vm-detail-actions.md: uptimeSeconds absent when not running).
	Uptime time.Duration
	// Description is the free-text note editable via PATCH (V17). Empty by
	// default in the fake dataset.
	Description string
}

// Storage is a storage backend attached to a node.
type Storage struct {
	Name            string
	Node            string
	Type            string
	Total           int64
	Used            int64
	SupportsVMState bool
}

// VMSnapshot is a live snapshot entry returned by a cluster for one VM.
type VMSnapshot struct {
	Name        string
	Description string
	CreatedAt   time.Time
	VMState     bool
}

// SnapshotReader reads live snapshots for a resolved VM.
type SnapshotReader interface {
	ListSnapshots(ctx context.Context, node string, vmid int) ([]VMSnapshot, error)
}

// SnapshotWriter dispatches asynchronous snapshot operations for a resolved VM.
type SnapshotWriter interface {
	CreateSnapshot(ctx context.Context, node string, vmid int, name, description string, vmstate bool) (string, error)
	RollbackSnapshot(ctx context.Context, node string, vmid int, name string) (string, error)
	DeleteSnapshot(ctx context.Context, node string, vmid int, name string) (string, error)
}

// VNCProxyTicket is the Proxmox-side VNC ticket, port, and node returned by
// GetVNCTicket. The real client populates all three from Proxmox's vncproxy
// response; the fake client returns a fixed fabricated pair. Neither value
// ever reaches the browser — only the opaque ConsoleTicketStore token does
// (FR-002). The relay reads them back from the ticket at upgrade time.
type VNCProxyTicket struct {
	Ticket string
	Port   int
	Node   string
}

// ConsoleRelay is the contract for relaying an already-upgraded WebSocket
// connection to a VM's VNC server (T10). It blocks for the lifetime of the
// console session, copying bytes both ways between the browser-side peer and
// whatever the implementation connects to (Proxmox's own VNC WebSocket for the
// real client, an in-process RFB 3.8 handshake for the fake client). It
// returns when either side closes (FR-008, FR-009).
//
// Kept separate from Client (constitution IV: reads and writes are separated)
// — a console relay is neither a read nor a write in the Index sense, it is a
// long-lived byte stream, and giving it its own interface keeps the Client
// surface focused. Both Fake and Proxmox satisfy it.
type ConsoleRelay interface {
	GetVNCTicket(ctx context.Context, clusterName string, vmid int, node string) (VNCProxyTicket, error)
	RelayConsole(ctx context.Context, clusterName string, vmid int, proxy VNCProxyTicket, peer io.ReadWriteCloser) error
}

// Pool is a tenancy anchor — one pool maps to one user (PD00).
type Pool struct {
	Name    string
	Comment string
}
