//nolint:goconst // test fixture strings
package vm_test

import (
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/vm"
	"slices"
	"testing"
)

// testIndex builds a small projection across two pools and two nodes —
// large enough to exercise scope, search, filter, sort, and pagination
// without depending on the T01 fake dataset's size.
func testIndex() *inventory.Index {
	index := inventory.BuildIndex(cluster.Snapshot{
		VMs: []cluster.VM{
			{VMID: 100, Name: "web-01", Node: "pve-node-01", Status: cluster.VMRunning, Pool: "pool-alice", Tags: []string{"pvmss", "web"}, CPUCores: 2, MemoryTotal: 4294967296},
			{VMID: 101, Name: "web-02", Node: "pve-node-01", Status: cluster.VMStopped, Pool: "pool-alice", Tags: []string{"pvmss", "web"}, CPUCores: 4, MemoryTotal: 2147483648},
			{VMID: 102, Name: "db-01", Node: "pve-node-02", Status: cluster.VMRunning, Pool: "pool-alice", Tags: []string{"pvmss", "db"}, CPUCores: 8, MemoryTotal: 8589934592},
			{VMID: 103, Name: "cache-01", Node: "pve-node-01", Status: cluster.VMPaused, Pool: "pool-bob", Tags: []string{"pvmss", "cache"}, CPUCores: 1, MemoryTotal: 1073741824},
			{VMID: 104, Name: "build-01", Node: "pve-node-02", Status: cluster.VMStopped, Pool: "pool-bob", Tags: nil, CPUCores: 4, MemoryTotal: 8589934592},
		},
	})

	return &index
}

var (
	alice = auth.Identity{Username: "alice@pve", Pool: "pool-alice"}
	admin = auth.Identity{Username: "admin", IsAdmin: true}
)

func vmids(result vm.ListResult) []int {
	ids := make([]int, len(result.Items))
	for i, item := range result.Items {
		ids[i] = item.VMID
	}

	return ids
}

func TestList_ScopeEnforcement(t *testing.T) {
	tests := []struct {
		name     string
		identity auth.Identity
		scope    vm.Scope
		wantIDs  []int
	}{
		{name: "non-admin default scope sees own pool", identity: alice, scope: "", wantIDs: []int{100, 101, 102}},
		{name: "non-admin explicit mine sees own pool", identity: alice, scope: vm.ScopeMine, wantIDs: []int{100, 101, 102}},
		{name: "non-admin scope=all silently overridden to mine", identity: alice, scope: vm.ScopeAll, wantIDs: []int{100, 101, 102}},
		{name: "admin scope=all sees every pool", identity: admin, scope: vm.ScopeAll, wantIDs: []int{100, 101, 102, 103, 104}},
		{name: "admin without scope sees only own pool", identity: admin, scope: "", wantIDs: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := vm.List(testIndex(), vm.ListQuery{Scope: tt.scope}, tt.identity, -1)
			if err != nil {
				t.Fatalf("List: %v", err)
			}

			got := vmids(result)
			slices.Sort(got)

			if !slices.Equal(got, tt.wantIDs) {
				t.Errorf("vmids = %v, want %v", got, tt.wantIDs)
			}
		})
	}
}

func TestList_SearchClassification(t *testing.T) {
	tests := []struct {
		name    string
		search  string
		wantIDs []int
	}{
		{name: "name substring", search: "web", wantIDs: []int{100, 101}},
		{name: "name substring case-insensitive", search: "WEB", wantIDs: []int{100, 101}},
		{name: "tag match", search: "db", wantIDs: []int{102}},
		{name: "numeric id", search: "101", wantIDs: []int{101}},
		{name: "union of name and tag matches, deduplicated", search: "pvmss", wantIDs: []int{100, 101, 102}},
		{name: "no match", search: "does-not-exist", wantIDs: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := vm.List(testIndex(), vm.ListQuery{Search: tt.search}, alice, -1)
			if err != nil {
				t.Fatalf("List: %v", err)
			}

			got := vmids(result)
			slices.Sort(got)

			if !slices.Equal(got, tt.wantIDs) {
				t.Errorf("vmids = %v, want %v", got, tt.wantIDs)
			}

			if len(tt.wantIDs) > 1 && len(got) != len(tt.wantIDs) {
				t.Errorf("duplicate rows: %v", got)
			}
		})
	}
}

func TestList_SearchNeverCrossesScope(t *testing.T) {
	// "cache-01" is bob's VM; alice's search must not surface it (SC-005).
	result, err := vm.List(testIndex(), vm.ListQuery{Search: "cache"}, alice, -1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(result.Items) != 0 {
		t.Errorf("vmids = %v, want none", vmids(result))
	}

	if result.EmptyReason != vm.EmptyNoMatch {
		t.Errorf("EmptyReason = %q, want %q", result.EmptyReason, vm.EmptyNoMatch)
	}
}

func TestList_Filters(t *testing.T) {
	tests := []struct {
		name    string
		query   vm.ListQuery
		wantIDs []int
	}{
		{name: "status filter", query: vm.ListQuery{Status: cluster.VMRunning}, wantIDs: []int{100, 102}},
		{name: "node filter", query: vm.ListQuery{Node: "pve-node-02"}, wantIDs: []int{102}},
		{name: "status and node combined", query: vm.ListQuery{Status: cluster.VMStopped, Node: "pve-node-01"}, wantIDs: []int{101}},
		{name: "search combined with status", query: vm.ListQuery{Search: "web", Status: cluster.VMStopped}, wantIDs: []int{101}},
		{name: "unknown node yields zero results, not an error", query: vm.ListQuery{Node: "no-such-node"}, wantIDs: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := vm.List(testIndex(), tt.query, alice, -1)
			if err != nil {
				t.Fatalf("List: %v", err)
			}

			got := vmids(result)
			slices.Sort(got)

			if !slices.Equal(got, tt.wantIDs) {
				t.Errorf("vmids = %v, want %v", got, tt.wantIDs)
			}
		})
	}
}

func TestList_NodeFacetIgnoresNodeFilter(t *testing.T) {
	// The facet is computed before the node filter so the dropdown does not
	// shrink to hide its own selection (data-model.md step 4).
	result, err := vm.List(testIndex(), vm.ListQuery{Node: "pve-node-02"}, alice, -1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	want := []string{"pve-node-01", "pve-node-02"}
	if !slices.Equal(result.AvailableNodes, want) {
		t.Errorf("AvailableNodes = %v, want %v", result.AvailableNodes, want)
	}
}

func TestList_Sort(t *testing.T) {
	tests := []struct {
		name    string
		sortBy  vm.SortBy
		sortDir vm.SortDir
		wantIDs []int
	}{
		{name: "default sort is name ascending", sortBy: "", sortDir: "", wantIDs: []int{102, 100, 101}},
		{name: "vmid ascending", sortBy: vm.SortByVMID, sortDir: vm.SortAsc, wantIDs: []int{100, 101, 102}},
		{name: "vmid descending", sortBy: vm.SortByVMID, sortDir: vm.SortDesc, wantIDs: []int{102, 101, 100}},
		{name: "name descending", sortBy: vm.SortByName, sortDir: vm.SortDesc, wantIDs: []int{101, 100, 102}},
		{name: "node then vmid tiebreak", sortBy: vm.SortByNode, sortDir: vm.SortAsc, wantIDs: []int{100, 101, 102}},
		{name: "status ascending", sortBy: vm.SortByStatus, sortDir: vm.SortAsc, wantIDs: []int{100, 102, 101}},
		{name: "cpu descending", sortBy: vm.SortByCPU, sortDir: vm.SortDesc, wantIDs: []int{102, 101, 100}},
		{name: "memory ascending", sortBy: vm.SortByMemory, sortDir: vm.SortAsc, wantIDs: []int{101, 100, 102}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := vm.List(testIndex(), vm.ListQuery{SortBy: tt.sortBy, SortDir: tt.sortDir}, alice, -1)
			if err != nil {
				t.Fatalf("List: %v", err)
			}

			if got := vmids(result); !slices.Equal(got, tt.wantIDs) {
				t.Errorf("vmids = %v, want %v", got, tt.wantIDs)
			}
		})
	}
}

func TestList_InvalidSortByRejected(t *testing.T) {
	_, err := vm.List(testIndex(), vm.ListQuery{SortBy: "unknownColumn"}, alice, -1)
	if !errors.Is(err, vm.ErrInvalidSortBy) {
		t.Fatalf("err = %v, want ErrInvalidSortBy", err)
	}
}

func TestList_Pagination(t *testing.T) {
	tests := []struct {
		name      string
		page      int
		pageSize  int
		wantIDs   []int
		wantPage  int
		wantTotal int
	}{
		{name: "first page", page: 1, pageSize: 2, wantIDs: []int{102, 100}, wantPage: 1, wantTotal: 3},
		{name: "second page", page: 2, pageSize: 2, wantIDs: []int{101}, wantPage: 2, wantTotal: 3},
		{name: "page beyond range clamps to last", page: 9, pageSize: 2, wantIDs: []int{101}, wantPage: 2, wantTotal: 3},
		{name: "page below one clamps to first", page: 0, pageSize: 2, wantIDs: []int{102, 100}, wantPage: 1, wantTotal: 3},
		{name: "defaults apply", page: 0, pageSize: 0, wantIDs: []int{102, 100, 101}, wantPage: 1, wantTotal: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := vm.List(testIndex(), vm.ListQuery{Page: tt.page, PageSize: tt.pageSize}, alice, -1)
			if err != nil {
				t.Fatalf("List: %v", err)
			}

			if got := vmids(result); !slices.Equal(got, tt.wantIDs) {
				t.Errorf("vmids = %v, want %v", got, tt.wantIDs)
			}

			if result.Page != tt.wantPage {
				t.Errorf("Page = %d, want %d", result.Page, tt.wantPage)
			}

			if result.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", result.Total, tt.wantTotal)
			}
		})
	}
}

func TestList_EmptyReasons(t *testing.T) {
	t.Run("no VMs owned", func(t *testing.T) {
		carol := auth.Identity{Username: "carol@pve", Pool: "pool-carol"}

		result, err := vm.List(testIndex(), vm.ListQuery{}, carol, -1)
		if err != nil {
			t.Fatalf("List: %v", err)
		}

		if len(result.Items) != 0 {
			t.Fatalf("Items = %v, want none", vmids(result))
		}

		if result.EmptyReason != vm.EmptyNoVMsOwned {
			t.Errorf("EmptyReason = %q, want %q", result.EmptyReason, vm.EmptyNoVMsOwned)
		}
	})
	t.Run("search matches nothing", func(t *testing.T) {
		result, err := vm.List(testIndex(), vm.ListQuery{Search: "does-not-exist"}, alice, -1)
		if err != nil {
			t.Fatalf("List: %v", err)
		}

		if result.EmptyReason != vm.EmptyNoMatch {
			t.Errorf("EmptyReason = %q, want %q", result.EmptyReason, vm.EmptyNoMatch)
		}
	})
	t.Run("non-empty result has no empty reason", func(t *testing.T) {
		result, err := vm.List(testIndex(), vm.ListQuery{}, alice, -1)
		if err != nil {
			t.Fatalf("List: %v", err)
		}

		if result.EmptyReason != "" {
			t.Errorf("EmptyReason = %q, want empty", result.EmptyReason)
		}
	})
}

func TestList_Quota(t *testing.T) {
	t.Run("non-admin default scope reports used against allowed", func(t *testing.T) {
		result, err := vm.List(testIndex(), vm.ListQuery{}, alice, 10)
		if err != nil {
			t.Fatalf("List: %v", err)
		}

		if result.Quota == nil || *result.Quota != (vm.Quota{Used: 3, Allowed: 10}) {
			t.Errorf("Quota = %+v, want {Used:3 Allowed:10}", result.Quota)
		}
	})
	t.Run("unlimited quota represented as -1", func(t *testing.T) {
		result, err := vm.List(testIndex(), vm.ListQuery{}, alice, -1)
		if err != nil {
			t.Fatalf("List: %v", err)
		}

		if result.Quota == nil || result.Quota.Allowed != -1 {
			t.Errorf("Quota = %+v, want Allowed -1", result.Quota)
		}
	})
	t.Run("admin scope=all carries no quota", func(t *testing.T) {
		result, err := vm.List(testIndex(), vm.ListQuery{Scope: vm.ScopeAll}, admin, 10)
		if err != nil {
			t.Fatalf("List: %v", err)
		}

		if result.Quota != nil {
			t.Errorf("Quota = %+v, want nil", result.Quota)
		}
	})
	t.Run("admin default scope still sees own quota", func(t *testing.T) {
		result, err := vm.List(testIndex(), vm.ListQuery{}, admin, 10)
		if err != nil {
			t.Fatalf("List: %v", err)
		}

		if result.Quota == nil || result.Quota.Allowed != 10 {
			t.Errorf("Quota = %+v, want {Used:0 Allowed:10}", result.Quota)
		}
	})
}
