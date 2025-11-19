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

// SnapshotVM represents a lightweight view of a VM used in the cached snapshot.
// It contains only the fields required by admin pages (tags, basic resources, status).
type SnapshotVM struct {
	Node     string
	VMID     int
	Name     string
	Status   string
	Tags     string
	Sockets  int
	Cores    int
	MemoryMB int64
}

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
	VMs            []SnapshotVM
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

	// Collect VM list and selected config fields (tags and basic resource config) for snapshot.
	if vms, err := proxmox.GetVMsResty(ctx, client); err != nil {
		log.Warn().Err(err).Msg("Failed to refresh VM list for snapshot")
		snapshot.Errors = append(snapshot.Errors, fmt.Sprintf("vms: %v", err))
	} else if len(vms) > 0 {
		vmSnapshots := make([]SnapshotVM, 0, len(vms))
		var vmMu sync.Mutex
		semVM := make(chan struct{}, 6)
		gVM, gvmCtx := errgroup.WithContext(ctx)

		for _, vm := range vms {
			vm := vm
			gVM.Go(func() error {
				select {
				case semVM <- struct{}{}:
					defer func() { <-semVM }()
				case <-gvmCtx.Done():
					return gvmCtx.Err()
				}

				cfgCtx, cancelCfg := context.WithTimeout(gvmCtx, constants.ClusterCacheRequestTimeout)
				defer cancelCfg()

				cfg, cfgErr := proxmox.GetVMConfigResty(cfgCtx, client, vm.Node, vm.VMID)
				if cfgErr != nil {
					log.Warn().Err(cfgErr).Str("node", vm.Node).Int("vmid", vm.VMID).Msg("Failed to refresh VM config for snapshot")
					recordError("vm config %s/%d: %v", vm.Node, vm.VMID, cfgErr)
					return nil
				}

				var tags string
				if v, ok := cfg["tags"].(string); ok {
					tags = v
				}

				sockets := 1
				cores := 1
				var memoryMB int64
				if raw, ok := cfg["sockets"]; ok {
					if f, ok := raw.(float64); ok && f > 0 {
						sockets = int(f)
					}
				}
				if raw, ok := cfg["cores"]; ok {
					if f, ok := raw.(float64); ok && f > 0 {
						cores = int(f)
					}
				}
				if raw, ok := cfg["memory"]; ok {
					if f, ok := raw.(float64); ok && f > 0 {
						memoryMB = int64(f)
					}
				}

				vmSnapshot := SnapshotVM{
					Node:     vm.Node,
					VMID:     vm.VMID,
					Name:     vm.Name,
					Status:   vm.Status,
					Tags:     tags,
					Sockets:  sockets,
					Cores:    cores,
					MemoryMB: memoryMB,
				}

				vmMu.Lock()
				vmSnapshots = append(vmSnapshots, vmSnapshot)
				vmMu.Unlock()
				return nil
			})
		}

		if err := gVM.Wait(); err != nil && err != context.Canceled {
			log.Warn().Err(err).Msg("VM snapshot worker exited early due to context cancellation")
		}

		if len(vmSnapshots) > 0 {
			snapshot.VMs = vmSnapshots
			log.Debug().Int("vms", len(vmSnapshots)).Msg("VM snapshot updated")
		}
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
		VMs:            cloneSnapshotVMs(snapshot.VMs),
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

func cloneSnapshotVMs(vms []SnapshotVM) []SnapshotVM {
	if len(vms) == 0 {
		return nil
	}
	cloned := make([]SnapshotVM, len(vms))
	copy(cloned, vms)
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
