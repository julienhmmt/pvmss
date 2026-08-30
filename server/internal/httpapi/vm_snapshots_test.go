//nolint:noctx,goconst // test scaffolding uses in-memory requests; snapshot body literal reused across IDOR cases
package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

const (
	snapshotTestLogLevel  = "info"
	snapshotTestLogFormat = "json"
	snapshotTestLogOutput = "stdout"
)

type snapshotListResponse struct {
	Snapshots []struct {
		Name string `json:"name"`
	} `json:"snapshots"`
	MaxSnapshots int `json:"maxSnapshots"`
}

type snapshotTaskResponse struct {
	Cluster string `json:"cluster"`
	VMID    int    `json:"vmid"`
	Name    string `json:"name"`
	UPID    string `json:"upid"`
}

type snapshotErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func newVMSnapshotsHandler(t *testing.T) (*httpapi.VMSnapshots, *httpapi.Auth) {
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

	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "snapshots.db"), LogLevel: snapshotTestLogLevel, LogFormat: snapshotTestLogFormat, LogOutput: snapshotTestLogOutput})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	return httpapi.NewVMSnapshots(projection, authHandler, cluster.Fake{}, cluster.Fake{}, st, logger), authHandler
}

func snapshotRequest(method, path, body string, cookie *http.Cookie) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	if cookie != nil {
		request.AddCookie(cookie)
	}

	request.SetPathValue("cluster", "default")
	request.SetPathValue("vmid", pathVmid(path))

	// The snapshot name is the segment after "snapshots" — for
	// /snapshots/{name}, /snapshots/{name}/rollback and /snapshots/{name}/config.
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for index, segment := range segments {
		if segment == "snapshots" && index+1 < len(segments) {
			request.SetPathValue("name", segments[index+1])
			break
		}
	}

	return request
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshots_ListAndCreate_OwnerContract(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)

	getRecorder := httptest.NewRecorder()
	handler.ServeHTTP(getRecorder, snapshotRequest(http.MethodGet, "/api/v1/vms/default/101/snapshots", "", cookie))

	if getRecorder.Code != http.StatusOK {
		t.Fatalf("GET status = %d: %s", getRecorder.Code, getRecorder.Body.String())
	}

	var list snapshotListResponse
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}

	if len(list.Snapshots) != 0 || list.MaxSnapshots != 5 {
		t.Fatalf("list = %+v, want empty list and max 5", list)
	}

	postRecorder := httptest.NewRecorder()
	handler.ServeHTTP(postRecorder, snapshotRequest(http.MethodPost, "/api/v1/vms/default/101/snapshots", `{"name":"before-upgrade","description":"pre-migration"}`, cookie))

	if postRecorder.Code != http.StatusAccepted {
		t.Fatalf("POST status = %d, want 202: %s", postRecorder.Code, postRecorder.Body.String())
	}

	var task snapshotTaskResponse
	if err := json.Unmarshal(postRecorder.Body.Bytes(), &task); err != nil {
		t.Fatalf("decode task: %v", err)
	}

	if task.UPID == "" || task.Name != "before-upgrade" || task.VMID != 101 {
		t.Fatalf("task = %+v", task)
	}
}

// TestVMSnapshots_ConfigEndpoint — ticket 08: GET .../snapshots/{name}/config
// returns the stored config; a missing snapshot is a 404 snapshot_not_found;
// "current" resolves to the live config without a list entry.
//
//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshots_ConfigEndpoint(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, snapshotRequest(http.MethodGet, "/api/v1/vms/default/101/snapshots/ghost/config", "", cookie))

	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d, want 404: %s", missing.Code, missing.Body.String())
	}

	assertAPIError(t, missing.Body.Bytes(), "snapshot_not_found")

	seed := seedSnapshotRequest(t, "restore-point")

	configRecorder := httptest.NewRecorder()
	handler.ServeHTTP(configRecorder, snapshotRequest(http.MethodGet, "/api/v1/vms/default/101/snapshots/"+seed+"/config", "", cookie))

	if configRecorder.Code != http.StatusOK {
		t.Fatalf("config status = %d, want 200: %s", configRecorder.Code, configRecorder.Body.String())
	}

	var body struct {
		Name   string            `json:"name"`
		Config map[string]string `json:"config"`
	}
	if err := json.Unmarshal(configRecorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode config: %v", err)
	}

	if body.Name != seed || len(body.Config) == 0 {
		t.Fatalf("config = %+v, want name %q and a non-empty config", body, seed)
	}

	currentRecorder := httptest.NewRecorder()
	handler.ServeHTTP(currentRecorder, snapshotRequest(http.MethodGet, "/api/v1/vms/default/101/snapshots/current/config", "", cookie))

	if currentRecorder.Code != http.StatusOK {
		t.Fatalf("current config status = %d, want 200: %s", currentRecorder.Code, currentRecorder.Body.String())
	}
}

// seedSnapshotRequest creates a snapshot on fake VM 101 through the fake's
// own async task path and returns its name.
func seedSnapshotRequest(t *testing.T, name string) string {
	t.Helper()

	upid, err := (cluster.Fake{}).CreateSnapshot(context.Background(), cluster.FakeNode01, 101, name, "", false)
	if err != nil {
		t.Fatalf("CreateSnapshot seed: %v", err)
	}

	for range 3 {
		if _, err := (cluster.Fake{}).TaskStatus(context.Background(), upid); err != nil {
			t.Fatalf("TaskStatus seed: %v", err)
		}
	}

	return name
}

// TestVMSnapshots_List_IncludesCapability — ticket 07: the list carries the
// snapshot capability so the create dialog can grey options with a reason.
//
//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshots_List_IncludesCapability(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, snapshotRequest(http.MethodGet, "/api/v1/vms/default/101/snapshots", "", cookie))

	if recorder.Code != http.StatusOK {
		t.Fatalf("GET status = %d: %s", recorder.Code, recorder.Body.String())
	}

	var body struct {
		Capability struct {
			CanSnapshot bool     `json:"canSnapshot"`
			CanVMState  bool     `json:"canVMState"`
			Warnings    []string `json:"warnings"`
		} `json:"capability"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode list: %v", err)
	}

	// Fake VM 101: disks on lvmthin (snapshot-capable), stopped (no RAM state).
	if !body.Capability.CanSnapshot {
		t.Error("canSnapshot = false, want true")
	}

	if body.Capability.CanVMState {
		t.Error("canVMState = true, want false (VM stopped)")
	}

	if len(body.Capability.Warnings) == 0 {
		t.Error("warnings empty, want the running-state reason")
	}
}

// TestVMSnapshots_ClusterRejection_SurfacesProxmoxMessage asserts ticket 02:
// a Proxmox rejection (4xx/5xx) is surfaced as a 502 with a stable machine
// code and Proxmox's own message — never a generic 500 — and that 401/403
// messages are suppressed (a PVE auth body can name the token).
//
//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshots_ClusterRejection_SurfacesProxmoxMessage(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)

	cases := []struct {
		name        string
		rejection   *cluster.RejectionError
		wantCode    string
		wantMessage string
	}{
		{
			name:        "storage unsupported",
			rejection:   &cluster.RejectionError{Status: 500, Method: "POST", Path: "/nodes/n1/qemu/101/snapshot", Message: "snapshot feature not available for storage 'local'"},
			wantCode:    "snapshot_storage_unsupported",
			wantMessage: "snapshot feature not available for storage 'local'",
		},
		{
			name:        "vm locked",
			rejection:   &cluster.RejectionError{Status: 500, Method: "POST", Path: "/nodes/n1/qemu/101/snapshot", Message: "cannot delete snapshot: VM is locked by an ongoing backup"},
			wantCode:    "vm_locked",
			wantMessage: "cannot delete snapshot: VM is locked by an ongoing backup",
		},
		{
			name:        "name already used",
			rejection:   &cluster.RejectionError{Status: 500, Method: "POST", Path: "/nodes/n1/qemu/101/snapshot", Message: "snapshot name already used"},
			wantCode:    "snapshot_name_exists",
			wantMessage: "snapshot name already used",
		},
		{
			name:        "auth error message suppressed",
			rejection:   &cluster.RejectionError{Status: 401, Method: "POST", Path: "/nodes/n1/qemu/101/snapshot", Message: "no such API token: user@pve!tokenname"},
			wantCode:    "cluster_rejected",
			wantMessage: "cluster rejected the request",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cluster.SetFakeSnapshotWriteError(tc.rejection)
			defer cluster.SetFakeSnapshotWriteError(nil)

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, snapshotRequest(http.MethodPost, "/api/v1/vms/default/101/snapshots", `{"name":"snap1"}`, cookie))

			if recorder.Code != http.StatusBadGateway {
				t.Fatalf("status = %d, want 502: %s", recorder.Code, recorder.Body.String())
			}

			var body snapshotErrorResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error: %v", err)
			}

			if body.Code != tc.wantCode {
				t.Errorf("code = %q, want %q", body.Code, tc.wantCode)
			}

			if body.Message != tc.wantMessage {
				t.Errorf("message = %q, want %q", body.Message, tc.wantMessage)
			}
		})
	}
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshots_CreateAuthorizationAndValidation(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)

	cases := []struct {
		name   string
		cookie *http.Cookie
		path   string
		body   string
		status int
		code   string
	}{
		{name: "non owner", cookie: bobCookie(t, authHandler), path: "/api/v1/vms/default/101/snapshots", body: `{"name":"x"}`, status: http.StatusForbidden, code: apiCodeForbidden},
		{name: "untagged", cookie: aliceCookie(t, authHandler), path: "/api/v1/vms/default/109/snapshots", body: `{"name":"x"}`, status: http.StatusNotFound, code: apiCodeNotFound},
		{name: "invalid name", cookie: aliceCookie(t, authHandler), path: "/api/v1/vms/default/101/snapshots", body: `{"name":"bad name"}`, status: http.StatusBadRequest, code: "invalid_name"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, snapshotRequest(http.MethodPost, test.path, test.body, test.cookie))

			if recorder.Code != test.status {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, test.status, recorder.Body.String())
			}

			assertAPIError(t, recorder.Body.Bytes(), test.code)
		})
	}
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshots_RollbackAndDeleteUnknownSnapshot(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)

	for _, methodPath := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/api/v1/vms/default/101/snapshots/missing/rollback"},
		{method: http.MethodDelete, path: "/api/v1/vms/default/101/snapshots/missing"},
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, snapshotRequest(methodPath.method, methodPath.path, "", cookie))

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404: %s", methodPath.method, recorder.Code, recorder.Body.String())
		}

		assertAPIError(t, recorder.Body.Bytes(), "snapshot_not_found")
	}
}
