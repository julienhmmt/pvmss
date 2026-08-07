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

		if err := writeError(w, http.StatusMethodNotAllowed, "method not allowed"); err != nil {
			h.log.Error("failed to write method not allowed", "component", "httpapi", "error", err)
		}

		return
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)
	checks := make(map[string]CheckResult, 1)
	checks["database"] = CheckResult{Status: "healthy"}
	resp := HealthResponse{
		Status:    "healthy",
		Checks:    checks,
		Timestamp: timestamp,
	}
	status := http.StatusOK

	if err := h.store.Ping(); err != nil {
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
