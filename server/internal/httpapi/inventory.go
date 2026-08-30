package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/inventory"
	"strconv"
	"time"
)

// ClusterRefresh serves POST /api/v1/cluster/refresh — a manual refresh
// action guarded by a minimum interval (FR-005, FR-006). The guard is
// enforced server-side (constitution VI), not only by disabling a button.
type ClusterRefresh struct {
	refresher *inventory.Refresher
	log       *slog.Logger
}

// NewClusterRefresh creates the handler for the given refresher.
func NewClusterRefresh(refresher *inventory.Refresher, log *slog.Logger) *ClusterRefresh {
	return &ClusterRefresh{refresher: refresher, log: log}
}

type clusterRefreshResponse struct {
	RefreshedAt string `json:"refreshedAt"`
}

type clusterRefreshTooSoon struct {
	Code              string `json:"code"`
	Message           string `json:"message"`
	RetryAfterSeconds int    `json:"retryAfterSeconds"`
}

func (h *ClusterRefresh) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")

		if err := writeClusterError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed); err != nil {
			h.log.Error("failed to write method not allowed", "component", "httpapi", "error", err)
		}

		return
	}

	// Async refresh: the guard check runs synchronously (returns 429 if too
	// soon), but the actual cluster call runs in a background goroutine so
	// the server's WriteTimeout cannot cancel a slow/dead cluster's refresh
	// before it completes. The client gets 202 immediately and learns the
	// outcome by re-reading the projection (re-loading the VM list or
	// polling /health) — which the frontend already does.
	if err := h.refresher.RefreshAsync(r.Context()); err != nil {
		h.writeRefreshError(w, err)
		return
	}

	resp := clusterRefreshResponse{RefreshedAt: time.Now().UTC().Format(time.RFC3339Nano)}

	body, err := json.Marshal(resp)
	if err != nil {
		h.log.Error("failed to marshal refresh response", "component", "httpapi", "error", err)

		if writeErr := writeClusterError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError); writeErr != nil {
			h.log.Error("failed to write internal_error", "component", "httpapi", "error", writeErr)
		}

		return
	}

	if err := writeJSON(w, http.StatusAccepted, body); err != nil {
		h.log.Error("failed to write refresh response", "component", "httpapi", "error", err)
	}
}

// writeRefreshError maps a refresher error to the correct HTTP response, including
// the Retry-After header for ErrRefreshTooSoon. Extracted from ServeHTTP to keep
// its Cognitive Complexity under the go:S3776 threshold.
func (h *ClusterRefresh) writeRefreshError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, inventory.ErrRefreshTooSoon):
		retryAfter := h.computeRetryAfterSeconds(err)

		body, marshalErr := json.Marshal(clusterRefreshTooSoon{
			Code:              "refresh_too_soon",
			Message:           "please wait before refreshing again",
			RetryAfterSeconds: retryAfter,
		})
		if marshalErr != nil {
			h.log.Error("failed to marshal refresh_too_soon", "component", "httpapi", "error", marshalErr)
			return
		}

		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))

		if writeErr := writeJSON(w, http.StatusTooManyRequests, body); writeErr != nil {
			h.log.Error("failed to write refresh_too_soon", "component", "httpapi", "error", writeErr)
		}
	case errors.Is(err, inventory.ErrClusterUnreachable):
		h.log.Error("manual refresh failed: cluster unreachable", "component", "httpapi", "error", err)

		if writeErr := writeClusterError(w, http.StatusBadGateway, "cluster_unreachable", "refresh failed: cluster is not reachable"); writeErr != nil {
			h.log.Error("failed to write cluster_unreachable", "component", "httpapi", "error", writeErr)
		}
	default:
		h.log.Error("manual refresh failed", "component", "httpapi", "error", err)

		if writeErr := writeClusterError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError); writeErr != nil {
			h.log.Error("failed to write internal_error", "component", "httpapi", "error", writeErr)
		}
	}
}

// computeRetryAfterSeconds returns how many seconds the client should wait
// before retrying. When err carries the precise remaining guard time
// (*inventory.TooSoonError), that value is used — not the full configured
// interval, so a refusal near the end of the guard window reports a short
// wait, not the whole interval again. Falls back to the full interval only
// if the error doesn't carry a remaining time.
func (h *ClusterRefresh) computeRetryAfterSeconds(err error) int {
	if tooSoon, ok := errors.AsType[*inventory.TooSoonError](err); ok {
		seconds := int(tooSoon.RetryAfter / time.Second)
		if tooSoon.RetryAfter%time.Second != 0 {
			seconds++
		}

		if seconds < 1 {
			seconds = 1
		}

		return seconds
	}

	seconds := max(int(h.refresher.MinInterval().Seconds()), 1)

	return seconds
}
