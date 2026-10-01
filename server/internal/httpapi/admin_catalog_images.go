//nolint:wsl_v5 // parallel catalog handlers keep validation and contract mapping adjacent
package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"strings"
)

type adminImageDTO struct {
	Storage   string `json:"storage"`
	Node      string `json:"node"`
	File      string `json:"file"`
	SizeBytes int64  `json:"sizeBytes"`
	Enabled   bool   `json:"enabled"`
	Missing   bool   `json:"missing"`
}

// ServeImages handles GET /api/v1/admin/images.
//
//nolint:dupl // intentionally parallel to ServeISOs (same shape, different resource)
func (h *AdminCatalog) ServeImages(w http.ResponseWriter, r *http.Request) {
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
	images, err := catalog.AdminListImages(r.Context(), h.store, client, clusterName)
	if err != nil {
		SetErrorMsg(w, "admin list images failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	dto := make([]adminImageDTO, len(images))
	for i, image := range images {
		dto[i] = adminImageDTO{
			Storage: image.Storage, Node: image.Node, File: image.File,
			SizeBytes: image.SizeBytes, Enabled: image.Enabled, Missing: image.Missing,
		}
	}

	writeAdminJSON(w, http.StatusOK, dto)
}

type imageToggleRequest struct {
	Cluster string `json:"cluster"`
	Node    string `json:"node"`
	Storage string `json:"storage"`
	File    string `json:"file"`
	Enabled bool   `json:"enabled"`
}

type imageToggleResponse struct {
	Node    string `json:"node"`
	Storage string `json:"storage"`
	File    string `json:"file"`
	Enabled bool   `json:"enabled"`
}

// ServeImageToggle handles POST /api/v1/admin/images/toggle.
//
//nolint:dupl // intentionally parallel to ServeISOToggle (same shape, different resource)
func (h *AdminCatalog) ServeImageToggle(w http.ResponseWriter, r *http.Request) {
	var req imageToggleRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}
	if strings.TrimSpace(req.Node) == "" {
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
	err = catalog.SetImageEnabled(r.Context(), h.store, client, clusterName, catalog.ImageRef{Node: req.Node, Storage: req.Storage, File: req.File}, req.Enabled)
	if errors.Is(err, cluster.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", imageNotFoundMsg(req.Node, req.Storage, req.File))
		return
	}

	if err != nil {
		SetErrorMsg(w, "admin toggle image failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	h.recordAdminAction(r, "admin.images.toggle", "image", req.File,
		fmt.Sprintf("image %s on storage %s node %s cluster %s set enabled=%v", req.File, req.Storage, req.Node, clusterName, req.Enabled),
		[]any{map[string]any{auditKeyCluster: clusterName, auditKeyNode: req.Node, auditKeyStorage: req.Storage, auditKeyFile: req.File, auditKeyEnabled: req.Enabled}})
	writeAdminJSON(w, http.StatusOK, imageToggleResponse{Node: req.Node, Storage: req.Storage, File: req.File, Enabled: req.Enabled})
}

// ServeImageDelete handles DELETE /api/v1/admin/images/{cluster}/{node}/{storage}/{file}:
// removes an orphan cloud image approval row.
//
//nolint:dupl // intentionally parallel to ServeISODelete (same shape, different resource)
func (h *AdminCatalog) ServeImageDelete(w http.ResponseWriter, r *http.Request) {
	clusterName := r.PathValue("cluster")
	node := r.PathValue("node")
	storage := r.PathValue("storage")
	file := r.PathValue("file")
	if node == "" || storage == "" || file == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "node, storage, and file are required")
		return
	}

	err := catalog.DeleteImage(r.Context(), h.store, clusterName, node, storage, file)
	if errors.Is(err, catalog.ErrImageNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", fmt.Sprintf("image %q on storage %q node %q not found on cluster %q", file, storage, node, clusterName))
		return
	}

	if err != nil {
		SetErrorMsg(w, "admin delete image failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}

	h.recordAdminAction(r, "admin.images.delete", "image", file,
		fmt.Sprintf("deleted image approval %q on storage %s node %s cluster %s", file, storage, node, clusterName),
		[]any{map[string]any{auditKeyCluster: clusterName, auditKeyNode: node, auditKeyStorage: storage, auditKeyFile: file}})
	w.WriteHeader(http.StatusNoContent)
}

// ServeISODelete handles DELETE /api/v1/admin/isos/{cluster}/{node}/{storage}/{file}:
// removes an orphan ISO approval row.
//
//nolint:dupl // intentionally parallel to ServeImageDelete (same shape, different resource)
func (h *AdminCatalog) ServeISODelete(w http.ResponseWriter, r *http.Request) {
	clusterName := r.PathValue("cluster")
	node := r.PathValue("node")
	storage := r.PathValue("storage")
	file := r.PathValue("file")
	if node == "" || storage == "" || file == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "node, storage, and file are required")
		return
	}

	err := catalog.DeleteISO(r.Context(), h.store, clusterName, node, storage, file)
	if errors.Is(err, catalog.ErrISONotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", fmt.Sprintf("iso %q on storage %q node %q not found on cluster %q", file, storage, node, clusterName))
		return
	}

	if err != nil {
		SetErrorMsg(w, "admin delete iso failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}

	h.recordAdminAction(r, "admin.isos.delete", "iso", file,
		fmt.Sprintf("deleted iso approval %q on storage %s node %s cluster %s", file, storage, node, clusterName),
		[]any{map[string]any{auditKeyCluster: clusterName, auditKeyNode: node, auditKeyStorage: storage, auditKeyFile: file}})
	w.WriteHeader(http.StatusNoContent)
}
