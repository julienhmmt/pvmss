package httpapi_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/httpapi"
	"strings"
	"testing"
	"time"
)

// A VM power action can wait 30s on a VM lock; behind the router it must
// outlive the server's write timeout instead of losing its response.
//
//nolint:paralleltest // serial: shared fake auth and session fixtures
func TestRouter_VMWriteOutlivesServerWriteTimeout(t *testing.T) {
	handler := newAuthHandler(t)
	slowAction := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)

		_, _ = w.Write([]byte("done"))
	})
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	router := httpapi.NewRouter(httpapi.RouterConfig{
		Health: ok, ClusterNodes: ok, ClusterRefresh: ok, VMs: ok, VMDetail: slowAction,
		Auth: handler, Log: slog.New(slog.DiscardHandler),
	})

	server := httptest.NewUnstartedServer(router)
	server.Config.ReadTimeout = 50 * time.Millisecond
	server.Config.WriteTimeout = 100 * time.Millisecond
	server.Start()
	t.Cleanup(server.Close)

	session, csrf := loginCSRF(t, handler, `{"username":"alice","password":"pvmss-alice"}`)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/api/v1/vms/default/100/actions", strings.NewReader(`{"action":"stop"}`))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	setCSRF(req, session, csrf)

	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatalf("POST: %v (response cut by the server write timeout)", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil || string(body) != "done" {
		t.Fatalf("status = %d, body = %q, err = %v; want \"done\"", resp.StatusCode, body, err)
	}
}
