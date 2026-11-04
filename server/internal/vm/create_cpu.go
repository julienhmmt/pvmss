package vm

import (
	"context"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/logctx"
	"time"
)

// hostCPUFlagsTimeout bounds the per-create node status reads, so a slow node
// cannot hold up the create. Expiry takes the same fallback as any read error.
const hostCPUFlagsTimeout = 5 * time.Second

// resolveCPUType picks the CPU model for a new VM from the cluster's host
// flags. It never fails the create: unreadable flags fall back to
// cluster.CPUTypeV2AES.
func resolveCPUType(ctx context.Context, creator cluster.Creator, clusterName string) string {
	flagsCtx, cancel := context.WithTimeout(ctx, hostCPUFlagsTimeout)
	defer cancel()

	flags, err := creator.HostCPUFlags(flagsCtx)
	if err != nil {
		logctx.From(ctx).WarnContext(ctx, "host cpu flags unreadable, using default cpu type",
			"component", "vm", "cluster", clusterName, "error", err)

		return cluster.CPUTypeV2AES
	}

	cpuType := cluster.SelectCPUType(flags)
	if cpuType == cluster.CPUTypeHost {
		logctx.From(ctx).WarnContext(ctx, "nodes share no x86-64 cpu model, using host cpu type",
			"component", "vm", "cluster", clusterName)
	}

	return cpuType
}
