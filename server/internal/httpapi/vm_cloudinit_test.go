package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"pvmss/server/internal/catalog"
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
	handler := httpapi.NewVMCloudInit(httpapi.VMCloudInitDeps{Projection: projection, Auth: authHandler, Reader: cluster.Fake{}, Writer: cluster.Fake{}, StatusReader: cluster.Fake{}, Store: st, Refresher: worker, Log: logger})

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
		{name: "untagged", cookie: aliceCookie(t, authHandler), path: "/api/v1/vms/default/109/cloudinit", body: `{"user":"x"}`, status: http.StatusNotFound, code: apiCodeNotFound},
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
func TestVMCloudInit_AddSSHKey(t *testing.T) {
	handler, authHandler, _ := newVMCloudInitHandler(t)
	path := "/api/v1/vms/default/101/cloudinit/ssh-keys"

	t.Run("unauthenticated", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, cloudInitRequest(http.MethodPost, path, `{"key":"ssh-ed25519 AAAA x"}`, nil))

		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", recorder.Code)
		}
	})

	t.Run("forbidden non owner", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, cloudInitRequest(http.MethodPost, path, `{"key":"ssh-ed25519 AAAA x"}`, bobCookie(t, authHandler)))

		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", recorder.Code)
		}
	})

	t.Run("invalid key rejected before agent", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, cloudInitRequest(http.MethodPost, path, `{"key":"ssh-rsa AAAA\ninjected"}`, aliceCookie(t, authHandler)))

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", recorder.Code)
		}

		assertAPIError(t, recorder.Body.Bytes(), "invalid_key")

		if len(cluster.FakeCallsFor(101)) != 0 {
			t.Fatalf("invalid key reached fake: %+v", cluster.FakeCallsFor(101))
		}
	})

	t.Run("injects valid key", func(t *testing.T) {
		assertSSHKeyInjected(t, handler, path, authHandler)
	})

	t.Run("agent failure maps to bad gateway", func(t *testing.T) {
		cluster.SetFakeSSHKeyError(cluster.ErrUnreachable)

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, cloudInitRequest(http.MethodPost, path, `{"key":"ssh-ed25519 AAAA x"}`, aliceCookie(t, authHandler)))

		if recorder.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502", recorder.Code)
		}
	})
}

func assertSSHKeyInjected(t *testing.T, handler http.Handler, path string, authHandler *httpapi.Auth) {
	t.Helper()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, cloudInitRequest(http.MethodPost, path, `{"user":"debian","key":"ssh-ed25519 AAAA x"}`, aliceCookie(t, authHandler)))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var sawAgent bool

	for _, c := range cluster.FakeCallsFor(101) {
		if c.Action == "add_ssh_key" {
			sawAgent = true

			if c.Content != "ssh-ed25519 AAAA x" || c.Name != "debian" {
				t.Errorf("add_ssh_key call = %+v", c)
			}
		}
	}

	if !sawAgent {
		t.Fatal("expected an add_ssh_key agent call")
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
//nolint:paralleltest // serial: shared fake VM and SQLite fixtures
func TestVMCloudInit_DocumentSwitch(t *testing.T) {
	handler, authHandler, st := newVMCloudInitHandler(t)
	cookie := aliceCookie(t, authHandler)
	path := "/api/v1/vms/default/101/cloudinit/document"

	get := func() map[string]any {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, cloudInitRequest(http.MethodGet, path, "", cookie))

		if recorder.Code != http.StatusOK {
			t.Fatalf("GET status = %d, body = %s", recorder.Code, recorder.Body.String())
		}

		var body map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}

		return body
	}

	if body := get(); body["templateId"] != nil {
		t.Fatalf("fresh templateId = %v, want null", body["templateId"])
	}

	tmplID := createCatalogTemplate(t, st)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, cloudInitRequest(http.MethodPut, path, `{"templateId":"`+tmplID+`"}`, cookie))

	if recorder.Code != http.StatusOK {
		t.Fatalf("switch status = %d, want 200, body = %s", recorder.Code, recorder.Body.String())
	}

	if body := get(); body["templateId"] != tmplID {
		t.Fatalf("templateId = %v, want %q", body["templateId"], tmplID)
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, cloudInitRequest(http.MethodPut, path, `{"templateId":""}`, cookie))

	if recorder.Code != http.StatusOK {
		t.Fatalf("detach status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	if body := get(); body["templateId"] != nil {
		t.Fatalf("templateId after detach = %v, want null", body["templateId"])
	}
}

// TestVMCloudInit_DocumentSwitch_Refusals - users cannot send YAML, and an
// unpublished template is a 409.
//
//nolint:paralleltest // serial: shared fake VM and SQLite fixtures
func TestVMCloudInit_DocumentSwitch_Refusals(t *testing.T) {
	handler, authHandler, st := newVMCloudInitHandler(t)
	cookie := aliceCookie(t, authHandler)
	path := "/api/v1/vms/default/101/cloudinit/document"

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, cloudInitRequest(http.MethodPut, path, `{"content":"#cloud-config\n"}`, cookie))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("YAML body status = %d, want 400, body = %s", recorder.Code, recorder.Body.String())
	}

	if _, err := catalog.CreateCloudInitTemplate(context.Background(), st, auditTestCluster, "Draft", "#cloud-config\n"); err != nil {
		t.Fatalf("CreateCloudInitTemplate: %v", err)
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, cloudInitRequest(http.MethodPut, path, `{"templateId":"draft"}`, cookie))

	if recorder.Code != http.StatusConflict {
		t.Fatalf("unpublished status = %d, want 409, body = %s", recorder.Code, recorder.Body.String())
	}

	assertAPIError(t, recorder.Body.Bytes(), "cloudinit_not_published")
}
