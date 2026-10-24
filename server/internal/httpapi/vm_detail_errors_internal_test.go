//nolint:goconst // table rows repeat mapper names, codes and a malformed-body literal
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
	"testing"
)

// The VM error mappers are the contract between domain errors and the status
// and machine code the SPA switches on; each row pins one mapping.
func TestVMDetailErrorMappers(t *testing.T) {
	t.Parallel()

	h := &VMDetail{log: slog.New(slog.DiscardHandler)}
	unknown := errors.New("boom")

	cases := []struct {
		mapper string
		fn     func(http.ResponseWriter, error)
		err    error
		status int
		code   string
	}{
		{"retrofit", h.writeRetrofitError, vm.ErrForbidden, http.StatusForbidden, "forbidden"},
		{"retrofit", h.writeRetrofitError, vm.ErrNotFound, http.StatusNotFound, "not_found"},
		{"retrofit", h.writeRetrofitError, vm.ErrRetrofitRefused, http.StatusConflict, "retrofit_refused"},
		{"retrofit", h.writeRetrofitError, vm.ErrRetrofitRequiresConfirmation, http.StatusConflict, "retrofit_confirm_required"},
		{"retrofit", h.writeRetrofitError, vm.ErrRetrofitRestartFailed, http.StatusInternalServerError, "retrofit_restart_failed"},
		{"retrofit", h.writeRetrofitError, unknown, http.StatusInternalServerError, "internal_error"},
		{"hardware", h.writeHardwareError, vm.ErrEmptyHardwarePatch, http.StatusBadRequest, "empty_patch"},
		{"hardware", h.writeHardwareError, vm.ErrNotApproved, http.StatusBadRequest, "not_approved"},
		{"hardware", h.writeHardwareError, policy.ErrNodeCapacityExceeded, http.StatusBadRequest, "capacity_exceeded"},
		{"hardware", h.writeHardwareError, policy.ErrUnavailable, http.StatusServiceUnavailable, "policy_unavailable"},
		{"hardware", h.writeHardwareError, vm.ErrHardwareExceedsLimit, http.StatusBadRequest, "hardware_exceeds_limit"},
		{"hardware", h.writeHardwareError, unknown, http.StatusInternalServerError, "internal_error"},
		{"network", h.writeNetworkError, vm.ErrForbidden, http.StatusForbidden, "forbidden"},
		{"network", h.writeNetworkError, vm.ErrNotFound, http.StatusNotFound, "not_found"},
		{"network", h.writeNetworkError, vm.ErrBridgeNotApproved, http.StatusBadRequest, "bridge_not_approved"},
		{"network", h.writeNetworkError, vm.ErrNetworkCardsExceedLimit, http.StatusBadRequest, "network_cards_exceed_limit"},
		{"network", h.writeNetworkError, policy.ErrUnavailable, http.StatusServiceUnavailable, "policy_unavailable"},
		{"network", h.writeNetworkError, vm.ErrInvalidNetworkModel, http.StatusBadRequest, "invalid_request"},
		{"network", h.writeNetworkError, vm.ErrDuplicateNetworkIndex, http.StatusBadRequest, "invalid_request"},
		{"network", h.writeNetworkError, &cluster.RejectionError{Status: http.StatusBadRequest, Message: "bad bridge"}, http.StatusBadGateway, ""},
		{"network", h.writeNetworkError, unknown, http.StatusInternalServerError, "internal_error"},
		{"cdrom", h.writeCDROMError, vm.ErrInvalidCDROMAction, http.StatusBadRequest, "invalid_action"},
		{"cdrom", h.writeCDROMError, vm.ErrISOVolumeNotApproved, http.StatusBadRequest, "iso_not_approved"},
		{"cdrom", h.writeCDROMError, unknown, http.StatusInternalServerError, "internal_error"},
		{"disk", h.writeDiskError, vm.ErrDiskNotFound, http.StatusNotFound, "disk_not_found"},
		{"disk", h.writeDiskError, vm.ErrBootDiskProtected, http.StatusBadRequest, "boot_disk_protected"},
		{"disk", h.writeDiskError, vm.ErrVMNotStopped, http.StatusBadRequest, "vm_not_stopped"},
		{"disk", h.writeDiskError, vm.ErrDiskStorageNotApproved, http.StatusBadRequest, "storage_not_approved"},
		{"disk", h.writeDiskError, policy.ErrUnavailable, http.StatusServiceUnavailable, "policy_unavailable"},
		{"disk", h.writeDiskError, vm.ErrDiskSizeExceedsLimit, http.StatusBadRequest, "disk_size_exceeds_limit"},
		{"disk", h.writeDiskError, vm.ErrDiskSizeNotGreater, http.StatusBadRequest, "disk_size_not_greater"},
		{"disk", h.writeDiskError, vm.ErrBusFull, http.StatusBadRequest, "bus_full"},
		{"resolve", h.writeResolveError, vm.ErrForbidden, http.StatusForbidden, "forbidden"},
		{"resolve", h.writeResolveError, vm.ErrNotFound, http.StatusNotFound, "not_found"},
		{"resolve", h.writeResolveError, unknown, http.StatusInternalServerError, "internal_error"},
		{"action", h.writeActionError, vm.ErrActionRejected, http.StatusBadRequest, "invalid_action"},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s/%v", tc.mapper, tc.err), func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			tc.fn(rec, fmt.Errorf("wrapped: %w", tc.err))

			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}

			var body struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body %q: %v", rec.Body.String(), err)
			}

			if tc.code != "" && body.Code != tc.code {
				t.Fatalf("code = %q, want %q", body.Code, tc.code)
			}
		})
	}
}
