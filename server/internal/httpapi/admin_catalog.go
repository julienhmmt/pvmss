//nolint:wsl_v5 // parallel catalog handlers keep validation and contract mapping adjacent
package httpapi

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
)

// AdminCatalog serves admin catalog and node-detail endpoints. Every route is wrapped by Auth.RequireAdmin.
type AdminCatalog struct {
	auth             *Auth
	store            *store.Store
	client           cluster.Client
	projection       *inventory.Projection
	inventory        dashboardInventory
	clusters         ClusterLister
	clients          cluster.ClientProvider
	log              *slog.Logger
	trustedProxyHops int
}

// NewAdminCatalog creates the handler for all admin catalog endpoints. The
// projection feeds tag counts and the single-cluster node detail fallback.
func NewAdminCatalog(authHandler *Auth, st *store.Store, client cluster.Client, projection *inventory.Projection, log *slog.Logger) *AdminCatalog {
	return &AdminCatalog{auth: authHandler, store: st, client: client, projection: projection, inventory: projection, log: log}
}

// NewAdminCatalogWithRegistry creates catalog handlers with mandatory cluster selection.
func NewAdminCatalogWithRegistry(authHandler *Auth, st *store.Store, registry cluster.ClientProvider, projection *inventory.Projection, log *slog.Logger) *AdminCatalog {
	return &AdminCatalog{auth: authHandler, store: st, projection: projection, inventory: projection, clusters: registry, clients: registry, log: log}
}

//  - Nodes -

type adminNodeDTO struct {
	Name         string  `json:"name"`
	Status       string  `json:"status"`
	CPUCores     int     `json:"cpuCores"`
	CPUUsage     float64 `json:"cpuUsage"`
	MemoryTotal  int64   `json:"memoryTotal"`
	MemoryUsed   int64   `json:"memoryUsed"`
	StorageTotal int64   `json:"storageTotal"`
	StorageUsed  int64   `json:"storageUsed"`
	VMCount      int     `json:"vmCount"`
	Enabled      bool    `json:"enabled"`
	Missing      bool    `json:"missing"`
}

// ServeNodes handles GET /api/v1/admin/nodes.
//
//nolint:dupl // structurally similar to ServeTemplates by design (row→DTO mapping)
func (h *AdminCatalog) ServeNodes(w http.ResponseWriter, r *http.Request) {
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
	nodes, err := catalog.AdminListNodes(r.Context(), h.store, client, clusterName)
	if err != nil {
		SetErrorMsg(w, "admin list nodes failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	dto := make([]adminNodeDTO, len(nodes))
	for i, n := range nodes {
		dto[i] = adminNodeDTO{
			Name: n.Name, Status: n.Status, CPUCores: n.CPUCores, CPUUsage: n.CPUUsage,
			MemoryTotal: n.MemoryTotal, MemoryUsed: n.MemoryUsed,
			StorageTotal: n.StorageTotal, StorageUsed: n.StorageUsed,
			VMCount: n.VMCount, Enabled: n.Enabled, Missing: n.Missing,
		}
	}

	writeAdminJSON(w, http.StatusOK, dto)
}

type nodeToggleRequest struct {
	Cluster string `json:"cluster"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type toggleResponse struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// ServeNodeToggle handles POST /api/v1/admin/nodes/toggle.
func (h *AdminCatalog) ServeNodeToggle(w http.ResponseWriter, r *http.Request) {
	var req nodeToggleRequest
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
	err = catalog.SetNodeEnabled(r.Context(), h.store, client, clusterName, req.Name, req.Enabled)
	if errors.Is(err, cluster.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", nodeNotFoundMsg(req.Name))
		return
	}

	if err != nil {
		SetErrorMsg(w, "admin toggle node failed", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	h.recordAdminAction(r, "admin.nodes.toggle", "node", req.Name,
		fmt.Sprintf("node %s on cluster %s set enabled=%v", req.Name, clusterName, req.Enabled),
		[]any{map[string]any{auditKeyCluster: clusterName, auditKeyName: req.Name, auditKeyEnabled: req.Enabled}})
	writeAdminJSON(w, http.StatusOK, toggleResponse{Name: req.Name, Enabled: req.Enabled})
}

//  - Storages -
