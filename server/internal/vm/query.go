// Package vm resolves VM list queries against the inventory projection.
// List is the single read path behind GET /api/v1/vms — scope enforcement,
// search classification, filtering, sorting, and pagination all happen here,
// in one pure function with no I/O (T04 data-model.md).
package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"slices"
	"strconv"
	"strings"
)

// ErrInvalidSortBy rejects a sort column the list does not support (FR-005) —
// never silently defaulted.
var ErrInvalidSortBy = errors.New("invalid sort column")

// Scope is the requested result perimeter. All is honoured only for an admin
// caller; any other caller is silently treated as Mine (FR-003 — an override,
// never an error that would confirm the parameter exists to a probing caller).
type Scope string

// List scopes: "mine" limits to the caller's pool, "all" spans every pool (admin only).
const (
	ScopeMine Scope = "mine"
	ScopeAll  Scope = "all"
)

// SortBy is a column the list can be ordered by.
type SortBy string

// Sort columns accepted by the list endpoint.
const (
	SortByVMID   SortBy = "vmid"
	SortByName   SortBy = "name"
	SortByNode   SortBy = "node"
	SortByStatus SortBy = "status"
	SortByCPU    SortBy = "cpu"
	SortByMemory SortBy = "memory"
)

// SortDir is the ordering direction.
type SortDir string

// Sort directions for the list endpoint.
const (
	SortAsc  SortDir = "asc"
	SortDesc SortDir = "desc"
)

// EmptyReason distinguishes why a result page is empty (FR-008): the caller
// owns no VMs at all, or the current search/filters match none of them.
type EmptyReason string

// Empty reasons surfaced to the UI when a result page is empty.
const (
	EmptyNoVMsOwned EmptyReason = "no_vms_owned"
	EmptyNoMatch    EmptyReason = "no_match"
)

const (
	defaultPage     = 1
	defaultPageSize = 10
)

// ListQuery is the resolved combination of search, filters, sort, page, and
// scope. Scope is a requested value only — List re-derives the effective
// scope from the caller's identity and never trusts it as-is (FR-003).
type ListQuery struct {
	Search   string
	Status   cluster.VMStatus
	Node     string
	SortBy   SortBy
	SortDir  SortDir
	Page     int
	PageSize int
	Scope    Scope
}

// Quota is the caller's VM count against their allowance. Allowed == -1 means
// unlimited (V07 convention).
type Quota struct {
	Used    int
	Allowed int
}

// ListResult is one page of VMs plus everything the frontend needs to render
// the list's chrome: total before pagination, the page actually served after
// clamping, the node facet, and the empty-state distinction.
type ListResult struct {
	Items          []cluster.VM
	Total          int
	Page           int
	PageSize       int
	AvailableNodes []string
	EmptyReason    EmptyReason
	Quota          *Quota
}

// List resolves query against the index snapshot for identity. It is pure —
// no I/O, no mutation of the index — and is the only branch point for scope
// in the whole request path (SC-005). allowedQuota is the configured per-user
// VM allowance reported in Quota (-1 = unlimited); the quota is attached
// whenever the caller is not an admin, or an admin listing their own pool
// (spec Assumptions 5.3).
func List(index *inventory.Index, query ListQuery, identity auth.Identity, allowedQuota int, services ...*policy.Policy) (ListResult, error) {
	return list(context.Background(), index, query, identity, allowedQuota, services...)
}

// ListWithContext resolves a VM list while allowing policy reads to honor request cancellation.
func ListWithContext(ctx context.Context, index *inventory.Index, query ListQuery, identity auth.Identity, allowedQuota int, services ...*policy.Policy) (ListResult, error) {
	return list(ctx, index, query, identity, allowedQuota, services...)
}

func list(ctx context.Context, index *inventory.Index, query ListQuery, identity auth.Identity, allowedQuota int, services ...*policy.Policy) (ListResult, error) {
	query = withDefaults(query)
	if !validSortBy(query.SortBy) {
		return ListResult{}, fmt.Errorf("%w: %q", ErrInvalidSortBy, query.SortBy)
	}

	scoped := scopedVMs(index, query, identity)
	searched := searchVMs(scoped, query.Search)
	facetedNodes := nodeFacet(searched)
	filtered := filterVMs(searched, query)
	sortVMs(filtered, query.SortBy, query.SortDir)

	result := ListResult{
		Total:          len(filtered),
		Page:           query.Page,
		PageSize:       query.PageSize,
		AvailableNodes: facetedNodes,
	}
	if len(services) > 0 && services[0] != nil {
		quota, err := services[0].Quota(ctx, "default", identity)
		if err != nil {
			return ListResult{}, fmt.Errorf("read quota: %w", err)
		}
		allowedQuota = quota.Allowed
	}
	if query.Scope != ScopeAll || !identity.IsAdmin {
		result.Quota = &Quota{Used: len(scoped), Allowed: allowedQuota}
	}

	if len(filtered) == 0 {
		result.EmptyReason = EmptyNoMatch
		if len(scoped) == 0 {
			result.EmptyReason = EmptyNoVMsOwned
		}

		result.Items = []cluster.VM{}

		return result, nil
	}

	maxPage := (len(filtered) + query.PageSize - 1) / query.PageSize
	if result.Page > maxPage {
		result.Page = maxPage
	}

	start := (result.Page - 1) * query.PageSize
	end := min(start+query.PageSize, len(filtered))
	result.Items = slices.Clone(filtered[start:end])

	return result, nil
}

// withDefaults fills zero-valued fields with the data-model.md defaults.
func withDefaults(query ListQuery) ListQuery {
	if query.Page < 1 {
		query.Page = defaultPage
	}

	if query.PageSize < 1 {
		query.PageSize = defaultPageSize
	}

	if query.SortBy == "" {
		query.SortBy = SortByName
	}

	if query.SortDir == "" {
		query.SortDir = SortAsc
	}

	return query
}

func validSortBy(sortBy SortBy) bool {
	switch sortBy {
	case SortByVMID, SortByName, SortByNode, SortByStatus, SortByCPU, SortByMemory:
		return true
	}

	return false
}

// scopedVMs is the ONLY branch point for scope in the whole request path
// (FR-003, SC-005): scope=all is honoured for an admin and silently treated
// as mine for anyone else.
func scopedVMs(index *inventory.Index, query ListQuery, identity auth.Identity) []cluster.VM {
	if query.Scope == ScopeAll && identity.IsAdmin {
		all := make([]cluster.VM, 0, len(index.ByVMID))
		for _, machine := range index.ByVMID {
			all = append(all, machine)
		}

		return all
	}

	return index.ByPool[identity.Pool]
}

// searchVMs classifies the raw text server-side (research.md): numeric-only
// also tries an exact VMID match; name substring and tag match always run.
// The union is deduplicated because the source set already is.
func searchVMs(vms []cluster.VM, search string) []cluster.VM {
	search = strings.TrimSpace(search)
	if search == "" {
		return vms
	}

	lowered := strings.ToLower(search)
	id, numeric := parseNumericID(search)

	matched := make([]cluster.VM, 0, len(vms))
	for _, machine := range vms {
		if strings.Contains(strings.ToLower(machine.Name), lowered) || hasMatchingTag(machine.Tags, lowered) || numeric && machine.VMID == id {
			matched = append(matched, machine)
		}
	}

	return matched
}

func parseNumericID(search string) (int, bool) {
	id, err := strconv.Atoi(search)
	return id, err == nil
}

func hasMatchingTag(tags []string, loweredSearch string) bool {
	return slices.ContainsFunc(tags, func(tag string) bool {
		return strings.Contains(strings.ToLower(tag), loweredSearch)
	})
}

// nodeFacet lists the nodes present before the node filter is applied, so
// the filter's own dropdown never shrinks to hide its selection
// (data-model.md step 4).
func nodeFacet(vms []cluster.VM) []string {
	seen := make(map[string]struct{}, len(vms))

	nodes := make([]string, 0, len(vms))
	for _, machine := range vms {
		if _, ok := seen[machine.Node]; !ok {
			seen[machine.Node] = struct{}{}
			nodes = append(nodes, machine.Node)
		}
	}

	slices.Sort(nodes)

	return nodes
}

func filterVMs(vms []cluster.VM, query ListQuery) []cluster.VM {
	if query.Status == "" && query.Node == "" {
		return vms
	}

	filtered := make([]cluster.VM, 0, len(vms))
	for _, machine := range vms {
		if query.Status != "" && machine.Status != query.Status {
			continue
		}

		if query.Node != "" && machine.Node != query.Node {
			continue
		}

		filtered = append(filtered, machine)
	}

	return filtered
}

// sortVMs orders in place by the requested column, with VMID as the
// tiebreaker so pagination is stable.
func sortVMs(vms []cluster.VM, sortBy SortBy, sortDir SortDir) {
	slices.SortStableFunc(vms, func(a, b cluster.VM) int {
		order := compareVMs(a, b, sortBy)
		if order == 0 {
			order = a.VMID - b.VMID
		}

		if sortDir == SortDesc {
			return -order
		}

		return order
	})
}

func compareVMs(a, b cluster.VM, sortBy SortBy) int {
	switch sortBy {
	case SortByVMID:
		return a.VMID - b.VMID
	case SortByName:
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	case SortByNode:
		return strings.Compare(a.Node, b.Node)
	case SortByStatus:
		return strings.Compare(string(a.Status), string(b.Status))
	case SortByCPU:
		return a.CPUCores - b.CPUCores
	case SortByMemory:
		return compareInt64(a.MemoryTotal, b.MemoryTotal)
	}

	return 0
}

func compareInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}

	return 0
}
