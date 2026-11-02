package vm

import (
	"context"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
)

// GetCloudInitConfig reads live structured state after the shared ownership gate.
func GetCloudInitConfig(ctx context.Context, index *inventory.Index, actor auth.Identity, clusterName string, vmid int, reader cluster.CloudInitReader) (cluster.CloudInitConfig, error) {
	entity, err := resolveCloudInitTarget(index, actor, clusterName, vmid)
	if err != nil {
		return cluster.CloudInitConfig{}, err
	}

	config, err := reader.GetCloudInitConfig(ctx, entity.Node, entity.VMID)
	if err != nil {
		return cluster.CloudInitConfig{}, err
	}

	return config, nil
}

func resolveCloudInitTarget(index *inventory.Index, actor auth.Identity, clusterName string, vmid int) (Entity, error) {
	if index == nil {
		return Entity{}, ErrNotFound
	}

	return Resolve(index, actor, clusterName, vmid)
}
