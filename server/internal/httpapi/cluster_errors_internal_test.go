package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
	"strings"
	"testing"
)

// A Proxmox 403 "Permission check failed (path, priv)" is a missing privilege
// on the service token: one stable code, the privilege named, the full error
// kept for the access log - on VM and admin endpoints alike.
func TestClusterRejectionResponse_HandlesMissingDetails(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		ok   bool
	}{
		{"bare rejection", fmt.Errorf("action: %w", cluster.ErrClusterRejected), true},
		{"unauthorized", &cluster.RejectionError{Status: http.StatusUnauthorized, Message: "token identity"}, true},
		{"forbidden without ACL", &cluster.RejectionError{Status: http.StatusForbidden, Message: "token identity"}, true},
		{"unrelated", errors.New("database failure"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			response, ok := clusterRejectionResponse(tc.err)
			if ok != tc.ok || response.Code != "cluster_rejected" || response.Message != msgClusterRejected || response.Privilege != "" || response.Path != "" {
				t.Errorf("rejection response = %+v/%v", response, ok)
			}
		})
	}
}

func TestClusterPermissionDenied_ReportsPrivilegeAndPath(t *testing.T) {
	t.Parallel()

	denied := &cluster.RejectionError{
		Status: http.StatusForbidden, Method: "POST", Path: "/nodes/n/qemu/999102/agent/exec",
		Message: "Permission check failed (/vms/999102, VM.GuestAgent.Unrestricted)",
	}

	writers := map[string]func(http.ResponseWriter, error){
		"cloud-init domain": (&VMCloudInit{log: slog.New(slog.DiscardHandler)}).writeDomainError,
		"vm network":        (&VMDetail{log: slog.New(slog.DiscardHandler)}).writeNetworkError,
		"migrate preflight": func(w http.ResponseWriter, err error) {
			(&AdminMigration{log: slog.New(slog.DiscardHandler)}).writeMigrationError(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), vm.MigrationDependencies{}, err)
		},
	}

	for name, write := range writers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			logged := &logWriter{ResponseWriter: rec}
			write(logged, denied)

			if !errors.Is(logged.err, denied) {
				t.Errorf("access log lost the rejection: %v", logged.err)
			}

			var body struct{ Code, Message, Privilege, Path string }

			_ = json.Unmarshal(rec.Body.Bytes(), &body)

			if rec.Code != http.StatusBadGateway || body.Code != "cluster_permission_denied" {
				t.Fatalf("got %d %q, want 502 cluster_permission_denied", rec.Code, body.Code)
			}

			if body.Privilege != "VM.GuestAgent.Unrestricted" || body.Path != "/vms/999102" {
				t.Errorf("permission fields = %q/%q", body.Privilege, body.Path)
			}

			if !strings.Contains(body.Message, "VM.GuestAgent.Unrestricted") || !strings.Contains(body.Message, "/vms/999102") {
				t.Errorf("message %q does not name the privilege and path", body.Message)
			}
		})
	}
}
