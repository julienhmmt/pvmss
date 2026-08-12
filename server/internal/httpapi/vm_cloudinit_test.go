package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"testing"
	"time"
)

const cloudInitConfigPath = "/api/v1/vms/default/101/cloudinit"

func newVMCloudInitHandler(t *testing.T) (*httpapi.VMCloudInit, *httpapi.Auth, *store.Store) {
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
	handler := httpapi.NewVMCloudInit(projection, authHandler, cluster.Fake{}, cluster.Fake{}, st, worker, logger)

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
		body   string
		status int
		code   string
	}{
		{name: "non owner", cookie: bobCookie(t, authHandler), path: cloudInitConfigPath, body: `{"user":"x"}`, status: http.StatusForbidden, code: apiCodeForbidden},
		{name: "untagged", cookie: aliceCookie(t, authHandler), path: "/api/v1/vms/default/109/cloudinit", body: `{"user":"x"}`, status: http.StatusNotFound, code: "not_found"},
		{name: "invalid static", cookie: aliceCookie(t, authHandler), path: cloudInitConfigPath, body: `{"ipMode":"invalid"}`, status: http.StatusBadRequest, code: "invalid_config"},
		{name: "forged node", cookie: aliceCookie(t, authHandler), path: cloudInitConfigPath, body: `{"user":"x","node":"evil"}`, status: http.StatusBadRequest, code: "invalid_config"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, cloudInitRequest(http.MethodPut, tt.path, tt.body, tt.cookie))

			if recorder.Code != tt.status {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, tt.status, recorder.Body.String())
			}

			assertAPIError(t, recorder.Body.Bytes(), tt.code)
		})
	}
}

//nolint:paralleltest // serial: shared fake VM and SQLite fixtures
func TestVMCloudInit_AdminCanUpdateAnyTaggedVM(t *testing.T) {
	handler, authHandler, _ := newVMCloudInitHandler(t)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, cloudInitRequest(http.MethodPut, "/api/v1/vms/default/103/cloudinit", `{"user":"admin-updated"}`, adminCookie(t, authHandler)))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake VM and SQLite fixtures
func TestVMCloudInit_SnippetNullEmptyAndPushFailure(t *testing.T) {
	handler, authHandler, st := newVMCloudInitHandler(t)
	cookie := aliceCookie(t, authHandler)

	get := func() map[string]any {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, cloudInitRequest(http.MethodGet, "/api/v1/vms/default/101/cloudinit/snippet", "", cookie))

		if recorder.Code != http.StatusOK {
			t.Fatalf("GET status = %d, body = %s", recorder.Code, recorder.Body.String())
		}

		var body map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}

		return body
	}
	if got := get(); got["content"] != nil {
		t.Fatalf("fresh content = %v, want null", got["content"])
	}

	body := `{"content":"#cloud-config\nusers: {}\n"}`
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, cloudInitRequest(http.MethodPut, "/api/v1/vms/default/101/cloudinit/snippet", body, cookie))

	if recorder.Code != http.StatusOK {
		t.Fatalf("save status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	if got := get(); got["content"] != "#cloud-config\nusers: {}\n" {
		t.Fatalf("saved content = %v", got["content"])
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, cloudInitRequest(http.MethodPut, "/api/v1/vms/default/101/cloudinit/snippet", `{"content":""}`, cookie))

	if recorder.Code != http.StatusOK {
		t.Fatalf("clear status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	if got := get(); got["content"] != "" {
		t.Fatalf("cleared content = %v, want empty string", got["content"])
	}

	cluster.SetFakeCloudInitPushError(errors.New("offline"))

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, cloudInitRequest(http.MethodPut, "/api/v1/vms/default/101/cloudinit/snippet", `{"content":"#cloud-config\nnew: true\n"}`, cookie))

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("push failure status = %d, want 502", recorder.Code)
	}

	assertAPIError(t, recorder.Body.Bytes(), "push_failed")

	stored, found, err := st.GetCloudInitSnippet(context.Background(), "default", 101)
	if err != nil || !found || stored.Content != "#cloud-config\nnew: true\n" {
		t.Fatalf("stored after push failure = %+v, found %v, err %v", stored, found, err)
	}
}

//nolint:paralleltest // serial: shared fake VM and SQLite fixtures
func TestVMCloudInit_SnippetRejectsInvalidBeforeCluster(t *testing.T) {
	handler, authHandler, st := newVMCloudInitHandler(t)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, cloudInitRequest(http.MethodPut, "/api/v1/vms/default/101/cloudinit/snippet", `{"content":"users: {}"}`, aliceCookie(t, authHandler)))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}

	if _, found, err := st.GetCloudInitSnippet(context.Background(), "default", 101); err != nil || found {
		t.Fatalf("invalid snippet persisted: found %v, err %v", found, err)
	}

	if len(cluster.FakeCallsFor(101)) != 0 {
		t.Fatalf("invalid snippet reached fake: %+v", cluster.FakeCallsFor(101))
	}
}
