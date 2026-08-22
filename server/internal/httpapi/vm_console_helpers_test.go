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
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"testing"
)

// ticketResponse is the JSON body for POST /vnc-ticket and POST /serial-ticket.
// Both endpoints return the same shape: only the opaque token and its TTL.
// Kept here so the shared helpers below do not depend on either path's local
// response type.
type ticketResponse struct {
	Token            string `json:"token"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
}

// consoleTestParams parameterizes the shared console test helpers over the
// VNC and serial paths. Each path supplies its handler, request builder,
// ticket/websocket path templates, and the vm.ConsoleKind used to verify
// single-use semantics directly against the store.
type consoleTestParams struct {
	kind       vm.ConsoleKind
	handler    http.Handler
	tickets    *vm.ConsoleTicketStore
	cookie     *http.Cookie
	request    func(method, path, body string, cookie *http.Cookie) *http.Request
	ticketPath func(vmid int) string
	wsPath     func(vmid int) string
}

// assertTicketBoundToDifferentVMRejected is the shared body of
// TestVMConsole_WebSocket_TicketBoundToDifferentVMRejected and
// TestVMSerialConsole_WebSocket_TicketBoundToDifferentVMRejected. It issues a
// ticket for VM 100, attempts to use it against VM 101, asserts a 400, and
// verifies the mismatched attempt did NOT consume the ticket.
func assertTicketBoundToDifferentVMRejected(t *testing.T, p consoleTestParams) {
	t.Helper()

	// Issue a ticket for VM 100 via the HTTP endpoint.
	ticketRec := httptest.NewRecorder()
	p.handler.ServeHTTP(ticketRec, p.request(http.MethodPost, p.ticketPath(100), "", p.cookie))

	if ticketRec.Code != http.StatusOK {
		t.Fatalf("ticket issuance: status = %d", ticketRec.Code)
	}

	var ticket ticketResponse

	_ = json.Unmarshal(ticketRec.Body.Bytes(), &ticket)

	// Try to use it against VM 101 — must be rejected before the upgrade.
	rec := httptest.NewRecorder()
	req := p.request(http.MethodGet, p.wsPath(101)+"?token="+ticket.Token, "", p.cookie)
	p.handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (ticket bound to different VM)", rec.Code, http.StatusBadRequest)
	}

	// The ticket is NOT consumed by the mismatched attempt — it remains valid
	// for its real (cluster, vmid). Verify by consuming it directly.
	if _, err := p.tickets.Consume(p.kind, ticket.Token, "default", 100); err != nil {
		t.Fatalf("ticket was consumed by the mismatched attempt: %v", err)
	}
}

// buildConsoleTestHandler builds a console handler (VNC or serial) over the
// fake dataset with a real audit store. The port and dbPath keep the SQLite
// files distinct between the VNC and serial variants so parallel runs do not
// collide. newHandler is the path-specific constructor that wires the relay
// (Fake for normal tests, a failing relay for 502 tests).
func buildConsoleTestHandler(t *testing.T, port int, dbPath string, newHandler func(cluster.Snapshot, *httpapi.Auth, *vm.ConsoleTicketStore, *store.Store, *slog.Logger) http.Handler) (http.Handler, *httpapi.Auth, *http.Cookie) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	authHandler := newAuthHandler(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	cfg := config.Configuration{
		Port:      port,
		DBPath:    filepath.Join(t.TempDir(), dbPath),
		LogLevel:  snapshotTestLogLevel,
		LogFormat: snapshotTestLogFormat,
		LogOutput: snapshotTestLogOutput,
	}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	tickets := vm.NewConsoleTicketStore()
	handler := newHandler(snap, authHandler, tickets, st, logger)
	cookie := aliceCookie(t, authHandler)

	return handler, authHandler, cookie
}

// buildFailingRelayHandler is a thin alias for buildConsoleTestHandler kept
// for readability at 502-test call sites — the constructor passed in wires a
// failing relay rather than cluster.Fake{}.
func buildFailingRelayHandler(t *testing.T, port int, dbPath string, newHandler func(cluster.Snapshot, *httpapi.Auth, *vm.ConsoleTicketStore, *store.Store, *slog.Logger) http.Handler) (http.Handler, *httpapi.Auth, *http.Cookie) {
	t.Helper()

	return buildConsoleTestHandler(t, port, dbPath, newHandler)
}

// assertClusterUnavailableReturns502 is the shared body of
// TestVMConsole_PostVNCTicket_ClusterUnavailableReturns502 and
// TestVMSerialConsole_PostSerialTicket_ClusterUnavailableReturns502. It issues
// a ticket through a handler whose relay always fails and asserts the response
// is 502 console_unavailable.
func assertClusterUnavailableReturns502(t *testing.T, handler http.Handler, request func(method, path, body string, cookie *http.Cookie) *http.Request, ticketPath string, cookie *http.Cookie) {
	t.Helper()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, request(http.MethodPost, ticketPath, "", cookie))

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}

	var env apiErrorEnvelope

	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Code != "console_unavailable" {
		t.Fatalf("error code = %q, want %q", env.Code, "console_unavailable")
	}
}

// assertOwnerGetsOpaqueToken is the shared body of
// TestVMConsole_PostVNCTicket_OwnerGetsOpaqueToken and
// TestVMSerialConsole_PostSerialTicket_OwnerGetsOpaqueToken. It issues a
// ticket for VM 100 and asserts the response carries only the opaque token and
// its TTL — no Proxmox ticket, node, or port leaks (FR-002, FR-003).
func assertOwnerGetsOpaqueToken(t *testing.T, handler http.Handler, request func(method, path, body string, cookie *http.Cookie) *http.Request, ticketPath string, cookie *http.Cookie) {
	t.Helper()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, request(http.MethodPost, ticketPath, "", cookie))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp ticketResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Token == "" {
		t.Fatalf("token is empty")
	}

	if resp.ExpiresInSeconds != 30 {
		t.Fatalf("expiresInSeconds = %d, want 30", resp.ExpiresInSeconds)
	}

	// FR-002: the response body must contain ONLY token and expiresInSeconds —
	// no Proxmox ticket, node, or port.
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw: %v", err)
	}

	if len(raw) != 2 {
		t.Fatalf("response has %d keys, want exactly 2 (token, expiresInSeconds): %+v", len(raw), raw)
	}
}
