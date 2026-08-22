//nolint:noctx // test scaffolding uses in-memory requests
package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// errFailingTerminal is the sentinel returned by failingTerminalRelay's
// GetTermProxy to exercise the 502 console_unavailable path.
var errFailingTerminal = errors.New("terminal unreachable")

// serialTicketResponse mirrors the POST /serial-ticket 200 contract.
type serialTicketResponse = ticketResponse

// newVMSerialConsoleHandler builds the serial console handler over the fake
// dataset with a real audit store and a fresh in-memory ticket store. Mirrors
// newVMConsoleHandler.
//
//nolint:dupl // VNC and serial test handlers differ only in the handler constructor and port; the shared buildConsoleTestHelper does the heavy lifting. The remaining closure is type-specific (VMConsole vs VMSerialConsole) and cannot be unified without generics.
func newVMSerialConsoleHandler(t *testing.T) (*httpapi.VMSerialConsole, *httpapi.Auth, *vm.ConsoleTicketStore) {
	t.Helper()

	var handler *httpapi.VMSerialConsole

	var tickets *vm.ConsoleTicketStore

	var authHandler *httpapi.Auth

	_, authHandler, _ = buildConsoleTestHandler(t, 50002, "vm-serial-console.db",
		func(snap cluster.Snapshot, auth *httpapi.Auth, tk *vm.ConsoleTicketStore, st *store.Store, logger *slog.Logger) http.Handler {
			tickets = tk
			projection := buildProjectionWithIndex(t, snap, time.Now())
			handler = httpapi.NewVMSerialConsole(projection, auth, cluster.Fake{}, tk, st, logger)

			return handler
		},
	)

	return handler, authHandler, tickets
}

// serialRequest builds a request with the cluster and vmid path values set.
func serialRequest(method, path, body string, cookie *http.Cookie) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}

	req.Header.Set("Content-Type", "application/json")

	if cookie != nil {
		req.AddCookie(cookie)
	}

	req.SetPathValue("cluster", "default")
	req.SetPathValue("vmid", pathVmid(path))

	return req
}

// TestVMSerialConsole_PostSerialTicket_OwnerGetsOpaqueToken — the owner of VM
// 100 receives a 200 with a non-empty opaque token. No Proxmox ticket, node, or
// port leaks into the response.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMSerialConsole_PostSerialTicket_OwnerGetsOpaqueToken(t *testing.T) {
	handler, authHandler, _ := newVMSerialConsoleHandler(t)
	assertOwnerGetsOpaqueToken(t, handler, serialRequest, "/api/v1/vms/default/100/serial-ticket", aliceCookie(t, authHandler))
}

// TestVMSerialConsole_PostSerialTicket_NonOwnerForbidden — a non-owner gets 403.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMSerialConsole_PostSerialTicket_NonOwnerForbidden(t *testing.T) {
	handler, authHandler, _ := newVMSerialConsoleHandler(t)
	cookie := bobCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, serialRequest(http.MethodPost, "/api/v1/vms/default/100/serial-ticket", "", cookie))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}

	assertAPIError(t, rec.Body.Bytes(), apiCodeForbidden)
}

// TestVMSerialConsole_PostSerialTicket_NotFound — a non-existent VMID gets 404.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMSerialConsole_PostSerialTicket_NotFound(t *testing.T) {
	handler, authHandler, _ := newVMSerialConsoleHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, serialRequest(http.MethodPost, "/api/v1/vms/default/999/serial-ticket", "", cookie))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}

	assertAPIError(t, rec.Body.Bytes(), apiCodeNotFound)
}

// TestVMSerialConsole_PostSerialTicket_Unauthenticated — no cookie → 401.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMSerialConsole_PostSerialTicket_Unauthenticated(t *testing.T) {
	handler, _, _ := newVMSerialConsoleHandler(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, serialRequest(http.MethodPost, "/api/v1/vms/default/100/serial-ticket", "", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

// TestVMSerialConsole_WebSocket_MissingTokenReturns400 — a WebSocket request
// without a token parameter is rejected with 400 before the upgrade.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMSerialConsole_WebSocket_MissingTokenReturns400(t *testing.T) {
	handler, authHandler, _ := newVMSerialConsoleHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, serialRequest(http.MethodGet, "/api/v1/vms/default/100/serial/websocket", "", cookie))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

// TestVMSerialConsole_WebSocket_InvalidTokenReturns400 — a WebSocket request
// with a token that was never issued is rejected with 400.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMSerialConsole_WebSocket_InvalidTokenReturns400(t *testing.T) {
	handler, authHandler, _ := newVMSerialConsoleHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec := httptest.NewRecorder()
	req := serialRequest(http.MethodGet, "/api/v1/vms/default/100/serial/websocket?token=never-issued", "", cookie)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

// TestVMSerialConsole_WebSocket_TicketBoundToDifferentVMRejected — a serial
// ticket issued for VM 100 cannot be used against VM 101.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMSerialConsole_WebSocket_TicketBoundToDifferentVMRejected(t *testing.T) {
	handler, authHandler, tickets := newVMSerialConsoleHandler(t)

	assertTicketBoundToDifferentVMRejected(t, consoleTestParams{
		kind:       vm.KindTerminal,
		handler:    handler,
		tickets:    tickets,
		cookie:     aliceCookie(t, authHandler),
		request:    serialRequest,
		ticketPath: func(vmid int) string { return "/api/v1/vms/default/" + strconv.Itoa(vmid) + "/serial-ticket" },
		wsPath:     func(vmid int) string { return "/api/v1/vms/default/" + strconv.Itoa(vmid) + "/serial/websocket" },
	})
}

// TestVMSerialConsole_WebSocket_ValidTokenUpgradesAndRelaysEcho — with a valid
// token, the handler upgrades to WebSocket and the fake serial relay echoes
// keystrokes back as "0:len:data" frames. A real WebSocket client dials the
// endpoint, sends a few bytes, and asserts it receives an echoed frame —
// proving the serial relay is genuinely functional, not a stub.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMSerialConsole_WebSocket_ValidTokenUpgradesAndRelaysEcho(t *testing.T) {
	handler, authHandler, _ := newVMSerialConsoleHandler(t)
	cookie := aliceCookie(t, authHandler)

	ticketRec := httptest.NewRecorder()
	handler.ServeHTTP(ticketRec, serialRequest(http.MethodPost, "/api/v1/vms/default/100/serial-ticket", "", cookie))

	if ticketRec.Code != http.StatusOK {
		t.Fatalf("ticket issuance: status = %d", ticketRec.Code)
	}

	var ticket serialTicketResponse

	_ = json.Unmarshal(ticketRec.Body.Bytes(), &ticket)

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/vms/{cluster}/{vmid}/serial-ticket", handler)
	mux.Handle("GET /api/v1/vms/{cluster}/{vmid}/serial/websocket", handler)

	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/vms/default/100/serial/websocket?token=" + ticket.Token

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	header := http.Header{}
	header.Set("Cookie", cookie.String())
	header.Set("Origin", server.URL)

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: header}) //nolint:bodyclose // coder/websocket owns the response body lifecycle per its Dial docs
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()

	// Send a few keystroke bytes; the fake relay echoes them back as
	// "0:len:data". Serial tunnels speak TEXT frames (matching the browser
	// xterm.js client and Proxmox's termproxy protocol), so the client dials
	// with MessageText.
	if err := conn.Write(ctx, websocket.MessageText, []byte("hi")); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, reader, err := conn.Reader(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	echo, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read echo: %v", err)
	}

	// The fake frames the echo as "0:2:" followed by "hi".
	want := "0:2:hi"
	if string(echo) != want {
		t.Fatalf("echo = %q, want %q", string(echo), want)
	}
}

// TestVMSerialConsole_PostSerialTicket_ClusterUnavailableReturns502 — when the
// cluster client's GetTermProxy fails, the response is 502 console_unavailable.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMSerialConsole_PostSerialTicket_ClusterUnavailableReturns502(t *testing.T) {
	handler, _, cookie := buildFailingRelayHandler(t, 50003, "vm-serial-console-502.db",
		func(snap cluster.Snapshot, authHandler *httpapi.Auth, tickets *vm.ConsoleTicketStore, st *store.Store, logger *slog.Logger) http.Handler {
			projection := buildProjectionWithIndex(t, snap, time.Now())

			return httpapi.NewVMSerialConsole(projection, authHandler, &failingTerminalRelay{}, tickets, st, logger)
		},
	)

	assertClusterUnavailableReturns502(t, handler, serialRequest, "/api/v1/vms/default/100/serial-ticket", cookie)
}

// failingTerminalRelay is a TerminalRelay whose GetTermProxy always fails —
// used to test the 502 console_unavailable path.
type failingTerminalRelay struct{}

func (failingTerminalRelay) GetTermProxy(_ context.Context, _ string, _ int, _ string) (cluster.TermProxyTicket, error) {
	return cluster.TermProxyTicket{}, errFailingTerminal
}

func (failingTerminalRelay) RelaySerial(_ context.Context, _ string, _ int, _ cluster.TermProxyTicket, _ io.ReadWriteCloser) error {
	return nil
}
