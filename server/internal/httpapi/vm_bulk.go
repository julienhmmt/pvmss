//nolint:wsl_v5 // bulk handler keeps validation and dispatch adjacent
package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strings"
	"time"
)

// bulkActionBudget bounds the whole BulkAction loop: up to MaxBulkTargets
// sequential actions, each of which may wait on a VM lock for up to
// vm.MaxLockRetryWait, must stay under the response write deadline or the
// client loses the per-target results while the actions keep running.
const bulkActionBudget = 4 * time.Minute

// bulkWriteDeadlineMargin covers JSON encoding and the response write after
// the action budget is spent.
const bulkWriteDeadlineMargin = 15 * time.Second

// VMBulk serves POST /api/v1/vms/bulk-action. It authenticates the
// actor through the same Auth.Principal path every other VM endpoint uses,
// validates the whole request (action enum, target count 1..100) before
// touching any target, then delegates to vm.BulkAction - a pure loop over
// the existing Action(). No ownership logic, no Resolve() call, no
// cluster.Client call lives here; all of that stays inside Action(), called
// once per target.
type VMBulk struct {
	resolver          vm.ClusterIndexResolver
	writerResolver    vm.ClusterWriterResolver
	refresherResolver vm.ClusterRefresherResolver
	projection        *inventory.Projection
	auth              *Auth
	writer            cluster.Writer
	store             *store.Store
	refresher         vm.IndexRefresher
	log               *slog.Logger
}

// registryResolver adapts the inventory Registry to ClusterIndexResolver -
// each target's cluster name resolves to that cluster's own projection.
// depsSourceRegistry extracts the concrete inventory registry the deps
// structs carry; nil when Source is absent so registryResolver answers
// cluster_not_found instead of panicking.
func depsSourceRegistry(source inventory.LookupSource) *inventory.Registry {
	registry, _ := source.(*inventory.Registry)
	return registry
}

type registryResolver struct {
	registry *inventory.Registry
}

func (r registryResolver) IndexFor(clusterName string) (*inventory.Index, error) {
	if r.registry == nil {
		return nil, inventory.ErrClusterNotFound
	}

	return r.registry.Index(clusterName)
}

// clientWriterResolver adapts a cluster.ClientProvider to
// vm.ClusterWriterResolver - each bulk target's cluster name resolves to
// that cluster's own cluster.Writer, closing the gap where BulkDeps.Writer
// alone could not vary per target. fallback is used when clients is nil
// (single-cluster wiring), matching every other resolveCapability call site.
type clientWriterResolver struct {
	clients  cluster.ClientProvider
	fallback cluster.Writer
}

func (r clientWriterResolver) WriterFor(clusterName string) (cluster.Writer, error) {
	return resolveCapability(r.clients, r.fallback, clusterName, "Writer")
}

// VMBulkRegistryDeps groups the collaborators NewVMBulkWithRegistry needs for
// multi-cluster wiring. Bundling them keeps the parameter count under go:S107.
type VMBulkRegistryDeps struct {
	Registry   *inventory.Registry
	Projection *inventory.Projection
	Auth       *Auth
	Writer     cluster.Writer
	Store      *store.Store
	Refresher  vm.IndexRefresher
	Log        *slog.Logger
	Clients    cluster.ClientProvider
}

// NewVMBulkWithRegistry creates the handler wired to a multi-cluster
// inventory Registry. Each target's cluster name resolves to that cluster's
// own projection via Registry.Index.
func NewVMBulkWithRegistry(deps VMBulkRegistryDeps) *VMBulk {
	return &VMBulk{
		resolver:          registryResolver{registry: deps.Registry},
		writerResolver:    clientWriterResolver{clients: deps.Clients, fallback: deps.Writer},
		refresherResolver: registryRefresherResolver{registry: deps.Registry},
		projection:        deps.Projection,
		auth:              deps.Auth,
		writer:            deps.Writer,
		store:             deps.Store,
		refresher:         deps.Refresher,
		log:               deps.Log,
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

	// Extend the write deadline just past the shared action budget, then
	// bound the loop itself - targets the budget never reaches report
	// ctx.DeadlineExceeded as their per-target error.
	if rc := http.NewResponseController(w); rc != nil {
		_ = rc.SetWriteDeadline(time.Now().Add(bulkActionBudget + bulkWriteDeadlineMargin))
	}
	actionCtx, cancel := context.WithTimeout(r.Context(), bulkActionBudget)
	defer cancel()

	results := vm.BulkAction(actionCtx, vm.BulkDeps{
		Resolver:          h.resolver,
		WriterResolver:    h.writerResolver,
		RefresherResolver: h.refresherResolver,
		Actor:             identity,
		Writer:            h.writer,
		Audit:             h.store,
		Refresher:         h.refresher,
		TaskWait:          ActionTaskWait,
	}, req.Targets, req.Action)
	h.writeJSONStatus(w, http.StatusOK, bulkActionResponseDTO{Results: results})
}

func (h *VMBulk) writeError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Warn("failed to write error response", "component", "httpapi", "code", code, "error", err)
	}
}

func (h *VMBulk) writeJSONStatus(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		SetErrorMsg(w, "failed to marshal response", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	if err := writeJSON(w, status, body); err != nil {
		h.log.Warn("failed to write response", "component", "httpapi", "error", err)
	}
}

// Compile-time interface checks.
var (
	_ vm.ClusterIndexResolver = registryResolver{}
)
