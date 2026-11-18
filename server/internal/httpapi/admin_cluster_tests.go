package httpapi

import (
	"net/http"
	"pvmss/server/internal/store"
	"strconv"
	"time"
)

// clusterTestLimits are the page sizes the history filter offers.
var clusterTestLimits = map[int]bool{5: true, 10: true, 25: true, 50: true, 100: true}

const defaultClusterTestLimit = 5

type clusterTestEntry struct {
	TestedAt   string `json:"testedAt"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
	Version    string `json:"proxmoxVersion,omitempty"`
	Actor      string `json:"actor,omitempty"`
	DurationMs *int64 `json:"durationMs"`
}

func toClusterTestEntry(test store.ClusterTest) clusterTestEntry {
	return clusterTestEntry{TestedAt: test.TestedAt.UTC().Format(time.RFC3339Nano), Status: test.Status, Message: test.Message, Version: test.Version, Actor: test.Actor, DurationMs: test.DurationMs}
}

// recordTest stamps the actor and the elapsed time since test.TestedAt, then
// persists the test and its history row.
func (handler *AdminClusters) recordTest(r *http.Request, test store.ClusterTest) (clusterTestEntry, error) {
	principal, _ := handler.auth.Principal(r)
	duration := time.Since(test.TestedAt).Milliseconds()

	test.Actor, test.DurationMs = principal.Username, &duration
	if err := handler.store.RecordClusterTest(r.Context(), test); err != nil {
		return clusterTestEntry{}, err
	}

	return toClusterTestEntry(test), nil
}

// ServeTestHistory handles GET /api/v1/admin/clusters/:name/tests?limit=N.
func (handler *AdminClusters) ServeTestHistory(w http.ResponseWriter, r *http.Request) {
	limit := defaultClusterTestLimit

	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || !clusterTestLimits[parsed] {
			writeAdminError(w, http.StatusBadRequest, "invalid_limit", "limit must be 5, 10, 25, 50 or 100")
			return
		}

		limit = parsed
	}

	name := r.PathValue("name")
	if _, err := handler.store.GetCluster(r.Context(), name); err != nil {
		handler.writeStoreFailure(w, err)
		return
	}

	tests, err := handler.store.ListClusterTests(r.Context(), name, limit)
	if err != nil {
		handler.writeFailure(w, err)
		return
	}

	entries := make([]clusterTestEntry, 0, len(tests))
	for _, test := range tests {
		entries = append(entries, toClusterTestEntry(test))
	}

	writeAdminJSON(w, http.StatusOK, map[string]any{"tests": entries})
}
