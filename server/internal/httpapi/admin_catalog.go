//nolint:wsl_v5 // parallel catalog handlers keep validation and contract mapping adjacent
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"strconv"
	"strings"
)

// AdminCatalog serves the admin catalog endpoints: the four discover-and-approve
// resources (nodes/storages/bridges/isos), VM profiles (full CRUD), and tags
// (CRUD with protected pvmss). Every route is wrapped by Auth.RequireAdmin
// (FR-008).
type AdminCatalog struct {
	auth             *Auth
	store            *store.Store
	client           cluster.Client
	projection       *inventory.Projection
	clusters         ClusterLister
	clients          cluster.ClientProvider
	log              *slog.Logger
	trustedProxyHops int
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
		h.log.Error("admin toggle node failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	h.recordAdminAction(r, "admin.nodes.toggle", "node", req.Name,
		fmt.Sprintf("node %s on cluster %s set enabled=%v", req.Name, clusterName, req.Enabled),
		[]any{map[string]any{auditKeyCluster: clusterName, auditKeyName: req.Name, auditKeyEnabled: req.Enabled}})
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
		h.log.Error("admin list storages failed", "component", "httpapi", "error", err)
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
		h.log.Error("admin toggle storage failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	h.recordAdminAction(r, "admin.storages.toggle", "storage", req.Name,
		fmt.Sprintf("storage %s on node %s cluster %s set enabled=%v", req.Name, req.Node, clusterName, req.Enabled),
		[]any{map[string]any{auditKeyCluster: clusterName, auditKeyName: req.Name, "node": req.Node, auditKeyEnabled: req.Enabled}})
	writeAdminJSON(w, http.StatusOK, storageToggleResponse{Name: req.Name, Node: req.Node, Enabled: req.Enabled})
}

// --- Bridges ---

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
		h.log.Error("admin list bridges failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

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
		h.log.Error("admin toggle bridge failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	h.recordAdminAction(r, "admin.bridges.toggle", "bridge", req.Name,
		fmt.Sprintf("bridge %s on node %s cluster %s set enabled=%v", req.Name, req.Node, clusterName, req.Enabled),
		[]any{map[string]any{auditKeyCluster: clusterName, auditKeyName: req.Name, "node": req.Node, auditKeyEnabled: req.Enabled}})
	writeAdminJSON(w, http.StatusOK, bridgeToggleResponse{Node: req.Node, Name: req.Name, Enabled: req.Enabled})
}

// --- ISOs ---

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
		h.log.Error("admin list isos failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

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
		h.log.Error("admin toggle iso failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	h.recordAdminAction(r, "admin.isos.toggle", "iso", req.File,
		fmt.Sprintf("iso %s on storage %s node %s cluster %s set enabled=%v", req.File, req.Storage, req.Node, clusterName, req.Enabled),
		[]any{map[string]any{auditKeyCluster: clusterName, "node": req.Node, "storage": req.Storage, "file": req.File, auditKeyEnabled: req.Enabled}})
	writeAdminJSON(w, http.StatusOK, isoToggleResponse{Node: req.Node, Storage: req.Storage, File: req.File, Enabled: req.Enabled})
}

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
		h.log.Error("admin delete node failed", "component", "httpapi", "error", err)
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
		h.log.Error("admin delete storage failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}

	h.recordAdminAction(r, "admin.storages.delete", "storage", name,
		fmt.Sprintf("deleted storage approval %q on node %s cluster %s", name, node, clusterName),
		[]any{map[string]any{auditKeyCluster: clusterName, auditKeyName: name, "node": node}})
	w.WriteHeader(http.StatusNoContent)
}

// ServeBridgeDelete handles DELETE /api/v1/admin/bridges/{cluster}/{node}/{name}:
// removes an orphan bridge approval row.
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
		h.log.Error("admin delete bridge failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}

	h.recordAdminAction(r, "admin.bridges.delete", "bridge", name,
		fmt.Sprintf("deleted bridge approval %q on node %s cluster %s", name, node, clusterName),
		[]any{map[string]any{auditKeyCluster: clusterName, auditKeyName: name, "node": node}})
	w.WriteHeader(http.StatusNoContent)
}

// ServeISODelete handles DELETE /api/v1/admin/isos/{cluster}/{node}/{storage}/{file}:
// removes an orphan ISO approval row.
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
		h.log.Error("admin delete iso failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}

	h.recordAdminAction(r, "admin.isos.delete", "iso", file,
		fmt.Sprintf("deleted iso approval %q on storage %s node %s cluster %s", file, storage, node, clusterName),
		[]any{map[string]any{auditKeyCluster: clusterName, "node": node, "storage": storage, "file": file}})
	w.WriteHeader(http.StatusNoContent)
}

// adminTemplateDTO is the admin response shape for one discovered template.
// missing is true for a stored approval whose template Proxmox no longer
// reports (issue 02) — the UI offers Remove on those rows only.
type adminTemplateDTO struct {
	VMID              int    `json:"vmid"`
	Node              string `json:"node"`
	Name              string `json:"name"`
	CloudInitCapable  bool   `json:"cloudInitCapable"`
	DiskStorage       string `json:"diskStorage"`
	DiskSizeGB        int    `json:"diskSizeGB"`
	DiskBus           string `json:"diskBus"`
	Enabled           bool   `json:"enabled"`
	Missing           bool   `json:"missing"`
	DiskUnreadable    bool   `json:"diskUnreadable"`
	OverrideDiscovery bool   `json:"overrideDiscovery"`
}

// ServeTemplates handles GET /api/v1/admin/templates.
//
//nolint:dupl // structurally similar to ServeNodes by design (row→DTO mapping)
func (h *AdminCatalog) ServeTemplates(w http.ResponseWriter, r *http.Request) {
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

	templates, err := catalog.AdminListTemplates(r.Context(), h.store, client, clusterName)
	if err != nil {
		h.log.Error("admin list templates failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	dto := make([]adminTemplateDTO, len(templates))
	for i, tmpl := range templates {
		dto[i] = adminTemplateDTO{
			VMID: tmpl.VMID, Node: tmpl.Node, Name: tmpl.Name,
			CloudInitCapable: tmpl.CloudInitCapable, DiskStorage: tmpl.DiskStorage,
			DiskSizeGB: tmpl.DiskSizeGB, DiskBus: tmpl.DiskBus, Enabled: tmpl.Enabled,
			Missing: tmpl.Missing, DiskUnreadable: tmpl.DiskUnreadable,
			OverrideDiscovery: tmpl.OverrideDiscovery,
		}
	}

	writeAdminJSON(w, http.StatusOK, dto)
}

type templateToggleRequest struct {
	Cluster string `json:"cluster"`
	VMID    int    `json:"vmid"`
	Enabled bool   `json:"enabled"`
}

type templateToggleResponse struct {
	VMID    int  `json:"vmid"`
	Enabled bool `json:"enabled"`
}

// ServeTemplateToggle handles POST /api/v1/admin/templates/toggle.
func (h *AdminCatalog) ServeTemplateToggle(w http.ResponseWriter, r *http.Request) {
	var req templateToggleRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}
	if req.VMID == 0 {
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

	// catalog.SetTemplateEnabled fetches the discovery set once, finds the
	// template, extracts its values for the first-approval insert, and
	// upserts the enabled state. No pre-fetch here — that would duplicate
	// the cluster round-trip.
	err = catalog.SetTemplateEnabled(r.Context(), h.store, client, clusterName, catalog.TemplateRef{VMID: req.VMID}, req.Enabled)
	if errors.Is(err, cluster.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", fmt.Sprintf("template vmid %d not found in cluster", req.VMID))
		return
	}

	if errors.Is(err, catalog.ErrTemplateUnreadable) {
		writeAdminError(w, http.StatusBadRequest, "template_unreadable", fmt.Sprintf("template vmid %d disk could not be read; it cannot be approved", req.VMID))
		return
	}

	if err != nil {
		h.log.Error("admin toggle template failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	h.recordAdminAction(r, "admin.templates.toggle", "template", strconv.Itoa(req.VMID),
		fmt.Sprintf("template vmid %d cluster %s set enabled=%v", req.VMID, clusterName, req.Enabled),
		[]any{map[string]any{auditKeyCluster: clusterName, "vmid": req.VMID, auditKeyEnabled: req.Enabled}})
	writeAdminJSON(w, http.StatusOK, templateToggleResponse{VMID: req.VMID, Enabled: req.Enabled})
}

// ServeTemplateDelete handles DELETE /api/v1/admin/templates/{cluster}/{vmid}
// (issue 02): removes an approval row — the UI offers Remove only on missing
// (orphaned) rows, but the API deletes any approval.
func (h *AdminCatalog) ServeTemplateDelete(w http.ResponseWriter, r *http.Request) {
	clusterName := r.PathValue("cluster")

	vmid, err := strconv.Atoi(r.PathValue("vmid"))
	if err != nil || vmid <= 0 {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "template vmid is required")

		return
	}

	err = catalog.DeleteTemplate(r.Context(), h.store, clusterName, vmid)
	if errors.Is(err, catalog.ErrTemplateNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", fmt.Sprintf("template vmid %d not found on cluster %q", vmid, clusterName))

		return
	}

	if err != nil {
		h.log.Error("admin delete template failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	h.recordAdminAction(r, "admin.templates.delete", "template", strconv.Itoa(vmid),
		fmt.Sprintf("deleted template approval vmid %d on cluster %s", vmid, clusterName),
		[]any{map[string]any{auditKeyCluster: clusterName, "vmid": vmid}})
	w.WriteHeader(http.StatusNoContent)
}

// templateUpdateRequest is the body of PUT /api/v1/admin/templates/{cluster}/{vmid}
// (schemaV26 override). The cluster is taken from the path to match the
// delete handler's convention; the body carries the editable field values.
type templateUpdateRequest struct {
	Node             string `json:"node"`
	Name             string `json:"name"`
	CloudInitCapable bool   `json:"cloudInitCapable"`
	DiskStorage      string `json:"diskStorage"`
	DiskSizeGB       int    `json:"diskSizeGB"`
	DiskBus          string `json:"diskBus"`
}

// ServeTemplateUpdate handles PUT /api/v1/admin/templates/{cluster}/{vmid}.
// Overrides the discovered template field values and pins the row against
// discovery-wins write-back (schemaV26). The create path still enforces the
// gabarit on clones, so an override above the gabarit simply means clones
// from this template are rejected at create time — the admin owns that.
func (h *AdminCatalog) ServeTemplateUpdate(w http.ResponseWriter, r *http.Request) {
	clusterName := r.PathValue("cluster")

	vmid, err := strconv.Atoi(r.PathValue("vmid"))
	if err != nil || vmid <= 0 {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "template vmid is required")

		return
	}

	var req templateUpdateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)

		return
	}

	if req.Node == "" || req.DiskStorage == "" || req.DiskBus == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "node, diskStorage, and diskBus are required")

		return
	}

	if req.DiskSizeGB < 0 {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "diskSizeGB must not be negative")

		return
	}

	values := store.TemplateValues{
		Node: req.Node, Name: req.Name, CloudInitCapable: req.CloudInitCapable,
		DiskStorage: req.DiskStorage, DiskSizeGB: req.DiskSizeGB, DiskBus: req.DiskBus,
	}
	if err := catalog.UpdateTemplate(r.Context(), h.store, clusterName, vmid, values); err != nil {
		if errors.Is(err, catalog.ErrTemplateNotFound) {
			writeAdminError(w, http.StatusNotFound, "not_found", fmt.Sprintf("template vmid %d not found on cluster %q", vmid, clusterName))

			return
		}

		h.log.Error("admin update template failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	h.recordAdminAction(r, "admin.templates.update", "template", strconv.Itoa(vmid),
		fmt.Sprintf("overrode template vmid %d on cluster %s: node=%s name=%q disk=%dGB@%s bus=%s cloudinit=%v",
			vmid, clusterName, req.Node, req.Name, req.DiskSizeGB, req.DiskStorage, req.DiskBus, req.CloudInitCapable),
		[]any{map[string]any{
			auditKeyCluster: clusterName, "vmid": vmid, "node": req.Node, "name": req.Name,
			"diskStorage": req.DiskStorage, "diskSizeGB": req.DiskSizeGB, "diskBus": req.DiskBus,
			"cloudInitCapable": req.CloudInitCapable,
		}})
	writeAdminJSON(w, http.StatusOK, adminTemplateDTO{
		VMID: vmid, Node: req.Node, Name: req.Name, CloudInitCapable: req.CloudInitCapable,
		DiskStorage: req.DiskStorage, DiskSizeGB: req.DiskSizeGB, DiskBus: req.DiskBus,
		OverrideDiscovery: true,
	})
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

// SetTrustedProxyHops configures how many X-Forwarded-For hops are trusted
// when extracting the client IP for audit entries.
func (h *AdminCatalog) SetTrustedProxyHops(n int) {
	h.trustedProxyHops = n
}

// recordAdminAction writes one admin audit row for a catalog mutation. It
// never fails the request — a failed audit write is logged and ignored.
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
