//nolint:wsl_v5 // node policy handlers keep validation and cluster mapping adjacent
package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/policy"
	"time"
)

type nodePolicyResponse struct {
	Node          string `json:"node"`
	Status        string `json:"status"`
	MaxVMs        int    `json:"maxVms"`
	MaxVCPUs      int    `json:"maxVcpus"`
	MaxRAMGB      int    `json:"maxRamGb"`
	MaxDiskGB     int    `json:"maxDiskGb"`
	UsedVMs       int    `json:"usedVms"`
	UsedVCPUs     int    `json:"usedVcpus"`
	UsedRAMGB     int    `json:"usedRamGb"`
	UsedDiskGB    int    `json:"usedDiskGb"`
	PhysicalVCPUs int    `json:"physicalVcpus"`
	PhysicalRAMGB int    `json:"physicalRamGb"`
	// Live, all-VMs node load (versus the pvmss-tagged Used* aggregate).
	NodeCPUUsage       float64 `json:"nodeCpuUsage"`
	NodeMemoryUsedGB   int     `json:"nodeMemoryUsedGb"`
	NodeStorageUsedGB  int     `json:"nodeStorageUsedGb"`
	NodeStorageTotalGB int     `json:"nodeStorageTotalGb"`
	TotalVMs           int     `json:"totalVms"`
}

// nodePolicyListResponse wraps the list with the inventory refresh timestamp
// so the page can say how old the live figures are.
type nodePolicyListResponse struct {
	Nodes       []nodePolicyResponse `json:"nodes"`
	RefreshedAt string               `json:"refreshedAt"`
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

	clusterName, clusterErr := ResolveClusterParam(r, handler.clusters)
	if clusterErr != nil {
		code, message := clusterParamError(clusterErr)
		writeAdminError(w, http.StatusBadRequest, code, message)
		return
	}
	capacities, err := handler.service.NodeCapacities(r.Context(), clusterName)
	if err != nil {
		handler.writeFailure(w, "list node capacities", err)
		return
	}

	response := nodePolicyListResponse{Nodes: make([]nodePolicyResponse, 0, len(capacities))}
	for _, capacity := range capacities {
		response.Nodes = append(response.Nodes, nodePolicyResponseFromModel(capacity))
	}

	if refreshedAt := handler.service.RefreshedAt(); !refreshedAt.IsZero() {
		response.RefreshedAt = refreshedAt.UTC().Format(time.RFC3339Nano)
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

	clusterName, clusterErr := ResolveClusterValue(request.Cluster, handler.clusters)
	if clusterErr != nil {
		code, message := clusterParamError(clusterErr)
		writeAdminError(w, http.StatusBadRequest, code, message)
		return
	}

	current, err := handler.service.NodeCapacity(r.Context(), clusterName, node)
	if err != nil {
		handler.writeNodeFailure(w, node, err)
		return
	}

	original := current
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

	changes := nodeCapacityChangeDiff(original, updated)
	handler.recordAdminAction(r, "admin.policy.nodes.update", "node_capacity", node,
		fmt.Sprintf("updated node capacity for %s on cluster %s", node, clusterName), changes)
	writeAdminJSON(w, http.StatusOK, nodePolicyResponseFromModel(updated))
}

// nodeCapacityChangeDiff compares before/after node capacity snapshots and
// returns change entries for the audit detail payload.
func nodeCapacityChangeDiff(original, updated policy.Capacity) []any {
	changes := []any{}
	if original.MaxVMs != updated.MaxVMs {
		changes = append(changes, map[string]any{auditKeyField: "maxVms", auditKeyOld: original.MaxVMs, auditKeyNew: updated.MaxVMs})
	}
	if original.MaxVCPUs != updated.MaxVCPUs {
		changes = append(changes, map[string]any{auditKeyField: "maxVcpus", auditKeyOld: original.MaxVCPUs, auditKeyNew: updated.MaxVCPUs})
	}
	if original.MaxRAMGB != updated.MaxRAMGB {
		changes = append(changes, map[string]any{auditKeyField: "maxRamGb", auditKeyOld: original.MaxRAMGB, auditKeyNew: updated.MaxRAMGB})
	}
	if original.MaxDiskGB != updated.MaxDiskGB {
		changes = append(changes, map[string]any{auditKeyField: "maxDiskGb", auditKeyOld: original.MaxDiskGB, auditKeyNew: updated.MaxDiskGB})
	}
	return changes
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
	return nodePolicyResponse{
		Node: capacity.Node, Status: string(capacity.Status),
		MaxVMs: capacity.MaxVMs, MaxVCPUs: capacity.MaxVCPUs, MaxRAMGB: capacity.MaxRAMGB, MaxDiskGB: capacity.MaxDiskGB,
		UsedVMs: capacity.UsedVMs, UsedVCPUs: capacity.UsedVCPUs, UsedRAMGB: capacity.UsedRAMGB, UsedDiskGB: capacity.UsedDiskGB,
		PhysicalVCPUs: capacity.PhysicalVCPUs, PhysicalRAMGB: capacity.PhysicalRAMGB,
		NodeCPUUsage: capacity.CPUUsage, NodeMemoryUsedGB: capacity.MemoryUsedGB,
		NodeStorageUsedGB: capacity.StorageUsedGB, NodeStorageTotalGB: capacity.StorageTotalGB,
		TotalVMs: capacity.TotalVMs,
	}
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
