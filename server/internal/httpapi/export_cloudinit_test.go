package httpapi

import "strings"

// CloudInitPaused exposes cloudInitPaused to the external test package.
const CloudInitPaused = cloudInitPaused

// IsPausedCloudInitRoute reports whether a "METHOD /path" pattern is left
// unregistered while cloud-init is paused. The VM cloud-init read stays.
func IsPausedCloudInitRoute(pattern string) bool {
	if !cloudInitPaused {
		return false
	}

	if strings.Contains(pattern, "/api/v1/admin/cloudinit-templates") || strings.Contains(pattern, "/api/v1/admin/baseline") {
		return true
	}

	return strings.Contains(pattern, "/cloudinit") && pattern != "GET /api/v1/vms/{cluster}/{vmid}/cloudinit"
}
