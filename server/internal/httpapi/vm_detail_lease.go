package httpapi

import (
	"context"
	"errors"
	"net/http"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"time"
)

// leaseExtendRequest is the owner's PUT /vms/:cluster/:vmid/lease body.
type leaseExtendRequest struct {
	Days int `json:"days"`
}

// leaseResponse carries the VM's new expiry after an extension.
type leaseResponse struct {
	ExpiresAt string `json:"expiresAt"`
}

// handleLease routes the /lease sub-resource by method: PUT extends the
// lease (owner), DELETE removes it (administrator).
func (h *VMDetail) handleLease(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		h.handleLeaseExtend(w, r)
	case http.MethodDelete:
		h.handleLeaseRemove(w, r)
	default:
		w.Header().Set("Allow", "PUT, DELETE")
		h.writeDetailError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)
	}
}

// handleLeaseExtend serves PUT /vms/:cluster/:vmid/lease. The owner asks for
// days more; the server clamps to the cluster's lease_days.
func (h *VMDetail) handleLeaseExtend(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeDetailError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	clusterName, vmid, ok := h.parsePath(r)
	if !ok {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidVMPath)
		return
	}

	index, ok := h.index(w, clusterName)
	if !ok {
		return
	}

	// The ownership gate runs before the body is read, so a foreign VM is
	// refused with 403 whatever the body holds.
	if _, err := vm.Resolve(index, identity, clusterName, vmid, h.store); err != nil {
		h.writeResolveError(w, err)
		return
	}

	var req leaseExtendRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}

	gabarit, err := h.policy.Gabarit(r.Context(), clusterName)
	if err != nil {
		h.writeActionError(w, err)
		return
	}

	lease, err := vm.ExtendLease(r.Context(), index, h.store, vm.LeaseExtension{
		Actor: identity, ClusterName: clusterName, VMID: vmid, Days: req.Days, MaxDays: gabarit.LeaseDays,
	}, time.Now())
	if err != nil {
		h.writeLeaseError(w, err)
		return
	}

	h.writeJSONStatus(w, http.StatusOK, leaseResponse{ExpiresAt: lease.ExpiresAt.UTC().Format(time.RFC3339)})
}

// handleLeaseRemove serves DELETE /vms/:cluster/:vmid/lease. Administrators
// only: removing the lease exempts the VM from expiry.
func (h *VMDetail) handleLeaseRemove(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeDetailError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	if !identity.IsAdmin {
		h.writeDetailError(w, http.StatusForbidden, "forbidden", "administrator role required")
		return
	}

	clusterName, vmid, ok := h.parsePath(r)
	if !ok {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidVMPath)
		return
	}

	if err := vm.RemoveLease(r.Context(), h.store, identity, clusterName, vmid, ""); err != nil {
		h.writeLeaseError(w, err)
		return
	}

	h.writeJSONStatus(w, http.StatusOK, deleteResponse{Status: "removed"})
}

// writeLeaseError maps lease errors to HTTP statuses, deferring VM scope
// errors to the shared resolve mapping.
func (h *VMDetail) writeLeaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrLeaseNotFound):
		h.writeDetailError(w, http.StatusNotFound, "lease_not_found", "this vm has no lease")
	case errors.Is(err, vm.ErrLeaseDisabled):
		h.writeDetailError(w, http.StatusConflict, "lease_disabled", "leases are disabled for this cluster")
	case errors.Is(err, vm.ErrLeaseDays):
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", "days must be at least 1")
	case errors.Is(err, vm.ErrNotFound), errors.Is(err, vm.ErrForbidden):
		h.writeResolveError(w, err)
	default:
		SetErrorMsg(w, "vm lease failed", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
	}
}

// leaseExpiry returns the VM's lease expiry in RFC3339 UTC, or "" when it has
// no lease. A read failure is logged and reported as no lease, so the detail
// page still renders.
func (h *VMDetail) leaseExpiry(ctx context.Context, clusterName string, vmid int) string {
	if h.store == nil {
		return ""
	}

	lease, err := h.store.GetVMLease(ctx, clusterName, vmid)
	if errors.Is(err, store.ErrLeaseNotFound) {
		return ""
	}

	if err != nil {
		h.log.WarnContext(ctx, "vm lease read failed", "component", "httpapi", "cluster", clusterName, "vmid", vmid, "error", err)

		return ""
	}

	return lease.ExpiresAt.UTC().Format(time.RFC3339)
}
