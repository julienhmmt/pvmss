package policy

import (
	"context"
	"fmt"
	"pvmss/server/internal/cluster"
	"slices"
)

// NodeCapacities returns the live discovery set joined with configured capacité
// and current pvmss-tagged usage.
func (service *Policy) NodeCapacities(ctx context.Context, clusterName string) ([]Capacity, error) {
	nodes, err := service.discoveredNodes(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]Capacity, 0, len(nodes))
	for _, node := range nodes {
		capacity, err := service.NodeCapacity(ctx, clusterName, node.Name)
		if err != nil {
			return nil, err
		}

		capacity.PhysicalVCPUs = node.CPUCores
		capacity.PhysicalRAMGB = int(node.MemoryTotal / bytesPerGB)
		result = append(result, capacity)
	}

	slices.SortFunc(result, func(left, right Capacity) int { return compareNode(left.Node, right.Node) })

	return result, nil
}

func (service *Policy) discoveredNodes(ctx context.Context) ([]cluster.Node, error) {
	if service.client != nil {
		snapshot, err := service.client.Snapshot(ctx)
		if err != nil {
			return nil, fmt.Errorf("discover nodes: %w", err)
		}

		return snapshot.Nodes, nil
	}

	if service.projection != nil && service.projection.Load() != nil {
		return service.projection.Load().Nodes, nil
	}

	return nil, nil
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
