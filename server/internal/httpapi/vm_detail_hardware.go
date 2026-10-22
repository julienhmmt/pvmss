package httpapi

import (
	"encoding/json"
	"net/http"
	"pvmss/server/internal/vm"
	"time"
)

// hardwareWriteDeadline outlives the server's global WriteTimeout (10 s): a
// hardware change on a running VM waits for the guest shutdown (up to 75 s).
const hardwareWriteDeadline = 2 * time.Minute

// parseHardwareRequest decodes and validates the PUT .../hardware body: at
// least one field must be present. Split out of handleHardware to keep its
// cyclomatic complexity under the linter's ceiling.
func (h *VMDetail) parseHardwareRequest(w http.ResponseWriter, r *http.Request) (hardwareRequest, bool) {
	var request hardwareRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return hardwareRequest{}, false
	}

	if request.Sockets == nil && request.Cores == nil && request.MemoryMB == nil && request.Tags == nil {
		h.writeDetailError(w, http.StatusBadRequest, "empty_patch", "at least one hardware field is required")
		return hardwareRequest{}, false
	}

	return request, true
}

func (h *VMDetail) handleHardware(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", "PUT")
		h.writeDetailError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)

		return
	}

	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(hardwareWriteDeadline))

	wctx, ok := h.resolveWriteContext(w, r)
	if !ok {
		return
	}

	request, ok := h.parseHardwareRequest(w, r)
	if !ok {
		return
	}

	allowedTags, err := h.allowedTagNames(r.Context(), wctx.clusterName)
	if err != nil {
		SetErrorMsg(w, msgHardwareCatalogFailed, err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	err = vm.UpdateHardware(r.Context(), vm.HardwareDependencies{
		Index: wctx.index, Actor: wctx.identity, ClusterName: wctx.clusterName, VMID: wctx.vmid, Writer: wctx.writer,
		Policy: h.policy, Audit: h.store, Refresher: h.refresherFor(wctx.clusterName), AllowedTags: allowedTags,
	}, vm.HardwarePatch{Sockets: request.Sockets, Cores: request.Cores, MemoryMB: request.MemoryMB, Tags: request.Tags})
	if err != nil {
		h.writeHardwareError(w, err)
		return
	}

	h.writeRefreshedEntity(w, r, wctx.identity, wctx.clusterName, wctx.vmid)
}

// handleEnableSerial serves POST /vms/:cluster/:vmid/serial - the serial-
// console retrofit for VMs created before serial0 was added at create time.
// Reuses vm.EnableSerialConsole (Resolve ownership gate → Writer.EnableSerial
// → audit + inventory refresh) and returns the refreshed entity so the UI can
// flip its "no serial" state without a poll cycle.
func (h *VMDetail) handleEnableSerial(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		h.writeDetailError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)

		return
	}

	wctx, ok := h.resolveWriteContext(w, r)
	if !ok {
		return
	}

	err := vm.EnableSerialConsole(r.Context(), vm.EnableSerialDependencies{
		Index:       wctx.index,
		Actor:       wctx.identity,
		ClusterName: wctx.clusterName,
		VMID:        wctx.vmid,
		Writer:      wctx.writer,
		Audit:       h.store,
		Refresher:   h.refresherFor(wctx.clusterName),
	})
	if err != nil {
		if h.writeCommonVMError(w, err) {
			return
		}

		SetErrorMsg(w, "enable serial console failed", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	h.writeRefreshedEntity(w, r, wctx.identity, wctx.clusterName, wctx.vmid)
}

// handleRetrofitSeaBIOS serves POST /vms/:cluster/:vmid/retrofit-seabios -
// the admin-only action that switches an existing UEFI VM to SeaBIOS so its
// graphical console becomes readable. Refuses
// VMs with TPM state or Secure Boot before changing anything; a running VM
// requires confirm=true in the request body. Reports each step's outcome.
func (h *VMDetail) handleRetrofitSeaBIOS(w http.ResponseWriter, r *http.Request) {
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

	// admin-only - a tenant cannot retrofit firmware.
	if !identity.IsAdmin {
		h.writeDetailError(w, http.StatusForbidden, "forbidden", msgAdminOnly)
		return
	}

	wctx, ok := h.resolveWriteContext(w, r)
	if !ok {
		return
	}

	var request struct {
		Confirm bool `json:"confirm"`
	}

	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			h.writeDetailError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
			return
		}
	}

	err = vm.RetrofitToSeaBIOS(r.Context(), vm.RetrofitDependencies{
		Index:        wctx.index,
		Actor:        wctx.identity,
		ClusterName:  wctx.clusterName,
		VMID:         wctx.vmid,
		Writer:       wctx.writer,
		StatusReader: h.statusReaderFor(wctx.clusterName),
		Audit:        h.store,
		Refresher:    h.refresherFor(wctx.clusterName),
		Confirm:      request.Confirm,
	})
	if err != nil {
		h.writeRetrofitError(w, err)
		return
	}

	h.writeRefreshedEntity(w, r, wctx.identity, wctx.clusterName, wctx.vmid)
}
