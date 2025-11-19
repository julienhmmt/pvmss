package state

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"pvmss/constants"
	"pvmss/logger"
	"pvmss/proxmox"
)

// ProxmoxClusterSnapshot stores a best-effort view of the cluster retrieved in the background.
// It contains the subset of data frequently used by handlers so that they can serve pages
// without waiting on live Proxmox calls.
type ProxmoxClusterSnapshot struct {
	GeneratedAt    time.Time
	Duration       time.Duration
	NodeNames      []string
	OnlineNodes    []string
	NodeDetails    []*proxmox.NodeDetails
	GlobalStorages []proxmox.Storage
	NodeStorages   map[string][]proxmox.Storage
	NetworkBridges map[string][]proxmox.VMBR
	Errors         []string
}

// buildProxmoxSnapshot collects cluster information using the resty client.
// It never returns nil/partial data silently: callers receive either a fully populated snapshot
// or an error when even the node inventory could not be fetched.
func buildProxmoxSnapshot(ctx context.Context, client *proxmox.RestyClient) (*ProxmoxClusterSnapshot, error) {
	log := logger.Get().With().Str("component", "ProxmoxSnapshotBuilder").Logger()
	start := time.Now()

	nodes, err := proxmox.GetNodeNamesResty(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to list Proxmox nodes: %w", err)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no Proxmox nodes discovered")
	}
	sort.Strings(nodes)

	snapshot := &ProxmoxClusterSnapshot{
		NodeNames:      nodes,
		NodeStorages:   make(map[string][]proxmox.Storage, len(nodes)),
		NetworkBridges: make(map[string][]proxmox.VMBR, len(nodes)),
	}

	if onlineNodes, err := proxmox.GetOnlineNodeNamesResty(ctx, client); err != nil {
		log.Warn().Err(err).Msg("Failed to resolve online nodes; falling back to full node list")
		snapshot.Errors = append(snapshot.Errors, fmt.Sprintf("online nodes: %v", err))
		snapshot.OnlineNodes = append([]string(nil), nodes...)
	} else {
		sort.Strings(onlineNodes)
		if len(onlineNodes) == 0 {
			onlineNodes = nodes
		}
		snapshot.OnlineNodes = onlineNodes
	}

	if nodeDetails, err := proxmox.FetchAllNodeDetailsResty(ctx, client); err != nil {
		log.Warn().Err(err).Msg("Failed to refresh node details for snapshot")
		snapshot.Errors = append(snapshot.Errors, fmt.Sprintf("node details: %v", err))
	} else {
		snapshot.NodeDetails = nodeDetails
	}

	if storages, err := proxmox.GetStoragesResty(ctx, client); err != nil {
		log.Warn().Err(err).Msg("Failed to refresh global storages for snapshot")
		snapshot.Errors = append(snapshot.Errors, fmt.Sprintf("global storages: %v", err))
	} else {
		snapshot.GlobalStorages = storages
	}

	// Collect per-node storages and networks concurrently while throttling outbound requests.
	var storageMu sync.Mutex
	var networkMu sync.Mutex
	var errorMu sync.Mutex
	recordError := func(format string, args ...interface{}) {
		errorMu.Lock()
		snapshot.Errors = append(snapshot.Errors, fmt.Sprintf(format, args...))
		errorMu.Unlock()
	}

	sem := make(chan struct{}, 6)
	g, gctx := errgroup.WithContext(ctx)
	for _, node := range nodes {
		nodeName := node
		g.Go(func() error {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-gctx.Done():
				return gctx.Err()
			}

			// Fetch storages for the node
			storageCtx, cancelStorage := context.WithTimeout(gctx, constants.ClusterCacheRequestTimeout)
			storages, storErr := proxmox.GetNodeStoragesResty(storageCtx, client, nodeName)
			cancelStorage()
			if storErr != nil {
				log.Warn().Err(storErr).Str("node", nodeName).Msg("Failed to refresh storages for snapshot")
				recordError("storages for %s: %v", nodeName, storErr)
			} else {
				storageMu.Lock()
				snapshot.NodeStorages[nodeName] = storages
				storageMu.Unlock()
			}

			// Fetch network bridges for the node
			networkCtx, cancelNetwork := context.WithTimeout(gctx, constants.ClusterCacheRequestTimeout)
			vmbrs, vmbrErr := proxmox.GetVMBRsResty(networkCtx, client, nodeName)
			cancelNetwork()
			if vmbrErr != nil {
				log.Warn().Err(vmbrErr).Str("node", nodeName).Msg("Failed to refresh network bridges for snapshot")
				recordError("vmbr for %s: %v", nodeName, vmbrErr)
			} else if len(vmbrs) > 0 {
				networkMu.Lock()
				snapshot.NetworkBridges[nodeName] = vmbrs
				networkMu.Unlock()
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil && err != context.Canceled {
		log.Warn().Err(err).Msg("Snapshot worker exited early due to context cancellation")
	}

	snapshot.GeneratedAt = time.Now()
	snapshot.Duration = time.Since(start)
	return snapshot, nil
}

// cloneProxmoxSnapshot performs a deep copy so callers cannot mutate shared state.
func cloneProxmoxSnapshot(snapshot *ProxmoxClusterSnapshot) *ProxmoxClusterSnapshot {
	if snapshot == nil {
		return nil
	}

	cloned := &ProxmoxClusterSnapshot{
		GeneratedAt:    snapshot.GeneratedAt,
		Duration:       snapshot.Duration,
		NodeNames:      append([]string(nil), snapshot.NodeNames...),
		OnlineNodes:    append([]string(nil), snapshot.OnlineNodes...),
		NodeDetails:    cloneNodeDetails(snapshot.NodeDetails),
		GlobalStorages: cloneStorages(snapshot.GlobalStorages),
		Errors:         append([]string(nil), snapshot.Errors...),
	}

	if len(snapshot.NodeStorages) > 0 {
		cloned.NodeStorages = make(map[string][]proxmox.Storage, len(snapshot.NodeStorages))
		for node, storages := range snapshot.NodeStorages {
			cloned.NodeStorages[node] = cloneStorages(storages)
		}
	}

	if len(snapshot.NetworkBridges) > 0 {
		cloned.NetworkBridges = make(map[string][]proxmox.VMBR, len(snapshot.NetworkBridges))
		for node, vmbrs := range snapshot.NetworkBridges {
			cloned.NetworkBridges[node] = cloneVMBRs(vmbrs)
		}
	}

	return cloned
}

func cloneStorages(storages []proxmox.Storage) []proxmox.Storage {
	if len(storages) == 0 {
		return nil
	}
	cloned := make([]proxmox.Storage, len(storages))
	copy(cloned, storages)
	return cloned
}

func cloneVMBRs(vmbrs []proxmox.VMBR) []proxmox.VMBR {
	if len(vmbrs) == 0 {
		return nil
	}
	cloned := make([]proxmox.VMBR, len(vmbrs))
	copy(cloned, vmbrs)
	return cloned
}
