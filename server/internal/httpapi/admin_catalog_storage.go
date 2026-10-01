//nolint:wsl_v5 // parallel catalog handlers keep validation and contract mapping adjacent
package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
)

type adminStorageDTO struct {
	Name    string `json:"name"`
	Node    string `json:"node"`
	Type    string `json:"type"`
	Total   int64  `json:"totalBytes"`
	Used    int64  `json:"usedBytes"`
	Enabled bool   `json:"enabled"`
	Missing bool   `json:"missing"`
}

// ServeStorages handles GET /api/v1/admin/storages.
func (h *AdminCatalog) ServeStorages(w http.ResponseWriter, r *http.Request) {
	clusterName, clusterErr := ResolveClusterParam(r, h.clusters)
	if clusterErr != nil {
		code, message := clusterParamError(clusterErr)
		writeAdminError(w, http.StatusBadRequest, code, message)
		return
	}

	client, err := h.clientFor(clusterName)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "not_found", msgClusterNotFound)
		return
	}
	storages, err := catalog.AdminListStorages(r.Context(), h.store, client, clusterName)
	if err != nil {
		SetErrorMsg(w, "admin list storages failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	dto := make([]adminStorageDTO, len(storages))
	for i, s := range storages {
		dto[i] = adminStorageDTO{
			Name: s.Name, Node: s.Node, Type: s.Type,
			Total: s.Total, Used: s.Used, Enabled: s.Enabled, Missing: s.Missing,
		}
	}

	writeAdminJSON(w, http.StatusOK, dto)
}

// snippetStorageDTO is one snippet-capable storage row returned by
// ServeSnippetStorages. The admin cluster form uses this list to populate the
// snippet storage picker so administrators select a compatible storage instead
// of typing a name that may not have the snippets content type enabled.
type snippetStorageDTO struct {
	Name string `json:"name"`
	Node string `json:"node"`
	Type string `json:"type"`
}

// ServeSnippetStorages handles GET /api/v1/admin/snippet-storages?cluster=<name>.
// It returns every storage the cluster reports with the snippets content type
// enabled, deduplicated by name (a shared storage visible on two nodes appears
// once). The admin cluster form uses this list to populate the snippet storage
// picker.
func (h *AdminCatalog) ServeSnippetStorages(w http.ResponseWriter, r *http.Request) {
	clusterName, clusterErr := ResolveClusterParam(r, h.clusters)
	if clusterErr != nil {
		code, message := clusterParamError(clusterErr)
		writeAdminError(w, http.StatusBadRequest, code, message)
		return
	}

	client, err := h.clientFor(clusterName)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "not_found", msgClusterNotFound)
		return
	}

	snap, err := client.Snapshot(r.Context())
	if err != nil {
		SetErrorMsg(w, "admin list snippet storages failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	seen := make(map[string]bool, len(snap.Storages))
	dto := make([]snippetStorageDTO, 0, len(snap.Storages))
	for _, s := range snap.Storages {
		if !cluster.IsSnippetCapableStorage(s) {
			continue
		}

		if seen[s.Name] {
			continue
		}

		seen[s.Name] = true
		dto = append(dto, snippetStorageDTO{Name: s.Name, Node: s.Node, Type: s.Type})
	}

	writeAdminJSON(w, http.StatusOK, dto)
}

type storageToggleRequest struct {
	Cluster string `json:"cluster"`
	Name    string `json:"name"`
	Node    string `json:"node"`
	Enabled bool   `json:"enabled"`
}

type storageToggleResponse struct {
	Name    string `json:"name"`
	Node    string `json:"node"`
	Enabled bool   `json:"enabled"`
}

// ServeStorageToggle handles POST /api/v1/admin/storages/toggle.
func (h *AdminCatalog) ServeStorageToggle(w http.ResponseWriter, r *http.Request) {
	var req storageToggleRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}

	clusterName, clusterErr := ResolveClusterValue(req.Cluster, h.clusters)
	if clusterErr != nil {
		code, message := clusterParamError(clusterErr)
		writeAdminError(w, http.StatusBadRequest, code, message)
		return
	}

	client, err := h.clientFor(clusterName)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "not_found", msgClusterNotFound)
		return
	}
	err = catalog.SetStorageEnabled(r.Context(), h.store, client, clusterName, req.Name, req.Node, req.Enabled)
	if errors.Is(err, cluster.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", storageNotFoundMsg(req.Name, req.Node))
		return
	}

	if err != nil {
		SetErrorMsg(w, "admin toggle storage failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	h.recordAdminAction(r, "admin.storages.toggle", "storage", req.Name,
		fmt.Sprintf("storage %s on node %s cluster %s set enabled=%v", req.Name, req.Node, clusterName, req.Enabled),
		[]any{map[string]any{auditKeyCluster: clusterName, auditKeyName: req.Name, auditKeyNode: req.Node, auditKeyEnabled: req.Enabled}})
	writeAdminJSON(w, http.StatusOK, storageToggleResponse{Name: req.Name, Node: req.Node, Enabled: req.Enabled})
}

//  - Bridges -
