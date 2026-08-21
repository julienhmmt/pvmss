//nolint:wsl_v5 // endpoint handlers keep validation and contract mapping adjacent
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/pools"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
)

// AdminPools serves the admin pool list, provisioning, and cascade endpoints.
type AdminPools struct {
	auth       *Auth
	client     cluster.Client
	clients    cluster.ClientProvider
	source     inventory.LookupSource
	projection *inventory.Projection
	writer     cluster.Writer
	audit      vm.AuditRecorder
	refresher  vm.IndexRefresher
	store      *store.Store
	log        *slog.Logger
}

// AdminPoolsDeps groups the collaborators AdminPools needs. Bundling them
// keeps NewAdminPools's parameter count under go:S107's ceiling and makes
// mis-ordering at call sites impossible.
type AdminPoolsDeps struct {
	Auth       *Auth
	Client     cluster.Client
	Projection *inventory.Projection
	Writer     cluster.Writer
	Audit      vm.AuditRecorder
	Refresher  vm.IndexRefresher
	Store      *store.Store
	Log        *slog.Logger
}

// NewAdminPools creates the pool administration handler, bound to a single
// cluster. The store enables managed-pool tracking: only pools PVMSS
// provisioned may be deleted. Use NewAdminPoolsWithRegistry for multi-cluster
// deployments.
func NewAdminPools(deps AdminPoolsDeps) *AdminPools {
	return &AdminPools{auth: deps.Auth, client: deps.Client, projection: deps.Projection, writer: deps.Writer, audit: deps.Audit, refresher: deps.Refresher, store: deps.Store, log: deps.Log}
}

// AdminPoolsRegistryDeps groups the collaborators NewAdminPoolsWithRegistry
// needs for multi-cluster wiring. Bundling them keeps the parameter count
// under go:S107's ceiling and makes mis-ordering at call sites impossible.
type AdminPoolsRegistryDeps struct {
	Auth       *Auth
	Clients    cluster.ClientProvider
	Source     inventory.LookupSource
	Projection *inventory.Projection
	Writer     cluster.Writer
	Audit      vm.AuditRecorder
	Refresher  vm.IndexRefresher
	Store      *store.Store
	Log        *slog.Logger
}

// NewAdminPoolsWithRegistry creates the handler with per-request client and
// projection resolution, keyed on the ?cluster= query parameter every
// endpoint here already reads — without this, an admin managing pools on a
// non-default cluster would silently operate against the default cluster's
// Proxmox API instead.
func NewAdminPoolsWithRegistry(deps AdminPoolsRegistryDeps) *AdminPools {
	handler := NewAdminPools(AdminPoolsDeps{Auth: deps.Auth, Client: nil, Projection: deps.Projection, Writer: deps.Writer, Audit: deps.Audit, Refresher: deps.Refresher, Store: deps.Store, Log: deps.Log})
	handler.clients = deps.Clients
	handler.source = deps.Source

	return handler
}

// clientFor resolves the cluster.Client for clusterName, falling back to the
// single bound client when clients is nil (legacy single-cluster ctor).
func (h *AdminPools) clientFor(clusterName string) (cluster.Client, error) {
	if h.clients == nil {
		if h.client == nil {
			return nil, cluster.ErrClusterNotFound
		}

		return h.client, nil
	}

	return h.clients.Client(clusterName)
}

// projectionFor resolves the inventory.Projection for clusterName, falling
// back to the single bound projection when source is nil.
func (h *AdminPools) projectionFor(clusterName string) (*inventory.Projection, error) {
	registry, ok := h.source.(*inventory.Registry)
	if !ok {
		return h.projection, nil
	}

	return registry.Projection(clusterName)
}

// ServeList handles GET /api/v1/admin/pools.
func (h *AdminPools) ServeList(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.adminActor(w, r); !ok {
		return
	}
	clusterName := queryCluster(r)
	client, err := h.clientFor(clusterName)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	projection, err := h.projectionFor(clusterName)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	rows, err := pools.ListWithManaged(r.Context(), client, projection, h.store, clusterName, r.URL.Query().Get("search"))
	if err != nil {
		h.log.Error("admin pool list failed", "component", "httpapi", "error", err)
		writeAdminError(w, http.StatusBadGateway, "cluster_unreachable", "failed to list pools")
		return
	}
	writeAdminJSON(w, http.StatusOK, poolSummaries(rows))
}

// ServeCreate handles POST /api/v1/admin/pools.
func (h *AdminPools) ServeCreate(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.adminActor(w, r)
	if !ok {
		return
	}
	var request createPoolRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}
	client, err := h.clientFor(queryCluster(r))
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	creds, err := pools.CreateManaged(r.Context(), actor, client, h.store, queryCluster(r), request.Name, request.Comment)
	if err != nil {
		h.writeCreateError(w, err)
		return
	}
	writeAdminJSON(w, http.StatusCreated, createPoolResponse{
		Name:     creds.PoolName,
		Username: creds.Username,
		Password: creds.Password,
		Comment:  creds.Comment,
		Managed:  true,
	})
}

// ServeDelete handles DELETE /api/v1/admin/pools/{name}.
func (h *AdminPools) ServeDelete(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.adminActor(w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	if name == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_pool_name", "invalid pool name")
		return
	}
	clusterName := queryCluster(r)
	client, err := h.clientFor(clusterName)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	projection, err := h.projectionFor(clusterName)
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	writer, err := resolveCapability(h.clients, h.writer, clusterName, "Writer")
	if err != nil {
		writeAdminError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}
	result, err := pools.Delete(r.Context(), pools.CascadeDeps{Actor: actor, Client: client, Projection: projection, ClusterName: clusterName, Writer: writer, Audit: h.audit, Refresher: h.refresher, Managed: h.store}, name)
	if errors.Is(err, pools.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", "pool \""+name+"\" not found")
		return
	}
	if errors.Is(err, pools.ErrForbidden) {
		writeAdminError(w, http.StatusForbidden, "forbidden", msgAdminOnly)
		return
	}
	if errors.Is(err, pools.ErrNotManaged) {
		writeAdminError(w, http.StatusConflict, "not_managed", "pool \""+name+"\" is not managed by PVMSS")
		return
	}
	if err != nil {
		h.log.Error("admin pool deletion failed", "component", "httpapi", "pool", name, "error", err)
		writeAdminError(w, http.StatusBadGateway, "deletion_failed", "pool deletion failed")
		return
	}
	writeAdminJSON(w, http.StatusOK, deletePoolResponse{Status: result.Status, UserDeleted: result.UserDeleted})
}

// deletePoolResponse is the stable JSON contract for DELETE /api/v1/admin/pools/{name}.
type deletePoolResponse struct {
	Status      string `json:"status"`
	UserDeleted bool   `json:"userDeleted"`
}

type createPoolRequest struct {
	Name    string `json:"name"`
	Comment string `json:"comment"`
}

type createPoolResponse struct {
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
	Comment  string `json:"comment"`
	Managed  bool   `json:"managed"`
}

type poolSummary struct {
	Name    string `json:"name"`
	Comment string `json:"comment"`
	Total   int    `json:"total"`
	Running int    `json:"running"`
	Stopped int    `json:"stopped"`
	Managed bool   `json:"managed"`
}

type poolSummaryList []poolSummary

func poolSummaries(rows []pools.PoolSummary) poolSummaryList {
	result := make(poolSummaryList, len(rows))
	for index, row := range rows {
		result[index] = poolSummary{Name: row.Name, Comment: row.Comment, Total: row.Total, Running: row.Running, Stopped: row.Stopped, Managed: row.Managed}
	}
	return result
}

func (h *AdminPools) adminActor(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
	actor, err := h.auth.Principal(r)
	if err != nil {
		writeAdminError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return auth.Identity{}, false
	}
	if !actor.IsAdmin {
		writeAdminError(w, http.StatusForbidden, "forbidden", msgAdminOnly)
		return auth.Identity{}, false
	}
	return actor, true
}

func (h *AdminPools) writeCreateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, pools.ErrInvalidName):
		writeAdminError(w, http.StatusBadRequest, "invalid_pool_name", "invalid pool name")
	case errors.Is(err, pools.ErrAlreadyExists):
		writeAdminError(w, http.StatusConflict, "duplicate_pool", err.Error())
	case errors.Is(err, pools.ErrForbidden):
		writeAdminError(w, http.StatusForbidden, "forbidden", msgAdminOnly)
	default:
		var provisioningErr *pools.ProvisionError
		if errors.As(err, &provisioningErr) {
			h.log.Error("admin pool provisioning step failed", "component", "httpapi", "step", provisioningErr.Step, "error", provisioningErr.Err)
		} else {
			h.log.Error("admin pool provisioning failed", "component", "httpapi", "error", err)
		}
		writeAdminError(w, http.StatusBadGateway, "provisioning_failed", "pool provisioning failed")
	}
}
