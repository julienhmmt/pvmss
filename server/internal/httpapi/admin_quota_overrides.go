package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"pvmss/server/internal/logctx"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
)

// quotaOverrideDTO is one per-pool VM allowance that replaces the cluster quota.
type quotaOverrideDTO struct {
	Cluster   string `json:"cluster"`
	Pool      string `json:"pool"`
	MaxVMs    int    `json:"maxVms"`
	UpdatedBy string `json:"updatedBy"`
	UpdatedAt string `json:"updatedAt"`
}

type quotaOverrideUpdate struct {
	MaxVMs *int `json:"maxVms"`
}

func quotaOverrideDTOFrom(row store.PoolQuotaOverride) quotaOverrideDTO {
	return quotaOverrideDTO{
		Cluster: row.Cluster, Pool: row.Pool, MaxVMs: row.MaxVMs,
		UpdatedBy: row.UpdatedBy, UpdatedAt: row.UpdatedAt,
	}
}

// ServeOverrideList handles GET /api/v1/admin/quota-overrides?cluster=<name>.
// The cluster filter is optional; without it every cluster's overrides return.
func (handler *QuotaRequests) ServeOverrideList(w http.ResponseWriter, r *http.Request) {
	rows, err := handler.store.ListPoolQuotaOverrides(r.Context(), r.URL.Query().Get("cluster"))
	if err != nil {
		handler.writeFailure(w, "list quota overrides", err)
		return
	}

	items := make([]quotaOverrideDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, quotaOverrideDTOFrom(row))
	}

	writeAdminJSON(w, http.StatusOK, map[string]any{"overrides": items})
}

// ServeOverrideUpsert handles PUT /api/v1/admin/quota-overrides/{cluster}/{pool}.
// The pool must be a managed pool on the cluster; maxVms is absolute (-1 = unlimited).
func (handler *QuotaRequests) ServeOverrideUpsert(w http.ResponseWriter, r *http.Request) {
	cluster, pool := r.PathValue("cluster"), r.PathValue("pool")

	var body quotaOverrideUpdate
	if err := decodeJSON(w, r, &body); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	if body.MaxVMs == nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "maxVms is required")
		return
	}

	managed, err := handler.store.IsPoolManaged(r.Context(), cluster, pool)
	if err != nil {
		handler.writeFailure(w, "read managed pool", err)
		return
	}

	if !managed {
		writeAdminError(w, http.StatusNotFound, "not_found", "managed pool not found on this cluster")
		return
	}

	before, err := handler.service.PoolAllowance(r.Context(), cluster, pool)
	if err != nil {
		handler.writeFailure(w, "read pool quota", err)
		return
	}

	actor := handler.actorName(r)

	if err := handler.service.SetPoolQuotaOverride(r.Context(), cluster, pool, *body.MaxVMs, actor); err != nil {
		if errors.Is(err, policy.ErrInvalidPolicy) {
			writeAdminError(w, http.StatusBadRequest, "invalid_policy", err.Error())
			return
		}

		handler.writeFailure(w, "set quota override", err)

		return
	}

	row, err := handler.store.PoolQuotaOverride(r.Context(), cluster, pool)
	if err != nil {
		handler.writeFailure(w, "read quota override", err)
		return
	}

	summary := "set quota override for pool " + pool + " (" + cluster + ")"
	handler.recordOverrideAudit(r, actor, "quota.override_updated", cluster, pool, summary, []any{
		map[string]any{auditKeyField: "quota.poolMaxVmPerUser", auditKeyOld: before, auditKeyNew: *body.MaxVMs},
	})

	writeAdminJSON(w, http.StatusOK, quotaOverrideDTOFrom(row))
}

// ServeOverrideDelete handles DELETE /api/v1/admin/quota-overrides/{cluster}/{pool}.
// The pool falls back to the cluster-wide quota. 404 when it has no override.
func (handler *QuotaRequests) ServeOverrideDelete(w http.ResponseWriter, r *http.Request) {
	cluster, pool := r.PathValue("cluster"), r.PathValue("pool")

	err := handler.service.DeletePoolQuotaOverride(r.Context(), cluster, pool)
	if errors.Is(err, sql.ErrNoRows) {
		writeAdminError(w, http.StatusNotFound, "not_found", "no quota override for this pool")
		return
	}

	if err != nil {
		handler.writeFailure(w, "delete quota override", err)
		return
	}

	actor := handler.actorName(r)
	summary := "removed quota override for pool " + pool + " (" + cluster + ")"
	handler.recordOverrideAudit(r, actor, "quota.override_deleted", cluster, pool, summary, []any{})

	w.WriteHeader(http.StatusNoContent)
}

// recordOverrideAudit writes an override change to the audit log. A failed
// write is logged and does not undo the change.
func (handler *QuotaRequests) recordOverrideAudit(r *http.Request, actor, action, cluster, pool, summary string, changes []any) {
	if handler.store == nil {
		return
	}

	if err := handler.store.RecordAdminAction(r.Context(), actor, action, "pool_quota_override",
		cluster+"/"+pool, detailJSON(summary, changes), clientIP(r, handler.trustedProxyHops)); err != nil {
		logctx.FromOr(r.Context(), handler.log).WarnContext(r.Context(), "failed to record admin action",
			"component", "httpapi", "action", action, "error", err)
	}
}
