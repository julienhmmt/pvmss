package policy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
)

// Upper sanity bounds for editable policy values. These are safety rails, not
// Proxmox hard limits - they prevent fat-fingered values from being persisted.
const (
	maxSocketsLimit      = 16
	maxCoresLimit        = 128
	maxMemoryMBLimit     = 1048576
	maxDiskPerVMGBLimit  = 1048576
	maxNetworkCardsLimit = 32
	maxSnapshotsLimit    = 1000
	maxVMPerUserLimit    = 100000
)

// Update is the outcome of UpdatePolicy: the policy state before and
// after the mutation. Changed is false when the mutation produced no
// difference - nothing was written and nothing is worth auditing.
type Update struct {
	Before  Settings
	After   Settings
	Changed bool
}

// maxPolicyUpdateAttempts bounds the optimistic-concurrency retries of
// UpdatePolicy. Concurrent admin writes are rare; a few retries absorb a
// losing race instead of surfacing a spurious conflict.
const maxPolicyUpdateAttempts = 3

// UpdatePolicy applies mutate to the cluster's stored policy and writes the
// result atomically: the write only lands when the stored row still equals
// what mutate saw (compare-and-swap), so a concurrent update between the read
// and the write is retried rather than silently overwritten. A mutation that
// changes nothing skips the write and reports Changed=false.
func (service *Policy) UpdatePolicy(ctx context.Context, clusterName string, mutate func(current Settings) Settings) (Update, error) {
	for range maxPolicyUpdateAttempts {
		row, err := service.store.PolicyRow(ctx, clusterName)

		missing := errors.Is(err, sql.ErrNoRows)
		if err != nil && !missing {
			return Update{}, err
		}

		if missing {
			row = defaultPolicyRow(clusterName)
		}

		before := Settings{Gabarit: gabaritFromRow(row), Allowed: row.MaxVMPerUser}
		after := mutate(before)

		if err := validateSettings(after); err != nil {
			return Update{}, err
		}

		if after == before {
			return Update{Before: before, After: after}, nil
		}

		var expected *store.PolicyRow
		if !missing {
			expected = &row
		}

		err = service.store.ReplacePolicyRow(ctx, expected, policyRowFrom(clusterName, after))
		if errors.Is(err, store.ErrPolicyConflict) {
			continue
		}

		if err != nil {
			return Update{}, err
		}

		return Update{Before: before, After: after, Changed: true}, nil
	}

	return Update{}, ErrConcurrentUpdate
}

// SetPolicy replaces the global gabarit and quota in one atomic update.
func (service *Policy) SetPolicy(ctx context.Context, clusterName string, gabarit Gabarit, allowed int) error {
	_, err := service.UpdatePolicy(ctx, clusterName, func(Settings) Settings {
		return Settings{Gabarit: gabarit, Allowed: allowed}
	})

	return err
}

func validateSettings(settings Settings) error {
	if err := validateGabarit(settings.Gabarit); err != nil {
		return err
	}

	if settings.Allowed < -1 || settings.Allowed > maxVMPerUserLimit {
		return fmt.Errorf("%w: maxVmPerUser must be between -1 and %d", ErrInvalidPolicy, maxVMPerUserLimit)
	}

	return nil
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
		max   int
	}{
		{"maxSockets", gabarit.MaxSockets, maxSocketsLimit},
		{"maxCores", gabarit.MaxCores, maxCoresLimit},
		{"maxMemoryMB", gabarit.MaxMemoryMB, maxMemoryMBLimit},
		{"maxDiskPerVmGb", gabarit.MaxDiskPerVMGB, maxDiskPerVMGBLimit},
		{"maxNetworkCards", gabarit.MaxNetworkCards, maxNetworkCardsLimit},
		{"maxSnapshots", gabarit.MaxSnapshots, maxSnapshotsLimit},
	}
	for _, item := range values {
		if item.value < 0 {
			return fmt.Errorf("%w: %s must not be negative", ErrInvalidPolicy, item.name)
		}

		if item.value > item.max {
			return fmt.Errorf("%w: %s exceeds the upper limit of %d", ErrInvalidPolicy, item.name, item.max)
		}
	}

	// Isolation VLAN tag is 0 (no tag) or a valid 802.1Q
	// tag (1–4094). 4095 is reserved in Proxmox and rejected here.
	if gabarit.IsolationVLANTag < 0 || gabarit.IsolationVLANTag > 4094 {
		return fmt.Errorf("%w: isolationVlanTag must be between 0 and 4094", ErrInvalidPolicy)
	}

	return nil
}

func validateCapacity(capacity Capacity) error {
	values := []struct {
		name  string
		value int
	}{
		{"maxVms", capacity.MaxVMs},
		{"maxVcpus", capacity.MaxVCPUs},
		{"maxRamGb", capacity.MaxRAMGB},
		{"maxDiskGb", capacity.MaxDiskGB},
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
		{dimensionVMs, requested.MaxVMs, current.UsedVMs},
		{dimensionVCPUs, requested.MaxVCPUs, current.UsedVCPUs},
		{dimensionRAM, requested.MaxRAMGB, current.UsedRAMGB},
		{dimensionDisk, requested.MaxDiskGB, current.UsedDiskGB},
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
		{dimensionVCPU, requested.MaxVCPUs, physical.CPUCores},
		{dimensionRAM, requested.MaxRAMGB, int(physical.MemoryTotal / bytesPerGB)},
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
	if dimension == dimensionVCPUs {
		dimension = dimensionVCPU
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
	if failure.Dimension == dimensionRAM {
		unit = " GB"
	}

	return fmt.Sprintf("%s cap (%d%s) exceeds %s's physical capacity (%d%s)", failure.Dimension, failure.Requested, unit, failure.Node, failure.Physical, unit)
}
func (failure *AboveNodeCapacityError) Unwrap() error { return ErrAboveNodeCapacity }
