//nolint:wsl_v5 // snapshot handlers keep validation and dispatch together
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strconv"
	"strings"
)

// VMSnapshots serves the live snapshot list and asynchronous snapshot actions.
type VMSnapshots struct {
	projection *inventory.Projection
	auth       *Auth
	reader     cluster.SnapshotReader
	writer     cluster.SnapshotWriter
	store      *store.Store
	policy     *policy.Policy
	log        *slog.Logger
}

// NewVMSnapshots creates the snapshot handler with the T05 Resolve projection.
//
//nolint:wsl_v5 // snapshot request boundaries keep validation and dispatch adjacent
func NewVMSnapshots(projection *inventory.Projection, authHandler *Auth, reader cluster.SnapshotReader, writer cluster.SnapshotWriter, st *store.Store, log *slog.Logger, services ...*policy.Policy) *VMSnapshots {
	var policyService *policy.Policy
	if len(services) > 0 {
		policyService = services[0]
	}
	if policyService == nil && st != nil {
		policyService = policy.New(st, projection, nil)
	}
	return &VMSnapshots{projection: projection, auth: authHandler, reader: reader, writer: writer, store: st, policy: policyService, log: log}
}

type snapshotCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	VMState     bool   `json:"vmstate"`
}

type snapshotDTO struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"createdAt"`
	VMState     bool   `json:"vmstate"`
}

type snapshotListDTO struct {
	Snapshots    []snapshotDTO `json:"snapshots"`
	MaxSnapshots int           `json:"maxSnapshots"`
}

type snapshotTaskDTO struct {
	Cluster string `json:"cluster"`
	VMID    int    `json:"vmid"`
	Name    string `json:"name"`
	UPID    string `json:"upid"`
}

// ServeHTTP dispatches the four snapshot routes registered by NewRouter.
//
//nolint:wsl_v5 // snapshot request boundaries keep validation and dispatch adjacent
func (h *VMSnapshots) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/rollback"):
		h.handleNamedAction(w, r, vm.RollbackSnapshot)
	case r.Method == http.MethodGet:
		h.handleList(w, r)
	case r.Method == http.MethodPost:
		h.handleCreate(w, r)
	case r.Method == http.MethodDelete:
		h.handleNamedAction(w, r, vm.DeleteSnapshot)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		h.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

//nolint:wsl_v5 // snapshot request boundaries keep validation and dispatch adjacent
func (h *VMSnapshots) handleList(w http.ResponseWriter, r *http.Request) {
	identity, clusterName, vmid, ok := h.requestTarget(w, r)
	if !ok {
		return
	}

	index := h.projection.Load()
	if index == nil {
		h.writeError(w, http.StatusServiceUnavailable, "inventory_not_ready", "inventory has not been populated yet")
		return
	}

	snapshots, maxSnapshots, err := vm.ListSnapshots(r.Context(), h.dependencies(index, identity, clusterName, vmid))
	if err != nil {
		h.writeSnapshotError(w, err)
		return
	}

	result := snapshotListDTO{Snapshots: make([]snapshotDTO, 0, len(snapshots)), MaxSnapshots: maxSnapshots}
	for _, snapshot := range snapshots {
		result.Snapshots = append(result.Snapshots, snapshotDTOFromModel(snapshot))
	}

	h.writePayload(w, http.StatusOK, result)
}

//nolint:wsl_v5 // snapshot request boundaries keep validation and dispatch adjacent
func (h *VMSnapshots) handleCreate(w http.ResponseWriter, r *http.Request) {
	var request snapshotCreateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	identity, clusterName, vmid, ok := h.requestTarget(w, r)
	if !ok {
		return
	}

	index := h.projection.Load()
	if index == nil {
		h.writeError(w, http.StatusServiceUnavailable, "inventory_not_ready", "inventory has not been populated yet")
		return
	}

	upid, err := vm.CreateSnapshot(r.Context(), h.dependencies(index, identity, clusterName, vmid), strings.TrimSpace(request.Name), request.Description, request.VMState)
	if err != nil {
		h.writeSnapshotError(w, err)
		return
	}

	h.writePayload(w, http.StatusAccepted, snapshotTaskDTO{Cluster: clusterName, VMID: vmid, Name: strings.TrimSpace(request.Name), UPID: upid})
}

type snapshotNamedAction func(context.Context, vm.SnapshotDependencies, string) (string, error)

//nolint:wsl_v5 // snapshot request boundaries keep validation and dispatch adjacent
func (h *VMSnapshots) handleNamedAction(w http.ResponseWriter, r *http.Request, action snapshotNamedAction) {
	identity, clusterName, vmid, ok := h.requestTarget(w, r)
	if !ok {
		return
	}

	index := h.projection.Load()
	if index == nil {
		h.writeError(w, http.StatusServiceUnavailable, "inventory_not_ready", "inventory has not been populated yet")
		return
	}

	name := r.PathValue("name")

	upid, err := action(r.Context(), h.dependencies(index, identity, clusterName, vmid), name)
	if err != nil {
		h.writeSnapshotError(w, err)
		return
	}

	h.writePayload(w, http.StatusAccepted, snapshotTaskDTO{Cluster: clusterName, VMID: vmid, Name: name, UPID: upid})
}

//nolint:wsl_v5 // snapshot request boundaries keep validation and dispatch adjacent
func (h *VMSnapshots) requestTarget(w http.ResponseWriter, r *http.Request) (auth.Identity, string, int, bool) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return auth.Identity{}, "", 0, false
	}

	clusterName := r.PathValue("cluster")

	vmid, err := strconv.Atoi(r.PathValue("vmid"))
	if clusterName == "" || err != nil || vmid < 1 {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "invalid VM path")
		return auth.Identity{}, "", 0, false
	}

	return identity, clusterName, vmid, true
}

//nolint:wsl_v5 // snapshot request boundaries keep validation and dispatch adjacent
func (h *VMSnapshots) dependencies(index *inventory.Index, actor auth.Identity, clusterName string, vmid int) vm.SnapshotDependencies {
	return vm.SnapshotDependencies{Index: index, Actor: actor, ClusterName: clusterName, VMID: vmid, Reader: h.reader, Writer: h.writer, Policy: h.policy, Audit: h.store}
}

//nolint:wsl_v5 // snapshot request boundaries keep validation and dispatch adjacent
func snapshotDTOFromModel(snapshot vm.Snapshot) snapshotDTO {
	return snapshotDTO{Name: snapshot.Name, Description: snapshot.Description, CreatedAt: snapshot.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"), VMState: snapshot.VMState}
}

//nolint:wsl_v5 // snapshot request boundaries keep validation and dispatch adjacent
func (h *VMSnapshots) writeSnapshotError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeError(w, http.StatusForbidden, "forbidden", "not your VM")
	case errors.Is(err, vm.ErrNotFound):
		h.writeError(w, http.StatusNotFound, "not_found", "VM not found")
	case errors.Is(err, vm.ErrInvalidSnapshotName):
		h.writeError(w, http.StatusBadRequest, "invalid_name", err.Error())
	case errors.Is(err, vm.ErrDuplicateSnapshotName):
		h.writeError(w, http.StatusBadRequest, "duplicate_name", err.Error())
	case errors.Is(err, vm.ErrMaxSnapshotsReached):
		h.writeError(w, http.StatusBadRequest, "max_snapshots_reached", err.Error())
	case errors.Is(err, vm.ErrVMStateRequiresRunning):
		h.writeError(w, http.StatusBadRequest, "vmstate_requires_running", "RAM state can only be captured while the VM is running")
	case errors.Is(err, vm.ErrVMStateUnsupportedStorage):
		h.writeError(w, http.StatusBadRequest, "vmstate_unsupported_storage", err.Error())
	case errors.Is(err, vm.ErrSnapshotNotFound):
		h.writeError(w, http.StatusNotFound, "snapshot_not_found", err.Error())
	case errors.Is(err, policy.ErrUnavailable):
		h.writeError(w, http.StatusServiceUnavailable, "policy_unavailable", "policy service is not configured")
	case errors.Is(err, cluster.ErrNotFound):
		h.writeError(w, http.StatusBadGateway, "cluster_error", "cluster rejected the request")
	default:
		h.log.Error("vm snapshot operation failed", "component", "httpapi", "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

//nolint:wsl_v5 // snapshot request boundaries keep validation and dispatch adjacent
func (h *VMSnapshots) writePayload(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	if err := writeJSON(w, status, body); err != nil {
		h.log.Error("failed to write snapshot response", "component", "httpapi", "error", err)
	}
}

//nolint:wsl_v5 // snapshot request boundaries keep validation and dispatch adjacent
func (h *VMSnapshots) writeError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Error("failed to write snapshot error", "component", "httpapi", "code", code, "error", err)
	}
}
