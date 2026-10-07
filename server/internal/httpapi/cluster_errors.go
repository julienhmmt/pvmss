package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cluster"
	"regexp"
	"strings"
)

// permissionCheckFailed matches Proxmox's missing-privilege 403 body,
// "Permission check failed (<path>, <privilege>)". Path and privilege name
// the ACL, never the token, so they are safe to show the admin.
var permissionCheckFailed = regexp.MustCompile(`Permission check failed \(([^,()]+), ([^()]+)\)`)

// clusterRejectionResponse maps a cluster.ErrClusterRejected (wrapped by
// cluster.RejectionError) to a stable machine code and the message to
// surface. ok=false when err is not a cluster rejection - callers keep their
// existing typed-error cases and only add one branch for this.
//
// The machine code lets the frontend act on the failure (retry on vm_locked,
// explain snapshot_storage_unsupported) and render its own i18n text; the raw
// Proxmox message is the fallback content (ADR 0002). For 401/403 the message
// is suppressed - a PVE auth error body can name the API token - except a
// missing privilege, which is named so the admin knows what to grant.
// The caller records err on the access-log line (log once, where handled).
func clusterRejectionResponse(err error) (clusterErrorEnvelope, bool) {
	var rejection *cluster.RejectionError

	if !errors.As(err, &rejection) {
		return clusterErrorEnvelope{Code: "cluster_rejected", Message: msgClusterRejected}, errors.Is(err, cluster.ErrClusterRejected)
	}

	if match := permissionCheckFailed.FindStringSubmatch(rejection.Message); rejection.Status == http.StatusForbidden && match != nil {
		privilege, path := strings.TrimSpace(match[2]), strings.TrimSpace(match[1])

		return clusterErrorEnvelope{
			Code: "cluster_permission_denied", Privilege: privilege, Path: path,
			Message: fmt.Sprintf("the PVMSS service token lacks the Proxmox privilege %s on %s", privilege, path),
		}, true
	}

	if rejection.Status == http.StatusUnauthorized || rejection.Status == http.StatusForbidden {
		return clusterErrorEnvelope{Code: "cluster_rejected", Message: msgClusterRejected}, true
	}

	return clusterErrorEnvelope{Code: clusterRejectionCode(rejection.Message), Message: rejection.Message}, true
}

func writeClusterRejection(w http.ResponseWriter, err error, log *slog.Logger) bool {
	response, ok := clusterRejectionResponse(err)
	if !ok {
		return false
	}

	SetError(w, err)

	body, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		SetErrorMsg(w, "failed to marshal cluster rejection", marshalErr)
		return true
	}

	if writeErr := writeJSON(w, http.StatusBadGateway, body); writeErr != nil {
		log.Warn("failed to write cluster rejection", "component", "httpapi", "error", writeErr)
	}

	return true
}

// clusterRejectionCode derives a stable machine code from Proxmox's own
// message text. The matching is deliberately loose - codes are hints; the
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
