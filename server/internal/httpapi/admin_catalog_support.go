//nolint:wsl_v5 // parallel catalog handlers keep validation and contract mapping adjacent
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"pvmss/server/internal/cluster"
)

func (h *AdminCatalog) clientFor(name string) (cluster.Client, error) {
	if h.clients == nil {
		if h.client == nil {
			return nil, cluster.ErrClusterNotFound
		}
		return h.client, nil
	}
	return h.clients.Client(name)
}

// SetTrustedProxyHops configures how many X-Forwarded-For hops are trusted
// when extracting the client IP for audit entries.
func (h *AdminCatalog) SetTrustedProxyHops(n int) {
	h.trustedProxyHops = n
}

// recordAdminAction writes one admin audit row for a catalog mutation. It
// never fails the request - a failed audit write is logged and ignored.
func (h *AdminCatalog) recordAdminAction(r *http.Request, action, targetType, targetID, summary string, changes []any) {
	actor, err := h.auth.Principal(r)
	if err != nil {
		return
	}

	_ = h.store.RecordAdminAction(r.Context(), actor.Username, action, targetType, targetID, detailJSON(summary, changes), clientIP(r, h.trustedProxyHops))
}

func queryCluster(r *http.Request) string {
	c := r.URL.Query().Get("cluster")
	if c == "" {
		return ""
	}

	return c
}

func writeAdminJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}

	_ = writeJSON(w, status, body)
}

// writeAdminFailure answers an admin catalog failure: an unreachable or
// TLS-failing cluster is 503 cluster_unavailable (the admin can act on it),
// anything else 500. The cause goes to the access log either way.
func writeAdminFailure(w http.ResponseWriter, message string, err error) {
	SetErrorMsg(w, message, err)

	if errors.Is(err, cluster.ErrUnreachable) || errors.Is(err, cluster.ErrTLSVerify) {
		writeAdminError(w, http.StatusServiceUnavailable, "cluster_unavailable", msgClusterUnavailable)
		return
	}

	writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
}

func writeAdminError(w http.ResponseWriter, status int, code, message string) {
	_ = writeClusterError(w, status, code, message)
}

func nodeNotFoundMsg(name string) string {
	return "node \"" + name + "\"" + msgNotReportedByCluster
}

func storageNotFoundMsg(name, node string) string {
	return "storage \"" + name + msgOnNode + node + "\"" + msgNotReportedByCluster
}

func bridgeNotFoundMsg(node, name string) string {
	return "bridge \"" + name + msgOnNode + node + "\"" + msgNotReportedByCluster
}

func isoNotFoundMsg(node, storage, file string) string {
	return "iso \"" + file + "\" on storage \"" + storage + msgOnNode + node + "\"" + msgNotReportedByCluster
}

func imageNotFoundMsg(node, storage, file string) string {
	return "image \"" + file + "\" on storage \"" + storage + msgOnNode + node + "\"" + msgNotReportedByCluster
}
