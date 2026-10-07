//nolint:goconst // table rows repeat mapper names and machine codes
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

type mapperCase struct {
	mapper string
	fn     func(http.ResponseWriter, error)
	err    error
	status int
	code   string
}

func runMapperCases(t *testing.T, cases []mapperCase) {
	t.Helper()

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

			if body.Code != tc.code {
				t.Fatalf("code = %q, want %q", body.Code, tc.code)
			}
		})
	}
}

func TestVMSnapshotsErrorMapper(t *testing.T) {
	t.Parallel()

	h := &VMSnapshots{log: slog.New(slog.DiscardHandler)}
	unknown := errors.New("boom")

	runMapperCases(t, []mapperCase{
		{"snapshot", h.writeSnapshotError, vm.ErrForbidden, http.StatusForbidden, "forbidden"},
		{"snapshot", h.writeSnapshotError, vm.ErrNotFound, http.StatusNotFound, "not_found"},
		{"snapshot", h.writeSnapshotError, vm.ErrInvalidSnapshotName, http.StatusBadRequest, "invalid_name"},
		{"snapshot", h.writeSnapshotError, vm.ErrDuplicateSnapshotName, http.StatusBadRequest, "duplicate_name"},
		{"snapshot", h.writeSnapshotError, vm.ErrMaxSnapshotsReached, http.StatusBadRequest, "max_snapshots_reached"},
		{"snapshot", h.writeSnapshotError, vm.ErrVMStateRequiresRunning, http.StatusBadRequest, "vmstate_requires_running"},
		{"snapshot", h.writeSnapshotError, vm.ErrVMStateUnsupportedStorage, http.StatusBadRequest, "vmstate_unsupported_storage"},
		{"snapshot", h.writeSnapshotError, vm.ErrSnapshotUnsupportedStorage, http.StatusBadRequest, "snapshot_storage_unsupported"},
		{"snapshot", h.writeSnapshotError, vm.ErrSnapshotNotFound, http.StatusNotFound, "snapshot_not_found"},
		{"snapshot", h.writeSnapshotError, policy.ErrUnavailable, http.StatusServiceUnavailable, "policy_unavailable"},
		{"snapshot", h.writeSnapshotError, cluster.ErrNotFound, http.StatusBadGateway, "cluster_error"},
		{"snapshot", h.writeSnapshotError, unknown, http.StatusInternalServerError, "internal_error"},
	})
}

func TestVMCloudInitErrorMapper(t *testing.T) {
	t.Parallel()

	h := &VMCloudInit{log: slog.New(slog.DiscardHandler)}
	unknown := errors.New("boom")

	runMapperCases(t, []mapperCase{
		{"cloudinit", h.writeDomainError, vm.ErrCloudInitWriteUnavailable, http.StatusConflict, "cloudinit_write_unavailable"},
		{"cloudinit", h.writeDomainError, vm.ErrCloudInitNotPublished, http.StatusConflict, "cloudinit_not_published"},
		{"cloudinit", h.writeDomainError, vm.ErrNotApproved, http.StatusBadRequest, "not_approved"},
		{"cloudinit", h.writeDomainError, policy.ErrUnavailable, http.StatusServiceUnavailable, "policy_unavailable"},
		{"cloudinit", h.writeDomainError, vm.ErrForbidden, http.StatusForbidden, "forbidden"},
		{"cloudinit", h.writeDomainError, vm.ErrNotFound, http.StatusNotFound, "not_found"},
		{"cloudinit", h.writeDomainError, vm.ErrInvalidCloudInitConfig, http.StatusBadRequest, "invalid_config"},
		{"cloudinit", h.writeDomainError, vm.ErrSSHKeyInvalid, http.StatusBadRequest, "invalid_key"},
		{"cloudinit", h.writeDomainError, cluster.ErrSSHKeyUserUnknown, http.StatusBadRequest, "ssh_user_unknown"},
		{"cloudinit", h.writeDomainError, vm.ErrNoCloudInitUser, http.StatusBadRequest, "no_cloudinit_user"},
		{"cloudinit", h.writeDomainError, vm.ErrGuestAgentDisabled, http.StatusConflict, "guest_agent_disabled"},
		{"cloudinit", h.writeDomainError, vm.ErrVMNotRunning, http.StatusConflict, "vm_not_running"},
		{"cloudinit", h.writeDomainError, vm.ErrGuestAgentUnreachable, http.StatusGatewayTimeout, "guest_agent_unreachable"},
		{"cloudinit", h.writeDomainError, vm.ErrSnippetPushFailed, http.StatusBadGateway, "push_failed"},
		{"cloudinit", h.writeDomainError, cluster.ErrUnreachable, http.StatusBadGateway, "cluster_error"},
		{"cloudinit", h.writeDomainError, unknown, http.StatusInternalServerError, "internal_error"},
	})
}
