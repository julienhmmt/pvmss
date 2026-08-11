package httpapi

import (
	"errors"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/policy"
)

type nodePolicyResponse struct {
	Node          string `json:"node"`
	MaxVMs        int    `json:"maxVms"`
	MaxVCPUs      int    `json:"maxVcpus"`
	MaxRAMGB      int    `json:"maxRamGb"`
	MaxDiskGB     int    `json:"maxDiskGb"`
	UsedVMs       int    `json:"usedVms"`
	UsedVCPUs     int    `json:"usedVcpus"`
	UsedRAMGB     int    `json:"usedRamGb"`
	PhysicalVCPUs int    `json:"physicalVcpus"`
	PhysicalRAMGB int    `json:"physicalRamGb"`
}

type nodePolicyUpdateRequest struct {
	Cluster   string `json:"cluster"`
	MaxVMs    *int   `json:"maxVms"`
	MaxVCPUs  *int   `json:"maxVcpus"`
	MaxRAMGB  *int   `json:"maxRamGb"`
	MaxDiskGB *int   `json:"maxDiskGb"`
}

// ServePolicyNodes handles GET /api/v1/admin/policy/nodes.
func (handler *AdminPolicy) ServePolicyNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAdminError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	capacities, err := handler.service.NodeCapacities(r.Context(), queryCluster(r))
	if err != nil {
		handler.writeFailure(w, "list node capacities", err)
		return
	}
	response := make([]nodePolicyResponse, 0, len(capacities))
	for _, capacity := range capacities {
		response = append(response, nodePolicyResponseFromModel(capacity))
	}
	writeAdminJSON(w, http.StatusOK, response)
}

// ServePolicyNodeUpdate handles PUT /api/v1/admin/policy/nodes/{node}.
func (handler *AdminPolicy) ServePolicyNodeUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeAdminError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	node := r.PathValue("node")
	if node == "" {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "node is required")
		return
	}
	var request nodePolicyUpdateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	clusterName := resolveCluster(request.Cluster)
	current, err := handler.service.NodeCapacity(r.Context(), clusterName, node)
	if err != nil {
		handler.writeNodeFailure(w, node, err)
		return
	}
	applyNodePolicyPatch(&current, request)
	if err := handler.service.SetNodeCapacity(r.Context(), clusterName, node, current); err != nil {
		handler.writeNodeFailure(w, node, err)
		return
	}
	updated, err := handler.service.NodeCapacity(r.Context(), clusterName, node)
	if err != nil {
		handler.writeFailure(w, "read node capacity after update", err)
		return
	}
	capacities, err := handler.service.NodeCapacities(r.Context(), clusterName)
	if err != nil {
		handler.writeFailure(w, "read node capacity after update", err)
		return
	}
	for _, capacity := range capacities {
		if capacity.Node == node {
			updated.PhysicalVCPUs = capacity.PhysicalVCPUs
			updated.PhysicalRAMGB = capacity.PhysicalRAMGB
		}
	}
	writeAdminJSON(w, http.StatusOK, nodePolicyResponseFromModel(updated))
}

func applyNodePolicyPatch(capacity *policy.Capacity, request nodePolicyUpdateRequest) {
	if request.MaxVMs != nil {
		capacity.MaxVMs = *request.MaxVMs
	}
	if request.MaxVCPUs != nil {
		capacity.MaxVCPUs = *request.MaxVCPUs
	}
	if request.MaxRAMGB != nil {
		capacity.MaxRAMGB = *request.MaxRAMGB
	}
	if request.MaxDiskGB != nil {
		capacity.MaxDiskGB = *request.MaxDiskGB
	}
}

func nodePolicyResponseFromModel(capacity policy.Capacity) nodePolicyResponse {
	return nodePolicyResponse{Node: capacity.Node, MaxVMs: capacity.MaxVMs, MaxVCPUs: capacity.MaxVCPUs, MaxRAMGB: capacity.MaxRAMGB, MaxDiskGB: capacity.MaxDiskGB, UsedVMs: capacity.UsedVMs, UsedVCPUs: capacity.UsedVCPUs, UsedRAMGB: capacity.UsedRAMGB, PhysicalVCPUs: capacity.PhysicalVCPUs, PhysicalRAMGB: capacity.PhysicalRAMGB}
}

func (handler *AdminPolicy) writeNodeFailure(w http.ResponseWriter, node string, err error) {
	switch {
	case errors.Is(err, cluster.ErrNotFound):
		writeAdminError(w, http.StatusNotFound, "not_found", "node \""+node+"\" not reported by the cluster")
	case errors.Is(err, policy.ErrBelowCurrentUsage):
		writeAdminError(w, http.StatusBadRequest, "node_limit_below_usage", err.Error())
	case errors.Is(err, policy.ErrAboveNodeCapacity):
		writeAdminError(w, http.StatusBadRequest, "node_limit_above_capacity", err.Error())
	case errors.Is(err, policy.ErrInvalidPolicy):
		writeAdminError(w, http.StatusBadRequest, "invalid_policy", err.Error())
	default:
		handler.writeFailure(w, "write node capacity", err)
	}
}
