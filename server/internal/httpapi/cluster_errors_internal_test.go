package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
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

			assertPermissionDenied(t, write, denied)
		})
	}
}

// assertPermissionDenied runs one error writer and checks the 502 body names
// the missing privilege and path, and that the access log keeps the rejection.
func assertPermissionDenied(t *testing.T, write func(http.ResponseWriter, error), denied *cluster.RejectionError) {
	t.Helper()

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
}

// A transport failure carries the internal Proxmox URL and the Go client chain;
// the central helper must hand clients the fallback instead, while Proxmox's
// own rejection text stays intact.
func TestSafeClientMessage_HidesTransportURL(t *testing.T) {
	t.Parallel()

	urlErr := &url.Error{Op: "Get", URL: "https://192.168.1.1:8006/api2/json/nodes", Err: context.DeadlineExceeded}

	const fallback = "cluster is unavailable"

	cases := []struct {
		name string
		err  error
		want string
	}{
		{"wrapped url error", fmt.Errorf("%w: %w", cluster.ErrUnreachable, urlErr), fallback},
		{"bare url error", urlErr, fallback},
		{"unreachable sentinel", cluster.ErrUnreachable, fallback},
		{"url flattened into text", errors.New("create: " + urlErr.Error()), fallback},
		{"proxmox rejection kept", &cluster.RejectionError{Status: http.StatusBadRequest, Message: "has wrong type 'iso'"}, "proxmox  : HTTP 400: has wrong type 'iso'"},
		{"plain error kept", errors.New("disk too small"), "disk too small"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := safeClientMessage(tc.err, fallback); got != tc.want {
				t.Errorf("safeClientMessage() = %q, want %q", got, tc.want)
			}
		})
	}
}

// The create path echoes ErrClusterCreate text; a transport failure inside it
// must not reach the client.
func TestMapCreateError_ClusterErrorHidesTransportURL(t *testing.T) {
	t.Parallel()

	urlErr := &url.Error{Op: "Post", URL: "https://192.168.1.1:8006/api2/json/nodes/n/qemu", Err: context.DeadlineExceeded}
	err := fmt.Errorf("%w: create: %w", vm.ErrClusterCreate, fmt.Errorf("%w: %w", cluster.ErrUnreachable, urlErr))

	status, code, msg, ok := mapCreateError(err)
	if !ok || status != http.StatusBadGateway || code != "cluster_error" {
		t.Fatalf("mapCreateError = %d %q %v", status, code, ok)
	}

	for _, leak := range []string{"http", "8006", "api2"} {
		if strings.Contains(msg, leak) {
			t.Errorf("client message leaks %q: %q", leak, msg)
		}
	}
}
