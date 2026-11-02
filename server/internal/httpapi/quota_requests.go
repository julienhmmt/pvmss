package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/logctx"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"strconv"
	"strings"
	"unicode/utf8"
)

// unlimitedQuota is the stored allowance meaning "no cap" (-1).
const unlimitedQuota = -1

// maxQuotaReasonRunes bounds the free-text reason on a quota request.
const maxQuotaReasonRunes = 500

// opReadQuotaRequest labels the pending-request read for failure logging.
const opReadQuotaRequest = "read quota request"

// QuotaRequests serves the in-app quota increase flow. Users file requests on
// their own pool (this file); administrators decide them
// (admin_quota_requests.go). Both sides share this struct.
type QuotaRequests struct {
	auth             *Auth
	service          *policy.Policy
	store            *store.Store
	log              *slog.Logger
	trustedProxyHops int
}

// NewQuotaRequests creates the quota request handlers.
func NewQuotaRequests(auth *Auth, service *policy.Policy, st *store.Store, log *slog.Logger) *QuotaRequests {
	return &QuotaRequests{auth: auth, service: service, store: st, log: log}
}

// SetTrustedProxyHops configures X-Forwarded-For trust for audit client IPs.
func (handler *QuotaRequests) SetTrustedProxyHops(n int) {
	handler.trustedProxyHops = n
}

type quotaRequestDTO struct {
	ID             int64  `json:"id"`
	Requester      string `json:"requester"`
	Cluster        string `json:"cluster"`
	Pool           string `json:"pool"`
	CurrentQuota   int    `json:"currentQuota"`
	RequestedQuota int    `json:"requestedQuota"`
	Reason         string `json:"reason"`
	Status         string `json:"status"`
	DecidedBy      string `json:"decidedBy"`
	DecidedAt      string `json:"decidedAt"`
	CreatedAt      string `json:"createdAt"`
}

type quotaRequestCreate struct {
	RequestedQuota *int   `json:"requestedQuota"`
	Reason         string `json:"reason"`
}

func quotaRequestDTOFrom(row store.QuotaRequest) quotaRequestDTO {
	return quotaRequestDTO{
		ID: row.ID, Requester: row.Requester, Cluster: row.Cluster, Pool: row.Pool,
		CurrentQuota: row.CurrentQuota, RequestedQuota: row.RequestedQuota, Reason: row.Reason,
		Status: row.Status, DecidedBy: row.DecidedBy, DecidedAt: row.DecidedAt, CreatedAt: row.CreatedAt,
	}
}

// registerQuotaRequestRoutes wires the user-facing quota request endpoints.
// The POST is CSRF-protected and rate limited like the other VM writes.
func registerQuotaRequestRoutes(mux *http.ServeMux, protect protectFunc, limiter *userRateLimiter, h *QuotaRequests) {
	mux.Handle("GET /api/v1/quota-requests/mine", http.HandlerFunc(h.ServeMine))
	mux.Handle("POST /api/v1/quota-requests", protect(http.HandlerFunc(h.ServeCreate), limiter))
}

// ServeMine handles GET /api/v1/quota-requests/mine: the requester's latest
// request on their own pool, or null when they never asked.
func (handler *QuotaRequests) ServeMine(w http.ResponseWriter, r *http.Request) {
	identity, ok := handler.requesterPool(w, r)
	if !ok {
		return
	}

	row, err := handler.store.LatestQuotaRequest(r.Context(), identity.Username, identity.Cluster, identity.Pool)
	if errors.Is(err, sql.ErrNoRows) {
		writeAdminJSON(w, http.StatusOK, map[string]any{"request": nil})
		return
	}

	if err != nil {
		handler.writeFailure(w, opReadQuotaRequest, err)
		return
	}

	writeAdminJSON(w, http.StatusOK, map[string]any{"request": quotaRequestDTOFrom(row)})
}

// ServeCreate handles POST /api/v1/quota-requests. The cluster and pool come
// from the session identity, never from the body.
func (handler *QuotaRequests) ServeCreate(w http.ResponseWriter, r *http.Request) {
	identity, ok := handler.requesterPool(w, r)
	if !ok {
		return
	}

	var request quotaRequestCreate
	if err := decodeJSON(w, r, &request); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	reason := strings.TrimSpace(request.Reason)
	if reason == "" || utf8.RuneCountInString(reason) > maxQuotaReasonRunes {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "reason is required (500 characters maximum)")
		return
	}

	if request.RequestedQuota == nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "requestedQuota is required")
		return
	}

	quota, err := handler.service.Quota(r.Context(), identity.Cluster, identity)
	if err != nil {
		handler.writeFailure(w, "read quota", err)
		return
	}

	if message := quotaRangeError(*request.RequestedQuota, quota.Allowed); message != "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_quota", message)
		return
	}

	created := store.QuotaRequest{
		Requester: identity.Username, Cluster: identity.Cluster, Pool: identity.Pool,
		CurrentQuota: quota.Allowed, RequestedQuota: *request.RequestedQuota, Reason: reason,
	}

	if err := handler.store.CreateQuotaRequest(r.Context(), created); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			handler.writePending(w, r, identity)
			return
		}

		handler.writeFailure(w, "create quota request", err)

		return
	}

	row, err := handler.store.PendingQuotaRequest(r.Context(), identity.Username, identity.Cluster, identity.Pool)
	if err != nil {
		handler.writeFailure(w, opReadQuotaRequest, err)
		return
	}

	writeAdminJSON(w, http.StatusCreated, quotaRequestDTOFrom(row))
}

// requesterPool resolves the session and refuses callers without a quota pool
// (administrators and cluster admins with no personal pool).
func (handler *QuotaRequests) requesterPool(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
	identity, err := handler.auth.Principal(r)
	if err != nil {
		writeAdminError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return auth.Identity{}, false
	}

	if identity.IsAdmin || identity.Pool == "" {
		writeAdminError(w, http.StatusBadRequest, "no_pool", "your account has no quota pool")
		return auth.Identity{}, false
	}

	return identity, true
}

// writePending answers the duplicate-pending case with the open request's id.
func (handler *QuotaRequests) writePending(w http.ResponseWriter, r *http.Request, identity auth.Identity) {
	row, err := handler.store.PendingQuotaRequest(r.Context(), identity.Username, identity.Cluster, identity.Pool)
	if err != nil {
		handler.writeFailure(w, opReadQuotaRequest, err)
		return
	}

	body, err := json.Marshal(map[string]any{
		"code": "quota_request_pending", "message": "a quota request is already pending", "id": row.ID,
	})
	if err != nil {
		handler.writeFailure(w, "encode quota conflict", err)
		return
	}

	_ = writeJSON(w, http.StatusConflict, body)
}

// quotaRangeError validates a requested allowance against the current one.
// It returns an empty string when the request is acceptable.
func quotaRangeError(requested, current int) string {
	switch {
	case current == unlimitedQuota:
		return "your quota is already unlimited"
	case requested == unlimitedQuota:
		return ""
	case requested <= current:
		return "requested quota must be above the current quota"
	case requested > policy.MaxVMPerUserLimit:
		return "requested quota exceeds the maximum allowed value"
	default:
		return ""
	}
}

func (handler *QuotaRequests) writeFailure(w http.ResponseWriter, operation string, err error) {
	SetErrorMsg(w, operation+" failed", err)
	writeAdminError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

// recordAudit writes an admin decision to the audit log. A failed audit write
// is logged and does not undo the decision: the request row keeps the outcome.
func (handler *QuotaRequests) recordAudit(r *http.Request, actor, action string, id int64, summary string, changes []any) {
	if handler.store == nil {
		return
	}

	if err := handler.store.RecordAdminAction(r.Context(), actor, action, "quota_request",
		strconv.FormatInt(id, 10), detailJSON(summary, changes), clientIP(r, handler.trustedProxyHops)); err != nil {
		logctx.FromOr(r.Context(), handler.log).WarnContext(r.Context(), "failed to record admin action",
			"component", "httpapi", "action", action, "error", err)
	}
}
