//nolint:wsl_v5 // parallel catalog handlers keep validation and contract mapping adjacent
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"strings"
)

// AdminCatalog serves the admin catalog endpoints: the four discover-and-approve
// resources (nodes/storages/bridges/isos), VM profiles (full CRUD), and tags
// (CRUD with protected pvmss). Every route is wrapped by Auth.RequireAdmin
// (FR-008).
type AdminCatalog struct {
	auth       *Auth
	store      *store.Store
	client     cluster.Client
	projection *inventory.Projection
	clusters   ClusterLister
	clients    cluster.ClientProvider
	log        *slog.Logger
}

// NewAdminCatalog creates the handler for all admin catalog endpoints. The
// projection is needed for tag VM counts (FR-015); it may be nil when tags
// are not used (tests that only exercise nodes/storages/bridges/isos).
func NewAdminCatalog(authHandler *Auth, st *store.Store, client cluster.Client, projection *inventory.Projection, log *slog.Logger) *AdminCatalog {
	return &AdminCatalog{auth: authHandler, store: st, client: client, projection: projection, log: log}
}

// NewAdminCatalogWithRegistry creates catalog handlers with mandatory cluster selection.
func NewAdminCatalogWithRegistry(authHandler *Auth, st *store.Store, registry cluster.ClientProvider, projection *inventory.Projection, log *slog.Logger) *AdminCatalog {
	return &AdminCatalog{auth: authHandler, store: st, projection: projection, clusters: registry, clients: registry, log: log}
}

// --- Nodes ---

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
}

// ServeNodes handles GET /api/v1/admin/nodes.
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
		h.log.Error("admin list nodes failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	dto := make([]adminNodeDTO, len(nodes))
	for i, n := range nodes {
		dto[i] = adminNodeDTO{
			Name: n.Name, Status: n.Status, CPUCores: n.CPUCores, CPUUsage: n.CPUUsage,
			MemoryTotal: n.MemoryTotal, MemoryUsed: n.MemoryUsed,
			StorageTotal: n.StorageTotal, StorageUsed: n.StorageUsed,
			VMCount: n.VMCount, Enabled: n.Enabled,
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
		h.log.Error("admin toggle node failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	writeAdminJSON(w, http.StatusOK, toggleResponse{Name: req.Name, Enabled: req.Enabled})
}

// --- Storages ---

type adminStorageDTO struct {
	Name    string `json:"name"`
	Node    string `json:"node"`
	Type    string `json:"type"`
	Total   int64  `json:"totalBytes"`
	Used    int64  `json:"usedBytes"`
	Enabled bool   `json:"enabled"`
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
		h.log.Error("admin list storages failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	dto := make([]adminStorageDTO, len(storages))
	for i, s := range storages {
		dto[i] = adminStorageDTO{
			Name: s.Name, Node: s.Node, Type: s.Type,
			Total: s.Total, Used: s.Used, Enabled: s.Enabled,
		}
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
//
//nolint:dupl // intentionally parallel to ServeISOToggle (same shape, different resource)
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
		h.log.Error("admin toggle storage failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	writeAdminJSON(w, http.StatusOK, storageToggleResponse{Name: req.Name, Node: req.Node, Enabled: req.Enabled})
}

// --- Bridges ---

type adminBridgeDTO struct {
	Name    string `json:"name"`
	Node    string `json:"node"`
	Active  bool   `json:"active"`
	Comment string `json:"comment"`
	Enabled bool   `json:"enabled"`
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
		h.log.Error("admin list bridges failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	dto := make([]adminBridgeDTO, len(bridges))
	for i, b := range bridges {
		dto[i] = adminBridgeDTO{
			Name: b.Name, Node: b.Node, Active: b.Active,
			Comment: b.Comment, Enabled: b.Enabled,
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
		h.log.Error("admin toggle bridge failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	writeAdminJSON(w, http.StatusOK, bridgeToggleResponse{Node: req.Node, Name: req.Name, Enabled: req.Enabled})
}

// --- ISOs ---

type adminISODTO struct {
	Storage   string `json:"storage"`
	Node      string `json:"node"`
	File      string `json:"file"`
	SizeBytes int64  `json:"sizeBytes"`
	Enabled   bool   `json:"enabled"`
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
		h.log.Error("admin list isos failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	dto := make([]adminISODTO, len(isos))
	for i, iso := range isos {
		dto[i] = adminISODTO{
			Storage: iso.Storage, Node: iso.Node, File: iso.File,
			SizeBytes: iso.SizeBytes, Enabled: iso.Enabled,
		}
	}

	writeAdminJSON(w, http.StatusOK, dto)
}

type isoToggleRequest struct {
	Cluster string `json:"cluster"`
	Storage string `json:"storage"`
	File    string `json:"file"`
	Enabled bool   `json:"enabled"`
}

type isoToggleResponse struct {
	Storage string `json:"storage"`
	File    string `json:"file"`
	Enabled bool   `json:"enabled"`
}

// ServeISOToggle handles POST /api/v1/admin/isos/toggle.
//
//nolint:dupl // intentionally parallel to ServeStorageToggle (same shape, different resource)
func (h *AdminCatalog) ServeISOToggle(w http.ResponseWriter, r *http.Request) {
	var req isoToggleRequest
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
	err = catalog.SetISOEnabled(r.Context(), h.store, client, clusterName, req.Storage, req.File, req.Enabled)
	if errors.Is(err, cluster.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", isoNotFoundMsg(req.Storage, req.File))
		return
	}

	if err != nil {
		h.log.Error("admin toggle iso failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	writeAdminJSON(w, http.StatusOK, isoToggleResponse{Storage: req.Storage, File: req.File, Enabled: req.Enabled})
}

// --- helpers ---

func (h *AdminCatalog) clientFor(name string) (cluster.Client, error) {
	if h.clients == nil {
		if h.client == nil {
			return nil, cluster.ErrClusterNotFound
		}
		return h.client, nil
	}
	return h.clients.Client(name)
}

func queryCluster(r *http.Request) string {
	c := r.URL.Query().Get("cluster")
	if c == "" {
		return defaultClusterName
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

func writeAdminError(w http.ResponseWriter, status int, code, message string) {
	_ = writeClusterError(w, status, code, message)
}

func nodeNotFoundMsg(name string) string {
	return "node \"" + name + "\"" + msgNotReportedByCluster
}

func storageNotFoundMsg(name, node string) string {
	return "storage \"" + name + "\" on node \"" + node + "\"" + msgNotReportedByCluster
}

func bridgeNotFoundMsg(node, name string) string {
	return "bridge \"" + name + "\" on node \"" + node + "\"" + msgNotReportedByCluster
}

func isoNotFoundMsg(storage, file string) string {
	return "iso \"" + file + "\" on storage \"" + storage + "\"" + msgNotReportedByCluster
}
