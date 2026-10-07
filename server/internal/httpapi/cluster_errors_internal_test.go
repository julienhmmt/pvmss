package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"strings"
	"testing"
)

// A Proxmox 403 "Permission check failed (path, priv)" is a missing privilege
// on the service token: one stable code, the privilege named, the full error
// kept for the access log - on VM and admin endpoints alike.
func TestClusterPermissionDenied(t *testing.T) {
	t.Parallel()

	denied := &cluster.RejectionError{
		Status: http.StatusForbidden, Method: "POST", Path: "/nodes/n/qemu/999102/agent/exec",
		Message: "Permission check failed (/vms/999102, VM.GuestAgent.Unrestricted)",
	}

	writers := map[string]func(http.ResponseWriter, error){
		"cloud-init domain": (&VMCloudInit{log: slog.New(slog.DiscardHandler)}).writeDomainError,
		"vm network":        (&VMDetail{log: slog.New(slog.DiscardHandler)}).writeNetworkError,
		"migrate preflight": func(w http.ResponseWriter, err error) {
			code, message, _ := clusterRejectionResponse(w, err)
			_ = writeClusterError(w, http.StatusBadGateway, code, message)
		},
	}

	for name, write := range writers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			write(rec, denied)

			var body struct{ Code, Message string }

			_ = json.Unmarshal(rec.Body.Bytes(), &body)

			if rec.Code != http.StatusBadGateway || body.Code != "cluster_permission_denied" {
				t.Fatalf("got %d %q, want 502 cluster_permission_denied", rec.Code, body.Code)
			}

			if !strings.Contains(body.Message, "VM.GuestAgent.Unrestricted") || !strings.Contains(body.Message, "/vms/999102") {
				t.Errorf("message %q does not name the privilege and path", body.Message)
			}
		})
	}
}
