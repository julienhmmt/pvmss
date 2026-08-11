package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/policy"
)

// AdminPolicy serves the single global gabarit/quota surface and the dedicated
// node-capacité surface. Router registration supplies the admin role guard.
type AdminPolicy struct {
	service *policy.Policy
	log     *slog.Logger
}

// NewAdminPolicy creates the policy admin handler.
func NewAdminPolicy(_ *Auth, service *policy.Policy, log *slog.Logger) *AdminPolicy {
	return &AdminPolicy{service: service, log: log}
}

type policyResponse struct {
	Cluster string           `json:"cluster"`
	Gabarit policyGabaritDTO `json:"gabarit"`
	Quota   policyQuotaDTO   `json:"quota"`
}

type policyGabaritDTO struct {
	MaxSockets      int  `json:"maxSockets"`
	MaxCores        int  `json:"maxCores"`
	MaxMemoryMB     int  `json:"maxMemoryMB"`
	MaxDiskPerVMGB  int  `json:"maxDiskPerVmGb"`
	MaxNetworkCards int  `json:"maxNetworkCards"`
	MaxSnapshots    int  `json:"maxSnapshots"`
	AllowCustomYaml bool `json:"allowCustomYaml"`
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
	MaxSockets      *int  `json:"maxSockets"`
	MaxCores        *int  `json:"maxCores"`
	MaxMemoryMB     *int  `json:"maxMemoryMB"`
	MaxDiskPerVMGB  *int  `json:"maxDiskPerVmGb"`
	MaxNetworkCards *int  `json:"maxNetworkCards"`
	MaxSnapshots    *int  `json:"maxSnapshots"`
	AllowCustomYaml *bool `json:"allowCustomYaml"`
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
	clusterName := queryCluster(r)
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
	clusterName := resolveCluster(request.Cluster)
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
	writeAdminJSON(w, http.StatusOK, updated)
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
	return policy.Gabarit{MaxSockets: dto.MaxSockets, MaxCores: dto.MaxCores, MaxMemoryMB: dto.MaxMemoryMB, MaxDiskPerVMGB: dto.MaxDiskPerVMGB, MaxNetworkCards: dto.MaxNetworkCards, MaxSnapshots: dto.MaxSnapshots, AllowCustomYaml: dto.AllowCustomYaml}
}

func policyGabaritDTOFromModel(gabarit policy.Gabarit) policyGabaritDTO {
	return policyGabaritDTO{MaxSockets: gabarit.MaxSockets, MaxCores: gabarit.MaxCores, MaxMemoryMB: gabarit.MaxMemoryMB, MaxDiskPerVMGB: gabarit.MaxDiskPerVMGB, MaxNetworkCards: gabarit.MaxNetworkCards, MaxSnapshots: gabarit.MaxSnapshots, AllowCustomYaml: gabarit.AllowCustomYaml}
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
	if patch.AllowCustomYaml != nil {
		gabarit.AllowCustomYaml = *patch.AllowCustomYaml
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
