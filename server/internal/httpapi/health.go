package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Health checks the runtime dependencies and writes the health contract.
type Health struct {
	store           Pinger
	freshness       ClusterFreshnessChecker
	staleThreshold  time.Duration
	log             *slog.Logger
}

// ServeHTTP implements http.Handler.
func (h *Health) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")

		if err := writeError(w, http.StatusMethodNotAllowed, "method not allowed"); err != nil {
			h.log.Error("failed to write method not allowed", "component", "httpapi", "error", err)
		}

		return
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)
	checks := make(map[string]CheckResult, 2)
	checks["database"] = CheckResult{Status: "healthy"}
	checks["clusters"] = h.clustersCheck()
	resp := HealthResponse{
		Status:    "healthy",
		Checks:    checks,
		DemoMode:  h.demoMode(),
		Timestamp: timestamp,
	}
	status := http.StatusOK

	if err := h.store.Ping(r.Context()); err != nil {
		h.log.Error("database health check failed", "component", "httpapi", "error", err)

		resp.Status = "unhealthy"
		resp.Checks["database"] = CheckResult{Status: "unhealthy", Detail: "database unreachable"}
		status = http.StatusServiceUnavailable
	}

	body, err := json.Marshal(resp)
	if err != nil {
		h.log.Error("failed to marshal health response", "component", "httpapi", "error", err)

		if writeErr := writeError(w, http.StatusInternalServerError, "internal server error"); writeErr != nil {
			h.log.Error("failed to write health error response", "component", "httpapi", "error", writeErr)
		}

		return
	}

	if err := writeJSON(w, status, body); err != nil {
		h.log.Error("failed to write health response", "component", "httpapi", "error", err)
	}
}

// clustersCheck derives the aggregate clusters check from each configured
// cluster's RefreshedAt, without calling cluster.Client (FR-010). A cluster is
// stale when time.Since(RefreshedAt) exceeds the stale threshold. The detail
// is a count, never a cluster name (FR-012).
func (h *Health) clustersCheck() CheckResult {
	if h.freshness == nil {
		return CheckResult{Status: "healthy"}
	}
	clusters := h.freshness.Clusters()
	if len(clusters) == 0 {
		return CheckResult{Status: "healthy"}
	}
	stale := 0
	now := time.Now()
	for _, c := range clusters {
		if now.Sub(c.RefreshedAt) > h.staleThreshold {
			stale++
		}
	}
	if stale == 0 {
		return CheckResult{Status: "healthy"}
	}
	return CheckResult{
		Status: "unhealthy",
		Detail: fmt.Sprintf("%d of %d clusters unreachable", stale, len(clusters)),
	}
}

// demoMode reports whether the instance is wired to the fake cluster client.
func (h *Health) demoMode() bool {
	if h.freshness == nil {
		return false
	}
	return h.freshness.DemoMode()
}
