package cluster

import (
	"context"
	"strings"
)

// HostCPUFlags implements Creator: every online fake node reports the full
// x86-64-v3 flag set, so the fake cluster selects CPUTypeV3.
func (fake Fake) HostCPUFlags(_ context.Context) ([][]string, error) {
	if fake.unavailable() {
		return nil, ErrUnreachable
	}

	state := fake.stateOrDefault()

	state.vmMu.RLock()
	defer state.vmMu.RUnlock()

	flags := make([][]string, 0, len(state.nodes))
	for _, node := range state.nodes {
		if node.Status == NodeOnline {
			flags = append(flags, strings.Fields(cpuFlagsV3))
		}
	}

	return flags, nil
}
