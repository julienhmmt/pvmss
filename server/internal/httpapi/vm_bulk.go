//nolint:wsl_v5 // bulk handler keeps validation and dispatch adjacent
package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strings"
)

// VMBulk serves POST /api/v1/vms/bulk-action (T17). It authenticates the
// actor through the same Auth.Principal path every other VM endpoint uses,
// validates the whole request (action enum, target count 1..100) before
// touching any target, then delegates to vm.BulkAction — a pure loop over
// T05's existing Action(). No ownership logic, no Resolve() call, no
// cluster.Client call lives here; all of that stays inside Action(), called
// once per target.
type VMBulk struct {
	resolver   vm.ClusterIndexResolver
	projection *inventory.Projection
	auth       *Auth
	writer     cluster.Writer
	store      *store.Store
	refresher  vm.IndexRefresher
	log        *slog.Logger
}

// singleClusterResolver adapts a default projection to the
// ClusterIndexResolver interface — every target resolves against this one
// projection's current Index, keyed under whatever cluster name the target
// carries. This is the single-cluster wiring; NewVMBulkWithRegistry supplies
// the multi-cluster variant.
type singleClusterResolver struct {
	projection *inventory.Projection
}

func (r singleClusterResolver) IndexFor(clusterName string) (*inventory.Index, error) {
	idx := r.projection.Load()
	if idx == nil {
		return nil, fmt.Errorf("inventory has not been populated yet")
	}
	return idx, nil
}

// registryResolver adapts the inventory Registry to ClusterIndexResolver —
// each target's cluster name resolves to that cluster's own projection.
type registryResolver struct {
	registry *inventory.Registry
}

func (r registryResolver) IndexFor(clusterName string) (*inventory.Index, error) {
	return r.registry.Index(clusterName)
}

// NewVMBulk creates the handler wired to a single default projection. Use
// NewVMBulkWithRegistry when targets may span multiple clusters.
func NewVMBulk(projection *inventory.Projection, authHandler *Auth, writer cluster.Writer, st *store.Store, refresher vm.IndexRefresher, log *slog.Logger) *VMBulk {
	return &VMBulk{
		resolver:   singleClusterResolver{projection: projection},
		projection: projection,
		auth:       authHandler,
		writer:     writer,
		store:      st,
		refresher:  refresher,
		log:        log,
	}
}

// NewVMBulkWithRegistry creates the handler wired to a multi-cluster
// inventory Registry. Each target's cluster name resolves to that cluster's
// own projection via Registry.Index.
func NewVMBulkWithRegistry(registry *inventory.Registry, projection *inventory.Projection, authHandler *Auth, writer cluster.Writer, st *store.Store, refresher vm.IndexRefresher, log *slog.Logger) *VMBulk {
	return &VMBulk{
		resolver:   registryResolver{registry: registry},
		projection: projection,
		auth:       authHandler,
		writer:     writer,
		store:      st,
		refresher:  refresher,
		log:        log,
	}
}

// bulkActionRequestDTO is the POST /api/v1/vms/bulk-action body.
type bulkActionRequestDTO struct {
	Action  string          `json:"action"`
	Targets []vm.BulkTarget `json:"targets"`
}

// bulkActionResponseDTO is the 200 body.
type bulkActionResponseDTO struct {
	Results []vm.BulkTargetResult `json:"results"`
}

// ServeHTTP handles POST /api/v1/vms/bulk-action.
func (h *VMBulk) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	var req bulkActionRequestDTO
	if err := decodeJSONLimit(w, r, &req, 1<<20); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	req.Action = strings.TrimSpace(req.Action)
	if !vm.IsValidAction(req.Action) {
		h.writeError(w, http.StatusBadRequest, "invalid_action", fmt.Sprintf("unknown action %q", req.Action))
		return
	}

	targetCount := len(req.Targets)
	if targetCount == 0 {
		h.writeError(w, http.StatusBadRequest, "empty_targets", "targets must contain at least one VM")
		return
	}
	if targetCount > vm.MaxBulkTargets {
		h.writeError(w, http.StatusBadRequest, "too_many_targets", fmt.Sprintf("targets exceeds the maximum of %d", vm.MaxBulkTargets))
		return
	}

	results := vm.BulkAction(r.Context(), h.resolver, identity, req.Targets, req.Action, h.writer, h.store, h.refresher)
	h.writeJSONStatus(w, http.StatusOK, bulkActionResponseDTO{Results: results})
}

func (h *VMBulk) writeError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Error("failed to write error response", "component", "httpapi", "code", code, "error", err)
	}
}

func (h *VMBulk) writeJSONStatus(w http.ResponseWriter, status int, value any) {
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
	_ vm.ClusterIndexResolver = singleClusterResolver{}
	_ vm.ClusterIndexResolver = registryResolver{}
)
