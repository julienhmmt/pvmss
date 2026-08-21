package vm

import (
	"context"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
)

// MetricsSample is a VM's metrics history point, re-exported from cluster
// so callers outside this package never import cluster directly (matching
// the Snapshot type alias pattern in snapshots.go).
type MetricsSample = cluster.MetricsSample

// MetricsDependencies contains the resolved read dependency for a metrics
// history request. No Writer, Policy, or Audit — this is a plain read, same
// as ListSnapshots needs no Audit for its own read path.
type MetricsDependencies struct {
	Index       *inventory.Index
	Actor       auth.Identity
	ClusterName string
	VMID        int
	Reader      cluster.MetricsHistoryReader
}

// GetMetricsHistory resolves ownership then reads the VM's metrics history
// for the given timeframe.
func GetMetricsHistory(ctx context.Context, deps MetricsDependencies, timeframe cluster.MetricsTimeframe) ([]MetricsSample, error) {
	entity, err := resolveMetricsEntity(deps.Index, deps.Actor, deps.ClusterName, deps.VMID)
	if err != nil {
		return nil, err
	}

	samples, err := deps.Reader.GetMetricsHistory(ctx, entity.Node, entity.VMID, timeframe)
	if err != nil {
		return nil, fmt.Errorf("get metrics history: %w", err)
	}

	return samples, nil
}

// MetricsCurrentDependencies contains the resolved read dependency for a
// current metrics request. Like MetricsDependencies, no Writer or Audit.
type MetricsCurrentDependencies struct {
	Index       *inventory.Index
	Actor       auth.Identity
	ClusterName string
	VMID        int
	Reader      cluster.MetricsCurrentReader
}

// GetMetricsCurrent resolves ownership then reads one current metrics sample
// for the resolved VM.
func GetMetricsCurrent(ctx context.Context, deps MetricsCurrentDependencies) (MetricsSample, error) {
	entity, err := resolveMetricsEntity(deps.Index, deps.Actor, deps.ClusterName, deps.VMID)
	if err != nil {
		return MetricsSample{}, err
	}

	sample, err := deps.Reader.GetMetricsCurrent(ctx, entity.Node, entity.VMID)
	if err != nil {
		return MetricsSample{}, fmt.Errorf("get metrics current: %w", err)
	}

	return sample, nil
}

func resolveMetricsEntity(index *inventory.Index, actor auth.Identity, clusterName string, vmid int) (Entity, error) {
	if index == nil {
		return Entity{}, ErrNotFound
	}

	return Resolve(index, actor, clusterName, vmid)
}
