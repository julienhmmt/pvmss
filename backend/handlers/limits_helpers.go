package handlers

import (
	"context"
	"fmt"
	"strings"
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

// CalculateNodeResourceUsage calculates the aggregated resources used by VMs with the "pvmss" tag
// for each node in the Proxmox cluster using resty
func CalculateNodeResourceUsage(ctx context.Context, client proxmox.ClientInterface, sm LimitsGetter) (map[string]*NodeResourceUsage, error) {
	log := logger.Get().With().Str("function", "CalculateNodeResourceUsage").Logger()

	// Create resty client
	restyClient, err := proxmox.NewRestyClientFromEnv(30 * time.Second)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create resty client")
		return nil, err
	}

	// Get all nodes
	nodes, err := proxmox.GetNodeNamesResty(ctx, restyClient)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get nodes")
		return nil, err
	}

	usage := make(map[string]*NodeResourceUsage)

	// Initialize usage for each node
	for _, node := range nodes {
		usage[node] = &NodeResourceUsage{
			Node: node,
		}
	}

	// Get all VMs
	vms, err := proxmox.GetVMsResty(ctx, restyClient)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get VMs")
		return usage, nil // Return empty usage instead of error
	}

	// Iterate through VMs and accumulate resources for pvmss-tagged VMs
	for _, vm := range vms {
		// Get VM config to check tags
		cfg, err := proxmox.GetVMConfigResty(ctx, restyClient, vm.Node, vm.VMID)
		if err != nil {
			log.Warn().Err(err).Str("node", vm.Node).Int("vmid", vm.VMID).Msg("Failed to get VM config")
			continue
		}

		// Check if VM has pvmss tag
		hasPvmssTag := false
		if tagsStr, ok := cfg["tags"].(string); ok && tagsStr != "" {
			// Parse tags (can be separated by semicolon or comma)
			tags := parseTags(tagsStr)
			for _, tag := range tags {
				if strings.EqualFold(strings.TrimSpace(tag), "pvmss") {
					hasPvmssTag = true
					break
				}
			}
		}

		if !hasPvmssTag {
			continue
		}

		// Get node usage tracker
		nodeUsage := usage[vm.Node]
		nodeUsage.TotalVMs++

		// Extract CPU configuration (sockets and cores)
		vmSockets := 1
		vmCores := 1

		if socketsRaw, ok := cfg["sockets"]; ok {
			if socketsFloat, ok := socketsRaw.(float64); ok {
				vmSockets = int(socketsFloat)
			}
		}

		if coresRaw, ok := cfg["cores"]; ok {
			if coresFloat, ok := coresRaw.(float64); ok {
				vmCores = int(coresFloat)
			}
		}

		// Total cores for this VM = sockets * cores
		nodeUsage.Cores += vmSockets * vmCores

		// Extract memory (stored in MB in Proxmox)
		if memRaw, ok := cfg["memory"]; ok {
			if memFloat, ok := memRaw.(float64); ok {
				nodeUsage.RamMB += int64(memFloat)
			}
		}
	}

	// Convert RAM from MB to GB for display
	for _, nodeUsage := range usage {
		nodeUsage.RamGB = int(nodeUsage.RamMB / 1024)
	}

	// Get limits from settings to populate max values
	settings := sm.GetSettings()
	if settings != nil && settings.Limits != nil {
		if nodesLimits, ok := settings.Limits["nodes"].(map[string]interface{}); ok {
			for nodeName, nodeUsage := range usage {
				if nodeLimitRaw, ok := nodesLimits[nodeName].(map[string]interface{}); ok {
					// Extract max cores
					if _, max, ok := readMinMax(nodeLimitRaw, "cores"); ok {
						nodeUsage.MaxCores = max
					}
					// Extract max RAM
					if _, max, ok := readMinMax(nodeLimitRaw, "ram"); ok {
						nodeUsage.MaxRamGB = max
					}
				}
			}
		}
	}

	return usage, nil
}

// parseTags splits a tag string by semicolons and commas
func parseTags(tagsStr string) []string {
	var tags []string
	// First split by semicolon
	semiParts := strings.Split(tagsStr, ";")
	for _, part := range semiParts {
		// Then split by comma
		commaParts := strings.Split(part, ",")
		for _, tag := range commaParts {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

// LimitsGetter defines the minimal interface needed to get settings
type LimitsGetter interface {
	GetSettings() *state.AppSettings
}

// NodeCapacity represents the physical hardware capacity of a Proxmox node
type NodeCapacity struct {
	Node     string
	CPUs     int   // Total physical CPU cores
	MemoryGB int   // Total RAM in GB
	MemoryMB int64 // Total RAM in MB
}

// GetNodeCapacity retrieves the physical hardware capacity of a node using resty
func GetNodeCapacity(ctx context.Context, client proxmox.ClientInterface, nodeName string) (*NodeCapacity, error) {
	log := logger.Get().With().Str("function", "GetNodeCapacity").Logger()

	// Create resty client
	restyClient, err := proxmox.NewRestyClientFromEnv(10 * time.Second)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create resty client")
		return nil, err
	}

	nodeDetails, err := proxmox.GetNodeDetailsResty(ctx, restyClient, nodeName)
	if err != nil {
		log.Error().Err(err).Str("node", nodeName).Msg("Failed to get node details")
		return nil, err
	}

	memoryMB := int64(nodeDetails.MaxMemory)
	capacity := &NodeCapacity{
		Node:     nodeName,
		CPUs:     nodeDetails.MaxCPU,
		MemoryMB: memoryMB,
		MemoryGB: int(memoryMB / (1024 * 1024 * 1024)), // MaxMemory is in bytes
	}

	log.Debug().
		Str("node", nodeName).
		Int("cpus", capacity.CPUs).
		Int("memory_gb", capacity.MemoryGB).
		Msg("Retrieved node capacity")

	return capacity, nil
}

// ValidateNodeLimitsAgainstCapacity validates that configured limits don't exceed node physical capacity
func ValidateNodeLimitsAgainstCapacity(ctx context.Context, client proxmox.ClientInterface, nodeName string, maxCores, maxRamGB int) error {
	log := logger.Get().With().Str("function", "ValidateNodeLimitsAgainstCapacity").Logger()

	capacity, err := GetNodeCapacity(ctx, client, nodeName)
	if err != nil {
		log.Warn().Err(err).Msg("Could not retrieve node capacity, skipping validation")
		return nil // Don't block if we can't get capacity
	}

	// Validate cores
	if maxCores > capacity.CPUs {
		return fmt.Errorf("aggregate cores limit (%d) exceeds node physical capacity (%d CPUs)", maxCores, capacity.CPUs)
	}

	// Validate RAM
	if maxRamGB > capacity.MemoryGB {
		return fmt.Errorf("aggregate RAM limit (%d GB) exceeds node physical capacity (%d GB)", maxRamGB, capacity.MemoryGB)
	}

	log.Info().
		Str("node", nodeName).
		Int("max_cores", maxCores).
		Int("max_ram_gb", maxRamGB).
		Int("node_cpus", capacity.CPUs).
		Int("node_ram_gb", capacity.MemoryGB).
		Msg("Node limits validated against physical capacity")

	return nil
}

// ValidateVMResourcesAgainstNodeLimits validates that adding a new VM won't exceed node aggregate limits
func ValidateVMResourcesAgainstNodeLimits(ctx context.Context, client proxmox.ClientInterface, sm LimitsGetter, node string, sockets, cores int, memoryMB int) error {
	log := logger.Get().With().Str("function", "ValidateVMResourcesAgainstNodeLimits").Logger()

	// Calculate current usage
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	usageMap, err := CalculateNodeResourceUsage(ctxWithTimeout, client, sm)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to calculate node resource usage, skipping aggregate validation")
		return nil // Don't block VM creation if we can't calculate usage
	}

	nodeUsage, exists := usageMap[node]
	if !exists {
		log.Warn().Str("node", node).Msg("Node not found in usage map")
		return nil // Don't block if node not found
	}

	// Check if limits are configured for this node
	if nodeUsage.MaxCores == 0 && nodeUsage.MaxRamGB == 0 {
		// No aggregate limits configured for this node
		return nil
	}

	memoryGB := memoryMB / 1024

	// Validate cores
	if nodeUsage.MaxCores > 0 {
		totalCores := sockets * cores
		newTotal := nodeUsage.Cores + totalCores
		if newTotal > nodeUsage.MaxCores {
			return fmt.Errorf("adding this VM would exceed node '%s' aggregate cores limit (current: %d, requested: %d, max: %d)",
				node, nodeUsage.Cores, totalCores, nodeUsage.MaxCores)
		}
	}

	// Validate RAM
	if nodeUsage.MaxRamGB > 0 {
		newTotal := nodeUsage.RamGB + memoryGB
		if newTotal > nodeUsage.MaxRamGB {
			return fmt.Errorf("adding this VM would exceed node '%s' aggregate RAM limit (current: %d GB, requested: %d GB, max: %d GB)",
				node, nodeUsage.RamGB, memoryGB, nodeUsage.MaxRamGB)
		}
	}

	log.Info().
		Str("node", node).
		Int("current_cores", nodeUsage.Cores).
		Int("current_ram_gb", nodeUsage.RamGB).
		Msg("VM creation validated against aggregate node limits")

	return nil
}
