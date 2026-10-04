//nolint:wsl_v5 // policy handlers keep cluster selection and response mapping adjacent
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/logctx"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
)

// AdminPolicy serves the single global gabarit/quota surface and the dedicated
// node-capacité surface. Router registration supplies the admin role guard.
type AdminPolicy struct {
	auth             *Auth
	service          *policy.Policy
	clusters         ClusterLister
	store            *store.Store
	log              *slog.Logger
	trustedProxyHops int
}

// NewAdminPolicy creates the policy admin handler.
func NewAdminPolicy(auth *Auth, service *policy.Policy, log *slog.Logger) *AdminPolicy {
	return &AdminPolicy{auth: auth, service: service, log: log}
}

// NewAdminPolicyWithRegistry creates policy handlers with explicit cluster selection.
func NewAdminPolicyWithRegistry(auth *Auth, service *policy.Policy, registry ClusterLister, log *slog.Logger) *AdminPolicy {
	return &AdminPolicy{auth: auth, service: service, clusters: registry, log: log}
}

// SetStore wires the store used for admin action audit records.
func (handler *AdminPolicy) SetStore(st *store.Store) {
	handler.store = st
}

// SetTrustedProxyHops configures how many X-Forwarded-For hops to trust for
// client IP extraction used in audit log entries.
func (handler *AdminPolicy) SetTrustedProxyHops(n int) {
	handler.trustedProxyHops = n
}

type policyResponse struct {
	Cluster string           `json:"cluster"`
	Gabarit policyGabaritDTO `json:"gabarit"`
	Quota   policyQuotaDTO   `json:"quota"`
}

type policyGabaritDTO struct {
	MaxSockets       int `json:"maxSockets"`
	MaxCores         int `json:"maxCores"`
	MaxMemoryMB      int `json:"maxMemoryMB"`
	MaxDiskPerVMGB   int `json:"maxDiskPerVmGb"`
	MaxNetworkCards  int `json:"maxNetworkCards"`
	MaxSnapshots     int `json:"maxSnapshots"`
	IsolationVLANTag int `json:"isolationVlanTag"`
}

type policyQuotaDTO struct {
	MaxVMPerUser int `json:"maxVmPerUser"`
}

type policyUpdateRequest struct {
	Cluster string              `json:"cluster"`
	Gabarit *policyGabaritPatch `json:"gabarit"`
	Quota   *policyQuotaPatch   `json:"quota"`
}

type policyGabaritPatch struct {
	MaxSockets       *int `json:"maxSockets"`
	MaxCores         *int `json:"maxCores"`
	MaxMemoryMB      *int `json:"maxMemoryMB"`
	MaxDiskPerVMGB   *int `json:"maxDiskPerVmGb"`
	MaxNetworkCards  *int `json:"maxNetworkCards"`
	MaxSnapshots     *int `json:"maxSnapshots"`
	IsolationVLANTag *int `json:"isolationVlanTag"`
}

type policyQuotaPatch struct {
	MaxVMPerUser *int `json:"maxVmPerUser"`
}

// ServePolicy handles GET /api/v1/admin/policy.
func (handler *AdminPolicy) ServePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAdminError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	clusterName, clusterErr := ResolveClusterParam(r, handler.clusters)
	if clusterErr != nil {
		code, message := clusterParamError(clusterErr)
		writeAdminError(w, http.StatusBadRequest, code, message)
		return
	}

	response, err := handler.readPolicy(r.Context(), clusterName)
	if err != nil {
		handler.writeFailure(w, "read policy", err)
		return
	}

	writeAdminJSON(w, http.StatusOK, response)
}

// ServePolicyUpdate handles PUT /api/v1/admin/policy.
func (handler *AdminPolicy) ServePolicyUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeAdminError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var request policyUpdateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	clusterName, clusterErr := ResolveClusterValue(request.Cluster, handler.clusters)
	if clusterErr != nil {
		code, message := clusterParamError(clusterErr)
		writeAdminError(w, http.StatusBadRequest, code, message)
		return
	}

	update, err := handler.service.UpdatePolicy(r.Context(), clusterName, func(current policy.Settings) policy.Settings {
		next := current
		applyGabaritPatch(&next.Gabarit, request.Gabarit)
		applyQuotaPatch(&next.Allowed, request.Quota)

		return next
	})
	if err != nil {
		handler.writePolicyValidation(w, err)
		return
	}

	if update.Changed {
		changes := policyChangeDiff(update.Before, update.After)
		handler.recordAdminAction(r, "admin.policy.update", "policy", clusterName,
			"updated policy for cluster "+clusterName, changes)
	}

	writeAdminJSON(w, http.StatusOK, policyResponse{
		Cluster: clusterName,
		Gabarit: policyGabaritDTOFromModel(update.After.Gabarit),
		Quota:   policyQuotaDTO{MaxVMPerUser: update.After.Allowed},
	})
}

// policyChangeDiff compares the before/after policy settings and returns a
// list of change entries for the audit detail payload.
func policyChangeDiff(before, after policy.Settings) []any {
	changes := []any{}
	if before.Allowed != after.Allowed {
		changes = append(changes, map[string]any{auditKeyField: "quota.maxVmPerUser", auditKeyOld: before.Allowed, auditKeyNew: after.Allowed})
	}
	if before.Gabarit.MaxSockets != after.Gabarit.MaxSockets {
		changes = append(changes, map[string]any{auditKeyField: "gabarit.maxSockets", auditKeyOld: before.Gabarit.MaxSockets, auditKeyNew: after.Gabarit.MaxSockets})
	}
	if before.Gabarit.MaxCores != after.Gabarit.MaxCores {
		changes = append(changes, map[string]any{auditKeyField: "gabarit.maxCores", auditKeyOld: before.Gabarit.MaxCores, auditKeyNew: after.Gabarit.MaxCores})
	}
	if before.Gabarit.MaxMemoryMB != after.Gabarit.MaxMemoryMB {
		changes = append(changes, map[string]any{auditKeyField: "gabarit.maxMemoryMB", auditKeyOld: before.Gabarit.MaxMemoryMB, auditKeyNew: after.Gabarit.MaxMemoryMB})
	}
	if before.Gabarit.MaxDiskPerVMGB != after.Gabarit.MaxDiskPerVMGB {
		changes = append(changes, map[string]any{auditKeyField: "gabarit.maxDiskPerVmGb", auditKeyOld: before.Gabarit.MaxDiskPerVMGB, auditKeyNew: after.Gabarit.MaxDiskPerVMGB})
	}
	if before.Gabarit.MaxNetworkCards != after.Gabarit.MaxNetworkCards {
		changes = append(changes, map[string]any{auditKeyField: "gabarit.maxNetworkCards", auditKeyOld: before.Gabarit.MaxNetworkCards, auditKeyNew: after.Gabarit.MaxNetworkCards})
	}
	if before.Gabarit.MaxSnapshots != after.Gabarit.MaxSnapshots {
		changes = append(changes, map[string]any{auditKeyField: "gabarit.maxSnapshots", auditKeyOld: before.Gabarit.MaxSnapshots, auditKeyNew: after.Gabarit.MaxSnapshots})
	}
	if before.Gabarit.IsolationVLANTag != after.Gabarit.IsolationVLANTag {
		changes = append(changes, map[string]any{auditKeyField: "gabarit.isolationVlanTag", auditKeyOld: before.Gabarit.IsolationVLANTag, auditKeyNew: after.Gabarit.IsolationVLANTag})
	}
	return changes
}

func (handler *AdminPolicy) readPolicy(ctx context.Context, clusterName string) (policyResponse, error) {
	settings, err := handler.service.Settings(ctx, clusterName)
	if err != nil {
		return policyResponse{}, err
	}

	return policyResponse{Cluster: clusterName, Gabarit: policyGabaritDTOFromModel(settings.Gabarit), Quota: policyQuotaDTO{MaxVMPerUser: settings.Allowed}}, nil
}

func policyGabaritDTOFromModel(gabarit policy.Gabarit) policyGabaritDTO {
	return policyGabaritDTO{MaxSockets: gabarit.MaxSockets, MaxCores: gabarit.MaxCores, MaxMemoryMB: gabarit.MaxMemoryMB, MaxDiskPerVMGB: gabarit.MaxDiskPerVMGB, MaxNetworkCards: gabarit.MaxNetworkCards, MaxSnapshots: gabarit.MaxSnapshots, IsolationVLANTag: gabarit.IsolationVLANTag}
}

func applyGabaritPatch(gabarit *policy.Gabarit, patch *policyGabaritPatch) {
	if patch == nil {
		return
	}

	if patch.MaxSockets != nil {
		gabarit.MaxSockets = *patch.MaxSockets
	}

	if patch.MaxCores != nil {
		gabarit.MaxCores = *patch.MaxCores
	}

	if patch.MaxMemoryMB != nil {
		gabarit.MaxMemoryMB = *patch.MaxMemoryMB
	}

	if patch.MaxDiskPerVMGB != nil {
		gabarit.MaxDiskPerVMGB = *patch.MaxDiskPerVMGB
	}

	if patch.MaxNetworkCards != nil {
		gabarit.MaxNetworkCards = *patch.MaxNetworkCards
	}

	if patch.MaxSnapshots != nil {
		gabarit.MaxSnapshots = *patch.MaxSnapshots
	}

	if patch.IsolationVLANTag != nil {
		gabarit.IsolationVLANTag = *patch.IsolationVLANTag
	}
}

func applyQuotaPatch(quota *int, patch *policyQuotaPatch) {
	if patch != nil && patch.MaxVMPerUser != nil {
		*quota = *patch.MaxVMPerUser
	}
}

func (handler *AdminPolicy) writePolicyValidation(w http.ResponseWriter, err error) {
	if errors.Is(err, policy.ErrInvalidPolicy) {
		writeAdminError(w, http.StatusBadRequest, "invalid_policy", err.Error())
		return
	}

	if errors.Is(err, policy.ErrConcurrentUpdate) {
		writeAdminError(w, http.StatusConflict, "policy_conflict", "the policy was modified concurrently; reload and retry")
		return
	}

	handler.writeFailure(w, "write policy", err)
}

func (handler *AdminPolicy) writeFailure(w http.ResponseWriter, operation string, err error) {
	SetErrorMsg(w, operation+" failed", err)
	writeAdminError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

func (handler *AdminPolicy) recordAdminAction(r *http.Request, action, targetType, targetID, summary string, changes []any) {
	if handler.store == nil {
		return
	}

	actor, _ := handler.auth.Principal(r)
	if err := handler.store.RecordAdminAction(r.Context(), actor.Username, action, targetType, targetID, detailJSON(summary, changes), clientIP(r, handler.trustedProxyHops)); err != nil {
		logctx.FromOr(r.Context(), handler.log).WarnContext(r.Context(), "failed to record admin action", "component", "httpapi", "action", action, "error", err)
	}
}
