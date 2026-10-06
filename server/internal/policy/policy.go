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
	// ErrQuotaExceeded reports a pool at or above the configured quota.
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
	// ErrConcurrentUpdate reports a policy row still changing after the
	// optimistic-concurrency retries of UpdatePolicy.
	ErrConcurrentUpdate = errors.New("policy updated concurrently")
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
	dimensionDisk  = "disk"
)

// Gabarit is the administrator-editable size ceiling for one VM. A zero
// field imposes no cap - the same convention as the node capacités.
// IsolationVLANTag is the per-cluster imposed VLAN:
// 0 means no tag imposed; a positive value is stamped on every created NIC.
type Gabarit struct {
	MaxSockets       int
	MaxCores         int
	MaxMemoryMB      int
	MaxDiskPerVMGB   int
	MaxNetworkCards  int
	MaxSnapshots     int
	IsolationVLANTag int
}

// Quota is the actor's pool VM count and the pool allowance.
type Quota struct {
	Used    int
	Allowed int
}

// Capacity is a node's configured aggregate capacité, live usage, and physical
// CPU/RAM/disk facts. UsedDiskGB is the provisioned disk total from the
// inventory projection, parallel to UsedRAMGB. The Node* fields carry the
// live, all-VMs load of the node itself: Used* counts only pvmss-tagged VMs,
// so an uncapped but heavily loaded node must not read as empty.
type Capacity struct {
	Node          string
	MaxVMs        int
	MaxVCPUs      int
	MaxRAMGB      int
	MaxDiskGB     int
	UsedVMs       int
	UsedVCPUs     int
	UsedRAMGB     int
	UsedDiskGB    int
	PhysicalVCPUs int
	PhysicalRAMGB int

	Status         cluster.NodeStatus
	CPUUsage       float64
	MemoryUsedGB   int
	StorageUsedGB  int
	StorageTotalGB int
	TotalVMs       int

	// Approved reports the node's catalog_nodes.enabled state: an unapproved
	// node is excluded from placement entirely, so its caps never apply.
	Approved bool
}

// CapacityDelta is the incremental VM footprint a capacity check is asked to
// absorb. Sockets, Cores, MemoryMB, and DiskGB are the new or resized VM's
// contribution; ExcludeVMID removes an existing VM's current contribution
// before applying the delta (used by the resize path).
type CapacityDelta struct {
	Sockets     int
	Cores       int
	MemoryMB    int
	DiskGB      int
	ExcludeVMID int
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

// DefaultGabarit returns the compatibility values shipped.
func DefaultGabarit() Gabarit {
	return Gabarit{
		MaxSockets: defaultMaxSockets, MaxCores: defaultMaxCores, MaxMemoryMB: defaultMaxMemoryMB,
		MaxDiskPerVMGB: defaultMaxDiskPerVMGB, MaxNetworkCards: defaultMaxNetworkCards,
		MaxSnapshots: defaultMaxSnapshots,
	}
}

// Settings is the stored administrator-edited policy for a cluster: the
// gabarit plus the pool quota allowance, without usage data.
type Settings struct {
	Gabarit Gabarit
	// Allowed is the pool VM allowance. -1 means unlimited.
	Allowed int
}

// Settings reads the cluster's stored policy row in one query, falling back
// to the shipped defaults when the cluster has none. Unlike Quota it reports
// the stored allowance verbatim, with no identity-dependent adjustment.
func (service *Policy) Settings(ctx context.Context, clusterName string) (Settings, error) {
	row, err := service.policyRowOrDefault(ctx, clusterName)
	if err != nil {
		return Settings{}, err
	}

	return Settings{Gabarit: gabaritFromRow(row), Allowed: row.MaxVMPerUser}, nil
}

// Gabarit reads the current cluster gabarit from SQLite, falling back to the
// shipped defaults when the cluster has no stored row.
func (service *Policy) Gabarit(ctx context.Context, clusterName string) (Gabarit, error) {
	row, err := service.policyRowOrDefault(ctx, clusterName)
	if err != nil {
		return Gabarit{}, err
	}

	return gabaritFromRow(row), nil
}

func gabaritFromRow(row store.PolicyRow) Gabarit {
	return Gabarit{
		MaxSockets: row.MaxSockets, MaxCores: row.MaxCores, MaxMemoryMB: row.MaxMemoryMB,
		MaxDiskPerVMGB: row.MaxDiskPerVMGB, MaxNetworkCards: row.MaxNetworkCards,
		MaxSnapshots:     row.MaxSnapshots,
		IsolationVLANTag: row.IsolationVLANTag,
	}
}

// policyRowOrDefault reads the persisted policy row, substituting the shipped
// defaults when the cluster has none yet: clusters created before
// CreateCluster seeded a row, and pseudo-cluster names such as the
// all-clusters view, which has no single cluster and must not fail the whole
// request with a 500.
func (service *Policy) policyRowOrDefault(ctx context.Context, clusterName string) (store.PolicyRow, error) {
	row, err := service.store.PolicyRow(ctx, clusterName)
	if errors.Is(err, sql.ErrNoRows) {
		return defaultPolicyRow(clusterName), nil
	}

	return row, err
}

// defaultPolicyRow is the shipped policy row for a cluster that has none yet.
func defaultPolicyRow(clusterName string) store.PolicyRow {
	defaults := DefaultGabarit()

	return store.PolicyRow{
		Cluster:          clusterName,
		MaxSockets:       defaults.MaxSockets,
		MaxCores:         defaults.MaxCores,
		MaxMemoryMB:      defaults.MaxMemoryMB,
		MaxDiskPerVMGB:   defaults.MaxDiskPerVMGB,
		MaxNetworkCards:  defaults.MaxNetworkCards,
		MaxSnapshots:     defaults.MaxSnapshots,
		MaxVMPerUser:     defaultMaxVMPerUser,
		IsolationVLANTag: defaults.IsolationVLANTag,
	}
}

// policyRowFrom flattens Settings back into its persisted row form.
func policyRowFrom(clusterName string, settings Settings) store.PolicyRow {
	return store.PolicyRow{
		Cluster:          clusterName,
		MaxSockets:       settings.Gabarit.MaxSockets,
		MaxCores:         settings.Gabarit.MaxCores,
		MaxMemoryMB:      settings.Gabarit.MaxMemoryMB,
		MaxDiskPerVMGB:   settings.Gabarit.MaxDiskPerVMGB,
		MaxNetworkCards:  settings.Gabarit.MaxNetworkCards,
		MaxSnapshots:     settings.Gabarit.MaxSnapshots,
		MaxVMPerUser:     settings.Allowed,
		IsolationVLANTag: settings.Gabarit.IsolationVLANTag,
	}
}

// Quota reads the cluster allowance and calculates the actor's pool usage
// from the immutable inventory projection. The allowance applies to the
// whole pool: accounts sharing one share its budget. Administrators have no
// pool and therefore always receive the unlimited allowance.
func (service *Policy) Quota(ctx context.Context, clusterName string, actor auth.Identity) (Quota, error) {
	row, err := service.policyRowOrDefault(ctx, clusterName)
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

	usage := pvmssUsage(index, node)
	capacity.TotalVMs = len(index.ByNode[node])
	capacity.UsedVMs = usage.vms
	capacity.UsedVCPUs = usage.vcpus
	capacity.UsedRAMGB = int(usage.ramBytes / bytesPerGB)
	capacity.UsedDiskGB = int(usage.diskBytes / bytesPerGB)

	for _, machine := range index.Nodes {
		if machine.Name != node {
			continue
		}

		capacity.PhysicalVCPUs = machine.CPUCores
		capacity.PhysicalRAMGB = int(machine.MemoryTotal / bytesPerGB)
		capacity.Status = machine.Status
		capacity.CPUUsage = machine.CPUUsage
		capacity.MemoryUsedGB = int(machine.MemoryUsed / bytesPerGB)
		capacity.StorageUsedGB = int(machine.StorageUsed / bytesPerGB)
		capacity.StorageTotalGB = int(machine.StorageTotal / bytesPerGB)

		break
	}

	return capacity, nil
}

const bytesPerGB int64 = 1024 * 1024 * 1024

type nodeUsage struct {
	vms, vcpus          int
	ramBytes, diskBytes int64
}

// pvmssUsage totals the pvmss-tagged VMs the index places on node.
func pvmssUsage(index *inventory.Index, node string) nodeUsage {
	var usage nodeUsage

	for _, machine := range index.ByNode[node] {
		if !slices.Contains(machine.Tags, "pvmss") {
			continue
		}

		usage.vms++
		usage.vcpus += vmVCPUs(machine)
		usage.ramBytes += machine.MemoryTotal
		usage.diskBytes += machine.DiskTotal
	}

	return usage
}

// NodeOverflow returns the cap dimensions ("vms","vcpus","ram","disk") the
// node would exceed if machine were added to it. Zero caps mean uncapped.
func NodeOverflow(index *inventory.Index, caps store.NodePolicyRow, node string, machine cluster.VM) []string {
	usage := pvmssUsage(index, node)
	overflow := []string{}

	if caps.MaxVMs > 0 && usage.vms+1 > caps.MaxVMs {
		overflow = append(overflow, dimensionVMs)
	}

	if caps.MaxVCPUs > 0 && usage.vcpus+vmVCPUs(machine) > caps.MaxVCPUs {
		overflow = append(overflow, dimensionVCPUs)
	}

	if caps.MaxRAMGB > 0 && int((usage.ramBytes+machine.MemoryTotal)/bytesPerGB) > caps.MaxRAMGB {
		overflow = append(overflow, dimensionRAM)
	}

	if caps.MaxDiskGB > 0 && int((usage.diskBytes+machine.DiskTotal)/bytesPerGB) > caps.MaxDiskGB {
		overflow = append(overflow, dimensionDisk)
	}

	return overflow
}

func (service *Policy) poolVMCount(pool string) int {
	if service.projection == nil || service.projection.Load() == nil {
		return 0
	}

	return len(service.projection.Load().ByPool[pool])
}

// PoolHasName reports whether any VM in the given pool already carries name
// (per-pool name uniqueness). The check is case-sensitive
// ValidateName already enforces lowercase hostname form, so "Web" and "web"
// cannot both pass validation. Returns false when no projection is wired
// (unit tests that don't need the check); callers that need a hard check
// must ensure a projection is configured.
func (service *Policy) PoolHasName(pool, name string) bool {
	if service.projection == nil || service.projection.Load() == nil {
		return false
	}

	for _, machine := range service.projection.Load().ByPool[pool] {
		if machine.Name == name {
			return true
		}
	}

	return false
}

func vmVCPUs(machine cluster.VM) int {
	if machine.Sockets > 0 && machine.Cores > 0 {
		return machine.Sockets * machine.Cores
	}

	return machine.CPUCores
}
