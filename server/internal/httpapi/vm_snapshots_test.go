//nolint:noctx // test scaffolding uses in-memory requests
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

//nolint:wsl_v5 // handler tests keep request setup and assertions adjacent
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

//nolint:wsl_v5 // handler tests keep request setup and assertions adjacent
func snapshotRequest(method, path, body string, cookie *http.Cookie) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	if cookie != nil {
		request.AddCookie(cookie)
	}

	request.SetPathValue("cluster", "default")
	request.SetPathValue("vmid", pathVmid(path))

	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) >= 6 {
		request.SetPathValue("name", segments[5])
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
		{name: "non owner", cookie: bobCookie(t, authHandler), path: "/api/v1/vms/default/101/snapshots", body: `{"name":"x"}`, status: http.StatusForbidden, code: "forbidden"},
		{name: "untagged", cookie: aliceCookie(t, authHandler), path: "/api/v1/vms/default/109/snapshots", body: `{"name":"x"}`, status: http.StatusNotFound, code: "not_found"},
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
