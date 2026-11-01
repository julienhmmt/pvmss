package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"pvmss/server/internal/logctx"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"strconv"
)

// registerAdminQuotaRequestRoutes wires the admin inbox for quota requests.
func registerAdminQuotaRequestRoutes(mux *http.ServeMux, adminProtect adminRouteProtect, h *QuotaRequests) {
	mux.Handle("GET /api/v1/admin/quota-requests", adminProtect(http.MethodGet, http.HandlerFunc(h.ServeAdminList)))
	mux.Handle("POST /api/v1/admin/quota-requests/{id}/approve", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeApprove)))
	mux.Handle("POST /api/v1/admin/quota-requests/{id}/deny", adminProtect(http.MethodPost, http.HandlerFunc(h.ServeDeny)))
}

type quotaApproveRequest struct {
	Quota *int `json:"quota"`
}

// ServeAdminList handles GET /api/v1/admin/quota-requests?status=pending.
// status is pending (default), approved, denied or all.
func (handler *QuotaRequests) ServeAdminList(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = store.QuotaRequestPending
	}

	filter := status
	switch status {
	case store.QuotaRequestPending, store.QuotaRequestApproved, store.QuotaRequestDenied:
	case "all":
		filter = ""
	default:
		writeAdminError(w, http.StatusBadRequest, "invalid_status", "status must be pending, approved, denied or all")
		return
	}

	rows, err := handler.store.ListQuotaRequests(r.Context(), filter)
	if err != nil {
		handler.writeFailure(w, "list quota requests", err)
		return
	}

	items := make([]quotaRequestDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, quotaRequestDTOFrom(row))
	}

	writeAdminJSON(w, http.StatusOK, map[string]any{"requests": items})
}

// ServeApprove handles POST /api/v1/admin/quota-requests/{id}/approve. The
// body is optional JSON; quota overrides the requested number. The cap is
// written through the policy service, then the request is marked approved.
func (handler *QuotaRequests) ServeApprove(w http.ResponseWriter, r *http.Request) {
	id, row, ok := handler.pendingRequest(w, r)
	if !ok {
		return
	}

	var body quotaApproveRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	target := row.RequestedQuota
	if body.Quota != nil {
		target = *body.Quota
	}

	update, err := handler.service.UpdatePolicy(r.Context(), row.Cluster, func(current policy.Settings) policy.Settings {
		current.Allowed = target

		return current
	})
	if err != nil {
		handler.writeApproveError(w, err)
		return
	}

	actor := handler.actorName(r)
	if err := handler.store.DecideQuotaRequest(r.Context(), id, store.QuotaRequestApproved, actor); err != nil {
		handler.writeDecidedConflict(w, r, id, err)
		return
	}

	summary := "approved quota request " + strconv.FormatInt(id, 10) + " for " + row.Requester +
		" on pool " + row.Pool + " (" + row.Cluster + ")"
	handler.recordAudit(r, actor, "quota.request_approved", id, summary, []any{
		map[string]any{auditKeyField: "quota.maxVmPerUser", auditKeyOld: update.Before.Allowed, auditKeyNew: update.After.Allowed},
		map[string]any{auditKeyField: "quota.requested", auditKeyOld: row.CurrentQuota, auditKeyNew: row.RequestedQuota},
	})

	handler.writeDecided(w, r, id)
}

// ServeDeny handles POST /api/v1/admin/quota-requests/{id}/deny.
func (handler *QuotaRequests) ServeDeny(w http.ResponseWriter, r *http.Request) {
	id, row, ok := handler.pendingRequest(w, r)
	if !ok {
		return
	}

	actor := handler.actorName(r)
	if err := handler.store.DecideQuotaRequest(r.Context(), id, store.QuotaRequestDenied, actor); err != nil {
		handler.writeDecidedConflict(w, r, id, err)
		return
	}

	summary := "denied quota request " + strconv.FormatInt(id, 10) + " for " + row.Requester +
		" on pool " + row.Pool + " (" + row.Cluster + ")"
	handler.recordAudit(r, actor, "quota.request_denied", id, summary, []any{
		map[string]any{auditKeyField: "quota.requested", auditKeyOld: row.CurrentQuota, auditKeyNew: row.RequestedQuota},
	})

	handler.writeDecided(w, r, id)
}

// pendingRequest parses the {id} path value and loads the request, answering
// 400, 404 or 409 itself when it cannot be decided.
func (handler *QuotaRequests) pendingRequest(w http.ResponseWriter, r *http.Request) (int64, store.QuotaRequest, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeAdminError(w, http.StatusBadRequest, "invalid_id", "invalid quota request id")
		return 0, store.QuotaRequest{}, false
	}

	row, err := handler.store.QuotaRequestByID(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		writeAdminError(w, http.StatusNotFound, "not_found", "quota request not found")
		return 0, store.QuotaRequest{}, false
	}

	if err != nil {
		handler.writeFailure(w, "read quota request", err)
		return 0, store.QuotaRequest{}, false
	}

	if row.Status != store.QuotaRequestPending {
		writeAdminError(w, http.StatusConflict, "quota_request_decided", "quota request already decided")
		return 0, store.QuotaRequest{}, false
	}

	return id, row, true
}

func (handler *QuotaRequests) writeApproveError(w http.ResponseWriter, err error) {
	if errors.Is(err, policy.ErrInvalidPolicy) {
		writeAdminError(w, http.StatusBadRequest, "invalid_policy", err.Error())
		return
	}

	if errors.Is(err, policy.ErrConcurrentUpdate) {
		writeAdminError(w, http.StatusConflict, "policy_conflict", "the policy was modified concurrently; reload and retry")
		return
	}

	handler.writeFailure(w, "approve quota request", err)
}

// writeDecidedConflict handles a decision that lost a race to another admin.
// The policy write of an approval may already have landed; the log says so.
func (handler *QuotaRequests) writeDecidedConflict(w http.ResponseWriter, r *http.Request, id int64, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		logctx.FromOr(r.Context(), handler.log).WarnContext(r.Context(), "quota request decided concurrently",
			"component", "httpapi", "event", "quota.request_race", "id", id)
		writeAdminError(w, http.StatusConflict, "quota_request_decided", "quota request already decided")

		return
	}

	handler.writeFailure(w, "decide quota request", err)
}

func (handler *QuotaRequests) writeDecided(w http.ResponseWriter, r *http.Request, id int64) {
	row, err := handler.store.QuotaRequestByID(r.Context(), id)
	if err != nil {
		handler.writeFailure(w, "read quota request", err)
		return
	}

	body, err := json.Marshal(quotaRequestDTOFrom(row))
	if err != nil {
		handler.writeFailure(w, "encode quota request", err)
		return
	}

	_ = writeJSON(w, http.StatusOK, body)
}

func (handler *QuotaRequests) actorName(r *http.Request) string {
	identity, _ := handler.auth.Principal(r)

	return identity.Username
}
