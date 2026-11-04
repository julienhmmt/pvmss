package cluster

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var errNoCPUFlags = errors.New("no cpu flags reported")

// HostCPUFlags implements Creator: one cpuinfo flag list per online node,
// read from GET /nodes/{node}/status (Sys.Audit on /nodes/{node}). Offline
// nodes are skipped; any failed node read fails the whole call.
func (p Proxmox) HostCPUFlags(ctx context.Context) ([][]string, error) {
	nodes, err := proxmoxNodes(ctx, p.rest())
	if err != nil {
		return nil, fmt.Errorf("list nodes for cpu flags: %w", err)
	}

	flags := make([][]string, 0, len(nodes))
	for _, node := range nodes {
		if !proxmoxNodeOnline(node.Status) {
			continue
		}

		status, err := readProxmoxNodeData[proxmoxNodeHealthResponse](ctx, p, node.Name, "status")
		if err != nil {
			return nil, fmt.Errorf("read node %q cpu flags: %w", node.Name, err)
		}

		nodeFlags := strings.Fields(status.CPUInfo.Flags)
		if len(nodeFlags) == 0 {
			// An error, not an empty list, so the caller's fallback logs a Warn.
			return nil, fmt.Errorf("read node %q cpu flags: %w", node.Name, errNoCPUFlags)
		}

		flags = append(flags, nodeFlags)
	}

	if len(flags) == 0 {
		return nil, fmt.Errorf("read cpu flags: %w", errNoCPUFlags)
	}

	return flags, nil
}
