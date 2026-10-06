package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/logctx"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strings"
)

// AdminMigration serves the admin-only VM migration preflight and start.
type AdminMigration struct {
	resolver vm.ClusterIndexResolver
	auth     *Auth
	migrator cluster.Migrator
	status   cluster.VMStatusReader
	clients  cluster.ClientProvider
	store    *store.Store
	log      *slog.Logger
}

// AdminMigrationRegistryDeps groups the collaborators NewAdminMigrationWithRegistry needs.
type AdminMigrationRegistryDeps struct {
	Source     inventory.LookupSource
	Projection *inventory.Projection
	Auth       *Auth
	Migrator   cluster.Migrator
	Status     cluster.VMStatusReader
	Clients    cluster.ClientProvider
	Store      *store.Store
	Log        *slog.Logger
}

// NewAdminMigrationWithRegistry creates the migration handler with per-request
// index and capability resolution keyed on the request's {cluster} path value.
func NewAdminMigrationWithRegistry(deps AdminMigrationRegistryDeps) *AdminMigration {
	handler := &AdminMigration{
		resolver: singleClusterResolver{projection: deps.Projection},
		auth:     deps.Auth,
		migrator: deps.Migrator,
		status:   deps.Status,
		clients:  deps.Clients,
		store:    deps.Store,
		log:      deps.Log,
	}
	if registry, ok := deps.Source.(*inventory.Registry); ok {
		handler.resolver = registryResolver{registry: registry}
	}

	return handler
}

type migrationRequest struct {
	Target string `json:"target"`
}

type migrationCandidateDTO struct {
	Node     string   `json:"node"`
	Warnings []string `json:"warnings"`
}

type migrationExclusionDTO struct {
	Node   string `json:"node"`
	Reason string `json:"reason"`
	Detail string `json:"detail"`
}

type migrationPreflightDTO struct {
	Cluster    string                  `json:"cluster"`
	VMID       int                     `json:"vmid"`
	Node       string                  `json:"node"`
	Running    bool                    `json:"running"`
	Lock       string                  `json:"lock"`
	Blocked    bool                    `json:"blocked"`
	Blockers   []string                `json:"blockers"`
	LocalDisks []string                `json:"localDisks"`
	Candidates []migrationCandidateDTO `json:"candidates"`
	Excluded   []migrationExclusionDTO `json:"excluded"`
}

type migrationStartDTO struct {
	Cluster string `json:"cluster"`
	VMID    int    `json:"vmid"`
	UPID    string `json:"upid"`
	Source  string `json:"source"`
	Target  string `json:"target"`
}

// ServePreflight serves GET .../migrate.
func (h *AdminMigration) ServePreflight(w http.ResponseWriter, r *http.Request) {
	deps, ok := h.dependencies(w, r)
	if !ok {
		return
	}

	preflight, err := vm.PreflightMigration(r.Context(), deps)
	if err != nil {
		h.writeMigrationError(w, r, deps, err)
		return
	}

	h.writePayload(w, http.StatusOK, preflightDTO(deps, preflight))
}

// ServeStart serves POST .../migrate.
func (h *AdminMigration) ServeStart(w http.ResponseWriter, r *http.Request) {
	var request migrationRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeError(w, http.StatusBadRequest, codeInvalidRequest, msgInvalidRequestBody)
		return
	}

	deps, ok := h.dependencies(w, r)
	if !ok {
		return
	}

	started, err := vm.StartMigration(r.Context(), deps, strings.TrimSpace(request.Target))
	if err != nil {
		h.writeMigrationError(w, r, deps, err)
		return
	}

	h.writePayload(w, http.StatusAccepted, migrationStartDTO{Cluster: deps.ClusterName, VMID: deps.VMID, UPID: started.UPID, Source: started.Source, Target: started.Target})
}

func (h *AdminMigration) dependencies(w http.ResponseWriter, r *http.Request) (vm.MigrationDependencies, bool) {
	writeErr := func(status int, code, message string) { h.writeError(w, status, code, message) }

	var (
		actor       auth.Identity
		clusterName string
		vmid        int
		ok          bool
	)

	if actor, clusterName, vmid, ok = parseVMRequestTarget(h.auth, r, writeErr); !ok {
		return vm.MigrationDependencies{}, false
	}

	index, ok := loadClusterIndex(h.resolver, clusterName, writeErr)
	if !ok {
		return vm.MigrationDependencies{}, false
	}

	migrator, err := resolveCapability(h.clients, h.migrator, clusterName, "Migrator")
	if err != nil {
		h.writeError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return vm.MigrationDependencies{}, false
	}

	status, err := resolveCapability(h.clients, h.status, clusterName, "VMStatusReader")
	if err != nil {
		h.writeError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return vm.MigrationDependencies{}, false
	}

	return vm.MigrationDependencies{Index: index, Actor: actor, ClusterName: clusterName, VMID: vmid, Migrator: migrator, Status: status, Store: h.store}, true
}

func preflightDTO(deps vm.MigrationDependencies, preflight vm.MigrationPreflight) migrationPreflightDTO {
	result := migrationPreflightDTO{
		Cluster: deps.ClusterName, VMID: deps.VMID, Node: preflight.Node, Running: preflight.Running,
		Lock: preflight.Lock, Blocked: preflight.Blocked(), Blockers: preflight.Blockers, LocalDisks: preflight.LocalDisks,
		Candidates: make([]migrationCandidateDTO, 0, len(preflight.Candidates)),
		Excluded:   make([]migrationExclusionDTO, 0, len(preflight.Excluded)),
	}
	for _, candidate := range preflight.Candidates {
		result.Candidates = append(result.Candidates, migrationCandidateDTO{Node: candidate.Node, Warnings: candidate.Warnings})
	}

	for _, excluded := range preflight.Excluded {
		result.Excluded = append(result.Excluded, migrationExclusionDTO{Node: excluded.Node, Reason: excluded.Reason, Detail: excluded.Detail})
	}

	return result
}

func (h *AdminMigration) writeMigrationError(w http.ResponseWriter, r *http.Request, deps vm.MigrationDependencies, err error) {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeError(w, http.StatusForbidden, "forbidden", msgNotYourVM)
	case errors.Is(err, vm.ErrNotFound):
		h.writeError(w, http.StatusNotFound, "not_found", msgVMNotFound)
	case errors.Is(err, vm.ErrInvalidMigrationTarget):
		h.warnRefusal(r, deps, "invalid_target")
		h.writeError(w, http.StatusBadRequest, "invalid_target", err.Error())
	case errors.Is(err, vm.ErrVMLocked):
		h.warnRefusal(r, deps, "vm_locked")
		h.writeError(w, http.StatusConflict, "vm_locked", err.Error())
	case errors.Is(err, vm.ErrMigrationBlocked):
		h.warnRefusal(r, deps, "migration_blocked")
		h.writeError(w, http.StatusConflict, "migration_blocked", err.Error())
	default:
		if code, message, ok := clusterRejectionResponse(err); ok {
			h.writeError(w, http.StatusBadGateway, code, message)
			return
		}

		if errors.Is(err, cluster.ErrNotFound) {
			h.writeError(w, http.StatusBadGateway, "cluster_error", msgClusterRejected)
			return
		}

		SetErrorMsg(w, "vm migration failed", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
	}
}

func (h *AdminMigration) warnRefusal(r *http.Request, deps vm.MigrationDependencies, reason string) {
	logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "vm migration refused",
		"component", "httpapi", "cluster", deps.ClusterName, "vmid", deps.VMID, "reason", reason)
}

func (h *AdminMigration) writePayload(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}

	if err := writeJSON(w, status, body); err != nil {
		h.log.Warn("failed to write migration response", "component", "httpapi", "error", err)
	}
}

func (h *AdminMigration) writeError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Warn("failed to write migration error", "component", "httpapi", "code", code, "error", err)
	}
}
