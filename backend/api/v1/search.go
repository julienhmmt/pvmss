package apiv1

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"pvmss/proxmox"
	"pvmss/state"
)

// SearchResult is a VMSummary returned in search results.
type SearchResult struct {
	VMSummary
}

// SearchResponse wraps search results.
type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Total   int            `json:"total"`
}

// SearchHandler handles VM search requests.
type SearchHandler struct {
	state state.StateManager
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(s state.StateManager) *SearchHandler {
	return &SearchHandler{state: s}
}

// SearchVMs handles GET /api/v1/search/vms?vmid=&name=&tags=&limit=
func (h *SearchHandler) SearchVMs(w http.ResponseWriter, r *http.Request) {
	if h.state != nil && h.state.IsOfflineMode() {
		writeJSON(w, SearchResponse{Results: []SearchResult{}, Total: 0})
		return
	}

	client, err := restyClient()
	if err != nil {
		errInternal(w)
		return
	}

	q := r.URL.Query()
	filterVMID := strings.TrimSpace(q.Get("vmid"))
	filterName := strings.TrimSpace(strings.ToLower(q.Get("name")))
	filterTags := strings.TrimSpace(strings.ToLower(q.Get("tags")))
	limit := 25
	if l, err := strconv.Atoi(q.Get("limit")); err == nil && l > 0 && l <= 500 {
		limit = l
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	vms, err := proxmox.GetVMsResty(ctx, client)
	if err != nil {
		errInternal(w)
		return
	}

	results := make([]SearchResult, 0, len(vms))
	for _, vm := range vms {
		if filterVMID != "" && !strings.Contains(strconv.Itoa(vm.VMID), filterVMID) {
			continue
		}
		if filterName != "" && !strings.Contains(strings.ToLower(vm.Name), filterName) {
			continue
		}
		if filterTags != "" && !strings.Contains(strings.ToLower(vm.Tags), filterTags) {
			continue
		}
		results = append(results, SearchResult{VMSummary: vmToSummary(vm)})
		if len(results) >= limit {
			break
		}
	}

	writeJSON(w, SearchResponse{Results: results, Total: len(results)})
}
