package httpapi

import (
	"errors"
	"net/http"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
	"strings"
	"time"
)

// cloneRequest is the body of POST /api/v1/vms/{cluster}/{vmid}/clone.
type cloneRequest struct {
	Name string `json:"name"`
}

// SetInventorySource wires the index the clone route resolves its source VM
// through. Without it the route cannot find any VM.
func (h *VMCreate) SetInventorySource(source inventory.LookupSource) {
	h.resolver = registryResolver{registry: depsSourceRegistry(source)}
}

// ServeClone handles POST /api/v1/vms/{cluster}/{vmid}/clone: a full copy of
// an owned VM under a new name. 202 means the clone task finished (the VM
// exists); the response carries the UPID like create does.
func (h *VMCreate) ServeClone(w http.ResponseWriter, r *http.Request) {
	// The clone waits for the task before responding, like create does, so
	// extend this response's write deadline the same way.
	if rc := http.NewResponseController(w); rc != nil {
		_ = rc.SetWriteDeadline(time.Now().Add(vm.MaxCreateTaskWait + createWriteDeadlineMargin))
	}

	identity, clusterName, vmid, ok := parseVMRequestTarget(h.auth, r, func(status int, code, message string) {
		h.writeCreateError(w, status, code, message)
	})
	if !ok {
		return
	}

	var req cloneRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeCreateError(w, http.StatusBadRequest, codeInvalidRequest, msgInvalidRequestBody)
		return
	}

	target, ok := h.resolveCreateTarget(w, clusterName)
	if !ok {
		return
	}

	// An unwired route (no SetInventorySource) cannot resolve any source.
	if h.resolver == nil {
		h.writeCreateError(w, http.StatusServiceUnavailable, "inventory_not_ready", msgInventoryNotReady)
		return
	}

	index, ok := loadClusterIndex(h.resolver, target.clusterName, func(status int, code, message string) {
		h.writeCreateError(w, status, code, message)
	})
	if !ok {
		return
	}

	ctx := policy.ContextWithAuditIP(r.Context(), clientIP(r, h.trustedProxyHops))

	result, err := vm.CloneOwn(ctx, identity, index, target.clusterName, vmid, strings.TrimSpace(req.Name), vm.CreateDeps{
		Store:     h.store,
		Creator:   target.creator,
		Writer:    target.writer,
		FreeSpace: target.freeSpace,
		Audit:     h.store,
		Log:       h.log,
		Services:  []*policy.Policy{h.policy},
	})
	if err != nil {
		h.writeCloneFailure(w, err)
		return
	}

	h.refreshAfterCreate(r.Context(), target.creator, target.clusterName, result.UPID)
	h.bindLeaseAfterCreate(r.Context(), target, result)

	h.writeCreateJSON(w, http.StatusAccepted, createResultDTO{
		Cluster: result.Cluster,
		VMID:    result.VMID,
		Name:    result.Name,
		Node:    result.Node,
		UPID:    result.UPID,
	})
}

// writeCloneFailure maps the source-resolution errors (which create never
// meets) and defers everything else to the create error table.
func (h *VMCreate) writeCloneFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrNotFound):
		h.writeCreateError(w, http.StatusNotFound, "not_found", msgVMNotFound)
	case errors.Is(err, vm.ErrForbidden):
		h.writeCreateError(w, http.StatusForbidden, "forbidden", msgNotYourVM)
	default:
		h.writeCreateFailure(w, err)
	}
}
