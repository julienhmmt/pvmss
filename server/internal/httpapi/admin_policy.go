//nolint:wsl_v5 // policy handlers keep cluster selection and response mapping adjacent
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/auth"
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
	MaxSockets       int  `json:"maxSockets"`
	MaxCores         int  `json:"maxCores"`
	MaxMemoryMB      int  `json:"maxMemoryMB"`
	MaxDiskPerVMGB   int  `json:"maxDiskPerVmGb"`
	MaxNetworkCards  int  `json:"maxNetworkCards"`
	MaxSnapshots     int  `json:"maxSnapshots"`
	AllowCustomYAML  bool `json:"allowCustomYaml"`
	IsolationVLANTag int  `json:"isolationVlanTag"`
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
	MaxSockets       *int  `json:"maxSockets"`
	MaxCores         *int  `json:"maxCores"`
	MaxMemoryMB      *int  `json:"maxMemoryMB"`
	MaxDiskPerVMGB   *int  `json:"maxDiskPerVmGb"`
	MaxNetworkCards  *int  `json:"maxNetworkCards"`
	MaxSnapshots     *int  `json:"maxSnapshots"`
	AllowCustomYAML  *bool `json:"allowCustomYaml"`
	IsolationVLANTag *int  `json:"isolationVlanTag"`
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

	current, err := handler.readPolicy(r.Context(), clusterName)
	if err != nil {
		handler.writeFailure(w, "read policy before update", err)
		return
	}

	gabarit := gabaritFromDTO(current.Gabarit)
	quota := current.Quota.MaxVMPerUser

	applyGabaritPatch(&gabarit, request.Gabarit)
	applyQuotaPatch(&quota, request.Quota)

	if err := handler.service.SetPolicy(r.Context(), clusterName, gabarit, quota); err != nil {
		handler.writePolicyValidation(w, err)
		return
	}

	updated, err := handler.readPolicy(r.Context(), clusterName)
	if err != nil {
		handler.writeFailure(w, "read policy after update", err)
		return
	}

	changes := policyChangeDiff(current, updated)
	handler.recordAdminAction(r, "admin.policy.update", "policy", clusterName,
		"updated policy for cluster "+clusterName, changes)
	writeAdminJSON(w, http.StatusOK, updated)
}

// policyChangeDiff compares the before/after policy snapshots and returns a
// list of change entries for the audit detail payload.
func policyChangeDiff(current, updated policyResponse) []any {
	changes := []any{}
	if current.Quota.MaxVMPerUser != updated.Quota.MaxVMPerUser {
		changes = append(changes, map[string]any{auditKeyField: "quota.maxVmPerUser", auditKeyOld: current.Quota.MaxVMPerUser, auditKeyNew: updated.Quota.MaxVMPerUser})
	}
	if current.Gabarit.MaxSockets != updated.Gabarit.MaxSockets {
		changes = append(changes, map[string]any{auditKeyField: "gabarit.maxSockets", auditKeyOld: current.Gabarit.MaxSockets, auditKeyNew: updated.Gabarit.MaxSockets})
	}
	if current.Gabarit.MaxCores != updated.Gabarit.MaxCores {
		changes = append(changes, map[string]any{auditKeyField: "gabarit.maxCores", auditKeyOld: current.Gabarit.MaxCores, auditKeyNew: updated.Gabarit.MaxCores})
	}
	if current.Gabarit.MaxMemoryMB != updated.Gabarit.MaxMemoryMB {
		changes = append(changes, map[string]any{auditKeyField: "gabarit.maxMemoryMB", auditKeyOld: current.Gabarit.MaxMemoryMB, auditKeyNew: updated.Gabarit.MaxMemoryMB})
	}
	if current.Gabarit.MaxDiskPerVMGB != updated.Gabarit.MaxDiskPerVMGB {
		changes = append(changes, map[string]any{auditKeyField: "gabarit.maxDiskPerVmGb", auditKeyOld: current.Gabarit.MaxDiskPerVMGB, auditKeyNew: updated.Gabarit.MaxDiskPerVMGB})
	}
	if current.Gabarit.MaxNetworkCards != updated.Gabarit.MaxNetworkCards {
		changes = append(changes, map[string]any{auditKeyField: "gabarit.maxNetworkCards", auditKeyOld: current.Gabarit.MaxNetworkCards, auditKeyNew: updated.Gabarit.MaxNetworkCards})
	}
	if current.Gabarit.MaxSnapshots != updated.Gabarit.MaxSnapshots {
		changes = append(changes, map[string]any{auditKeyField: "gabarit.maxSnapshots", auditKeyOld: current.Gabarit.MaxSnapshots, auditKeyNew: updated.Gabarit.MaxSnapshots})
	}
	if current.Gabarit.AllowCustomYAML != updated.Gabarit.AllowCustomYAML {
		changes = append(changes, map[string]any{auditKeyField: "gabarit.allowCustomYaml", auditKeyOld: current.Gabarit.AllowCustomYAML, auditKeyNew: updated.Gabarit.AllowCustomYAML})
	}
	if current.Gabarit.IsolationVLANTag != updated.Gabarit.IsolationVLANTag {
		changes = append(changes, map[string]any{auditKeyField: "gabarit.isolationVlanTag", auditKeyOld: current.Gabarit.IsolationVLANTag, auditKeyNew: updated.Gabarit.IsolationVLANTag})
	}
	return changes
}

func (handler *AdminPolicy) readPolicy(ctx context.Context, clusterName string) (policyResponse, error) {
	gabarit, err := handler.service.Gabarit(ctx, clusterName)
	if err != nil {
		return policyResponse{}, err
	}

	quota, err := handler.service.Quota(ctx, clusterName, auth.Identity{})
	if err != nil {
		return policyResponse{}, err
	}

	return policyResponse{Cluster: clusterName, Gabarit: policyGabaritDTOFromModel(gabarit), Quota: policyQuotaDTO{MaxVMPerUser: quota.Allowed}}, nil
}

func gabaritFromDTO(dto policyGabaritDTO) policy.Gabarit {
	return policy.Gabarit{MaxSockets: dto.MaxSockets, MaxCores: dto.MaxCores, MaxMemoryMB: dto.MaxMemoryMB, MaxDiskPerVMGB: dto.MaxDiskPerVMGB, MaxNetworkCards: dto.MaxNetworkCards, MaxSnapshots: dto.MaxSnapshots, AllowCustomYAML: dto.AllowCustomYAML, IsolationVLANTag: dto.IsolationVLANTag}
}

func policyGabaritDTOFromModel(gabarit policy.Gabarit) policyGabaritDTO {
	return policyGabaritDTO{MaxSockets: gabarit.MaxSockets, MaxCores: gabarit.MaxCores, MaxMemoryMB: gabarit.MaxMemoryMB, MaxDiskPerVMGB: gabarit.MaxDiskPerVMGB, MaxNetworkCards: gabarit.MaxNetworkCards, MaxSnapshots: gabarit.MaxSnapshots, AllowCustomYAML: gabarit.AllowCustomYAML, IsolationVLANTag: gabarit.IsolationVLANTag}
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

	if patch.AllowCustomYAML != nil {
		gabarit.AllowCustomYAML = *patch.AllowCustomYAML
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

	handler.writeFailure(w, "write policy", err)
}

func (handler *AdminPolicy) writeFailure(w http.ResponseWriter, operation string, err error) {
	handler.log.Error(operation+" failed", "component", "httpapi", "error", err)
	writeAdminError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

func (handler *AdminPolicy) recordAdminAction(r *http.Request, action, targetType, targetID, summary string, changes []any) {
	if handler.store == nil {
		return
	}

	actor, _ := handler.auth.Principal(r)
	if err := handler.store.RecordAdminAction(r.Context(), actor.Username, action, targetType, targetID, detailJSON(summary, changes), clientIP(r, handler.trustedProxyHops)); err != nil {
		handler.log.Error("failed to record admin action", "component", "httpapi", "action", action, "error", err)
	}
}
