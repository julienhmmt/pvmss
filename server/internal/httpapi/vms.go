//nolint:wsl_v5 // list parsing and response mapping stay in one handler flow
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strconv"
)

// VMs serves GET /api/v1/vms — the ONLY VM-listing endpoint in the system
// (FR-001, SC-001). It reads the inventory projection, never the cluster
// client (constitution IV), and enforces scope server-side via vm.List
// (FR-003).
type VMs struct {
	projection   *inventory.Projection
	source       inventory.Source
	auth         *Auth
	maxPageSize  int
	quota        int
	policy       *policy.Policy
	clusterStore *store.Store
	log          *slog.Logger
}

// NewVMs creates the handler for the given inventory projection. maxPageSize
// caps an accepted pageSize (rejected, not truncated); quota is the per-user
// allowance reported to non-admin callers (-1 = unlimited, V07).
func NewVMs(projection *inventory.Projection, authHandler *Auth, maxPageSize, quota int, log *slog.Logger, services ...*policy.Policy) *VMs {
	var policyService *policy.Policy
	if len(services) > 0 {
		policyService = services[0]
	}

	return &VMs{projection: projection, source: projection, auth: authHandler, maxPageSize: maxPageSize, quota: quota, policy: policyService, log: log}
}

// NewVMsWithRegistry creates the cross-cluster VM list handler. clusterStore
// resolves each cluster's real Proxmox display name (discovered via the admin
// "test connection" flow) for the response; nil skips display-name lookup.
func NewVMsWithRegistry(registry inventory.Source, authHandler *Auth, maxPageSize, quota int, log *slog.Logger, clusterStore *store.Store, services ...*policy.Policy) *VMs {
	var policyService *policy.Policy
	if len(services) > 0 {
		policyService = services[0]
	}
	return &VMs{source: registry, auth: authHandler, maxPageSize: maxPageSize, quota: quota, policy: policyService, clusterStore: clusterStore, log: log}
}

type vmDTO struct {
	Cluster            string   `json:"cluster"`
	ClusterDisplayName string   `json:"clusterDisplayName"`
	VMID               int      `json:"vmid"`
	Name               string   `json:"name"`
	Node               string   `json:"node"`
	Status             string   `json:"status"`
	Pool               string   `json:"pool"`
	Tags               []string `json:"tags"`
	CPUCores           int      `json:"cpuCores"`
	MemoryTotal        int64    `json:"memoryTotal"`
}

type quotaDTO struct {
	Used    int `json:"used"`
	Allowed int `json:"allowed"`
}

type vmListResponse struct {
	Items          []vmDTO   `json:"items"`
	Total          int       `json:"total"`
	Page           int       `json:"page"`
	PageSize       int       `json:"pageSize"`
	AvailableNodes []string  `json:"availableNodes"`
	EmptyReason    string    `json:"emptyReason,omitempty"`
	Quota          *quotaDTO `json:"quota,omitempty"`
}

func (h *VMs) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		h.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")

		return
	}

	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}

	query, queryErr := h.parseQuery(r)
	if queryErr != nil {
		h.writeError(w, http.StatusBadRequest, queryErr.code, queryErr.message)
		return
	}

	if !sourceHasReadyIndex(h.source, query.Cluster) {
		h.writeError(w, http.StatusServiceUnavailable, "inventory_not_ready", "inventory has not been populated yet")
		return
	}

	result, err := vm.ListWithContext(r.Context(), h.source, query, identity, h.quota, h.policy)
	if err != nil {
		if errors.Is(err, vm.ErrInvalidSortBy) {
			h.writeError(w, http.StatusBadRequest, "invalid_sort_column", fmt.Sprintf("cannot sort by %q", query.SortBy))
			return
		}

		h.log.Error("vm list failed", "component", "httpapi", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")

		return
	}

	h.writeList(r.Context(), w, result)
}

type queryError struct {
	code    string
	message string
}

// parseQuery reads the request's list parameters. Unknown or malformed values
// are rejected explicitly (constitution XIII) — never silently defaulted,
// except scope, which vm.List re-derives from the identity regardless.
func (h *VMs) parseQuery(r *http.Request) (vm.ListQuery, *queryError) {
	params := r.URL.Query()
	query := vm.ListQuery{
		Cluster: params.Get("cluster"),
		Search:  params.Get("search"),
		Status:  cluster.VMStatus(params.Get("status")),
		Node:    params.Get("node"),
		SortBy:  vm.SortBy(params.Get("sortBy")),
		SortDir: vm.SortDir(params.Get("sortDir")),
		Scope:   vm.Scope(params.Get("scope")),
	}

	page, err := parseOptionalInt(params.Get("page"))
	if err != nil {
		return query, &queryError{code: "invalid_request", message: "page must be an integer"}
	}

	query.Page = page

	pageSize, err := parseOptionalInt(params.Get("pageSize"))
	if err != nil {
		return query, &queryError{code: "invalid_request", message: "pageSize must be an integer"}
	}

	if pageSize > h.maxPageSize {
		return query, &queryError{code: "page_size_too_large", message: fmt.Sprintf("pageSize exceeds the maximum of %d", h.maxPageSize)}
	}

	query.PageSize = pageSize

	return query, nil
}

func parseOptionalInt(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %q: %w", raw, err)
	}

	return value, nil
}

func sourceHasReadyIndex(source inventory.Source, clusterName string) bool {
	if source == nil {
		return false
	}
	indexes := source.All()
	if clusterName != "" {
		index, ok := indexes[clusterName]
		if ok {
			return index != nil
		}
		if len(indexes) == 1 {
			for _, index := range indexes {
				return index != nil
			}
		}
		return false
	}
	for _, index := range indexes {
		if index != nil {
			return true
		}
	}
	return false
}

func (h *VMs) writeList(ctx context.Context, w http.ResponseWriter, result vm.ListResult) {
	displayNames := h.clusterDisplayNames(ctx)
	response := vmListResponse{
		Items:          make([]vmDTO, len(result.Items)),
		Total:          result.Total,
		Page:           result.Page,
		PageSize:       result.PageSize,
		AvailableNodes: result.AvailableNodes,
		EmptyReason:    string(result.EmptyReason),
	}
	for i, machine := range result.Items {
		displayName := displayNames[machine.Cluster]
		if displayName == "" {
			displayName = machine.Cluster
		}
		response.Items[i] = vmDTO{
			Cluster:            machine.Cluster,
			ClusterDisplayName: displayName,
			VMID:               machine.VMID,
			Name:               machine.Name,
			Node:               machine.Node,
			Status:             string(machine.Status),
			Pool:               machine.Pool,
			Tags:               machine.Tags,
			CPUCores:           machine.CPUCores,
			MemoryTotal:        machine.MemoryTotal,
		}
	}

	if result.Quota != nil {
		response.Quota = &quotaDTO{Used: result.Quota.Used, Allowed: result.Quota.Allowed}
	}

	body, err := json.Marshal(response)
	if err != nil {
		h.log.Error("failed to marshal vm list response", "component", "httpapi", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")

		return
	}

	if err := writeJSON(w, http.StatusOK, body); err != nil {
		h.log.Error("failed to write vm list response", "component", "httpapi", "error", err)
	}
}

// clusterDisplayNames maps each configured cluster's internal name to its
// real Proxmox cluster name, discovered via the admin "test connection" flow
// (store.SetClusterDisplayName). Empty when clusterStore is nil or a row has
// no display name yet — callers fall back to the internal name.
func (h *VMs) clusterDisplayNames(ctx context.Context) map[string]string {
	if h.clusterStore == nil {
		return nil
	}
	rows, err := h.clusterStore.ListClusters(ctx)
	if err != nil {
		h.log.Warn("list clusters for display names failed", "component", "httpapi", "error", err)
		return nil
	}
	names := make(map[string]string, len(rows))
	for _, row := range rows {
		names[row.Name] = row.DisplayName
	}
	return names
}

func (h *VMs) writeError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Error("failed to write error response", "component", "httpapi", "code", code, "error", err)
	}
}
