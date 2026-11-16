//nolint:wsl_v5 // policy transfer handlers keep cluster selection and mapping adjacent
package httpapi

import (
	"fmt"
	"net/http"
	"pvmss/server/internal/policy"
)

const (
	// maxPolicyImportBytes bounds a policy import body: every gabarit field
	// plus the cluster list fits well under it.
	maxPolicyImportBytes    = 64 << 10
	maxPolicyImportClusters = 100
	// maxNodeImportBytes bounds a node-capacity import body: 500 nodes fit
	// well under it.
	maxNodeImportBytes = 256 << 10
	maxNodeImportItems = 500
)

type policyImportRequest struct {
	Clusters []string            `json:"clusters"`
	Gabarit  *policyGabaritPatch `json:"gabarit"`
	Quota    *policyQuotaPatch   `json:"quota"`
}

type policyImportOutcomeDTO struct {
	Cluster string `json:"cluster"`
	Result  string `json:"result"`
	Error   string `json:"error,omitempty"`
}

type nodeImportItemDTO struct {
	Node      string `json:"node"`
	MaxVMs    *int   `json:"maxVms"`
	MaxVCPUs  *int   `json:"maxVcpus"`
	MaxRAMGB  *int   `json:"maxRamGb"`
	MaxDiskGB *int   `json:"maxDiskGb"`
}

type nodeImportRequest struct {
	Cluster   string              `json:"cluster"`
	Overwrite bool                `json:"overwrite"`
	Nodes     []nodeImportItemDTO `json:"nodes"`
}

type nodeImportOutcomeDTO struct {
	Node   string `json:"node"`
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

// ServePolicyImport handles POST /api/v1/admin/policy/import - writes one
// exported policy to each target cluster and reports a per-cluster outcome.
func (handler *AdminPolicy) ServePolicyImport(w http.ResponseWriter, r *http.Request) {
	var request policyImportRequest
	if err := decodeJSONLimit(w, r, &request, maxPolicyImportBytes); err != nil ||
		len(request.Clusters) == 0 || len(request.Clusters) > maxPolicyImportClusters {
		writeAdminError(w, http.StatusBadRequest, codeInvalidRequest, msgInvalidRequestBody)
		return
	}

	settings, ok := importedSettings(request)
	if !ok {
		writeAdminError(w, http.StatusBadRequest, codeInvalidRequest, "gabarit and quota must list every field")
		return
	}

	clusters, err := handler.resolveClusters(request.Clusters)
	if err != nil {
		status, code, message := clusterParamError(err)
		writeAdminError(w, status, code, message)
		return
	}

	outcomes, err := handler.service.ImportPolicy(r.Context(), clusters, settings)

	dto := make([]policyImportOutcomeDTO, len(outcomes))
	counts := map[string]int{}

	for i, outcome := range outcomes {
		dto[i] = policyImportOutcomeDTO{Cluster: outcome.Cluster, Result: outcome.Result, Error: outcome.Error}
		counts[outcome.Result]++
	}

	// A mid-import failure still returns the outcomes already written: audit
	// them so a partial import never lands silently.
	if len(outcomes) > 0 {
		handler.recordAdminAction(r, "admin.policy.import", "policy", "",
			fmt.Sprintf("imported policy to %d cluster(s): %d updated, %d unchanged, %d invalid, %d failed",
				len(clusters), counts[policy.ImportUpdated], counts[policy.ImportUnchanged],
				counts[policy.ImportInvalid], counts[policy.ImportFailed]),
			[]any{map[string]any{"clusters": clusters, "results": counts}})
	}
	if err != nil {
		handler.writeFailure(w, "import policy", err)
		return
	}

	writeAdminJSON(w, http.StatusOK, dto)
}

// ServePolicyNodesImport handles POST /api/v1/admin/policy/nodes/import -
// writes exported node caps to one cluster and reports a per-node outcome.
func (handler *AdminPolicy) ServePolicyNodesImport(w http.ResponseWriter, r *http.Request) {
	var request nodeImportRequest
	if err := decodeJSONLimit(w, r, &request, maxNodeImportBytes); err != nil ||
		len(request.Nodes) == 0 || len(request.Nodes) > maxNodeImportItems {
		writeAdminError(w, http.StatusBadRequest, codeInvalidRequest, msgInvalidRequestBody)
		return
	}

	clusterName, clusterErr := ResolveClusterValue(request.Cluster, handler.clusters)
	if clusterErr != nil {
		status, code, message := clusterParamError(clusterErr)
		writeAdminError(w, status, code, message)
		return
	}

	items, ok := importedCapacities(request.Nodes)
	if !ok {
		writeAdminError(w, http.StatusBadRequest, codeInvalidRequest, "every node needs node, maxVms, maxVcpus, maxRamGb and maxDiskGb")
		return
	}

	outcomes, err := handler.service.ImportNodeCapacities(r.Context(), clusterName, items, request.Overwrite)

	dto := make([]nodeImportOutcomeDTO, len(outcomes))
	counts := map[string]int{}

	for i, outcome := range outcomes {
		dto[i] = nodeImportOutcomeDTO{Node: outcome.Node, Result: outcome.Result, Error: outcome.Error}
		counts[outcome.Result]++
	}

	// Same partial-write rule as the policy import above.
	if len(outcomes) > 0 {
		handler.recordAdminAction(r, "admin.policy.nodes.import", "node_capacity", clusterName,
			fmt.Sprintf("imported node capacity on cluster %s: %d updated, %d unchanged, %d skipped, %d not found, %d invalid",
				clusterName, counts[policy.ImportUpdated], counts[policy.ImportUnchanged], counts[policy.ImportSkipped],
				counts[policy.ImportNotFound], counts[policy.ImportInvalid]),
			[]any{map[string]any{auditKeyCluster: clusterName, "overwrite": request.Overwrite, "results": counts}})
	}
	if err != nil {
		handler.writeFailure(w, "import node capacities", err)
		return
	}

	writeAdminJSON(w, http.StatusOK, dto)
}

func (handler *AdminPolicy) resolveClusters(names []string) ([]string, error) {
	clusters := make([]string, 0, len(names))

	for _, name := range names {
		clusterName, err := ResolveClusterValue(name, handler.clusters)
		if err != nil {
			return nil, err
		}

		clusters = append(clusters, clusterName)
	}

	return clusters, nil
}

// importedSettings turns an import body into policy settings. Every field is
// required: a missing one would silently uncap that dimension.
func importedSettings(request policyImportRequest) (policy.Settings, bool) {
	gabarit, ok := completeGabarit(request.Gabarit)
	if !ok || request.Quota == nil || request.Quota.MaxVMPerUser == nil {
		return policy.Settings{}, false
	}

	return policy.Settings{Gabarit: gabarit, Allowed: *request.Quota.MaxVMPerUser}, true
}

func completeGabarit(patch *policyGabaritPatch) (policy.Gabarit, bool) {
	if patch == nil {
		return policy.Gabarit{}, false
	}

	fields := []*int{
		patch.MaxSockets, patch.MaxCores, patch.MaxMemoryMB, patch.MaxDiskPerVMGB,
		patch.MaxNetworkCards, patch.MaxSnapshots, patch.IsolationVLANTag,
		patch.VMIDMin, patch.VMIDMax, patch.LeaseDays,
	}
	for _, field := range fields {
		if field == nil {
			return policy.Gabarit{}, false
		}
	}

	return policy.Gabarit{
		MaxSockets: *patch.MaxSockets, MaxCores: *patch.MaxCores, MaxMemoryMB: *patch.MaxMemoryMB,
		MaxDiskPerVMGB: *patch.MaxDiskPerVMGB, MaxNetworkCards: *patch.MaxNetworkCards,
		MaxSnapshots: *patch.MaxSnapshots, IsolationVLANTag: *patch.IsolationVLANTag,
		VMIDMin: *patch.VMIDMin, VMIDMax: *patch.VMIDMax, LeaseDays: *patch.LeaseDays,
	}, true
}

// importedCapacities turns import rows into capacities. Every cap is required
// for the same reason as the policy fields.
func importedCapacities(nodes []nodeImportItemDTO) ([]policy.NodeCapacityImport, bool) {
	items := make([]policy.NodeCapacityImport, len(nodes))

	for i, node := range nodes {
		if node.Node == "" || node.MaxVMs == nil || node.MaxVCPUs == nil || node.MaxRAMGB == nil || node.MaxDiskGB == nil {
			return nil, false
		}

		items[i] = policy.NodeCapacityImport{
			Node: node.Node,
			Capacity: policy.Capacity{
				MaxVMs: *node.MaxVMs, MaxVCPUs: *node.MaxVCPUs,
				MaxRAMGB: *node.MaxRAMGB, MaxDiskGB: *node.MaxDiskGB,
			},
		}
	}

	return items, true
}
