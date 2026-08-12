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
	log        *slog.Logger
}

// NewAdminPools creates the pool administration handler.
func NewAdminPools(authHandler *Auth, client cluster.Client, projection *inventory.Projection, writer cluster.Writer, audit vm.AuditRecorder, refresher vm.IndexRefresher, log *slog.Logger) *AdminPools {
	return &AdminPools{auth: authHandler, client: client, projection: projection, writer: writer, audit: audit, refresher: refresher, log: log}
}

// ServeList handles GET /api/v1/admin/pools.
func (h *AdminPools) ServeList(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.adminActor(w, r); !ok {
		return
	}
	rows, err := pools.List(r.Context(), h.client, h.projection, r.URL.Query().Get("search"))
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
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	created, err := pools.Create(r.Context(), actor, h.client, request.Name, request.Password, request.Comment)
	if err != nil {
		h.writeCreateError(w, err)
		return
	}
	writeAdminJSON(w, http.StatusCreated, poolSummary{Name: created.Name, Comment: created.Comment})
}

// ServeDelete handles DELETE /api/v1/admin/pools/{name}.
func (h *AdminPools) ServeDelete(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.adminActor(w, r)
	if !ok {
		return
	}
	name := r.PathValue("name")
	if err := pools.ValidateName(name); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_pool_name", "invalid pool name")
		return
	}
	result, err := pools.Delete(r.Context(), actor, h.client, h.projection, queryCluster(r), name, h.writer, h.audit, h.refresher)
	if errors.Is(err, pools.ErrNotFound) {
		writeAdminError(w, http.StatusNotFound, "not_found", "pool \""+name+"\" not found")
		return
	}
	if errors.Is(err, pools.ErrForbidden) {
		writeAdminError(w, http.StatusForbidden, "forbidden", "admin only")
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
	Name     string `json:"name"`
	Comment  string `json:"comment"`
	Password string `json:"password"`
}

type poolSummary struct {
	Name    string `json:"name"`
	Comment string `json:"comment"`
	Total   int    `json:"total"`
	Running int    `json:"running"`
	Stopped int    `json:"stopped"`
}

type poolSummaryList []poolSummary

func poolSummaries(rows []pools.PoolSummary) poolSummaryList {
	result := make(poolSummaryList, len(rows))
	for index, row := range rows {
		result[index] = poolSummary{Name: row.Name, Comment: row.Comment, Total: row.Total, Running: row.Running, Stopped: row.Stopped}
	}
	return result
}

func (h *AdminPools) adminActor(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
	actor, err := h.auth.Principal(r)
	if err != nil {
		writeAdminError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return auth.Identity{}, false
	}
	if !actor.IsAdmin {
		writeAdminError(w, http.StatusForbidden, "forbidden", "admin only")
		return auth.Identity{}, false
	}
	return actor, true
}

func (h *AdminPools) writeCreateError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, pools.ErrInvalidName):
		writeAdminError(w, http.StatusBadRequest, "invalid_pool_name", "invalid pool name")
	case errors.Is(err, pools.ErrWeakPassword):
		writeAdminError(w, http.StatusBadRequest, "invalid_password", "password must contain at least 8 characters")
	case errors.Is(err, pools.ErrAlreadyExists):
		writeAdminError(w, http.StatusConflict, "duplicate_pool", err.Error())
	case errors.Is(err, pools.ErrForbidden):
		writeAdminError(w, http.StatusForbidden, "forbidden", "admin only")
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
