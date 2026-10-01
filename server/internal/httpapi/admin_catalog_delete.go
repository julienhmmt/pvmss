//nolint:wsl_v5 // parallel catalog handlers keep validation and contract mapping adjacent
package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"pvmss/server/internal/catalog"
)

// ServeNodeDelete handles DELETE /api/v1/admin/nodes/{cluster}/{name}: removes
// an orphan node approval row. The UI offers Remove only on missing rows, but
// the API deletes any approval.
func (h *AdminCatalog) ServeNodeDelete(w http.ResponseWriter, r *http.Request) {
	clusterName := r.PathValue("cluster")
	name := r.PathValue("name")
	if name == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "node name is required")
		return
	}

	err := catalog.DeleteNode(r.Context(), h.store, clusterName, name)
	if errors.Is(err, catalog.ErrNodeNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", fmt.Sprintf("node %q not found on cluster %q", name, clusterName))
		return
	}

	if err != nil {
		SetErrorMsg(w, "admin delete node failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}

	h.recordAdminAction(r, "admin.nodes.delete", "node", name,
		fmt.Sprintf("deleted node approval %q on cluster %s", name, clusterName),
		[]any{map[string]any{auditKeyCluster: clusterName, auditKeyName: name}})
	w.WriteHeader(http.StatusNoContent)
}

// ServeStorageDelete handles DELETE /api/v1/admin/storages/{cluster}/{node}/{name}:
// removes an orphan storage approval row.
//
//nolint:dupl // intentionally parallel to ServeBridgeDelete (same shape, different resource)
func (h *AdminCatalog) ServeStorageDelete(w http.ResponseWriter, r *http.Request) {
	clusterName := r.PathValue("cluster")
	node := r.PathValue("node")
	name := r.PathValue("name")
	if name == "" || node == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "storage name and node are required")
		return
	}

	err := catalog.DeleteStorage(r.Context(), h.store, clusterName, name, node)
	if errors.Is(err, catalog.ErrStorageNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", fmt.Sprintf("storage %q on node %q not found on cluster %q", name, node, clusterName))
		return
	}

	if err != nil {
		SetErrorMsg(w, "admin delete storage failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}

	h.recordAdminAction(r, "admin.storages.delete", "storage", name,
		fmt.Sprintf("deleted storage approval %q on node %s cluster %s", name, node, clusterName),
		[]any{map[string]any{auditKeyCluster: clusterName, auditKeyName: name, auditKeyNode: node}})
	w.WriteHeader(http.StatusNoContent)
}

// ServeBridgeDelete handles DELETE /api/v1/admin/bridges/{cluster}/{node}/{name}:
// removes an orphan bridge approval row.
//
//nolint:dupl // intentionally parallel to ServeStorageDelete (same shape, different resource)
func (h *AdminCatalog) ServeBridgeDelete(w http.ResponseWriter, r *http.Request) {
	clusterName := r.PathValue("cluster")
	node := r.PathValue("node")
	name := r.PathValue("name")
	if name == "" || node == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "bridge name and node are required")
		return
	}

	err := catalog.DeleteBridge(r.Context(), h.store, clusterName, node, name)
	if errors.Is(err, catalog.ErrBridgeNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", fmt.Sprintf("bridge %q on node %q not found on cluster %q", name, node, clusterName))
		return
	}

	if err != nil {
		SetErrorMsg(w, "admin delete bridge failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}

	h.recordAdminAction(r, "admin.bridges.delete", "bridge", name,
		fmt.Sprintf("deleted bridge approval %q on node %s cluster %s", name, node, clusterName),
		[]any{map[string]any{auditKeyCluster: clusterName, auditKeyName: name, auditKeyNode: node}})
	w.WriteHeader(http.StatusNoContent)
}

//  - Images (cloud images) -
