package httpapi

import (
	"errors"
	"net/http"
	"pvmss/server/internal/cluster"
	"strings"
)

// clusterRejectionResponse maps a cluster.ErrClusterRejected (wrapped by
// cluster.RejectionError) to a stable machine code and the message to
// surface. ok=false when err is not a cluster rejection — callers keep their
// existing typed-error cases and only add one branch for this.
//
// The machine code lets the frontend act on the failure (retry on vm_locked,
// explain snapshot_storage_unsupported) and render its own i18n text; the raw
// Proxmox message is the fallback content (ADR 0002). For 401/403 the message
// is suppressed — a PVE auth error body can name the API token.
func clusterRejectionResponse(err error) (code, message string, ok bool) {
	var rejection *cluster.RejectionError
	if !errors.As(err, &rejection) {
		return "", "", false
	}

	if rejection.Status == http.StatusUnauthorized || rejection.Status == http.StatusForbidden {
		return "cluster_rejected", msgClusterRejected, true
	}

	return clusterRejectionCode(rejection.Message), rejection.Message, true
}

// clusterRejectionCode derives a stable machine code from Proxmox's own
// message text. The matching is deliberately loose — codes are hints; the
// raw message is the content.
func clusterRejectionCode(message string) string {
	lower := strings.ToLower(message)

	switch {
	case strings.Contains(lower, "not supported") || strings.Contains(lower, "not available"):
		return "snapshot_storage_unsupported"
	case strings.Contains(lower, "lock"):
		return "vm_locked"
	case strings.Contains(lower, "already"):
		return "snapshot_name_exists"
	default:
		return "cluster_rejected"
	}
}
