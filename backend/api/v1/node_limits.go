package apiv1

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"pvmss/constants"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

const (
	liveNodeUsageTimeout = 10 * time.Second
	nodeReservationTTL   = 10 * time.Minute
)

// nodeResourceUsage is the aggregate footprint of pvmss-tagged VMs on a node.
type nodeResourceUsage struct {
	Cores    int
	RamGB    int
	MaxCores int
	MaxRamGB int
}

type nodeResourceReservation struct {
	Cores int
	RamGB int
	Until time.Time
}

type nodeLimitError struct {
	code    string
	message string
}

func (e nodeLimitError) Error() string {
	return e.message
}

var nodeCreateLimitMu sync.Mutex
var nodeReservations = map[string][]nodeResourceReservation{}

func nodeLimitCode(err error) string {
	var limitErr nodeLimitError
	if errors.As(err, &limitErr) {
		return limitErr.code
	}
	return "node_limit_exceeded"
}

// validateNodeAggregateLimits checks that adding a VM with the given resources
// would not push the target node past its configured aggregate cores/RAM caps.
// Returns nil when no enforcement applies (node missing or no caps).
func validateNodeAggregateLimits(sm state.StateManager, node string, sockets, cores, memoryMB int) error {
	nodeCreateLimitMu.Lock()
	defer nodeCreateLimitMu.Unlock()

	settings := sm.GetSettings()
	if settings == nil {
		return nil
	}
	nodeLimits, ok := settings.Limits.Nodes[node]
	if !ok || (nodeLimits.Cores.Max == 0 && nodeLimits.RAM.Max == 0) {
		return nil
	}

	usage, err := fetchLiveNodeUsage(settings)
	if err != nil {
		logger.Get().Warn().Err(err).Str("node", node).Msg("Live node usage unavailable; falling back to cached snapshot for degraded validation")
		snapshot := sm.GetProxmoxSnapshot()
		if snapshot == nil {
			return nodeLimitError{
				code:    "node_limit_degraded",
				message: fmt.Sprintf("node %q aggregate limits cannot be safely evaluated because live Proxmox usage and cached snapshot are unavailable", node),
			}
		}
		usage = computeNodeUsage(snapshot, settings)
	}

	nodeUsage := usage[node]
	nodeUsage.MaxCores = nodeLimits.Cores.Max
	nodeUsage.MaxRamGB = nodeLimits.RAM.Max
	nodeUsage = addActiveNodeReservations(nodeUsage, node)

	requestedCores := sockets * cores
	requestedRamGB := mbToRoundedGB(int64(memoryMB))

	if nodeUsage.MaxCores > 0 && nodeUsage.Cores+requestedCores > nodeUsage.MaxCores {
		return fmt.Errorf("adding this VM would exceed node %q aggregate cores limit (current: %d, requested: %d, max: %d)",
			node, nodeUsage.Cores, requestedCores, nodeUsage.MaxCores)
	}
	if nodeUsage.MaxRamGB > 0 && nodeUsage.RamGB+requestedRamGB > nodeUsage.MaxRamGB {
		return fmt.Errorf("adding this VM would exceed node %q aggregate RAM limit (current: %d GB, requested: %d GB, max: %d GB)",
			node, nodeUsage.RamGB, requestedRamGB, nodeUsage.MaxRamGB)
	}
	nodeReservations[node] = append(nodeReservations[node], nodeResourceReservation{
		Cores: requestedCores,
		RamGB: requestedRamGB,
		Until: time.Now().Add(nodeReservationTTL),
	})
	return nil
}

func releaseNodeAggregateReservation(node string, sockets, cores, memoryMB int) {
	nodeCreateLimitMu.Lock()
	defer nodeCreateLimitMu.Unlock()
	reservations := nodeReservations[node]
	if len(reservations) == 0 {
		return
	}
	requestedCores := sockets * cores
	requestedRamGB := mbToRoundedGB(int64(memoryMB))
	for idx, reservation := range reservations {
		if reservation.Cores == requestedCores && reservation.RamGB == requestedRamGB {
			nodeReservations[node] = append(reservations[:idx], reservations[idx+1:]...)
			return
		}
	}
}

func addActiveNodeReservations(usage nodeResourceUsage, node string) nodeResourceUsage {
	now := time.Now()
	active := nodeReservations[node][:0]
	for _, reservation := range nodeReservations[node] {
		if reservation.Until.Before(now) {
			continue
		}
		usage.Cores += reservation.Cores
		usage.RamGB += reservation.RamGB
		active = append(active, reservation)
	}
	if len(active) == 0 {
		delete(nodeReservations, node)
		return usage
	}
	nodeReservations[node] = active
	return usage
}

func fetchLiveNodeUsage(settings *state.AppSettings) (map[string]nodeResourceUsage, error) {
	client, err := proxmox.MakeRestyClientFromEnv(liveNodeUsageTimeout)
	if err != nil {
		return nil, fmt.Errorf("create Proxmox client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), liveNodeUsageTimeout)
	defer cancel()
	vms, err := proxmox.GetVMsRestyFresh(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("fetch VM list: %w", err)
	}
	return computeNodeUsageFromVMs(vms, settings), nil
}

// computeNodeUsage aggregates cores + RAM for pvmss-tagged VMs on each node.
func computeNodeUsage(snapshot *state.ProxmoxClusterSnapshot, settings *state.AppSettings) map[string]nodeResourceUsage {
	usage := make(map[string]nodeResourceUsage, len(snapshot.NodeNames))
	for _, node := range snapshot.NodeNames {
		if node != "" {
			usage[node] = nodeResourceUsage{}
		}
	}

	for _, vm := range snapshot.VMs {
		if vm.Node == "" || !hasPvmssTag(vm.Tags) {
			continue
		}
		entry := usage[vm.Node]
		s := vm.Sockets
		if s <= 0 {
			s = 1
		}
		c := vm.Cores
		if c <= 0 {
			c = 1
		}
		entry.Cores += s * c
		if vm.MemoryMB > 0 {
			entry.RamGB += mbToRoundedGB(vm.MemoryMB)
		}
		usage[vm.Node] = entry
	}

	if settings != nil {
		for node, entry := range usage {
			if nl, ok := settings.Limits.Nodes[node]; ok {
				entry.MaxCores = nl.Cores.Max
				entry.MaxRamGB = nl.RAM.Max
				usage[node] = entry
			}
		}
	}

	return usage
}

func computeNodeUsageFromVMs(vms []proxmox.VM, settings *state.AppSettings) map[string]nodeResourceUsage {
	usage := make(map[string]nodeResourceUsage)
	for _, vm := range vms {
		if vm.Node == "" || !hasPvmssTag(vm.Tags) {
			continue
		}
		entry := usage[vm.Node]
		cores := vm.CPUs
		if cores <= 0 {
			cores = 1
		}
		entry.Cores += cores
		if vm.MaxMem > 0 {
			entry.RamGB += mbToRoundedGB(vm.MaxMem / (1024 * 1024))
		}
		usage[vm.Node] = entry
	}
	applyNodeLimits(usage, settings)
	return usage
}

func applyNodeLimits(usage map[string]nodeResourceUsage, settings *state.AppSettings) {
	if settings == nil {
		return
	}
	for node, entry := range usage {
		if nl, ok := settings.Limits.Nodes[node]; ok {
			entry.MaxCores = nl.Cores.Max
			entry.MaxRamGB = nl.RAM.Max
			usage[node] = entry
		}
	}
}

func hasPvmssTag(tagsStr string) bool {
	if tagsStr == "" {
		return false
	}
	for _, part := range strings.Split(tagsStr, ";") {
		for _, t := range strings.Split(part, ",") {
			if strings.EqualFold(strings.TrimSpace(t), constants.RequiredTag) {
				return true
			}
		}
	}
	return false
}

func mbToRoundedGB(mb int64) int {
	return int((mb + 512) / 1024)
}
