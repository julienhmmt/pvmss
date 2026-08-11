package policy

import (
	"context"
	"fmt"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
)

// SetGabarit replaces the cluster gabarit while preserving its quota.
func (service *Policy) SetGabarit(ctx context.Context, clusterName string, gabarit Gabarit) error {
	if err := validateGabarit(gabarit); err != nil {
		return err
	}
	row, err := service.store.PolicyRow(ctx, clusterName)
	if err != nil {
		return err
	}
	row.MaxSockets = gabarit.MaxSockets
	row.MaxCores = gabarit.MaxCores
	row.MaxMemoryMB = gabarit.MaxMemoryMB
	row.MaxDiskPerVMGB = gabarit.MaxDiskPerVMGB
	row.MaxNetworkCards = gabarit.MaxNetworkCards
	row.MaxSnapshots = gabarit.MaxSnapshots
	row.AllowCustomYaml = gabarit.AllowCustomYaml
	return service.store.UpsertPolicyRow(ctx, row)
}

// SetPolicy replaces the global gabarit and quota in one persistence operation.
func (service *Policy) SetPolicy(ctx context.Context, clusterName string, gabarit Gabarit, allowed int) error {
	if err := validateGabarit(gabarit); err != nil {
		return err
	}
	if allowed < -1 {
		return fmt.Errorf("%w: maxVmPerUser must be -1 or greater", ErrInvalidPolicy)
	}
	row, err := service.store.PolicyRow(ctx, clusterName)
	if err != nil {
		return err
	}
	row.MaxSockets, row.MaxCores, row.MaxMemoryMB = gabarit.MaxSockets, gabarit.MaxCores, gabarit.MaxMemoryMB
	row.MaxDiskPerVMGB, row.MaxNetworkCards, row.MaxSnapshots = gabarit.MaxDiskPerVMGB, gabarit.MaxNetworkCards, gabarit.MaxSnapshots
	row.AllowCustomYaml, row.MaxVMPerUser = gabarit.AllowCustomYaml, allowed
	return service.store.UpsertPolicyRow(ctx, row)
}

// SetQuota replaces the per-user cluster quota. -1 means unlimited.
func (service *Policy) SetQuota(ctx context.Context, clusterName string, allowed int) error {
	if allowed < -1 {
		return fmt.Errorf("%w: maxVmPerUser must be -1 or greater", ErrInvalidPolicy)
	}
	row, err := service.store.PolicyRow(ctx, clusterName)
	if err != nil {
		return err
	}
	row.MaxVMPerUser = allowed
	return service.store.UpsertPolicyRow(ctx, row)
}

// SetNodeCapacity validates current usage and physical CPU/RAM before replacing
// a node capacité row. A zero value means no cap and always passes validation.
func (service *Policy) SetNodeCapacity(ctx context.Context, clusterName, node string, requested Capacity) error {
	if err := validateCapacity(requested); err != nil {
		return err
	}
	physicalNode, err := service.discoveredNode(ctx, node)
	if err != nil {
		return err
	}
	current, err := service.NodeCapacity(ctx, clusterName, node)
	if err != nil {
		return err
	}
	if err := checkBelowUsage(node, requested, current); err != nil {
		return err
	}
	if err := checkPhysicalCapacity(node, requested, physicalNode); err != nil {
		return err
	}
	return service.store.UpsertNodePolicyRow(ctx, store.NodePolicyRow{
		Cluster: clusterName, Node: node, MaxVMs: requested.MaxVMs,
		MaxVCPUs: requested.MaxVCPUs, MaxRAMGB: requested.MaxRAMGB, MaxDiskGB: requested.MaxDiskGB,
	})
}

func validateGabarit(gabarit Gabarit) error {
	values := []struct {
		name  string
		value int
	}{
		{"maxSockets", gabarit.MaxSockets}, {"maxCores", gabarit.MaxCores},
		{"maxMemoryMB", gabarit.MaxMemoryMB}, {"maxDiskPerVmGb", gabarit.MaxDiskPerVMGB},
		{"maxNetworkCards", gabarit.MaxNetworkCards}, {"maxSnapshots", gabarit.MaxSnapshots},
	}
	for _, item := range values {
		if item.value < 0 {
			return fmt.Errorf("%w: %s must not be negative", ErrInvalidPolicy, item.name)
		}
	}
	return nil
}

func validateCapacity(capacity Capacity) error {
	values := []struct {
		name  string
		value int
	}{
		{"maxVms", capacity.MaxVMs}, {"maxVcpus", capacity.MaxVCPUs},
		{"maxRamGb", capacity.MaxRAMGB}, {"maxDiskGb", capacity.MaxDiskGB},
	}
	for _, item := range values {
		if item.value < 0 {
			return fmt.Errorf("%w: %s must not be negative", ErrInvalidPolicy, item.name)
		}
	}
	return nil
}

func (service *Policy) discoveredNode(ctx context.Context, node string) (cluster.Node, error) {
	if service.client != nil {
		snapshot, err := service.client.Snapshot(ctx)
		if err != nil {
			return cluster.Node{}, fmt.Errorf("discover node: %w", err)
		}
		for _, item := range snapshot.Nodes {
			if item.Name == node {
				return item, nil
			}
		}
		return cluster.Node{}, cluster.ErrNotFound
	}
	if service.projection != nil && service.projection.Load() != nil {
		for _, item := range service.projection.Load().Nodes {
			if item.Name == node {
				return item, nil
			}
		}
	}
	return cluster.Node{}, cluster.ErrNotFound
}

func checkBelowUsage(node string, requested, current Capacity) error {
	values := []struct {
		dimension       string
		requested, used int
	}{
		{"vms", requested.MaxVMs, current.UsedVMs},
		{"vcpus", requested.MaxVCPUs, current.UsedVCPUs},
		{"ram", requested.MaxRAMGB, current.UsedRAMGB},
	}
	for _, item := range values {
		if item.requested != 0 && item.requested < item.used {
			return &BelowCurrentUsageError{Node: node, Dimension: item.dimension, Requested: item.requested, Used: item.used}
		}
	}
	return nil
}

func checkPhysicalCapacity(node string, requested Capacity, physical cluster.Node) error {
	values := []struct {
		dimension           string
		requested, physical int
	}{
		{"vcpu", requested.MaxVCPUs, physical.CPUCores},
		{"ram", requested.MaxRAMGB, int(physical.MemoryTotal / bytesPerGB)},
	}
	for _, item := range values {
		if item.requested != 0 && item.requested > item.physical {
			return &AboveNodeCapacityError{Node: node, Dimension: item.dimension, Requested: item.requested, Physical: item.physical}
		}
	}
	return nil
}

// BelowCurrentUsageError identifies a node capacité below live usage.
type BelowCurrentUsageError struct {
	Node, Dimension string
	Requested, Used int
}

func (failure *BelowCurrentUsageError) Error() string {
	dimension := failure.Dimension
	if dimension == "vcpus" {
		dimension = "vcpu"
	}
	return fmt.Sprintf("%s cap (%d) is below %s's current usage (%d)", dimension, failure.Requested, failure.Node, failure.Used)
}
func (failure *BelowCurrentUsageError) Unwrap() error { return ErrBelowCurrentUsage }

// AboveNodeCapacityError identifies a node capacité above physical capacity.
type AboveNodeCapacityError struct {
	Node, Dimension     string
	Requested, Physical int
}

func (failure *AboveNodeCapacityError) Error() string {
	unit := ""
	if failure.Dimension == "ram" {
		unit = " GB"
	}
	return fmt.Sprintf("%s cap (%d%s) exceeds %s's physical capacity (%d%s)", failure.Dimension, failure.Requested, unit, failure.Node, failure.Physical, unit)
}
func (failure *AboveNodeCapacityError) Unwrap() error { return ErrAboveNodeCapacity }
