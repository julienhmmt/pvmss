package apiv1

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVNCTicketStore_ConsumeReturnsTicketOnce(t *testing.T) {
	store := makeVNCTicketStore(time.Minute)
	token, err := store.create("100", 5901, "pve-a", "PVEVNC:ticket")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	consoleTicket, ok := store.consume(token, "100")
	if !ok {
		t.Fatalf("expected token to resolve")
	}
	if consoleTicket.ticket != "PVEVNC:ticket" {
		t.Fatalf("expected ticket to match, got %q", consoleTicket.ticket)
	}
	if consoleTicket.port != 5901 {
		t.Fatalf("expected port 5901, got %d", consoleTicket.port)
	}
	if consoleTicket.node != "pve-a" {
		t.Fatalf("expected node pve-a, got %q", consoleTicket.node)
	}
	if _, ok := store.consume(token, "100"); ok {
		t.Fatalf("expected token to be single-use")
	}
}

func TestVNCTicketStore_ConsumeRejectsExpiredToken(t *testing.T) {
	store := makeVNCTicketStore(-time.Second)
	token, err := store.create("100", 5901, "pve-a", "PVEVNC:ticket")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	if _, ok := store.consume(token, "100"); ok {
		t.Fatalf("expected expired token to be rejected")
	}
}

func TestVNCHandler_ResolveVNCConsoleParamsUsesOpaqueToken(t *testing.T) {
	handler := &VNCHandler{vncTickets: makeVNCTicketStore(time.Minute)}
	token, err := handler.vncTickets.create("100", 5901, "pve-a", "PVEVNC:secret")
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/console/websocket?token="+token, nil)
	recorder := httptest.NewRecorder()
	port, node, ticket, ok := handler.resolveVNCConsoleParams(recorder, req, "100")
	if !ok {
		t.Fatalf("expected token params to resolve, status %d", recorder.Code)
	}
	if port != 5901 || node != "pve-a" || ticket != "PVEVNC:secret" {
		t.Fatalf("unexpected params: port=%d node=%q ticket=%q", port, node, ticket)
	}
}

func TestVNCTicketStore_EvictsOldestWhenCapReached(t *testing.T) {
	store := makeVNCTicketStore(time.Hour)
	store.maxEntries = 2

	oldestToken, err := store.create("100", 5901, "pve-a", "ticket-oldest")
	if err != nil {
		t.Fatalf("create oldest: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if _, err := store.create("101", 5902, "pve-a", "ticket-middle"); err != nil {
		t.Fatalf("create middle: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if _, err := store.create("102", 5903, "pve-a", "ticket-newest"); err != nil {
		t.Fatalf("create newest: %v", err)
	}

	if len(store.tickets) != 2 {
		t.Fatalf("expected cap to hold the store at 2 entries, got %d", len(store.tickets))
	}
	if _, ok := store.consume(oldestToken, "100"); ok {
		t.Fatalf("expected oldest token to have been evicted")
	}
}

func TestVNCHandler_ResolveVNCConsoleParamsPreservesLegacyQuery(t *testing.T) {
	handler := &VNCHandler{vncTickets: makeVNCTicketStore(time.Minute)}
	req := httptest.NewRequest(http.MethodGet, "/console/websocket?port=5901&node=pve-a&vncticket=PVEVNC%3Asecret", nil)
	recorder := httptest.NewRecorder()
	port, node, ticket, ok := handler.resolveVNCConsoleParams(recorder, req, "100")
	if !ok {
		t.Fatalf("expected legacy params to resolve, status %d", recorder.Code)
	}
	if port != 5901 || node != "pve-a" || ticket != "PVEVNC:secret" {
		t.Fatalf("unexpected params: port=%d node=%q ticket=%q", port, node, ticket)
	}
}
