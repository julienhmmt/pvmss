package apiv1

import (
	"context"
	"sort"
	"strings"

	"pvmss/constants"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// resolveNodes returns node options from snapshot or live data.
// A node is disabled when its current pvmss aggregate usage leaves no room
// for even a minimum-sized VM, according to the limits defined in settings.
func (h *VMCreateHandler) resolveNodes(ctx context.Context, snapshot *state.ProxmoxClusterSnapshot, settings *state.AppSettings) ([]VMCreateNodeOption, map[string]bool) {
	disabledNodes := make(map[string]bool)
	var nodeNames []string

	if snapshot != nil && len(snapshot.OnlineNodes) > 0 {
		nodeNames = append(nodeNames, snapshot.OnlineNodes...)
	}

	if len(nodeNames) == 0 {
		client, err := restyClient()
		if err == nil {
			names, _ := proxmox.GetNodeNamesResty(ctx, client)
			nodeNames = names
		}
	}

	if snapshot != nil && settings != nil && len(settings.Limits.Nodes) > 0 {
		if settings.Limits.VM.Sockets.Min > 0 && settings.Limits.VM.Cores.Min > 0 && settings.Limits.VM.RAM.Min > 0 {
			nodeUsage := computeNodeUsageFromSnapshot(snapshot)
			minCores := settings.Limits.VM.Sockets.Min * settings.Limits.VM.Cores.Min
			minRAMGB := settings.Limits.VM.RAM.Min
			if minCores <= 0 {
				minCores = 1
			}
			if minRAMGB <= 0 {
				minRAMGB = 1
			}
			for nodeName, nodeLimits := range settings.Limits.Nodes {
				usage := nodeUsage[nodeName]
				if nodeLimits.MaxVMs > 0 && usage.totalVMs >= nodeLimits.MaxVMs {
					disabledNodes[nodeName] = true
				}
				if nodeLimits.Cores.Max > 0 && usage.cores+minCores > nodeLimits.Cores.Max {
					disabledNodes[nodeName] = true
				}
				if nodeLimits.RAM.Max > 0 && usage.ramGB+minRAMGB > nodeLimits.RAM.Max {
					disabledNodes[nodeName] = true
				}
			}
		}
	}

	sort.Strings(nodeNames)
	options := make([]VMCreateNodeOption, 0, len(nodeNames))
	for _, name := range nodeNames {
		opt := VMCreateNodeOption{Name: name, Disabled: disabledNodes[name]}
		if disabledNodes[name] {
			opt.Reason = "Node limit reached"
		}
		options = append(options, opt)
	}
	return options, disabledNodes
}

// nodeAggregateUsage holds the aggregate resource usage for pvmss VMs on a node.
type nodeAggregateUsage struct {
	totalVMs int
	cores    int
	ramGB    int
}

// computeNodeUsageFromSnapshot sums cores and RAM for pvmss-tagged VMs per node.
func computeNodeUsageFromSnapshot(snapshot *state.ProxmoxClusterSnapshot) map[string]nodeAggregateUsage {
	usage := make(map[string]nodeAggregateUsage)
	for _, vm := range snapshot.VMs {
		if vm.Node == "" || vm.Tags == "" {
			continue
		}
		hasPvmss := false
		tagParts := strings.Split(vm.Tags, ";")
		if len(tagParts) == 1 {
			tagParts = strings.Fields(vm.Tags)
		}
		for _, tag := range tagParts {
			if strings.EqualFold(strings.TrimSpace(tag), constants.RequiredTag) {
				hasPvmss = true
				break
			}
		}
		if !hasPvmss {
			continue
		}
		sockets := vm.Sockets
		if sockets <= 0 {
			sockets = 1
		}
		cores := vm.Cores
		if cores <= 0 {
			cores = 1
		}
		u := usage[vm.Node]
		u.totalVMs++
		u.cores += sockets * cores
		u.ramGB += int((vm.MemoryMB + 512) / 1024) //nolint:gosec
		usage[vm.Node] = u
	}
	return usage
}

// vmDiskCompatibleStorageTypes defines storage types that support VM disk images.
var vmDiskCompatibleStorageTypes = map[string]bool{
	"cifs":    true,
	"dir":     true,
	"iscsi":   true,
	"lvm":     true,
	"lvmthin": true,
	"nfs":     true,
	"rbd":     true,
	"zfs":     true,
}

// resolveStorages returns storage options from snapshot or live data.
func (h *VMCreateHandler) resolveStorages(_ context.Context, snapshot *state.ProxmoxClusterSnapshot, settings *state.AppSettings, disabledNodes map[string]bool) []VMCreateStorageOption {
	enabledSet := make(map[string]bool, len(settings.EnabledStorages))
	for _, s := range settings.EnabledStorages {
		enabledSet[s] = true
	}
	allowAll := len(settings.EnabledStorages) == 0

	for i, s := range settings.EnabledStorages {
		logger.Get().Debug().Int("index", i).Str("enabled_storage", s).Msg("resolveStorages: configured in settings")
	}

	nodeStoragesCount := 0
	globalStoragesCount := 0
	if snapshot != nil {
		nodeStoragesCount = len(snapshot.NodeStorages)
		globalStoragesCount = len(snapshot.GlobalStorages)
	}
	logger.Get().Debug().
		Int("enabled_storages_count", len(settings.EnabledStorages)).
		Bool("allow_all", allowAll).
		Int("node_storages_count", nodeStoragesCount).
		Int("global_storages_count", globalStoragesCount).
		Msg("resolveStorages: starting storage resolution")

	globalInfo := make(map[string]proxmox.Storage)
	if snapshot != nil {
		for _, st := range snapshot.GlobalStorages {
			globalInfo[st.Storage] = st
		}
	}

	storageMap := make(map[string]string)

	if snapshot != nil && len(snapshot.NodeStorages) > 0 {
		for nodeName, nodeStorages := range snapshot.NodeStorages {
			if disabledNodes[nodeName] {
				logger.Get().Debug().Str("node", nodeName).Msg("resolveStorages: skipping disabled node")
				continue
			}
			for _, storage := range nodeStorages {
				info := storage
				if global, exists := globalInfo[storage.Storage]; exists {
					if info.Content == "" && global.Content != "" {
						info.Content = global.Content
					}
					if info.Type == "" && global.Type != "" {
						info.Type = global.Type
					}
				}

				uniqueID := nodeName + ":" + storage.Storage
				isEnabled := allowAll || enabledSet[uniqueID]

				logger.Get().Debug().
					Str("node", nodeName).
					Str("storage", storage.Storage).
					Str("unique_id", uniqueID).
					Bool("is_enabled", isEnabled).
					Int("storage_enabled", storage.Enabled).
					Str("content", info.Content).
					Str("type", info.Type).
					Msg("resolveStorages: evaluating storage")

				if !isEnabled {
					continue
				}
				if storage.Enabled != 1 {
					continue
				}

				storageType := strings.ToLower(info.Type)
				storageContent := strings.ToLower(info.Content)
				supportsVMDisk := strings.Contains(storageContent, "images")
				if !supportsVMDisk {
					_, supportsVMDisk = vmDiskCompatibleStorageTypes[storageType]
				}
				if !supportsVMDisk {
					logger.Get().Debug().
						Str("storage", storage.Storage).
						Str("content", storageContent).
						Str("type", storageType).
						Msg("resolveStorages: storage rejected - does not support VM disk")
					continue
				}

				if _, exists := storageMap[storage.Storage]; !exists {
					if storageType == "rbd" || info.Shared == 1 {
						storageMap[storage.Storage] = ""
					} else {
						storageMap[storage.Storage] = nodeName
					}
					logger.Get().Debug().
						Str("storage", storage.Storage).
						Str("node", storageMap[storage.Storage]).
						Msg("resolveStorages: storage added")
				}
			}
		}
	} else {
		logger.Get().Debug().Msg("resolveStorages: using fallback from settings.EnabledStorages")
		for _, s := range settings.EnabledStorages {
			parts := strings.SplitN(s, ":", 2)
			if len(parts) == 2 {
				storageMap[parts[1]] = parts[0]
				logger.Get().Debug().Str("storage", parts[1]).Str("node", parts[0]).Msg("resolveStorages: added from settings")
			} else {
				storageMap[s] = ""
				logger.Get().Debug().Str("storage", s).Msg("resolveStorages: added shared from settings")
			}
		}
	}

	result := make([]VMCreateStorageOption, 0, len(storageMap))
	for name, node := range storageMap {
		result = append(result, VMCreateStorageOption{Name: name, Node: node})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })

	logger.Get().Info().
		Int("result_count", len(result)).
		Int("source_enabled_count", len(settings.EnabledStorages)).
		Int("source_node_storage_count", len(snapshot.NodeStorages)).
		Msg("resolveStorages: completed storage resolution")

	return result
}

// resolveBridges returns bridge options from snapshot or settings.
func (h *VMCreateHandler) resolveBridges(_ context.Context, snapshot *state.ProxmoxClusterSnapshot, settings *state.AppSettings, disabledNodes map[string]bool) []VMCreateBridgeOption {
	bridgeNodes := make(map[string]string)
	bridgeDescs := make(map[string]string)

	if snapshot != nil && len(snapshot.NetworkBridges) > 0 {
		for nodeName, vmbrs := range snapshot.NetworkBridges {
			if disabledNodes[nodeName] {
				continue
			}
			for _, vmbr := range vmbrs {
				name := extractVMBRIface(vmbr)
				if name == "" {
					continue
				}
				if _, exists := bridgeNodes[name]; !exists {
					bridgeNodes[name] = nodeName
				}
				if bridgeDescs[name] == "" {
					bridgeDescs[name] = strings.TrimSpace(vmbr.Comments)
				}
			}
		}
	}

	result := make([]VMCreateBridgeOption, 0, len(settings.VMBRs))
	for _, bridgeID := range settings.VMBRs {
		bridgeName := bridgeID
		if idx := strings.Index(bridgeID, ":"); idx != -1 {
			bridgeName = bridgeID[idx+1:]
		}
		result = append(result, VMCreateBridgeOption{
			Name:        bridgeName,
			Node:        bridgeNodes[bridgeName],
			Description: bridgeDescs[bridgeName],
		})
	}
	return result
}

// extractVMBRIface extracts the interface name from a VMBR struct.
func extractVMBRIface(vmbr proxmox.VMBR) string {
	return vmbr.Iface
}

// nodesFromSettings derives node list from settings when offline.
func (h *VMCreateHandler) nodesFromSettings(settings *state.AppSettings) []VMCreateNodeOption {
	nodeSet := make(map[string]bool)
	for _, s := range settings.EnabledStorages {
		parts := strings.SplitN(s, ":", 2)
		if len(parts) == 2 {
			nodeSet[parts[0]] = true
		}
	}
	for _, v := range settings.VMBRs {
		parts := strings.SplitN(v, ":", 2)
		if len(parts) == 2 {
			nodeSet[parts[0]] = true
		}
	}
	result := make([]VMCreateNodeOption, 0, len(nodeSet))
	for name := range nodeSet {
		result = append(result, VMCreateNodeOption{Name: name})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// storagesFromSettings derives storages from settings when offline.
func (h *VMCreateHandler) storagesFromSettings(settings *state.AppSettings) []VMCreateStorageOption {
	result := make([]VMCreateStorageOption, 0, len(settings.EnabledStorages))
	for _, s := range settings.EnabledStorages {
		parts := strings.SplitN(s, ":", 2)
		if len(parts) == 2 {
			result = append(result, VMCreateStorageOption{Name: parts[1], Node: parts[0]})
		} else {
			result = append(result, VMCreateStorageOption{Name: s})
		}
	}
	return result
}

// bridgesFromSettings derives bridges from settings when offline.
func (h *VMCreateHandler) bridgesFromSettings(settings *state.AppSettings) []VMCreateBridgeOption {
	result := make([]VMCreateBridgeOption, 0, len(settings.VMBRs))
	for _, v := range settings.VMBRs {
		bridgeName := v
		node := ""
		if idx := strings.Index(v, ":"); idx != -1 {
			node = v[:idx]
			bridgeName = v[idx+1:]
		}
		result = append(result, VMCreateBridgeOption{Name: bridgeName, Node: node})
	}
	return result
}
