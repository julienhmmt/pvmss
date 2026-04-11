package handlers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// NodeResourceUsage represents the aggregated resource usage for VMs with pvmss tag on a node
type NodeResourceUsage struct {
	Node     string
	TotalVMs int
	Cores    int
	RamMB    int64
	RamGB    int
	MaxCores int
	MaxRamGB int
}

const nodeUsageCacheTTL = 30 * time.Second

var (
	nodeUsageCache      map[string]*NodeResourceUsage
	nodeUsageCacheMu    sync.RWMutex
	nodeUsageCacheReady bool
	nodeUsageCacheExp   time.Time
)

// LimitsGetter defines the minimal interface needed to get settings
type LimitsGetter interface {
	GetSettings() *state.AppSettings
}

// splitTags splits a tag string by semicolons and commas
func splitTags(tagsStr string) []string {
	var tags []string
	for _, part := range strings.Split(tagsStr, ";") {
		for _, tag := range strings.Split(part, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

// CalculateNodeResourceUsage calculates the aggregated resources used by VMs with the "pvmss" tag
// for each node in the Proxmox cluster.
func CalculateNodeResourceUsage(ctx context.Context, sm LimitsGetter) (map[string]*NodeResourceUsage, error) {
	log := logger.Get().With().Str("function", "CalculateNodeResourceUsage").Logger()

	if cached, ok := getCachedNodeUsage(); ok {
		log.Debug().Msg("Returning cached node resource usage")
		return cached, nil
	}

	// Prefer using the cached Proxmox cluster snapshot when available
	if snapshotProvider, ok := sm.(interface {
		GetProxmoxSnapshot() *state.ProxmoxClusterSnapshot
	}); ok {
		if snapshot := snapshotProvider.GetProxmoxSnapshot(); snapshot != nil && len(snapshot.VMs) > 0 {
			usageFromSnapshot := buildNodeUsageFromSnapshot(snapshot, sm.GetSettings())
			if len(usageFromSnapshot) > 0 {
				storeNodeUsageCache(usageFromSnapshot)
				if cached, ok := getCachedNodeUsage(); ok {
					log.Debug().Int("nodes", len(cached)).Msg("Returning node resource usage from snapshot")
					return cached, nil
				}
				return usageFromSnapshot, nil
			}
		}
	}

	restyClient, err := proxmox.MakeRestyClientFromEnv(30 * time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to create resty client for resource usage: %w", err)
	}

	nodes, err := proxmox.GetNodeNamesResty(ctx, restyClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get node names for resource usage: %w", err)
	}

	usage := make(map[string]*NodeResourceUsage, len(nodes))
	for _, node := range nodes {
		usage[node] = &NodeResourceUsage{Node: node}
	}

	vms, err := proxmox.GetVMsResty(ctx, restyClient)
	if err != nil {
		return usage, nil
	}

	for _, vm := range vms {
		cfg, err := proxmox.GetVMConfigResty(ctx, restyClient, vm.Node, vm.VMID)
		if err != nil {
			continue
		}

		hasPvmssTag := false
		if tagsStr, ok := cfg["tags"].(string); ok && tagsStr != "" {
			for _, tag := range splitTags(tagsStr) {
				if strings.EqualFold(strings.TrimSpace(tag), "pvmss") {
					hasPvmssTag = true
					break
				}
			}
		}
		if !hasPvmssTag {
			continue
		}

		nodeUsage := usage[vm.Node]
		nodeUsage.TotalVMs++

		vmSockets := 1
		if socketsRaw, ok := cfg["sockets"]; ok {
			if socketsFloat, ok := socketsRaw.(float64); ok {
				vmSockets = int(socketsFloat)
			}
		}
		vmCores := 1
		if coresRaw, ok := cfg["cores"]; ok {
			if coresFloat, ok := coresRaw.(float64); ok {
				vmCores = int(coresFloat)
			}
		}
		nodeUsage.Cores += vmSockets * vmCores

		if memRaw, ok := cfg["memory"]; ok {
			if memFloat, ok := memRaw.(float64); ok {
				nodeUsage.RamMB += int64(memFloat)
			}
		}
	}

	for _, nodeUsage := range usage {
		nodeUsage.RamGB = int(MBToGB(nodeUsage.RamMB))
	}

	settings := sm.GetSettings()
	if settings != nil {
		for nodeName, nodeUsage := range usage {
			if nodeLimits, ok := settings.Limits.Nodes[nodeName]; ok {
				nodeUsage.MaxCores = nodeLimits.Cores.Max
				nodeUsage.MaxRamGB = nodeLimits.RAM.Max
			}
		}
	}

	storeNodeUsageCache(usage)
	if cached, ok := getCachedNodeUsage(); ok {
		return cached, nil
	}
	return usage, nil
}

// buildNodeUsageFromSnapshot aggregates resource usage per node from a cached snapshot.
func buildNodeUsageFromSnapshot(snapshot *state.ProxmoxClusterSnapshot, settings *state.AppSettings) map[string]*NodeResourceUsage {
	if snapshot == nil {
		return map[string]*NodeResourceUsage{}
	}

	usage := make(map[string]*NodeResourceUsage, len(snapshot.NodeNames))
	for _, node := range snapshot.NodeNames {
		if node != "" {
			usage[node] = &NodeResourceUsage{Node: node}
		}
	}

	for _, vm := range snapshot.VMs {
		if vm.Node == "" || vm.Tags == "" {
			continue
		}
		hasPvmss := false
		for _, tag := range splitTags(vm.Tags) {
			if strings.EqualFold(strings.TrimSpace(tag), "pvmss") {
				hasPvmss = true
				break
			}
		}
		if !hasPvmss {
			continue
		}

		nodeUsage, ok := usage[vm.Node]
		if !ok {
			nodeUsage = &NodeResourceUsage{Node: vm.Node}
			usage[vm.Node] = nodeUsage
		}
		nodeUsage.TotalVMs++
		if vm.Sockets <= 0 {
			vm.Sockets = 1
		}
		if vm.Cores <= 0 {
			vm.Cores = 1
		}
		nodeUsage.Cores += vm.Sockets * vm.Cores
		if vm.MemoryMB > 0 {
			nodeUsage.RamMB += vm.MemoryMB
		}
	}

	for _, nodeUsage := range usage {
		nodeUsage.RamGB = int(MBToGB(nodeUsage.RamMB))
	}

	if settings != nil {
		for nodeName, nodeUsage := range usage {
			if nodeLimits, ok := settings.Limits.Nodes[nodeName]; ok {
				nodeUsage.MaxCores = nodeLimits.Cores.Max
				nodeUsage.MaxRamGB = nodeLimits.RAM.Max
			}
		}
	}

	return usage
}

func getCachedNodeUsage() (map[string]*NodeResourceUsage, bool) {
	nodeUsageCacheMu.RLock()
	defer nodeUsageCacheMu.RUnlock()
	if !nodeUsageCacheReady || time.Now().After(nodeUsageCacheExp) || len(nodeUsageCache) == 0 {
		return nil, false
	}
	copied := make(map[string]*NodeResourceUsage, len(nodeUsageCache))
	for k, v := range nodeUsageCache {
		if v == nil {
			continue
		}
		copyVal := *v
		copied[k] = &copyVal
	}
	return copied, true
}

func storeNodeUsageCache(usage map[string]*NodeResourceUsage) {
	nodeUsageCacheMu.Lock()
	defer nodeUsageCacheMu.Unlock()
	copied := make(map[string]*NodeResourceUsage, len(usage))
	for k, v := range usage {
		if v == nil {
			continue
		}
		copyVal := *v
		copied[k] = &copyVal
	}
	nodeUsageCache = copied
	nodeUsageCacheExp = time.Now().Add(nodeUsageCacheTTL)
	nodeUsageCacheReady = true
}

// returnLocalizedError returns a simple error (no i18n)
func returnLocalizedError(messageID string, templateData map[string]interface{}, fallbackFormat string, args ...interface{}) error {
	return fmt.Errorf(fallbackFormat, args...)
}

// ValidateVMResourcesAgainstNodeLimits validates that adding a new VM won't exceed node aggregate limits.
func ValidateVMResourcesAgainstNodeLimits(ctx context.Context, sm LimitsGetter, node string, sockets, cores int, memoryMB int) error {
	log := logger.Get().With().Str("function", "ValidateVMResourcesAgainstNodeLimits").Logger()

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	usageMap, err := CalculateNodeResourceUsage(ctxWithTimeout, sm)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to calculate node resource usage, skipping aggregate validation")
		return nil
	}

	nodeUsage, exists := usageMap[node]
	if !exists {
		log.Warn().Str("node", node).Msg("Node not found in usage map")
		return nil
	}

	if nodeUsage.MaxCores == 0 && nodeUsage.MaxRamGB == 0 {
		return nil
	}

	memoryGB := int(MBToGB(int64(memoryMB)))

	if nodeUsage.MaxCores > 0 {
		totalCores := sockets * cores
		newTotal := nodeUsage.Cores + totalCores
		if newTotal > nodeUsage.MaxCores {
			if err := returnLocalizedError("", nil, "adding this VM would exceed node '%s' aggregate cores limit (current: %d, requested: %d, max: %d)",
				node, nodeUsage.Cores, totalCores, nodeUsage.MaxCores); err != nil {
				return err
			}
		}
	}

	if nodeUsage.MaxRamGB > 0 {
		newTotal := nodeUsage.RamGB + memoryGB
		if newTotal > nodeUsage.MaxRamGB {
			if err := returnLocalizedError("", nil, "adding this VM would exceed node '%s' aggregate RAM limit (current: %d GB, requested: %d GB, max: %d GB)",
				node, nodeUsage.RamGB, memoryGB, nodeUsage.MaxRamGB); err != nil {
				return err
			}
		}
	}

	log.Info().Str("node", node).Int("current_cores", nodeUsage.Cores).Int("current_ram_gb", nodeUsage.RamGB).Msg("VM creation validated against aggregate node limits")
	return nil
}
