package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"pvmss/server/internal/logctx"
	"time"
)

const (
	healthStatusHealthy   = "healthy"
	healthStatusUnhealthy = "unhealthy"
)

// Health checks the runtime dependencies and writes the health contract.
type Health struct {
	store          Pinger
	freshness      ClusterFreshnessChecker
	staleThreshold time.Duration
	auth           *Auth
	log            *slog.Logger
}

// SetAuthHandler wires session resolution so a non-admin scoped session also
// gets a myCluster check for its own cluster. The aggregate clusters check
// keeps answering counts for everyone - the public login page needs them.
func (h *Health) SetAuthHandler(auth *Auth) {
	h.auth = auth
}

// ServeHTTP implements http.Handler.
func (h *Health) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")

		if err := writeError(w, http.StatusMethodNotAllowed, "method not allowed"); err != nil {
			logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "failed to write method not allowed", "component", "httpapi", "error", err)
		}

		return
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)
	checks := make(map[string]CheckResult, 2)
	checks["database"] = CheckResult{Status: healthStatusHealthy}
	checks["clusters"] = h.clustersCheck()

	if mine := h.ownClusterCheck(r); mine != nil {
		checks["myCluster"] = *mine
	}

	resp := HealthResponse{
		Status:    healthStatusHealthy,
		Checks:    checks,
		DemoMode:  h.demoMode(),
		Timestamp: timestamp,
	}
	status := http.StatusOK

	if err := h.store.Ping(r.Context()); err != nil {
		// Not SetError: /health is capped at Debug in the access log, so a failing
		// dependency must be logged here to stay visible.
		logctx.FromOr(r.Context(), h.log).ErrorContext(r.Context(), "database health check failed", "component", "httpapi", "error", err)

		resp.Status = healthStatusUnhealthy
		resp.Checks["database"] = CheckResult{Status: healthStatusUnhealthy, Detail: "database unreachable"}
		status = http.StatusServiceUnavailable
	}

	body, err := json.Marshal(resp)
	if err != nil {
		logctx.FromOr(r.Context(), h.log).ErrorContext(r.Context(), "failed to marshal health response", "component", "httpapi", "error", err)

		if writeErr := writeError(w, http.StatusInternalServerError, "internal server error"); writeErr != nil {
			logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "failed to write health error response", "component", "httpapi", "error", writeErr)
		}

		return
	}

	if err := writeJSON(w, status, body); err != nil {
		logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "failed to write health response", "component", "httpapi", "error", err)
	}
}

// clustersCheck derives the aggregate clusters check from each configured
// cluster's RefreshedAt, without calling cluster.Client. A cluster is
// stale when time.Since(RefreshedAt) exceeds the stale threshold. The detail
// is a count, never a cluster name.
func (h *Health) clustersCheck() CheckResult {
	if h.freshness == nil {
		return CheckResult{Status: healthStatusHealthy}
	}

	clusters := h.freshness.Clusters()
	if len(clusters) == 0 {
		return CheckResult{Status: healthStatusHealthy}
	}

	stale := 0

	now := time.Now()
	for _, c := range clusters {
		if now.Sub(c.RefreshedAt) > h.staleThreshold {
			stale++
		}
	}

	if stale == 0 {
		return CheckResult{Status: healthStatusHealthy}
	}

	return CheckResult{
		Status:      healthStatusUnhealthy,
		Detail:      fmt.Sprintf("%d of %d clusters unreachable", stale, len(clusters)),
		Unreachable: stale,
		Total:       len(clusters),
	}
}

// ownClusterCheck reports the freshness of the caller's own cluster for a
// non-admin scoped session, so the banner only fires when THEIR cluster is
// down. Anonymous callers, admins (who watch the aggregate), and sessions
// without a cluster get nil - no myCluster check at all.
func (h *Health) ownClusterCheck(r *http.Request) *CheckResult {
	if h.auth == nil || h.freshness == nil {
		return nil
	}

	identity, err := h.auth.Principal(r)
	if err != nil || identity.IsAdmin || identity.Cluster == "" {
		return nil
	}

	for _, c := range h.freshness.Clusters() {
		if c.Name != identity.Cluster {
			continue
		}

		if time.Since(c.RefreshedAt) > h.staleThreshold {
			return &CheckResult{Status: healthStatusUnhealthy, Detail: "cluster unreachable"}
		}

		return &CheckResult{Status: healthStatusHealthy}
	}

	return nil
}

// demoMode reports whether the instance is wired to the fake cluster client.
func (h *Health) demoMode() bool {
	if h.freshness == nil {
		return false
	}

	return h.freshness.DemoMode()
}
