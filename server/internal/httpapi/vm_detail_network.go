package httpapi

import (
	"context"
	"errors"
	"net/http"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
)

// writeRetrofitError maps a RetrofitToSeaBIOS failure to its HTTP response:
// refusals and the confirmation gate are 409s, a restart failure is a 500,
// anything unmapped is logged and reported as internal_error.
func (h *VMDetail) writeRetrofitError(w http.ResponseWriter, err error) {
	if h.writeCommonVMError(w, err) {
		return
	}

	switch {
	case errors.Is(err, vm.ErrRetrofitRefused):
		h.writeDetailError(w, http.StatusConflict, "retrofit_refused", err.Error())
	case errors.Is(err, vm.ErrRetrofitRequiresConfirmation):
		h.writeDetailError(w, http.StatusConflict, "retrofit_confirm_required", err.Error())
	case errors.Is(err, vm.ErrRetrofitRestartFailed):
		h.writeDetailError(w, http.StatusInternalServerError, "retrofit_restart_failed", err.Error())
	default:
		SetErrorMsg(w, "retrofit seabios failed", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
	}
}

func (h *VMDetail) writeHardwareError(w http.ResponseWriter, err error) {
	if h.writeCommonVMError(w, err) {
		return
	}

	switch {
	case errors.Is(err, vm.ErrEmptyHardwarePatch):
		h.writeDetailError(w, http.StatusBadRequest, "empty_patch", err.Error())
	case errors.Is(err, vm.ErrNotApproved):
		h.writeDetailError(w, http.StatusBadRequest, "not_approved", err.Error())
	case errors.Is(err, policy.ErrNodeCapacityExceeded):
		h.writeDetailError(w, http.StatusBadRequest, "capacity_exceeded", err.Error())
	case errors.Is(err, policy.ErrUnavailable):
		h.writeDetailError(w, http.StatusServiceUnavailable, "policy_unavailable", msgPolicyUnavailable)
	case errors.Is(err, vm.ErrHardwareExceedsLimit):
		h.writeDetailError(w, http.StatusBadRequest, "hardware_exceeds_limit", err.Error())
	case errors.Is(err, vm.ErrShutdownTimeout):
		h.writeDetailError(w, http.StatusConflict, "shutdown_timeout", "the VM did not shut down in time; nothing was changed")
	default:
		h.writeUnhandledVMError(w, "vm hardware operation failed", err)
	}
}

func (h *VMDetail) writeCommonVMError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeDetailError(w, http.StatusForbidden, "forbidden", msgNotYourVM)
	case errors.Is(err, vm.ErrNotFound):
		h.writeDetailError(w, http.StatusNotFound, "not_found", msgVMNotFound)
	default:
		return false
	}

	return true
}

func (h *VMDetail) writeUnhandledVMError(w http.ResponseWriter, message string, err error) {
	// A cluster rejection is not an unhandled error: surface Proxmox's own
	// message with its machine code instead of a generic 500 (ADR 0002).
	if code, msg, ok := clusterRejectionResponse(err); ok {
		h.writeDetailError(w, http.StatusBadGateway, code, msg)

		return
	}

	SetErrorMsg(w, message, err)
	h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
}

func (h *VMDetail) handleNetwork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", "PUT")
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

	var request networkRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}

	interfaces := make([]cluster.NetworkInterface, 0, len(request.Interfaces))
	for _, iface := range request.Interfaces {
		interfaces = append(interfaces, cluster.NetworkInterface{
			Index: iface.Index, Bridge: iface.Bridge, Model: iface.Model, VLAN: iface.VLAN, RateMbps: iface.RateMbps,
		})
	}

	updated, err := vm.UpdateNetwork(r.Context(), vm.NetworkDependencies{
		Index: index, Actor: identity, ClusterName: clusterName, VMID: vmid, Writer: writer,
		Resources: resources, Policy: h.policy, Audit: h.store, Refresher: h.refresherFor(clusterName),
	}, interfaces)
	if err != nil {
		h.writeNetworkError(w, err)
		return
	}

	h.writeJSONStatus(w, http.StatusOK, updated)
}

func (h *VMDetail) writeNetworkError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeDetailError(w, http.StatusForbidden, "forbidden", msgNotYourVM)
	case errors.Is(err, vm.ErrNotFound):
		h.writeDetailError(w, http.StatusNotFound, "not_found", msgVMNotFound)
	case errors.Is(err, vm.ErrBridgeNotApproved):
		h.writeDetailError(w, http.StatusBadRequest, "bridge_not_approved", err.Error())
	case errors.Is(err, vm.ErrNetworkCardsExceedLimit):
		h.writeDetailError(w, http.StatusBadRequest, "network_cards_exceed_limit", err.Error())
	case errors.Is(err, policy.ErrUnavailable):
		h.writeDetailError(w, http.StatusServiceUnavailable, "policy_unavailable", msgPolicyUnavailable)
	case errors.Is(err, vm.ErrInvalidNetworkModel), errors.Is(err, vm.ErrDuplicateNetworkIndex):
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, cluster.ErrClusterRejected):
		code, message, _ := clusterRejectionResponse(err)
		h.writeDetailError(w, http.StatusBadGateway, code, message)
	default:
		SetErrorMsg(w, "vm network operation failed", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
	}
}

func (h *VMDetail) writeCDROMError(w http.ResponseWriter, err error) {
	if h.writeCommonVMError(w, err) {
		return
	}

	switch {
	case errors.Is(err, vm.ErrInvalidCDROMAction):
		h.writeDetailError(w, http.StatusBadRequest, "invalid_action", err.Error())
	case errors.Is(err, vm.ErrISOVolumeNotApproved):
		h.writeDetailError(w, http.StatusBadRequest, "iso_not_approved", err.Error())
	default:
		h.writeUnhandledVMError(w, "vm cdrom operation failed", err)
	}
}

// writeBootCDROMError maps vm.BootFromCDROM errors to HTTP statuses.
func (h *VMDetail) writeBootCDROMError(w http.ResponseWriter, err error) {
	if h.writeCommonVMError(w, err) {
		return
	}

	switch {
	case errors.Is(err, vm.ErrCDROMNotMounted):
		h.writeDetailError(w, http.StatusConflict, "no_cdrom", err.Error())
	case errors.Is(err, cluster.ErrVMRunning):
		h.writeDetailError(w, http.StatusConflict, "vm_running", msgVMRunning)
	default:
		h.writeUnhandledVMError(w, "vm boot-cdrom failed", err)
	}
}

// allowedTagNames loads the admin-curated tag allowlist for a cluster.
// ListTags lazily seeds the mandatory pvmss tag, so the allowlist
// is never empty on a healthy store.
func (h *VMDetail) allowedTagNames(ctx context.Context, clusterName string) ([]string, error) {
	tags, err := catalog.ListTags(ctx, h.store, h.projection, clusterName)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}

	return names, nil
}

func (h *VMDetail) handleHardwareOptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
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

	entity, err := vm.Resolve(index, identity, clusterName, vmid)
	if err != nil {
		h.writeResolveError(w, err)
		return
	}

	resources, err := catalog.ApprovedResources(r.Context(), h.store, clusterName)
	if err != nil {
		SetErrorMsg(w, msgHardwareCatalogFailed, err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	if h.policy == nil {
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}

	gabarit, err := h.policy.Gabarit(r.Context(), clusterName)
	if err != nil {
		SetErrorMsg(w, "read gabarit failed", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	remaining := map[string]int{}

	for bus, max := range map[cluster.DiskBus]int{
		cluster.DiskBusVirtio: 16,
		cluster.DiskBusSCSI:   31,
		cluster.DiskBusSATA:   6,
		cluster.DiskBusIDE:    3,
	} {
		used := 0

		for _, disk := range entity.Disks {
			if disk.Bus == bus {
				used++
			}
		}

		remaining[string(bus)] = max - used
	}

	tagDTOs, err := hardwareTagDTOs(r.Context(), h, clusterName)
	if err != nil {
		SetErrorMsg(w, msgHardwareCatalogFailed, err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	h.writeJSONStatus(w, http.StatusOK, hardwareOptionsDTO{
		Storages: hardwareStorages(resources.Storages, index),
		Bridges:  hardwareBridges(resources.Bridges, entity.Node),
		ISOs:     hardwareISOs(resources.ISOs),
		Tags:     tagDTOs,
		Limits: vmLimitsDTO{
			MaxSockets:        gabarit.MaxSockets,
			MaxCores:          gabarit.MaxCores,
			MaxMemoryMB:       gabarit.MaxMemoryMB,
			MaxDiskPerVMGB:    gabarit.MaxDiskPerVMGB,
			MaxNetworkCards:   gabarit.MaxNetworkCards,
			RemainingBusSlots: remaining,
		},
	})
}

// hardwareTagDTOs loads the cluster's admin-curated tags for the VM tag
// picker. The protected pvmss tag is excluded - users cannot toggle it.
func hardwareTagDTOs(ctx context.Context, h *VMDetail, clusterName string) ([]hardwareTagDTO, error) {
	tags, err := catalog.ListTags(ctx, h.store, h.projection, clusterName)
	if err != nil {
		return nil, err
	}

	dtOs := make([]hardwareTagDTO, 0, len(tags))
	for _, tag := range tags {
		if tag.Protected {
			continue
		}

		dtOs = append(dtOs, hardwareTagDTO{Name: tag.Name, Color: tag.Color})
	}

	return dtOs, nil
}

func hardwareStorages(storages []catalog.Storage, index *inventory.Index) []hardwareStorageDTO {
	result := make([]hardwareStorageDTO, 0, len(storages))
	for _, storage := range storages {
		available, ok := vmCapableStorage(storage, index.StoragesByNode[storage.Node])
		if !ok {
			continue
		}

		result = append(result, hardwareStorageDTO{Node: storage.Node, Storage: storage.Name, Type: available.Type})
	}

	return result
}

func hardwareBridges(bridges []catalog.Bridge, node string) []hardwareBridgeDTO {
	result := make([]hardwareBridgeDTO, 0, len(bridges))
	for _, bridge := range bridges {
		if bridge.Node == node {
			result = append(result, hardwareBridgeDTO{Node: bridge.Node, Bridge: bridge.Name})
		}
	}

	return result
}

func hardwareISOs(isos []catalog.ISO) []hardwareISODTO {
	result := make([]hardwareISODTO, 0, len(isos))
	for _, iso := range isos {
		result = append(result, hardwareISODTO{
			VolID: iso.Storage + ":iso/" + iso.File,
			Node:  iso.Node, Storage: iso.Storage, Name: iso.File,
		})
	}

	return result
}

func (h *VMDetail) writeDiskError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeDetailError(w, http.StatusForbidden, "forbidden", msgNotYourVM)
	case errors.Is(err, vm.ErrNotFound):
		h.writeDetailError(w, http.StatusNotFound, "not_found", msgVMNotFound)
	case errors.Is(err, vm.ErrDiskNotFound):
		h.writeDetailError(w, http.StatusNotFound, "disk_not_found", err.Error())
	case errors.Is(err, vm.ErrBootDiskProtected):
		h.writeDetailError(w, http.StatusBadRequest, "boot_disk_protected", "the boot disk cannot be deleted")
	case errors.Is(err, vm.ErrVMNotStopped):
		h.writeDetailError(w, http.StatusBadRequest, "vm_not_stopped", err.Error())
	case errors.Is(err, vm.ErrDiskStorageNotApproved):
		h.writeDetailError(w, http.StatusBadRequest, "storage_not_approved", err.Error())
	case errors.Is(err, policy.ErrUnavailable):
		h.writeDetailError(w, http.StatusServiceUnavailable, "policy_unavailable", msgPolicyUnavailable)
	case errors.Is(err, vm.ErrDiskSizeExceedsLimit):
		h.writeDetailError(w, http.StatusBadRequest, "disk_size_exceeds_limit", err.Error())
	case errors.Is(err, vm.ErrDiskSizeNotGreater):
		h.writeDetailError(w, http.StatusBadRequest, "disk_size_not_greater", err.Error())
	case errors.Is(err, vm.ErrBusFull):
		h.writeDetailError(w, http.StatusBadRequest, "bus_full", err.Error())
	case errors.Is(err, cluster.ErrNotFound):
		h.writeDetailError(w, http.StatusBadGateway, "cluster_error", msgClusterRejected)
	case errors.Is(err, cluster.ErrClusterRejected):
		code, message, _ := clusterRejectionResponse(err)
		h.writeDetailError(w, http.StatusBadGateway, code, message)
	default:
		SetErrorMsg(w, "vm disk operation failed", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
	}
}
