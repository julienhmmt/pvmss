package httpapi

import (
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
	"time"
)

func (h *VMDetail) handleDisk(w http.ResponseWriter, r *http.Request) {
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

	writer, ok := h.writerFor(w, clusterName)
	if !ok {
		return
	}

	resources, err := catalog.ApprovedResources(r.Context(), h.store, clusterName)
	if err != nil {
		SetErrorMsg(w, msgHardwareCatalogFailed, err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	deps := vm.DiskDependencies{
		Index:       index,
		Actor:       identity,
		ClusterName: clusterName,
		VMID:        vmid,
		Writer:      writer,
		Resources:   resources,
		Policy:      h.policy,
		Audit:       h.store,
		Refresher:   h.refresherFor(clusterName),
	}

	switch r.Method {
	case http.MethodPost:
		h.handleDiskCreate(w, r, deps, vmid)
	case http.MethodPut:
		h.handleDiskResize(w, r, deps, identity, clusterName, vmid)
	case http.MethodDelete:
		h.handleDiskDelete(w, r, deps, r.PathValue("diskKey"))
	default:
		w.Header().Set("Allow", "POST, PUT, DELETE")
		h.writeDetailError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)
	}
}

// handleDiskCreate adds a new disk to the VM from a POST body.
func (h *VMDetail) handleDiskCreate(w http.ResponseWriter, r *http.Request, deps vm.DiskDependencies, _ int) {
	var request diskRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}

	disk, err := vm.AddDisk(r.Context(), deps, cluster.DiskBus(request.Bus), request.Storage, request.SizeGB)
	if err != nil {
		h.writeDiskError(w, err)
		return
	}

	h.writeJSONStatus(w, http.StatusOK, disk)
}

// handleDiskResize grows an existing disk from a PUT body, then re-resolves the
// VM to return the updated disk.
func (h *VMDetail) handleDiskResize(w http.ResponseWriter, r *http.Request, deps vm.DiskDependencies, identity auth.Identity, clusterName string, vmid int) {
	// The resize waits for its Proxmox task (up to 60 s), past the 10 s
	// server WriteTimeout.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(hardwareWriteDeadline))

	var request resizeDiskRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}

	if err := vm.ResizeDisk(r.Context(), deps, r.PathValue("diskKey"), request.SizeGB); err != nil {
		h.writeDiskError(w, err)
		return
	}

	refreshed, ok := h.index(w, clusterName)
	if !ok {
		return
	}

	entity, err := vm.Resolve(refreshed, identity, clusterName, vmid)
	if err != nil {
		h.writeResolveError(w, err)
		return
	}

	for _, disk := range entity.Disks {
		if disk.Key == r.PathValue("diskKey") {
			h.writeJSONStatus(w, http.StatusOK, disk)
			return
		}
	}

	h.writeDiskError(w, vm.ErrDiskNotFound)
}

// handleDiskDelete removes a disk by key.
func (h *VMDetail) handleDiskDelete(w http.ResponseWriter, r *http.Request, deps vm.DiskDependencies, diskKey string) {
	if err := vm.DeleteDisk(r.Context(), deps, diskKey); err != nil {
		h.writeDiskError(w, err)
		return
	}

	h.writeJSONStatus(w, http.StatusOK, deleteResponse{Status: "deleted"})
}

// handleBootCDROM serves POST /vms/:cluster/:vmid/boot-cdrom: a one-time boot
// from the mounted CD-ROM. The VM must be stopped - the UI shuts a running VM
// down first (confirm dialog) and calls this once it is stopped. The server
// sets the CD-first boot order, starts the VM, and restores the original boot
// order once the guest is up (one-time semantics).
func (h *VMDetail) handleBootCDROM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		h.writeDetailError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)

		return
	}

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

	writer, ok := h.writerFor(w, clusterName)
	if !ok {
		return
	}

	if err := vm.BootFromCDROM(r.Context(), vm.BootDependencies{
		Index:        index,
		Actor:        identity,
		ClusterName:  clusterName,
		VMID:         vmid,
		Writer:       writer,
		Audit:        h.store,
		Refresher:    h.refresherFor(clusterName),
		StatusReader: h.statusReaderFor(clusterName),
	}); err != nil {
		h.writeBootCDROMError(w, err)
		return
	}

	h.writeJSONStatus(w, http.StatusOK, actionResponse{Status: "accepted"})
}

func (h *VMDetail) handleCDROM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		w.Header().Set("Allow", "PATCH")
		h.writeDetailError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)

		return
	}

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

	writer, ok := h.writerFor(w, clusterName)
	if !ok {
		return
	}

	resources, err := catalog.ApprovedResources(r.Context(), h.store, clusterName)
	if err != nil {
		SetErrorMsg(w, msgHardwareCatalogFailed, err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	var request cdromRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}

	state, err := vm.SetCDROM(r.Context(), vm.CDROMDependencies{
		Index:       index,
		Actor:       identity,
		ClusterName: clusterName,
		VMID:        vmid,
		Writer:      writer,
		Resources:   resources,
		Audit:       h.store,
		Refresher:   h.refresherFor(clusterName),
	}, request.Action, request.ISOVolID)
	if err != nil {
		h.writeCDROMError(w, err)
		return
	}

	h.writeJSONStatus(w, http.StatusOK, state)
}
