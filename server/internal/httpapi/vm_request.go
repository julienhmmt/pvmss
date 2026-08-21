package httpapi

import (
	"net/http"
	"pvmss/server/internal/auth"
	"strconv"
)

// parseVMRequestTarget authenticates the caller and parses the {cluster}/
// {vmid} path segments shared by every /api/v1/vms/{cluster}/{vmid}/...
// read handler. Extracted once VMSnapshots and VMMetrics both needed the
// exact same four lines (golangci-lint dupl).
func parseVMRequestTarget(authHandler *Auth, r *http.Request, writeErr func(status int, code, message string)) (auth.Identity, string, int, bool) {
	identity, err := authHandler.Principal(r)
	if err != nil {
		writeErr(http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return auth.Identity{}, "", 0, false
	}

	clusterName := r.PathValue("cluster")

	vmid, err := strconv.Atoi(r.PathValue("vmid"))
	if clusterName == "" || err != nil || vmid < 1 {
		writeErr(http.StatusBadRequest, "invalid_request", msgInvalidVMPath)
		return auth.Identity{}, "", 0, false
	}

	return identity, clusterName, vmid, true
}
