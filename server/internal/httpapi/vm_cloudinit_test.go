package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"strings"
	"testing"
	"time"
)

const cloudInitConfigPath = "/api/v1/vms/default/101/cloudinit"

func newVMCloudInitHandler(t *testing.T) (*httpapi.VMCloudInit, *httpapi.Auth, *store.Store) {
	t.Helper()

	return newVMCloudInitHandlerDeps(t, cluster.Fake{}, cluster.Fake{})
}

// newVMCloudInitHandlerDeps is newVMCloudInitHandler with the cluster writer
// and VM status reader injectable, so a test can make the guest agent fail the
// way the real Proxmox client does.
func newVMCloudInitHandlerDeps(t *testing.T, writer cluster.Writer, status cluster.VMStatusReader) (*httpapi.VMCloudInit, *httpapi.Auth, *store.Store) {
	t.Helper()
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	snapshot, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	index := inventory.BuildIndex(snapshot)
	index.RefreshedAt = time.Now()
	projection := inventory.NewProjectionFromIndex(&index)
	authHandler := newAuthHandler(t)

	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "cloudinit.db"), LogLevel: snapshotTestLogLevel, LogFormat: snapshotTestLogFormat, LogOutput: snapshotTestLogOutput})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	worker := inventory.NewWorker(cluster.Fake{}, projection, time.Hour, logger)
	handler := httpapi.NewVMCloudInit(httpapi.VMCloudInitDeps{Source: projectionRegistry(projection), Projection: projection, Auth: authHandler, Reader: cluster.Fake{}, Writer: writer, StatusReader: status, Store: st, Refresher: worker, Log: logger})

	return handler, authHandler, st
}

func cloudInitRequest(method, path, body string, cookie *http.Cookie) *http.Request {
	req := detailRequest(method, path, body, cookie)
	req.SetPathValue("cluster", "default")
	req.SetPathValue("vmid", pathVmid(path))

	return req
}

//nolint:paralleltest // serial: shared fake VM and SQLite fixtures
func TestVMCloudInit_GetConfig_OmitsPassword(t *testing.T) {
	handler, authHandler, _ := newVMCloudInitHandler(t)
	request := cloudInitRequest(http.MethodGet, cloudInitConfigPath, "", aliceCookie(t, authHandler))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if _, ok := body["password"]; ok {
		t.Fatalf("password leaked in response: %s", recorder.Body.String())
	}

	if body["ipMode"] != string(cluster.CloudInitIPModeStatic) {
		t.Fatalf("ipMode = %v, want static", body["ipMode"])
	}
}

//nolint:paralleltest // serial: shared fake VM and SQLite fixtures
func TestVMCloudInit_ConfigAuthorizationAndValidation(t *testing.T) {
	handler, authHandler, _ := newVMCloudInitHandler(t)

	cases := []struct {
		name   string
		cookie *http.Cookie
		path   string
		status int
		code   string
	}{
		{name: "non owner", cookie: bobCookie(t, authHandler), path: cloudInitConfigPath, status: http.StatusForbidden, code: apiCodeForbidden},
		{name: "untagged", cookie: aliceCookie(t, authHandler), path: "/api/v1/vms/default/109/cloudinit", status: http.StatusNotFound, code: apiCodeNotFound},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, cloudInitRequest(http.MethodGet, tt.path, ``, tt.cookie))

			if recorder.Code != tt.status {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, tt.status, recorder.Body.String())
			}

			assertAPIError(t, recorder.Body.Bytes(), tt.code)
		})
	}
}

// TestVMCloudInit_ConsolePassword - the console-password action
// generates a password, applies it via the guest agent, and returns it once.
// A stopped VM is refused with a clear error before the agent is probed.
//
//nolint:paralleltest // serial: shared fake VM and SQLite fixtures
func TestVMCloudInit_ConsolePassword(t *testing.T) {
	handler, authHandler, _ := newVMCloudInitHandler(t)
	path := "/api/v1/vms/default/101/console-password"

	t.Run("unauthenticated", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, cloudInitRequest(http.MethodPost, path, "", nil))

		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", recorder.Code)
		}
	})

	t.Run("refuses stopped VM", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, cloudInitRequest(http.MethodPost, path, "", aliceCookie(t, authHandler)))

		if recorder.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409 (vm not running)", recorder.Code)
		}
	})
}

// TestVMCloudInit_DocumentSwitch - a user switches the VM to a published
// admin template (200), GET reports it, and "" detaches.
//
// TestVMCloudInit_DocumentSwitch_Refusals - users cannot send YAML, and an
// unpublished template is a 409.
//
// TestVMCloudInit_AddSSHKey_StoppedVM - injecting a key into a stopped guest is
// refused with 409 vm_not_running before any agent call is made. The fake
// starts VM 101 stopped.
//
// urlFailingAgentWriter answers the guest agent ping with the wrapped
// *url.Error the Proxmox HTTP client produces when the node times out.
type urlFailingAgentWriter struct{ cluster.Fake }

func (urlFailingAgentWriter) PingGuestAgent(context.Context, string, int) error {
	return fmt.Errorf("%w: %w", cluster.ErrUnreachable, &url.Error{
		Op:  "Post",
		URL: "https://192.168.1.1:8006/api2/json/nodes/miniquarium/qemu/101/agent/ping",
		Err: context.DeadlineExceeded,
	})
}

// runningLongEnoughStatus reports a running VM past the boot grace, so the
// preflight pings the agent instead of trusting it is still booting.
type runningLongEnoughStatus struct{ cluster.Fake }

func (runningLongEnoughStatus) VMStatus(context.Context, string, int) (cluster.VMLiveStatus, error) {
	return cluster.VMLiveStatus{Status: cluster.VMRunning, Uptime: 10 * time.Minute}, nil
}

// TestVMCloudInit_ConsolePassword_AgentTimeoutHidesInternals - a VM whose
// guest agent never answers gets a 504 with a fixed, user-actionable message;
// the Proxmox URL and the Go error chain stay in the access log only.
//
//nolint:paralleltest // serial: shared fake VM and SQLite fixtures
func TestVMCloudInit_ConsolePassword_AgentTimeoutHidesInternals(t *testing.T) {
	handler, authHandler, _ := newVMCloudInitHandlerDeps(t, urlFailingAgentWriter{}, runningLongEnoughStatus{})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, cloudInitRequest(http.MethodPost, "/api/v1/vms/default/101/console-password", "", aliceCookie(t, authHandler)))

	if recorder.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504, body = %s", recorder.Code, recorder.Body.String())
	}

	body := recorder.Body.String()
	for _, leak := range []string{"http", "8006", "api2", "192.168.1.1"} {
		if strings.Contains(body, leak) {
			t.Errorf("response body leaks %q: %s", leak, body)
		}
	}

	var got struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}

	if got.Code != "guest_agent_unreachable" || got.Message != "guest agent did not answer in time; is qemu-guest-agent installed and running?" {
		t.Errorf("body = %+v", got)
	}
}
