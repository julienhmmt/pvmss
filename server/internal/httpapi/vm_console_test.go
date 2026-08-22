//nolint:noctx // test scaffolding uses in-memory requests
package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
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

// vncTicketResponse mirrors the POST /vnc-ticket 200 contract.
type vncTicketResponse = ticketResponse

// newVMConsoleHandler builds the console handler over the fake dataset with a
// real audit store and a fresh in-memory ticket store.
//
//nolint:dupl // VNC and serial test handlers differ only in the handler constructor and port; the shared buildConsoleTestHelper does the heavy lifting. The remaining closure is type-specific (VMConsole vs VMSerialConsole) and cannot be unified without generics.
func newVMConsoleHandler(t *testing.T) (*httpapi.VMConsole, *httpapi.Auth, *vm.ConsoleTicketStore) {
	t.Helper()

	var handler *httpapi.VMConsole

	var tickets *vm.ConsoleTicketStore

	var authHandler *httpapi.Auth

	_, authHandler, _ = buildConsoleTestHandler(t, 50001, "vm-console.db",
		func(snap cluster.Snapshot, auth *httpapi.Auth, tk *vm.ConsoleTicketStore, st *store.Store, logger *slog.Logger) http.Handler {
			tickets = tk
			projection := buildProjectionWithIndex(t, snap, time.Now())
			handler = httpapi.NewVMConsole(projection, auth, cluster.Fake{}, tk, st, logger)

			return handler
		},
	)

	return handler, authHandler, tickets
}

// consoleRequest builds a request with the cluster and vmid path values set.
func consoleRequest(method, path, body string, cookie *http.Cookie) *http.Request {
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

// =============================================================================
// Phase 3 — User Story 1: POST /vnc-ticket (T013–T018)
// =============================================================================

// TestVMConsole_PostVNCTicket_OwnerGetsOpaqueToken — T015: the owner of VM 100
// receives a 200 with a non-empty opaque token. No Proxmox ticket, node, or
// port leaks into the response (FR-002, FR-003).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMConsole_PostVNCTicket_OwnerGetsOpaqueToken(t *testing.T) {
	handler, authHandler, _ := newVMConsoleHandler(t)
	assertOwnerGetsOpaqueToken(t, handler, consoleRequest, "/api/v1/vms/default/100/vnc-ticket", aliceCookie(t, authHandler))
}

// TestVMConsole_PostVNCTicket_NonOwnerForbidden — T016: a non-owner gets 403,
// byte-identical with the other VM endpoints (contracts).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMConsole_PostVNCTicket_NonOwnerForbidden(t *testing.T) {
	handler, authHandler, _ := newVMConsoleHandler(t)
	cookie := bobCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, consoleRequest(http.MethodPost, "/api/v1/vms/default/100/vnc-ticket", "", cookie))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}

	assertAPIError(t, rec.Body.Bytes(), apiCodeForbidden)
}

// TestVMConsole_PostVNCTicket_NotFound — T017: a non-existent VMID gets 404.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMConsole_PostVNCTicket_NotFound(t *testing.T) {
	handler, authHandler, _ := newVMConsoleHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, consoleRequest(http.MethodPost, "/api/v1/vms/default/999/vnc-ticket", "", cookie))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}

	assertAPIError(t, rec.Body.Bytes(), apiCodeNotFound)
}

// TestVMConsole_PostVNCTicket_Unauthenticated — T018: no cookie → 401.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMConsole_PostVNCTicket_Unauthenticated(t *testing.T) {
	handler, _, _ := newVMConsoleHandler(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, consoleRequest(http.MethodPost, "/api/v1/vms/default/100/vnc-ticket", "", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

// =============================================================================
// Phase 3 — User Story 1: GET /console/websocket (T019–T021)
// =============================================================================

// TestVMConsole_WebSocket_MissingTokenReturns400 — T020: a WebSocket request
// without a token parameter is rejected with 400 before the upgrade.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMConsole_WebSocket_MissingTokenReturns400(t *testing.T) {
	handler, authHandler, _ := newVMConsoleHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, consoleRequest(http.MethodGet, "/api/v1/vms/default/100/console/websocket", "", cookie))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

// TestVMConsole_WebSocket_InvalidTokenReturns400 — T020: a WebSocket request
// with a token that was never issued (or already consumed) is rejected with 400.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMConsole_WebSocket_InvalidTokenReturns400(t *testing.T) {
	handler, authHandler, _ := newVMConsoleHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec := httptest.NewRecorder()
	req := consoleRequest(http.MethodGet, "/api/v1/vms/default/100/console/websocket?token=never-issued", "", cookie)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

// TestVMConsole_WebSocket_TicketBoundToDifferentVMRejected — T020: a ticket
// issued for VM 100 cannot be used against VM 101 (FR-004 defense in depth).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMConsole_WebSocket_TicketBoundToDifferentVMRejected(t *testing.T) {
	handler, authHandler, tickets := newVMConsoleHandler(t)

	assertTicketBoundToDifferentVMRejected(t, consoleTestParams{
		kind:       vm.KindVNC,
		handler:    handler,
		tickets:    tickets,
		cookie:     aliceCookie(t, authHandler),
		request:    consoleRequest,
		ticketPath: func(vmid int) string { return "/api/v1/vms/default/" + strconv.Itoa(vmid) + "/vnc-ticket" },
		wsPath:     func(vmid int) string { return "/api/v1/vms/default/" + strconv.Itoa(vmid) + "/console/websocket" },
	})
}

// TestVMConsole_WebSocket_ValidTokenUpgradesAndRelaysRFBHandshake — T016: with
// a valid token, the handler upgrades to WebSocket and the fake relay speaks
// the RFB 3.8 handshake. A real WebSocket client dials the endpoint, reads the
// ProtocolVersion bytes, and asserts they match "RFB 003.008\n" — proving the
// relay is genuinely functional, not a stub (constitution XI).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMConsole_WebSocket_ValidTokenUpgradesAndRelaysRFBHandshake(t *testing.T) {
	handler, authHandler, _ := newVMConsoleHandler(t)
	cookie := aliceCookie(t, authHandler)

	// Issue a ticket via the HTTP endpoint.
	ticketRec := httptest.NewRecorder()
	handler.ServeHTTP(ticketRec, consoleRequest(http.MethodPost, "/api/v1/vms/default/100/vnc-ticket", "", cookie))

	if ticketRec.Code != http.StatusOK {
		t.Fatalf("ticket issuance: status = %d", ticketRec.Code)
	}

	var ticket vncTicketResponse

	_ = json.Unmarshal(ticketRec.Body.Bytes(), &ticket)

	// Start a real HTTP server with a mux that sets the path values the
	// handler expects — the handler reads r.PathValue("cluster") and
	// r.PathValue("vmid"), which only a ServeMux with {cluster}/{vmid}
	// patterns populates.
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/vms/{cluster}/{vmid}/vnc-ticket", handler)
	mux.Handle("GET /api/v1/vms/{cluster}/{vmid}/console/websocket", handler)

	server := httptest.NewServer(mux)
	defer server.Close()

	// Convert the HTTP server URL to a WebSocket URL.
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/vms/default/100/console/websocket?token=" + ticket.Token

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Pass the session cookie and Origin in the WebSocket dial headers — the
	// handler requires an authenticated session and a valid same-origin check
	// before it will upgrade.
	header := http.Header{}
	header.Set("Cookie", cookie.String())
	header.Set("Origin", server.URL)

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: header}) //nolint:bodyclose // coder/websocket owns the response body lifecycle per its Dial docs
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()

	// Read the first message — the fake RFB server's ProtocolVersion string.
	_, reader, err := conn.Reader(ctx)
	if err != nil {
		t.Fatalf("read first message: %v", err)
	}

	versionBytes, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read protocol version: %v", err)
	}

	wantVersion := "RFB 003.008\n"
	if string(versionBytes) != wantVersion {
		t.Fatalf("protocol version = %q, want %q", string(versionBytes), wantVersion)
	}
}

// TestVMConsole_PostVNCTicket_ClusterUnavailableReturns502 — contracts: when
// the cluster client's GetVNCTicket fails, the response is 502
// console_unavailable, not 500.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMConsole_PostVNCTicket_ClusterUnavailableReturns502(t *testing.T) {
	handler, _, cookie := buildFailingRelayHandler(t, 50001, "vm-console-502.db",
		func(snap cluster.Snapshot, authHandler *httpapi.Auth, tickets *vm.ConsoleTicketStore, st *store.Store, logger *slog.Logger) http.Handler {
			projection := buildProjectionWithIndex(t, snap, time.Now())

			return httpapi.NewVMConsole(projection, authHandler, &failingConsoleRelay{}, tickets, st, logger)
		},
	)

	assertClusterUnavailableReturns502(t, handler, consoleRequest, "/api/v1/vms/default/100/vnc-ticket", cookie)
}

// failingConsoleRelay is a ConsoleRelay whose GetVNCTicket always fails — used
// to test the 502 console_unavailable path.
type failingConsoleRelay struct{}

func (failingConsoleRelay) GetVNCTicket(_ context.Context, _ string, _ int, _ string) (cluster.VNCProxyTicket, error) {
	return cluster.VNCProxyTicket{}, net.ErrClosed
}

func (failingConsoleRelay) RelayConsole(_ context.Context, _ string, _ int, _ cluster.VNCProxyTicket, _ io.ReadWriteCloser) error {
	return nil
}
