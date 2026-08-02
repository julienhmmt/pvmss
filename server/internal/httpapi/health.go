package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// Health checks the runtime dependencies and writes the health contract.
type Health struct {
	store Pinger
	log   *slog.Logger
}

// ServeHTTP implements http.Handler.
func (h *Health) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)
	resp := HealthResponse{
		Status:    "healthy",
		Checks:    map[string]CheckResult{"database": {Status: "healthy"}},
		Timestamp: timestamp,
	}
	status := http.StatusOK

	if err := h.store.Ping(); err != nil {
		h.log.Error("database health check failed", "component", "httpapi", "error", err)
		resp.Status = "unhealthy"
		resp.Checks["database"] = CheckResult{Status: "unhealthy", Detail: "database unreachable"}
		status = http.StatusServiceUnavailable
	}

	body, _ := json.Marshal(resp)
	writeJSON(w, status, body)
}
