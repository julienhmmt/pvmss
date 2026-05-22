package apiv1

import (
	"pvmss/state"
)

// AdminMutationsHandler handles admin write operations. The endpoint
// implementations live in resource-scoped sibling files in this package:
//
//   - admin_pools.go      — user pools
//   - admin_tags.go       — tag CRUD + colors
//   - admin_limits.go     — VM / node resource limits
//   - admin_cloudinit.go  — cloud-init templates + SFTP toggle
//   - admin_toggles.go    — storage / vmbr / iso toggles
//   - admin_profiles.go   — VM profiles
type AdminMutationsHandler struct {
	state state.StateManager
}

// MakeAdminMutationsHandler creates a new AdminMutationsHandler.
func MakeAdminMutationsHandler(s state.StateManager) *AdminMutationsHandler {
	return &AdminMutationsHandler{state: s}
}

// copyVMProfiles deep-copies a profile slice to avoid shared backing array.
func copyVMProfiles(src []state.VMProfileConfig) []state.VMProfileConfig {
	dst := make([]state.VMProfileConfig, len(src))
	copy(dst, src)
	return dst
}

// copyNodeLimits deep-copies the node limits map to avoid shared references.
func copyNodeLimits(src map[string]state.NodeResourceLimits) map[string]state.NodeResourceLimits {
	if src == nil {
		return make(map[string]state.NodeResourceLimits)
	}
	dst := make(map[string]state.NodeResourceLimits, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
