//nolint:noctx // test scaffolding uses in-memory requests
package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// sessionWatchTestInterval keeps the session re-check fast enough that a
// revoked stream must close well inside the test deadline.
const sessionWatchTestInterval = 20 * time.Millisecond

// logoutAs revokes the session behind cookie through the real logout handler.
func logoutAs(t *testing.T, authHandler *httpapi.Auth, cookie *http.Cookie) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	authHandler.Logout(rec, req)

	if rec.Code >= http.StatusMultipleChoices {
		t.Fatalf("logout status = %d, want 2xx", rec.Code)
	}
}

// assertWebSocketEnds reads until the server closes the socket. A socket still
// open after five seconds fails the test.
func assertWebSocketEnds(t *testing.T, conn *websocket.Conn) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for {
		if _, _, err := conn.Read(ctx); err != nil {
			if ctx.Err() != nil {
				t.Fatal("console websocket still open 5s after session revoked")
			}

			return
		}
	}
}

// TestVMConsole_WebSocket_ClosesWhenSessionRevoked - an open VNC console must
// stop relaying once its session is logged out, not only at upgrade time.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMConsole_WebSocket_ClosesWhenSessionRevoked(t *testing.T) {
	handler, authHandler, cookie := buildConsoleTestHandler(t, 50011, "session-watch-console.db",
		func(snap cluster.Snapshot, auth *httpapi.Auth, tk *vm.ConsoleTicketStore, st *store.Store, logger *slog.Logger) http.Handler {
			projection := buildProjectionWithIndex(t, snap, time.Now())

			return httpapi.NewVMConsoleWithRegistry(httpapi.VMConsoleRegistryDeps{Source: projectionRegistry(projection), Projection: projection, Auth: auth, Relay: cluster.Fake{}, Tickets: tk, Store: st, Log: logger}).WithSessionCheckInterval(sessionWatchTestInterval)
		},
	)

	ticketRec := httptest.NewRecorder()
	handler.ServeHTTP(ticketRec, consoleRequest(http.MethodPost, "/api/v1/vms/default/100/vnc-ticket", "", cookie))

	if ticketRec.Code != http.StatusOK {
		t.Fatalf("ticket issuance: status = %d", ticketRec.Code)
	}

	var ticket vncTicketResponse
	if err := json.Unmarshal(ticketRec.Body.Bytes(), &ticket); err != nil {
		t.Fatalf("decode ticket: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/vms/{cluster}/{vmid}/console/websocket", handler)

	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/vms/default/100/console/websocket?token=" + ticket.Token

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	header := http.Header{}
	header.Set("Cookie", cookie.String())
	header.Set("Origin", "http://example.com")

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: header}) //nolint:bodyclose // coder/websocket owns the response body lifecycle per its Dial docs
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()

	logoutAs(t, authHandler, cookie)
	assertWebSocketEnds(t, conn)
}

// TestVMMetrics_Stream_ClosesWhenSessionRevoked - an open metrics SSE stream
// must end once its session is logged out, not keep ticking.
//
//nolint:paralleltest // serial: shared fake fixtures
func TestVMMetrics_Stream_ClosesWhenSessionRevoked(t *testing.T) {
	handler, authHandler := newVMMetricsHandler(t)
	handler = handler.WithSessionCheckInterval(sessionWatchTestInterval)
	cookie := aliceCookie(t, authHandler)

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/vms/{cluster}/{vmid}/metrics/stream", handler)

	server := httptest.NewServer(mux)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/vms/default/100/metrics/stream", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	req.AddCookie(cookie)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("stream request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	logoutAs(t, authHandler, cookie)

	// Ticks arrive every second, so the body only ends if the handler returns.
	_, _ = io.Copy(io.Discard, resp.Body)

	if ctx.Err() != nil {
		t.Fatal("metrics stream still open 10s after session revoked")
	}
}
