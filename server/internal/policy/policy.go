// Package policy provides the persistent gabarit, quota, and node-capacité
// values used by VM creation and mutation guards.
package policy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"slices"
)

var (
	// ErrQuotaExceeded reports a user at or above the configured quota.
	ErrQuotaExceeded = errors.New("quota exceeded")
	// ErrGabaritExceeded reports a VM value above the configured gabarit.
	ErrGabaritExceeded = errors.New("gabarit exceeded")
	// ErrNodeCapacityExceeded reports aggregate node usage above capacité.
	ErrNodeCapacityExceeded = errors.New("node capacity exceeded")
	// ErrBelowCurrentUsage reports a capacité below live usage.
	ErrBelowCurrentUsage = errors.New("node capacity below current usage")
	// ErrAboveNodeCapacity reports a capacité above physical node resources.
	ErrAboveNodeCapacity = errors.New("node capacity above physical capacity")
	// ErrInvalidPolicy reports malformed policy values.
	ErrInvalidPolicy = errors.New("invalid policy")
	// ErrUnavailable reports a VM domain without its required policy service.
	ErrUnavailable = errors.New("policy service unavailable")
)

const (
	defaultMaxSockets      = 4
	defaultMaxCores        = 8
	defaultMaxMemoryMB     = 16384
	defaultMaxDiskPerVMGB  = 500
	defaultMaxNetworkCards = 4
	defaultMaxSnapshots    = 5
	defaultMaxVMPerUser    = -1
)

// Dimension identifiers used in capacity errors and guards.
const (
	dimensionVCPUs = "vcpus"
	dimensionVCPU  = "vcpu"
	dimensionVMs   = "vms"
	dimensionRAM   = "ram"
)

// Gabarit is the administrator-editable size ceiling for one VM.
type Gabarit struct {
	MaxSockets      int
	MaxCores        int
	MaxMemoryMB     int
	MaxDiskPerVMGB  int
	MaxNetworkCards int
	MaxSnapshots    int
	AllowCustomYaml bool
}

// Quota is the current VM count and per-user allowance.
type Quota struct {
	Used    int
	Allowed int
}

// Capacity is a node's configured aggregate capacité, live usage, and physical
// CPU/RAM facts. MaxDiskGB is intentionally displayed but not enforced.
type Capacity struct {
	Node          string
	MaxVMs        int
	MaxVCPUs      int
	MaxRAMGB      int
	MaxDiskGB     int
	UsedVMs       int
	UsedVCPUs     int
	UsedRAMGB     int
	PhysicalVCPUs int
	PhysicalRAMGB int
}

// Policy owns persistence and the immutable inventory projection used to
// calculate current usage. The cluster client is only used for admin writes'
// physical-node validation.
type Policy struct {
	store      *store.Store
	projection *inventory.Projection
	client     cluster.Client
}

// New creates a policy service backed by the store and inventory projection.
func New(st *store.Store, projection *inventory.Projection, client cluster.Client) *Policy {
	return &Policy{store: st, projection: projection, client: client}
}

// DefaultGabarit returns the compatibility values shipped before T12.
func DefaultGabarit() Gabarit {
	return Gabarit{
		MaxSockets: defaultMaxSockets, MaxCores: defaultMaxCores, MaxMemoryMB: defaultMaxMemoryMB,
		MaxDiskPerVMGB: defaultMaxDiskPerVMGB, MaxNetworkCards: defaultMaxNetworkCards,
		MaxSnapshots: defaultMaxSnapshots, AllowCustomYaml: true,
	}
}

// Gabarit reads the current cluster gabarit from SQLite.
func (service *Policy) Gabarit(ctx context.Context, clusterName string) (Gabarit, error) {
	row, err := service.store.PolicyRow(ctx, clusterName)
	if err != nil {
		return Gabarit{}, err
	}
	return Gabarit{
		MaxSockets: row.MaxSockets, MaxCores: row.MaxCores, MaxMemoryMB: row.MaxMemoryMB,
		MaxDiskPerVMGB: row.MaxDiskPerVMGB, MaxNetworkCards: row.MaxNetworkCards,
		MaxSnapshots: row.MaxSnapshots, AllowCustomYaml: row.AllowCustomYaml,
	}, nil
}

// Quota reads the cluster allowance and calculates the actor's current pool
// usage from the immutable inventory projection. Administrators have no pool
// and therefore always receive the unlimited allowance.
func (service *Policy) Quota(ctx context.Context, clusterName string, actor auth.Identity) (Quota, error) {
	row, err := service.store.PolicyRow(ctx, clusterName)
	if err != nil {
		return Quota{}, err
	}
	if actor.IsAdmin {
		return Quota{Allowed: defaultMaxVMPerUser}, nil
	}
	return Quota{Used: service.poolVMCount(actor.Pool), Allowed: row.MaxVMPerUser}, nil
}

// NodeCapacity reads one node's configured capacité and live usage. Only VMs
// carrying the mandatory pvmss tag count toward the aggregate.
func (service *Policy) NodeCapacity(ctx context.Context, clusterName, node string) (Capacity, error) {
	row, err := service.store.NodePolicyRow(ctx, clusterName, node)
	if errors.Is(err, sql.ErrNoRows) {
		row = store.NodePolicyRow{Cluster: clusterName, Node: node}
	} else if err != nil {
		return Capacity{}, fmt.Errorf("read node capacity: %w", err)
	}
	capacity := Capacity{Node: node, MaxVMs: row.MaxVMs, MaxVCPUs: row.MaxVCPUs, MaxRAMGB: row.MaxRAMGB, MaxDiskGB: row.MaxDiskGB}
	if service.projection == nil || service.projection.Load() == nil {
		return capacity, nil
	}
	index := service.projection.Load()
	var usedRAMBytes int64
	for _, machine := range index.ByNode[node] {
		if !slices.Contains(machine.Tags, "pvmss") {
			continue
		}
		capacity.UsedVMs++
		capacity.UsedVCPUs += vmVCPUs(machine)
		usedRAMBytes += machine.MemoryTotal
	}
	capacity.UsedRAMGB = int(usedRAMBytes / bytesPerGB)
	for _, machine := range index.Nodes {
		if machine.Name != node {
			continue
		}
		capacity.PhysicalVCPUs = machine.CPUCores
		capacity.PhysicalRAMGB = int(machine.MemoryTotal / bytesPerGB)
		break
	}
	return capacity, nil
}

const bytesPerGB int64 = 1024 * 1024 * 1024

func (service *Policy) poolVMCount(pool string) int {
	if service.projection == nil || service.projection.Load() == nil {
		return 0
	}
	return len(service.projection.Load().ByPool[pool])
}

func vmVCPUs(machine cluster.VM) int {
	if machine.Sockets > 0 && machine.Cores > 0 {
		return machine.Sockets * machine.Cores
	}
	return machine.CPUCores
}
