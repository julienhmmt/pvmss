package policy

import (
	"context"
	"fmt"
	"pvmss/server/internal/cluster"
	"slices"
	"time"
)

// RefreshedAt reports when the inventory projection last populated; the zero
// Time means it never has.
func (service *Policy) RefreshedAt() time.Time {
	if service.projection == nil || service.projection.Load() == nil {
		return time.Time{}
	}

	return service.projection.Load().RefreshedAt
}

// NodeCapacities returns the live discovery set joined with configured capacité
// and current pvmss-tagged usage.
func (service *Policy) NodeCapacities(ctx context.Context, clusterName string) ([]Capacity, error) {
	nodes, err := service.discoveredNodes(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	approvals, err := service.store.CatalogNodesEnabled(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	approved := make(map[string]bool, len(approvals))
	for _, row := range approvals {
		approved[row.Name] = row.Enabled
	}

	result := make([]Capacity, 0, len(nodes))
	for _, node := range nodes {
		capacity, err := service.NodeCapacity(ctx, clusterName, node.Name)
		if err != nil {
			return nil, err
		}

		capacity.Approved = approved[node.Name]
		capacity.PhysicalVCPUs = node.CPUCores
		capacity.PhysicalRAMGB = int(node.MemoryTotal / bytesPerGB)
		capacity.Status = node.Status
		capacity.CPUUsage = node.CPUUsage
		capacity.MemoryUsedGB = int(node.MemoryUsed / bytesPerGB)
		capacity.StorageUsedGB = int(node.StorageUsed / bytesPerGB)
		capacity.StorageTotalGB = int(node.StorageTotal / bytesPerGB)
		result = append(result, capacity)
	}

	slices.SortFunc(result, func(left, right Capacity) int { return compareNode(left.Node, right.Node) })

	return result, nil
}

func (service *Policy) discoveredNodes(ctx context.Context, clusterName string) ([]cluster.Node, error) {
	snapshot, ok, err := service.snapshotFor(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	if ok {
		return snapshot.Nodes, nil
	}

	if service.projection != nil && service.projection.Load() != nil {
		return service.projection.Load().Nodes, nil
	}

	return nil, nil
}

// snapshotFor reads the named cluster's live inventory. ok is false when no
// live source is wired, so callers fall back to the inventory projection.
func (service *Policy) snapshotFor(ctx context.Context, clusterName string) (cluster.Snapshot, bool, error) {
	source := service.client
	if service.clusters != nil {
		client, err := service.clusters.Client(clusterName)
		if err != nil {
			return cluster.Snapshot{}, true, fmt.Errorf("discover nodes: %w", err)
		}

		source = client
	}

	if source == nil {
		return cluster.Snapshot{}, false, nil
	}

	snapshot, err := source.Snapshot(ctx)
	if err != nil {
		return cluster.Snapshot{}, true, fmt.Errorf("discover nodes: %w", err)
	}

	return snapshot, true, nil
}

// SetClusterProvider makes node discovery follow each request's cluster
// instead of one fixed client.
func (service *Policy) SetClusterProvider(provider cluster.ClientProvider) {
	service.clusters = provider
}

func compareNode(left, right string) int {
	if left < right {
		return -1
	}

	if left > right {
		return 1
	}

	return 0
}
