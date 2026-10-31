package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/logctx"
	"pvmss/server/internal/vm"
	"strconv"
	"time"
)

// ClusterRefresh serves POST /api/v1/cluster/refresh - a manual refresh
// action guarded by a minimum interval. The guard is
// enforced server-side, not only by disabling a button.
type ClusterRefresh struct {
	refresher *inventory.Refresher
	registry  *inventory.Registry
	clusters  cluster.ClientProvider
	auth      *Auth
	log       *slog.Logger
}

// NewClusterRefresh creates the handler for the given refresher.
func NewClusterRefresh(refresher *inventory.Refresher, log *slog.Logger) *ClusterRefresh {
	return &ClusterRefresh{refresher: refresher, log: log}
}

// NewClusterRefreshWithRegistry creates the handler that scopes refreshes per
// request cluster: a non-admin refreshes only the cluster it signed in to
// (?cluster= is ignored); an admin may name one via ?cluster= and without it
// refreshes every cluster, reporting per-cluster outcomes.
func NewClusterRefreshWithRegistry(registry *inventory.Registry, clusters cluster.ClientProvider, authHandler *Auth, log *slog.Logger) *ClusterRefresh {
	return &ClusterRefresh{registry: registry, clusters: clusters, auth: authHandler, log: log}
}

// Outcome labels reported per cluster in an admin refresh-all response.
const (
	outcomeAccepted    = "accepted"
	outcomeTooSoon     = "too_soon"
	outcomeUnreachable = "unreachable"
	outcomeNotFound    = "not_found"
	outcomeError       = "error"
)

type clusterRefreshResponse struct {
	RefreshedAt string `json:"refreshedAt"`
	// Clusters reports each cluster's outcome on an admin refresh-all.
	Clusters map[string]string `json:"clusters,omitempty"`
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
			logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "failed to write method not allowed", "component", "httpapi", "error", err)
		}

		return
	}

	if h.registry != nil && h.auth != nil {
		h.serveScoped(w, r)
		return
	}

	h.serveLegacy(w, r)
}

// serveScoped resolves the caller's target clusters and refreshes only those:
// one for a non-admin or an admin naming ?cluster=, all of them for an admin
// without a parameter.
func (h *ClusterRefresh) serveScoped(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		writeClusterErrorLogged(w, r, h.log, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}

	requested := ""
	if identity.IsAdmin {
		requested = r.URL.Query().Get("cluster")
	}

	clusterName, _ := vm.TargetCluster(identity, requested)

	if clusterName != "" {
		refresher, err := h.registry.Refresher(clusterName)
		if err != nil {
			writeClusterErrorLogged(w, r, h.log, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
			return
		}

		h.refreshOne(w, r, refresher, nil)

		return
	}

	if !identity.IsAdmin {
		writeClusterErrorLogged(w, r, h.log, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}

	h.refreshAll(w, r)
}

// refreshAll runs RefreshAsync on every cluster for an admin who named none.
// One accepted refresh is enough for 202; when all fail the first failure
// decides the error response so a global "too soon" still answers 429.
func (h *ClusterRefresh) refreshAll(w http.ResponseWriter, r *http.Request) {
	names := h.clusters.List()
	outcomes := make(map[string]string, len(names))

	var firstRefresher *inventory.Refresher

	var firstErr error

	accepted := false

	for _, name := range names {
		refresher, err := h.registry.Refresher(name)
		if err != nil {
			outcomes[name] = outcomeNotFound

			if firstErr == nil {
				firstErr = err
			}

			continue
		}

		if err := refresher.RefreshAsync(r.Context()); err != nil {
			outcomes[name] = refreshOutcome(err)

			if firstErr == nil {
				firstErr = err
				firstRefresher = refresher
			}

			continue
		}

		outcomes[name] = outcomeAccepted
		accepted = true
	}

	if !accepted {
		if firstRefresher != nil {
			h.writeRefreshError(w, firstRefresher, firstErr)
			return
		}

		writeClusterErrorLogged(w, r, h.log, http.StatusServiceUnavailable, "inventory_not_ready", msgInventoryNotReady)

		return
	}

	h.writeAccepted(w, r, outcomes)
}

// refreshOutcome maps a RefreshAsync failure to the label reported in the
// admin per-cluster outcomes map.
func refreshOutcome(err error) string {
	switch {
	case errors.Is(err, inventory.ErrRefreshTooSoon):
		return outcomeTooSoon
	case errors.Is(err, inventory.ErrClusterUnreachable):
		return outcomeUnreachable
	default:
		return outcomeError
	}
}

// serveLegacy is the single-refresher path kept for tests that construct the
// handler directly without a registry.
func (h *ClusterRefresh) serveLegacy(w http.ResponseWriter, r *http.Request) {
	h.refreshOne(w, r, h.refresher, nil)
}

// refreshOne runs RefreshAsync on refresher and writes the 202 response (or
// the mapped error). outcomes is nil for a single-cluster refresh - the
// per-cluster map only ships on an admin refresh-all.
func (h *ClusterRefresh) refreshOne(w http.ResponseWriter, r *http.Request, refresher *inventory.Refresher, outcomes map[string]string) {
	// Async refresh: the guard check runs synchronously (returns 429 if too
	// soon), but the actual cluster call runs in a background goroutine so
	// the server's WriteTimeout cannot cancel a slow/dead cluster's refresh
	// before it completes. The client gets 202 immediately and learns the
	// outcome by re-reading the projection (re-loading the VM list or
	// polling /health) - which the frontend already does.
	if err := refresher.RefreshAsync(r.Context()); err != nil {
		h.writeRefreshError(w, refresher, err)
		return
	}

	h.writeAccepted(w, r, outcomes)
}

func (h *ClusterRefresh) writeAccepted(w http.ResponseWriter, r *http.Request, outcomes map[string]string) {
	resp := clusterRefreshResponse{RefreshedAt: time.Now().UTC().Format(time.RFC3339Nano), Clusters: outcomes}

	body, err := json.Marshal(resp)
	if err != nil {
		SetErrorMsg(w, "failed to marshal refresh response", err)

		if writeErr := writeClusterError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError); writeErr != nil {
			logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "failed to write internal_error", "component", "httpapi", "error", writeErr)
		}

		return
	}

	if err := writeJSON(w, http.StatusAccepted, body); err != nil {
		logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "failed to write refresh response", "component", "httpapi", "error", err)
	}
}

// writeRefreshError maps a refresher error to the correct HTTP response, including
// the Retry-After header for ErrRefreshTooSoon. Extracted from ServeHTTP to keep
// its Cognitive Complexity under the go:S3776 threshold.
func (h *ClusterRefresh) writeRefreshError(w http.ResponseWriter, refresher *inventory.Refresher, err error) {
	switch {
	case errors.Is(err, inventory.ErrRefreshTooSoon):
		retryAfter := h.computeRetryAfterSeconds(refresher, err)

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
			h.log.Warn("failed to write refresh_too_soon", "component", "httpapi", "error", writeErr)
		}
	case errors.Is(err, inventory.ErrClusterUnreachable):
		SetErrorMsg(w, "manual refresh failed: cluster unreachable", err)

		if writeErr := writeClusterError(w, http.StatusBadGateway, "cluster_unreachable", "refresh failed: cluster is not reachable"); writeErr != nil {
			h.log.Warn("failed to write cluster_unreachable", "component", "httpapi", "error", writeErr)
		}
	default:
		SetErrorMsg(w, "manual refresh failed", err)

		if writeErr := writeClusterError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError); writeErr != nil {
			h.log.Warn("failed to write internal_error", "component", "httpapi", "error", writeErr)
		}
	}
}

// computeRetryAfterSeconds returns how many seconds the client should wait
// before retrying. When err carries the precise remaining guard time
// (*inventory.TooSoonError), that value is used - not the full configured
// interval, so a refusal near the end of the guard window reports a short
// wait, not the whole interval again. Falls back to the full interval only
// if the error doesn't carry a remaining time.
func (h *ClusterRefresh) computeRetryAfterSeconds(refresher *inventory.Refresher, err error) int {
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

	seconds := max(int(refresher.MinInterval().Seconds()), 1)

	return seconds
}
