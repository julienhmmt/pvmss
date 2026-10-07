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

type adminBridgeDTO struct {
	Name    string `json:"name"`
	Node    string `json:"node"`
	Active  bool   `json:"active"`
	Comment string `json:"comment"`
	Enabled bool   `json:"enabled"`
	Missing bool   `json:"missing"`
}

// ServeBridges handles GET /api/v1/admin/bridges.
//
//nolint:dupl // intentionally parallel to ServeISOs (same shape, different resource)
func (h *AdminCatalog) ServeBridges(w http.ResponseWriter, r *http.Request) {
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
	bridges, err := catalog.AdminListBridges(r.Context(), h.store, client, clusterName)
	if err != nil {
		writeAdminFailure(w, "admin list bridges failed", err)

		return
	}

	dto := make([]adminBridgeDTO, len(bridges))
	for i, b := range bridges {
		dto[i] = adminBridgeDTO{
			Name: b.Name, Node: b.Node, Active: b.Active,
			Comment: b.Comment, Enabled: b.Enabled, Missing: b.Missing,
		}
	}

	writeAdminJSON(w, http.StatusOK, dto)
}

type bridgeToggleRequest struct {
	Cluster string `json:"cluster"`
	Node    string `json:"node"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type bridgeToggleResponse struct {
	Node    string `json:"node"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// ServeBridgeToggle handles POST /api/v1/admin/bridges/toggle.
func (h *AdminCatalog) ServeBridgeToggle(w http.ResponseWriter, r *http.Request) {
	var req bridgeToggleRequest
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
	err = catalog.SetBridgeEnabled(r.Context(), h.store, client, clusterName, req.Node, req.Name, req.Enabled)
	if errors.Is(err, cluster.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", bridgeNotFoundMsg(req.Node, req.Name))
		return
	}

	if err != nil {
		writeAdminFailure(w, "admin toggle bridge failed", err)

		return
	}

	h.recordAdminAction(r, "admin.bridges.toggle", "bridge", req.Name,
		fmt.Sprintf("bridge %s on node %s cluster %s set enabled=%v", req.Name, req.Node, clusterName, req.Enabled),
		[]any{map[string]any{auditKeyCluster: clusterName, auditKeyName: req.Name, auditKeyNode: req.Node, auditKeyEnabled: req.Enabled}})
	writeAdminJSON(w, http.StatusOK, bridgeToggleResponse{Node: req.Node, Name: req.Name, Enabled: req.Enabled})
}

//  - ISOs -
