//nolint:wsl_v5 // batch handler keeps validation and dispatch adjacent
package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/vm"
	"strings"
)

// VMStatusBatch serves POST /api/v1/vms/status — the batch live-status read
// (ADR 0001). The front's list converge loop polls this once per tick for all
// flipping rows instead of N per-VM calls. Each target is resolved through
// vm.Resolve (same ownership gate as every other VM endpoint), then read live
// via VMStatusReader. Read-only: never writes the projection.
type VMStatusBatch struct {
	resolver     vm.ClusterIndexResolver
	auth         *Auth
	statusReader cluster.VMStatusReader
	clients      cluster.ClientProvider
	fallback     cluster.VMStatusReader
	log          *slog.Logger
}

// VMStatusBatchDeps groups the collaborators for the batch status handler.
type VMStatusBatchDeps struct {
	Source       inventory.LookupSource
	Auth         *Auth
	StatusReader cluster.VMStatusReader
	Clients      cluster.ClientProvider
	Log          *slog.Logger
}

// NewVMStatusBatch creates the handler. StatusReader is the single-cluster
// fallback; multi-cluster resolution uses Clients to resolve per target.
func NewVMStatusBatch(deps VMStatusBatchDeps) *VMStatusBatch {
	h := &VMStatusBatch{
		auth:         deps.Auth,
		statusReader: deps.StatusReader,
		clients:      deps.Clients,
		fallback:     deps.StatusReader,
		log:          deps.Log,
	}
	if registry, ok := deps.Source.(*inventory.Registry); ok {
		h.resolver = registryResolver{registry: registry}
	}
	return h
}

// NewVMStatusBatchSingle creates the handler wired to a single projection.
// Use NewVMStatusBatch for multi-cluster deployments.
func NewVMStatusBatchSingle(projection *inventory.Projection, authHandler *Auth, statusReader cluster.VMStatusReader, log *slog.Logger) *VMStatusBatch {
	return &VMStatusBatch{
		resolver:     singleClusterResolver{projection: projection},
		auth:         authHandler,
		statusReader: statusReader,
		fallback:     statusReader,
		log:          log,
	}
}

// statusTargetRequest is one element of the POST /vms/status body.
type statusTargetRequest struct {
	Cluster string `json:"cluster"`
	VMID    int    `json:"vmid"`
}

// statusTargetResponse is one element of the response.
type statusTargetResponse struct {
	Cluster string `json:"cluster"`
	VMID    int    `json:"vmid"`
	Status  string `json:"status"`
	Lock    string `json:"lock,omitempty"`
	Uptime  int64  `json:"uptime"`
}

// ServeHTTP handles POST /api/v1/vms/status.
func (h *VMStatusBatch) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		h.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}

	if h.statusReader == nil {
		h.writeError(w, http.StatusServiceUnavailable, "no_status_reader", "live status reader not configured")
		return
	}

	targets, ok := h.decodeTargets(w, r)
	if !ok {
		return
	}

	results := h.readTargets(r.Context(), identity, targets)
	h.writeJSONStatus(w, http.StatusOK, results)
}

// decodeTargets parses and validates the request body. Returns the targets and
// ok=false when an error response has already been written.
func (h *VMStatusBatch) decodeTargets(w http.ResponseWriter, r *http.Request) ([]statusTargetRequest, bool) {
	var targets []statusTargetRequest
	if err := decodeJSONLimit(w, r, &targets, 1<<20); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return nil, false
	}

	if len(targets) == 0 {
		h.writeError(w, http.StatusBadRequest, "empty_targets", "targets must contain at least one VM")
		return nil, false
	}

	if len(targets) > vm.MaxBulkTargets {
		h.writeError(w, http.StatusBadRequest, "too_many_targets", "targets exceeds the maximum")
		return nil, false
	}

	return targets, true
}

// readTargets resolves and reads the live status for each target. A target
// that fails resolution or the live read is omitted from the response (not a
// whole-request failure), matching the spec in ticket 01b.
func (h *VMStatusBatch) readTargets(ctx context.Context, identity auth.Identity, targets []statusTargetRequest) []statusTargetResponse {
	results := make([]statusTargetResponse, 0, len(targets))

	for _, target := range targets {
		target.Cluster = strings.TrimSpace(target.Cluster)
		if target.Cluster == "" || target.VMID == 0 {
			continue
		}

		index, err := h.resolver.IndexFor(target.Cluster)
		if err != nil || index == nil {
			continue
		}

		entity, err := vm.Resolve(index, identity, target.Cluster, target.VMID)
		if err != nil {
			continue
		}

		reader := h.readerFor(target.Cluster)
		if reader == nil {
			continue
		}

		live, err := reader.VMStatus(ctx, entity.Node, target.VMID)
		if err != nil {
			h.log.Error("batch live status read failed", "component", "httpapi",
				"cluster", target.Cluster, "vmid", target.VMID, "error", err)
			continue
		}

		results = append(results, statusTargetResponse{
			Cluster: target.Cluster,
			VMID:    target.VMID,
			Status:  string(live.Status),
			Lock:    live.Lock,
			Uptime:  int64(live.Uptime.Seconds()),
		})
	}

	return results
}

// readerFor resolves the VMStatusReader for clusterName, falling back to the
// single-cluster reader when the per-cluster resolution is not available.
func (h *VMStatusBatch) readerFor(clusterName string) cluster.VMStatusReader {
	if h.clients == nil {
		return h.fallback
	}
	reader, err := resolveCapability(h.clients, h.fallback, clusterName, "VMStatusReader")
	if err != nil {
		return h.fallback
	}
	return reader
}

func (h *VMStatusBatch) writeError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Error("failed to write error response", "component", "httpapi", "code", code, "error", err)
	}
}

func (h *VMStatusBatch) writeJSONStatus(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		h.log.Error("failed to marshal response", "component", "httpapi", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	if err := writeJSON(w, status, body); err != nil {
		h.log.Error("failed to write response", "component", "httpapi", "error", err)
	}
}

// Compile-time interface checks.
var (
	_ cluster.VMStatusReader = cluster.Fake{}
	_ cluster.VMStatusReader = cluster.Proxmox{}
)
