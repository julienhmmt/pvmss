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

type adminISODTO struct {
	Storage   string `json:"storage"`
	Node      string `json:"node"`
	File      string `json:"file"`
	SizeBytes int64  `json:"sizeBytes"`
	Enabled   bool   `json:"enabled"`
	Missing   bool   `json:"missing"`
}

// ServeISOs handles GET /api/v1/admin/isos.
//
//nolint:dupl // intentionally parallel to ServeBridges (same shape, different resource)
func (h *AdminCatalog) ServeISOs(w http.ResponseWriter, r *http.Request) {
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
	isos, err := catalog.AdminListISOs(r.Context(), h.store, client, clusterName)
	if err != nil {
		writeAdminFailure(w, "admin list isos failed", err)

		return
	}

	dto := make([]adminISODTO, len(isos))
	for i, iso := range isos {
		dto[i] = adminISODTO{
			Storage: iso.Storage, Node: iso.Node, File: iso.File,
			SizeBytes: iso.SizeBytes, Enabled: iso.Enabled, Missing: iso.Missing,
		}
	}

	writeAdminJSON(w, http.StatusOK, dto)
}

type isoToggleRequest struct {
	Cluster string `json:"cluster"`
	Node    string `json:"node"`
	Storage string `json:"storage"`
	File    string `json:"file"`
	Enabled bool   `json:"enabled"`
}

type isoToggleResponse struct {
	Node    string `json:"node"`
	Storage string `json:"storage"`
	File    string `json:"file"`
	Enabled bool   `json:"enabled"`
}

// ServeISOToggle handles POST /api/v1/admin/isos/toggle.
//
//nolint:dupl // intentionally parallel to ServeImageToggle (same shape, different resource)
func (h *AdminCatalog) ServeISOToggle(w http.ResponseWriter, r *http.Request) {
	var req isoToggleRequest
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
	err = catalog.SetISOEnabled(r.Context(), h.store, client, clusterName, catalog.ISORef{Node: req.Node, Storage: req.Storage, File: req.File}, req.Enabled)
	if errors.Is(err, cluster.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", isoNotFoundMsg(req.Node, req.Storage, req.File))
		return
	}

	if err != nil {
		writeAdminFailure(w, "admin toggle iso failed", err)

		return
	}

	h.recordAdminAction(r, "admin.isos.toggle", "iso", req.File,
		fmt.Sprintf("iso %s on storage %s node %s cluster %s set enabled=%v", req.File, req.Storage, req.Node, clusterName, req.Enabled),
		[]any{map[string]any{auditKeyCluster: clusterName, auditKeyNode: req.Node, auditKeyStorage: req.Storage, auditKeyFile: req.File, auditKeyEnabled: req.Enabled}})
	writeAdminJSON(w, http.StatusOK, isoToggleResponse{Node: req.Node, Storage: req.Storage, File: req.File, Enabled: req.Enabled})
}
