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
	projection *inventory.Projection
	writer     cluster.Writer
	audit      vm.AuditRecorder
	refresher  vm.IndexRefresher
	store      *store.Store
	log        *slog.Logger
}

// NewAdminPools creates the pool administration handler. The store enables
// managed-pool tracking: only pools PVMSS provisioned may be deleted.
func NewAdminPools(authHandler *Auth, client cluster.Client, projection *inventory.Projection, writer cluster.Writer, audit vm.AuditRecorder, refresher vm.IndexRefresher, st *store.Store, log *slog.Logger) *AdminPools {
	return &AdminPools{auth: authHandler, client: client, projection: projection, writer: writer, audit: audit, refresher: refresher, store: st, log: log}
}

// ServeList handles GET /api/v1/admin/pools.
func (h *AdminPools) ServeList(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.adminActor(w, r); !ok {
		return
	}
	clusterName := queryCluster(r)
	rows, err := pools.ListWithManaged(r.Context(), h.client, h.projection, h.store, clusterName, r.URL.Query().Get("search"))
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
	creds, err := pools.CreateManaged(r.Context(), actor, h.client, h.store, queryCluster(r), request.Name, request.Comment)
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
	result, err := pools.Delete(r.Context(), pools.CascadeDeps{Actor: actor, Client: h.client, Projection: h.projection, ClusterName: queryCluster(r), Writer: h.writer, Audit: h.audit, Refresher: h.refresher, Managed: h.store}, name)
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
